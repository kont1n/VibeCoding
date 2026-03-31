// Package model defines data models for the application.
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Person represents a clustered person.
type Person struct {
	ID                  uuid.UUID       `json:"id"`
	Name                string          `json:"name"`
	CustomName          string          `json:"custom_name"`
	AvatarPath          string          `json:"avatar_path"`
	AvatarThumbnailPath string          `json:"avatar_thumbnail_path"`
	QualityScore        float32         `json:"quality_score"`
	FaceCount           int             `json:"face_count"`
	PhotoCount          int             `json:"photo_count"`
	Metadata            json.RawMessage `json:"metadata"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// Face represents a detected face.
type Face struct {
	ID            uuid.UUID     `json:"id"`
	PersonID      uuid.UUID     `json:"person_id"`
	PhotoID       uuid.UUID     `json:"photo_id"`
	Embedding     []float32     `json:"embedding"` // float32 matches pgvector vector(512).
	BBox          BBox          `json:"bbox"`
	Keypoints     [5][2]float64 `json:"keypoints"`
	DetScore      float32       `json:"det_score"`
	QualityScore  float32       `json:"quality_score"`
	Thumbnail     string        `json:"thumbnail"`
	FilePath      string        `json:"file_path"`
	ThumbnailPath string        `json:"thumbnail_path"`
	CreatedAt     time.Time     `json:"created_at"`
}

// BBox represents a bounding box.
type BBox struct {
	X1 float32 `json:"x1"`
	Y1 float32 `json:"y1"`
	X2 float32 `json:"x2"`
	Y2 float32 `json:"y2"`
}

// Photo represents a processed photo.
type Photo struct {
	ID           uuid.UUID       `json:"id"`
	Path         string          `json:"path"`
	OriginalPath string          `json:"original_path"`
	Width        int32           `json:"width"`
	Height       int32           `json:"height"`
	FileSize     int64           `json:"file_size"`
	MimeType     string          `json:"mime_type"`
	Metadata     json.RawMessage `json:"metadata"`
	UploadedAt   time.Time       `json:"uploaded_at"`
}

// PersonRelation represents a relation between two persons.
type PersonRelation struct {
	Person1ID  uuid.UUID `json:"person1_id"`
	Person2ID  uuid.UUID `json:"person2_id"`
	Similarity float32   `json:"similarity"`
	CreatedAt  time.Time `json:"created_at"`
}

// PersonNode represents a node in the person graph.
type PersonNode struct {
	ID                     uuid.UUID   `json:"id"`
	Name                   string      `json:"name"`
	CustomName             string      `json:"custom_name"`
	AvatarPath             string      `json:"avatar_path"`
	FaceCount              int         `json:"face_count"`
	PhotoCount             int         `json:"photo_count"`
	ConnectedPersonIDs     []uuid.UUID `json:"connected_person_ids"`
	ConnectionSimilarities []float32   `json:"connection_similarities"`
}

// ProcessingSession represents a processing job.
type ProcessingSession struct {
	ID             uuid.UUID       `json:"id"`
	Status         string          `json:"status"`
	Stage          string          `json:"stage"`
	Progress       float32         `json:"progress"`
	TotalItems     int             `json:"total_items"`
	ProcessedItems int             `json:"processed_items"`
	Errors         int             `json:"errors"`
	ErrorDetails   []ErrorDetail   `json:"error_details"`
	Config         json.RawMessage `json:"config"`
	StartedAt      time.Time       `json:"started_at"`
	CompletedAt    time.Time       `json:"completed_at"`
	CreatedBy      uuid.UUID       `json:"created_by"`
}

// ErrorDetail represents a processing error.
type ErrorDetail struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

// ProcessingStats represents processing statistics.
type ProcessingStats struct {
	TotalSessions      int     `json:"total_sessions"`
	Completed          int     `json:"completed"`
	Failed             int     `json:"failed"`
	Processing         int     `json:"processing"`
	AvgDurationSeconds float32 `json:"avg_duration_seconds"`
}

// SimilarFace represents a face with similarity score.
type SimilarFace struct {
	Face             Face    `json:"face"`
	PersonName       string  `json:"person_name"`
	PersonCustomName string  `json:"person_custom_name"`
	Similarity       float32 `json:"similarity"`
}

// Cluster represents a group of faces belonging to one person.
type Cluster struct {
	ID    int    `json:"id"`
	Faces []Face `json:"faces"`
}
