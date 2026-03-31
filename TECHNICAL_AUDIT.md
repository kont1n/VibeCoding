# Technical Audit Report: Face Grouper

**Date:** 2026-03-31
**Reviewer:** Senior Software Architect (AI-assisted)
**Branch:** v2
**Module:** `github.com/kont1n/face-grouper`

---

## 1. Executive Summary

**Overall Health Score: 7.5 / 10**

Проект демонстрирует зрелую архитектуру Go-приложения с грамотным применением паттернов (DI, Repository, Gateway, Union-Find). Безопасность реализована на высоком уровне. Основные зоны риска: крайне низкое покрытие тестами, дублирование конфигурации провайдера, отсутствие поддержки контекста в критической секции (кластеризация), и монолитный SPA-фронтенд в одном HTML-файле.

### Главные риски

| Риск | Severity | Описание |
|------|----------|----------|
| Покрытие тестами < 5% | 🔴 Critical | 4 test-файла на ~30 `.go` файлов. Нет тестов для HTTP handlers, ML, repositories |
| Кластеризация не отменяема | 🟡 High | `Cluster()` игнорирует `context.Context` — при 10k+ лиц невозможно остановить |
| Дублирование provider config | 🟡 Medium | Одна и та же логика выбора провайдера в `app.go`, `di.go` (3 раза) |
| Монолитный index.html | 🟡 Medium | 1000+ строк HTML/CSS/JS в одном файле без бандлера |

---

## 2. Архитектура и Структура

### 2.1 Соответствие принципам

**SOLID — 7/10**

| Принцип | Оценка | Комментарий |
|---------|--------|-------------|
| **S** — Single Responsibility | 8/10 | Сервисы хорошо разделены. Но `app.go:runProcess()` совмещает оркестрацию, I/O и форматирование |
| **O** — Open/Closed | 7/10 | Gateway-интерфейсы для ML позволяют подменять реализации. Но `DiContainer` создаёт конкретные типы напрямую |
| **L** — Liskov Substitution | 8/10 | Интерфейсы `ExtractionService`, `ClusterService`, `ScanService` корректны |
| **I** — Interface Segregation | 7/10 | `HealthChecker` — минималистичен. Но `PipelineRunner` мог бы быть разбит на `Runner` + `ProgressReporter` |
| **D** — Dependency Inversion | 6/10 | DI container использует service locator pattern. Репозитории возвращают конкретные типы (`*postgres.PersonRepository`) вместо интерфейсов |

**DRY — 6/10**

Основная проблема — дублирование логики выбора провайдера:

```go
// Этот блок повторяется 3 раза: app.go:144-153, di.go:153-162, di.go:229-238
var preferred provider.ProviderType
if cfg.GPU {
    preferred = provider.ProviderCUDA
    if cfg.ProviderPriority != "" && cfg.ProviderPriority != providerPriorityAuto {
        preferred = provider.ParseProviderType(cfg.ProviderPriority)
    }
} else {
    preferred = provider.ProviderCPU
}
```

**Как должно стать:**
```go
// internal/infrastructure/ml/provider/selection.go
func ResolveProvider(cfg env.ExtractConfig) ProviderConfig {
    preferred := ProviderCPU
    if cfg.GPU {
        preferred = ProviderCUDA
        if cfg.ProviderPriority != "" && cfg.ProviderPriority != "auto" {
            preferred = ParseProviderType(cfg.ProviderPriority)
        }
    }
    return ProviderConfig{
        Preferred:     preferred,
        ForceCPU:      cfg.ForceCPU,
        DeviceID:      cfg.GPUDeviceID,
        AllowFallback: true,
    }
}
```

**KISS — 8/10**

Код в целом прямолинейный. Исключения:
- `recognizerBatcher` — сложная, но оправданная оптимизация
- Кластеризация с блочным умножением матриц — обоснована производительностью

### 2.2 Модульность и связность

```
cmd/main.go
  └── internal/app/app.go         (оркестрация)
       ├── internal/app/di.go     (DI container)
       ├── internal/service/      (бизнес-логика)
       │    ├── scan/
       │    ├── extraction/
       │    ├── clustering/
       │    ├── organizer/
       │    └── report/
       ├── internal/infrastructure/ml/   (ML inference)
       ├── internal/repository/          (данные)
       └── internal/web/                 (HTTP)
```

**Плюсы:**
- Чёткое разделение на слои (infrastructure → repository → service → api → app)
- Gateway-паттерн для ML изолирует ONNX Runtime
- Graceful shutdown через `closer` — единая точка управления ресурсами

**Проблемы:**

1. **DiContainer возвращает конкретные типы для репозиториев** (`di.go:292-329`):
```go
// Сейчас:
func (d *DiContainer) PersonRepository() *postgres.PersonRepository {

// Должно быть:
type PersonRepository interface {
    Create(ctx context.Context, person *model.Person) error
    GetByID(ctx context.Context, id uuid.UUID) (*model.Person, error)
    // ...
}
func (d *DiContainer) PersonRepository() PersonRepository {
```

2. **`app.go:runProcess()` — 150 строк процедурного кода** (строки 140-287). Это God Method. Каждый этап (scan, extract, cluster, organize, report) вызывается последовательно с дублированием паттерна замера времени.

### 2.3 Архитектурные антипаттерны

| Антипаттерн | Где | Описание |
|-------------|-----|----------|
| Service Locator | `di.go` | DiContainer — это service locator, а не true DI. Зависимости тянутся лениво изнутри, а не инжектируются снаружи |
| God Method | `app.go:140` | `runProcess()` содержит всю последовательность обработки в одном методе |
| Primitive Obsession | `session.go:52` | `sessionState.Status` — `string` вместо типизированного enum |
| Feature Envy | `app.go:290` | `buildReportFromResults()` конвертирует `organizer.PersonInfo` → `report.PersonBuildInfo` поле-за-полем; логика принадлежит пакету `report` |
| Hardcoded Strings | `app.go:167` | Путь к ORT DLL для Windows захардкожен |

---

## 3. Качество кода

### 3.1 Читаемость

**Общая оценка: 7/10**

**Плюсы:**
- Консистентное именование Go-стиля (camelCase для приватных, PascalCase для экспортируемых)
- Пакетные комментарии присутствуют
- Ранний возврат при ошибках

**Проблемы:**

1. **Смешение языков в комментариях:**
   - `app.go`: комментарии на русском (`// Создаём директорию вывода`)
   - `clustering.go`: комментарии на английском
   - `di.go`: микс обоих языков

   **Рекомендация:** единый язык (английский) для всех комментариев.

2. **Не информативные имена:**
   - `di.go`: `d` вместо `container` или `di`
   - `extraction/service.go:122`: `g` для errgroup (допустимо, но `eg` или `group` яснее)
   - `clustering.go:69`: `intPair` — слишком обобщённо, лучше `similarPair`

### 3.2 Дублирование кода

| Место | Описание |
|-------|----------|
| `di.go:148-215` vs `di.go:224-289` | `detectorPoolLocked()` и `recognizerPoolLocked()` — практически идентичны (~60 строк каждый). Различаются только типом модели и конфигом сессий |
| `web/server.go:267` vs `handler/upload.go:278` | Два одинаковых определения `writeJSON()` |
| Provider config (см. выше) | 3 копии в `app.go` и `di.go` |

### 3.3 Cyclomatic Complexity

| Функция | Файл | CC (approx) | Рекомендация |
|---------|------|-------------|--------------|
| `runProcess` | `app.go:140` | 12 | Разбить на пайплайн-шаги |
| `detectorPoolLocked` | `di.go:148` | 10 | Вынести фабрику пулов |
| `Extract` | `extraction/service.go:57` | 8 | Приемлемо |
| `handleZip` | `upload.go:187` | 9 | Приемлемо для zip-обработки |
| `Detect` | `detector.go` | 11 | Разбить пре/пост-процессинг |

---

## 4. Безопасность

### 4.1 Общая оценка: 8.5/10

Безопасность — сильная сторона проекта.

**Отличные практики:**

| Мера | Файл | Строки |
|------|------|--------|
| Path traversal protection (double-check) | `web/server.go` | 130-159 |
| Zip bomb protection (2GB limit) | `handler/upload.go` | 209, 234, 262 |
| Zip slip prevention | `handler/upload.go` | 228-231 |
| Magic bytes validation | `handler/upload.go` | 135, 246 |
| SQL injection protection | `postgres/*.go` | Всюду `$1, $2` |
| Rate limiting per-IP | `middleware/middleware.go` | 27-125 |
| CORS (same-origin default) | `web/server.go` | 189 |
| Directory listing disabled | `web/server.go` | 171-173 |
| Request body limits | `middleware/middleware.go` | MaxBodySize |
| Graceful shutdown | `web/server.go` | 226-265 |

### 4.2 Найденные проблемы

**🔴 DB_SSLMODE=disable по умолчанию**

`deploy/env/.env.example:15` — `DB_SSLMODE=disable`. При деплое на сервер с PostgreSQL по сети пароли идут открытым текстом.

```diff
- DB_SSLMODE=disable
+ DB_SSLMODE=require
```

**🟡 Отсутствие аутентификации API**

Все API-эндпоинты доступны без авторизации. Для внутреннего/локального использования приемлемо, но при сетевом деплое это критично.

**🟡 `os.Create` без ограничения permissions**

`handler/upload.go:149`, `handler/upload.go:255` — `os.Create` создаёт файлы с umask (обычно 0644). Для загружаемых файлов лучше `os.OpenFile(..., 0600)`.

**🟢 Rate limiter не различает эндпоинты**

`middleware.go:182` — одинаковый лимит 100 RPS для `/health` и `/api/v1/upload`. Upload должен иметь более строгий лимит.

---

## 5. Производительность

### 5.1 Сильные стороны

| Оптимизация | Файл | Описание |
|-------------|------|----------|
| BLAS-accelerated cosine similarity | `clustering.go:104-154` | Gonum mat.Mul вместо наивного O(n^2*d) |
| Block-wise matrix computation | `clustering.go:115` | blockSize=512, снижает пиковое потребление памяти |
| sync.Pool для blob recycling | `recognizer.go:72-76` | Снижает GC pressure при batch inference |
| Recognizer batching | `extraction/service.go:363-480` | Time/size-based flushing для оптимального GPU utilization |
| Connection pooling | `postgres/pool.go` | pgxpool с настраиваемым размером |
| IVFFlat vector indexes | `migrations/004` | Для similarity search на больших объёмах |

### 5.2 Узкие места

**🔴 Кластеризация не поддерживает контекст**

`clustering.go:27-29`:
```go
func (s *clusterService) Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error) {
    return Cluster(faces, threshold), nil // ctx игнорируется!
}
```

При 10,000+ лиц блочное умножение матриц может занять минуты. Невозможно отменить через context.

**Как исправить:**
```go
func Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error) {
    // ...
    for iStart := 0; iStart < n; iStart += blockSize {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
        // block computation...
    }
}
```

**🟡 O(n^2) memory в кластеризации**

`clustering.go:131` — для каждого блока создаётся `mat.NewDense(rows, cols, nil)`. При 10k лиц это 10000/512 = ~20 блоков по 512*512*8 bytes = ~2MB каждый. Суммарно приемлемо, но стоит добавить `sync.Pool` для переиспользования.

**🟡 SPA загружается целиком**

`web/server.go:21` — `//go:embed index.html` загружает весь HTML (1000+ строк) в память. Для текущего размера приемлемо, но при добавлении ассетов (шрифты, иконки) стоит перейти на `embed.FS`.

**🟡 Отсутствие кэширования**

Нет HTTP-кэширования для статических файлов (images, thumbnails). `serveOutputFile()` не устанавливает `Cache-Control` заголовки.

```go
// web/server.go, в serveOutputFile():
w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
```

**🟢 Thumbnails генерируются синхронно**

`extraction/service.go:274-314` — thumbnail для каждого лица создаётся в потоке обработки. При большом количестве лиц на фото это замедляет pipeline. Можно вынести в отдельную горутину.

---

## 6. Тестирование и Документация

### 6.1 Покрытие тестами

**Оценка: 3/10** — критически недостаточно.

| Test File | Покрывает | Тип |
|-----------|-----------|-----|
| `config/env/config_test.go` | Парсинг конфига, DSN, хелперы | Unit |
| `service/report/report_test.go` | Save/Load отчёта | Unit |
| `service/avatar/score_test.go` | Скоринг качества лица | Unit |
| `service/clustering/clustering_test.go` | Кластеризация + benchmark | Unit |

**Не покрыто тестами:**

| Компонент | Критичность | Риск |
|-----------|-------------|------|
| HTTP handlers (upload, session, person) | 🔴 Critical | XSS, path traversal может появиться при рефакторинге |
| ML inference (detector, recognizer) | 🔴 Critical | Регрессии при обновлении ONNX Runtime |
| Repositories (postgres/*) | 🟡 High | SQL-ошибки при миграциях |
| Middleware (rate limiter, CORS) | 🟡 High | Обход rate limit |
| Web server routing | 🟡 Medium | 404/403 для edge cases |
| Pipeline orchestration | 🟡 Medium | Race conditions в async processing |
| Image processing (imageutil) | 🟢 Medium | Corrupted images |

**Рекомендация:** минимальный набор тестов, который нужен немедленно:
1. Интеграционные тесты для HTTP handlers (httptest)
2. Unit тесты для middleware (rate limiter, CORS)
3. Unit тесты для `processImage()` с моками детектора/рекогнайзера

### 6.2 Документация

- **README:** Отсутствует в корне проекта.
- **GoDoc:** Пакетные комментарии есть, но не везде.
- **API docs:** Нет OpenAPI/Swagger спецификации.
- **Architecture Decision Records:** Нет.

### 6.3 CI/CD

Присутствуют два workflow:
- `ci.yml` — Lint (golangci-lint v2), Test, Build, Docker
- `docker-build.yml` — Multi-platform Docker builds (CPU, GPU, ROCm) + Trivy scanning

**Проблемы CI:**
- Тесты запускаются с `grep -v 'infrastructure/ml'` — ML-код исключён из тестирования
- Нет e2e тестов в pipeline
- Codecov upload с `fail_ci_if_error: false` — молча проглатывает ошибки покрытия

---

## 7. Приоритетный план действий (Roadmap)

### 🔴 Критические проблемы (Sprint 1, 1-2 недели)

| # | Задача | Файл | Описание |
|---|--------|------|----------|
| 1 | Добавить контекст в кластеризацию | `clustering/clustering.go` | Поддержка отмены через `ctx.Done()` в цикле блоков |
| 2 | Тесты для HTTP handlers | `handler/*.go` | Минимум: upload, session start/cancel, path traversal |
| 3 | Тесты для middleware | `middleware/middleware.go` | Rate limiter, CORS, recovery |
| 4 | DB_SSLMODE=require по умолчанию | `deploy/env/.env.example` | Безопасный default |
| 5 | Устранить дублирование `writeJSON` | `upload.go`, `server.go` | Вынести в общий пакет |

### 🟡 Важные улучшения (Sprint 2-3, 2-4 недели)

| # | Задача | Описание |
|---|--------|----------|
| 6 | Вынести provider config в отдельную функцию | Убрать 3 копии из `app.go` и `di.go` |
| 7 | Интерфейсы для репозиториев | `DiContainer` должен возвращать интерфейсы, не конкретные типы |
| 8 | Типизированный enum для session status | `"processing"` → `StatusProcessing` |
| 9 | Разбить `runProcess()` на pipeline pattern | Каждый этап — отдельный `Step` с общим контрактом |
| 10 | HTTP Cache-Control для output files | `Cache-Control: public, max-age=86400` для изображений |
| 11 | README.md | Описание проекта, Quick Start, API docs |
| 12 | Пул матриц в кластеризации | `sync.Pool` для `mat.Dense` в блочном умножении |
| 13 | Единый язык комментариев | Перевести все комментарии на английский |
| 14 | Per-endpoint rate limiting | Разные лимиты для `/health` и `/api/v1/upload` |
| 15 | Интеграционные тесты для postgres repos | Тесты с реальной БД (testcontainers-go) |

### 🟢 Опциональные улучшения (Backlog)

| # | Задача | Описание |
|---|--------|----------|
| 16 | OpenAPI спецификация | Swagger/OpenAPI 3.0 для API |
| 17 | Refactor фабрик пулов | Общая `poolFactory[T]` для детекторов и рекогнайзеров |
| 18 | Metrics (Prometheus) | Метрики обработки, задержки API, размеры batch |
| 19 | Structured error types | `type AppError struct { Code, Message, Details }` вместо `map[string]string` |
| 20 | Аутентификация API | JWT или API key для сетевого деплоя |
| 21 | Разделить SPA на компоненты | Lit/Preact + Vite для фронтенда |
| 22 | Вынести thumbnail generation | Асинхронная генерация thumbnails |
| 23 | e2e тесты в CI | Playwright или testcontainers для полного пайплайна |

---

## 8. Рекомендуемые инструменты

| Категория | Инструмент | Назначение |
|-----------|------------|------------|
| Тестирование | `testcontainers-go` | Интеграционные тесты с PostgreSQL + pgvector |
| Тестирование | `go-cmp` | Сравнение сложных структур в тестах |
| Тестирование | `httptest` (stdlib) | Тесты HTTP handlers |
| Линтинг | golangci-lint v2 (уже есть) | Статический анализ |
| Линтинг | `gocognit` | Проверка когнитивной сложности функций |
| API | `oapi-codegen` | Генерация Go-кода из OpenAPI спецификации |
| Мониторинг | `promhttp` | Prometheus метрики для HTTP server |
| Безопасность | `gosec` (уже в golangci-lint) | Поиск уязвимостей |
| Безопасность | `govulncheck` | Проверка зависимостей на CVE |
| CI | `testcontainers` | PostgreSQL в CI для интеграционных тестов |

---

## 9. Заключение

Проект построен на прочной архитектурной основе. Код безопасен, производителен и хорошо структурирован. Главный технический долг — тестирование (покрытие < 5%) и дублирование кода в DI/provider. Рекомендуемый фокус на ближайший спринт: тесты для HTTP-слоя и поддержка контекста в кластеризации. Эти два изменения дадут наибольший эффект для надёжности и пользовательского опыта.
