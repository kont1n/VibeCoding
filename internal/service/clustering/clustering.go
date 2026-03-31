// Package clustering implements face embedding clustering algorithms.
package clustering

import (
	"context"
	"math"
	"sync"

	"gonum.org/v1/gonum/mat"

	"github.com/kont1n/face-grouper/internal/model"
)

// ClusterService defines the interface for clustering operations.
type ClusterService interface {
	Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error)
}

type clusterService struct{}

// NewClusterService creates a new ClusterService.
func NewClusterService() ClusterService {
	return &clusterService{}
}

// Cluster groups faces using cosine similarity.
func (s *clusterService) Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error) {
	return Cluster(ctx, faces, threshold)
}

type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	uf := &unionFind{
		parent: make([]int, n),
		rank:   make([]int, n),
	}
	for i := range uf.parent {
		uf.parent[i] = i
	}
	return uf
}

func (uf *unionFind) find(x int) int {
	for uf.parent[x] != x {
		uf.parent[x] = uf.parent[uf.parent[x]]
		x = uf.parent[x]
	}
	return x
}

func (uf *unionFind) union(x, y int) {
	rx, ry := uf.find(x), uf.find(y)
	if rx == ry {
		return
	}
	if uf.rank[rx] < uf.rank[ry] {
		rx, ry = ry, rx
	}
	uf.parent[ry] = rx
	if uf.rank[rx] == uf.rank[ry] {
		uf.rank[rx]++
	}
}

type intPair struct{ i, j int }

const blockSize = 512

// denseMatrixPool is a sync.Pool for reusing []float64 slices to reduce GC pressure
// during block-wise matrix multiplication.
var denseMatrixPool = sync.Pool{
	New: func() any {
		// Pre-allocate slice for blockSize x blockSize matrix.
		data := make([]float64, blockSize*blockSize)
		return &data
	},
}

// Cluster groups faces using BLAS-accelerated matrix multiplication for similarity.
// Applies L2-normalization to embeddings to ensure dot product = cosine similarity.
func Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error) {
	n := len(faces)
	if n == 0 {
		return nil, nil
	}

	dim := len(faces[0].Embedding)
	if dim == 0 {
		return nil, nil
	}

	// L2-normalize embeddings (defensive, even if recognizer already normalized).
	// Embeddings are now float32, but Gonum uses float64 internally.
	embData := make([]float64, n*dim)
	for i, f := range faces {
		norm := float64(0)
		for _, v := range f.Embedding {
			vf := float64(v)
			norm += vf * vf
		}
		norm = math.Sqrt(norm)
		if norm == 0 {
			norm = 1
		}
		for j, v := range f.Embedding {
			embData[i*dim+j] = float64(v) / norm
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	E := mat.NewDense(n, dim, embData)
	uf := newUnionFind(n)

	pairs := make(chan intPair, 4096)
	errCh := make(chan error, 1)

	var scanWg sync.WaitGroup
	scanWg.Add(1)
	go func() {
		defer scanWg.Done()
		defer close(pairs)

		for iStart := 0; iStart < n; iStart += blockSize {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			iEnd := iStart + blockSize
			if iEnd > n {
				iEnd = n
			}
			rows := iEnd - iStart
			blockI := E.Slice(iStart, iEnd, 0, dim).(*mat.Dense) //nolint:forcetypeassert,errcheck

			for jStart := iStart; jStart < n; jStart += blockSize {
				// Check context in inner loop for faster cancellation.
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				default:
				}

				jEnd := jStart + blockSize
				if jEnd > n {
					jEnd = n
				}
				cols := jEnd - jStart
				blockJ := E.Slice(jStart, jEnd, 0, dim).(*mat.Dense) //nolint:forcetypeassert,errcheck

				// Get pre-allocated slice from pool for similarity matrix.
				simDataSlicePtr, ok := denseMatrixPool.Get().(*[]float64)
				if !ok {
					fallback := make([]float64, blockSize*blockSize)
					simDataSlicePtr = &fallback
				}
				simDataSlice := *simDataSlicePtr
				sim := mat.NewDense(rows, cols, simDataSlice[:rows*cols])
				sim.Mul(blockI, blockJ.T())

				simData := sim.RawMatrix()
				for li := 0; li < rows; li++ {
					gi := iStart + li
					rowOff := li * simData.Stride
					jBegin := 0
					if iStart == jStart {
						jBegin = li + 1
					}
					for lj := jBegin; lj < cols; lj++ {
						gj := jStart + lj
						if gi >= gj {
							continue
						}
						if simData.Data[rowOff+lj] >= threshold {
							select {
							case pairs <- intPair{gi, gj}:
							case <-ctx.Done():
								denseMatrixPool.Put(simDataSlicePtr)
								errCh <- ctx.Err()
								return
							}
						}
					}
				}

				// Return slice to pool after use.
				denseMatrixPool.Put(simDataSlicePtr)
			}
		}
	}()

	// Process pairs with context cancellation support.
processPairs:
	for {
		select {
		case p, ok := <-pairs:
			if !ok {
				break processPairs
			}
			uf.union(p.i, p.j)
		case <-ctx.Done():
			scanWg.Wait()
			// Drain remaining pairs to avoid goroutine leak.
			go func() {
				for range pairs {
					_ = struct{}{} // Drain.
				}
			}()
			return nil, ctx.Err()
		}
	}
	scanWg.Wait()

	// Check for cancellation error from worker.
	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	// Group faces by cluster root.
	groups := make(map[int][]int)
	for i := 0; i < n; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		root := uf.find(i)
		groups[root] = append(groups[root], i)
	}

	// Refinement: verify each face against cluster centroid.
	// Faces with low average similarity to cluster members are split off.
	refined := refineClusters(ctx, groups, embData, dim, threshold)

	clusters := make([]model.Cluster, 0, len(refined))
	id := 1
	for _, indices := range refined {
		clusterFaces := make([]model.Face, len(indices))
		for k, idx := range indices {
			clusterFaces[k] = faces[idx]
		}
		clusters = append(clusters, model.Cluster{
			ID:    id,
			Faces: clusterFaces,
		})
		id++
	}

	return clusters, nil
}

// refineClusters splits outlier faces from clusters.
// For each cluster with >2 faces, compute the centroid and remove faces
// whose average cosine similarity to other members is below the threshold.
func refineClusters(ctx context.Context, groups map[int][]int, embData []float64, dim int, threshold float64) [][]int {
	// Use a slightly more lenient threshold for refinement to avoid over-splitting.
	refineThreshold := threshold * 0.85

	var result [][]int
	for _, indices := range groups {
		select {
		case <-ctx.Done():
			// On cancellation, return what we have without refinement.
			result = append(result, indices)
			continue
		default:
		}

		if len(indices) <= 2 {
			result = append(result, indices)
			continue
		}

		// Compute centroid of the cluster.
		centroid := make([]float64, dim)
		for _, idx := range indices {
			off := idx * dim
			for d := 0; d < dim; d++ {
				centroid[d] += embData[off+d]
			}
		}
		n := float64(len(indices))
		norm := float64(0)
		for d := 0; d < dim; d++ {
			centroid[d] /= n
			norm += centroid[d] * centroid[d]
		}
		norm = math.Sqrt(norm)
		if norm > 0 {
			for d := 0; d < dim; d++ {
				centroid[d] /= norm
			}
		}

		// Check each face's similarity to centroid.
		var keep, outliers []int
		for _, idx := range indices {
			sim := dotProduct(embData, idx*dim, centroid, dim)
			if sim >= refineThreshold {
				keep = append(keep, idx)
			} else {
				outliers = append(outliers, idx)
			}
		}

		if len(keep) == 0 {
			// All faces are outliers — keep original cluster.
			result = append(result, indices)
			continue
		}

		result = append(result, keep)

		// Each outlier becomes its own singleton cluster.
		for _, idx := range outliers {
			result = append(result, []int{idx})
		}
	}

	return result
}

// dotProduct computes dot product between embedding at offset and a vector.
func dotProduct(embData []float64, offset int, vec []float64, dim int) float64 {
	sum := float64(0)
	for d := 0; d < dim; d++ {
		sum += embData[offset+d] * vec[d]
	}
	return sum
}
