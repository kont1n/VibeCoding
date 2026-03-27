package clustering

import (
	"math"

	"github.com/kont1n/face-grouper/internal/models"
)

// unionFind implements a disjoint-set data structure with path compression and union by rank.
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
	if uf.parent[x] != x {
		uf.parent[x] = uf.find(uf.parent[x])
	}
	return uf.parent[x]
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

// cosineSimilarity computes cosine similarity between two vectors.
// Both vectors are expected to be L2-normalized by InsightFace, so this simplifies to dot product.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// Cluster groups faces by person using cosine similarity of embeddings.
// Faces with similarity >= threshold are considered the same person.
func Cluster(faces []models.Face, threshold float64) []models.Cluster {
	n := len(faces)
	if n == 0 {
		return nil
	}

	uf := newUnionFind(n)

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			sim := cosineSimilarity(faces[i].Embedding, faces[j].Embedding)
			if sim >= threshold {
				uf.union(i, j)
			}
		}
	}

	groups := make(map[int][]int)
	for i := 0; i < n; i++ {
		root := uf.find(i)
		groups[root] = append(groups[root], i)
	}

	clusters := make([]models.Cluster, 0, len(groups))
	id := 1
	for _, indices := range groups {
		var clusterFaces []models.Face
		for _, idx := range indices {
			clusterFaces = append(clusterFaces, faces[idx])
		}
		clusters = append(clusters, models.Cluster{
			ID:    id,
			Faces: clusterFaces,
		})
		id++
	}

	return clusters
}
