package models

// Face represents a single detected face with its embedding vector.
type Face struct {
	BBox      [4]float64  `json:"bbox"`
	Embedding []float64   `json:"embedding"`
	DetScore  float64     `json:"det_score"`
	FilePath  string      `json:"-"`
}

// ExtractionResult is the JSON structure returned by the Python script.
type ExtractionResult struct {
	Faces []Face `json:"faces"`
	Error string `json:"error,omitempty"`
}

// Cluster groups faces identified as the same person.
type Cluster struct {
	ID    int
	Faces []Face
}
