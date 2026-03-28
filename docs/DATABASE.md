# Face Grouper - Database Integration

## PostgreSQL + pgvector Integration

Face Grouper теперь поддерживает PostgreSQL с расширением pgvector для персистентного хранения данных о лицах, персонах и сессиях обработки.

### Возможности

- ✅ **Персистентное хранение** - все результаты сохраняются в БД
- ✅ **Векторный поиск** - поиск похожих лиц через cosine similarity
- ✅ **Full-text search** - поиск персон по имени (русский язык)
- ✅ **Graph relations** - хранение связей между персонами
- ✅ **Session tracking** - отслеживание прогресса обработки
- ✅ **Health checks** - проверка состояния БД при старте
- ✅ **Auto-migrations** - автоматическое применение миграций

### Быстрый старт

#### 1. Запуск PostgreSQL с pgvector

```bash
# Docker
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

#### 2. Настройка конфигурации

Создайте или обновите `.env`:

```bash
# Database Configuration
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

# Migrations
DB_RUN_MIGRATIONS=true

# Redis (optional cache)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

#### 3. Запуск приложения

```bash
# Сборка
go build -o face-grouper.exe ./cmd

# Запуск
./face-grouper.exe

# С веб-интерфейсом
./face-grouper.exe --serve
```

Приложение автоматически:
1. Подключится к PostgreSQL
2. Применит миграции
3. Создаст таблицы и индексы
4. Начнёт обработку с сохранением в БД

### Схема базы данных

```sql
-- Persons: кластеризованные персоны
CREATE TABLE persons (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    custom_name TEXT,
    avatar_path TEXT,
    quality_score REAL,
    face_count INTEGER,
    photo_count INTEGER,
    metadata JSONB,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

-- Faces: лица с векторными эмбеддингами
CREATE TABLE faces (
    id UUID PRIMARY KEY,
    person_id UUID REFERENCES persons(id),
    photo_id UUID REFERENCES photos(id),
    embedding vector(512),  -- 512-dim ArcFace embedding
    bbox_x1 REAL, bbox_y1 REAL,
    bbox_x2 REAL, bbox_y2 REAL,
    det_score REAL,
    quality_score REAL,
    created_at TIMESTAMPTZ
);

-- Photos: метаданные фотографий
CREATE TABLE photos (
    id UUID PRIMARY KEY,
    path TEXT UNIQUE,
    width INTEGER, height INTEGER,
    file_size BIGINT,
    mime_type TEXT,
    metadata JSONB
);

-- Relations: граф связей
CREATE TABLE person_relations (
    person1_id UUID,
    person2_id UUID,
    similarity REAL,
    PRIMARY KEY (person1_id, person2_id)
);

-- Sessions: трекинг обработки
CREATE TABLE processing_sessions (
    id UUID PRIMARY KEY,
    status TEXT,
    progress REAL,
    total_items INTEGER,
    errors INTEGER,
    created_at TIMESTAMPTZ
);
```

### Векторный поиск

```sql
-- Поиск похожих лиц
SELECT 
    p.name,
    1 - (f.embedding <=> query_embedding) as similarity
FROM faces f
JOIN persons p ON f.person_id = p.id
ORDER BY f.embedding <=> query_embedding
LIMIT 10;

-- Полнотекстовый поиск по именам
SELECT * FROM persons
WHERE to_tsvector('russian', COALESCE(custom_name, name)) 
      @@ plainto_tsquery('russian', 'Иван')
ORDER BY ts_rank(...) DESC;
```

### Repository Pattern

Приложение использует паттерн Repository для работы с БД:

```go
// Получение репозиториев из DI контейнера
personRepo := diContainer.PersonRepository()
faceRepo := diContainer.FaceRepository()
photoRepo := diContainer.PhotoRepository()

// Создание персоны
person := &model.Person{
    ID: uuid.New(),
    Name: "John Doe",
    FaceCount: 5,
    PhotoCount: 3,
}
err := personRepo.Create(ctx, person)

// Поиск похожих лиц
similarFaces, err := personRepo.FindSimilarFaces(ctx, embedding, 10)

// Полнотекстовый поиск
persons, err := personRepo.Search(ctx, "Иван", 10)
```

### Миграции

Миграции применяются автоматически при старте. Файлы миграций:
- `internal/database/migrations/001_initial.sql` - основная схема
- `internal/database/migrations/002_add_indexes.sql` - индексы
- `internal/database/migrations/003_add_fulltext_search.sql` - full-text search

Ручное управление миграциями:

```go
migrator := database.NewMigrator(pool)
err := migrator.Migrate(ctx)

// Получить версию
version, dirty, err := migrator.GetVersion(ctx)

// Откатить последнюю миграцию
err = migrator.Rollback(ctx)
```

### Health Check

При старте приложение проверяет подключение к БД:

```
INFO database connected version="PostgreSQL 16.x" 
                     connections=5 
                     extensions="[vector]"
```

### Тестирование

Интеграционные тесты требуют запущенного PostgreSQL:

```bash
# Запуск тестов
go test -v -tags=integration ./internal/repository/postgres/...

# С переменными окружения
TEST_DB_HOST=localhost \
TEST_DB_PORT=5432 \
TEST_DB_NAME=face-grouper-test \
TEST_DB_USER=face-grouper \
TEST_DB_PASSWORD=secret \
go test -v -tags=integration ./internal/repository/postgres/...
```

### Production Deployment

#### Docker Compose

```yaml
version: '3.8'

services:
  face-grouper:
    image: ghcr.io/kont1n/face-grouper:latest
    environment:
      - DB_HOST=postgres
      - DB_PASSWORD=${DB_PASSWORD}
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "8080:8080"

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

#### Переменные окружения для production

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

### Troubleshooting

#### Ошибки подключения

```bash
# Проверка PostgreSQL
docker ps | grep face-grouper-db

# Тест подключения
psql -h localhost -U face-grouper -d face-grouper
```

#### Ошибки миграций

```bash
# Проверка схемы
psql -h localhost -U face-grouper -d face-grouper -c "\dt"

# Сброс БД (ОСТОРОЖНО: удаляет все данные!)
docker rm -f face-grouper-db
docker run ... (см. выше)
```

#### Производительность векторного поиска

```sql
-- Увеличить lists для больших датасетов
DROP INDEX IF EXISTS idx_faces_embedding;
CREATE INDEX CONCURRENTLY idx_faces_embedding 
ON faces USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 200);  -- rows / 1000

-- Анализ таблиц
ANALYZE faces;
ANALYZE persons;
```

### API Endpoints

Приложение предоставляет REST API для работы с БД:

```
GET  /api/v1/persons          # Список персон
GET  /api/v1/persons/{id}     # Детали персоны
PUT  /api/v1/persons/{id}     # Обновление (переименование)
GET  /api/v1/persons/{id}/download  # Скачать архив

GET  /api/v1/graph            # Граф связей
POST /api/v1/search           # Поиск похожих лиц

GET  /api/v1/sessions         # Сессии обработки
GET  /api/v1/sessions/{id}    # Детали сессии
```

### Поддержка

- Документация: `docs/DATABASE.md`
- Issues: https://github.com/kont1n/face-grouper/issues
- Discussions: https://github.com/kont1n/face-grouper/discussions
