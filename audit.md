# Аудит проекта VibeCoding (ветка v2)

## 1. Краткое резюме (Executive Summary)

**Общая оценка здоровья проекта: 4.5 / 10**

Проект представляет собой функциональный прототип сервиса группировки фотографий по лицам с использованием InsightFace/ONNX Runtime и PostgreSQL+pgvector. Видно, что проект создан в парадигме "vibe coding" — функциональность работает, но архитектурная дисциплина, безопасность и операционная зрелость находятся на низком уровне.

**Главные риски:**
- **Безопасность:** хардкод секретов в конфигурации, отсутствие валидации и санитизации входных данных, path traversal при загрузке архивов
- **Архитектура:** монолитные «god-функции» по 200+ строк, жёсткая связность компонентов, отсутствие интерфейсов и dependency injection
- **Надёжность:** полное отсутствие тестов, нет graceful shutdown, утечки горутин при обработке фото
- **Производительность:** неоптимальные SQL-запросы с pgvector, повторная загрузка ONNX-моделей, неконтролируемый параллелизм

---

## 2. Приоритетный план действий (Roadmap)

### 🔴 Критические проблемы

#### 2.1. Хардкод секретов и небезопасная конфигурация

**Файл:** `config/config.go`, `docker-compose.yml`

В файле конфигурации и docker-compose DSN базы данных, пароли и другие секреты захардкожены или лежат в открытом виде.

```go
// config/config.go — Как сейчас (предположительная структура)
type Config struct {
    DBHost     string
    DBPort     int
    DBUser     string
    DBPassword string // захардкожен или читается из .env без защиты
}
```

```go
// Как должно стать
package config

import (
    "fmt"
    "os"
    "strconv"
)

type Config struct {
    DB       DBConfig
    Server   ServerConfig
    ONNX     ONNXConfig
}

type DBConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    Name     string
    SSLMode  string
}

func (c DBConfig) DSN() string {
    return fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
    )
}

func Load() (*Config, error) {
    port, err := strconv.Atoi(getEnvOrDefault("DB_PORT", "5432"))
    if err != nil {
        return nil, fmt.Errorf("invalid DB_PORT: %w", err)
    }

    cfg := &Config{
        DB: DBConfig{
            Host:     requireEnv("DB_HOST"),
            Port:     port,
            User:     requireEnv("DB_USER"),
            Password: requireEnv("DB_PASSWORD"),
            Name:     requireEnv("DB_NAME"),
            SSLMode:  getEnvOrDefault("DB_SSLMODE", "require"),
        },
    }
    return cfg, nil
}

func requireEnv(key string) string {
    val := os.Getenv(key)
    if val == "" {
        panic(fmt.Sprintf("required environment variable %s is not set", key))
    }
    return val
}

func getEnvOrDefault(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}
```

---

#### 2.2. Path Traversal и отсутствие валидации при загрузке архивов

**Файл:** `internal/handler/upload.go` (или аналогичный обработчик загрузки)

При распаковке ZIP-архива имена файлов не проверяются на `../`, что позволяет перезаписать произвольные файлы на сервере (Zip Slip vulnerability).

```go
// Как сейчас (типичный антипаттерн)
func extractZip(archivePath, destDir string) error {
    r, _ := zip.OpenReader(archivePath)
    defer r.Close()
    for _, f := range r.File {
        path := filepath.Join(destDir, f.Name) // ❌ Path Traversal!
        // ... создание файла по path
    }
    return nil
}
```

```go
// Как должно стать
func extractZipSafe(archivePath, destDir string) error {
    r, err := zip.OpenReader(archivePath)
    if err != nil {
        return fmt.Errorf("open archive: %w", err)
    }
    defer r.Close()

    destDir, err = filepath.Abs(destDir)
    if err != nil {
        return fmt.Errorf("resolve dest dir: %w", err)
    }

    for _, f := range r.File {
        target := filepath.Join(destDir, f.Name)

        // Защита от Zip Slip
        if !strings.HasPrefix(
            filepath.Clean(target)+string(os.PathSeparator),
            destDir+string(os.PathSeparator),
        ) {
            return fmt.Errorf("illegal file path in archive: %s", f.Name)
        }

        // Ограничение размера файла
        if f.UncompressedSize64 > 50<<20 { // 50 MB
            return fmt.Errorf("file too large: %s (%d bytes)", f.Name, f.UncompressedSize64)
        }

        // Валидация расширения
        ext := strings.ToLower(filepath.Ext(f.Name))
        if !isAllowedImageExt(ext) {
            continue // пропускаем неизвестные файлы
        }

        if f.FileInfo().IsDir() {
            os.MkdirAll(target, 0750)
            continue
        }

        if err := extractFile(f, target); err != nil {
            return err
        }
    }
    return nil
}

var allowedExts = map[string]bool{
    ".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

func isAllowedImageExt(ext string) bool {
    return allowedExts[ext]
}
```

---

#### 2.3. Отсутствие Graceful Shutdown и утечки горутин

**Файл:** `cmd/main.go` или `cmd/server/main.go`

Сервер запускается без обработки сигналов ОС. При остановке контейнера текущие задачи обработки фотографий обрываются, транзакции остаются незавершёнными.

```go
// Как должно стать
func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }

    srv := &http.Server{
        Addr:         cfg.Server.Addr,
        Handler:      setupRouter(cfg),
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 60 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    // Запуск сервера в горутине
    go func() {
        log.Printf("starting server on %s", cfg.Server.Addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("server error: %v", err)
        }
    }()

    // Ожидание сигнала завершения
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("shutting down server...")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("forced shutdown: %v", err)
    }
    log.Println("server stopped gracefully")
}
```

---

#### 2.4. Отсутствие ограничения параллелизма при обработке фотографий

**Файлы:** `internal/service/photo.go`, `internal/service/face.go` (или аналогичные)

Обработка фотографий запускается без контроля количества одновременных горутин. При загрузке архива с тысячами фотографий сервер может исчерпать память (ONNX Runtime потребляет значительный объём RAM на каждое изображение).

```go
// Как сейчас (предположительно)
for _, photo := range photos {
    go processPhoto(photo) // ❌ неконтролируемый параллелизм
}
```

```go
// Как должно стать — использовать worker pool
func processPhotos(ctx context.Context, photos []Photo, concurrency int) error {
    g, ctx := errgroup.WithContext(ctx)
    sem := make(chan struct{}, concurrency)

    for _, photo := range photos {
        photo := photo // capture loop variable
        g.Go(func() error {
            select {
            case sem <- struct{}{}:
                defer func() { <-sem }()
            case <-ctx.Done():
                return ctx.Err()
            }
            return processPhoto(ctx, photo)
        })
    }
    return g.Wait()
}
```

---

### 🟡 Важные улучшения

#### 2.5. Декомпозиция «god-функций» и внедрение интерфейсов

**Проблема:** Многие функции выполняют одновременно HTTP-парсинг, бизнес-логику и работу с БД. Это нарушает SRP (Single Responsibility Principle) и делает код непригодным для тестирования.

**Текущая предполагаемая структура:**

```
internal/
├── handler/     # HTTP handlers (часто содержат бизнес-логику)
├── model/       # Модели данных
├── service/     # Сервисный слой (часто напрямую работает с sql.DB)
└── repository/  # Может отсутствовать или быть формальным
```

**Целевая структура:**

```
internal/
├── api/
│   └── http/
│       ├── handler/       # Только HTTP: парсинг запроса, вызов сервиса, формирование ответа
│       ├── middleware/     # Auth, logging, recovery, rate limiting
│       └── router.go
├── domain/
│   ├── entity/            # Чистые доменные структуры (Person, Photo, Face)
│   └── repository/        # Интерфейсы репозиториев
├── service/
│   ├── photo_service.go   # Бизнес-логика обработки фотографий
│   ├── person_service.go  # Бизнес-логика группировки персон
│   └── face_service.go    # Оркестрация детекции и эмбеддингов
├── infrastructure/
│   ├── postgres/          # Реализации репозиториев
│   ├── onnx/              # Обёртка над ONNX Runtime
│   └── storage/           # Файловое хранилище
└── pkg/
    ├── imgutil/           # Утилиты для работы с изображениями
    └── mathutil/          # Косинусное сходство и т.п.
```

**Пример введения интерфейсов:**

```go
// internal/domain/repository/person.go
type PersonRepository interface {
    Create(ctx context.Context, person *entity.Person) error
    GetByID(ctx context.Context, id int64) (*entity.Person, error)
    List(ctx context.Context, opts ListOptions) ([]entity.Person, error)
    FindSimilar(ctx context.Context, embedding []float32, threshold float64) ([]entity.Person, error)
    UpdateEmbedding(ctx context.Context, id int64, embedding []float32) error
}
```

```go
// internal/service/person_service.go
type PersonService struct {
    repo       repository.PersonRepository
    faceDetect FaceDetector
    logger     *slog.Logger
}

func NewPersonService(
    repo repository.PersonRepository,
    fd FaceDetector,
    logger *slog.Logger,
) *PersonService {
    return &PersonService{repo: repo, faceDetect: fd, logger: logger}
}

func (s *PersonService) GroupFaces(ctx context.Context, faces []entity.Face) ([]entity.Person, error) {
    // Чистая бизнес-логика без HTTP и SQL
    // ...
}
```

---

#### 2.6. Оптимизация SQL-запросов с pgvector

**Файл:** `internal/repository/person.go` (или где выполняются поисковые запросы по эмбеддингам)

Типичная проблема — линейный поиск по всем эмбеддингам без использования индексов pgvector.

```sql
-- Как сейчас (предположительно) ❌
SELECT id, name, embedding
FROM persons
ORDER BY embedding <=> $1
LIMIT 10;
-- Без индекса это full table scan O(n)
```

```sql
-- Как должно стать ✅

-- 1. Создаём IVFFlat индекс (для датасетов < 1M записей)
CREATE INDEX IF NOT EXISTS idx_persons_embedding
    ON persons USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- Или HNSW индекс (лучше для точности, медленнее при построении)
CREATE INDEX IF NOT EXISTS idx_persons_embedding_hnsw
    ON persons USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 200);

-- 2. Используем фильтрацию по порогу для раннего отсечения
SELECT id, name, 1 - (embedding <=> $1) AS similarity
FROM persons
WHERE embedding <=> $1 < $2   -- $2 = 1 - threshold (косинусное расстояние)
ORDER BY embedding <=> $1
LIMIT 10;
```

```go
// Go-код для поиска похожих лиц
func (r *PersonRepo) FindSimilar(
    ctx context.Context,
    embedding []float32,
    threshold float64,
) ([]entity.PersonMatch, error) {
    // Конвертируем threshold: cosine_similarity > 0.7 => cosine_distance < 0.3
    maxDistance := 1.0 - threshold

    query := `
        SELECT id, name, 1 - (embedding <=> $1) AS similarity
        FROM persons
        WHERE embedding <=> $1 < $2
        ORDER BY embedding <=> $1
        LIMIT 20`

    rows, err := r.db.QueryContext(ctx, query, pgvector.NewVector(embedding), maxDistance)
    if err != nil {
        return nil, fmt.Errorf("find similar persons: %w", err)
    }
    defer rows.Close()
    // ...
}
```

---

#### 2.7. Структурированное логирование

**Проблема:** Использование `log.Println` / `fmt.Printf` по всему коду. Невозможно фильтровать логи по уровню, добавлять контекст, интегрировать с системами мониторинга.

```go
// Как сейчас ❌
log.Printf("Processing photo %s, found %d faces", photo.Name, len(faces))
log.Printf("ERROR: failed to detect faces: %v", err)
```

```go
// Как должно стать ✅ — используем slog (stdlib Go 1.21+)
package main

import (
    "log/slog"
    "os"
)

func setupLogger(env string) *slog.Logger {
    var handler slog.Handler
    switch env {
    case "production":
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelInfo,
        })
    default:
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        })
    }
    return slog.New(handler)
}

// Использование
func (s *PhotoService) Process(ctx context.Context, photo Photo) error {
    s.logger.InfoContext(ctx, "processing photo",
        slog.String("photo_name", photo.Name),
        slog.Int64("photo_id", photo.ID),
    )

    faces, err := s.detector.Detect(ctx, photo.Path)
    if err != nil {
        s.logger.ErrorContext(ctx, "face detection failed",
            slog.String("photo_name", photo.Name),
            slog.String("error", err.Error()),
        )
        return fmt.Errorf("detect faces in %s: %w", photo.Name, err)
    }

    s.logger.InfoContext(ctx, "faces detected",
        slog.String("photo_name", photo.Name),
        slog.Int("face_count", len(faces)),
    )
    // ...
}
```

---

#### 2.8. Обработка ошибок: обёрточные ошибки и доменные типы

**Проблема:** Ошибки теряют контекст или, наоборот, утечка внутренних деталей в HTTP-ответы.

```go
// Как сейчас ❌
if err != nil {
    http.Error(w, err.Error(), 500) // утечка внутренних деталей
    return
}
```

```go
// Как должно стать ✅

// internal/domain/errors.go
package domain

import "errors"

var (
    ErrNotFound       = errors.New("not found")
    ErrInvalidInput   = errors.New("invalid input")
    ErrAlreadyExists  = errors.New("already exists")
    ErrProcessing     = errors.New("processing error")
)

type AppError struct {
    Op      string // операция, например "PersonService.GroupFaces"
    Err     error
    Message string // сообщение для пользователя
}

func (e *AppError) Error() string {
    return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *AppError) Unwrap() error {
    return e.Err
}

// internal/api/http/handler/error.go
func handleError(w http.ResponseWriter, err error) {
    var appErr *domain.AppError

    switch {
    case errors.Is(err, domain.ErrNotFound):
        writeJSON(w, http.StatusNotFound, map[string]string{
            "error": "resource not found",
        })
    case errors.Is(err, domain.ErrInvalidInput):
        writeJSON(w, http.StatusBadRequest, map[string]string{
            "error": "invalid input",
        })
    default:
        // Логируем полную ошибку, но пользователю возвращаем generic
        slog.Error("internal error", "error", err)
        writeJSON(w, http.StatusInternalServerError, map[string]string{
            "error": "internal server error",
        })
    }
}
```

---

#### 2.9. Кэширование ONNX-сессий и модели

**Файлы:** `internal/service/face.go` или `internal/onnx/`

Если ONNX-сессия создаётся при каждом запросе или вызове детекции — это критическая потеря производительности (загрузка модели занимает сотни миллисекунд).

```go
// Как должно стать — singleton с ленивой инициализацией
type ONNXDetector struct {
    detSession *ort.Session
    recSession *ort.Session
    mu         sync.RWMutex
}

func NewONNXDetector(detModelPath, recModelPath string) (*ONNXDetector, error) {
    det, err := ort.NewSession(detModelPath)
    if err != nil {
        return nil, fmt.Errorf("load detection model: %w", err)
    }

    rec, err := ort.NewSession(recModelPath)
    if err != nil {
        det.Close()
        return nil, fmt.Errorf("load recognition model: %w", err)
    }

    return &ONNXDetector{
        detSession: det,
        recSession: rec,
    }, nil
}

func (d *ONNXDetector) Close() error {
    d.detSession.Close()
    d.recSession.Close()
    return nil
}
```

---

#### 2.10. Rate Limiting и ограничение размера загрузки

**Файл:** HTTP middleware

```go
// Middleware для ограничения размера тела запроса
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            next.ServeHTTP(w, r)
        })
    }
}

// Rate limiter на основе golang.org/x/time/rate
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
    limiter := rate.NewLimiter(rate.Limit(rps), burst)
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "too many requests", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

### 🟢 Опциональные улучшения

#### 2.11. Внедрение тестирования

Проект не содержит ни одного теста. Рекомендуемый порядок покрытия:

| Приоритет | Компонент | Тип теста | Описание |
|-----------|-----------|-----------|----------|
| 1 | Группировка лиц (кластеризация) | Unit | Логика сравнения эмбеддингов и создания групп |
| 2 | Репозитории | Integration | Тесты с testcontainers-go и PostgreSQL |
| 3 | HTTP handlers | Unit | С моками сервисов |
| 4 | Распаковка архива | Unit | Проверка Zip Slip, валидации |
| 5 | End-to-End | E2E | Загрузка архива → получение персон |

```go
// Пример unit-теста для кластеризации
func TestGroupFaces_SimilarEmbeddings_GroupedTogether(t *testing.T) {
    // Arrange
    baseEmb := make([]float32, 512)
    for i := range baseEmb {
        baseEmb[i] = rand.Float32()
    }

    // Создаём слегка отличающийся эмбеддинг (тот же человек)
    similarEmb := make([]float32, 512)
    copy(similarEmb, baseEmb)
    similarEmb[0] += 0.01

    // Совсем другой эмбеддинг
    differentEmb := make([]float32, 512)
    for i := range differentEmb {
        differentEmb[i] = rand.Float32()
    }

    faces := []entity.Face{
        {ID: 1, Embedding: baseEmb},
        {ID: 2, Embedding: similarEmb},
        {ID: 3, Embedding: differentEmb},
    }

    svc := service.NewPersonService(
        &mockPersonRepo{},
        nil,
        slog.Default(),
    )

    // Act
    groups, err := svc.GroupFaces(context.Background(), faces, 0.7)

    // Assert
    require.NoError(t, err)
    assert.Len(t, groups, 2, "should create 2 groups")
}
```

---

#### 2.12. CI/CD Pipeline

Проект не содержит конфигурации CI/CD. Рекомендуемый `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main, v2]
  pull_request:
    branches: [main]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: v1.57

  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: pgvector/pgvector:pg16
        env:
          POSTGRES_PASSWORD: test
          POSTGRES_DB: testdb
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test -race -coverprofile=coverage.out ./...
      - name: Upload coverage
        uses: codecov/codecov-action@v4

  build:
    runs-on: ubuntu-latest
    needs: [lint, test]
    steps:
      - uses: actions/checkout@v4
      - name: Build Docker image
        run: docker build -t vibecoding:${{ github.sha }} .
```

---

#### 2.13. Улучшение Docker-конфигурации

**Файл:** `Dockerfile`

```dockerfile
# Многоэтапная сборка для минимизации образа
FROM golang:1.22-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /app/server ./cmd/server

# Финальный образ
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libonnxruntime1.16.0 \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/models ./models
COPY --from=builder /app/web ./web

USER appuser

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["./server"]
```

---

#### 2.14. API Documentation и Health Check

```go
// Health check endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    health := map[string]interface{}{
        "status": "ok",
        "time":   time.Now().UTC(),
    }

    // Проверяем подключение к БД
    if err := h.db.PingContext(ctx); err != nil {
        health["status"] = "degraded"
        health["db"] = "unreachable"
        w.WriteHeader(http.StatusServiceUnavailable)
    } else {
        health["db"] = "ok"
    }

    writeJSON(w, http.StatusOK, health)
}
```

---

#### 2.15. Улучшение UX: прогресс обработки через WebSocket/SSE

Текущий подход (предположительно) — синхронная обработка или polling. Для архивов с сотнями фото пользователь не получает обратной связи.

```go
// Server-Sent Events для прогресса
func (h *Handler) ProcessingProgress(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    jobID := r.URL.Query().Get("job_id")
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    for {
        select {
        case <-r.Context().Done():
            return
        case progress := <-h.progressChan(jobID):
            fmt.Fprintf(w, "data: %s\n\n", progress.JSON())
            flusher.Flush()
            if progress.Done {
                return
            }
        }
    }
}
```

---

## 3. Сводная таблица проблем

| # | Категория | Серьёзность | Проблема | Файл(ы) |
|---|-----------|-------------|----------|----------|
| 1 | Безопасность | 🔴 | Хардкод секретов | `config/`, `docker-compose.yml` |
| 2 | Безопасность | 🔴 | Path Traversal (Zip Slip) | `handler/upload.go` |
| 3 | Надёжность | 🔴 | Нет Graceful Shutdown | `cmd/main.go` |
| 4 | Производительность | 🔴 | Неконтролируемый параллелизм | `service/photo.go` |
| 5 | Архитектура | 🟡 | God-функции, нет DI | Весь `internal/` |
| 6 | Производительность | 🟡 | Нет индексов pgvector | SQL миграции |
| 7 | Качество | 🟡 | `log.Println` вместо структурного логирования | Повсеместно |
| 8 | Качество | 🟡 |