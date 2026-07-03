# CLAUDE.md — mtt

Тонкая точка входа для агентов. Полные правила — в [AGENTS.md](AGENTS.md),
архитектура — в [DESIGN.md](DESIGN.md), план задач — в [TASKS.md](TASKS.md).

**В начале сессии прочитать:** AGENTS.md → DESIGN.md → TASKS.md.

## Что это

`mtt` — агент-дружелюбная лёгкая связка «задачи + знания» (Go CLI), как Jira+Confluence
без монструозности. Хранилище абстрагировано за портами; дефолт — YAML, один файл на задачу
в `.mtt/`, но подключается и внешняя связка (Jira+Confluence и пр.) через адаптер.

## Правила без компромиссов (детали — AGENTS.md)

- **Тест — до кода** (TDD: red → green → refactor). `make check` зелёный до коммита.
- Фанатично: **SOLID, DRY, KISS, чистая архитектура** (hexagonal). Зависимости внутрь:
  `cli → core → порт ← adapter`; контракт (домен-типы + порты) — в публичном `pkg/mtt`.
- Ветка+PR на задачу → CI зелёный → squash в `main`.
- Хранилище — **только через порт** (`TaskStore`/`KnowledgeStore`); YAML-адаптер по умолчанию.
- Каждый пакет в `internal/` держит свой тонкий `CLAUDE.md` в актуальном состоянии.

## Гейт

`make check` = gofmt + go vet + golangci-lint(v2) + `go test -race -cover` + build.

## Skills / guards

Плагин **superpowers** (skills: TDD, brainstorming, debugging, planning) — это **личное**
требование к процессу разработки, а не проектное: включается в `.claude/settings.local.json`
(per-user, gitignored). Инструкция активации — в [NEXT_SESSION.md](NEXT_SESSION.md).
