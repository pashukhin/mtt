# CLAUDE.md — mtt

Тонкая точка входа для агентов. Полные правила — в [AGENTS.md](AGENTS.md),
архитектура — в [DESIGN.md](DESIGN.md), план задач — в [TASKS.md](TASKS.md).

**В начале сессии прочитать:** AGENTS.md → DESIGN.md → TASKS.md.

## Что это

`mtt` — минималистичный файловый таск-трекер (Go CLI) для кодовых агентов и людей.
Хранение: YAML, один файл на задачу, каталог `.mtt/`. Цель — заменить beads.

## Правила без компромиссов (детали — AGENTS.md)

- **Тест — до кода** (TDD: red → green → refactor). `make check` зелёный до коммита.
- Фанатично: **SOLID, DRY, KISS, чистая архитектура**. Зависимости внутрь:
  `cli → engine/store → model`; `model` не знает про CLI/файлы/YAML.
- Ветка+PR на задачу → CI зелёный → squash в `main`.
- Данные `.mtt/` меняются **только** через `store`.
- Каждый пакет в `internal/` держит свой тонкий `CLAUDE.md` в актуальном состоянии.

## Гейт

`make check` = gofmt + go vet + golangci-lint(v2) + `go test -race -cover` + build.

## Skills / guards

Плагин **superpowers** (skills: TDD, brainstorming, debugging, planning) — это **личное**
требование к процессу разработки, а не проектное: включается в `.claude/settings.local.json`
(per-user, gitignored). Инструкция активации — в [NEXT_SESSION.md](NEXT_SESSION.md).
