package clustering

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/kont1n/face-grouper/internal/model"
)

func TestClusterGroupsSimilarFaces(t *testing.T) {
	t.Parallel()

	faces := []model.Face{
		{Embedding: []float32{1.0, 0.0, 0.0}},
		{Embedding: []float32{0.99, 0.01, 0.0}},
		{Embedding: []float32{-1.0, 0.0, 0.0}},
		{Embedding: []float32{-0.98, -0.02, 0.0}},
	}

	clusters, err := Cluster(context.Background(), faces, 0.95)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}

	sizes := make([]int, 0, len(clusters))
	for _, c := range clusters {
		sizes = append(sizes, len(c.Faces))
	}
	sort.Ints(sizes)
	if sizes[0] != 2 || sizes[1] != 2 {
		t.Fatalf("expected cluster sizes [2,2], got %v", sizes)
	}
}

func TestClusterAppliesTransitiveMerging(t *testing.T) {
	t.Parallel()

	// A~B and B~C above threshold, A~C below threshold.
	faces := []model.Face{
		{Embedding: []float32{1.0, 0.0}},
		{Embedding: []float32{0.8, 0.6}},
		{Embedding: []float32{0.28, 0.96}},
	}

	clusters, err := Cluster(context.Background(), faces, 0.75)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 transitive cluster, got %d", len(clusters))
	}
	if got := len(clusters[0].Faces); got != 3 {
		t.Fatalf("expected cluster size 3, got %d", got)
	}
}

func TestClusterHandlesEmptyInput(t *testing.T) {
	t.Parallel()

	got, err := Cluster(context.Background(), nil, 0.5)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for empty input, got %v", got)
	}
}

func TestClusterHandlesZeroDimensionEmbeddings(t *testing.T) {
	t.Parallel()

	faces := []model.Face{{Embedding: nil}, {Embedding: nil}}
	got, err := Cluster(context.Background(), faces, 0.5)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for zero-dimension embeddings, got %v", got)
	}
}

func TestClusterRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	faces := makeRandomFaces(512, 256)

	clusters, err := Cluster(ctx, faces, 0.5)
	if err == nil {
		t.Fatalf("expected context error, got nil")
	}
	if clusters != nil {
		t.Fatalf("expected nil clusters on context cancellation, got %v", clusters)
	}
}

func BenchmarkCluster512D(b *testing.B) {
	embeddings := makeRandomFaces(1200, 512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Cluster(context.Background(), embeddings, 0.5)
	}
}

func TestTwoStageReducesTransitiveChain(t *testing.T) {
	SetRefineFactor(1.0)
	SetTwoStageConfig(true, 0.96, 0.98, 1)
	defer SetTwoStageConfig(false, 0, 0, 1)

	faces := []model.Face{
		{Embedding: []float32{1.0, 0.0}},
		{Embedding: []float32{0.98, 0.20}},
		{Embedding: []float32{0.92, 0.39}},
		{Embedding: []float32{0.83, 0.56}},
		{Embedding: []float32{0.70, 0.71}},
	}

	clusters, err := Cluster(context.Background(), faces, 0.80)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if len(clusters) <= 1 {
		t.Fatalf("expected split clusters with two-stage mode, got %d", len(clusters))
	}
}

func TestTwoStageMergesMutualNearestMiniClusters(t *testing.T) {
	SetRefineFactor(1.0)
	SetTwoStageConfig(true, 0.99, 0.95, 1)
	defer SetTwoStageConfig(false, 0, 0, 1)

	faces := []model.Face{
		{Embedding: []float32{1.0, 0.0}},
		{Embedding: []float32{0.99, 0.02}},
		{Embedding: []float32{0.97, 0.23}},
		{Embedding: []float32{0.95, 0.30}},
	}

	clusters, err := Cluster(context.Background(), faces, 0.90)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	maxSize := 0
	for _, c := range clusters {
		if len(c.Faces) > maxSize {
			maxSize = len(c.Faces)
		}
	}
	if maxSize < 3 {
		t.Fatalf("expected centroid merge to recover bigger cluster, got max=%d", maxSize)
	}
}

func TestAmbiguityGateConfigCanBeSet(t *testing.T) {
	SetAmbiguityGateConfig(true, 8, 0.52, 0.60, 0.55)
	SetAmbiguityGateConfig(false, 0, 0, 0, 0)
}

func makeRandomFaces(n, dim int) []model.Face {
	r := rand.New(rand.NewSource(42))
	faces := make([]model.Face, n)
	for i := 0; i < n; i++ {
		emb := make([]float32, dim)
		var norm float32
		for j := 0; j < dim; j++ {
			v := float32(r.Float64()*2 - 1)
			emb[j] = v
			norm += v * v
		}
		norm = float32(math.Sqrt(float64(norm)))
		if norm == 0 {
			norm = 1
		}
		for j := 0; j < dim; j++ {
			emb[j] /= norm
		}
		faces[i] = model.Face{Embedding: emb}
	}
	return faces
}
