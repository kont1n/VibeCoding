// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/model"
)

// PhotoRepositoryImpl implements PhotoRepository interface.
type PhotoRepositoryImpl struct {
	pool *pgxpool.Pool
}

// NewPhotoRepository creates a new photo repository.
func NewPhotoRepository(pool *pgxpool.Pool) *PhotoRepositoryImpl {
	return &PhotoRepositoryImpl{pool: pool}
}

// Create creates a new photo.
func (r *PhotoRepositoryImpl) Create(ctx context.Context, photo *model.Photo) error {
	query := `
		INSERT INTO photos (id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.pool.Exec(ctx, query,
		photo.ID,
		photo.Path,
		photo.OriginalPath,
		photo.Width,
		photo.Height,
		photo.FileSize,
		photo.MimeType,
		photo.Metadata,
		photo.UploadedAt,
	)

	if err != nil {
		return fmt.Errorf("create photo: %w", err)
	}

	return nil
}

// CreateBatch creates multiple photos in a single transaction.
func (r *PhotoRepositoryImpl) CreateBatch(ctx context.Context, photos []*model.Photo) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO photos (id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	for _, photo := range photos {
		_, err := tx.Exec(ctx, query,
			photo.ID,
			photo.Path,
			photo.OriginalPath,
			photo.Width,
			photo.Height,
			photo.FileSize,
			photo.MimeType,
			photo.Metadata,
			photo.UploadedAt,
		)
		if err != nil {
			return fmt.Errorf("create photo: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetByID returns a photo by ID.
func (r *PhotoRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.Photo, error) {
	query := `
		SELECT id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at
		FROM photos
		WHERE id = $1
	`

	photo := &model.Photo{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&photo.ID,
		&photo.Path,
		&photo.OriginalPath,
		&photo.Width,
		&photo.Height,
		&photo.FileSize,
		&photo.MimeType,
		&photo.Metadata,
		&photo.UploadedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get photo by id: %w", err)
	}

	return photo, nil
}

// GetByPath returns a photo by path.
func (r *PhotoRepositoryImpl) GetByPath(ctx context.Context, path string) (*model.Photo, error) {
	query := `
		SELECT id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at
		FROM photos
		WHERE path = $1
	`

	photo := &model.Photo{}
	err := r.pool.QueryRow(ctx, query, path).Scan(
		&photo.ID,
		&photo.Path,
		&photo.OriginalPath,
		&photo.Width,
		&photo.Height,
		&photo.FileSize,
		&photo.MimeType,
		&photo.Metadata,
		&photo.UploadedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get photo by path: %w", err)
	}

	return photo, nil
}

// GetAll returns all photos.
func (r *PhotoRepositoryImpl) GetAll(ctx context.Context) ([]*model.Photo, error) {
	query := `
		SELECT id, path, original_path, width, height, file_size, mime_type, metadata, uploaded_at
		FROM photos
		ORDER BY uploaded_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all photos: %w", err)
	}
	defer rows.Close()

	var photos []*model.Photo
	for rows.Next() {
		photo := &model.Photo{}
		err := rows.Scan(
			&photo.ID,
			&photo.Path,
			&photo.OriginalPath,
			&photo.Width,
			&photo.Height,
			&photo.FileSize,
			&photo.MimeType,
			&photo.Metadata,
			&photo.UploadedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan photo: %w", err)
		}
		photos = append(photos, photo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate photos: %w", err)
	}

	return photos, nil
}

// GetByPersonID returns all photos for a person.
func (r *PhotoRepositoryImpl) GetByPersonID(ctx context.Context, personID uuid.UUID) ([]*model.Photo, error) {
	query := `
		SELECT DISTINCT p.id, p.path, p.original_path, p.width, p.height, p.file_size, p.mime_type, p.metadata, p.uploaded_at
		FROM photos p
		INNER JOIN faces f ON f.photo_id = p.id
		WHERE f.person_id = $1
		ORDER BY p.uploaded_at DESC
	`

	rows, err := r.pool.Query(ctx, query, personID)
	if err != nil {
		return nil, fmt.Errorf("get photos by person id: %w", err)
	}
	defer rows.Close()

	var photos []*model.Photo
	for rows.Next() {
		photo := &model.Photo{}
		err := rows.Scan(
			&photo.ID,
			&photo.Path,
			&photo.OriginalPath,
			&photo.Width,
			&photo.Height,
			&photo.FileSize,
			&photo.MimeType,
			&photo.Metadata,
			&photo.UploadedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan photo: %w", err)
		}
		photos = append(photos, photo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate photos: %w", err)
	}

	return photos, nil
}

// Update updates a photo.
func (r *PhotoRepositoryImpl) Update(ctx context.Context, photo *model.Photo) error {
	query := `
		UPDATE photos
		SET path = $2, original_path = $3, width = $4, height = $5, file_size = $6,
		    mime_type = $7, metadata = $8
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		photo.ID,
		photo.Path,
		photo.OriginalPath,
		photo.Width,
		photo.Height,
		photo.FileSize,
		photo.MimeType,
		photo.Metadata,
	)

	if err != nil {
		return fmt.Errorf("update photo: %w", err)
	}

	return nil
}

// Delete deletes a photo.
func (r *PhotoRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM photos WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete photo: %w", err)
	}

	return nil
}

// Exists checks if a photo exists by path.
func (r *PhotoRepositoryImpl) Exists(ctx context.Context, path string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM photos WHERE path = $1)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, path).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check photo exists: %w", err)
	}

	return exists, nil
}

// ListByPerson returns all photos for a person with pagination.
func (r *PhotoRepositoryImpl) ListByPerson(ctx context.Context, personID uuid.UUID, offset, limit int) ([]*model.Photo, error) {
	query := `
		SELECT DISTINCT p.id, p.path, p.original_path, p.width, p.height, p.file_size, p.mime_type, p.metadata, p.uploaded_at
		FROM photos p
		INNER JOIN faces f ON f.photo_id = p.id
		WHERE f.person_id = $1
		ORDER BY p.uploaded_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, personID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list photos by person: %w", err)
	}
	defer rows.Close()

	var photos []*model.Photo
	for rows.Next() {
		photo := &model.Photo{}
		err := rows.Scan(
			&photo.ID,
			&photo.Path,
			&photo.OriginalPath,
			&photo.Width,
			&photo.Height,
			&photo.FileSize,
			&photo.MimeType,
			&photo.Metadata,
			&photo.UploadedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan photo: %w", err)
		}
		photos = append(photos, photo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate photos: %w", err)
	}

	return photos, nil
}

// CountByPerson returns the number of photos for a person.
func (r *PhotoRepositoryImpl) CountByPerson(ctx context.Context, personID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(DISTINCT p.id)
		FROM photos p
		INNER JOIN faces f ON f.photo_id = p.id
		WHERE f.person_id = $1
	`

	var count int
	err := r.pool.QueryRow(ctx, query, personID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count photos by person: %w", err)
	}

	return count, nil
}

// ListByPersonWithFaces returns photos for a person with all faces and bounding boxes.
func (r *PhotoRepositoryImpl) ListByPersonWithFaces(ctx context.Context, personID uuid.UUID, offset, limit int) ([]*model.PhotoWithFaces, error) {
	query := `
		SELECT DISTINCT p.id, p.path, p.width, p.height
		FROM photos p
		INNER JOIN faces f ON f.photo_id = p.id
		WHERE f.person_id = $1
		ORDER BY p.uploaded_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, personID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list photos by person: %w", err)
	}
	defer rows.Close()

	var photos []*model.PhotoWithFaces
	for rows.Next() {
		photo := &model.PhotoWithFaces{}
		var photoID uuid.UUID
		err := rows.Scan(&photoID, &photo.URL, &photo.Width, &photo.Height)
		if err != nil {
			return nil, fmt.Errorf("scan photo: %w", err)
		}

		// Get all faces for this photo.
		facesQuery := `
			SELECT f.bbox_x1, f.bbox_y1, f.bbox_x2, f.bbox_y2, f.det_score, 
			       f.person_id, p.name, p.custom_name
			FROM faces f
			INNER JOIN persons p ON p.id = f.person_id
			WHERE f.photo_id = $1
		`
		facesRows, err := r.pool.Query(ctx, facesQuery, photoID)
		if err != nil {
			return nil, fmt.Errorf("get faces for photo: %w", err)
		}

		for facesRows.Next() {
			var face model.FaceInfo
			var facePersonID uuid.UUID
			var personName, customName string
			var x1, y1, x2, y2, detScore float64

			err := facesRows.Scan(&x1, &y1, &x2, &y2, &detScore, &facePersonID, &personName, &customName)
			if err != nil {
				facesRows.Close()
				return nil, fmt.Errorf("scan face: %w", err)
			}

			face.PersonID = facePersonID.String()
			face.PersonName = customName
			if face.PersonName == "" {
				face.PersonName = personName
			}
			face.IsThisPerson = facePersonID == personID
			face.BBox = model.BBox{
				X1: float32(x1),
				Y1: float32(y1),
				X2: float32(x2),
				Y2: float32(y2),
			}
			face.Confidence = float32(detScore)
			photo.Faces = append(photo.Faces, face)
		}
		facesRows.Close()

		photos = append(photos, photo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate photos: %w", err)
	}

	return photos, nil
}
