-- +migrate Up
-- =============================================================================
-- Face Grouper - Remove Duplicate IVFFlat Index
-- =============================================================================
-- Migration 004 added HNSW index which provides better accuracy.
-- Having both IVFFlat and HNSW on the same column creates unnecessary
-- write overhead. This migration removes the IVFFlat index.

DROP INDEX IF EXISTS idx_faces_embedding;

-- +migrate Down
-- =============================================================================
-- Restore IVFFlat Index (if needed for rollback)
-- =============================================================================
-- lists = rows / 1000 (adjust based on data volume)
CREATE INDEX IF NOT EXISTS idx_faces_embedding
ON faces USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);
