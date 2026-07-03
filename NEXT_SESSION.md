# NEXT_SESSION — затравка

Живой handoff-документ. Обновляй в конце каждой сессии (что сделано / что дальше).

## Где мы

- **Фаза 0 (каркас) завершена**, гейт `make check` зелёный, коммит(ы) в `main` (локально).
- Пуша на GitHub ещё **не было** (ждём явного «go» от пользователя).
- Стек: Go 1.23, cobra; хранение — YAML файл-на-задачу (см. DESIGN.md).

## Сессия начинается с планирования (обязательно)

Перед любым кодом — фаза планирования (используй skills superpowers: brainstorming/planning).
План обязан учесть ключевые инварианты из DESIGN.md:

- **Типы, иерархия и ID — из конфига, не из кода.** В коде НЕТ литералов имён типов
  (`epic`/`task`/`subtask`) и структуры ID. Иерархия задаётся полем `parent` у типа; ID
  собирается как `<prefix><N>` по цепочке родителей (`e1` → `e1_t3` → `e1_t3_s2`).
- **`mtt init`** создаёт `.mtt/config.yaml` с дефолтными типами `epic`/`task`/`subtask` и
  дефолтными flow. Дефолты живут в шаблоне init, а не в логике.
- Следствие для порядка: конфиг+типы нужны уже в Фазе 1 (от них зависит генерация ID);
  enforcement переходов flow — Фаза 3.

## Что прочитать в начале (по порядку)

1. [CLAUDE.md](CLAUDE.md) — точка входа
2. [AGENTS.md](AGENTS.md) — правила, гейт, принципы, DoD
3. [DESIGN.md](DESIGN.md) — архитектура и решения
4. [TASKS.md](TASKS.md) — план; следующая — секция **e2 (Фаза 1)**

## Активация guards (superpowers)

Плагин объявлен в [.claude/settings.json](.claude/settings.json)
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

- Ветка: `feat/phase-1-store` (реализация — уже после плана).
- Ориентир задач: **e2** из [TASKS.md](TASKS.md) (планирование уточнит разбивку/порядок).
- **Test-first** (TDD: red → green → refactor). `make check` зелёный до каждого коммита.
- Порядок по существу: сначала **конфиг+типы** и `mtt init` (от типов зависит ID),
  затем `internal/store` (сериализация, атомарная запись, генерация ID из конфига),
  затем команды `add/list/show`.
- Создавать `CLAUDE.md` для каждого нового пакета (`internal/config`, `internal/model`,
  `internal/store`, …).

## Готовый kickoff-промпт (можно вставить в новой сессии)

> Продолжаем mtt. Прочитай CLAUDE.md, AGENTS.md, DESIGN.md, TASKS.md и NEXT_SESSION.md.
> Убедись, что активны skills superpowers (иначе активируй по NEXT_SESSION.md).
> Начни с ПЛАНИРОВАНИЯ Фазы 1: типы/иерархия/ID — из конфига (в коде нет литералов имён
> типов и структуры ID), `mtt init` пишет дефолты epic/task/subtask с flow. Затем реализуй
> в ветке `feat/phase-1-store` строго test-first.
> Соблюдай принципы (SOLID/DRY/KISS/TDD/чистая архитектура) и самопроверку из AGENTS.md.
