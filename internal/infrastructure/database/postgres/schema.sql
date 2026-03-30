-- Face Grouper Database Schema for sqlc
-- This file is used by sqlc to generate Go code.

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Users Table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Persons Table
CREATE TABLE persons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    custom_name TEXT,
    avatar_path TEXT,
    avatar_thumbnail_path TEXT,
    quality_score REAL,
    face_count INTEGER NOT NULL DEFAULT 0,
    photo_count INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Photos Table
CREATE TABLE photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    path TEXT NOT NULL UNIQUE,
    original_path TEXT NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type TEXT NOT NULL DEFAULT 'image/jpeg',
    metadata JSONB NOT NULL DEFAULT '{}',
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Faces Table
CREATE TABLE faces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID REFERENCES persons(id) ON DELETE CASCADE,
    photo_id UUID REFERENCES photos(id) ON DELETE CASCADE,
    embedding vector(512) NOT NULL,
    bbox_x1 REAL NOT NULL,
    bbox_y1 REAL NOT NULL,
    bbox_x2 REAL NOT NULL,
    bbox_y2 REAL NOT NULL,
    det_score REAL NOT NULL,
    quality_score REAL,
    thumbnail_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Person Relations Table
CREATE TABLE person_relations (
    person1_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    person2_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    similarity REAL NOT NULL CHECK (similarity >= 0 AND similarity <= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (person1_id, person2_id)
);

-- Processing Sessions Table
CREATE TABLE processing_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    stage TEXT,
    progress REAL NOT NULL DEFAULT 0,
    total_items INTEGER NOT NULL DEFAULT 0,
    processed_items INTEGER NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0,
    error_details JSONB NOT NULL DEFAULT '[]',
    config JSONB NOT NULL DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id)
);

-- Application Settings Table
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_faces_person_id ON faces(person_id);
CREATE INDEX idx_faces_photo_id ON faces(photo_id);
CREATE INDEX idx_photos_path ON photos(path);
CREATE INDEX idx_persons_name ON persons(name);
CREATE INDEX idx_persons_updated_at ON persons(updated_at DESC);
CREATE INDEX idx_sessions_status ON processing_sessions(status);
CREATE INDEX idx_sessions_created_at ON processing_sessions(created_at DESC);

-- Vector index for similarity search (HNSW - more accurate than IVFFlat)
CREATE INDEX IF NOT EXISTS idx_faces_embedding_hnsw
ON faces USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 200);

-- Composite indexes
CREATE INDEX idx_faces_person_quality ON faces(person_id, quality_score DESC);
CREATE INDEX idx_relations_similarity ON person_relations(similarity DESC);
CREATE INDEX idx_persons_name_search ON persons USING gin(to_tsvector('russian', COALESCE(custom_name, name)));
CREATE INDEX idx_photos_uploaded_at ON photos(uploaded_at DESC);
CREATE INDEX idx_faces_det_score ON faces(det_score DESC);

-- Helper function for similarity search
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

-- Trigger function for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql VOLATILE;

-- Triggers
CREATE TRIGGER update_persons_updated_at
    BEFORE UPDATE ON persons
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_app_settings_updated_at
    BEFORE UPDATE ON app_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
