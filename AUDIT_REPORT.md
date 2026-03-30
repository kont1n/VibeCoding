# Аудит проекта Face Grouper — Отчёт и Roadmap

## 1. Краткое резюме (Executive Summary)

**Общая оценка здоровья проекта: 5.5 / 10**

Проект представляет собой функционально рабочий сервис группировки фотографий по лицам с продуманной ML-инфраструктурой (батчинг, пулы детекторов/рекогнайзеров) и хорошей основой CI/CD. Однако он содержит ряд критических проблем безопасности, архитектурных недостатков и багов, которые делают его непригодным для production-развёртывания без доработок.

### Главные риски

| Категория | Уровень риска | Описание |
|-----------|--------------|----------|
| Безопасность | 🔴 Критический | Нет аутентификации на API/UI; дефолтный пароль БД `secret`; Redis без пароля; сломанный rate limiter |
| Баги | 🔴 Критический | Pipeline падает при отключении HTTP-клиента; data race в логгере; утечка памяти сессий |
| Производительность | 🟡 Важный | Нет векторных индексов для поиска, попиксельная обработка изображений, float64↔float32 конверсии |
| Архитектура | 🟡 Важный | Глобальное мутабельное состояние, дублирование кода (pool.go ×2, health.go ×2, schema.sql ×3), мёртвый код |
| Тесты | 🟡 Важный | Минимальное покрытие (3 сервиса из ~15), нет интеграционных тестов |

---

## 2. Приоритетный план действий (Roadmap)

### 🔴 Критические проблемы (Sprint 1 — исправить немедленно)

#### 2.1.1 Безопасность: Сломанный Rate Limiter

**Файл:** `internal/api/http/middleware/middleware.go:87-106`

**Проблема:** Функция `Cleanup` удаляет ВСЕ лимитеры при каждом цикле очистки. `limiter.AllowN(time.Now(), 0)` всегда возвращает `true` (запрос 0 токенов всегда успешен), поэтому все записи удаляются. Rate limiting фактически не работает для повторных запросов.

**Как было:**
```go
func (rl *IPRateLimiter) Cleanup() {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    for ip, limiter := range rl.limiters {
        if limiter.AllowN(time.Now(), 0) {
            delete(rl.limiters, ip)
        }
    }
}
```

**Как должно стать:**
```go
func (rl *IPRateLimiter) Cleanup() {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    now := time.Now()
    for ip, entry := range rl.limiters {
        // Удаляем только те записи, которые не обращались > 3 минут
        if now.Sub(entry.lastSeen) > 3*time.Minute {
            delete(rl.limiters, ip)
        }
    }
}
```
Необходимо обернуть `*rate.Limiter` в структуру с полем `lastSeen time.Time`, обновляемым при каждом обращении.

---

#### 2.1.2 Безопасность: Обход Rate Limiter через X-Forwarded-For

**Файл:** `internal/api/http/middleware/middleware.go:70-73`

**Проблема:** Rate limiter доверяет заголовку `X-Forwarded-For`, который легко подделать. Атакующий может обойти ограничение, отправляя разный IP в каждом запросе.

**Как должно стать:**
```go
func getClientIP(r *http.Request) string {
    // Доверяем X-Forwarded-For только если запрос пришёл от
    // известного reverse proxy (настраивается через конфиг)
    // По умолчанию используем RemoteAddr
    ip, _, _ := net.SplitHostPort(r.RemoteAddr)
    return ip
}
```

---

#### 2.1.3 Безопасность: Дефолтный пароль БД и открытые порты

**Файлы:**
- `deploy/compose/docker-compose.yml` — `DB_PASSWORD:-secret`, PostgreSQL на 0.0.0.0:5432, Redis без пароля на 0.0.0.0:6379
- `internal/repository/postgres/pool.go:70-83` — `Password: "secret"` в `DefaultConfig()`
- `internal/config/env/config.go:163` — `SSLMode` по умолчанию `disable`

**Как должно стать:**
```yaml
# docker-compose.yml
services:
  postgres:
    ports:
      - "127.0.0.1:5432:5432"  # Только localhost
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD:?DB_PASSWORD must be set}  # Обязательный

  redis:
    command: redis-server --appendonly yes --requirepass ${REDIS_PASSWORD:?REDIS_PASSWORD must be set}
    ports:
      - "127.0.0.1:6379:6379"  # Только localhost
```

Удалить `DefaultConfig()` с хардкодом пароля из `pool.go`. Изменить дефолт `SSLMode` на `require`.

---

#### 2.1.4 Баг: Pipeline падает при отключении HTTP-клиента

**Файл:** `internal/api/http/handler/session.go:120`

**Проблема:** Контекст пайплайна создаётся от `r.Context()`. Если HTTP-клиент отключается (закрыл браузер), `r.Context()` отменяется, и вся обработка фотографий прерывается.

**Как было:**
```go
pipelineCtx, cancel := context.WithCancel(r.Context())
```

**Как должно стать:**
```go
pipelineCtx, cancel := context.WithCancel(context.Background())
```

---

#### 2.1.5 Баг: Парсер миграций ломается на PL/pgSQL функциях

**Файл:** `internal/infrastructure/database/migrations.go:46-51`

**Проблема:** SQL разбивается по `;`, что ломает тела PL/pgSQL функций (содержащих внутренние `;`). Функция `persons_search_vector` из `003_add_fulltext_search.sql` будет обрезана.

**Как должно стать:** Использовать полноценный migration-фреймворк (`golang-migrate/migrate`, `pressly/goose`) вместо самописного парсера. Это также решит проблему отсутствия версионирования миграций.

---

#### 2.1.6 Баг: Data race в логгере

**Файл:** `platform/pkg/logger/logger.go:64-93`

**Проблема:** Функции `Debug`, `Info`, `Warn`, `Error` читают переменные `initialized` и `log` без блокировки мьютекса. Если `Init()` вызывается конкурентно с логированием — data race.

**Как должно стать:**
```go
var (
    initOnce    sync.Once
    initialized atomic.Bool
    log         atomic.Pointer[zap.SugaredLogger]
)
```

---

#### 2.1.7 Баг: Утечка памяти сессий

**Файл:** `internal/api/http/handler/session.go:117, 121`

**Проблема:** Сессии и cancel-функции хранятся в `sync.Map`, но никогда не удаляются после завершения. Каждая новая обработка фотографий навсегда остаётся в памяти.

**Решение:** Добавить TTL-очистку завершённых сессий (например, через фоновую горутину, удаляющую сессии старше 1 часа).

---

#### 2.1.8 Безопасность: Directory listing на output директории

**Файл:** `internal/web/server.go:88`

**Проблема:** `http.FileServer(http.Dir(s.cfg.OutputDir))` отдаёт ВСЕ файлы из output, включая `processing.log`, `report.json` и потенциально чувствительные данные.

**Решение:** Ограничить отдачу только файлами изображений (по расширению) или использовать кастомный хендлер вместо `http.FileServer`.

---

#### 2.1.9 Конфигурация: Несуществующая версия Go

**Файлы:** `go.mod`, `.github/workflows/ci.yml`

**Проблема:** `go 1.25.6` не существует (на март 2026 актуальна 1.24.x). Это может вызвать проблемы при сборке стандартным тулчейном.

**Решение:** Исправить на актуальную версию Go.

---

### 🟡 Важные улучшения (Sprint 2-3)

#### 2.2.1 Производительность: Отсутствие векторного индекса

**Файл:** `internal/repository/postgres/schema.sql`

**Проблема:** В schema.sql для sqlc нет индекса на `faces.embedding`. Миграция 004 добавляет HNSW-индекс, но миграция 002 также добавляет IVFFlat-индекс на тот же столбец. Два индекса — лишняя нагрузка на запись.

**Решение:** Удалить IVFFlat из миграции 002 (или добавить миграцию с `DROP INDEX`), оставить только HNSW из 004. Убедиться, что sqlc-схема соответствует реальному состоянию БД.

---

#### 2.2.2 Производительность: Конверсия float64 ↔ float32 для эмбеддингов

**Файлы:**
- `internal/model/database.go` — `Face.Embedding` определён как `[]float64`
- `internal/repository/postgres/face.go:33-37, 77-81, 140-143, 251-254, 309-312` — конверсия при каждом чтении/записи

**Проблема:** PostgreSQL `vector(512)` хранит `float32`. Модель хранит `float64`. Каждая операция с БД конвертирует 512 элементов туда-обратно. Это создаёт: потерю точности, лишние аллокации, дублирование кода конверсии в 5+ местах.

**Как должно стать:**
```go
type Face struct {
    // ...
    Embedding []float32 `json:"embedding"` // Соответствует pgvector float32
}
```

---

#### 2.2.3 Производительность: Попиксельная обработка изображений

**Файл:** `internal/service/imageutil/image.go:57-68`

**Проблема:** Загрузка изображений через `img.At(x, y)` использует interface dispatch на каждый пиксель. Для RGBA-изображений прямой доступ к буферу пикселей быстрее в 10-50 раз.

**Как было:**
```go
for y := 0; y < height; y++ {
    for x := 0; x < width; x++ {
        r, g, b, _ := img.At(x, y).RGBA()
        // ...
    }
}
```

**Как должно стать:**
```go
switch src := img.(type) {
case *image.NRGBA:
    for y := 0; y < height; y++ {
        row := src.Pix[y*src.Stride : y*src.Stride+width*4]
        for x := 0; x < width; x++ {
            r := row[x*4]
            g := row[x*4+1]
            b := row[x*4+2]
            // ...
        }
    }
default:
    // fallback через img.At()
}
```

Аналогичная проблема в `internal/service/avatar/score.go:90-108` (Laplacian variance) и `internal/infrastructure/ml/detector.go:144-153` (BGR canvas copy).

---

#### 2.2.4 Производительность: Batch INSERT в цикле

**Файл:** `internal/repository/postgres/face.go:63-107`

**Проблема:** `CreateBatch` выполняет отдельный `INSERT` на каждое лицо в транзакции. Для 1000 лиц — 1000 SQL-запросов.

**Как должно стать:**
```go
func (r *FaceRepository) CreateBatch(ctx context.Context, faces []*model.Face) error {
    batch := &pgx.Batch{}
    for _, face := range faces {
        batch.Queue(
            `INSERT INTO faces (...) VALUES ($1, $2, ...)`,
            face.ID, face.PhotoID, /* ... */
        )
    }
    results := r.pool.SendBatch(ctx, batch)
    defer results.Close()
    // Проверить ошибки...
}
```
Альтернатива — использовать `COPY` через `pgx.CopyFrom` для максимальной скорости.

---

#### 2.2.5 Архитектура: Дублирование кода

| Что дублируется | Файлы | Решение |
|----------------|-------|---------|
| Pool creation | `internal/infrastructure/database/postgres/pool.go` ↔ `internal/repository/postgres/pool.go` | Удалить один, оставить единый пул |
| Health check | `internal/infrastructure/database/postgres/health.go` ↔ `internal/repository/postgres/health.go` | Удалить дублирующий |
| Schema SQL | `internal/infrastructure/database/postgres/schema.sql` ↔ `internal/repository/postgres/schema.sql` ↔ миграции | Генерировать schema из миграций |
| Detector/Recognizer pool init | `internal/app/di.go:148-215` ↔ `di.go:224-289` | Обобщить в generic pool factory |
| Provider config construction | `internal/app/app.go:144-161` ↔ `internal/app/di.go:153-169` | Вынести в общую функцию |
| `shortPathHash` | `internal/service/extraction/service.go:316-319` ↔ `internal/service/organizer/organizer.go:273` | Вынести в shared утилиту |
| `writeJSON` | `internal/api/http/handler/upload.go` ↔ `internal/web/server.go:200-204` | Вынести в общий пакет |
| Env helpers (`getEnv`, `getEnvInt`) | `internal/config/env/config.go` ↔ `internal/infrastructure/ml/provider/selection.go:74-104` | Использовать единый пакет конфигурации |

---

#### 2.2.6 Архитектура: Глобальное мутабельное состояние

**Файлы:**
- `internal/config/config.go:14` — `var AppConfig *Config`
- `internal/infrastructure/ml/ort.go:13-15` — `ortInitialized`, `sessionTuning`, `selectedProvider`
- `platform/pkg/closer/closer.go` — package-level состояние
- `platform/pkg/logger/logger.go` — package-level состояние

**Проблема:** Глобальные переменные затрудняют тестирование, создают implicit coupling и data race потенциал.

**Решение:** Передавать зависимости через DI-контейнер (уже есть `di.go`), а не обращаться к глобалам. Для логгера — использовать `context.Context` или передавать `*zap.Logger` через конструкторы.

---

#### 2.2.7 Баг: Проглоченная ошибка валидации конфига

**Файл:** `internal/config/config.go:52-54`

```go
if err := AppConfig.Database.Validate(); err != nil {
    _ = err  // Ошибка валидации игнорируется!
}
```

**Решение:** Вернуть ошибку из `Load()`.

---

#### 2.2.8 Баг: TOCTOU race в кэше анкоров детектора

**Файл:** `internal/infrastructure/ml/detector.go:241-266`

**Проблема:** Мьютекс разблокируется после чтения кэша и блокируется снова для записи. Между unlock и повторным lock другая горутина может вычислить тот же ключ — лишняя работа и потенциальный race.

**Как должно стать:** Использовать `sync.Map` или не разблокировать мьютекс между чтением и записью.

---

#### 2.2.9 Безопасность: CDN без SRI

**Файл:** `internal/web/index.html`

**Проблема:** D3.js загружается с `https://d3js.org/d3.v7.min.js` без Subresource Integrity (SRI) хэша. Компрометация CDN = выполнение произвольного JS.

**Как должно стать:**
```html
<script src="https://d3js.org/d3.v7.min.js"
        integrity="sha384-..." crossorigin="anonymous"></script>
```
Либо бандлить D3.js локально.

---

#### 2.2.10 Тестирование: Крайне низкое покрытие

**Текущие тесты:**
- `internal/config/env/config_test.go` — конфиг
- `internal/service/avatar/score_test.go` — скоринг лиц
- `internal/service/clustering/clustering_test.go` — кластеризация
- `internal/service/report/report_test.go` — отчёты

**Нет тестов для:**
- Все HTTP-хендлеры (`handler/`)
- Middleware (rate limiter, recovery, CORS)
- Репозитории (`repository/postgres/`)
- Сервисы: extraction, organizer, scan
- ML-инференс (detector, recognizer, align, nms)
- Pipeline (app/pipeline.go)
- Imageutil

**Решение:** Приоритизировать тесты для:
1. HTTP-хендлеров (можно с httptest)
2. Rate limiter (текущая реализация сломана — тесты бы это выявили)
3. Репозиториев (с testcontainers для PostgreSQL)
4. Imageutil (граничные случаи: nil image, нулевые размеры)

---

### 🟢 Опциональные улучшения (Backlog)

#### 2.3.1 Мёртвый код — удалить

| Файл | Что удалить |
|------|-------------|
| `internal/web/web.go` | Весь файл — `Serve()` нигде не вызывается |
| `internal/model/database.go:51` | `BBox.Array()` — не используется |
| `internal/repository/postgres/pool.go:86-96` | `ConfigFromEnv()` — закомментированная логика |
| `cmd/main.go:26` | `gracefulLogTimeout` — не используется |

---

#### 2.3.2 Именование: ML-адаптеры названы "Repository"

**Файл:** `internal/infrastructure/ml/inference.go`

`DetectorRepository` и `RecognizerRepository` — это не репозитории (паттерн для работы с данными), а ML inference-адаптеры/гейтвеи. Переименовать в `Detector`/`Recognizer` или `DetectorGateway`/`RecognizerGateway`.

---

#### 2.3.3 XSS-"санитизация" в person handler

**Файл:** `internal/api/http/handler/person.go:147-149`

Комментарий говорит "Sanitize name (prevent XSS)", но код проверяет только длину. Если имена отдаются в HTML, необходима реальная санитизация (хотя в `index.html` используется `esc()`, server-side защита отсутствует).

---

#### 2.3.4 Graceful shutdown: Порядок закрытия ресурсов

**Файл:** `platform/pkg/closer/closer.go:81-89`

Именованные closer'ы закрываются в случайном порядке (итерация по `map`). Если ресурсы зависят друг от друга (закрыть БД до ORT), порядок важен.

**Решение:** Использовать `[]struct{ name string; fn func() error }` вместо `map` для гарантии порядка.

---

#### 2.3.5 Filesystem scanner: несоответствие форматов

**Файл:** `internal/repository/filesystem/scanner.go:23-27`

Upload handler поддерживает `.webp`, но filesystem scanner — нет. Также `imageutil/image.go:379-414` валидирует GIF и BMP, которых нет в разрешённых расширениях. Унифицировать список форматов.

---

#### 2.3.6 CI/CD: Закрепить версии GitHub Actions по SHA

**Файлы:** `.github/workflows/ci.yml`, `.github/workflows/docker-build.yml`

`aquasecurity/trivy-action@master` — supply chain risk. Все actions должны быть закреплены по commit SHA, а не по тегу.

---

#### 2.3.7 Миграции: Удалить дублирующий IVFFlat индекс

Миграция 002 создаёт IVFFlat-индекс, миграция 004 — HNSW-индекс на тот же столбец `faces.embedding`. Два индекса создают лишнюю нагрузку при вставке. Добавить миграцию 005 с `DROP INDEX idx_faces_embedding_ivfflat`.

---

#### 2.3.8 Go mod tidy

Запустить `go mod tidy` для очистки `go.sum` от неиспользуемых зависимостей (`entgo.io/ent`, `gorm.io/gorm`, `go-pg/pg`, `uptrace/bun`, `jmoiron/sqlx`).

---

#### 2.3.9 Recognizer: Пулинг аллокаций blob

**Файл:** `internal/infrastructure/ml/recognizer.go:84-123`

`blob := make([]float32, batchSize*3*h*w)` аллоцирует ~9.6MB на каждый вызов инференса (batch 64). Использовать `sync.Pool` для переиспользования.

---

#### 2.3.10 Отсутствие аутентификации

**Файлы:** `internal/web/index.html`, `internal/web/server.go`

На данный момент все API-эндпоинты и UI доступны без аутентификации. Для production-развёртывания необходима хотя бы базовая аутентификация (Basic Auth или token) или ограничение доступа через reverse proxy (nginx с auth).

---

## 3. Рекомендуемые инструменты

| Инструмент | Назначение | Приоритет |
|-----------|-----------|-----------|
| [`golang-migrate/migrate`](https://github.com/golang-migrate/migrate) | Замена самописного парсера миграций | 🔴 Критический |
| [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Проверка зависимостей на CVE (добавить в CI) | 🟡 Важный |
| [`testcontainers-go`](https://github.com/testcontainers/testcontainers-go) | Интеграционные тесты с реальным PostgreSQL | 🟡 Важный |
| `golangci-lint` + `cyclop`/`gocyclo` | Контроль цикломатической сложности (не включён) | 🟡 Важный |
| `golangci-lint` + `nilerr`, `nilnil` | Контроль nil-safety | 🟢 Опционально |
| [`cosign`](https://github.com/sigstore/cosign) | Подпись Docker-образов | 🟢 Опционально |
| SRI Hash Generator | Генерация integrity-хэшей для CDN-скриптов | 🟡 Важный |

---

## 4. Сводная таблица находок

| # | Категория | Серьёзность | Файл | Описание |
|---|----------|-------------|------|----------|
| 1 | Безопасность | 🔴 | middleware.go:87-106 | Сломанный cleanup rate limiter |
| 2 | Безопасность | 🔴 | middleware.go:70-73 | X-Forwarded-For spoofing |
| 3 | Безопасность | 🔴 | docker-compose.yml | Дефолтный пароль `secret`, открытые порты |
| 4 | Безопасность | 🔴 | pool.go:70-83 | Хардкод пароля в DefaultConfig |
| 5 | Безопасность | 🔴 | server.go:88 | Directory listing output директории |
| 6 | Безопасность | 🟡 | env/config.go:163 | SSLMode=disable по умолчанию |
| 7 | Безопасность | 🟡 | index.html | CDN без SRI, нет CSRF |
| 8 | Безопасность | 🟡 | docker-build.yml | trivy-action@master — supply chain risk |
| 9 | Баг | 🔴 | session.go:120 | Pipeline context от HTTP request |
| 10 | Баг | 🔴 | migrations.go:46-51 | SQL split ломает PL/pgSQL |
| 11 | Баг | 🔴 | logger.go:64-93 | Data race при логировании |
| 12 | Баг | 🔴 | session.go:117,121 | Memory leak — сессии не очищаются |
| 13 | Баг | 🟡 | config.go:52-54 | Проглоченная ошибка валидации |
| 14 | Баг | 🟡 | detector.go:241-266 | TOCTOU race в кэше анкоров |
| 15 | Баг | 🟡 | main.go:57 | Двойная регистрация signal handler |
| 16 | Производительность | 🟡 | schema.sql | Нет векторного индекса в sqlc-схеме |
| 17 | Производительность | 🟡 | model/database.go | float64 вместо float32 для эмбеддингов |
| 18 | Производительность | 🟡 | imageutil.go:57-68 | Попиксельная обработка через interface |
| 19 | Производительность | 🟡 | face.go:63-107 | Batch INSERT в цикле |
| 20 | Производительность | 🟢 | recognizer.go:84 | 9.6MB аллокация на батч без пулинга |
| 21 | Архитектура | 🟡 | di.go, pool.go, health.go, schema.sql | Массовое дублирование кода |
| 22 | Архитектура | 🟡 | config.go, ort.go, logger.go, closer.go | Глобальное мутабельное состояние |
| 23 | Архитектура | 🟢 | web.go | Весь файл — мёртвый код |
| 24 | Архитектура | 🟢 | inference.go | ML-адаптеры названы "Repository" |
| 25 | Тесты | 🟡 | весь проект | Покрытие ~15-20% (4 файла из ~30 пакетов) |
| 26 | Конфигурация | 🔴 | go.mod, ci.yml | Несуществующая версия Go 1.25.6 |
| 27 | Конфигурация | 🟢 | go.sum | Stale записи — нужен `go mod tidy` |
| 28 | Документация | 🟢 | models/README.md | Нет верификации целостности моделей |
