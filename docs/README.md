# Face Grouper - Documentation

Welcome to the Face Grouper documentation hub.

## Documentation Index

### Getting Started

| Document | Description |
|----------|-------------|
| [Quick Start](QUICKSTART.md) | Запуск за 5 минут |
| [Download Models](DOWNLOAD_MODELS.md) | Загрузка InsightFace моделей |
| [README](../README.md) | Основная документация проекта |

### Deployment

| Document | Description |
|----------|-------------|
| [Docker Guide](DOCKER.md) | Полное руководство по Docker |
| [Database Guide](DATABASE.md) | PostgreSQL + pgvector интеграция |

---

## Quick Links

**Новый пользователь?**
1. Начните с [Quick Start](QUICKSTART.md)
2. Загрузите модели по [инструкции](DOWNLOAD_MODELS.md)
3. Запустите обработку

**Docker?**
- [Docker Guide](DOCKER.md) — CPU, NVIDIA GPU, AMD ROCm

**PostgreSQL?**
- [Database Guide](DATABASE.md) — опциональная интеграция, векторный поиск

---

## Quick Reference

### Основные команды

```bash
task build           # Сборка
task test            # Тесты
task lint            # Линтинг
task docker:run      # Docker запуск
task benchmark       # Бенчмарки
```

### Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|-------------|----------|
| `INPUT_DIR` | `./dataset` | Директория с фотографиями |
| `OUTPUT_DIR` | `./output` | Директория результатов |
| `MODELS_DIR` | `./models` | Директория с моделями |
| `GPU_ENABLED` | `0` | Включить GPU (1/0) |
| `EXTRACT_WORKERS` | `4` | Количество воркеров |
| `CLUSTER_THRESHOLD` | `0.5` | Порог кластеризации |
| `WEB_SERVE` | `false` | Запускать веб-интерфейс |
| `WEB_PORT` | `8080` | Порт веб-сервера |
| `DB_HOST` | — | PostgreSQL хост (опционально) |

### Структура документации

```
docs/
├── README.md              # Этот файл
├── QUICKSTART.md          # Быстрый старт
├── DOWNLOAD_MODELS.md     # Загрузка моделей
├── DOCKER.md              # Docker руководство
└── DATABASE.md            # PostgreSQL интеграция
```

---

## Support

- **Issues:** [GitHub Issues](https://github.com/kont1n/face-grouper/issues)
- **Discussions:** [GitHub Discussions](https://github.com/kont1n/face-grouper/discussions)
