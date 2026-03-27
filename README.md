# Face Grouping Service

Go-сервис для анализа фотографий: находит лица при помощи InsightFace и группирует фото по людям.

## Архитектура

- **Go CLI** — оркестрация: сканирование фото, вызов Python, кластеризация, создание symlinks
- **Python (InsightFace)** — детекция лиц и извлечение 512-мерных face embeddings
- **Кластеризация** — Union-Find по косинусному сходству embeddings

## Требования

- Go 1.21+
- Python 3.10+
- Права на создание символических ссылок (Windows: запуск от имени администратора или Developer Mode)

## Установка

```bash
# Go-зависимости
go build -o face-grouper.exe .

# Python-зависимости
pip install -r scripts/requirements.txt
```

При первом запуске InsightFace автоматически скачает модель `buffalo_l` (~300 MB).

## Запуск

```bash
./face-grouper.exe --input ./dataset --output ./output
```

### Флаги

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `--input` | `./dataset` | Директория с фотографиями |
| `--output` | `./output` | Директория для результатов |
| `--workers` | `4` | Количество параллельных воркеров |
| `--threshold` | `0.5` | Порог косинусного сходства (0.0–1.0) |
| `--python` | `python` | Путь к Python-интерпретатору |

## Результат

В директории `output/` создаются папки `Person_1/`, `Person_2/`, ... с символическими ссылками на оригинальные фотографии. Если на фото несколько людей — оно появится в нескольких папках.

## Структура проекта

```
├── main.go                         # точка входа, CLI
├── internal/
│   ├── models/models.go            # общие типы данных
│   ├── scanner/scanner.go          # сканирование директории
│   ├── extractor/extractor.go      # вызов Python, worker pool
│   ├── clustering/clustering.go    # Union-Find + cosine similarity
│   └── organizer/organizer.go      # создание папок и symlinks
├── scripts/
│   ├── extract_faces.py            # InsightFace: детекция + embeddings
│   └── requirements.txt            # Python-зависимости
└── dataset/                        # исходные фотографии
```
