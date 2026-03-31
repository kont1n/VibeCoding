// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/kont1n/face-grouper/internal/model"
)

// FaceRepository provides database operations for faces.
type FaceRepository struct {
	pool *pgxpool.Pool
}

// NewFaceRepository creates a new face repository.
func NewFaceRepository(pool *pgxpool.Pool) *FaceRepository {
	return &FaceRepository{pool: pool}
}

// Create creates a new face.
func (r *FaceRepository) Create(ctx context.Context, face *model.Face) error {
	query := `
		INSERT INTO faces (id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		                   det_score, quality_score, thumbnail_path, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	embedding := pgvector.NewVector(face.Embedding)

	_, err := r.pool.Exec(ctx, query,
		face.ID,
		face.PersonID,
		face.PhotoID,
		embedding,
		face.BBox.X1,
		face.BBox.Y1,
		face.BBox.X2,
		face.BBox.Y2,
		face.DetScore,
		face.QualityScore,
		face.ThumbnailPath,
		face.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("create face: %w", err)
	}

	return nil
}

// CreateBatch creates multiple faces using pgx.Batch for efficient bulk insert.
func (r *FaceRepository) CreateBatch(ctx context.Context, faces []*model.Face) error {
	if len(faces) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, face := range faces {
		embedding := pgvector.NewVector(face.Embedding)
		batch.Queue(
			`INSERT INTO faces (id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
			                    det_score, quality_score, thumbnail_path, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			face.ID,
			face.PersonID,
			face.PhotoID,
			embedding,
			face.BBox.X1,
			face.BBox.Y1,
			face.BBox.X2,
			face.BBox.Y2,
			face.DetScore,
			face.QualityScore,
			face.ThumbnailPath,
			face.CreatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer func() {
		_ = results.Close()
	}()

	// Check all results.
	for i := 0; i < batch.Len(); i++ {
		_, err := results.Exec()
		if err != nil {
			return fmt.Errorf("create face %d/%d: %w", i+1, batch.Len(), err)
		}
	}

	return nil
}

// GetByID returns a face by ID.
func (r *FaceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Face, error) {
	query := `
		SELECT id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		       det_score, quality_score, thumbnail_path, created_at
		FROM faces
		WHERE id = $1
	`

	face := &model.Face{}
	var embedding pgvector.Vector
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&face.ID,
		&face.PersonID,
		&face.PhotoID,
		&embedding,
		&face.BBox.X1,
		&face.BBox.Y1,
		&face.BBox.X2,
		&face.BBox.Y2,
		&face.DetScore,
		&face.QualityScore,
		&face.ThumbnailPath,
		&face.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get face by id: %w", err)
	}

	face.Embedding = embedding.Slice()
	return face, nil
}

// scanFace scans a face from the current row.
func scanFace(rows interface{ Scan(...any) error }) (*model.Face, error) {
	face := &model.Face{}
	var embedding pgvector.Vector
	err := rows.Scan(
		&face.ID,
		&face.PersonID,
		&face.PhotoID,
		&embedding,
		&face.BBox.X1,
		&face.BBox.Y1,
		&face.BBox.X2,
		&face.BBox.Y2,
		&face.DetScore,
		&face.QualityScore,
		&face.ThumbnailPath,
		&face.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan face: %w", err)
	}
	face.Embedding = embedding.Slice()
	return face, nil
}

// GetByPersonID returns all faces for a person.
func (r *FaceRepository) GetByPersonID(ctx context.Context, personID uuid.UUID) ([]*model.Face, error) {
	query := `
		SELECT id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		       det_score, quality_score, thumbnail_path, created_at
		FROM faces
		WHERE person_id = $1
	`

	rows, err := r.pool.Query(ctx, query, personID)
	if err != nil {
		return nil, fmt.Errorf("get faces by person id: %w", err)
	}
	defer rows.Close()

	var faces []*model.Face
	for rows.Next() {
		face, err := scanFace(rows)
		if err != nil {
			return nil, err
		}
		faces = append(faces, face)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate faces: %w", err)
	}

	return faces, nil
}

// GetByPhotoID returns all faces for a photo.
func (r *FaceRepository) GetByPhotoID(ctx context.Context, photoID uuid.UUID) ([]*model.Face, error) {
	query := `
		SELECT id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		       det_score, quality_score, thumbnail_path, created_at
		FROM faces
		WHERE photo_id = $1
	`

	rows, err := r.pool.Query(ctx, query, photoID)
	if err != nil {
		return nil, fmt.Errorf("get faces by photo id: %w", err)
	}
	defer rows.Close()

	var faces []*model.Face
	for rows.Next() {
		face, err := scanFace(rows)
		if err != nil {
			return nil, err
		}
		faces = append(faces, face)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate faces: %w", err)
	}

	return faces, nil
}

// FindSimilar finds similar faces using vector similarity.
func (r *FaceRepository) FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*model.Face, error) {
	query := `
		SELECT id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		       det_score, quality_score, thumbnail_path, created_at
		FROM faces
		ORDER BY embedding <-> $1
		LIMIT $2
	`

	vec := pgvector.NewVector(embedding)
	rows, err := r.pool.Query(ctx, query, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("find similar faces: %w", err)
	}
	defer rows.Close()

	var faces []*model.Face
	for rows.Next() {
		face, err := scanFace(rows)
		if err != nil {
			return nil, err
		}
		faces = append(faces, face)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate faces: %w", err)
	}

	return faces, nil
}

// Update updates a face.
func (r *FaceRepository) Update(ctx context.Context, face *model.Face) error {
	query := `
		UPDATE faces
		SET person_id = $2, photo_id = $3, embedding = $4, bbox_x1 = $5, bbox_y1 = $6,
		    bbox_x2 = $7, bbox_y2 = $8, det_score = $9, quality_score = $10,
		    thumbnail_path = $11
		WHERE id = $1
	`

	embedding := pgvector.NewVector(face.Embedding)

	_, err := r.pool.Exec(ctx, query,
		face.ID,
		face.PersonID,
		face.PhotoID,
		embedding,
		face.BBox.X1,
		face.BBox.Y1,
		face.BBox.X2,
		face.BBox.Y2,
		face.DetScore,
		face.QualityScore,
		face.ThumbnailPath,
	)

	if err != nil {
		return fmt.Errorf("update face: %w", err)
	}

	return nil
}

// Delete deletes a face.
func (r *FaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM faces WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete face: %w", err)
	}

	return nil
}

// DeleteByPersonID deletes all faces for a person.
func (r *FaceRepository) DeleteByPersonID(ctx context.Context, personID uuid.UUID) error {
	query := `DELETE FROM faces WHERE person_id = $1`

	_, err := r.pool.Exec(ctx, query, personID)
	if err != nil {
		return fmt.Errorf("delete faces by person id: %w", err)
	}

	return nil
}
