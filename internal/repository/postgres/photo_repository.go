package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/model"
)

// PhotoRepository implements repository pattern for photos.
type PhotoRepository struct {
	pool *pgxpool.Pool
}

// NewPhotoRepository creates a new PhotoRepository.
func NewPhotoRepository(pool *pgxpool.Pool) *PhotoRepository {
	return &PhotoRepository{pool: pool}
}

// Create creates a new photo.
func (r *PhotoRepository) Create(ctx context.Context, photo *model.Photo) error {
	query := `
		INSERT INTO photos (id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	`

	metadataJSON, err := json.Marshal(photo.Metadata)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, query,
		photo.ID,
		photo.Path,
		photo.OriginalPath,
		photo.Width,
		photo.Height,
		photo.FileSize,
		photo.MimeType,
		metadataJSON,
	)

	return err
}

// GetByID retrieves a photo by ID.
func (r *PhotoRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Photo, error) {
	query := `
		SELECT id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at
		FROM photos WHERE id = $1
	`

	photo := &model.Photo{}
	var metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&photo.ID,
		&photo.Path,
		&photo.OriginalPath,
		&photo.Width,
		&photo.Height,
		&photo.FileSize,
		&photo.MimeType,
		&metadataJSON,
		&photo.UploadedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(metadataJSON, &photo.Metadata); err != nil {
		return nil, err
	}

	return photo, nil
}

// GetByPath retrieves a photo by path.
func (r *PhotoRepository) GetByPath(ctx context.Context, path string) (*model.Photo, error) {
	query := `
		SELECT id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at
		FROM photos WHERE path = $1
	`

	photo := &model.Photo{}
	var metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, path).Scan(
		&photo.ID,
		&photo.Path,
		&photo.OriginalPath,
		&photo.Width,
		&photo.Height,
		&photo.FileSize,
		&photo.MimeType,
		&metadataJSON,
		&photo.UploadedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(metadataJSON, &photo.Metadata); err != nil {
		return nil, err
	}

	return photo, nil
}

// Exists checks if a photo exists by path.
func (r *PhotoRepository) Exists(ctx context.Context, path string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM photos WHERE path = $1)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, path).Scan(&exists)
	return exists, err
}

// List retrieves photos with pagination.
func (r *PhotoRepository) List(ctx context.Context, offset, limit int) ([]*model.Photo, error) {
	query := `
		SELECT id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at
		FROM photos ORDER BY uploaded_at DESC OFFSET $1 LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*model.Photo
	for rows.Next() {
		photo := &model.Photo{}
		var metadataJSON []byte

		err := rows.Scan(
			&photo.ID,
			&photo.Path,
			&photo.OriginalPath,
			&photo.Width,
			&photo.Height,
			&photo.FileSize,
			&photo.MimeType,
			&metadataJSON,
			&photo.UploadedAt,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(metadataJSON, &photo.Metadata); err != nil {
			return nil, err
		}

		photos = append(photos, photo)
	}

	return photos, nil
}

// ListByPerson retrieves photos for a specific person with pagination.
func (r *PhotoRepository) ListByPerson(ctx context.Context, personID uuid.UUID, offset, limit int) ([]*model.Photo, error) {
	query := `
		SELECT DISTINCT ph.id, ph.path, ph.original_path, ph.width, ph.height,
		       ph.file_size, ph.mime_type, ph.metadata, ph.uploaded_at
		FROM photos ph
		JOIN faces f ON f.photo_id = ph.id
		WHERE f.person_id = $1
		ORDER BY ph.uploaded_at DESC
		OFFSET $2 LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, personID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*model.Photo
	for rows.Next() {
		photo := &model.Photo{}
		var metadataJSON []byte

		err := rows.Scan(
			&photo.ID,
			&photo.Path,
			&photo.OriginalPath,
			&photo.Width,
			&photo.Height,
			&photo.FileSize,
			&photo.MimeType,
			&metadataJSON,
			&photo.UploadedAt,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(metadataJSON, &photo.Metadata); err != nil {
			return nil, err
		}

		photos = append(photos, photo)
	}

	return photos, nil
}

// Delete deletes a photo.
func (r *PhotoRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM photos WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// Count returns total number of photos.
func (r *PhotoRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM photos`).Scan(&count)
	return count, err
}

// CountByPerson returns number of photos for a person.
func (r *PhotoRepository) CountByPerson(ctx context.Context, personID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(DISTINCT ph.id)
		FROM photos ph
		JOIN faces f ON f.photo_id = ph.id
		WHERE f.person_id = $1
	`
	var count int
	err := r.pool.QueryRow(ctx, query, personID).Scan(&count)
	return count, err
}
