package models

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

// Face represents a single detected face with its embedding vector.
type Face struct {
	BBox      [4]float64 `json:"bbox"`
	Embedding []float64  `json:"-"`
	DetScore  float64    `json:"det_score"`
	Thumbnail string     `json:"thumbnail,omitempty"`
	FilePath  string     `json:"-"`
}

func (f *Face) UnmarshalJSON(data []byte) error {
	type Alias Face
	aux := &struct {
		*Alias
		RawEmb json.RawMessage `json:"embedding"`
	}{Alias: (*Alias)(f)}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	var s string
	if err := json.Unmarshal(aux.RawEmb, &s); err == nil {
		emb, decErr := decodeBase64Float32(s)
		if decErr != nil {
			return fmt.Errorf("decode base64 embedding: %w", decErr)
		}
		f.Embedding = emb
		return nil
	}

	var arr []float64
	if err := json.Unmarshal(aux.RawEmb, &arr); err != nil {
		return fmt.Errorf("decode embedding: expected base64 string or float array")
	}
	f.Embedding = arr
	return nil
}

func decodeBase64Float32(s string) ([]float64, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("invalid base64 embedding length: %d", len(raw))
	}
	n := len(raw) / 4
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(raw[i*4 : (i+1)*4])
		result[i] = float64(math.Float32frombits(bits))
	}
	return result, nil
}

// ExtractionResult is the JSON structure returned by the Python script.
type ExtractionResult struct {
	File       string `json:"file,omitempty"`
	Faces      []Face `json:"faces"`
	Error      string `json:"error,omitempty"`
	ErrorCount int    `json:"-"`
}

// Cluster groups faces identified as the same person.
type Cluster struct {
	ID    int
	Faces []Face
}
