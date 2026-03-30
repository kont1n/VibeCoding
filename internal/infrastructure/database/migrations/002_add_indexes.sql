-- +migrate Up
-- =============================================================================
-- Face Grouper - Indexes for Performance
-- =============================================================================

-- Vector index for fast similarity search (ivfflat for approximate nearest neighbor)
-- lists = rows / 1000 (adjust based on data volume)
CREATE INDEX IF NOT EXISTS idx_faces_embedding 
ON faces USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);

-- Composite index for filtering by person and quality
CREATE INDEX IF NOT EXISTS idx_faces_person_quality 
ON faces(person_id, quality_score DESC);

-- Index for graph relations
CREATE INDEX IF NOT EXISTS idx_relations_similarity 
ON person_relations(similarity DESC);

-- Full-text search index for person names
CREATE INDEX IF NOT EXISTS idx_persons_name_search 
ON persons USING gin(to_tsvector('russian', COALESCE(custom_name, name)));

-- Index for photo metadata
CREATE INDEX IF NOT EXISTS idx_photos_uploaded_at 
ON photos(uploaded_at DESC);

-- Index for face detection score
CREATE INDEX IF NOT EXISTS idx_faces_det_score 
ON faces(det_score DESC);

-- Update statistics for query optimizer
ANALYZE faces;
ANALYZE persons;
ANALYZE photos;
ANALYZE person_relations;

-- +migrate Down
-- =============================================================================
-- Drop Indexes
-- =============================================================================
DROP INDEX IF EXISTS idx_faces_embedding;
DROP INDEX IF EXISTS idx_faces_person_quality;
DROP INDEX IF EXISTS idx_relations_similarity;
DROP INDEX IF EXISTS idx_persons_name_search;
DROP INDEX IF EXISTS idx_photos_uploaded_at;
DROP INDEX IF EXISTS idx_faces_det_score;
