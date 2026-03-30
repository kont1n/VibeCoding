-- name: CreatePhoto :one
INSERT INTO photos (path, original_path, width, height, file_size, mime_type, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPhotoByID :one
SELECT * FROM photos
WHERE id = $1
LIMIT 1;

-- name: GetPhotoByPath :one
SELECT * FROM photos
WHERE path = $1
LIMIT 1;

-- name: ListPhotos :many
SELECT * FROM photos
ORDER BY uploaded_at DESC
OFFSET $1
LIMIT $2;

-- name: DeletePhoto :exec
DELETE FROM photos
WHERE id = $1;

-- name: PhotoExists :one
SELECT EXISTS(SELECT 1 FROM photos WHERE path = $1);

-- name: UpdatePhotoMetadata :exec
UPDATE photos
SET metadata = COALESCE($2, metadata)
WHERE id = $1;

-- name: CountPhotosByPerson :many
SELECT COUNT(DISTINCT ph.id)
FROM photos ph
JOIN faces f ON f.photo_id = ph.id
JOIN persons p ON f.person_id = p.id
WHERE p.id = $1;
