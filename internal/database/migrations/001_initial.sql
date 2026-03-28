-- +migrate Up
-- =============================================================================
-- Face Grouper - Initial Schema
-- =============================================================================

-- Enable pgvector extension for vector similarity search
CREATE EXTENSION IF NOT EXISTS vector;

-- =============================================================================
-- Users Table (for future authentication)
-- =============================================================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Persons Table
-- =============================================================================
CREATE TABLE persons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    custom_name TEXT,  -- User-defined name (if renamed)
    avatar_path TEXT,
    avatar_thumbnail_path TEXT,
    quality_score REAL,
    face_count INTEGER NOT NULL DEFAULT 0,
    photo_count INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Photos Table
-- =============================================================================
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

-- =============================================================================
-- Faces Table
-- =============================================================================
CREATE TABLE faces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID REFERENCES persons(id) ON DELETE CASCADE,
    photo_id UUID REFERENCES photos(id) ON DELETE CASCADE,
    embedding vector(512) NOT NULL,  -- 512-dimensional ArcFace embedding
    bbox_x1 REAL NOT NULL,
    bbox_y1 REAL NOT NULL,
    bbox_x2 REAL NOT NULL,
    bbox_y2 REAL NOT NULL,
    det_score REAL NOT NULL,
    quality_score REAL,
    thumbnail_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Person Relations Table (for graph visualization)
-- =============================================================================
CREATE TABLE person_relations (
    person1_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    person2_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    similarity REAL NOT NULL CHECK (similarity >= 0 AND similarity <= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (person1_id, person2_id)
);

-- =============================================================================
-- Processing Sessions Table
-- =============================================================================
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

-- =============================================================================
-- Application Settings Table
-- =============================================================================
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Indexes
-- =============================================================================
CREATE INDEX idx_faces_person_id ON faces(person_id);
CREATE INDEX idx_faces_photo_id ON faces(photo_id);
CREATE INDEX idx_photos_path ON photos(path);
CREATE INDEX idx_persons_name ON persons(name);
CREATE INDEX idx_persons_updated_at ON persons(updated_at DESC);
CREATE INDEX idx_sessions_status ON processing_sessions(status);
CREATE INDEX idx_sessions_created_at ON processing_sessions(created_at DESC);

-- =============================================================================
-- Triggers for updated_at
-- =============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

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

-- +migrate Down
-- =============================================================================
-- Drop Schema
-- =============================================================================
DROP TABLE IF EXISTS app_settings CASCADE;
DROP TABLE IF EXISTS processing_sessions CASCADE;
DROP TABLE IF EXISTS person_relations CASCADE;
DROP TABLE IF EXISTS faces CASCADE;
DROP TABLE IF EXISTS photos CASCADE;
DROP TABLE IF EXISTS persons CASCADE;
DROP TABLE IF EXISTS users CASCADE;

DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;

DROP EXTENSION IF EXISTS vector;
