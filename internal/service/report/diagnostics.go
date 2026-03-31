package report

import (
	"math"
	"sort"

	"github.com/kont1n/face-grouper/internal/model"
)

// ClusterDiagnostics contains diagnostics for top-N largest clusters.
type ClusterDiagnostics struct {
	TopClusters []ClusterDiagnostic `json:"top_clusters,omitempty"`
}

// ClusterDiagnostic stores aggregate quality signals for one cluster.
type ClusterDiagnostic struct {
	ClusterID            int             `json:"cluster_id"`
	FaceCount            int             `json:"face_count"`
	SimilarityMin        float64         `json:"similarity_min"`
	SimilarityMedian     float64         `json:"similarity_median"`
	SimilarityP95        float64         `json:"similarity_p95"`
	CentroidSimMin       float64         `json:"centroid_sim_min"`
	CentroidSimMedian    float64         `json:"centroid_sim_median"`
	CentroidSimP95       float64         `json:"centroid_sim_p95"`
	WeakEdgeRatio        float64         `json:"weak_edge_ratio"`
	LowConnectivityRatio float64         `json:"low_connectivity_ratio"`
	DetScoreMedian       float64         `json:"det_score_median"`
	QualityScoreMedian   float64         `json:"quality_score_median"`
	BBoxAreaMedian       float64         `json:"bbox_area_median"`
	TopOutliers          []OutlierSample `json:"top_outliers,omitempty"`
}

// OutlierSample stores a potentially problematic face within a cluster.
type OutlierSample struct {
	FilePath       string  `json:"file_path"`
	DetScore       float64 `json:"det_score"`
	QualityScore   float64 `json:"quality_score"`
	BBoxArea       float64 `json:"bbox_area"`
	CentroidSim    float64 `json:"centroid_sim"`
	NeighborDegree int     `json:"neighbor_degree"`
}

// AnalyzeClusters computes diagnostics for largest clusters in a run.
func AnalyzeClusters(clusters []model.Cluster, threshold, refineFactor float64, topN int) *ClusterDiagnostics {
	if len(clusters) == 0 {
		return nil
	}
	if topN <= 0 {
		topN = 5
	}
	sorted := make([]model.Cluster, len(clusters))
	copy(sorted, clusters)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i].Faces) > len(sorted[j].Faces) })
	if len(sorted) > topN {
		sorted = sorted[:topN]
	}
	refineThreshold := threshold * refineFactor
	out := &ClusterDiagnostics{TopClusters: make([]ClusterDiagnostic, 0, len(sorted))}
	for _, c := range sorted {
		d := analyzeCluster(c, threshold, refineThreshold)
		out.TopClusters = append(out.TopClusters, d)
	}
	return out
}

func analyzeCluster(c model.Cluster, threshold, refineThreshold float64) ClusterDiagnostic {
	faces := c.Faces
	diag := ClusterDiagnostic{
		ClusterID: c.ID,
		FaceCount: len(faces),
	}
	if len(faces) <= 1 {
		return diag
	}
	dim := len(faces[0].Embedding)
	normEmb := make([][]float64, len(faces))
	for i := range faces {
		normEmb[i] = normalizeEmbedding(faces[i].Embedding)
	}
	centroid := make([]float64, dim)
	for i := range normEmb {
		for d := 0; d < dim; d++ {
			centroid[d] += normEmb[i][d]
		}
	}
	n := float64(len(normEmb))
	cNorm := float64(0)
	for d := 0; d < dim; d++ {
		centroid[d] /= n
		cNorm += centroid[d] * centroid[d]
	}
	cNorm = math.Sqrt(cNorm)
	if cNorm > 0 {
		for d := 0; d < dim; d++ {
			centroid[d] /= cNorm
		}
	}
	pairSims := make([]float64, 0, len(faces)*(len(faces)-1)/2)
	centroidSims := make([]float64, 0, len(faces))
	degrees := make([]int, len(faces))
	detScores := make([]float64, 0, len(faces))
	qualityScores := make([]float64, 0, len(faces))
	bboxAreas := make([]float64, 0, len(faces))
	for i := range faces {
		detScores = append(detScores, float64(faces[i].DetScore))
		qualityScores = append(qualityScores, float64(faces[i].QualityScore))
		bboxAreas = append(bboxAreas, bboxArea(faces[i].BBox))
		centroidSims = append(centroidSims, dot(normEmb[i], centroid))
	}
	weakEdges := 0
	for i := 0; i < len(faces); i++ {
		for j := i + 1; j < len(faces); j++ {
			s := dot(normEmb[i], normEmb[j])
			pairSims = append(pairSims, s)
			if s < threshold {
				weakEdges++
			} else {
				degrees[i]++
				degrees[j]++
			}
		}
	}
	diag.SimilarityMin, diag.SimilarityMedian, diag.SimilarityP95 = minMedianP95(pairSims)
	diag.CentroidSimMin, diag.CentroidSimMedian, diag.CentroidSimP95 = minMedianP95(centroidSims)
	diag.DetScoreMedian = median(detScores)
	diag.QualityScoreMedian = median(qualityScores)
	diag.BBoxAreaMedian = median(bboxAreas)
	if len(pairSims) > 0 {
		diag.WeakEdgeRatio = float64(weakEdges) / float64(len(pairSims))
	}
	minDegree := 1
	if len(faces) >= 6 {
		minDegree = 2
	}
	lowConn := 0
	for _, d := range degrees {
		if d < minDegree {
			lowConn++
		}
	}
	diag.LowConnectivityRatio = float64(lowConn) / float64(len(degrees))
	type outlier struct {
		idx  int
		sim  float64
		deg  int
		file string
	}
	var outliers []outlier
	for i := range faces {
		if centroidSims[i] < refineThreshold {
			outliers = append(outliers, outlier{idx: i, sim: centroidSims[i], deg: degrees[i], file: faces[i].FilePath})
		}
	}
	sort.Slice(outliers, func(i, j int) bool { return outliers[i].sim < outliers[j].sim })
	if len(outliers) > 10 {
		outliers = outliers[:10]
	}
	for _, o := range outliers {
		f := faces[o.idx]
		diag.TopOutliers = append(diag.TopOutliers, OutlierSample{
			FilePath:       o.file,
			DetScore:       float64(f.DetScore),
			QualityScore:   float64(f.QualityScore),
			BBoxArea:       bboxArea(f.BBox),
			CentroidSim:    o.sim,
			NeighborDegree: o.deg,
		})
	}
	return diag
}

func normalizeEmbedding(in []float32) []float64 {
	out := make([]float64, len(in))
	norm := float64(0)
	for i, v := range in {
		out[i] = float64(v)
		norm += out[i] * out[i]
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		norm = 1
	}
	for i := range out {
		out[i] /= norm
	}
	return out
}

func dot(a, b []float64) float64 {
	sum := 0.0
	for i := 0; i < len(a) && i < len(b); i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func bboxArea(b model.BBox) float64 {
	w := float64(b.X2 - b.X1)
	h := float64(b.Y2 - b.Y1)
	if w < 0 || h < 0 {
		return 0
	}
	return w * h
}

func minMedianP95(values []float64) (minVal, medianVal, p95 float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	return cp[0], median(cp), percentile(cp, 0.95)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	m := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[m]
	}
	return (cp[m-1] + cp[m]) / 2
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(float64(len(sorted))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
