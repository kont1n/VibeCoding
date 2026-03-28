# Быстрый старт

## 1. Установка зависимостей

### ONNX Runtime GPU (автоматически при запуске)
Скрипт `run-gpu-simple.ps1` автоматически загрузит ONNX Runtime GPU при первом запуске.

### Модели InsightFace

**Вариант A: Через Python (рекомендуется)**
```powershell
py -m pip install huggingface_hub
py -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/det_10g.onnx', local_dir='./models')"
py -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/w600k_r50.onnx', local_dir='./models')"
```

**Вариант B: Вручную**
1. Скачайте с https://github.com/deepinsight/insightface/tree/master/model_zoo#buffalo_l
2. Поместите `det_10g.onnx` и `w600k_r50.onnx` в `./models/`

## 2. Сборка

```powershell
go build -o face-grouper.exe .
```

## 3. Запуск на GPU

### Базовый запуск
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\run-gpu-simple.ps1 -InputDir .\dataset -OutputDir .\output
```

### С веб-интерфейсом
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\run-gpu-simple.ps1 -Serve
```

### Прямой запуск (если ONNX Runtime уже загружен)
```powershell
.\face-grouper.exe --gpu --serve
```

## 4. Просмотр результатов

После обработки:
- Откройте браузер: http://localhost:8080
- Результаты в папке `./output/Person_N/`

## 5. Параметры

| Параметр | По умолчанию | Описание |
|----------|-------------|----------|
| `--input` | `./dataset` | Папка с фотографиями |
| `--output` | `./output` | Папка результатов |
| `--gpu` | ❌ | Использовать GPU |
| `--serve` | ❌ | Запустить веб-интерфейс |
| `--workers` | 4 | Количество воркеров |
| `--gpu-det-sessions` | 2 | Detector сессии на GPU |
| `--gpu-rec-sessions` | 2 | Recognizer сессии на GPU |
| `--embed-batch-size` | 64 | Размер батча распознавания |
| `--threshold` | 0.5 | Порог сходства лиц |

### Рекомендации по порогу (`--threshold`)

| Сценарий | Порог | Пример |
|----------|-------|--------|
| **Много разных людей** | 0.30-0.35 | Семейный альбом за годы |
| **Смешанный набор** | 0.40-0.45 | Корпоратив с сотрудниками |
| **Один человек** | 0.50-0.60 | Фото с одного мероприятия |

```powershell
# Пример: строгая группировка (много разных людей)
.\scripts\run-gpu-simple.ps1 -Threshold 0.35

# Пример: один человек (фото с мероприятия)
.\scripts\run-gpu-simple.ps1 -Threshold 0.55
```

### Пример тюнинга для RTX 4090/5090
```powershell
.\scripts\run-gpu-simple.ps1 -GpuDetSessions 4 -GpuRecSessions 4 -EmbedBatchSize 128 -Serve
```

## Troubleshooting

### "ONNX Runtime not found"
Скрипт `run-gpu-simple.ps1` загрузит его автоматически в `./runtime/`

### "CUDA DLLs missing"
```powershell
py -m pip install nvidia-cublas-cu12 nvidia-cuda-runtime-cu12 nvidia-cudnn-cu12 nvidia-cufft-cu12
```

### "Models not found"
См. раздел 1 выше — загрузите модели в `./models/`

### "No images found"
Поместите фотографии в `./dataset/` или укажите `--input <путь>`
