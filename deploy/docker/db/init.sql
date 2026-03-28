-- Face Grouper - PostgreSQL Initialization Script
-- This script runs on first container startup

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create app_settings default values
INSERT INTO app_settings (key, value, description) VALUES
    ('app.version', '"1.0.0"', 'Application version'),
    ('app.name', '"Face Grouper"', 'Application name'),
    ('processing.max_workers', '4', 'Maximum processing workers'),
    ('processing.batch_size', '64', 'Default batch size for embedding extraction'),
    ('clustering.threshold', '0.5', 'Default clustering threshold'),
    ('storage.max_file_size', '52428800', 'Maximum file size in bytes (50MB)'),
    ('storage.allowed_formats', '["image/jpeg", "image/png", "image/webp"]', 'Allowed image formats')
ON CONFLICT (key) DO NOTHING;

-- Grant permissions (if needed)
-- GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO face-grouper;
-- GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO face-grouper;
