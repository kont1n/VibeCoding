# Face Grouping Service

Сервис для автоматической группировки фотографий по людям. Анализирует изображения при помощи нейросетевых моделей [InsightFace](https://github.com/deepinsight/insightface) (SCRFD + ArcFace) через ONNX Runtime, извлекает face embeddings и кластеризует лица по косинусному сходству. Полностью нативная Go-реализация без зависимости от Python и OpenCV. Поддерживает GPU-ускорение (CUDA), BLAS-ускоренную кластеризацию, генерацию миниатюр лиц, алгоритмический выбор аватара и веб-интерфейс для просмотра результатов.

## Архитектура

Проект использует **Clean Architecture** с разделением на слои:

```
cmd/
└── main.go                    # Точка входа (DI, graceful shutdown)

internal/
├── api/cli/                   # CLI API handlers
├── app/                       # Приложение + DI контейнер
│   ├── app.go
│   └── di.go
├── config/                    # Конфигурация (.env + ENV)
│   ├── config.go
│   └── env/
├── model/                     # Доменные модели (Face, Cluster)
├── repository/                # Слой доступа к данным
│   ├── filesystem/            # Сканирование файлов
│   └── inference/             # ONNX Runtime inference
├── service/                   # Бизнес-логика
│   ├── scan/                  # Сканирование директорий
│   ├── extraction/            # Извлечение эмбеддингов
│   ├── clustering/            # Кластеризация лиц
│   └── organization/          # Организация результатов
└── ...

platform/pkg/                  # Платформенные пакеты
├── closer/                    # Graceful shutdown
└── logger/                    # Zap logger wrapper
```

### Слои

| Слой | Ответственность |
|------|----------------|
| **API** | Обработка CLI команд, маппинг запросов |
| **Service** | Бизнес-логика, координация между репозиториями |
| **Repository** | Доступ к данным (файлы, ONNX inference) |
| **Model** | Доменные модели (Face, Cluster) |
| **Config** | Конфигурация приложения |
| **Platform** | Инфраструктурные компоненты (logger, closer) |

## Как это работает

```mermaid
flowchart LR
    A["dataset/\n(фотографии)"] --> B["Go CLI\n(main.go)"]
    B --> C["Детекция лиц\nSCRFD (det_10g.onnx)\nONNX Runtime"]
    C --> D["Выравнивание\nSimilarity Transform\n+ WarpAffine"]
    D --> E["Эмбеддинги 512-dim\nArcFace (w600k_r50.onnx)\nONNX Runtime"]
    E --> F["Кластеризация\n(Union-Find +\nBLAS matrix mul)"]
    F --> G["Организация\nPerson_N/ + thumb.jpg"]
    G --> H["report.json\n+ processing.log"]
    H --> I["Web UI\n:8080"]
    G --> J["Оценка качества лица\n(Area × Sharpness × FrontalPose)"]
    J --> H
```

### Пайплайн

1. **Сканирование** — Go обходит входную директорию и собирает все `.jpeg`, `.jpg`, `.png` файлы
2. **Детекция лиц** — SCRFD модель (`det_10g.onnx`) через ONNX Runtime Go биндинги: letterbox preprocessing (640x640), 3-уровневый FPN (strides 8/16/32), декодирование bbox + keypoints, NMS (IoU 0.4)
3. **Выравнивание лиц** — similarity transform (Umeyama algorithm) по 5 ключевым точкам, WarpAffine до 112x112 на чистом Go (билинейная интерполяция)
4. **Извлечение embeddings** — ArcFace модель (`w600k_r50.onnx`): нормализация (mean=127.5, std=127.5), batch inference, L2-нормализация 512-мерных векторов
5. **Генерация миниатюр** — для каждого обнаруженного лица вырезается crop с паддингом 25%, масштабируется до 160x160 и сохраняется как JPEG (quality 90)
6. **Кластеризация** — Go вычисляет матрицу косинусного сходства через BLAS-ускоренное блочное матричное перемножение (gonum) и группирует лица через Union-Find (disjoint set с path compression и union by rank)
7. **Организация** — для каждого кластера создается папка `Person_N/` с символическими ссылками на оригиналы и лучшей миниатюрой лица (`thumb.jpg`); при совпадающих именах файлов применяется уникализация
8. **Выбор аватара** — для каждой персоны выбирается лучший crop по формуле `Score = Area × Sharpness × FrontalPoseFactor`; обновление выполняется только при приросте качества выше порога
9. **Отчёт** — сохраняется JSON-отчёт (`report.json`) и лог обработки (`processing.log`) c `avatar_path` и `quality_score`
10. **Веб-интерфейс (опционально)** — Go HTTP-сервер с graceful shutdown и тёмной темой для просмотра результатов в браузере

> Если на фото несколько людей — оно появится в нескольких папках `Person_N/`.

## Требования

| Компонент | Версия | Примечание |
|-----------|--------|------------|
| Go | 1.24+ | Основной язык |
| ONNX Runtime | 1.24+ | CPU или GPU версия |
| ОС | Windows / Linux / macOS | |
| GPU (опционально) | NVIDIA + CUDA 11.8+ | Для GPU-ускорения |
| cuDNN (опционально) | 8.x | Для GPU-ускорения |

> **Примечание:** С версии 0.2 проект использует чистый Go для обработки изображений и больше не требует OpenCV/gocv.

### Требования для GPU режима

Для работы на GPU требуется:

1. **NVIDIA GPU** с поддержкой CUDA (Compute Capability 5.0+)
2. **CUDA Toolkit 11.8** — https://developer.nvidia.com/cuda-11-8-0-download-archive
3. **cuDNN 8.x** — https://developer.nvidia.com/cudnn (требуется регистрация)
4. **ONNX Runtime GPU** — скачать с https://github.com/microsoft/onnxruntime/releases

**Установка CUDA и cuDNN на Windows:**

```powershell
# 1. Установите CUDA Toolkit 11.8
# Скачайте с https://developer.nvidia.com/cuda-11-8-0-download-archive

# 2. Установите cuDNN
# 1. Скачайте cuDNN 8.x для CUDA 11.8
# 2. Распакуйте архив
# 3. Скопируйте файлы из bin/ в C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v11.8\bin

# 3. Добавьте CUDA в PATH (если не добавлено автоматически)
$env:Path += ";C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v11.8\bin"
$env:Path += ";C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v11.8\libnvvp"
```

> **Примечание:** После установки CUDA перезапустите терминал.

На **Windows** для создания символических ссылок необходим Developer Mode или запуск от имени администратора.

## Установка

### 1. Конфигурация

Создайте файл `.env` в корне проекта (или используйте `.env.example` как шаблон):

```bash
# Face Grouper Configuration

# === Application ===
INPUT_DIR=./dataset
OUTPUT_DIR=./output

# === Models ===
MODELS_DIR=./models

# === Extraction ===
EXTRACT_WORKERS=4
GPU_ENABLED=0              # 1 для GPU, 0 для CPU
GPU_DET_SESSIONS=2
GPU_REC_SESSIONS=2
EMBED_BATCH_SIZE=64
EMBED_FLUSH_MS=10
MAX_DIM=1920
DET_THRESH=0.5

# === Clustering ===
CLUSTER_THRESHOLD=0.5

# === Organizer ===
AVATAR_UPDATE_THRESHOLD=0.10

# === Web ===
WEB_PORT=8080
WEB_SERVE=false
WEB_VIEW_ONLY=false

# === Logger ===
LOG_LEVEL=info
LOG_JSON=false
```

### 2. ONNX Runtime

Скачайте shared library с [github.com/microsoft/onnxruntime/releases](https://github.com/microsoft/onnxruntime/releases):

- **Windows CPU**: `onnxruntime.dll` (из `onnxruntime-win-x64-*.zip`)
- **Windows GPU**: `onnxruntime.dll` (из `onnxruntime-win-x64-gpu-*.zip`)
- **Linux**: `libonnxruntime.so` (из `onnxruntime-linux-x64-*.tgz`)
- **macOS**: `libonnxruntime.dylib` (из `onnxruntime-osx-*.tgz`)

Поместите библиотеку в:
- **CPU**: корень проекта или любую директорию в PATH
- **GPU**: `./runtime/onnxruntime-win-x64-gpu-*/lib/onnxruntime.dll`

### 3. ONNX-модели InsightFace

Скачайте модели из [InsightFace model zoo](https://github.com/deepinsight/insightface/tree/master/model_zoo#buffalo_l) (пакет `buffalo_l`):

- `det_10g.onnx` (~17 MB) — SCRFD детектор лиц
- `w600k_r50.onnx` (~174 MB) — ArcFace распознавание лиц

**Способ 1: Вручную через браузер**
1. Откройте https://github.com/deepinsight/insightface/tree/master/model_zoo#buffalo_l
2. Скачайте `det_10g.onnx` и `w600k_r50.onnx`
3. Поместите в `./models/`

**Способ 2: Через Python (рекомендуется)**
```powershell
py -m pip install huggingface_hub
py -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/det_10g.onnx', local_dir='./models')"
py -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/w600k_r50.onnx', local_dir='./models')"
```

### 4. Сборка проекта

**Windows:**
```powershell
go build -o face-grouper.exe ./cmd
```

**Linux / macOS:**
```bash
go build -o face-grouper ./cmd
```

> **Примечание:** Для сборки больше не требуется MSYS2/OpenCV. Только Go компилятор.

## Запуск

### CPU режим (базовый)

```bash
# Базовый запуск
.\face-grouper.exe

# С веб-интерфейсом
.\face-grouper.exe --serve

# Просмотр предыдущих результатов
.\face-grouper.exe --view
```

### GPU режим (требует CUDA)

```bash
# Запуск на GPU
.\face-grouper.exe

# GPU + веб-интерфейс
.\face-grouper.exe --serve
```

> **Важно:** Для GPU режима необходимо установить CUDA Toolkit 11.8+ и cuDNN 8.x

### CLI флаги

| Флаг | .env переменная | По умолчанию | Описание |
|------|----------------|-------------|----------|
| `--input` | `INPUT_DIR` | `./dataset` | Директория с фотографиями |
| `--output` | `OUTPUT_DIR` | `./output` | Директория для результатов |
| `--models-dir` | `MODELS_DIR` | `./models` | Директория с ONNX-моделями |
| `--workers` | `EXTRACT_WORKERS` | `4` | Количество воркеров (CPU) |
| `--gpu-det-sessions` | `GPU_DET_SESSIONS` | `2` | Detector сессий (GPU) |
| `--gpu-rec-sessions` | `GPU_REC_SESSIONS` | `2` | Recognizer сессий (GPU) |
| `--embed-batch-size` | `EMBED_BATCH_SIZE` | `64` | Размер батча распознавания |
| `--embed-flush-ms` | `EMBED_FLUSH_MS` | `10` | Таймаут flush батча (мс) |
| `--threshold` | `CLUSTER_THRESHOLD` | `0.5` | Порог кластеризации |
| `--det-thresh` | `DET_THRESH` | `0.5` | Порог детекции лиц |
| `--max-dim` | `MAX_DIM` | `1920` | Макс. размер изображения |
| `--avatar-update-threshold` | `AVATAR_UPDATE_THRESHOLD` | `0.10` | Порог обновления аватара |
| `--serve` | `WEB_SERVE` | `false` | Запустить веб-интерфейс |
| `--port` | `WEB_PORT` | `8080` | Порт веб-сервера |
| `--view` | `WEB_VIEW_ONLY` | `false` | Только просмотр |

> **Примечание:** Флаги имеют приоритет над `.env` файлом.

## Тестирование и CI

### Локальные тесты core-пакетов

```bash
go test ./internal/scanner ./internal/report ./internal/clustering -count=1
```

### Compile-check пакетов без CGO

```bash
go test ./internal/avatar ./internal/organizer ./internal/web -count=1
```

### Benchmark кластеризации

```bash
go test ./internal/clustering -bench BenchmarkCluster512D -benchmem -run ^$
```

### CI (GitHub Actions)

В репозитории настроен workflow `.github/workflows/ci.yml`, который запускается на `push` и `pull_request` и выполняет:
- unit-тесты `scanner/report/clustering`
- compile-check `avatar/organizer/web`

### Параметры CLI

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `--input` | `./dataset` | Директория с исходными фотографиями |
| `--output` | `./output` | Директория для результатов группировки |
| `--models-dir` | `./models` | Директория с ONNX-моделями (det_10g.onnx, w600k_r50.onnx) |
| `--ort-lib` | *авто* | Путь к ONNX Runtime shared library |
| `--workers` | `4` | Количество параллельных воркеров |
| `--gpu-det-sessions` | `2` | Количество detector-сессий в GPU режиме |
| `--gpu-rec-sessions` | `2` | Количество recognizer-сессий в GPU режиме |
| `--embed-batch-size` | `64` | Размер межфайлового батча распознавания лиц |
| `--embed-flush-ms` | `10` | Таймаут flush батча распознавания (мс) |
| `--threshold` | `0.5` | Порог косинусного сходства для объединения лиц (0.0–1.0) |
| `--det-thresh` | `0.5` | Порог уверенности детекции лиц |
| `--gpu` | `false` | Использовать CUDA GPU для ONNX Runtime |
| `--intra-threads` | `0` | Intra-op потоки ONNX Runtime (0 = default) |
| `--inter-threads` | `0` | Inter-op потоки ONNX Runtime (0 = default) |
| `--max-dim` | `1920` | Уменьшать изображения до N px по длинной стороне (0 = без ресайза) |
| `--avatar-update-threshold` | `0.10` | Минимальный относительный прирост quality score для обновления аватара |
| `--serve` | `false` | Запустить веб-интерфейс после обработки |
| `--port` | `8080` | Порт веб-сервера |
| `--view` | `false` | Только просмотр результатов (без обработки) |

### Пример вывода

```
=== Scanning directory ===
Found 685 image(s)

=== Extracting face embeddings ===
Mode: CPU, 4 worker(s)
Pre-resize: max 1920px
[1/685] C:\photos\TCF_001.jpeg — found 2 face(s)
[2/685] C:\photos\TCF_002.jpeg — found 1 face(s)
...

Total faces detected: 1247 (errors: 3)

=== Clustering faces ===
Found 42 person(s)

=== Organizing output ===
Person_1: 87 unique photo(s)
Person_2: 64 unique photo(s)
...

=== Summary ===
Images:  685
Faces:   1247
Persons: 42
Errors:  3
Time:    4m12s
Report:  ./output/report.json
Log:     ./output/processing.log

Tip: run with --serve to view results in browser, or --view to view previous results
```

## Алгоритмический выбор аватара

Для каждой персоны вычисляется quality score по формуле:

`Score = (Width * Height) * Sharpness * FrontalPoseFactor`

- `Width * Height` — площадь лица по bbox.
- `Sharpness` — дисперсия лапласиана по crop лица.
- `FrontalPoseFactor` — фактор фронтальности (по landmark-геометрии, ближе к 1 при меньшем повороте).

Если при повторной обработке новый score не превосходит предыдущий минимум на `--avatar-update-threshold` (по умолчанию 10%), аватар не обновляется.

## Веб-интерфейс

Встроенный HTTP-сервер с graceful shutdown и тёмной темой для просмотра результатов:

- Сетка карточек персон с миниатюрами лиц и количеством фото
- Отображение выбранного алгоритмом аватара
- Показ `quality_score` для контроля качества выбора
- Просмотр всех фотографий персоны по клику с превью лица в заголовке
- Кликабельный счётчик ошибок с детализацией (имя файла + текст ошибки)
- Полноэкранный просмотр фото
- Адаптивная вёрстка
- Корректное завершение по Ctrl+C (graceful shutdown с таймаутом 5 сек)

Запуск: `--serve` (после обработки) или `--view` (просмотр готовых результатов).

## Структура проекта

```
├── cmd/
│   └── main.go                       # Точка входа (DI, graceful shutdown)
├── internal/
│   ├── api/cli/                      # CLI API handlers
│   ├── app/                          # Приложение + DI контейнер
│   ├── config/                       # Конфигурация (.env + ENV)
│   ├── model/                        # Доменные модели
│   ├── repository/                   # Слой доступа к данным
│   ├── service/                      # Бизнес-логика
│   ├── inference/                    # ONNX Runtime inference
│   ├── clustering/                   # Кластеризация (Union-Find + BLAS)
│   ├── organizer/                    # Организация результатов
│   ├── report/                       # JSON отчёты
│   ├── avatar/                       # Оценка качества лиц
│   └── web/                          # HTTP сервер + UI
├── platform/pkg/                     # Платформенные пакеты
│   ├── closer/                       # Graceful shutdown
│   └── logger/                       # Zap logger wrapper
├── models/                           # ONNX модели
├── dataset/                          # Входные фото
└── output/                           # Результаты
```

## Модули

### `internal/service` — бизнес-логика

- **`scan/`** — сканирование директорий с изображениями
- **`extraction/`** — извлечение face embeddings (worker pool, batch inference)
- **`clustering/`** — кластеризация лиц по косинусному сходству
- **`organization/`** — организация результатов (Person_N/, аватары)

### `internal/repository` — доступ к данным

- **`filesystem/`** — сканирование файловой системы
- **`inference/`** — ONNX Runtime inference (detector, recognizer)

### `internal/inference` — нативный inference

ONNX Runtime Go-биндинги для прямого запуска моделей InsightFace:

- **`ort.go`** — инициализация/финализация ONNX Runtime, загрузка shared library, создание сессий с CPU/CUDA провайдерами
- **`detector.go`** — SCRFD детектор: letterbox resize, нормализация (mean=127.5, std=128.0), NCHW конвертация, декодирование anchor-based выходов по 3 уровням FPN, фильтрация по порогу
- **`recognizer.go`** — ArcFace: нормализация (mean=127.5, std=127.5), batch inference, L2-нормализация 512-мерных эмбеддингов
- **`align.go`** — Umeyama similarity transform по 5 facial landmarks, WarpAffine на чистом Go (билинейная интерполяция) для выравнивания лица до 112x112
- **`nms.go`** — greedy Non-Maximum Suppression с IoU threshold

### `internal/imageutil` — обработка изображений на чистом Go

- **`image.go`** — загрузка/сохранение JPEG, resize (билинейная интерполяция), blob conversion (NCHW), warp affine transform, crop

### `internal/extractor` — оркестрация extraction pipeline

Worker pool из горутин, каждый worker: загрузка изображения (чистый Go) → опциональный downscale (`--max-dim`) → SCRFD детекция → alignment по keypoints → отправка aligned-лиц в межфайловый batcher распознавания (`--embed-batch-size`, `--embed-flush-ms`) → сохранение thumbnail (crop + resize 160x160, JPEG q90).

CPU-режим использует пул ONNX-сессий размером `workers`. GPU-режим использует отдельные пулы detector/recognizer сессий (`--gpu-det-sessions`, `--gpu-rec-sessions`) без глобальной блокировки inference.

### `internal/model` — типы данных

- **`Face`** — обнаруженное лицо: bounding box `[x1, y1, x2, y2]`, 512-мерный embedding, уверенность детекции, путь к миниатюре, путь к исходному файлу
- **`Cluster`** — группа лиц одного человека

### `internal/clustering` — кластеризация

Строит матрицу L2-нормализованных embeddings и вычисляет косинусное сходство через блочное матричное перемножение (gonum BLAS `dgemm`). Блоки размером 512x512 обеспечивают эффективное использование CPU-кэша. Пары с сходством >= порога объединяются через Union-Find.

### `internal/organizer` — организация результатов

Создает директории `output/Person_N/`, сортирует кластеры по размеру. Создает символические ссылки на оригиналы (fallback на stream-copy). Выбирает лучший аватар по quality score (`Area × Sharpness × FrontalPoseFactor`) и сохраняет в `output/avatars/`. При коллизиях имён файлов применяется уникализация.

### `internal/report` — отчёт

Структурированный JSON-отчёт с метриками обработки.

### `internal/web` — веб-интерфейс

Встроенный HTTP-сервер (Go `net/http` + `embed`) с graceful shutdown.

### `platform/pkg/closer` — graceful shutdown

Механизм корректного завершения работы с таймаутом. Регистрирует ресурсы для закрытия при завершении приложения.

### `platform/pkg/logger` — Zap logger wrapper

Обёртка над Zap для структурированного логирования с поддержкой JSON и console форматов.

## Настройка порога

Параметр `--threshold` контролирует строгость группировки:

| Значение | Эффект | Когда использовать |
|----------|--------|-------------------|
| `0.30-0.35` | Очень строгая группировка, минимум ложных совпадений | Когда на фото много разных людей |
| `0.40-0.45` | Строгая группировка, баланс точности | Для смешанных наборов фото |
| `0.50` | Сбалансированный (по умолчанию) | Универсальный вариант |
| `0.60-0.70` | Агрессивная группировка, больше ложных совпадений | Когда на фото один человек |

> **Важно:** Для ArcFace/InsightFace модели типичные значения косинусного сходства:
> - **Один человек:** 0.50-0.90
> - **Разные люди:** 0.00-0.40
>
> Если все фото с одного мероприятия (один человек), используйте порог **0.35-0.40**.

Рекомендуется начать с `0.50` и корректировать по результатам.

## Зависимости

| Пакет | Назначение |
|-------|-----------|
| `github.com/yalue/onnxruntime_go` | Go-биндинги для ONNX Runtime (CPU/CUDA inference) |
| `golang.org/x/image` | Обработка изображений на чистом Go (resize, decode/encode) |
| `gonum.org/v1/gonum` | BLAS-ускоренное матричное перемножение для кластеризации embeddings |

## Лицензия

MIT
