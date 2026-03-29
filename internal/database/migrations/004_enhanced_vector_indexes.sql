-- +migrate Up
-- =============================================================================
-- Face Grouper - Enhanced Vector Indexes and Query Optimization
-- =============================================================================

-- HNSW index for better accuracy (slower to build, but more accurate than IVFFlat)
-- m = 16 (number of connections per node)
-- ef_construction = 200 (size of dynamic candidate list during construction)
CREATE INDEX IF NOT EXISTS idx_faces_embedding_hnsw
ON faces USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 200);

-- Index for session processing order
CREATE INDEX IF NOT EXISTS idx_sessions_progress
ON processing_sessions(progress DESC, status);

-- =============================================================================
-- Optimized query for finding similar faces with threshold filtering
-- This query uses the index efficiently by filtering before sorting
-- =============================================================================

-- Create a helper function for efficient similarity search with threshold
CREATE OR REPLACE FUNCTION find_similar_faces(
    query_embedding vector(512),
    max_results INTEGER DEFAULT 20,
    min_similarity REAL DEFAULT 0.5
)
RETURNS TABLE (
    face_id UUID,
    person_id UUID,
    photo_id UUID,
    similarity REAL,
    bbox_x1 REAL,
    bbox_y1 REAL,
    bbox_x2 REAL,
    bbox_y2 REAL,
    det_score REAL,
    quality_score REAL,
    thumbnail_path TEXT,
    person_name TEXT,
    person_custom_name TEXT
)
LANGUAGE sql
STABLE
AS $$
    SELECT 
        f.id AS face_id,
        f.person_id,
        f.photo_id,
        1 - (f.embedding <=> query_embedding) AS similarity,
        f.bbox_x1,
        f.bbox_y1,
        f.bbox_x2,
        f.bbox_y2,
        f.det_score,
        f.quality_score,
        f.thumbnail_path,
        p.name AS person_name,
        p.custom_name AS person_custom_name
    FROM faces f
    LEFT JOIN persons p ON f.person_id = p.id
    WHERE f.embedding <=> query_embedding < (1 - min_similarity)
    ORDER BY f.embedding <=> query_embedding
    LIMIT max_results;
$$;

-- =============================================================================
-- Set pgvector tuning parameters for better performance
-- =============================================================================

-- Set IVFFlat probe parameter for better accuracy (default is 1)
-- This can be adjusted at query time using SET
-- SET ivfflat.probes = 10;

-- Set HNSW ef_search parameter for better accuracy (default is 40)
-- This can be adjusted at query time using SET
-- SET hnsw.ef_search = 100;

-- +migrate Down
-- =============================================================================
-- Drop Enhanced Indexes and Functions
-- =============================================================================
DROP INDEX IF EXISTS idx_faces_embedding_hnsw;
DROP INDEX IF EXISTS idx_sessions_progress;
DROP FUNCTION IF EXISTS find_similar_faces(vector, INTEGER, REAL);
