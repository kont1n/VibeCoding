// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/model"
)

// PersonRepositoryImpl implements PersonRepository interface.
type PersonRepositoryImpl struct {
	pool *pgxpool.Pool
}

// NewPersonRepository creates a new person repository.
func NewPersonRepository(pool *pgxpool.Pool) *PersonRepositoryImpl {
	return &PersonRepositoryImpl{pool: pool}
}

// Create creates a new person.
func (r *PersonRepositoryImpl) Create(ctx context.Context, person *model.Person) error {
	query := `
		INSERT INTO persons (id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		                     quality_score, face_count, photo_count, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.pool.Exec(ctx, query,
		person.ID,
		person.Name,
		person.CustomName,
		person.AvatarPath,
		person.AvatarThumbnailPath,
		person.QualityScore,
		person.FaceCount,
		person.PhotoCount,
		person.Metadata,
		person.CreatedAt,
		person.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create person: %w", err)
	}

	return nil
}

// GetByID returns a person by ID.
func (r *PersonRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.Person, error) {
	query := `
		SELECT id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		       quality_score, face_count, photo_count, metadata, created_at, updated_at
		FROM persons
		WHERE id = $1
	`

	person := &model.Person{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&person.ID,
		&person.Name,
		&person.CustomName,
		&person.AvatarPath,
		&person.AvatarThumbnailPath,
		&person.QualityScore,
		&person.FaceCount,
		&person.PhotoCount,
		&person.Metadata,
		&person.CreatedAt,
		&person.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get person by id: %w", err)
	}

	return person, nil
}

// GetAll returns all persons.
func (r *PersonRepositoryImpl) GetAll(ctx context.Context) ([]*model.Person, error) {
	query := `
		SELECT id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		       quality_score, face_count, photo_count, metadata, created_at, updated_at
		FROM persons
		ORDER BY name
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all persons: %w", err)
	}
	defer rows.Close()

	var persons []*model.Person
	for rows.Next() {
		person := &model.Person{}
		err := rows.Scan(
			&person.ID,
			&person.Name,
			&person.CustomName,
			&person.AvatarPath,
			&person.AvatarThumbnailPath,
			&person.QualityScore,
			&person.FaceCount,
			&person.PhotoCount,
			&person.Metadata,
			&person.CreatedAt,
			&person.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		persons = append(persons, person)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate persons: %w", err)
	}

	return persons, nil
}

// Update updates a person.
func (r *PersonRepositoryImpl) Update(ctx context.Context, person *model.Person) error {
	query := `
		UPDATE persons
		SET name = $2, custom_name = $3, avatar_path = $4, avatar_thumbnail_path = $5,
		    quality_score = $6, face_count = $7, photo_count = $8, metadata = $9, updated_at = $10
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		person.ID,
		person.Name,
		person.CustomName,
		person.AvatarPath,
		person.AvatarThumbnailPath,
		person.QualityScore,
		person.FaceCount,
		person.PhotoCount,
		person.Metadata,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("update person: %w", err)
	}

	return nil
}

// UpdateName updates person's custom name.
func (r *PersonRepositoryImpl) UpdateName(ctx context.Context, id uuid.UUID, customName string) error {
	query := `
		UPDATE persons
		SET custom_name = $2, updated_at = $3
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, customName, time.Now())
	if err != nil {
		return fmt.Errorf("update person name: %w", err)
	}

	return nil
}

// IncrementFaceCount increments face count for a person.
func (r *PersonRepositoryImpl) IncrementFaceCount(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE persons
		SET face_count = face_count + 1, updated_at = $2
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("increment face count: %w", err)
	}

	return nil
}

// IncrementPhotoCount increments photo count for a person.
func (r *PersonRepositoryImpl) IncrementPhotoCount(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE persons
		SET photo_count = photo_count + 1, updated_at = $2
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("increment photo count: %w", err)
	}

	return nil
}

// Delete deletes a person.
func (r *PersonRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM persons WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete person: %w", err)
	}

	return nil
}

// Search searches for persons by name.
func (r *PersonRepositoryImpl) Search(ctx context.Context, query string) ([]*model.Person, error) {
	sql := `
		SELECT id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		       quality_score, face_count, photo_count, metadata, created_at, updated_at
		FROM persons
		WHERE name ILIKE $1 OR custom_name ILIKE $1
		ORDER BY name
	`

	rows, err := r.pool.Query(ctx, sql, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("search persons: %w", err)
	}
	defer rows.Close()

	var persons []*model.Person
	for rows.Next() {
		person := &model.Person{}
		err := rows.Scan(
			&person.ID,
			&person.Name,
			&person.CustomName,
			&person.AvatarPath,
			&person.AvatarThumbnailPath,
			&person.QualityScore,
			&person.FaceCount,
			&person.PhotoCount,
			&person.Metadata,
			&person.CreatedAt,
			&person.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		persons = append(persons, person)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate persons: %w", err)
	}

	return persons, nil
}

// List returns a paginated list of persons.
func (r *PersonRepositoryImpl) List(ctx context.Context, offset, limit int) ([]*model.Person, error) {
	query := `
		SELECT id, name, custom_name, avatar_path, avatar_thumbnail_path, 
		       quality_score, face_count, photo_count, metadata, created_at, updated_at
		FROM persons
		ORDER BY name
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list persons: %w", err)
	}
	defer rows.Close()

	var persons []*model.Person
	for rows.Next() {
		person := &model.Person{}
		err := rows.Scan(
			&person.ID,
			&person.Name,
			&person.CustomName,
			&person.AvatarPath,
			&person.AvatarThumbnailPath,
			&person.QualityScore,
			&person.FaceCount,
			&person.PhotoCount,
			&person.Metadata,
			&person.CreatedAt,
			&person.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		persons = append(persons, person)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate persons: %w", err)
	}

	return persons, nil
}

// Count returns the total number of persons.
func (r *PersonRepositoryImpl) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM persons`

	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count persons: %w", err)
	}

	return count, nil
}
