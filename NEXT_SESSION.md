# NEXT_SESSION — затравка

Живой handoff-документ. Обновляй в конце каждой сессии (что сделано / что дальше).

## Где мы

- **Фаза 0 (каркас) завершена**, гейт `make check` зелёный, коммит(ы) в `main` (локально).
- Пуша на GitHub ещё **не было** (ждём явного «go» от пользователя).
- Стек: Go 1.23, cobra; хранение — YAML файл-на-задачу (см. DESIGN.md).

## Сессия начинается с планирования (обязательно)

Перед любым кодом — фаза планирования (используй skills superpowers: brainstorming/planning).
План обязан учесть ключевые инварианты из DESIGN.md:

- **Типы и иерархия — домен (из конфига); ID/slug — дело адаптера.** В коде НЕТ литералов имён
  типов и структуры ID. Иерархия — из поля `parent` типа. ID минтит `TaskStore` (YAML:
  `<prefix><N>` по цепочке, `e1` → `e1_t3` → `e1_t3_s2`; `prefix` — поле YAML-адаптера).
- **Инварианты (валидирует загрузка конфига):** в наборе типов есть дефолтный `task`; у статуса
  есть категория `kind` (initial/active/terminal), терминалов ≥1 (в дефолте `done`+`cancelled`),
  ready/list — по категории; в любом flow есть якоря `tbd → in_progress → done` в этом порядке.
- **Capabilities:** возможности (история, зависимости, дерево-комментарии, поиск, **KB**) —
  опциональны per-адаптер (`Capabilities()` / `ErrUnsupported`); YAML — референс (умеет всё),
  `core` пишет на минимум и «зажигает» доступное. Задача несёт append-only `history` (в YAML всегда).
- **Ссылки:** задачи/комментарии несут структурные проверяемые `refs` (`note`/`task`/`comment`/`url`)
  — информационные, **≠ `depends_on`**. Верификация capability-aware (note — только при KB). Без KB
  знания живут в задачах/комментариях и связях между ними.
- **Killer-фича — исполняемые переходы:** на переход вешаются `description` + `commands`
  (все → 0, иначе переход блокируется). Исполнение — за портом `Runner` (`core` определяет,
  `internal/adapter/exec` реализует, тест — фейк). `start`/`done` — мета-команды `advance --to`
  (обход до цели; режимы `--stop`(деф)/`--atomic`/`--force`; без config-DSL). Фаза 3.
- **`mtt init`** пишет дефолтный `.mtt/config.yaml` (типы + flow, без команд). Дефолты — в
  шаблоне init, не в логике.
- Следствие для порядка: контракт+типы+адаптер (с минтингом ID) — Фаза 1; flow-enforcement
  с исполнением команд — Фаза 3.
- **Позиционирование (см. DESIGN.md → «Позиционирование vs beads»):** наш клин — per-type
  flow + zero-footprint + человеческий UI. Зависимости держим простыми, КБ — низкий
  приоритет. ID-коллизии приняты осознанно (не усложнять до появления реального параллелизма).

## Что прочитать в начале (по порядку)

1. [CLAUDE.md](CLAUDE.md) — точка входа
2. [AGENTS.md](AGENTS.md) — правила, гейт, принципы, DoD
3. [DESIGN.md](DESIGN.md) — архитектура и решения
4. [TASKS.md](TASKS.md) — план; следующая — секция **e2 (Фаза 1)**

## Активация guards (superpowers)

Плагин объявлен в личном `.claude/settings.local.json` (per-user, gitignored)
(`superpowers@superpowers-marketplace`). Плагины подхватываются **при старте сессии**:

1. При открытии проекта Claude Code может показать запрос доверия marketplace
   `obra/superpowers-marketplace` — подтвердить (один раз).
2. Если skills не появились автоматически, выполнить один раз:
   ```
   /plugin marketplace add obra/superpowers-marketplace
   /plugin install superpowers@superpowers-marketplace
   ```
   (альтернатива — официальный marketplace: `/plugin install superpowers@claude-plugins-official`)
3. Проверить, что доступны skills TDD/brainstorming/debugging, и **пользоваться ими**.

## Следующая задача — Фаза 1 (после планирования)

- Ветка: `feat/phase-1-core` (реализация — уже после плана).
- Ориентир задач: **e2** из [TASKS.md](TASKS.md) (планирование уточнит разбивку/порядок).
- **Test-first** (TDD: red → green → refactor). `make check` зелёный до каждого коммита.
- Архитектура — **hexagonal**: `cli → core → порт ← adapter`, контракт (домен-типы + порты)
  в публичном `pkg/mtt`. `core` не импортирует `adapter/*`.
- Порядок по существу: контракт `pkg/mtt` (типы + порт `TaskStore`) → конфиг+типы + `mtt init`
  (от типов зависит ID) → `internal/adapter/yaml` → `internal/core` (ID из конфига, usecase) →
  команды `add/list/show`.
- Создавать `CLAUDE.md` для каждого нового пакета (`pkg/mtt`, `internal/core`,
  `internal/adapter/yaml`, …).

## Готовый kickoff-промпт (можно вставить в новой сессии)

> Продолжаем mtt. Прочитай CLAUDE.md, AGENTS.md, DESIGN.md, TASKS.md и NEXT_SESSION.md.
> Убедись, что активны skills superpowers (иначе активируй по NEXT_SESSION.md).
> Начни с ПЛАНИРОВАНИЯ Фазы 1. Архитектура hexagonal: контракт (домен-типы + порты
> `TaskStore`/`KnowledgeStore`) в публичном `pkg/mtt`, логика в `internal/core`, YAML —
> дефолтный адаптер в `internal/adapter/yaml`; типы/иерархия/ID — из конфига (в коде нет
> литералов), `mtt init` пишет дефолты epic/task/subtask с flow. Затем реализуй в ветке
> `feat/phase-1-core` строго test-first.
> Соблюдай принципы (SOLID/DRY/KISS/TDD/чистая архитектура) и самопроверку из AGENTS.md.
