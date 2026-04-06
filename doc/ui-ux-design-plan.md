# 🎨 UI/UX Design Plan — Face Grouper v3

## 📊 Текущее состояние (Audit)

### ✅ Что уже работает хорошо
- **SPA Router** — навигация между страницами без перезагрузки
- **Upload Page** — drag-and-drop, превью файлов, валидация
- **Processing Page** — SSE streaming, прогресс-бар, ETA, отмена
- **Gallery Page** — сетка карточек, lazy loading через IntersectionObserver
- **Person Detail Page** — аватар, имя (редактируемое), сетка фото
- **Relations Graph** — D3.js force simulation, drag-and-drop, tooltips
- **Errors Page** — список ошибок с превью и классификацией
- **Modal Viewer** — полноэкранный просмотр, zoom (wheel/+/-), навигация (стрелки), keyboard shortcuts
- **Тёмная тема** — CSS variables, responsive design

### 🔴 Критические проблемы
1. **Граф НЕ показывает bbox на фото** — только аватары персон
2. **Нет слайдера "Минимальная сила связи"** — показываются все связи
3. **Нет отдельной страницы "Общий граф"** — только на странице персоны
4. **Нет skeleton loaders** — страница пустая во время загрузки
5. **Нет фильтров/сортировки в галерее** — невозможно найти конкретного человека
6. **Нет анимаций переходов** — резкое переключение страниц
7. **Нет glassmorphism** — выглядит "плоско" по современным стандартам

### 🟡 UX проблемы
1. **Нет поиска по имени** — при 50+ персонах невозможно найти
2. **Нет группировки по размеру** — персона с 1 фото рядом с персоном с 100 фото
3. **Нет индикации загрузки** — непонятно, грузится что-то или зависло
4. **Нет pagination/infinite scroll** — при 200+ персонах страница тормозит
5. **Нет breadcrumb навигации** — потерялся в разделах

---

## 🎯 Целевой UX (Wireframes & Flows)

### 1️⃣ **Главная страница / Upload**

```
┌─────────────────────────────────────────────────────────────┐
│  🔴 Face Grouper          [Галерея] [Загрузка] [Граф]       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                                                       │   │
│  │         📸 Перетащите фотографии сюда                │   │
│  │                                                       │   │
│  │      или нажмите для выбора файлов                   │   │
│  │                                                       │   │
│  │   ┌──────────────────────────────────────────────┐   │   │
│  │   │  📁 Выбранные файлы (3)          24.5 MB     │   │   │
│  │   │  ┌────┐                                      │   │   │
│  │   │  │🖼️  │ photo1.jpg            8.2 MB        │   │   │
│  │   │  └────┘                                      │   │   │
│  │   │  ┌────┐                                      │   │   │
│  │   │  │🖼️  │ photo2.jpg            12.1 MB       │   │   │
│  │   │  └────┘                                      │   │   │
│  │   │  ┌────┐                                      │   │   │
│  │   │  │📦  │ archive.zip           4.2 MB         │   │   │
│  │   │  └────┘                                      │   │   │
│  │   └──────────────────────────────────────────────┘   │   │
│  │                                                       │   │
│  │   [🚀 Начать обработку]  [🗑️ Очистить]               │   │
│  │                                                       │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ──────────── или ────────────                               │
│                                                              │
│  🔍 Быстрый поиск по селфи                                   │
│  ┌──────────────────────────────────────┐                   │
│  │  📷 Загрузите своё фото, чтобы       │                   │
│  │     найти себя на фотографиях        │                   │
│  │  [📤 Выбрать фото]                   │                   │
│  └──────────────────────────────────────┘                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Улучшения:**
- [ ] Добавить quick selfie search
- [ ] Улучшить визуал upload зоны (gradient border animation)
- [ ] Добавить skeleton для списка файлов

---

### 2️⃣ **Страница обработки (Processing)**

```
┌─────────────────────────────────────────────────────────────┐
│  🔴 Face Grouper          [Галерея] [Загрузка] [Граф]       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│              ┌───────────────────────────┐                  │
│              │                           │                  │
│              │     ⚙️ Обработка          │                  │
│              │                           │                  │
│              │   ┌─────────────────────┐ │                  │
│              │   │  ████████░░░░░  65% │ │                  │
│              │   └─────────────────────┘ │                  │
│              │                           │                  │
│              │   Обнаружение лиц...      │                  │
│              │   (127/196 файлов)        │                  │
│              │                           │                  │
│              │   ⏱️ Прошло: 2:34         │                  │
│              │   ⏰ Осталось: ~1:18      │                  │
│              │                           │                  │
│              │        ⟳ (spinner)        │                  │
│              │                           │                  │
│              │   [⛔ Отменить обработку]  │                  │
│              │                           │                  │
│              └───────────────────────────┘                  │
│                                                              │
│  ──────────── Детали обработки ────────────                  │
│  📊 Сканирование:   ✓ (0.5s)                                │
│  🔍 Обнаружение:    ⟳ 65% (2:31)                           │
│  🧠 Кластеризация:  ○                                       │
│  📁 Организация:    ○                                       │
│  📝 Отчёт:          ○                                       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Улучшения:**
- [ ] Добавить step-by-step индикатор (как в примере выше)
- [ ] Анимация spinner → morphing icons для каждого этапа
- [ ] Показывать скорость (фото/сек)
- [ ] Показывать найденные лица в реальном времени

---

### 3️⃣ **Галерея персон (Gallery)**

```
┌─────────────────────────────────────────────────────────────┐
│  🔴 Face Grouper          [Галерея] [Загрузка] [Граф]       │
├─────────────────────────────────────────────────────────────┤
│  📊 196 фото  |  423 лица  |  87 персон  |  ⚠️ 3 ошибки     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  🔍 [Поиск по имени...]    [Сортировка ▼]  [Фильтры]       │
│                                                              │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐   │
│  │        │ │        │ │        │ │        │ │        │   │
│  │  🖼️    │ │  🖼️    │ │  🖼️    │ │  🖼️    │ │  🖼️    │   │
│  │        │ │        │ │        │ │        │ │        │   │
│  │        │ │        │ │        │ │        │ │        │   │
│  ├────────┤ ├────────┤ ├────────┤ ├────────┤ ├────────┤   │
│  │ Иван   │ │ Мария  │ │ Person │ │ Алексей│ │ Person │   │
│  │ 45 фото│ │ 38 фото│ │ 5      │ │ 23 фото│ │ 4      │   │
│  │ 98 лиц │ │ 82 лица│ │ 12 лиц │ │ 47 лиц │ │ 8 лиц  │   │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘   │
│                                                              │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐   │
│  │        │ │        │ │        │ │        │ │        │   │
│  │  🖼️    │ │  🖼️    │ │  🖼️    │ │  🖼️    │ │  🖼️    │   │
│  │        │ │        │ │        │ │        │ │        │   │
│  │        │ │        │ │        │ │        │ │        │   │
│  ├────────┤ ├────────┤ ├────────┤ ├────────┤ ├────────┤   │
│  │ Ольга  │ │ Person │ │ Дмитрий│ │ Person │ │ Person │   │
│  │ 19 фото│ │ 6      │ │ 15 фото│ │ 7      │ │ 3      │   │
│  │ 41 лицо│ │ 15 лиц │ │ 32 лица│ │ 9 лиц  │ │ 6 лиц  │   │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘   │
│                                                              │
│  [1] [2] [3] ... [10]  (pagination)                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Улучшения:**
- [ ] **Поиск по имени** (debounce 300ms)
- [ ] **Сортировка**: по имени, по кол-ву фото, по качеству, по дате
- [ ] **Фильтр по диапазону фото**: слайдер min-max (1-10, 10-50, 50+)
- [ ] **Pagination** или infinite scroll (20 карточек на страницу)
- [ ] **Skeleton loaders** вместо пустой страницы
- [ ] **Glassmorphism** на карточках

---

### 4️⃣ **Страница персоны (Person Detail)**

```
┌─────────────────────────────────────────────────────────────┐
│  🔴 Face Grouper          [Галерея] [Загрузка] [Граф]       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ← Назад                                                    │
│                                                              │
│  ┌──────────┐                                               │
│  │          │  Иван Иванов ✏️                                │
│  │   🖼️     │  45 фото  •  98 лиц  •  quality: 0.87       │
│  │          │                                               │
│  └──────────┘  [🔍 Найти похожих]  [📊 Граф связей]        │
│                                                              │
│  ──────────── Фотографии (45) ────────────                   │
│                                                              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐           │
│  │ ┌─────┐ │ │ ┌─────┐ │ │ ┌─────┐ │ │ ┌─────┐ │           │
│  │ │ 👤  │ │ │ │ 👤  │ │ │ │     │ │ │ │ 👤  │ │           │
│  │ │  ▭  │ │ │ │     │ │ │ │ 👤  │ │ │ │     │ │           │
│  │ └─────┘ │ │ └─────┘ │ │ └─────┘ │ │ └─────┘ │           │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘           │
│   BBox overlay ↑        Multiple faces ↑                    │
│                                                              │
│  ──────────── Связи с другими персонами ────────────         │
│                                                              │
│  ┌────────────────────────────────────────────────┐         │
│  │  Мин. связь: [━━━●━━━━━━━] 0.3                │         │
│  │                                                │         │
│  │         (Мария)───0.72───(Иван)───0.45───(Ольга)│        │
│  │           │                    │               │         │
│  │         0.38                 0.51              │         │
│  │           │                    │               │         │
│  │        (Person 5)───0.29───(Алексей)           │         │
│  │                                                │         │
│  │  💡 Клик по узлу → страница персоны            │         │
│  └────────────────────────────────────────────────┘         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Улучшения:**
- [ ] **Bbox overlay** на фото (SVG rectangle с именем персоны)
- [ ] **Подсветка лиц** разных персон разными цветами
- [ ] **Слайдер минимальной связи** на графе
- [ ] **Кнопка "Найти похожих"** (selfie search)
- [ ] **Показ EXIF данных** (дата съёмки, если доступна)
- [ ] **Grid/List view toggle**

---

### 5️⃣ **Страница общего графа (Global Graph)** — НОВАЯ!

```
┌─────────────────────────────────────────────────────────────┐
│  🔴 Face Grouper          [Галерея] [Загрузка] [Граф]       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  🔍 [Поиск персоны...]    Мин. связь: [━●━━━━━━━] 0.25     │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                                                       │   │
│  │         (Иван)───────(Мария)                         │   │
│  │           │  \         /  │                          │   │
│  │           │   0.72    0.45  │                         │   │
│  │           │    \       /    │                         │   │
│  │        (Ольга)───(Person 5)──(Алексей)               │   │
│  │           │                          │                │   │
│  │           │ 0.38                     │ 0.51           │   │
│  │           │                          │                │   │
│  │        (Дмитрий)                 (Person 7)           │   │
│  │                                                       │   │
│  │  🖱️ Drag = перемещение узлов                         │   │
│  │  🔎 Scroll = zoom in/out                              │   │
│  │  👆 Клик = страница персоны                           │   │
│  │                                                       │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ──────────── Статистика графа ────────────                  │
│  📊 87 узлов  |  142 связи  |  clusters: 5                  │
│  🤝 Самая сильная связь: Иван ↔ Мария (0.72)                │
│  📸 Чаще всего вместе: Иван, Мария, Ольга (18 фото)         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Функционал:**
- [ ] **Force simulation** с оптимизацией производительности
- [ ] **Zoom/Pan** (mouse wheel + drag)
- [ ] **Фильтрация по связи** (slider)
- [ ] **Поиск персоны** (highlight узла)
- [ ] **Cluster highlighting** (разные цвета для кластеров)
- [ ] **Edge labels** с числом совместных фото
- [ ] **Export graph** (PNG/SVG)

---

### 6️⃣ **BBox Overlay на фото** — ДЕТАЛИ

```html
<div class="photo-with-faces" style="position: relative">
  <img src="/output/person_1/photo.jpg" style="width: 100%">
  
  <svg style="position: absolute; top: 0; left: 0; width: 100%; height: 100%">
    <!-- Face 1: This person -->
    <rect x="20%" y="10%" width="30%" height="50%" 
          style="fill: none; stroke: #e94560; stroke-width: 3"/>
    <text x="20%" y="8%" fill="#e94560" font-size="14" font-weight="bold">
      Иван
    </text>
    
    <!-- Face 2: Different person -->
    <rect x="60%" y="15%" width="25%" height="45%" 
          style="fill: none; stroke: #4ecdc4; stroke-width: 2; stroke-dasharray: 5,5"/>
    <text x="60%" y="13%" fill="#4ecdc4" font-size="12">
      Мария
    </text>
  </svg>
</div>
```

**Реализация:**
- [ ] Backend: Добавить в `/api/v1/persons/{id}/photos` поле `faces[]` с bbox
- [ ] Frontend: SVG overlay с относительными координатами (%)
- [ ] Цветовая кодировка: основной персон — solid, другие — dashed
- [ ] Tooltip при hover на bbox с именем и confidence

---

## 🎨 Дизайн-система

### Цветовая палитра

```css
:root {
  /* Background */
  --bg-primary: #0a0a0f;
  --bg-secondary: #12121a;
  --bg-tertiary: #1a1a2e;
  --bg-glass: rgba(18, 18, 26, 0.7);
  
  /* Accent */
  --accent-primary: #e94560;
  --accent-secondary: #ff6b81;
  --accent-soft: rgba(233, 69, 96, 0.12);
  
  /* Additional colors */
  --success: #4ecdc4;
  --warning: #f7b731;
  --error: #fc5c65;
  --info: #45aaf2;
  
  /* Text */
  --text-primary: #e0e0e0;
  --text-secondary: #777;
  --text-tertiary: #555;
  --text-inverse: #fff;
  
  /* Borders */
  --border-light: rgba(255, 255, 255, 0.06);
  --border-medium: rgba(255, 255, 255, 0.12);
  --border-accent: rgba(233, 69, 96, 0.3);
  
  /* Shadows */
  --shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.3);
  --shadow-md: 0 4px 16px rgba(0, 0, 0, 0.4);
  --shadow-lg: 0 8px 32px rgba(0, 0, 0, 0.5);
  --shadow-glow: 0 0 20px rgba(233, 69, 96, 0.15);
  
  /* Blur */
  --blur-sm: blur(4px);
  --blur-md: blur(8px);
  --blur-lg: blur(16px);
}
```

### Типографика

```css
:root {
  --font-primary: system-ui, -apple-system, 'Segoe UI', sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  
  --text-xs: 0.75rem;    /* 12px - captions, badges */
  --text-sm: 0.875rem;   /* 14px - secondary text */
  --text-base: 1rem;     /* 16px - body */
  --text-lg: 1.125rem;   /* 18px - subtitles */
  --text-xl: 1.25rem;    /* 20px - headers */
  --text-2xl: 1.5rem;    /* 24px - page titles */
  --text-3xl: 2rem;      /* 32px - hero */
  
  --font-light: 300;
  --font-regular: 400;
  --font-medium: 500;
  --font-semibold: 600;
  --font-bold: 700;
}
```

### Компоненты

#### 1. Glassmorphism Card
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
  border-color: var(--border-accent);
  box-shadow: var(--shadow-glow);
  transform: translateY(-4px);
}
```

#### 2. Skeleton Loader
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
  border-radius: 8px;
}

.skeleton-circle {
  width: 200px;
  height: 200px;
  border-radius: 50%;
}

.skeleton-text {
  height: 16px;
  margin: 8px 0;
}
```

#### 3. Page Transitions
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

.page.exit {
  opacity: 0;
  transform: translateY(-20px);
}
```

#### 4. Button Variants
```css
.btn-primary {
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
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

.btn-primary:active {
  transform: translateY(0);
}

.btn-primary:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  transform: none;
}
```

---

## 📋 Чек-лист задач

### 🔴 Критичные (P0)

| # | Задача | Файл | Сложность | Время |
|---|--------|------|-----------|-------|
| 1 | **Bbox overlay на фото** | `index.html`, `handler/person.go` | 🟡 Средняя | 2-3 часа |
| 2 | **Слайдер минимальной связи** | `index.html` (renderRelationsGraph) | 🟢 Низкая | 1-2 часа |
| 3 | **Отдельная страница "Граф"** | `index.html`, новый handler | 🟡 Средняя | 3-4 часа |
| 4 | **Поиск по имени в галерее** | `index.html` (filter функция) | 🟢 Низкая | 1 час |
| 5 | **Сортировка в галерее** | `index.html` (sort функция) | 🟢 Низкая | 1 час |

### 🟡 Важные (P1)

| # | Задача | Файл | Сложность | Время |
|---|--------|------|-----------|-------|
| 6 | **Skeleton loaders** | `index.html` CSS | 🟢 Низкая | 2 часа |
| 7 | **Page transitions** | `index.html` CSS + JS | 🟢 Низкая | 1-2 часа |
| 8 | **Glassmorphism стили** | `index.html` CSS | 🟢 Низкая | 2 часа |
| 9 | **Pagination/Infinite scroll** | `index.html` | 🟡 Средняя | 3-4 часа |
| 10 | **Фильтр по диапазону фото** | `index.html` | 🟢 Низкая | 1-2 часа |

### 🟢 Желательные (P2)

| # | Задача | Файл | Сложность | Время |
|---|--------|------|-----------|-------|
| 11 | **Selfie search UI** | `index.html`, новый handler | 🔴 Высокая | 6-8 часов |
| 12 | **Export graph (PNG/SVG)** | `index.html` JS | 🟡 Средняя | 2-3 часа |
| 13 | **EXIF дата на фото** | Backend + Frontend | 🟡 Средняя | 3-4 часа |
| 14 | **Grid/List view toggle** | `index.html` | 🟢 Низкая | 1-2 часа |
| 15 | **Cluster highlighting** | Graph страница | 🟡 Средняя | 3-4 часа |

---

## 🎯 Приоритетный план реализации

### Этап 1: Критичные улучшения (P0) — 8-11 часов
1. Bbox overlay на фото (самое важное для UX)
2. Слайдер минимальной связи на графе
3. Отдельная страница "Общий граф"
4. Поиск по имени в галерее
5. Сортировка в галерее

### Этап 2: Улучшение UX (P1) — 9-13 часов
6. Skeleton loaders
7. Page transitions
8. Glassmorphism стили
9. Pagination
10. Фильтр по диапазону фото

### Этап 3: Дополнительные фичи (P2) — 15-21 час
11. Selfie search UI
12. Export graph
13. EXIF данные
14. Grid/List view
15. Cluster highlighting

---

## 📐 Технические детали

### Backend изменения

#### 1. Модифицировать `GET /api/v1/persons/{id}/photos`

**Текущий ответ:**
```json
{
  "photos": ["/output/person_1/photo1.jpg", ...]
}
```

**Новый ответ:**
```json
{
  "photos": [
    {
      "url": "/output/person_1/photo1.jpg",
      "width": 1920,
      "height": 1080,
      "faces": [
        {
          "person_id": "uuid-1",
          "person_name": "Иван",
          "is_this_person": true,
          "bbox": { "x1": 0.2, "y1": 0.1, "x2": 0.5, "y2": 0.6 },
          "confidence": 0.92
        },
        {
          "person_id": "uuid-2",
          "person_name": "Мария",
          "is_this_person": false,
          "bbox": { "x1": 0.6, "y1": 0.15, "x2": 0.85, "y2": 0.6 },
          "confidence": 0.87
        }
      ]
    }
  ]
}
```

#### 2. Новый endpoint `GET /api/v1/graph`

**Ответ:**
```json
{
  "nodes": [
    {
      "id": "uuid-1",
      "name": "Иван",
      "custom_name": "Иван Иванов",
      "avatar": "person_1/avatar.jpg",
      "photo_count": 45,
      "face_count": 98,
      "cluster_id": 1
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
    "strongest_link": { "person1": "Иван", "person2": "Мария", "similarity": 0.72 }
  }
}
```

### Frontend изменения

#### 1. Bbox overlay renderer

```javascript
function renderPhotoWithBbox(photo, container) {
  const wrapper = document.createElement('div');
  wrapper.className = 'photo-with-faces';
  wrapper.style.position = 'relative';
  
  const img = document.createElement('img');
  img.src = photo.url;
  img.style.width = '100%';
  img.style.display = 'block';
  wrapper.appendChild(img);
  
  if (photo.faces && photo.faces.length > 0) {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.style.cssText = 'position: absolute; top: 0; left: 0; width: 100%; height: 100%; pointer-events: none;';
    
    photo.faces.forEach(face => {
      const rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
      rect.setAttribute('x', `${face.bbox.x1 * 100}%`);
      rect.setAttribute('y', `${face.bbox.y1 * 100}%`);
      rect.setAttribute('width', `${(face.bbox.x2 - face.bbox.x1) * 100}%`);
      rect.setAttribute('height', `${(face.bbox.y2 - face.bbox.y1) * 100}%`);
      rect.style.cssText = `
        fill: none;
        stroke: ${face.is_this_person ? '#e94560' : '#4ecdc4'};
        stroke-width: ${face.is_this_person ? 3 : 2};
        ${face.is_this_person ? '' : 'stroke-dasharray: 5,5;'}
      `;
      svg.appendChild(rect);
      
      const label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      label.setAttribute('x', `${face.bbox.x1 * 100}%`);
      label.setAttribute('y', `${(face.bbox.y1 - 0.02) * 100}%`);
      label.setAttribute('fill', face.is_this_person ? '#e94560' : '#4ecdc4');
      label.setAttribute('font-size', face.is_this_person ? '14' : '12');
      label.setAttribute('font-weight', face.is_this_person ? 'bold' : 'normal');
      label.textContent = face.person_name;
      svg.appendChild(label);
    });
    
    wrapper.appendChild(svg);
  }
  
  container.appendChild(wrapper);
}
```

#### 2. Global Graph page

```javascript
async function renderGlobalGraph() {
  const container = document.getElementById('global-graph');
  
  // Load graph data
  const resp = await fetch('/api/v1/graph');
  const data = await resp.json();
  
  // Filter by similarity threshold
  const minSimilarity = parseFloat(document.getElementById('graph-threshold').value);
  const filteredLinks = data.links.filter(l => l.similarity >= minSimilarity);
  
  // Build node set
  const nodeSet = new Set();
  filteredLinks.forEach(l => {
    nodeSet.add(l.source);
    nodeSet.add(l.target);
  });
  const nodes = data.nodes.filter(n => nodeSet.has(n.id));
  
  // D3 force simulation
  const width = container.clientWidth;
  const height = 600;
  
  const svg = d3.select(container).append('svg')
    .attr('viewBox', [0, 0, width, height])
    .call(d3.zoom().on('zoom', (event) => {
      g.attr('transform', event.transform);
    }));
  
  const g = svg.append('g');
  
  const simulation = d3.forceSimulation(nodes)
    .force('charge', d3.forceManyBody().strength(-500))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('link', d3.forceLink(filteredLinks).id(d => d.id).distance(150))
    .force('collide', d3.forceCollide(50));
  
  // Draw nodes, links, labels...
  // (similar to current renderRelationsGraph but optimized)
}
```

---

## 🧪 Тестирование UI

### Browser Support
- Chrome/Edge 90+
- Firefox 88+
- Safari 14+
- Mobile Safari/Chrome (iOS/Android)

### Performance Budget
- **First Contentful Paint**: < 1s
- **Time to Interactive**: < 2s
- **Gallery render (100 cards)**: < 100ms
- **Graph render (100 nodes)**: < 500ms
- **Photo modal open**: < 50ms

### Accessibility (A11y)
- [ ] Keyboard navigation (Tab, Enter, Escape)
- [ ] ARIA labels для интерактивных элементов
- [ ] Focus states для всех кнопок/ссылок
- [ ] Alt text для изображений
- [ ] Color contrast ratio ≥ 4.5:1

---

## 📱 Responsive Breakpoints

```css
/* Mobile */
@media (max-width: 640px) {
  .person-grid { grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); }
  .header { flex-direction: column; }
  .stats-bar { flex-wrap: wrap; gap: 1rem; }
}

/* Tablet */
@media (min-width: 641px) and (max-width: 1024px) {
  .person-grid { grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); }
}

/* Desktop */
@media (min-width: 1025px) {
  .person-grid { grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); }
  .container { max-width: 1440px; margin: 0 auto; }
}

/* Ultra-wide */
@media (min-width: 1920px) {
  .person-grid { grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); }
}
```

---

## 🚀 Rollout Plan

### Phase 1: Foundation (Week 1)
- [ ] Обновить CSS variables и дизайн-систему
- [ ] Добавить skeleton loaders
- [ ] Добавить page transitions
- [ ] Обновить glassmorphism стили

### Phase 2: Core Features (Week 2)
- [ ] Bbox overlay на фото
- [ ] Поиск/сортировка/фильтры в галерее
- [ ] Слайдер минимальной связи
- [ ] Pagination

### Phase 3: Advanced Features (Week 3)
- [ ] Отдельная страница "Общий граф"
- [ ] Selfie search UI
- [ ] Export graph
- [ ] Cluster highlighting

### Phase 4: Polish (Week 4)
- [ ] EXIF данные
- [ ] Grid/List view toggle
- [ ] Accessibility audit
- [ ] Performance optimization
- [ ] Mobile responsive testing

---

## 📊 Success Metrics

### UX Metrics
- **Time to find a person**: < 5s (search) or < 15s (manual browse)
- **Graph interaction rate**: > 60% users click on graph nodes
- **Photo view rate**: > 80% users click on photos to see fullscreen
- **Rename rate**: > 30% users rename at least one person

### Performance Metrics
- **Gallery render time**: < 100ms for 100 cards
- **Graph render time**: < 500ms for 100 nodes
- **Photo modal open**: < 50ms
- **Search response**: < 300ms (debounce)

### Error Metrics
- **Image load failures**: < 1%
- **Graph render errors**: < 0.5%
- **JS errors in production**: < 0.1%

---

## 🎓 Learning Resources

### Design Systems
- [Material Design 3](https://m3.material.io/)
- [Apple Human Interface Guidelines](https://developer.apple.com/design/human-interface-guidelines/)
- [Glassmorphism Generator](https://glassmorphism.com/)

### D3.js Graphs
- [D3 Force Simulation](https://observablehq.com/@d3/force-simulation)
- [D3 Zoom & Pan](https://observablehq.com/@d3/zoom)
- [D3 Drag & Drop](https://observablehq.com/@d3/drag-and-drop)

### Performance
- [Intersection Observer API](https://developer.mozilla.org/en-US/docs/Web/API/Intersection_Observer_API)
- [Virtual Scrolling](https://github.com/bvaughn/react-virtualized)
- [Image Optimization](https://web.dev/fast/#optimize-your-images)

---

## 📝 Notes

### Что НЕ делать (Anti-patterns)
- ❌ Не использовать React/Vue (текущий стек — vanilla JS, это ок для SPA)
- ❌ Не добавлять CSS frameworks (Bootstrap, Tailwind) — увеличат bundle
- ❌ Не усложнять анимации (60fps target, не больше)
- ❌ Не делать real-time collaboration (это не нужно)

### Что помнить
- ⚠️ D3.js уже подключён (v7.min.js, embedded)
- ⚠️ Все фото грузатся из `/output/` (с кэшированием)
- ⚠️ Report загружается из `report.json` (fallback mode)
- ⚠️ Граф строится на ко-апиренсе (shared photos), не на similarity embeddings
- ⚠️ Bbox координаты в relative format (0.0-1.0)

### Идеи на будущее (Post-v3)
- 🌟 AI-powered face tagging (распознавание по именам из соцсетей)
- 🌟 Timeline view (хронологическая лента фото)
- 🌟 Face aging analysis (как человек изменился со временем)
- 🌟 Privacy mode (blur faces для sharing)
- 🌟 Multi-language support (i18n)
- 🌟 Dark/Light theme toggle
