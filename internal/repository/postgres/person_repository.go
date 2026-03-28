// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/kont1n/face-grouper/internal/model"
)

// PersonRepository implements repository pattern for persons.
type PersonRepository struct {
	pool *pgxpool.Pool
}

// NewPersonRepository creates a new PersonRepository.
func NewPersonRepository(pool *pgxpool.Pool) *PersonRepository {
	return &PersonRepository{
		pool: pool,
	}
}

// Create creates a new person in the database.
func (r *PersonRepository) Create(ctx context.Context, person *model.Person) error {
	query := `
		INSERT INTO persons (id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		                     quality_score, face_count, photo_count, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`

	metadataJSON, err := json.Marshal(person.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = r.pool.Exec(ctx, query,
		person.ID,
		person.Name,
		nullStringToSql(person.CustomName),
		nullStringToSql(person.AvatarPath),
		nullStringToSql(person.AvatarThumbnailPath),
		nullFloat32ToSql(person.QualityScore),
		person.FaceCount,
		person.PhotoCount,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("create person: %w", err)
	}

	return nil
}

// GetByID retrieves a person by ID.
func (r *PersonRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Person, error) {
	query := `
		SELECT id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		       quality_score, face_count, photo_count, metadata, created_at, updated_at
		FROM persons
		WHERE id = $1
	`

	person := &model.Person{}
	var metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&person.ID,
		&person.Name,
		&person.CustomName,
		&person.AvatarPath,
		&person.AvatarThumbnailPath,
		&person.QualityScore,
		&person.FaceCount,
		&person.PhotoCount,
		&metadataJSON,
		&person.CreatedAt,
		&person.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get person by id: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &person.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return person, nil
}

// GetByName retrieves a person by name or custom_name.
func (r *PersonRepository) GetByName(ctx context.Context, name string) (*model.Person, error) {
	query := `
		SELECT id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		       quality_score, face_count, photo_count, metadata, created_at, updated_at
		FROM persons
		WHERE name = $1 OR custom_name = $1
	`

	person := &model.Person{}
	var metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, name).Scan(
		&person.ID,
		&person.Name,
		&person.CustomName,
		&person.AvatarPath,
		&person.AvatarThumbnailPath,
		&person.QualityScore,
		&person.FaceCount,
		&person.PhotoCount,
		&metadataJSON,
		&person.CreatedAt,
		&person.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get person by name: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &person.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return person, nil
}

// List retrieves a list of persons with pagination.
func (r *PersonRepository) List(ctx context.Context, offset, limit int) ([]*model.Person, error) {
	query := `
		SELECT id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		       quality_score, face_count, photo_count, metadata, created_at, updated_at
		FROM persons
		ORDER BY created_at DESC
		OFFSET $1 LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("list persons: %w", err)
	}
	defer rows.Close()

	var persons []*model.Person
	for rows.Next() {
		person := &model.Person{}
		var metadataJSON []byte

		err := rows.Scan(
			&person.ID,
			&person.Name,
			&person.CustomName,
			&person.AvatarPath,
			&person.AvatarThumbnailPath,
			&person.QualityScore,
			&person.FaceCount,
			&person.PhotoCount,
			&metadataJSON,
			&person.CreatedAt,
			&person.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &person.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}

		persons = append(persons, person)
	}

	return persons, nil
}

// Update updates an existing person.
func (r *PersonRepository) Update(ctx context.Context, person *model.Person) error {
	query := `
		UPDATE persons
		SET name = $2, custom_name = $3, avatar_path = $4, avatar_thumbnail_path = $5,
		    quality_score = $6, face_count = $7, photo_count = $8, 
		    metadata = $9, updated_at = NOW()
		WHERE id = $1
	`

	metadataJSON, err := json.Marshal(person.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = r.pool.Exec(ctx, query,
		person.ID,
		person.Name,
		nullStringToSql(person.CustomName),
		nullStringToSql(person.AvatarPath),
		nullStringToSql(person.AvatarThumbnailPath),
		nullFloat32ToSql(person.QualityScore),
		person.FaceCount,
		person.PhotoCount,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("update person: %w", err)
	}

	return nil
}

// Delete deletes a person by ID.
func (r *PersonRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM persons WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete person: %w", err)
	}

	return nil
}

// Search searches for persons by name using full-text search.
func (r *PersonRepository) Search(ctx context.Context, query string, limit int) ([]*model.Person, error) {
	sqlQuery := `
		SELECT id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		       quality_score, face_count, photo_count, metadata, created_at, updated_at
		FROM persons
		WHERE to_tsvector('russian', COALESCE(custom_name, name)) @@ plainto_tsquery('russian', $1)
		ORDER BY ts_rank(to_tsvector('russian', COALESCE(custom_name, name)), plainto_tsquery('russian', $1)) DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, sqlQuery, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search persons: %w", err)
	}
	defer rows.Close()

	var persons []*model.Person
	for rows.Next() {
		person := &model.Person{}
		var metadataJSON []byte

		err := rows.Scan(
			&person.ID,
			&person.Name,
			&person.CustomName,
			&person.AvatarPath,
			&person.AvatarThumbnailPath,
			&person.QualityScore,
			&person.FaceCount,
			&person.PhotoCount,
			&metadataJSON,
			&person.CreatedAt,
			&person.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &person.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}

		persons = append(persons, person)
	}

	return persons, nil
}

// FindSimilarFaces finds faces similar to the given embedding.
func (r *PersonRepository) FindSimilarFaces(ctx context.Context, embedding []float32, limit int) ([]*model.SimilarFace, error) {
	query := `
		SELECT f.id, f.person_id, f.photo_id, f.embedding, f.bbox_x1, f.bbox_y1, f.bbox_x2, f.bbox_y2,
		       f.det_score, f.quality_score, f.thumbnail_path, f.created_at,
		       p.name as person_name, p.custom_name as person_custom_name,
		       1 - (f.embedding <=> $1::vector) as similarity
		FROM faces f
		JOIN persons p ON f.person_id = p.id
		ORDER BY f.embedding <=> $1::vector
		LIMIT $2
	`

	vec := pgvector.NewVector(embedding)
	rows, err := r.pool.Query(ctx, query, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("find similar faces: %w", err)
	}
	defer rows.Close()

	var similarFaces []*model.SimilarFace
	for rows.Next() {
		var face model.SimilarFace
		var embeddingVec pgvector.Vector

		err := rows.Scan(
			&face.Face.ID,
			&face.Face.PersonID,
			&face.Face.PhotoID,
			&embeddingVec,
			&face.Face.BBox.X1,
			&face.Face.BBox.Y1,
			&face.Face.BBox.X2,
			&face.Face.BBox.Y2,
			&face.Face.DetScore,
			&face.Face.QualityScore,
			&face.Face.ThumbnailPath,
			&face.Face.CreatedAt,
			&face.PersonName,
			&face.PersonCustomName,
			&face.Similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("scan similar face: %w", err)
		}

		// Copy embedding data
		face.Face.Embedding = make([]float64, len(embeddingVec.Slice()))
		for i, v := range embeddingVec.Slice() {
			face.Face.Embedding[i] = float64(v)
		}

		similarFaces = append(similarFaces, &face)
	}

	return similarFaces, nil
}

// Count returns the total number of persons.
func (r *PersonRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM persons`

	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count persons: %w", err)
	}

	return count, nil
}

// Helper functions
func nullStringToSql(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullFloat32ToSql(f float32) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(f), Valid: true}
}
