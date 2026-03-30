# Быстрый старт

## 1. Требования

- Go 1.24+
- ONNX Runtime shared library
- InsightFace модели (`det_10g.onnx`, `w600k_r50.onnx`)

## 2. Загрузка моделей InsightFace

```bash
pip install huggingface_hub
python -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/det_10g.onnx', local_dir='./models')"
python -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/w600k_r50.onnx', local_dir='./models')"
```

Или через скрипт:
```bash
python scripts/download_models.py
```

## 3. ONNX Runtime

Скачайте с [github.com/microsoft/onnxruntime/releases](https://github.com/microsoft/onnxruntime/releases) и поместите в корень проекта:

| ОС | Файл | Архив |
|----|------|-------|
| Linux | `libonnxruntime.so` | `onnxruntime-linux-x64-*.tgz` |
| macOS | `libonnxruntime.dylib` | `onnxruntime-osx-*.tgz` |
| Windows (CPU) | `onnxruntime.dll` | `onnxruntime-win-x64-*.zip` |
| Windows (GPU) | `onnxruntime.dll` | `onnxruntime-win-x64-gpu-*.zip` |

## 4. Сборка

```bash
# Linux / macOS
go build -o face-grouper ./cmd

# Windows
go build -o face-grouper.exe ./cmd
```

## 5. Добавьте фотографии

```bash
cp /path/to/your/photos/* ./dataset/
```

## 6. Запуск

### CPU (базовый)

```bash
# Linux / macOS
./face-grouper --serve

# Windows
.\face-grouper.exe --serve
```

### GPU (NVIDIA CUDA)

В `.env` или через флаг:
```bash
./face-grouper --gpu --serve
```

### Просмотр результатов

```bash
./face-grouper --view
```

Откройте браузер: http://localhost:8080

## 7. Параметры

| Параметр | По умолчанию | Описание |
|----------|-------------|----------|
| `--input` | `./dataset` | Папка с фотографиями |
| `--output` | `./output` | Папка результатов |
| `--gpu` | `false` | Использовать GPU |
| `--serve` | `false` | Запустить веб-интерфейс |
| `--view` | `false` | Только просмотр (без обработки) |
| `--workers` | `4` | Количество воркеров (CPU) |
| `--gpu-det-sessions` | `2` | Detector сессии на GPU |
| `--gpu-rec-sessions` | `2` | Recognizer сессии на GPU |
| `--embed-batch-size` | `64` | Размер батча распознавания |
| `--threshold` | `0.5` | Порог сходства лиц (0.0–1.0) |
| `--det-thresh` | `0.5` | Порог детекции лиц |
| `--max-dim` | `1920` | Макс. размер изображения (0 = без ресайза) |

## 8. REST API (при запуске с --serve)

Помимо веб-интерфейса, сервер предоставляет REST API:

```bash
# Запуск асинхронной обработки
curl -X POST http://localhost:8080/api/v1/sessions/my-job/process \
  -H "Content-Type: application/json" \
  -d '{"input_dir": "./dataset"}'

# Статус обработки
curl http://localhost:8080/api/v1/sessions/my-job/status

# SSE прогресс
curl -N http://localhost:8080/api/v1/sessions/my-job/stream

# Список персон (с пагинацией)
curl "http://localhost:8080/api/v1/persons?offset=0&limit=20"

# Health check
curl http://localhost:8080/health
```

## Рекомендации по порогу (`--threshold`)

| Сценарий | Порог | Пример |
|----------|-------|--------|
| **Много разных людей** | 0.30-0.35 | Семейный альбом за годы |
| **Смешанный набор** | 0.40-0.45 | Корпоратив с сотрудниками |
| **По умолчанию** | 0.50 | Универсальный вариант |
| **Один человек** | 0.55-0.60 | Фото с одного мероприятия |

## Настройка для GPU (RTX 4090/5090)

```bash
./face-grouper --gpu --gpu-det-sessions 4 --gpu-rec-sessions 4 --embed-batch-size 128 --serve
```

## Troubleshooting

### "ONNX Runtime not found"

Убедитесь, что `libonnxruntime.so` / `onnxruntime.dll` находится в корне проекта или в `PATH`/`LD_LIBRARY_PATH`.

### "Models not found"

Проверьте наличие файлов:
```bash
ls -la ./models/
# Ожидается:
# det_10g.onnx    ~17 MB
# w600k_r50.onnx  ~174 MB
```

### "No images found"

Поместите фотографии (`.jpg`, `.jpeg`, `.png`) в `./dataset/` или укажите `--input <путь>`.

### Ошибки CUDA (Windows)

```powershell
pip install nvidia-cublas-cu12 nvidia-cuda-runtime-cu12 nvidia-cudnn-cu12 nvidia-cufft-cu12
```
