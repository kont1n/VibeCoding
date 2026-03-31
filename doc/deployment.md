# Deployment

## Docker

Проект предоставляет три варианта Docker-образа для разных платформ.

### CPU

```bash
docker build -t face-grouper:cpu -f deploy/docker/Dockerfile.cpu .
```

- Базовый образ: `debian:bookworm-slim`
- ONNX Runtime: CPU (auto-detect x64/aarch64)
- Размер образа: ~200 MB

### NVIDIA GPU

```bash
docker build -t face-grouper:gpu -f deploy/docker/Dockerfile.nvidia .
```

- Базовый образ: `nvidia/cuda:12.3.2-cudnn9-devel-ubuntu22.04`
- ONNX Runtime: CUDA GPU
- Требования:
  - NVIDIA GPU с Compute Capability 5.0+
  - NVIDIA Driver 450.80.02+
  - [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)

```bash
# Запуск с GPU
docker run -d --gpus all --name face-grouper \
  -v $(pwd)/dataset:/app/dataset \
  -v $(pwd)/output:/app/output \
  -v $(pwd)/models:/app/models \
  -p 8080:8080 \
  face-grouper:gpu
```

### AMD ROCm

```bash
docker build -t face-grouper:rocm -f deploy/docker/Dockerfile.rocm .
```

- Базовый образ: `rocm/dev-ubuntu-22.04:latest`
- ONNX Runtime: ROCm
- Требования:
  - AMD GPU (RX 5000+, RX 6000+, RX 7000+, MI50+)
  - ROCm 5.x+

```bash
# Запуск с AMD GPU
docker run -d --name face-grouper \
  --device=/dev/kfd --device=/dev/dri \
  --group-add video --group-add render \
  -v $(pwd)/dataset:/app/dataset \
  -v $(pwd)/output:/app/output \
  -v $(pwd)/models:/app/models \
  -p 8080:8080 \
  face-grouper:rocm
```

### Build args

| Аргумент | По умолчанию | Описание |
|----------|-------------|----------|
| `ONNXRUNTIME_VERSION` | `1.23.0` | Версия ONNX Runtime |

---

## Docker Compose

Файл: [`deploy/compose/docker-compose.yml`](../deploy/compose/docker-compose.yml)

### Сервисы

| Сервис | Образ | Порт | Описание |
|--------|-------|------|----------|
| `postgres` | `pgvector/pgvector:pg16` | 5432 | PostgreSQL с pgvector |
| `redis` | `redis:7-alpine` | 6379 | Redis (опционально) |
| `face-grouper-cpu` | `face-grouper:cpu` | 8080 | CPU-версия |
| `face-grouper-gpu` | `face-grouper:gpu` | 8081 | NVIDIA GPU-версия |
| `face-grouper-rocm` | `face-grouper:rocm` | 8082 | AMD ROCm-версия |

### Запуск

```bash
cd deploy/compose

# CPU-версия с PostgreSQL
docker compose up -d postgres face-grouper-cpu

# NVIDIA GPU-версия
docker compose up -d postgres face-grouper-gpu

# AMD ROCm-версия
docker compose up -d postgres face-grouper-rocm

# Все сервисы
docker compose up -d
```

### Ресурсы

CPU-версия:
- Limits: 4 CPU, 4 GB RAM
- Reservations: 2 CPU, 2 GB RAM

### Volumes

| Volume | Путь | Описание |
|--------|------|----------|
| `postgres_data` | `/var/lib/postgresql/data` | Данные PostgreSQL |
| `redis_data` | `/data` | Данные Redis |

### Сети

Все сервисы объединены в bridge-сеть `face-grouper-network`.

Порты БД и Redis привязаны к `127.0.0.1` (недоступны извне).

### Переменные окружения

Docker Compose ожидает файл с переменными. Создайте его из шаблона:

```bash
cp ../env/.env.example .env
# Отредактируйте .env
```

---

## PostgreSQL

### Требования

- PostgreSQL 14+
- Расширение [pgvector](https://github.com/pgvector/pgvector)

### Установка pgvector

```bash
# Ubuntu/Debian
sudo apt install postgresql-16-pgvector

# Docker (уже включено)
# Образ pgvector/pgvector:pg16 содержит расширение
```

### Миграции

Миграции запускаются автоматически при старте приложения (`DB_RUN_MIGRATIONS=true`).

Файлы миграций: `internal/infrastructure/database/migrations/`

| Файл | Описание |
|------|----------|
| `001_initial.sql` | Таблицы: persons, faces, photos, person_relations, processing_sessions |
| `002_add_indexes.sql` | Индексы для производительности |
| `003_add_fulltext_search.sql` | Полнотекстовый поиск по персонам |
| `004_enhanced_vector_indexes.sql` | IVFFlat индексы для pgvector |
| `005_remove_duplicate_ivfflat_index.sql` | Очистка дублирующихся индексов |

### Схема БД

```
persons                    faces                      photos
├── id (UUID PK)           ├── id (UUID PK)           ├── id (UUID PK)
├── name                   ├── person_id (FK)         ├── path
├── custom_name            ├── photo_id (FK)          ├── original_path
├── avatar_path            ├── embedding (vector)     ├── width, height
├── quality_score          ├── bbox (x1,y1,x2,y2)    ├── file_size
├── face_count             ├── keypoints [5][2]       ├── mime_type
├── photo_count            ├── det_score              └── uploaded_at
└── created_at/updated_at  ├── quality_score
                           └── thumbnail_path
person_relations
├── person1_id (FK)        processing_sessions
├── person2_id (FK)        ├── id (UUID PK)
├── similarity             ├── status, stage, progress
└── created_at             ├── total_items, processed_items
                           ├── errors, error_details
                           └── started_at, completed_at
```

---

## Health Checks

Все Docker-контейнеры имеют настроенные health checks.

### Endpoints

| Endpoint | Описание | Использование |
|----------|----------|---------------|
| `GET /health` | Liveness probe | Docker HEALTHCHECK, Kubernetes livenessProbe |
| `GET /ready` | Readiness probe | Kubernetes readinessProbe |

### Kubernetes пример

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

---

## Логирование

| Уровень | Переменная | Описание |
|---------|-----------|----------|
| `debug` | `LOG_LEVEL=debug` | Подробная отладочная информация |
| `info` | `LOG_LEVEL=info` | Штатные события (default) |
| `warn` | `LOG_LEVEL=warn` | Предупреждения |
| `error` | `LOG_LEVEL=error` | Только ошибки |

Для production рекомендуется `LOG_JSON=true` для structured logging.

Docker Compose настроен с ротацией логов:
```yaml
logging:
  driver: "json-file"
  options:
    max-size: "50m"
    max-file: "5"
```

---

## Модели

Для работы необходимы две ONNX-модели в директории `MODELS_DIR`:

| Файл | Модель | Описание |
|------|--------|----------|
| `det_10g.onnx` | SCRFD-10GF | Детекция лиц (5 keypoints) |
| `w600k_r50.onnx` | ArcFace-R50 | Извлечение 512-d эмбеддингов |

Модели можно скачать из репозитория [InsightFace](https://github.com/deepinsight/insightface/tree/master/model_zoo).

### Производительность моделей

| Режим | Скорость (фото/сек) | Примечание |
|-------|---------------------|------------|
| CPU (4 workers) | ~5-15 | Зависит от CPU и размера изображений |
| NVIDIA GPU | ~50-200 | Зависит от GPU и batch size |
| AMD ROCm | ~30-100 | Зависит от GPU |

> Значения приблизительные. Реальная производительность зависит от количества лиц на фото и параметров `MAX_DIM`, `EMBED_BATCH_SIZE`, `EXTRACT_WORKERS`.
