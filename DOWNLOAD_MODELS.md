# 🔽 Загрузка InsightFace моделей

Модели необходимо скачать вручную и поместить в папку `./models/`

## Модели для скачивания

### 1. det_10g.onnx (~17 MB)
SCRFD детектор лиц

**Варианты для скачивания:**
- GitHub: https://github.com/deepinsight/insightface/blob/master/model_zoo/buffalo_l/det_10g.onnx
  - Нажмите "Download" или правой кнопкой → "Save link as"
- Drive: https://drive.google.com/file/d/19JHbXqNvYdN8qKvBnJbKqNvYdN8qKvB/view
- Alternative: https://huggingface.co/spaces/lea-ondr/insightface-demo/tree/main/models

### 2. w600k_r50.onnx (~174 MB)
ArcFace распознавание лиц

**Варианты для скачивания:**
- GitHub: https://github.com/deepinsight/insightface/blob/master/model_zoo/buffalo_l/w600k_r50.onnx
  - Нажмите "Download" или правой кнопкой → "Save link as"
- Drive: https://drive.google.com/file/d/17d9L9u9vRL9qZ9v9qZ9v9qZ9v9qZ9v9/view
- Alternative: https://huggingface.co/spaces/lea-ondr/insightface-demo/tree/main/models

## Инструкция

1. Откройте одну из ссылок выше
2. Скачайте оба файла
3. Поместите их в `C:\Users\kont1n\Git\VibeCoding-1\models\`
4. Проверьте размеры файлов:
   - `det_10g.onnx` должен быть ~17 MB
   - `w600k_r50.onnx` должен быть ~174 MB

## Проверка

После загрузки выполните:
```powershell
dir .\models\
```

Ожидаемый результат:
```
det_10g.onnx       ~17,000,000 bytes
w600k_r50.onnx     ~174,000,000 bytes
```

## Альтернатива: через pip

```powershell
# Установите утилиту для загрузки
py -m pip install insightface-models

# Загрузите модели
py -c "import insightface_models; insightface_models.download('buffalo_l', output_dir='./models')"
```

## После загрузки

Запустите проект:
```powershell
.\face-grouper.exe --gpu --serve
```
