# Architecture

## Обзор

Face Grouper — монолитное Go-приложение с чёткой слоистой архитектурой. Работает как CLI-инструмент для пакетной обработки и как веб-сервер для интерактивной работы.

```
┌──────────────────────────────────────────────┐
│                   cmd/main.go                │  CLI-флаги, signal handling
├──────────────────────────────────────────────┤
│                  internal/app                │  Оркестрация, DI, Pipeline
├────────────┬─────────────────────────────────┤
│  api/cli   │         api/http                │  CLI API, HTTP handlers + middleware
├────────────┴──────┬──────────────────────────┤
│     service       │          web             │  Бизнес-логика, HTTP-сервер
├───────────────────┼──────────────────────────┤
│   repository      │     infrastructure       │  Хранение данных, ML-инференс
├───────────────────┴──────────────────────────┤
│               platform/pkg                   │  Logger, Closer (утилиты)
└──────────────────────────────────────────────┘
```

## Слои

### 1. Entry Point (`cmd/main.go`)

Точка входа приложения. Парсит CLI-флаги, загружает конфигурацию из `.env`, настраивает обработку сигналов (SIGINT, SIGTERM) и делегирует работу в `internal/app`.

### 2. Application Layer (`internal/app/`)

**app.go** — оркестратор. Два режима работы:
- `runProcess()` — полный пайплайн: scan → extract → cluster → organize → report → (web UI)
- `runViewOnly()` — только веб-интерфейс для просмотра предыдущих результатов

**di.go** — DI-контейнер с lazy initialization. Все зависимости создаются по запросу с защитой через mutex. Ресурсы (детекторы, рекогнайзеры, БД) регистрируются в `closer` для graceful shutdown.

**pipeline.go** — асинхронный runner для веб-режима. Запускает пайплайн в отдельной горутине и стримит прогресс через канал.

### 3. API Layer (`internal/api/`)

**cli/api.go** — фасад для CLI-вызовов. Объединяет сервисы в единый API: `Scan()`, `Extract()`, `Cluster()`, `Organize()`.

**http/handler/** — HTTP-обработчики:
- `upload.go` — загрузка файлов (multipart + zip)
- `session.go` — управление сессиями обработки, SSE-стриминг
- `person.go` — CRUD для персон (с fallback на report.json)
- `errors.go` — ошибки обработки
- `health.go` — health/readiness probes

**http/middleware/** — цепочка middleware:
```
Request → Recovery → RateLimit → MaxBodySize → CORS → Logger → Handler
```

### 4. Service Layer (`internal/service/`)

| Сервис | Описание |
|--------|----------|
| `scan` | Рекурсивный обход директории, фильтрация по расширению |
| `extraction` | Детекция лиц + извлечение эмбеддингов. Параллельная обработка с semaphore, batch inference через `recognizerBatcher` |
| `clustering` | Union-Find с BLAS-ускоренным cosine similarity (блочное умножение матриц через Gonum) |
| `organizer` | Создание директорий персон, symlink/copy фото, выбор лучшего аватара |
| `avatar` | Скоринг качества лица: area × sharpness × frontal_pose |
| `report` | Генерация и загрузка `report.json` |
| `imageutil` | Pure Go обработка изображений (BGR, resize, warp affine, crop) |

### 5. Infrastructure Layer (`internal/infrastructure/`)

**ml/** — ONNX Runtime интеграция:
- `detector.go` — SCRFD детектор лиц (multi-scale FPN, NMS)
- `recognizer.go` — ArcFace извлечение эмбеддингов (sync.Pool для blob recycling)
- `align.go` — выравнивание лица по 5 keypoints (Umeyama similarity transform)
- `ort.go` — инициализация ONNX Runtime (graph optimization, provider selection)
- `provider/` — автодетекция GPU (CUDA, ROCm, CoreML, DirectML)

**database/** — миграции PostgreSQL (embedded SQL-файлы).

### 6. Repository Layer (`internal/repository/`)

**filesystem/** — `ScannerRepository` — сканирование файлов.

**postgres/** — CRUD-репозитории для PostgreSQL:
- `PersonRepository` — персоны (CRUD, поиск, пагинация)
- `FaceRepository` — лица (batch insert, pgvector similarity search)
- `PhotoRepository` — фотографии (batch insert, пагинация по персоне)
- `RelationRepository` — связи персон (граф, ON CONFLICT)
- `SessionRepository` — сессии обработки (статус, ошибки)
- `HealthRepository` — здоровье БД (version, connections, extensions)

### 7. Web Layer (`internal/web/`)

HTTP-сервер со встроенным SPA (`//go:embed index.html`). Маршрутизация через stdlib `http.ServeMux`. Раздача файлов с whitelist-фильтрацией расширений.

### 8. Platform (`platform/pkg/`)

- `logger` — обёртка над zap (structured logging, уровни, JSON/console)
- `closer` — graceful shutdown manager (FIFO cleanup, timeout, named resources)

---

## Пайплайн обработки

```
┌─────────┐    ┌───────────┐    ┌───────────┐    ┌──────────┐    ┌────────┐
│  Scan   │───▶│  Extract  │───▶│  Cluster  │───▶│ Organize │───▶│ Report │
│         │    │           │    │           │    │          │    │        │
│ Файлы   │    │ Лица +    │    │ Кластеры  │    │ Персоны  │    │ JSON   │
│ *.jpg   │    │ эмбеддинги│    │ (groups)  │    │ аватары  │    │ отчёт  │
└─────────┘    └───────────┘    └───────────┘    └──────────┘    └────────┘
```

### Scan
Рекурсивный обход `INPUT_DIR`. Фильтрация по расширениям: `.jpg`, `.jpeg`, `.png`, `.webp`, `.gif`, `.bmp`.

### Extract
Параллельная обработка изображений:

```
Image → [Resize] → Detector → Detections → Align (5kps) → Recognizer → Embeddings
                      │                                         │
                  SCRFD model                              ArcFace model
                  (det_10g.onnx)                          (w600k_r50.onnx)
```

- `EXTRACT_WORKERS` горутин с semaphore
- Пул детекторов (round-robin через канал)
- `recognizerBatcher` — батч-инференс по таймеру (flush) или размеру (batch_size)
- Генерация thumbnails 160x160

### Cluster
BLAS-ускоренная кластеризация:

1. L2-нормализация эмбеддингов (float32 → float64)
2. Блочное умножение матриц (blockSize=512) через Gonum BLAS
3. Порог cosine similarity → Union-Find (path compression + union by rank)
4. Результат: массив кластеров с индексами лиц

### Organize
Для каждого кластера:
1. Создание директории `Person_N`
2. Symlink/copy фотографий в директорию
3. Выбор лучшего аватара (quality score = area × sharpness × frontal_pose)
4. Сохранение thumbnail

### Report
Генерация `report.json` с метаданными обработки, статистикой и списком персон.

---

## Concurrency Model

```
main goroutine
  │
  ├── Extract workers (N = EXTRACT_WORKERS)
  │     ├── goroutine 1: image → detect → align → batcher
  │     ├── goroutine 2: image → detect → align → batcher
  │     └── goroutine N: ...
  │
  ├── Recognizer batcher workers (N = EXTRACT_WORKERS)
  │     ├── worker 1: collect batch → infer → resolve
  │     ├── worker 2: collect batch → infer → resolve
  │     └── worker N: ...
  │
  ├── Web server (если --serve)
  │     ├── SSE stream goroutines
  │     ├── Rate limiter cleanup goroutine
  │     └── Session cleanup goroutine
  │
  └── Pipeline (web mode)
        └── goroutine: scan → extract → cluster → organize
```

**Синхронизация:**
- `sync.Mutex` — DI container, session state
- `sync.RWMutex` — rate limiter, session reads
- `errgroup.WithContext` — координация extraction workers
- Buffered channels — пулы детекторов/рекогнайзеров, batcher items
- `sync.Pool` — blob recycling в recognizer
- `atomic.Bool` — batcher closed flag

---

## Data Flow (Web mode)

```
Browser                    Server                     Pipeline
  │                          │                           │
  ├─POST /upload────────────▶│                           │
  │◀─── {session_id} ───────┤                           │
  │                          │                           │
  ├─POST /sessions/ID/process▶│                          │
  │◀─── 202 Accepted ───────┤──── RunPipeline() ───────▶│
  │                          │                           │
  ├─GET /sessions/ID/stream──▶│     ┌────────────────────┤
  │◀─── SSE: progress ──────┤◀────┤ progress channel    │
  │◀─── SSE: progress ──────┤◀────┤                     │
  │◀─── SSE: done ──────────┤◀────┤                     │
  │                          │     └────────────────────┘│
  ├─GET /persons─────────────▶│                           │
  │◀─── [{persons}] ────────┤                           │
  │                          │                           │
  ├─GET /persons/ID/relations▶│                           │
  │◀─── {graph} ────────────┤                           │
```

---

## Безопасность

| Мера | Реализация |
|------|------------|
| Path traversal | `filepath.Clean()` + prefix check + symlink check |
| Zip bomb | Лимит 2GB на распаковку + LimitReader |
| Zip slip | `filepath.Base()` для имён файлов из архива |
| Magic bytes | Валидация заголовков файлов |
| SQL injection | Параметризованные запросы (`$1`, `$2`) |
| XSS | `html.EscapeString()` для пользовательского ввода |
| Rate limiting | Token bucket per IP (100 RPS, 200 burst) |
| CORS | Same-origin по умолчанию |
| Directory listing | Запрещён |
| Body size | 500 MB лимит |

---

## Graceful Shutdown

```
SIGINT/SIGTERM
  │
  ├── Cancel pipeline context
  ├── Stop web server (10s timeout)
  ├── Close rate limiter cleanup
  ├── Close ONNX sessions (detectors, recognizers)
  ├── Close database pool
  ├── Destroy ONNX Runtime
  └── Sync logger (5s timeout)
```

Все ресурсы регистрируются в `closer` с именами для отладки. Cleanup в порядке FIFO. Общий таймаут: 30 секунд.

---

## Зависимости

| Пакет | Назначение |
|-------|------------|
| `jackc/pgx/v5` | PostgreSQL driver + connection pool |
| `pgvector/pgvector-go` | pgvector типы для Go |
| `yalue/onnxruntime_go` | ONNX Runtime bindings (CGO) |
| `go.uber.org/zap` | Structured logging |
| `gonum.org/v1/gonum` | BLAS matrix operations |
| `google/uuid` | UUID generation |
| `joho/godotenv` | .env file parsing |
| `golang.org/x/sync` | errgroup, semaphore |
| `golang.org/x/time` | Rate limiting |
