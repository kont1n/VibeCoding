# 📋 План реализации: Face Grouper v2.0

> **Дата:** 2026-04-06
> **Цель:** Интеграция БД, визуализация графа, bbox overlay, современный UI

---

## 📊 Аудит текущего состояния

| Компонент | Статус | Комментарий |
|-----------|--------|-------------|
| **ML Pipeline** (SCRFD + ArcFace) | ✅ Готово | CPU/GPU (CUDA/ROCm/DirectML), pool сессий |
| **Clustering** (Union-Find + BLAS) | ✅ Готово | Two-stage, ambiguity gate, pruning |
| **PostgreSQL + pgvector** | ⚠️ Частично | Схема готова, но pipeline **не пишет в БД** |
| **Organizer** (Person_N dirs) | ✅ Готово | Symlinks, avatars, thumbnails |
| **Web UI** (SPA) | ⚠️ Частично | Работает, но читает из `report.json`, fallback-логика |
| **API endpoints** | ⚠️ Частично | Persons/Photos работают, Relations требует БД |
| **Graph визуализация** | ⚠️ Базовая | Co-occurrence из report.json, нет слайдера, нет глобального графа |
| **Bbox overlay** | ❌ Отсутствует | Нет в API, нет во фронтенде |
| **Redis** | ❌ Не используется | Конфиг есть, клиент не подключён |

### 🔑 Ключевая проблема

**Pipeline обработки (scan → extract → cluster → organize → report) работает БЕЗ записи в PostgreSQL.** Все данные живут только в `report.json` и файловой системе. БД инициализируется, но используется только для чтения персон (если есть) и relations endpoint.

---

## 🎯 Цели

- [x] Подготовить надежное хранилище для персон, фото и связей
- [x] Интегрировать БД и логику графа, сохраняя типобезопасность
- [x] Визуализация галереи и графа связей
- [x] Убрать лишние зависимости и настроить конфиг
- [x] Обеспечить возможность работы как только на CPU, так и на CPU + GPU
- [x] Реализовать красивый современный UI

---

## ЭТАП 1: Database Integration (Фундамент)

### 1.1. Pipeline → БД: Запись результатов обработки

**Файлы:**
- `internal/app/pipeline_steps.go`
- `internal/app/pipeline.go`
- `internal/service/persist/` (новый пакет)

**Задача:** После кластеризации сохранять персоны, фото, лица и связи в PostgreSQL.

**Изменения:**
- Добавить `saveToDB()` шаг после `cluster` и до `organize`
- Создать сервис `internal/service/persist/` для маппинга кластеров → БД
- Сохранять `persons`, `photos`, `faces`, `person_relations` батчами
- Генерировать UUID для каждого Person/Photo/Face и сохранять маппинг `int_id → uuid`

**Новые структуры:**
```
internal/service/persist/
  ├── service.go       // Основной сервис сохранения
  └── mapper.go        // Маппинг model.Cluster → model.Person/Face/Photo
```

**Детали реализации:**
- `PersistService` принимает `[]model.Cluster`, извлекает уникальные фото, создаёт `Photo` записи
- Для каждого кластера создаётся `Person` с авто-именем `Person_N`
- Для каждого лица создаётся `Face` с embedding (pgvector), bbox, keypoints, quality score
- Для каждой пары персон с общими фото вычисляется similarity → `person_relations`
- Маппинг `int(cluster_id) → uuid(person_id)` сохраняется для Organizer

### 1.2. Расширить модель Photo для хранения faces

**Файлы:**
- `internal/model/database.go`
- `internal/repository/postgres/photo.go`

**Задача:** API `GET /api/v1/persons/{id}/photos` должен возвращать фото с bbox всех лиц.

**Новые структуры:**
```go
type PhotoWithFaces struct {
    Photo
    Faces []FaceInfo `json:"faces"`
}

type FaceInfo struct {
    PersonID       uuid.UUID `json:"person_id"`
    PersonName     string    `json:"person_name"`
    IsThisPerson   bool      `json:"is_this_person"`
    BBox           BBox      `json:"bbox"`
    Confidence     float32   `json:"confidence"`
    QualityScore   float32   `json:"quality_score"`
}
```

**Изменения:**
- Новый метод `ListByPersonWithFaces()` в PhotoRepository
- SQL JOIN `photos → faces → persons` для получения всех лиц на каждом фото
- Расширить `PersonHandler.Photos()` для поддержки нового формата

### 1.3. Global Graph endpoint

**Файлы:**
- `internal/api/http/handler/graph.go` (новый)
- `internal/web/server.go`

**Задача:** Новый endpoint `GET /api/v1/graph` для визуализации полного графа связей.

**Response:**
```json
{
  "nodes": [
    {
      "id": "uuid",
      "name": "Person 1",
      "custom_name": "Иван",
      "avatar": "avatars/Person_1_abc123.jpg",
      "photo_count": 45,
      "face_count": 98
    }
  ],
  "links": [
    {
      "source": "uuid-1",
      "target": "uuid-2",
      "similarity": 0.72,
      "shared_photos": 18
    }
  ],
  "stats": {
    "total_nodes": 87,
    "total_links": 142,
    "clusters": 5,
    "strongest_link": {
      "person1_name": "Иван",
      "person2_name": "Мария",
      "similarity": 0.72
    }
  }
}
```

**Fallback:** Если БД недоступна — генерировать граф из `report.json` (co-occurrence по общим фото).

---

## ЭТАП 2: Frontend — Bbox Overlay и Photo Detail

### 2.1. Bbox overlay на фото

**Файлы:** `internal/web/index.html`

**Задача:** На странице персоны показывать bbox всех лиц на каждом фото.

**Реализация:**
- SVG overlay поверх `<img>` с относительными координатами (%)
- Основной персон — solid border (`#e94560`, stroke-width: 3)
- Другие персоны — dashed border (`#4ecdc4`, stroke-width: 2, stroke-dasharray: 5,5)
- Label с именем над каждым bbox
- Tooltip при hover на bbox с именем и confidence

**CSS:**
```css
.photo-with-faces {
  position: relative;
  display: inline-block;
}
.photo-with-faces svg {
  position: absolute;
  top: 0; left: 0;
  width: 100%; height: 100%;
  pointer-events: none;
}
.face-bbox-primary {
  fill: none;
  stroke: #e94560;
  stroke-width: 3;
}
.face-bbox-secondary {
  fill: none;
  stroke: #4ecdc4;
  stroke-width: 2;
  stroke-dasharray: 5,5;
}
.face-label {
  font-size: 12px;
  font-weight: bold;
}
```

**JS:**
```javascript
function renderPhotoWithBbox(photo, faces, container) {
  const wrapper = document.createElement('div');
  wrapper.className = 'photo-with-faces';

  const img = document.createElement('img');
  img.src = photo.url;
  img.style.width = '100%';
  wrapper.appendChild(img);

  if (faces && faces.length > 0) {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    faces.forEach(face => {
      const rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
      rect.setAttribute('x', `${face.bbox.x1 * 100}%`);
      rect.setAttribute('y', `${face.bbox.y1 * 100}%`);
      rect.setAttribute('width', `${(face.bbox.x2 - face.bbox.x1) * 100}%`);
      rect.setAttribute('height', `${(face.bbox.y2 - face.bbox.y1) * 100}%`);
      rect.setAttribute('class', face.is_this_person ? 'face-bbox-primary' : 'face-bbox-secondary');
      svg.appendChild(rect);

      const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      text.setAttribute('x', `${face.bbox.x1 * 100}%`);
      text.setAttribute('y', `${(face.bbox.y1 - 0.02) * 100}%`);
      text.setAttribute('fill', face.is_this_person ? '#e94560' : '#4ecdc4');
      text.setAttribute('class', 'face-label');
      text.textContent = face.person_name;
      svg.appendChild(text);
    });
    wrapper.appendChild(svg);
  }

  container.appendChild(wrapper);
}
```

### 2.2. Улучшение страницы персоны

**Файлы:** `internal/web/index.html`

**Изменения:**
- Показывать `width × height` фото в meta
- Badge с количеством лиц на каждом фото
- Кнопка "Найти похожих" (placeholder на будущее)
- Показывать EXIF дату съёмки (если доступна)

---

## ЭТАП 3: Frontend — Graph Visualization

### 3.1. Слайдер «Минимальная сила связи»

**Файлы:** `internal/web/index.html`

**Задача:** Фильтрация рёбер графа по порогу similarity.

**HTML:**
```html
<div class="graph-controls">
  <label>
    Мин. связь:
    <input type="range" id="graph-threshold" min="0" max="1" step="0.01" value="0">
    <span id="graph-threshold-value">0.00</span>
  </label>
</div>
```

**Реализация:**
- При изменении слайдера — фильтрация `links` и перезапуск D3 force simulation
- Отображение текущего значения рядом со слайдером
- Debounce 100ms для плавности

### 3.2. Отдельная страница «Общий граф»

**Файлы:** `internal/web/index.html`

**Навигация:** Добавить кнопку «Граф» в header nav.

**Элементы страницы:**
- D3 force simulation с zoom/pan (d3.zoom)
- Поиск персоны (highlight узла с анимацией)
- Слайдер минимальной связи
- Статистика графа:
  - Количество узлов
  - Количество связей
  - Количество кластеров (connected components)
  - Самая сильная связь
  - Чаще всего вместе (top-3 по shared_photos)
- Клик по узлу → переход на страницу персоны
- Cluster highlighting (разные цвета для кластеров)
- Export graph в PNG/SVG

**CSS:**
```css
.graph-page-controls {
  display: flex;
  gap: 1.5rem;
  align-items: center;
  margin-bottom: 1rem;
  padding: 0.8rem 1rem;
  background: var(--bg-card);
  border-radius: var(--radius);
  border: 1px solid var(--border-light);
}

.graph-canvas-container {
  background: var(--bg-card);
  border-radius: var(--radius);
  border: 1px solid var(--border-light);
  min-height: 600px;
  position: relative;
}

.graph-stats {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 1rem;
  margin-top: 1.5rem;
}
```

---

## ЭТАП 4: Frontend — Gallery Improvements

### 4.1. Поиск и сортировка в галерее

**Файлы:** `internal/web/index.html`

**HTML:**
```html
<div class="gallery-controls">
  <div class="search-box">
    <input type="text" id="gallery-search" placeholder="Поиск по имени...">
  </div>
  <select id="gallery-sort">
    <option value="photos-desc">По кол-ву фото ↓</option>
    <option value="photos-asc">По кол-ву фото ↑</option>
    <option value="name-asc">По имени А-Я</option>
    <option value="name-desc">По имени Я-А</option>
    <option value="quality-desc">По качеству ↓</option>
  </select>
  <div class="filter-range">
    <label>Фото: от <input type="number" id="filter-min" value="1" min="1"> до <input type="number" id="filter-max" value="1000" min="1"></label>
  </div>
</div>
```

**JS:**
- Поле поиска с debounce 300ms (фильтрация по `custom_name` / `name`)
- Сортировка: по имени, по кол-ву фото (asc/desc), по качеству
- Фильтр по диапазону кол-ва фото (min-max)

### 4.2. Skeleton loaders и page transitions

**Файлы:** `internal/web/index.html`

**CSS Skeleton:**
```css
@keyframes shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

.skeleton {
  background: linear-gradient(
    90deg,
    var(--bg-tertiary) 25%,
    rgba(255, 255, 255, 0.08) 50%,
    var(--bg-tertiary) 75%
  );
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}

.skeleton-circle {
  width: 100%;
  aspect-ratio: 1;
  border-radius: 50%;
}
```

**CSS Page Transitions:**
```css
.page {
  opacity: 0;
  transform: translateY(20px);
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.page.active {
  opacity: 1;
  transform: translateY(0);
}
```

### 4.3. Pagination / Infinite scroll

**Файлы:** `internal/web/index.html`

**Задача:** При 200+ персонах не рендерить все карточки сразу.

**Реализация:**
- IntersectionObserver для подгрузки порциями по 50 карточек
- Кнопка «Загрузить ещё» или автоматическая подгрузка
- Показывать `loading...` skeleton при подгрузке

---

## ЭТАП 5: Дизайн и UX

### 5.1. Glassmorphism и визуальные улучшения

**Файлы:** `internal/web/index.html`

**CSS переменные (обновлённые):**
```css
:root {
  --bg: #0a0a0f;
  --bg-card: #12121a;
  --bg-glass: rgba(18, 18, 26, 0.7);
  --bg-header: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
  --accent: #e94560;
  --accent-soft: rgba(233, 69, 96, 0.12);
  --text: #e0e0e0;
  --text-muted: #777;
  --text-dim: #555;
  --border: rgba(255,255,255,0.06);
  --border-light: rgba(255,255,255,0.04);
  --radius: 12px;
  --shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.3);
  --shadow-md: 0 4px 16px rgba(0, 0, 0, 0.4);
  --shadow-lg: 0 8px 32px rgba(0, 0, 0, 0.5);
  --shadow-glow: 0 0 20px rgba(233, 69, 96, 0.15);
  --blur-sm: blur(4px);
  --blur-md: blur(8px);
  --blur-lg: blur(16px);
}
```

**Glassmorphism Card:**
```css
.card-glass {
  background: var(--bg-glass);
  backdrop-filter: var(--blur-md);
  -webkit-backdrop-filter: var(--blur-md);
  border: 1px solid var(--border-light);
  border-radius: 12px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.card-glass:hover {
  background: rgba(18, 18, 26, 0.85);
  border-color: rgba(233, 69, 96, 0.3);
  box-shadow: var(--shadow-glow);
  transform: translateY(-4px);
}
```

**Улучшенные кнопки:**
```css
.btn-primary {
  background: linear-gradient(135deg, var(--accent), #ff6b81);
  color: white;
  padding: 0.75rem 2rem;
  border: none;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: var(--shadow-sm);
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md), var(--shadow-glow);
}
```

### 5.2. Улучшение Upload страницы и Processing

**Файлы:** `internal/web/index.html`

**Upload:**
- Улучшить визуал upload зоны (gradient border animation)
- Skeleton для списка файлов

**Processing:**
- Step-by-step индикатор прогресса (5 этапов: Scan, Extract, Cluster, Organize, Report)
- Анимация иконок для каждого этапа
- Показ скорости (фото/сек) в реальном времени
- Показывать найденные лица в реальном времени

---

## ЭТАП 6: Конфигурация и зависимости

### 6.1. Убрать неиспользуемые зависимости

**Файлы:** `go.mod`, `deploy/env/.env.example`, `deploy/compose/docker-compose.yml`

**Задача:**
- Redis конфиг есть, но не используется → пометить как TODO или убрать из compose
- Проверить все indirect зависимости через `go mod tidy`

### 6.2. CPU/GPU конфигурация

**Файлы:** `internal/config/env/config.go`, `deploy/env/.env.example`

**Текущее состояние:** Уже реализовано корректно ✅

- `GPU_ENABLED`, `FORCE_CPU`, `PROVIDER_PRIORITY` — всё есть
- `GPUDetSessions`, `GPURecSessions` — настраиваемые
- Авто-Fallback между провайдерами
- Docker: CPU, NVIDIA GPU, AMD ROCm сборки

**Никаких изменений не требуется**

---

## 📁 Структура новых файлов

```
internal/
├── service/
│   └── persist/                    # NEW: сохранение результатов в БД
│       ├── service.go              # PersistService
│       └── mapper.go               # Cluster → Person/Face/Photo маппинг
├── api/http/handler/
│   └── graph.go                    # NEW: GET /api/v1/graph handler
```

## 📋 Чек-лист задач

### ЭТАП 1: Database Integration

| # | Задача | Файлы | Приоритет |
|---|--------|-------|-----------|
| 1.1 | `persist/service.go` — сохранение в БД | `internal/service/persist/` | 🔴 P0 |
| 1.2 | `ListByPersonWithFaces` — фото с bbox | `internal/repository/postgres/photo.go` | 🔴 P0 |
| 1.3 | `graph.go` — endpoint глобального графа | `internal/api/http/handler/graph.go` | 🔴 P0 |
| 1.4 | Pipeline integration — вызов persist | `internal/app/pipeline.go` | 🔴 P0 |

### ЭТАП 2: Bbox Overlay

| # | Задача | Файлы | Приоритет |
|---|--------|-------|-----------|
| 2.1 | Bbox overlay на фото (SVG) | `internal/web/index.html` | 🔴 P0 |
| 2.2 | Улучшение страницы персоны | `internal/web/index.html` | 🟡 P1 |

### ЭТАП 3: Graph Visualization

| # | Задача | Файлы | Приоритет |
|---|--------|-------|-----------|
| 3.1 | Слайдер «Минимальная сила связи» | `internal/web/index.html` | 🔴 P0 |
| 3.2 | Страница «Общий граф» | `internal/web/index.html` | 🔴 P0 |

### ЭТАП 4: Gallery Improvements

| # | Задача | Файлы | Приоритет |
|---|--------|-------|-----------|
| 4.1 | Поиск и сортировка в галерее | `internal/web/index.html` | 🟡 P1 |
| 4.2 | Skeleton loaders + page transitions | `internal/web/index.html` | 🟡 P1 |
| 4.3 | Pagination / Infinite scroll | `internal/web/index.html` | 🟡 P1 |

### ЭТАП 5: Дизайн и UX

| # | Задача | Файлы | Приоритет |
|---|--------|-------|-----------|
| 5.1 | Glassmorphism + визуал | `internal/web/index.html` | 🟡 P1 |
| 5.2 | Step-by-step processing | `internal/web/index.html` | 🟢 P2 |

### ЭТАП 6: Cleanup

| # | Задача | Файлы | Приоритет |
|---|--------|-------|-----------|
| 6.1 | Убрать неиспользуемые зависимости | `go.mod`, `docker-compose.yml` | 🟢 P2 |

---

## 🧪 Тестирование

| # | Задача | Тип |
|---|--------|-----|
| T1 | Unit тесты для `persist/service.go` | Unit |
| T2 | Integration тесты для `graph.go` API | Integration |
| T3 | Unit тесты для маппинга Cluster → DB | Unit |
| T4 | Тесты bbox координат (edge cases) | Unit |
| T5 | E2E тест pipeline (CPU only) | E2E |

---

## 📝 Заметки

- Pipeline должен работать **и с БД, и без неё** (fallback на report.json)
- Все новые endpoint'ы должны поддерживать fallback на report.json
- UI должен быть полностью responsive (mobile-friendly)
- Сохранить backward compatibility с существующими `report.json`
