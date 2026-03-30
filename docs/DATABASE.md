# Face Grouper - Database Integration

## PostgreSQL + pgvector

Face Grouper поддерживает PostgreSQL с расширением pgvector для персистентного хранения данных о лицах, персонах и сессиях обработки.

**База данных опциональна** — приложение запускается и без неё, читая данные из `report.json`.

## Возможности

| Функция | Без БД | С БД |
|---------|--------|------|
| Обработка фото | ✅ | ✅ |
| Просмотр результатов | ✅ (report.json) | ✅ (PostgreSQL) |
| Пагинация персон | ❌ | ✅ |
| Переименование персон | ❌ | ✅ |
| Векторный поиск лиц | ❌ | ✅ |
| Граф связей персон | ❌ | ✅ |
| Full-text search | ❌ | ✅ |
| Трекинг сессий | In-memory | PostgreSQL |

## Быстрый старт

### 1. Запуск PostgreSQL с pgvector

```bash
docker run -d \
  --name face-grouper-db \
  -e POSTGRES_DB=face-grouper \
  -e POSTGRES_USER=face-grouper \
  -e POSTGRES_PASSWORD=secret \
  -p 5432:5432 \
  pgvector/pgvector:pg16

# Проверка
docker ps | grep face-grouper-db
```

### 2. Настройка конфигурации

Добавьте в `.env`:

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=face-grouper
DB_USER=face-grouper
DB_PASSWORD=secret
DB_SSLMODE=disable

# Connection Pool
DB_MAX_CONNS=25
DB_MIN_CONNS=5
DB_MAX_CONN_LIFETIME=3600
DB_MAX_CONN_IDLE_TIME=1800
DB_HEALTH_CHECK_PERIOD=60

# Миграции
DB_RUN_MIGRATIONS=true

# Redis (опционально)
# REDIS_HOST=localhost
# REDIS_PORT=6379
# REDIS_PASSWORD=
# REDIS_DB=0
```

### 3. Запуск

```bash
go build -o face-grouper ./cmd
./face-grouper --serve
```

При старте приложение:
1. Подключится к PostgreSQL
2. Применит миграции автоматически
3. Создаст таблицы и индексы
4. Выведет в лог: `INFO database connected version="PostgreSQL 16.x" connections=5 extensions="[vector]"`

При ошибке подключения — выведет предупреждение и продолжит работу без БД.

## Схема базы данных

```sql
-- Кластеризованные персоны
CREATE TABLE persons (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    custom_name TEXT,
    avatar_path TEXT,
    avatar_thumbnail_path TEXT,
    quality_score REAL,
    face_count INTEGER,
    photo_count INTEGER,
    metadata JSONB,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

-- Фотографии
CREATE TABLE photos (
    id UUID PRIMARY KEY,
    path TEXT UNIQUE,
    original_path TEXT,
    width INTEGER, height INTEGER,
    file_size BIGINT,
    mime_type TEXT,
    metadata JSONB,
    uploaded_at TIMESTAMPTZ
);

-- Лица с векторными эмбеддингами
CREATE TABLE faces (
    id UUID PRIMARY KEY,
    person_id UUID REFERENCES persons(id) ON DELETE CASCADE,
    photo_id UUID REFERENCES photos(id) ON DELETE CASCADE,
    embedding vector(512),  -- 512-dim ArcFace embedding (pgvector)
    bbox_x1 REAL, bbox_y1 REAL,
    bbox_x2 REAL, bbox_y2 REAL,
    det_score REAL,
    quality_score REAL,
    thumbnail_path TEXT,
    created_at TIMESTAMPTZ
);

-- Граф связей между персонами
CREATE TABLE person_relations (
    person1_id UUID REFERENCES persons(id) ON DELETE CASCADE,
    person2_id UUID REFERENCES persons(id) ON DELETE CASCADE,
    similarity REAL,
    created_at TIMESTAMPTZ,
    PRIMARY KEY (person1_id, person2_id)
);

-- Сессии обработки
CREATE TABLE processing_sessions (
    id UUID PRIMARY KEY,
    status TEXT,          -- pending, processing, completed, failed
    stage TEXT,
    progress REAL,
    total_items INTEGER,
    processed_items INTEGER,
    errors INTEGER,
    error_details JSONB,
    config JSONB,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_by TEXT
);

-- Настройки приложения
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value JSONB,
    description TEXT,
    updated_at TIMESTAMPTZ
);
```

## Миграции

Миграции применяются автоматически при старте, если `DB_RUN_MIGRATIONS=true`.

Файлы миграций:
```
internal/database/migrations/
├── 001_initial.sql          # Основная схема
├── 002_add_indexes.sql      # Индексы
└── 003_add_fulltext_search.sql  # Full-text search
```

Индексы:
- `faces_person_id` — быстрый поиск лиц по персоне
- `faces_photo_id` — быстрый поиск лиц по фото
- `photos_path` — уникальность пути
- `persons_updated_at` — сортировка по дате
- `idx_faces_embedding` — IVFFlat индекс pgvector (cosine similarity)
- Full-text индекс на имена персон (русский язык)

## Векторный поиск

```sql
-- Поиск похожих лиц по embedding (cosine similarity)
SELECT
    p.name,
    1 - (f.embedding <=> $1::vector) as similarity
FROM faces f
JOIN persons p ON f.person_id = p.id
ORDER BY f.embedding <=> $1::vector
LIMIT 10;

-- Полнотекстовый поиск по именам персон
SELECT * FROM persons
WHERE to_tsvector('russian', COALESCE(custom_name, name))
      @@ plainto_tsquery('russian', 'Иван')
ORDER BY updated_at DESC;
```

## API Endpoints

### Работают без БД (fallback на report.json)

```
GET /api/v1/persons           # Список персон
GET /api/v1/persons/{id}      # Детали персоны
GET /api/v1/persons/{id}/photos  # Фотографии персоны
```

### Требуют БД (возвращают 503 без PostgreSQL)

```
PUT /api/v1/persons/{id}              # Переименование персоны
GET /api/v1/persons/{id}/relations    # Граф связей (с ?min_similarity=0.5)
```

**Переименование персоны:**
```json
PUT /api/v1/persons/{uuid}/
{ "name": "Иван Иванов" }
```

**Граф связей:**
```
GET /api/v1/persons/{uuid}/relations?min_similarity=0.5
Response: { "person_id": "...", "relations": [...], "nodes": [...] }
```

## Repository Pattern

Приложение использует паттерн Repository для работы с БД:

```go
// Получение репозиториев через database.DB
db.Persons   // PersonRepository
db.Photos    // PhotoRepository
db.Faces     // FaceRepository
db.Relations // RelationRepository

// Список персон (с пагинацией)
persons, err := db.Persons.List(ctx, offset, limit)
count, err := db.Persons.Count(ctx)

// По ID
person, err := db.Persons.GetByID(ctx, id)

// Обновление
person.CustomName = "Новое имя"
err = db.Persons.Update(ctx, person)

// Фотографии персоны
count, err := db.Photos.CountByPerson(ctx, personID)
photos, err := db.Photos.List(ctx, offset, limit)

// Граф связей
relations, err := db.Relations.GetByPersonID(ctx, personID, minSimilarity)
nodes, err := db.Relations.GetGraph(ctx, personIDs, minSimilarity)
```

## Производительность

Для больших датасетов (>100k лиц) настройте IVFFlat индекс:

```sql
-- Пересоздайте индекс с большим числом lists
DROP INDEX IF EXISTS idx_faces_embedding;
CREATE INDEX CONCURRENTLY idx_faces_embedding
ON faces USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 200);  -- rows / 1000, не менее 100

-- Обновление статистики
ANALYZE faces;
ANALYZE persons;
```

Connection pool для production:

```bash
DB_MAX_CONNS=50
DB_MIN_CONNS=10
DB_MAX_CONN_LIFETIME=7200
DB_MAX_CONN_IDLE_TIME=600
```

## Production Deployment

### Docker Compose

```yaml
services:
  face-grouper:
    image: ghcr.io/kont1n/face-grouper:latest
    environment:
      - DB_HOST=postgres
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_RUN_MIGRATIONS=true
      - WEB_SERVE=true
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "8080:8080"
    volumes:
      - ./dataset:/app/dataset:ro
      - ./output:/app/output
      - ./models:/app/models:ro

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

### Production переменные окружения

```bash
DB_HOST=prod-db.example.com
DB_PORT=5432
DB_NAME=face-grouper_prod
DB_USER=face-grouper_app
DB_PASSWORD=<strong-password>
DB_SSLMODE=require
DB_MAX_CONNS=50
DB_MIN_CONNS=10
```

## Troubleshooting

### Ошибка подключения

```bash
# Проверка PostgreSQL
docker ps | grep face-grouper-db

# Тест подключения
psql -h localhost -U face-grouper -d face-grouper

# Проверка схемы
psql -h localhost -U face-grouper -d face-grouper -c "\dt"
```

### Сброс БД

```bash
# ОСТОРОЖНО: удаляет все данные!
docker rm -f face-grouper-db
docker run -d --name face-grouper-db \
  -e POSTGRES_DB=face-grouper \
  -e POSTGRES_USER=face-grouper \
  -e POSTGRES_PASSWORD=secret \
  -p 5432:5432 \
  pgvector/pgvector:pg16
```

### Интеграционные тесты

```bash
TEST_DB_HOST=localhost \
TEST_DB_PORT=5432 \
TEST_DB_NAME=face-grouper-test \
TEST_DB_USER=face-grouper \
TEST_DB_PASSWORD=secret \
go test -v -tags=integration ./internal/repository/postgres/...
```
