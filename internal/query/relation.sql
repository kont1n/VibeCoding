-- name: CreatePersonRelation :exec
INSERT INTO person_relations (person1_id, person2_id, similarity)
VALUES ($1, $2, $3)
ON CONFLICT (person1_id, person2_id) DO NOTHING;

-- name: BatchCreatePersonRelations :exec
INSERT INTO person_relations (person1_id, person2_id, similarity)
SELECT * FROM UNNEST($1::uuid[], $2::uuid[], $3::float4[])
ON CONFLICT (person1_id, person2_id) DO NOTHING;

-- name: GetPersonRelations :many
SELECT 
    pr.person1_id,
    pr.person2_id,
    pr.similarity,
    p1.name as person1_name,
    p1.custom_name as person1_custom_name,
    p1.avatar_path as person1_avatar,
    p2.name as person2_name,
    p2.custom_name as person2_custom_name,
    p2.avatar_path as person2_avatar
FROM person_relations pr
JOIN persons p1 ON pr.person1_id = p1.id
JOIN persons p2 ON pr.person2_id = p2.id
WHERE pr.similarity >= $1
ORDER BY pr.similarity DESC;

-- name: GetPersonGraph :many
SELECT 
    p.id,
    p.name,
    p.custom_name,
    p.avatar_path,
    p.face_count,
    p.photo_count,
    ARRAY_AGG(DISTINCT pr.person2_id) FILTER (WHERE pr.person2_id IS NOT NULL) as connected_person_ids,
    ARRAY_AGG(DISTINCT pr.similarity) FILTER (WHERE pr.similarity IS NOT NULL) as connection_similarities
FROM persons p
LEFT JOIN person_relations pr ON p.id = pr.person1_id AND pr.similarity >= $2
WHERE p.id = ANY($1::uuid[])
GROUP BY p.id;

-- name: DeletePersonRelations :exec
DELETE FROM person_relations
WHERE person1_id = $1 OR person2_id = $1;

-- name: GetRelationStrength :one
SELECT similarity FROM person_relations
WHERE (person1_id = $1 AND person2_id = $2)
   OR (person1_id = $2 AND person2_id = $1)
LIMIT 1;

-- name: CountPersonRelations :one
SELECT COUNT(*) FROM person_relations
WHERE person1_id = $1 OR person2_id = $1;
