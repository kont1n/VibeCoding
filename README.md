# Face Grouper

**Face Grouper** — это высокопроизводительное Go-приложение для автоматической группировки лиц на фотографиях. Используйте ML-модели для обнаружения и распознавания лиц, кластеризации по сходству и организации результатов в удобную галерею.

## Особенности

- 🔍 **Обнаружение лиц** — детекция лиц на изображениях с помощью ONNX Runtime
- 🧠 **Распознавание** — извлечение эмбеддингов лиц для сравнения
- 📊 **Кластеризация** — группировка лиц по сходству (cosine similarity)
- 🖼️ **Галерея персон** — веб-интерфейс для просмотра результатов
- 🚀 **Производительность** — поддержка CPU/GPU (CUDA, ROCm, DirectML)
- 🔒 **Безопасность** — защита от path traversal, zip bomb, zip slip
- 📈 **Прогресс** — SSE streaming для отслеживания обработки

## Быстрый старт

### Требования

- Go 1.25+
- Docker и Docker Compose (опционально, для PostgreSQL)
- ONNX Runtime модели (загружаются автоматически)

### Установка

```bash
git clone https://github.com/kont1n/face-grouper.git
cd face-grouper
go mod download
```

### Настройка

Скопируйте `.env.example` в `.env` и настройте параметры:

```bash
cp deploy/env/.env.example .env
```

Основные параметры:

```ini
# Пути
INPUT_DIR=./dataset          # Директория с исходными фото
OUTPUT_DIR=./output          # Директория для результатов
MODELS_DIR=./models          # Директория с ML-моделями

# Обработка
EXTRACT_WORKERS=4            # Количество воркеров
GPU_ENABLED=0                # 1 для GPU, 0 для CPU
CLUSTER_THRESHOLD=0.5        # Порог кластеризации (0.0-1.0)

# Веб-интерфейс
WEB_PORT=8080
WEB_SERVE=false              # Запустить веб-UI после обработки
```

### Запуск

**Полная обработка с веб-интерфейсом:**

```bash
go run cmd/main.go --serve --port 8080
```

**Только просмотр предыдущих результатов:**

```bash
go run cmd/main.go --view --port 8080
```

**Обработка без веб-интерфейса:**

```bash
go run cmd/main.go
```

### Docker

```bash
# CPU версия
docker-compose -f docker-compose.cpu.yml up

# GPU версия (NVIDIA)
docker-compose -f docker-compose.gpu.yml up
```

## Архитектура

```
cmd/main.go
  └── internal/app/app.go         (оркестрация пайплайна)
       ├── internal/service/      (бизнес-логика)
       │    ├── scan/             (сканирование файлов)
       │    ├── extraction/       (ML inference)
       │    ├── clustering/       (группировка лиц)
       │    ├── organizer/        (организация результатов)
       │    └── report/           (генерация отчётов)
       ├── internal/infrastructure/ml/  (ONNX Runtime)
       ├── internal/repository/         (данные)
       └── internal/web/                (HTTP сервер)
```

### Пайплайн обработки

1. **Scan** — сканирование директории, поиск изображений и ZIP-архивов
2. **Extract** — обнаружение лиц, извлечение эмбеддингов, создание thumbnail
3. **Cluster** — кластеризация лиц по сходству (Union-Find + cosine similarity)
4. **Organize** — сохранение результатов, создание аватаров персон
5. **Report** — генерация `report.json` с итогами обработки

## API

### Endpoints

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/api/v1/upload` | Загрузка файлов (multipart/form-data) |
| `POST` | `/api/v1/sessions/{id}/process` | Начать обработку сессии |
| `GET` | `/api/v1/sessions/{id}/status` | Получить статус сессии |
| `GET` | `/api/v1/sessions/{id}/stream` | SSE streaming прогресса |
| `POST` | `/api/v1/sessions/{id}/cancel` | Отменить обработку |
| `GET` | `/api/v1/persons` | Список персон |
| `GET` | `/api/v1/persons/{id}` | Информация о персоне |
| `PUT` | `/api/v1/persons/{id}` | Переименовать персону |
| `GET` | `/api/v1/persons/{id}/photos` | Фотографии персоны |
| `GET` | `/api/v1/sessions/{id}/errors` | Ошибки обработки |

### Пример загрузки

```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -F "files=@photo1.jpg" \
  -F "files=@photo2.jpg" \
  -F "files=@archive.zip"
```

### SSE Progress Events

```json
{
  "session_id": "abc123",
  "stage": "extract",
  "stage_label": "Обнаружение лиц...",
  "progress": 0.45,
  "elapsed_ms": 12000,
  "estimated_ms": 26000,
  "eta_ms": 14000,
  "done": false
}
```

## Форматы файлов

### Поддерживаемые изображения

- JPEG (`.jpg`, `.jpeg`)
- PNG (`.png`)
- WEBP (`.webp`)

### ZIP-архивы

При загрузке ZIP-архива автоматически извлекаются все изображения.

**Защита:**
- Max размер архива: 2GB
- Zip slip prevention
- Magic bytes валидация

## Отчёты

После обработки в `OUTPUT_DIR` создаются:

- `report.json` — полный отчёт с результатами
- `processing.log` — лог обработки
- `person_N/` — директории с фото персон
- `.thumbnails/` — превью изображений

### Структура report.json

```json
{
  "started_at": "2026-03-31T10:00:00Z",
  "duration": "45s",
  "total_images": 150,
  "total_faces": 423,
  "total_persons": 87,
  "errors": 3,
  "threshold": 0.5,
  "persons": [
    {
      "id": "uuid",
      "photo_count": 5,
      "face_count": 12,
      "avatar_path": "person_1/avatar.jpg",
      "quality_score": 0.85
    }
  ]
}
```

## Конфигурация

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `INPUT_DIR` | Директория с исходными фото | `./dataset` |
| `OUTPUT_DIR` | Директория результатов | `./output` |
| `EXTRACT_WORKERS` | Количество воркеров | `4` |
| `GPU_ENABLED` | Включить GPU (1/0) | `0` |
| `GPU_DEVICE_ID` | ID GPU устройства | `0` |
| `FORCE_CPU` | Принудительно CPU | `0` |
| `PROVIDER_PRIORITY` | Приоритет провайдера | `auto` |
| `CLUSTER_THRESHOLD` | Порог кластеризации | `0.5` |
| `WEB_PORT` | Порт веб-интерфейса | `8080` |
| `LOG_LEVEL` | Уровень логирования | `info` |

### GPU поддержка

Face Grouper поддерживает следующие бэкенды ONNX Runtime:

- **CPU** — кроссплатформенный, по умолчанию
- **CUDA** — NVIDIA GPU (требуется установка CUDA Toolkit)
- **ROCm** — AMD GPU
- **DirectML** — Windows DirectML

## Безопасность

- ✅ Path traversal protection
- ✅ Zip bomb protection (2GB limit)
- ✅ Zip slip prevention
- ✅ Magic bytes validation
- ✅ SQL injection protection (parameterized queries)
- ✅ Rate limiting (100 RPS по умолчанию)
- ✅ CORS (same-origin default)
- ✅ Request body limits

## Тестирование

```bash
# Запустить все тесты
go test ./...

# Запустить с race detector
go test ./... -race

# Запустить конкретный пакет
go test ./internal/api/http/handler/... -v
```

## Производительность

| Сценарий | Время |
|----------|-------|
| 100 фото, 200 лиц | ~30 сек (CPU) |
| 1000 фото, 2000 лиц | ~5 мин (CPU) |
| 1000 фото, 2000 лиц | ~1 мин (GPU) |

*Время зависит от hardware и настроек.*
