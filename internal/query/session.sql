-- name: CreateProcessingSession :one
INSERT INTO processing_sessions (status, stage, progress, total_items, processed_items, errors, error_details, config, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetProcessingSession :one
SELECT * FROM processing_sessions
WHERE id = $1
LIMIT 1;

-- name: UpdateProcessingSession :one
UPDATE processing_sessions
SET 
    status = COALESCE($2, status),
    stage = COALESCE($3, stage),
    progress = COALESCE($4, progress),
    processed_items = COALESCE($5, processed_items),
    errors = COALESCE($6, errors),
    error_details = COALESCE($7, error_details),
    completed_at = CASE WHEN $2 = 'completed' OR $2 = 'failed' THEN NOW() ELSE completed_at END
WHERE id = $1
RETURNING *;

-- name: GetActiveProcessingSession :one
SELECT * FROM processing_sessions
WHERE status = 'processing'
ORDER BY started_at DESC
LIMIT 1;

-- name: ListProcessingSessions :many
SELECT * FROM processing_sessions
OFFSET $1
LIMIT $2;

-- name: DeleteOldProcessingSessions :exec
DELETE FROM processing_sessions
WHERE created_at < NOW() - INTERVAL '30 days';

-- name: GetProcessingSessionStats :one
SELECT 
    COUNT(*) as total_sessions,
    COUNT(*) FILTER (WHERE status = 'completed') as completed,
    COUNT(*) FILTER (WHERE status = 'failed') as failed,
    COUNT(*) FILTER (WHERE status = 'processing') as processing,
    AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) FILTER (WHERE status = 'completed') as avg_duration_seconds
FROM processing_sessions
WHERE created_at > NOW() - INTERVAL '7 days';
