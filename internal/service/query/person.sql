-- name: CreatePerson :one
INSERT INTO persons (name, custom_name, avatar_path, avatar_thumbnail_path, quality_score, face_count, photo_count, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPersonByID :one
SELECT * FROM persons
WHERE id = $1
LIMIT 1;

-- name: GetPersonByName :one
SELECT * FROM persons
WHERE name = $1 OR custom_name = $1
LIMIT 1;

-- name: ListPersons :many
SELECT * FROM persons
ORDER BY created_at DESC
OFFSET $1
LIMIT $2;

-- name: UpdatePerson :one
UPDATE persons
SET 
    name = COALESCE($2, name),
    custom_name = COALESCE($3, custom_name),
    avatar_path = COALESCE($4, avatar_path),
    avatar_thumbnail_path = COALESCE($5, avatar_thumbnail_path),
    quality_score = COALESCE($6, quality_score),
    face_count = COALESCE($7, face_count),
    photo_count = COALESCE($8, photo_count),
    metadata = COALESCE($9, metadata)
WHERE id = $1
RETURNING *;

-- name: DeletePerson :exec
DELETE FROM persons
WHERE id = $1;

-- name: SearchPersons :many
SELECT * FROM persons
WHERE to_tsvector('russian', COALESCE(custom_name, name)) @@ plainto_tsquery('russian', $1)
ORDER BY ts_rank(to_tsvector('russian', COALESCE(custom_name, name)), plainto_tsquery('russian', $1)) DESC
LIMIT $2;

-- name: FindSimilarFaces :many
SELECT 
    f.id,
    f.person_id,
    f.photo_id,
    f.embedding,
    f.bbox_x1,
    f.bbox_y1,
    f.bbox_x2,
    f.bbox_y2,
    f.det_score,
    f.quality_score,
    f.thumbnail_path,
    f.created_at,
    p.name as person_name,
    p.custom_name as person_custom_name,
    1 - (f.embedding <=> $1::vector) as similarity
FROM faces f
JOIN persons p ON f.person_id = p.id
ORDER BY f.embedding <=> $1::vector
LIMIT $2;

-- name: CountPersons :one
SELECT COUNT(*) FROM persons;

-- name: CountFaces :one
SELECT COUNT(*) FROM faces;

-- name: CountPhotos :one
SELECT COUNT(*) FROM photos;

-- name: UpdatePersonCounts :exec
UPDATE persons
SET 
    face_count = COALESCE($2, face_count),
    photo_count = COALESCE($3, photo_count)
WHERE id = $1;
