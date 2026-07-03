# TASKS

Bootstrap-трекер до самохостинга. Как только появятся задачи+иерархия+зависимости
(конец фазы 4) — разработка mtt переезжает на сам mtt, и этот файл замораживается.

Идентификаторы имитируют будущую схему mtt (`e{N}_t{M}`) для наглядности.
Порядок и архитектура — в [DESIGN.md](DESIGN.md); правила — в [AGENTS.md](AGENTS.md).

Легенда: `[ ]` todo · `[~]` in progress · `[x]` done.

---

## e1 — Фаза 0: каркас проекта  `[x]`

- [x] e1_t1 — git init, go-модуль `github.com/pashukhin/mtt`, ветка `main`
- [x] e1_t2 — скелет CLI: `cmd/mtt` + `internal/cli` (root + `version`) + тест
- [x] e1_t3 — гейт: Makefile `make check`, `.golangci.yml` (v2), `.gitignore`
- [x] e1_t4 — CI: `.github/workflows/ci.yml` (тот же гейт)
- [x] e1_t5 — DESIGN.md, AGENTS.md, README.md
- [x] e1_t6 — guards: принципы (SOLID/DRY/KISS/TDD), иерархические CLAUDE.md, superpowers

## e2 — Фаза 1: модель + файловое хранилище + базовые команды  `[ ]`

Делать test-first, каждую подзадачу — ветка+PR.

- [ ] e2_t1 — `internal/model`: типы `Task`, `Comment` (порядок полей = порядок сериализации)
      + `internal/model/CLAUDE.md`
- [ ] e2_t2 — `internal/store`: чтение/запись YAML, детерминированная сериализация,
      атомарная запись (temp+rename), поиск корня `.mtt/`  + `internal/store/CLAUDE.md`
- [ ] e2_t3 — генерация ID `e{N}` / `e{N}_t{M}` / `e{N}_t{M}_s{K}` (max+1 в области,
      `O_EXCL` при создании) + тесты на гонки/коллизии
- [ ] e2_t4 — golden-тесты сериализации задачи (флаг `-update`)
- [ ] e2_t5 — команда `mtt add` (эпик/задача/подзадача, `--parent`, `--type`, `--title`)
- [ ] e2_t6 — команда `mtt list` (фильтры: статус/тип/родитель; стабильный порядок)
- [ ] e2_t7 — команда `mtt show <id>`
- [ ] e2_t8 — команды `mtt edit` / `mtt close` (смена полей/статуса)
- [ ] e2_t9 — первый `testscript`-сценарий e2e для add/list/show
- [ ] e2_t10 — `init`: команда `mtt init` (создать `.mtt/config.yaml` с дефолтным типом)

## e3 — Фаза 2: иерархия, зависимости, ready  `[ ]`

- [ ] e3_t1 — `internal/engine`: индекс задач в память, обход иерархии
- [ ] e3_t2 — `depends_on`: добавление/снятие, валидация существования
- [ ] e3_t3 — детект циклов зависимостей
- [ ] e3_t4 — вычисление `ready` + команда `mtt ready`
- [ ] e3_t5 — `mtt tree` (иерархический вывод)

## e4 — Фаза 3: типы задач + flow  `[ ]`

- [ ] e4_t1 — модель типов/переходов в `config.yaml`
- [ ] e4_t2 — валидация перехода статуса по flow (+ текстовые условия)
- [ ] e4_t3 — `mtt status <id> <new>` с проверкой перехода; `mtt types` (просмотр)

## e5 — Фаза 4: комментарии (дерево)  `[ ]`

- [ ] e5_t1 — `mtt comment add <id> [--reply <cid>]`
- [ ] e5_t2 — вывод дерева комментариев в `show`
- [ ] e5_t3 — **догфудинг**: перевести этот трекер на сам mtt

## Далее (крупно)

- e6 — Фаза 5: база знаний + текстовый поиск
- e7 — Фаза 6: текстовый/ASCII Гант, богатый list/query
- e8 — Фаза 7: `mtt serve` (web UI, Гант SVG, браузер БЗ)
- e9 — Фаза 8: hook внешнего индексатора
- release — goreleaser, кросс-бинарники по тегам
