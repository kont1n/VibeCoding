# API Reference

Base URL: `http://localhost:8080`

Все ответы возвращаются в формате JSON. Content-Type: `application/json`.

---

## Оглавление

- [Health](#health)
- [Upload](#upload)
- [Sessions](#sessions)
- [Persons](#persons)
- [Errors](#errors)
- [Files](#files)
- [Report](#report)

---

## Health

### GET /health

Проверка состояния сервиса. Используется для мониторинга и Kubernetes liveness probe.

**Response 200:**
```json
{
  "status": "ok",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "db": "ok",
  "details": {}
}
```

**Response 503** (БД недоступна):
```json
{
  "status": "degraded",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "db": "unreachable",
  "details": {
    "db_error": "connection refused"
  }
}
```

### GET /ready

Kubernetes readiness probe. Возвращает 200, если сервис готов принимать трафик.

**Response 200:**
```json
{
  "status": "ready"
}
```

**Response 503:**
```json
{
  "status": "not ready",
  "reasons": ["database unreachable"]
}
```

---

## Upload

### POST /api/v1/upload

Загрузка изображений для обработки. Принимает отдельные файлы или `.zip`-архивы.

**Content-Type:** `multipart/form-data`
**Поле:** `files` (множественный выбор)

**Допустимые форматы:**
- Изображения: `.jpg`, `.jpeg`, `.png`, `.webp`
- Архивы: `.zip`

**Лимиты:**
- Максимальный размер загрузки: 500 MB
- Максимальный размер при распаковке zip: 2 GB

**Защита:**
- Валидация magic bytes (заголовков файлов)
- Защита от path traversal
- Защита от zip bomb и zip slip

**Response 200:**
```json
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "file_count": 42,
  "total_size": 125829120,
  "files": ["photo1.jpg", "photo2.png", "group.jpg"],
  "upload_path": "/app/.uploads/a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**Response 400:**
```json
{
  "error": "no valid image files found in upload"
}
```

**Пример (curl):**
```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -F "files=@photo1.jpg" \
  -F "files=@photo2.jpg"

# Или zip-архив:
curl -X POST http://localhost:8080/api/v1/upload \
  -F "files=@photos.zip"
```

---

## Sessions

### POST /api/v1/sessions/{id}/process

Запуск обработки загруженных файлов. Сессия обрабатывается асинхронно.

**Path Parameters:**
- `id` (string, required) — ID сессии из ответа upload

**Request Body:**
```json
{
  "input_dir": "/app/.uploads/a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**Response 202:**
```json
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "status": "processing"
}
```

**Response 409** (уже обрабатывается):
```json
{
  "error": "session is already processing"
}
```

### GET /api/v1/sessions/{id}/status

Текущий статус сессии обработки.

**Response 200:**
```json
{
  "session_id": "a1b2c3d4-...",
  "status": "processing",
  "stage": "extract",
  "progress": 0.45,
  "elapsed_ms": 12340,
  "error": ""
}
```

**Значения `status`:** `processing`, `completed`, `failed`

**Значения `stage`:** `starting`, `scan`, `extract`, `cluster`, `organize`

### GET /api/v1/sessions/{id}/stream

Server-Sent Events (SSE) стрим прогресса обработки. Обновляется каждые 500 мс.

**Headers:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**Формат событий:**
```
data: {"session_id":"...","stage":"extract","stage_label":"Обнаружение лиц...","progress":0.45,"processed_items":45,"total_items":100,"current_file":"IMG_001.jpg","done":false,"elapsed_ms":12340,"estimated_ms":27400,"eta_ms":15060}

data: {"session_id":"...","stage":"cluster","progress":0.75,"done":false,"elapsed_ms":20000}

data: {"session_id":"...","stage":"organize","progress":1.0,"done":true,"elapsed_ms":27400}
```

**Поля ProgressEvent:**

| Поле | Тип | Описание |
|------|-----|----------|
| `session_id` | string | ID сессии |
| `stage` | string | Текущий этап |
| `stage_label` | string | Человекочитаемое название этапа |
| `progress` | float | Прогресс 0.0 — 1.0 |
| `processed_items` | int | Обработано элементов |
| `total_items` | int | Всего элементов |
| `current_file` | string | Текущий обрабатываемый файл |
| `error` | string | Ошибка (пусто, если нет) |
| `done` | bool | Обработка завершена |
| `elapsed_ms` | int64 | Прошедшее время (мс) |
| `estimated_ms` | int64 | Оценка общего времени (мс) |
| `eta_ms` | int64 | Оставшееся время (мс) |

**Этапы пайплайна:**

| Этап | Вес | Описание |
|------|-----|----------|
| `scan` | 25% | Сканирование входной директории |
| `extract` | 25% | Детекция лиц и извлечение эмбеддингов |
| `cluster` | 25% | Кластеризация по персонам |
| `organize` | 25% | Организация результатов, генерация аватаров |

**Пример (JavaScript):**
```javascript
const evtSource = new EventSource('/api/v1/sessions/SESSION_ID/stream');
evtSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(`${data.stage}: ${(data.progress * 100).toFixed(0)}%`);
  if (data.done) evtSource.close();
};
```

### POST /api/v1/sessions/{id}/cancel

Отмена обработки.

**Response 200:**
```json
{
  "session_id": "a1b2c3d4-...",
  "status": "canceled"
}
```

**Response 409** (уже завершена):
```json
{
  "error": "session already finished"
}
```

---

## Persons

### GET /api/v1/persons

Список персон с пагинацией.

**Query Parameters:**

| Параметр | Тип | По умолчанию | Описание |
|----------|-----|-------------|----------|
| `offset` | int | 0 | Смещение |
| `limit` | int | 50 | Количество (max 100) |

**Response 200:**
```json
{
  "persons": [
    {
      "id": "uuid",
      "name": "Person_1",
      "custom_name": "",
      "avatar_path": "output/Person_1/avatar.jpg",
      "avatar_thumbnail_path": "output/.thumbnails/person_1.jpg",
      "quality_score": 0.87,
      "face_count": 15,
      "photo_count": 12,
      "metadata": null,
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    }
  ],
  "total": 18,
  "offset": 0,
  "limit": 50
}
```

> **Fallback:** Если БД недоступна, данные загружаются из `report.json`.

### GET /api/v1/persons/{id}

Получение персоны по ID.

**Path Parameters:**
- `id` — UUID персоны (при использовании БД) или числовой ID (при fallback на report.json)

**Response 200:**
```json
{
  "id": "uuid",
  "name": "Person_1",
  "custom_name": "Алексей",
  "avatar_path": "output/Person_1/avatar.jpg",
  "quality_score": 0.87,
  "face_count": 15,
  "photo_count": 12
}
```

### PUT /api/v1/persons/{id}

Переименование персоны. Требует подключения к БД.

**Request Body:**
```json
{
  "name": "Алексей Иванов"
}
```

**Ограничения:**
- Максимальная длина: 200 символов
- HTML-сущности экранируются (защита от XSS)

**Response 200:** Обновлённый объект персоны.

**Response 503:**
```json
{
  "error": "database required for rename operation"
}
```

### GET /api/v1/persons/{id}/photos

Фотографии персоны с пагинацией.

**Query Parameters:**

| Параметр | Тип | По умолчанию | Описание |
|----------|-----|-------------|----------|
| `offset` | int | 0 | Смещение |
| `limit` | int | 50 | Количество (max 100) |

**Response 200:**
```json
{
  "photos": [
    {
      "id": "uuid",
      "path": "output/Person_1/photo.jpg",
      "original_path": "dataset/IMG_001.jpg",
      "width": 4032,
      "height": 3024,
      "file_size": 3145728,
      "mime_type": "image/jpeg"
    }
  ],
  "total": 12,
  "offset": 0,
  "limit": 50
}
```

### GET /api/v1/persons/{id}/relations

Граф связей персоны с другими. Требует подключения к БД.

**Query Parameters:**

| Параметр | Тип | По умолчанию | Описание |
|----------|-----|-------------|----------|
| `min_similarity` | float | 0.0 | Минимальный порог сходства для фильтрации |

**Response 200:**
```json
{
  "person_id": "uuid",
  "relations": [
    {
      "person1_id": "uuid-1",
      "person2_id": "uuid-2",
      "similarity": 0.82,
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "nodes": [
    {
      "id": "uuid-1",
      "name": "Person_1",
      "custom_name": "Алексей",
      "avatar_path": "output/Person_1/avatar.jpg",
      "face_count": 15,
      "photo_count": 12,
      "connected_person_ids": ["uuid-2", "uuid-3"],
      "connection_similarities": [0.82, 0.65]
    }
  ]
}
```

---

## Errors

### GET /api/v1/sessions/{id}/errors

Список ошибок обработки.

**Response 200:**
```json
{
  "errors": [
    {
      "file": "broken.jpg",
      "error": "cannot read image: broken.jpg: image: unknown format",
      "error_type": "unsupported_format"
    },
    {
      "file": "landscape.jpg",
      "error": "no faces found",
      "error_type": "no_face"
    }
  ],
  "total": 2
}
```

**Типы ошибок:**

| Тип | Описание |
|-----|----------|
| `no_face` | Лица не обнаружены на изображении |
| `unsupported_format` | Неподдерживаемый или повреждённый формат |
| `processing_error` | Общая ошибка обработки |

---

## Files

### GET /output/{path}

Раздача файлов из директории результатов. Допускаются только изображения.

**Разрешённые расширения:** `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`, `.bmp`, `.svg`

**Защита:**
- Path traversal prevention (filepath.Clean + prefix check)
- Symlink escape prevention
- Directory listing запрещён
- Недопустимые расширения → 403 Forbidden

---

## Report

### GET /api/report

Полный JSON-отчёт о последней обработке.

**Response 200:** Содержимое файла `report.json`.

**Response 404:**
```json
{
  "error": "report not found"
}
```

---

## Middleware

Все запросы проходят через цепочку middleware:

| Middleware | Описание |
|-----------|----------|
| Recovery | Перехват panic → 500 Internal Server Error |
| Rate Limiter | 100 RPS / 200 burst per IP |
| MaxBodySize | Ограничение тела запроса: 500 MB |
| CORS | Same-origin по умолчанию |
| Request Logger | Логирование запросов (кроме `/` и `/health`) |

**Rate Limiting:**
- Алгоритм: Token Bucket (golang.org/x/time/rate)
- Per-IP (по `RemoteAddr`, без учёта `X-Forwarded-For`)
- Очистка неактивных лимитеров: каждые 5 минут (idle > 3 мин)
- При превышении: `429 Too Many Requests`
