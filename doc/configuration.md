# Configuration

Все параметры задаются через переменные окружения или файл `.env` в корне проекта. CLI-флаги переопределяют значения из `.env`.

Шаблон конфигурации: [`deploy/env/.env.example`](../deploy/env/.env.example).

---

## Application

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `INPUT_DIR` | string | `./dataset` | Директория с исходными фотографиями |
| `OUTPUT_DIR` | string | `./output` | Директория для результатов обработки |

## Models

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `MODELS_DIR` | string | `./models` | Директория с ONNX-моделями |

Необходимые файлы моделей:
- `det_10g.onnx` — SCRFD модель детекции лиц
- `w600k_r50.onnx` — ArcFace модель извлечения эмбеддингов

## Database (PostgreSQL + pgvector)

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `DB_HOST` | string | — (required) | Хост PostgreSQL |
| `DB_PORT` | int | `5432` | Порт PostgreSQL |
| `DB_NAME` | string | — (required) | Имя базы данных |
| `DB_USER` | string | — (required) | Пользователь |
| `DB_PASSWORD` | string | — (required) | Пароль |
| `DB_SSLMODE` | string | `require` | Режим SSL: `disable`, `require`, `verify-full` |
| `DB_MAX_CONNS` | int | `25` | Максимальное число соединений в пуле |
| `DB_MIN_CONNS` | int | `5` | Минимальное число соединений в пуле |
| `DB_MAX_CONN_LIFETIME` | duration | `1h` | Максимальное время жизни соединения |
| `DB_MAX_CONN_IDLE_TIME` | duration | `30m` | Максимальное время простоя соединения |
| `DB_HEALTH_CHECK_PERIOD` | duration | `1m` | Период проверки здоровья соединений |
| `DB_RUN_MIGRATIONS` | bool | `true` | Автоматический запуск миграций при старте |

> **Примечание:** БД опциональна. Если подключение не удалось, сервис продолжит работу с файловым хранением (report.json). Для rename и relations требуется БД.

## Redis (опционально)

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `REDIS_HOST` | string | `localhost` | Хост Redis |
| `REDIS_PORT` | int | `6379` | Порт Redis |
| `REDIS_PASSWORD` | string | — | Пароль |
| `REDIS_DB` | int | `0` | Номер БД |

> Redis сконфигурирован в docker-compose, но в текущей версии не используется. Подготовлен для будущей системы очередей.

## Extraction (обработка изображений)

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `EXTRACT_WORKERS` | int | `4` | Количество параллельных воркеров обработки |
| `GPU_ENABLED` | bool | `0` | Включить GPU-ускорение (`1` — да) |
| `GPU_DEVICE_ID` | int | `0` | ID GPU-устройства (для multi-GPU) |
| `FORCE_CPU` | bool | `0` | Принудительное использование CPU |
| `PROVIDER_PRIORITY` | string | `auto` | Приоритет провайдера: `auto`, `cpu`, `cuda`, `rocm`, `directml` |
| `GPU_DET_SESSIONS` | int | `2` | Количество ONNX-сессий детектора (GPU) |
| `GPU_REC_SESSIONS` | int | `2` | Количество ONNX-сессий рекогнайзера (GPU) |
| `EMBED_BATCH_SIZE` | int | `64` | Размер батча для извлечения эмбеддингов |
| `EMBED_FLUSH_MS` | int | `10` | Таймаут flush батча (мс) |
| `MAX_DIM` | int | `1920` | Максимальная размерность изображения (0 — без ограничения) |
| `DET_THRESH` | float | `0.5` | Порог уверенности детекции лиц |

### Выбор GPU-провайдера

При `GPU_ENABLED=1` система автоматически определяет доступные провайдеры:

| Провайдер | Приоритет | Детекция |
|-----------|-----------|----------|
| `cuda` | 10 | `nvidia-smi`, `CUDA_HOME`, `CUDA_PATH` |
| `coreml` | 15 | macOS (автоматически) |
| `rocm` | 20 | `rocm-smi`, `ROCM_PATH`, `HIP_PATH` |
| `directml` | 30 | Windows, `DirectML.dll` |
| `cpu` | — | Всегда доступен (fallback) |

Для ручного выбора: `PROVIDER_PRIORITY=cuda` (или `rocm`, `directml`, `cpu`).

## Clustering

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `CLUSTER_THRESHOLD` | float | `0.5` | Порог cosine similarity для объединения лиц в кластер. Меньше = строже, больше = свободнее |

> Рекомендуемые значения: 0.4 — строгая кластеризация, 0.5 — сбалансированная, 0.6 — мягкая.

## Organizer

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `AVATAR_UPDATE_THRESHOLD` | float | `0.10` | Минимальное улучшение quality score для обновления аватара |

## Web

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `WEB_PORT` | int | `8080` | Порт HTTP-сервера |
| `WEB_SERVE` | bool | `false` | Запускать веб-интерфейс после обработки |
| `WEB_VIEW_ONLY` | bool | `false` | Режим только просмотра |

### Таймауты сервера

| Параметр | Значение | Описание |
|----------|---------|----------|
| Read Timeout | 30 сек | Таймаут чтения запроса |
| Write Timeout | 120 сек | Таймаут записи ответа (увеличен для SSE) |
| Idle Timeout | 120 сек | Таймаут простоя соединения |
| Shutdown Timeout | 10 сек | Таймаут graceful shutdown сервера |

## Logger

| Переменная | Тип | По умолчанию | Описание |
|-----------|-----|-------------|----------|
| `LOG_LEVEL` | string | `info` | Уровень логирования: `debug`, `info`, `warn`, `error` |
| `LOG_JSON` | bool | `false` | Формат JSON (для production). `false` — human-readable |

## Пример минимальной конфигурации

```env
# Минимальная конфигурация для локального запуска (без БД)
INPUT_DIR=./photos
OUTPUT_DIR=./output
MODELS_DIR=./models
EXTRACT_WORKERS=4
CLUSTER_THRESHOLD=0.5
WEB_PORT=8080
WEB_SERVE=true
LOG_LEVEL=info
```

## Пример production-конфигурации

```env
# Production с GPU и PostgreSQL
INPUT_DIR=/data/photos
OUTPUT_DIR=/data/output
MODELS_DIR=/app/models

DB_HOST=postgres
DB_PORT=5432
DB_NAME=face-grouper
DB_USER=face-grouper
DB_PASSWORD=${DB_PASSWORD}
DB_SSLMODE=require
DB_MAX_CONNS=50
DB_MIN_CONNS=10

EXTRACT_WORKERS=8
GPU_ENABLED=1
PROVIDER_PRIORITY=cuda
GPU_DEVICE_ID=0
GPU_DET_SESSIONS=4
GPU_REC_SESSIONS=4
EMBED_BATCH_SIZE=128
MAX_DIM=0

CLUSTER_THRESHOLD=0.5
WEB_PORT=8080
WEB_SERVE=true
LOG_LEVEL=info
LOG_JSON=true
```
