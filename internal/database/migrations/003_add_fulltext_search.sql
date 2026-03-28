-- +migrate Up
-- =============================================================================
-- Face Grouper - Full-Text Search
-- =============================================================================

-- Function to create search vector for persons
CREATE OR REPLACE FUNCTION persons_search_vector(person persons)
RETURNS tsvector AS $$
BEGIN
    RETURN setweight(to_tsvector('russian', COALESCE(person.custom_name, '')), 'A') ||
           setweight(to_tsvector('russian', COALESCE(person.name, '')), 'B');
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Index for search function
CREATE INDEX IF NOT EXISTS idx_persons_search_vector 
ON persons USING gin(persons_search_vector(persons.*));

-- View for convenient searching
CREATE OR REPLACE VIEW persons_search AS
SELECT 
    id,
    name,
    custom_name,
    avatar_path,
    face_count,
    photo_count,
    quality_score,
    persons_search_vector(persons.*) as search_vector
FROM persons;

-- Comment for documentation
COMMENT ON FUNCTION persons_search_vector IS 'Creates tsvector for full-text search on person names';
COMMENT ON VIEW persons_search IS 'View for full-text search on persons';

-- +migrate Down
-- =============================================================================
-- Drop Full-Text Search
-- =============================================================================
DROP VIEW IF EXISTS persons_search;
DROP INDEX IF EXISTS idx_persons_search_vector;
DROP FUNCTION IF EXISTS persons_search_vector(persons);
