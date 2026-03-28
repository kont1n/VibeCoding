# Face Grouper - Database Usage Guide

## PostgreSQL Integration

Face Grouper now supports PostgreSQL with pgvector for persistent storage of faces, persons, and processing sessions.

## Quick Start

### 1. Start PostgreSQL with pgvector

```bash
# Using Docker
docker run -d \
  --name face-grouper-db \
  -e POSTGRES_DB=face-grouper \
  -e POSTGRES_USER=face-grouper \
  -e POSTGRES_PASSWORD=secret \
  -p 5432:5432 \
  pgvector/pgvector:pg16
```

### 2. Configure Environment

Create or update `.env`:

```bash
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_NAME=face-grouper
DB_USER=face-grouper
DB_PASSWORD=secret
DB_SSLMODE=disable

# Migration settings
DB_RUN_MIGRATIONS=true
DB_MAX_CONNS=25
DB_MIN_CONNS=5
DB_MAX_CONN_LIFETIME=3600
DB_MAX_CONN_IDLE_TIME=1800
DB_HEALTH_CHECK_PERIOD=60
```

### 3. Run Application

```bash
# Build
go build -o face-grouper.exe ./cmd

# Run
./face-grouper.exe
```

The application will:
1. Connect to PostgreSQL
2. Run migrations automatically
3. Create tables and indexes
4. Start processing with database support

## Database Schema

### Tables

- **persons** - Clustered persons with metadata
- **faces** - Face embeddings (512-dim vectors via pgvector)
- **photos** - Processed photos metadata
- **person_relations** - Graph relations between persons
- **processing_sessions** - Processing job tracking

### Vector Search

The database supports cosine similarity search on face embeddings:

```sql
-- Find similar faces
SELECT p.name, 1 - (f.embedding <=> query_embedding) as similarity
FROM faces f
JOIN persons p ON f.person_id = p.id
ORDER BY f.embedding <=> query_embedding
LIMIT 10;
```

## Repository Pattern

The application uses repository pattern for database access:

```go
// Get repositories from DI container
personRepo := diContainer.PersonRepository()
faceRepo := diContainer.FaceRepository()
photoRepo := diContainer.PhotoRepository()

// Create person
person := &model.Person{
    ID: uuid.New(),
    Name: "John Doe",
    FaceCount: 5,
    PhotoCount: 3,
}
err := personRepo.Create(ctx, person)

// Find similar faces
similarFaces, err := personRepo.FindSimilarFaces(ctx, embedding, 10)
```

## Health Check

Database health is checked on startup and logged:

```
INFO database connected version="PostgreSQL 16.x" connections=5 extensions="[vector]"
```

## Migrations

Migrations run automatically on startup. Manual migration control:

```go
// In your code
migrator := database.NewMigrator(pool)
err := migrator.Migrate(ctx)
```

Migration files are embedded in `internal/database/migrations/`.

## Redis Cache (Optional)

Redis can be used for caching search results:

```bash
# .env
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

## Testing

Run integration tests:

```bash
# Requires PostgreSQL running
go test -v ./internal/repository/postgres/... -tags=integration
```

## Troubleshooting

### Connection Issues

```bash
# Check PostgreSQL is running
docker ps | grep face-grouper-db

# Test connection
psql -h localhost -U face-grouper -d face-grouper
```

### Migration Errors

```bash
# Check current schema
psql -h localhost -U face-grouper -d face-grouper -c "\dt"

# Reset database (DANGER: deletes all data)
docker rm -f face-grouper-db
docker run ... (see above)
```

### Vector Search Performance

For better vector search performance:

```sql
-- Adjust ivfflat lists parameter
CREATE INDEX CONCURRENTLY idx_faces_embedding 
ON faces USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 200);  -- Increase for larger datasets
```

## Production Deployment

### Environment Variables

```bash
# Production database
DB_HOST=prod-db.example.com
DB_PORT=5432
DB_NAME=face-grouper_prod
DB_USER=face-grouper_app
DB_PASSWORD=<strong-password>
DB_SSLMODE=require

# Connection pool tuning
DB_MAX_CONNS=50
DB_MIN_CONNS=10
DB_MAX_CONN_LIFETIME=7200
DB_MAX_CONN_IDLE_TIME=600
```

### Docker Compose

```yaml
version: '3.8'

services:
  face-grouper:
    image: face-grouper:latest
    environment:
      - DB_HOST=postgres
      - DB_PASSWORD=${DB_PASSWORD}
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: pgvector/pgvector:pg16
    environment:
      - POSTGRES_DB=face-grouper
      - POSTGRES_USER=face-grouper
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U face-grouper"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
```

## Support

For issues or questions:
- GitHub Issues: https://github.com/kont1n/face-grouper/issues
- Documentation: docs/README.md
