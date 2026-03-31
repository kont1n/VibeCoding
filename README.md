# Face Grouper

Сервис автоматической группировки фотографий по людям. Анализирует изображения с помощью нейросетевых моделей (SCRFD + ArcFace), определяет и кластеризует лица, предоставляет веб-интерфейс для просмотра результатов.

## Возможности

- Обнаружение лиц на фотографиях (SCRFD через ONNX Runtime)
- Извлечение 512-мерных эмбеддингов (ArcFace)
- Кластеризация лиц по персонам (BLAS-accelerated cosine similarity + Union-Find)
- Веб-интерфейс: загрузка фото, галерея персон, граф связей, карточка персоны
- Поддержка GPU: NVIDIA CUDA, AMD ROCm, Apple CoreML
- REST API с SSE-стримингом прогресса
- PostgreSQL + pgvector для хранения эмбеддингов и поиска похожих лиц

## Быстрый старт

### Требования

- Go 1.25+
- PostgreSQL 14+ с расширением [pgvector](https://github.com/pgvector/pgvector)
- ONNX Runtime (скачивается автоматически в Docker)
- Модели: `det_10g.onnx` (детекция), `w600k_r50.onnx` (распознавание)

### Локальный запуск

```bash
# Клонирование
git clone https://github.com/kont1n/VibeCoding.git
cd VibeCoding

# Конфигурация
cp deploy/env/.env.example .env
# Отредактируйте .env — укажите параметры БД и путь к моделям

# Запуск обработки + веб-интерфейс
go run ./cmd --serve

# Только веб-интерфейс (просмотр предыдущих результатов)
go run ./cmd --view --port 3000
```

### Docker Compose (рекомендуется)

```bash
cd deploy/compose

# CPU-версия
docker compose up -d postgres face-grouper-cpu

# NVIDIA GPU
docker compose up -d postgres face-grouper-gpu

# AMD ROCm
docker compose up -d postgres face-grouper-rocm
```

Веб-интерфейс: http://localhost:8080

## CLI-флаги

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `-serve` | `false` | Запустить веб-интерфейс после обработки |
| `-view` | `false` | Только веб-интерфейс (без обработки) |
| `-port` | `8080` | Порт веб-интерфейса |

## Конфигурация

Все параметры задаются через переменные окружения или файл `.env`. Полное описание: [doc/configuration.md](doc/configuration.md).

Ключевые параметры:

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `INPUT_DIR` | `./dataset` | Директория с исходными фото |
| `OUTPUT_DIR` | `./output` | Директория результатов |
| `MODELS_DIR` | `./models` | Директория с ONNX-моделями |
| `EXTRACT_WORKERS` | `4` | Количество воркеров обработки |
| `GPU_ENABLED` | `0` | Включить GPU (`1` — да) |
| `CLUSTER_THRESHOLD` | `0.5` | Порог сходства для кластеризации |
| `WEB_PORT` | `8080` | Порт веб-сервера |

## API

REST API доступен по адресу `http://localhost:8080/api/v1/`. Полная документация: [doc/api.md](doc/api.md).

Основные эндпоинты:

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/api/v1/upload` | Загрузка фото (multipart) |
| `POST` | `/api/v1/sessions/{id}/process` | Запуск обработки |
| `GET` | `/api/v1/sessions/{id}/stream` | Прогресс (SSE) |
| `GET` | `/api/v1/persons` | Список персон |
| `GET` | `/api/v1/persons/{id}` | Карточка персоны |
| `GET` | `/api/v1/persons/{id}/relations` | Граф связей |
| `GET` | `/health` | Проверка здоровья |

## Архитектура

```
cmd/main.go                     # Точка входа, CLI-флаги
internal/
  app/                          # Оркестрация, DI-контейнер, pipeline
  api/http/handler/             # HTTP-обработчики
  api/http/middleware/          # Rate limiter, CORS, recovery
  service/
    extraction/                 # Детекция лиц + эмбеддинги
    clustering/                 # Кластеризация (Union-Find + BLAS)
    organizer/                  # Организация результатов, аватары
    report/                     # Генерация отчёта
  infrastructure/
    ml/                         # ONNX Runtime, SCRFD, ArcFace
    database/                   # Миграции PostgreSQL
  repository/
    postgres/                   # CRUD-операции (persons, faces, photos)
    filesystem/                 # Сканирование файлов
  web/                          # HTTP-сервер, встроенный SPA
platform/pkg/                   # Logger (zap), Closer (graceful shutdown)
deploy/
  docker/                       # Dockerfile.cpu, Dockerfile.nvidia, Dockerfile.rocm
  compose/                      # docker-compose.yml
  env/                          # .env.example
```

Подробнее: [doc/architecture.md](doc/architecture.md).

## Деплой

Три варианта Docker-образа:

| Образ | Базовый образ | GPU |
|-------|---------------|-----|
| `Dockerfile.cpu` | `debian:bookworm-slim` | Нет |
| `Dockerfile.nvidia` | `nvidia/cuda:12.3.2-cudnn9` | NVIDIA CUDA |
| `Dockerfile.rocm` | `rocm/dev-ubuntu-22.04` | AMD ROCm |

Подробнее: [doc/deployment.md](doc/deployment.md).

## Стек технологий

| Компонент | Технология |
|-----------|------------|
| Язык | Go 1.25 |
| ML-инференс | ONNX Runtime (SCRFD + ArcFace) |
| БД | PostgreSQL 16 + pgvector |
| Веб-сервер | net/http (stdlib) |
| Логирование | go.uber.org/zap |
| Числа | gonum.org/v1/gonum |
| Драйвер БД | jackc/pgx/v5 |
| Фронтенд | Vanilla JS + D3.js (embedded SPA) |

## Лицензия

MIT
