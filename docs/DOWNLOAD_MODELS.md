# Загрузка InsightFace моделей

Модели необходимо скачать и поместить в папку `./models/`.

## Модели

| Файл | Размер | Назначение |
|------|--------|-----------|
| `det_10g.onnx` | ~17 MB | SCRFD детектор лиц |
| `w600k_r50.onnx` | ~174 MB | ArcFace распознавание лиц |

Оба файла из пакета `buffalo_l` проекта [InsightFace](https://github.com/deepinsight/insightface).

## Способ 1: Через скрипт (рекомендуется)

```bash
python scripts/download_models.py
```

## Способ 2: Через Python huggingface_hub

```bash
pip install huggingface_hub

python -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/det_10g.onnx', local_dir='./models')"
python -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/w600k_r50.onnx', local_dir='./models')"
```

Windows (PowerShell):
```powershell
py -m pip install huggingface_hub
py -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/det_10g.onnx', local_dir='./models')"
py -c "from huggingface_hub import hf_hub_download; hf_hub_download('deepinsight/insightface', 'buffalo_l/w600k_r50.onnx', local_dir='./models')"
```

## Способ 3: Вручную через браузер

1. Откройте [InsightFace model zoo](https://github.com/deepinsight/insightface/tree/master/model_zoo#buffalo_l)
2. Скачайте `det_10g.onnx` и `w600k_r50.onnx`
3. Поместите оба файла в `./models/`

## Проверка

```bash
ls -lh ./models/
```

Ожидаемый результат:
```
det_10g.onnx      ~17M
w600k_r50.onnx   ~174M
```

## После загрузки

```bash
# Linux / macOS
./face-grouper --serve

# Windows
.\face-grouper.exe --serve
```
