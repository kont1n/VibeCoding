// Package clustering implements face embedding clustering algorithms.
package clustering

import (
	"context"
	"math"
	"sort"
	"sync"

	"gonum.org/v1/gonum/mat"

	"github.com/kont1n/face-grouper/internal/model"
)

// ClusterService defines the interface for clustering operations.
type ClusterService interface {
	Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error)
}

type clusterService struct{}

var refineFactor = 1.0
var twoStageEnabled bool
var twoStagePreThreshold float64
var twoStageCentroidThreshold float64
var twoStageMutualK = 1
var ambiguityGateEnabled bool
var ambiguityTopK = 12
var ambiguityMeanMin float64
var ambiguityMeanMax float64
var ambiguityCentroidMax float64
var configMu sync.RWMutex

// NewClusterService creates a new ClusterService.
func NewClusterService() ClusterService {
	return &clusterService{}
}

// SetRefineFactor tunes centroid refinement strictness.
// Values < 1.0 are more lenient; values > 1.0 are stricter.
func SetRefineFactor(v float64) {
	configMu.Lock()
	defer configMu.Unlock()
	if v <= 0 {
		refineFactor = 1.0
		return
	}
	refineFactor = v
}

// SetTwoStageConfig configures optional two-stage clustering mode.
func SetTwoStageConfig(enabled bool, preThreshold, centroidThreshold float64, mutualK int) {
	configMu.Lock()
	defer configMu.Unlock()
	twoStageEnabled = enabled
	twoStagePreThreshold = preThreshold
	twoStageCentroidThreshold = centroidThreshold
	if mutualK <= 0 {
		mutualK = 1
	}
	twoStageMutualK = mutualK
}

// SetAmbiguityGateConfig configures optional ambiguity-based face pruning.
func SetAmbiguityGateConfig(enabled bool, topK int, meanMin, meanMax, centroidMax float64) {
	configMu.Lock()
	defer configMu.Unlock()
	ambiguityGateEnabled = enabled
	if topK <= 0 {
		topK = 12
	}
	ambiguityTopK = topK
	ambiguityMeanMin = meanMin
	ambiguityMeanMax = meanMax
	ambiguityCentroidMax = centroidMax
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
type simEdge struct {
	j   int
	sim float64
}

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

	configMu.RLock()
	localRefine := refineFactor
	localTwoStage := twoStageEnabled
	localPre := twoStagePreThreshold
	localCentroid := twoStageCentroidThreshold
	localMutualK := twoStageMutualK
	localAmbigEnabled := ambiguityGateEnabled
	localAmbigTopK := ambiguityTopK
	localAmbigMeanMin := ambiguityMeanMin
	localAmbigMeanMax := ambiguityMeanMax
	localAmbigCentroidMax := ambiguityCentroidMax
	configMu.RUnlock()

	var groups map[int][]int
	var err error
	if localTwoStage {
		groups, err = clusterTwoStage(ctx, embData, dim, threshold, localRefine, localPre, localCentroid, localMutualK)
	} else {
		groups, err = clusterByPairThreshold(ctx, embData, dim, threshold)
	}
	if err != nil {
		return nil, err
	}

	// Refinement: verify each face against cluster centroid.
	// Faces with low average similarity to cluster members are split off.
	refined := refineClusters(
		ctx,
		groups,
		embData,
		dim,
		threshold,
		localRefine,
		localAmbigEnabled,
		localAmbigTopK,
		localAmbigMeanMin,
		localAmbigMeanMax,
		localAmbigCentroidMax,
	)

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

func clusterByPairThreshold(ctx context.Context, embData []float64, dim int, threshold float64) (map[int][]int, error) {
	n := len(embData) / dim
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
				denseMatrixPool.Put(simDataSlicePtr)
			}
		}
	}()

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
			go func() {
				for range pairs {
				}
			}()
			return nil, ctx.Err()
		}
	}
	scanWg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	groups := make(map[int][]int)
	for i := 0; i < n; i++ {
		root := uf.find(i)
		groups[root] = append(groups[root], i)
	}
	return groups, nil
}

func clusterTwoStage(
	ctx context.Context,
	embData []float64,
	dim int,
	threshold float64,
	refine float64,
	preCfg float64,
	centroidCfg float64,
	mutualKCfg int,
) (map[int][]int, error) {
	preThreshold := preCfg
	if preThreshold <= 0 {
		preThreshold = threshold + 0.08
	}
	if preThreshold > 0.99 {
		preThreshold = 0.99
	}
	centroidThreshold := centroidCfg
	if centroidThreshold <= 0 {
		centroidThreshold = threshold
	}

	stage1, err := clusterByPairThreshold(ctx, embData, dim, preThreshold)
	if err != nil {
		return nil, err
	}
	stage1Refined := refineClusters(ctx, stage1, embData, dim, preThreshold, refine, false, 0, 0, 0, 0)
	if len(stage1Refined) <= 1 {
		return sliceGroupsToMap(stage1Refined), nil
	}

	centroids := make([][]float64, len(stage1Refined))
	for i, idxs := range stage1Refined {
		centroids[i] = computeCentroid(embData, idxs, dim)
	}

	top := make([][]simEdge, len(stage1Refined))
	for i := 0; i < len(stage1Refined); i++ {
		for j := i + 1; j < len(stage1Refined); j++ {
			sim := dotProduct(centroids[i], 0, centroids[j], dim)
			if sim < centroidThreshold {
				continue
			}
			top[i] = append(top[i], simEdge{j: j, sim: sim})
			top[j] = append(top[j], simEdge{j: i, sim: sim})
		}
	}
	k := mutualKCfg
	if k <= 0 {
		k = 1
	}
	for i := range top {
		sort.Slice(top[i], func(a, b int) bool { return top[i][a].sim > top[i][b].sim })
		if len(top[i]) > k {
			top[i] = top[i][:k]
		}
	}

	uf := newUnionFind(len(stage1Refined))
	for i := 0; i < len(stage1Refined); i++ {
		for _, e := range top[i] {
			if containsNeighbor(top[e.j], i) {
				uf.union(i, e.j)
			}
		}
	}

	merged := make(map[int][]int)
	for i := 0; i < len(stage1Refined); i++ {
		root := uf.find(i)
		merged[root] = append(merged[root], stage1Refined[i]...)
	}
	return merged, nil
}

func containsNeighbor(edges []simEdge, target int) bool {
	for _, e := range edges {
		if e.j == target {
			return true
		}
	}
	return false
}

func computeCentroid(embData []float64, indices []int, dim int) []float64 {
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
	return centroid
}

func sliceGroupsToMap(groups [][]int) map[int][]int {
	out := make(map[int][]int, len(groups))
	for i, g := range groups {
		out[i] = g
	}
	return out
}

// refineClusters splits outlier faces from clusters.
// For each cluster with >2 faces, compute the centroid and remove faces
// whose average cosine similarity to other members is below the threshold.
func refineClusters(
	ctx context.Context,
	groups map[int][]int,
	embData []float64,
	dim int,
	threshold, factor float64,
	ambigEnabled bool,
	ambigTopK int,
	ambigMeanMin float64,
	ambigMeanMax float64,
	ambigCentroidMax float64,
) [][]int {
	refineThreshold := threshold * factor

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

		if len(keep) > 0 {
			// For large clusters, require local neighborhood support to prevent
			// transitive "bridge" merges that create one giant identity.
			keep, outliers = pruneByLocalDensity(keep, outliers, embData, dim, refineThreshold)
		}
		if ambigEnabled && len(keep) > 0 {
			keep, outliers = pruneByEmbeddingAmbiguity(
				keep,
				outliers,
				embData,
				dim,
				refineThreshold,
				ambigTopK,
				ambigMeanMin,
				ambigMeanMax,
				ambigCentroidMax,
			)
		}

		parts := splitLargeClusterByConnectivity(keep, embData, dim, refineThreshold)
		result = append(result, parts...)

		// Each outlier becomes its own singleton cluster.
		for _, idx := range outliers {
			result = append(result, []int{idx})
		}
	}

	return result
}

func pruneByEmbeddingAmbiguity(
	keep, outliers []int,
	embData []float64,
	dim int,
	refineThreshold float64,
	topK int,
	meanMinCfg float64,
	meanMaxCfg float64,
	centroidMaxCfg float64,
) ([]int, []int) {
	if len(keep) < 80 {
		return keep, outliers
	}
	if topK <= 0 {
		topK = 12
	}
	meanMin := meanMinCfg
	meanMax := meanMaxCfg
	centroidMax := centroidMaxCfg
	if meanMin <= 0 {
		meanMin = refineThreshold + 0.01
	}
	if meanMax <= 0 {
		meanMax = refineThreshold + 0.12
	}
	if centroidMax <= 0 {
		centroidMax = refineThreshold + 0.05
	}
	if meanMax <= meanMin {
		meanMax = meanMin + 0.05
	}

	centroid := make([]float64, dim)
	for _, idx := range keep {
		off := idx * dim
		for d := 0; d < dim; d++ {
			centroid[d] += embData[off+d]
		}
	}
	n := float64(len(keep))
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

	limit := topK
	if limit > len(keep)-1 {
		limit = len(keep) - 1
	}
	if limit <= 0 {
		return keep, outliers
	}

	denseKeep := make([]int, 0, len(keep))
	for i, idx := range keep {
		sims := make([]float64, 0, len(keep)-1)
		for j, other := range keep {
			if i == j {
				continue
			}
			sims = append(sims, dotProduct(embData, idx*dim, embData[other*dim:other*dim+dim], dim))
		}
		sort.Slice(sims, func(a, b int) bool { return sims[a] > sims[b] })
		sum := 0.0
		for k := 0; k < limit; k++ {
			sum += sims[k]
		}
		meanTopK := sum / float64(limit)
		centroidSim := dotProduct(embData, idx*dim, centroid, dim)

		isAmbiguous := meanTopK >= meanMin && meanTopK <= meanMax && centroidSim <= centroidMax
		if isAmbiguous {
			outliers = append(outliers, idx)
		} else {
			denseKeep = append(denseKeep, idx)
		}
	}
	if len(denseKeep) == 0 {
		return keep, outliers
	}
	return denseKeep, outliers
}

func pruneByLocalDensity(keep, outliers []int, embData []float64, dim int, refineThreshold float64) ([]int, []int) {
	// Small clusters are already stable enough.
	if len(keep) < 25 {
		return keep, outliers
	}

	strictThreshold := refineThreshold + 0.03
	if strictThreshold > 0.98 {
		strictThreshold = 0.98
	}

	const minNeighbors = 2
	var denseKeep []int
	for i, idx := range keep {
		neighbors := 0
		for j, other := range keep {
			if i == j {
				continue
			}
			if dotProduct(embData, idx*dim, embData[other*dim:other*dim+dim], dim) >= strictThreshold {
				neighbors++
				if neighbors >= minNeighbors {
					break
				}
			}
		}

		if neighbors >= minNeighbors {
			denseKeep = append(denseKeep, idx)
		} else {
			outliers = append(outliers, idx)
		}
	}

	// Avoid collapsing a large cluster entirely due to over-strict settings.
	if len(denseKeep) == 0 {
		return keep, outliers
	}

	return denseKeep, outliers
}

func splitLargeClusterByConnectivity(keep []int, embData []float64, dim int, refineThreshold float64) [][]int {
	if len(keep) == 0 {
		return nil
	}
	// Keep smaller/medium clusters untouched.
	if len(keep) < 40 {
		return [][]int{keep}
	}

	splitThreshold := refineThreshold + 0.05
	if splitThreshold > 0.985 {
		splitThreshold = 0.985
	}

	adj := make([][]int, len(keep))
	for i := 0; i < len(keep); i++ {
		for j := i + 1; j < len(keep); j++ {
			if dotProduct(embData, keep[i]*dim, embData[keep[j]*dim:keep[j]*dim+dim], dim) >= splitThreshold {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			}
		}
	}

	visited := make([]bool, len(keep))
	var comps [][]int
	for i := 0; i < len(keep); i++ {
		if visited[i] {
			continue
		}
		queue := []int{i}
		visited[i] = true
		var comp []int
		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			comp = append(comp, keep[u])
			for _, v := range adj[u] {
				if !visited[v] {
					visited[v] = true
					queue = append(queue, v)
				}
			}
		}
		comps = append(comps, comp)
	}

	return comps
}

// dotProduct computes dot product between embedding at offset and a vector.
func dotProduct(embData []float64, offset int, vec []float64, dim int) float64 {
	sum := float64(0)
	for d := 0; d < dim; d++ {
		sum += embData[offset+d] * vec[d]
	}
	return sum
}
