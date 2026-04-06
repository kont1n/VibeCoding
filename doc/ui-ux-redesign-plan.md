# 🎨 UI/UX Redesign Plan — Face Grouper v3.5

## Концепция: "Фотоархивный ретро-футуризм"

Смесь тёплой эстетики старого фотоархива (коричневые тона, бумажная текстура, классическая типографика) с современными технологиями (glassmorphism, неоновые акценты, плавные анимации).

**Настроение:** Тёплый, уютный, но технологичный интерфейс. Как фотолаборатория будущего, где ретро-эстетика встречается с ML-технологиями.

---

## 📊 Текущее состояние (Audit)

### ✅ Что уже реализовано
- **SPA Router** — навигация без перезагрузки
- **Upload Page** — drag-and-drop, превью файлов
- **Processing Page** — SSE streaming, прогресс-бар, ETA
- **Gallery Page** — сетка карточек, lazy loading
- **Person Detail Page** — аватар, редактируемое имя, сетка фото
- **Relations Graph** — D3.js force simulation
- **Errors Page** — список ошибок с классификацией
- **Modal Viewer** — полноэкранный просмотр, zoom, навигация
- **Search/Sort/Filter** — в галерее
- **Glassmorphism** — базовые стили карточек
- **Skeleton loaders** — при загрузке

### 🔴 Критические проблемы дизайна
1. **Generic "AI slop" эстетика** — тёмная тема с красным акцентом, системные шрифты
2. **Отсутствие характера** — выглядит как типичный современный сервис
3. **Плоская композиция** — нет глубины, всё на одном уровне
4. **Скучная типографика** — system-ui без индивидуальности
5. **Нет атмосферы** — фон сплошной, без текстур и глубины

---

## 🎯 Целевой дизайн

### Эмоциональный отклик
- **Тепло и ностальгия** — как старые фотографии
- **Технологичность** — ML, современные возможности
- **Уют** — приятно пользоваться, не агрессивно
- **Запоминаемость** — уникальный визуальный язык

### Ключевые метафоры
- **Фотолаборатория** — процесс обработки как проявка плёнки
- **Архив/Каталог** — организованность, порядок
- **Связи между людьми** — социальный граф как карта отношений

---

## 🎨 Дизайн-система v2

### Цветовая палитра: "Пленочная теплота"

```css
:root {
  /* Базовые фоны — тёплые, сепия-оттенки */
  --bg-primary: #0f0d0a;           /* Глубокий тёплый чёрный */
  --bg-secondary: #1a1612;          /* Тёплый уголь */
  --bg-tertiary: #252019;           /* Сепия-коричневый */
  --bg-card: #1e1a15;               /* Карточка */
  --bg-glass: rgba(30, 26, 21, 0.75);
  
  /* Акценты — пленочные цвета */
  --accent-primary: #d4a574;        /* Тёплое золото/песок */
  --accent-secondary: #e8c4a0;      /* Светлое золото */
  --accent-glow: rgba(212, 165, 116, 0.25);
  --accent-soft: rgba(212, 165, 116, 0.1);
  
  /* Функциональные цвета */
  --success: #7fb069;               /* Пленочный зелёный */
  --warning: #e6a85c;               /* Тёплый оранжевый */
  --error: #c75b5b;                 /* Приглушённый красный */
  --info: #6b9dc7;                  /* Приглушённый синий */
  
  /* Текст — тёплые оттенки */
  --text-primary: #f5ebe0;          /* Тёплый белый */
  --text-secondary: #b8a99a;        /* Песочный */
  --text-muted: #8a7f72;            /* Приглушённый */
  --text-dim: #5c544a;              /* Тёмный */
  
  /* Границы */
  --border-light: rgba(212, 165, 116, 0.08);
  --border-medium: rgba(212, 165, 116, 0.15);
  --border-accent: rgba(212, 165, 116, 0.35);
  
  /* Текстуры */
  --noise-opacity: 0.03;
  --grain-opacity: 0.02;
  
  /* Тени — тёплые */
  --shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.4);
  --shadow-md: 0 4px 20px rgba(0, 0, 0, 0.5);
  --shadow-lg: 0 8px 40px rgba(0, 0, 0, 0.6);
  --shadow-glow: 0 0 30px rgba(212, 165, 116, 0.12);
  
  /* Blur */
  --blur-sm: blur(6px);
  --blur-md: blur(12px);
  --blur-lg: blur(24px);
}
```

### Типографика: "Классика и современность"

```css
:root {
  /* 
   * Display/Headlines: Playfair Display — элегантный serif
   * Body: Source Sans 3 — читаемый sans-serif
   * Mono: JetBrains Mono — для кода/технических данных
   */
  --font-display: 'Playfair Display', 'Georgia', serif;
  --font-body: 'Source Sans 3', 'system-ui', sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  
  /* Размеры */
  --text-xs: 0.75rem;      /* 12px — captions */
  --text-sm: 0.875rem;     /* 14px — secondary */
  --text-base: 1rem;       /* 16px — body */
  --text-lg: 1.125rem;     /* 18px — subtitles */
  --text-xl: 1.25rem;      /* 20px — h3 */
  --text-2xl: 1.5rem;      /* 24px — h2 */
  --text-3xl: 2rem;        /* 32px — h1 */
  --text-hero: 3rem;       /* 48px — hero */
  
  /* Начертания */
  --font-light: 300;
  --font-regular: 400;
  --font-medium: 500;
  --font-semibold: 600;
  --font-bold: 700;
}
```

**Google Fonts для подключения:**
- Playfair Display (400, 500, 600, 700)
- Source Sans 3 (300, 400, 500, 600, 700)
- JetBrains Mono (400, 500)

### Визуальные эффекты

#### 1. Paper Texture Overlay
```css
/* Глобальная текстура бумаги */
body::before {
  content: '';
  position: fixed;
  inset: 0;
  background-image: url("data:image/svg+xml,..."); /* SVG noise pattern */
  opacity: var(--noise-opacity);
  pointer-events: none;
  z-index: 9999;
}
```

#### 2. Warm Gradient Mesh Background
```css
/* Тёплый градиент вместо сплошного цвета */
.bg-warm-gradient {
  background: 
    radial-gradient(ellipse at 20% 20%, rgba(212, 165, 116, 0.08) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 80%, rgba(139, 90, 43, 0.05) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 50%, rgba(30, 26, 21, 0.5) 0%, transparent 70%),
    var(--bg-primary);
}
```

#### 3. Sepia Card Glow
```css
.card-warm {
  background: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: 16px;
  box-shadow: 
    var(--shadow-md),
    inset 0 1px 0 rgba(255, 255, 255, 0.03);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.card-warm:hover {
  border-color: var(--border-accent);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
  transform: translateY(-2px);
}
```

---

## 🧩 Компоненты

### 1. Кнопки

```css
/* Primary — золотистый градиент */
.btn-primary {
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  color: var(--bg-primary);
  font-family: var(--font-body);
  font-weight: 600;
  padding: 0.75rem 2rem;
  border: none;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 
    0 4px 12px rgba(212, 165, 116, 0.25),
    inset 0 1px 0 rgba(255, 255, 255, 0.2);
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 
    0 8px 24px rgba(212, 165, 116, 0.35),
    inset 0 1px 0 rgba(255, 255, 255, 0.3);
}

/* Secondary — outline */
.btn-secondary {
  background: transparent;
  border: 1.5px solid var(--border-medium);
  color: var(--text-primary);
  padding: 0.75rem 2rem;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.25s ease;
}

.btn-secondary:hover {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  background: var(--accent-soft);
}
```

### 2. Карточки персон

```css
.person-card {
  position: relative;
  background: var(--bg-card);
  border-radius: 16px;
  overflow: hidden;
  cursor: pointer;
  border: 1px solid var(--border-light);
  transition: all 0.35s cubic-bezier(0.4, 0, 0.2, 1);
}

.person-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(
    180deg,
    transparent 50%,
    rgba(15, 13, 10, 0.8) 100%
  );
  z-index: 1;
  opacity: 0;
  transition: opacity 0.3s ease;
}

.person-card:hover {
  transform: translateY(-4px) scale(1.01);
  border-color: var(--border-accent);
  box-shadow: 
    var(--shadow-lg),
    0 0 40px rgba(212, 165, 116, 0.1);
}

.person-card:hover::before {
  opacity: 1;
}

.person-card .thumb {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  transition: transform 0.5s ease;
}

.person-card:hover .thumb {
  transform: scale(1.05);
}

.person-card .info {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 1rem;
  z-index: 2;
  transform: translateY(8px);
  opacity: 0.9;
  transition: all 0.3s ease;
}

.person-card:hover .info {
  transform: translateY(0);
  opacity: 1;
}

.person-card .name {
  font-family: var(--font-display);
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 0.25rem;
}

.person-card .count {
  font-family: var(--font-body);
  font-size: 0.8rem;
  color: var(--text-secondary);
}
```

### 3. Upload Zone

```css
.upload-zone {
  position: relative;
  border: 2px dashed var(--border-medium);
  border-radius: 24px;
  padding: 4rem 2rem;
  text-align: center;
  background: 
    radial-gradient(ellipse at center, var(--accent-soft) 0%, transparent 70%),
    var(--bg-card);
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
}

.upload-zone::before {
  content: '';
  position: absolute;
  inset: -2px;
  border-radius: 24px;
  padding: 2px;
  background: linear-gradient(135deg, var(--accent-primary), transparent, var(--accent-primary));
  -webkit-mask: 
    linear-gradient(#fff 0 0) content-box, 
    linear-gradient(#fff 0 0);
  mask: 
    linear-gradient(#fff 0 0) content-box, 
    linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  opacity: 0;
  transition: opacity 0.4s ease;
}

.upload-zone:hover::before,
.upload-zone.dragover::before {
  opacity: 1;
}

.upload-zone:hover,
.upload-zone.dragover {
  border-color: transparent;
  background: 
    radial-gradient(ellipse at center, rgba(212, 165, 116, 0.15) 0%, transparent 70%),
    var(--bg-card);
  transform: scale(1.005);
}

.upload-zone .icon {
  font-size: 3rem;
  margin-bottom: 1rem;
  opacity: 0.6;
  transition: all 0.3s ease;
}

.upload-zone:hover .icon {
  opacity: 1;
  transform: scale(1.1);
}
```

### 4. Progress Bar (Processing)

```css
.progress-container {
  background: var(--bg-secondary);
  border-radius: 20px;
  padding: 3rem;
  max-width: 600px;
  margin: 0 auto;
  border: 1px solid var(--border-light);
  box-shadow: var(--shadow-lg);
}

.progress-bar-outer {
  background: rgba(0, 0, 0, 0.3);
  border-radius: 100px;
  height: 8px;
  overflow: hidden;
  position: relative;
}

.progress-bar-outer::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.1) 50%,
    transparent 100%
  );
  animation: shimmer 2s infinite;
}

.progress-bar-inner {
  height: 100%;
  background: linear-gradient(90deg, var(--accent-primary), var(--accent-secondary));
  border-radius: 100px;
  width: 0%;
  transition: width 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 0 20px rgba(212, 165, 116, 0.4);
  position: relative;
}

.progress-bar-inner::after {
  content: '';
  position: absolute;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 4px;
  height: 16px;
  background: var(--accent-secondary);
  border-radius: 2px;
  box-shadow: 0 0 10px var(--accent-secondary);
}

@keyframes shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}
```

### 5. Header & Navigation

```css
.header {
  background: 
    linear-gradient(180deg, var(--bg-secondary) 0%, transparent 100%),
    var(--bg-glass);
  backdrop-filter: var(--blur-md);
  -webkit-backdrop-filter: var(--blur-md);
  border-bottom: 1px solid var(--border-light);
  padding: 1rem 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: sticky;
  top: 0;
  z-index: 100;
}

.logo {
  font-family: var(--font-display);
  font-size: 1.75rem;
  font-weight: 600;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 0.75rem;
  cursor: pointer;
}

.logo::before {
  content: '◈';
  color: var(--accent-primary);
  font-size: 1.5rem;
}

.nav-button {
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-secondary);
  padding: 0.5rem 1.25rem;
  border-radius: 10px;
  font-family: var(--font-body);
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.25s ease;
  position: relative;
}

.nav-button:hover {
  color: var(--text-primary);
  background: rgba(212, 165, 116, 0.05);
}

.nav-button.active {
  color: var(--accent-primary);
  background: var(--accent-soft);
  border-color: var(--border-accent);
}

.nav-button.active::after {
  content: '';
  position: absolute;
  bottom: -1px;
  left: 20%;
  right: 20%;
  height: 2px;
  background: var(--accent-primary);
  border-radius: 2px;
}
```

---

## ✨ Анимации и микро-взаимодействия

### 1. Page Transitions

```css
.page {
  opacity: 0;
  transform: translateY(30px) scale(0.98);
  filter: blur(4px);
  transition: 
    opacity 0.5s cubic-bezier(0.4, 0, 0.2, 1),
    transform 0.5s cubic-bezier(0.4, 0, 0.2, 1),
    filter 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.page.active {
  opacity: 1;
  transform: translateY(0) scale(1);
  filter: blur(0);
}
```

### 2. Card Stagger Animation

```css
@keyframes cardEnter {
  from {
    opacity: 0;
    transform: translateY(40px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.person-card {
  animation: cardEnter 0.5s cubic-bezier(0.4, 0, 0.2, 1) backwards;
}

/* Stagger delay через inline style или JS */
.person-card:nth-child(1) { animation-delay: 0ms; }
.person-card:nth-child(2) { animation-delay: 50ms; }
.person-card:nth-child(3) { animation-delay: 100ms; }
/* ... и так далее */
```

### 3. Photo Modal Zoom

```css
.modal {
  display: none;
  position: fixed;
  inset: 0;
  background: rgba(15, 13, 10, 0.95);
  backdrop-filter: blur(20px);
  z-index: 1000;
  justify-content: center;
  align-items: center;
  opacity: 0;
  transition: opacity 0.3s ease;
}

.modal.active {
  display: flex;
  opacity: 1;
}

.modal img {
  max-width: 90vw;
  max-height: 90vh;
  border-radius: 8px;
  box-shadow: 0 25px 80px rgba(0, 0, 0, 0.8);
  transform: scale(0.9);
  transition: transform 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.modal.active img {
  transform: scale(1);
}
```

### 4. Skeleton Shimmer

```css
.skeleton {
  background: linear-gradient(
    90deg,
    var(--bg-tertiary) 25%,
    rgba(212, 165, 116, 0.08) 50%,
    var(--bg-tertiary) 75%
  );
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  border-radius: 8px;
}

@keyframes shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
```

### 5. Graph Node Hover

```css
.graph-node {
  cursor: pointer;
  transition: all 0.3s ease;
}

.graph-node circle {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.graph-node:hover circle {
  filter: drop-shadow(0 0 8px var(--accent-primary));
  transform: scale(1.1);
}

.graph-link {
  stroke: var(--border-medium);
  stroke-linecap: round;
  transition: all 0.3s ease;
}

.graph-link:hover {
  stroke: var(--accent-primary);
  stroke-width: 3 !important;
  filter: drop-shadow(0 0 4px var(--accent-primary));
}
```

---

## 📱 Responsive Design

```css
/* Mobile */
@media (max-width: 640px) {
  :root {
    --text-hero: 2rem;
    --text-3xl: 1.5rem;
  }
  
  .header {
    padding: 0.75rem 1rem;
    flex-direction: column;
    gap: 0.5rem;
  }
  
  .person-grid {
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 0.75rem;
  }
  
  .upload-zone {
    padding: 2.5rem 1rem;
    border-radius: 16px;
  }
}

/* Tablet */
@media (min-width: 641px) and (max-width: 1024px) {
  .person-grid {
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  }
}

/* Desktop */
@media (min-width: 1025px) {
  .person-grid {
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 1.5rem;
  }
  
  .container {
    max-width: 1440px;
    margin: 0 auto;
    padding: 2.5rem;
  }
}

/* Ultra-wide */
@media (min-width: 1920px) {
  .person-grid {
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  }
  
  .container {
    max-width: 1600px;
  }
}
```

---

## 📋 Чек-лист внедрения

### Phase 1: Foundation (2-3 часа)
- [ ] Подключить Google Fonts (Playfair Display, Source Sans 3)
- [ ] Обновить CSS variables (новая цветовая схема)
- [ ] Добавить paper texture overlay
- [ ] Обновить header (logo, nav buttons)

### Phase 2: Core Components (4-5 часов)
- [ ] Редизайн upload zone
- [ ] Редизайн person cards
- [ ] Редизайн progress bar
- [ ] Обновить кнопки (primary, secondary)
- [ ] Обновить modal viewer

### Phase 3: Animations (2-3 часа)
- [ ] Page transitions
- [ ] Card stagger animations
- [ ] Hover effects
- [ ] Graph animations

### Phase 4: Polish (1-2 часа)
- [ ] Responsive testing
- [ ] Accessibility check
- [ ] Performance audit
- [ ] Cross-browser testing

---

## 🎯 Success Metrics

### Визуальные
- [ ] Уникальная типографика (не system fonts)
- [ ] Тёплая цветовая схема (не холодный "tech" look)
- [ ] Текстуры и глубина (не плоский дизайн)
- [ ] Плавные анимации (60fps)

### UX
- [ ] Time to find person: < 5s с поиском
- [ ] Photo view engagement: > 80%
- [ ] Graph interaction rate: > 50%
- [ ] Mobile usability: полная функциональность

### Technical
- [ ] First Contentful Paint: < 1s
- [ ] Time to Interactive: < 2s
- [ ] No layout shift (CLS < 0.1)
- [ ] Works offline (service worker — optional)

---

## 📚 Assets Needed

### Fonts (Google CDN)
```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500&family=Playfair+Display:wght@400;500;600;700&family=Source+Sans+3:wght@300;400;500;600;700&display=swap" rel="stylesheet">
```

### Icons (Optional — можно использовать эмодзи/символы)
- Upload: 📸 или custom SVG
- Person: 👤 или custom SVG  
- Graph: ◈ или custom SVG
- Back: ←
- Search: 🔍
- Settings: ⚙️

### Textures (CSS/SVG — no external assets)
- Paper noise: CSS/SVG pattern
- Gradient mesh: CSS radial gradients
- Subtle grain: CSS filter

---

## 🚀 Implementation Order

### Priority 1: Must Have
1. Colors & Typography (фундамент)
2. Header & Navigation
3. Upload Zone
4. Person Cards

### Priority 2: Should Have  
5. Progress Bar
6. Modal Viewer
7. Page Transitions
8. Graph Styling

### Priority 3: Nice to Have
9. Advanced Animations
10. Micro-interactions
11. Accessibility enhancements

---

## 📝 Notes

### What NOT to do
- ❌ Не использовать React/Vue (vanilla JS is perfect for this SPA)
- ❌ Не добавлять тяжёлые CSS frameworks
- ❌ Не перегружать анимациями (60fps target)
- ❌ Не забывать про accessibility

### Browser Support
- Chrome/Edge 90+
- Firefox 88+
- Safari 14+
- Mobile: iOS Safari 14+, Chrome Android

### Performance Considerations
- Использовать CSS transforms (не top/left)
- Hardware acceleration для анимаций
- Intersection Observer для lazy loading
- Debounce для поиска (300ms)
