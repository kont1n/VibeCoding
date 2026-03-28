-- name: CreateFace :one
INSERT INTO faces (person_id, photo_id, embedding, bbox_x1, bbox_y1, bbox_x2, bbox_y2, det_score, quality_score, thumbnail_path)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetFaceByID :one
SELECT * FROM faces
WHERE id = $1
LIMIT 1;

-- name: GetFacesByPersonID :many
SELECT * FROM faces
WHERE person_id = $1
ORDER BY quality_score DESC;

-- name: GetFacesByPhotoID :many
SELECT * FROM faces
WHERE photo_id = $1;

-- name: UpdateFacePerson :exec
UPDATE faces
SET person_id = $2
WHERE id = $1;

-- name: BatchUpdateFacePerson :exec
UPDATE faces
SET person_id = $2
WHERE id = ANY($1::uuid[]);

-- name: DeleteFace :exec
DELETE FROM faces
WHERE id = $1;

-- name: DeleteFacesByPersonID :exec
DELETE FROM faces
WHERE person_id = $1;

-- name: DeleteFacesByPhotoID :exec
DELETE FROM faces
WHERE photo_id = $1;

-- name: GetFaceEmbedding :one
SELECT embedding FROM faces
WHERE id = $1;

-- name: FindFacesBySimilarity :many
SELECT 
    f.*,
    p.name as person_name,
    1 - (f.embedding <=> $1::vector) as similarity
FROM faces f
JOIN persons p ON f.person_id = p.id
WHERE 1 - (f.embedding <=> $1::vector) >= $2
ORDER BY f.embedding <=> $1::vector
LIMIT $3;

-- name: GetFacesWithoutPerson :many
SELECT * FROM faces
WHERE person_id IS NULL
ORDER BY det_score DESC
LIMIT $1;

-- name: CountFacesByPerson :one
SELECT COUNT(*) FROM faces
WHERE person_id = $1;

-- name: CountFacesByPhoto :one
SELECT COUNT(*) FROM faces
WHERE photo_id = $1;
