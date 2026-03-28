package clustering

import (
	"sync"

	"github.com/kont1n/face-grouper/internal/model"
	"gonum.org/v1/gonum/mat"
)

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

// Cluster groups faces using BLAS-accelerated matrix multiplication for similarity.
// Embeddings are already L2-normalized by the recognizer, so dot product = cosine similarity.
func Cluster(faces []model.Face, threshold float64) []model.Cluster {
	n := len(faces)
	if n == 0 {
		return nil
	}

	dim := len(faces[0].Embedding)
	if dim == 0 {
		return nil
	}

	// Embeddings are already L2-normalized by recognizer, use them directly
	embData := make([]float64, n*dim)
	for i, f := range faces {
		for j, v := range f.Embedding {
			embData[i*dim+j] = v
		}
	}

	E := mat.NewDense(n, dim, embData)
	uf := newUnionFind(n)

	pairs := make(chan intPair, 4096)

	var scanWg sync.WaitGroup
	scanWg.Add(1)
	go func() {
		defer scanWg.Done()
		defer close(pairs)

		for iStart := 0; iStart < n; iStart += blockSize {
			iEnd := iStart + blockSize
			if iEnd > n {
				iEnd = n
			}
			rows := iEnd - iStart
			blockI := E.Slice(iStart, iEnd, 0, dim).(*mat.Dense)

			for jStart := iStart; jStart < n; jStart += blockSize {
				jEnd := jStart + blockSize
				if jEnd > n {
					jEnd = n
				}
				cols := jEnd - jStart
				blockJ := E.Slice(jStart, jEnd, 0, dim).(*mat.Dense)

				sim := mat.NewDense(rows, cols, nil)
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
							pairs <- intPair{gi, gj}
						}
					}
				}
			}
		}
	}()

	for p := range pairs {
		uf.union(p.i, p.j)
	}
	scanWg.Wait()

	groups := make(map[int][]int)
	for i := 0; i < n; i++ {
		root := uf.find(i)
		groups[root] = append(groups[root], i)
	}

	clusters := make([]model.Cluster, 0, len(groups))
	id := 1
	for _, indices := range groups {
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

	return clusters
}
