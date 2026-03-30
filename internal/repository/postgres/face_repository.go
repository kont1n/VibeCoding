package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/kont1n/face-grouper/internal/model"
)

// FaceRepository implements repository pattern for faces.
type FaceRepository struct {
	pool *pgxpool.Pool
}

// NewFaceRepository creates a new FaceRepository.
func NewFaceRepository(pool *pgxpool.Pool) *FaceRepository {
	return &FaceRepository{pool: pool}
}

// Create creates a new face.
func (r *FaceRepository) Create(ctx context.Context, face *model.Face) error {
	query := `
		INSERT INTO faces (id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		                   det_score, quality_score, thumbnail_path, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
	`

	// Convert []float64 to []float32 for pgvector.
	embedding32 := make([]float32, len(face.Embedding))
	for i, v := range face.Embedding {
		embedding32[i] = float32(v)
	}
	embedding := pgvector.NewVector(embedding32)

	_, err := r.pool.Exec(ctx, query,
		face.ID,
		uuidFromSQL(face.PersonID),
		uuidFromSQL(face.PhotoID),
		embedding,
		face.BBox.X1,
		face.BBox.Y1,
		face.BBox.X2,
		face.BBox.Y2,
		face.DetScore,
		face.QualityScore,
		nullStringToSQL(face.ThumbnailPath),
	)

	return err
}

// GetByID retrieves a face by ID.
func (r *FaceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Face, error) {
	query := `
		SELECT id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		       det_score, quality_score, thumbnail_path, created_at
		FROM faces WHERE id = $1
	`

	face := &model.Face{}
	var embeddingVec pgvector.Vector

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&face.ID,
		&face.PersonID,
		&face.PhotoID,
		&embeddingVec,
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Convert pgvector.Vector ([]float32) to []float64.
	vecSlice := embeddingVec.Slice()
	face.Embedding = make([]float64, len(vecSlice))
	for i, v := range vecSlice {
		face.Embedding[i] = float64(v)
	}

	return face, nil
}

// GetByPersonID retrieves all faces for a person.
//
//nolint:dupl
func (r *FaceRepository) GetByPersonID(ctx context.Context, personID uuid.UUID) ([]*model.Face, error) {
	query := `
		SELECT id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		       det_score, quality_score, thumbnail_path, created_at
		FROM faces WHERE person_id = $1 ORDER BY quality_score DESC
	`

	rows, err := r.pool.Query(ctx, query, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var faces []*model.Face
	for rows.Next() {
		face := &model.Face{}
		var embeddingVec pgvector.Vector

		err := rows.Scan(
			&face.ID,
			&face.PersonID,
			&face.PhotoID,
			&embeddingVec,
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
			return nil, err
		}

		face.Embedding = make([]float64, len(embeddingVec.Slice()))
		for i, v := range embeddingVec.Slice() {
			face.Embedding[i] = float64(v)
		}

		faces = append(faces, face)
	}

	return faces, nil
}

// GetByPhotoID retrieves all faces from a photo.
//
//nolint:dupl
func (r *FaceRepository) GetByPhotoID(ctx context.Context, photoID uuid.UUID) ([]*model.Face, error) {
	query := `
		SELECT id, person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2,
		       det_score, quality_score, thumbnail_path, created_at
		FROM faces WHERE photo_id = $1
	`

	rows, err := r.pool.Query(ctx, query, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var faces []*model.Face
	for rows.Next() {
		face := &model.Face{}
		var embeddingVec pgvector.Vector

		err := rows.Scan(
			&face.ID,
			&face.PersonID,
			&face.PhotoID,
			&embeddingVec,
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
			return nil, err
		}

		face.Embedding = make([]float64, len(embeddingVec.Slice()))
		for i, v := range embeddingVec.Slice() {
			face.Embedding[i] = float64(v)
		}

		faces = append(faces, face)
	}

	return faces, nil
}

// UpdatePersonID updates the person_id for a face.
func (r *FaceRepository) UpdatePersonID(ctx context.Context, faceID, personID uuid.UUID) error {
	query := `UPDATE faces SET person_id = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, faceID, personID)
	return err
}

// Delete deletes a face.
func (r *FaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM faces WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// DeleteByPersonID deletes all faces for a person.
func (r *FaceRepository) DeleteByPersonID(ctx context.Context, personID uuid.UUID) error {
	query := `DELETE FROM faces WHERE person_id = $1`
	_, err := r.pool.Exec(ctx, query, personID)
	return err
}

// Count returns total number of faces.
func (r *FaceRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM faces`).Scan(&count)
	return count, err
}

// Helper.
func float32SliceToFloat64Slice(in []float32) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}

func uuidFromSQL(id any) uuid.UUID {
	if uid, ok := id.(uuid.UUID); ok {
		return uid
	}
	return uuid.Nil
}
