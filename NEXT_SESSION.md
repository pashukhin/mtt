# NEXT_SESSION — затравка

Живой handoff-документ. Обновляй в конце каждой сессии (что сделано / что дальше).

## Где мы

- **Фаза 0 (каркас) завершена**, гейт `make check` зелёный, коммит(ы) в `main` (локально).
- Пуша на GitHub ещё **не было** (ждём явного «go» от пользователя).
- Стек: Go 1.23, cobra; хранение — YAML файл-на-задачу (см. DESIGN.md).

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

## Следующая задача — Фаза 1 (модель + store + базовые команды)

- Ветка: `feat/phase-1-store`.
- Порядок задач: **e2_t1 → e2_t10** из [TASKS.md](TASKS.md).
- **Test-first** (TDD: red → green → refactor). `make check` зелёный до каждого коммита.
- Начать с `internal/model` (Task/Comment) и `internal/store` (детерминированная
  сериализация + атомарная запись + генерация ID), затем команды `add/list/show`.
- Создавать `CLAUDE.md` для каждого нового пакета (`internal/model`, `internal/store`, …).

## Готовый kickoff-промпт (можно вставить в новой сессии)

> Продолжаем mtt. Прочитай CLAUDE.md, AGENTS.md, DESIGN.md, TASKS.md и NEXT_SESSION.md.
> Убедись, что активны skills superpowers (иначе активируй по NEXT_SESSION.md).
> Начинаем Фазу 1 в ветке `feat/phase-1-store`, строго test-first, с задачи e2_t1.
> Соблюдай принципы (SOLID/DRY/KISS/TDD/чистая архитектура) и самопроверку из AGENTS.md.
