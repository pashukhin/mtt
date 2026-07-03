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

## e2 — Фаза 1: контракт `pkg/mtt`, конфиг, `mtt init`, YAML-адаптер, core, команды  `[ ]`

Test-first, каждую подзадачу — ветка+PR. **Начать с планирования** (см. NEXT_SESSION.md);
разбивка ниже — ориентир, план её уточнит. Инварианты: типы/иерархия/ID — из конфига (без
литералов в коде); хранилище — за портом, `core` не импортирует `adapter/*`.

- [ ] e2_t1 — планирование Фазы 1 (superpowers), сверка с инвариантами DESIGN.md
- [ ] e2_t2 — контракт `pkg/mtt`: домен-типы (`Task`, `Comment`, `Type`, `Flow`, `Status`,
      `Config`) + порт `TaskStore` + `pkg/mtt/CLAUDE.md`  (порядок полей = порядок сериализации)
- [ ] e2_t3 — конфиг: тип (`name/prefix/parent/initial/statuses/transitions`), валидация;
      дефолтный шаблон (epic/task/subtask + flow)
- [ ] e2_t4 — `mtt init`: запись дефолтного `.mtt/config.yaml`
- [ ] e2_t5 — `internal/adapter/yaml`: реализация `TaskStore` — детерминированная сериализация,
      атомарная запись (temp+rename), поиск корня `.mtt/`, загрузка конфига + `.../yaml/CLAUDE.md`
- [ ] e2_t6 — `internal/core`: usecase-слой; генерация ID из конфига (`<prefix><N>` по цепочке
      родителей, `max+1`, `O_EXCL` через порт), валидация parent-типа; тесты на коллизии;
      **без хардкод-префиксов** + `internal/core/CLAUDE.md`
- [ ] e2_t7 — golden-тесты сериализации задачи и конфига (флаг `-update`)
- [ ] e2_t8 — `mtt add` (тип из конфига, `--parent`, `--title`)
- [ ] e2_t9 — `mtt list` (фильтры: статус/тип/родитель; стабильный порядок) + `mtt show <id>`
- [ ] e2_t10 — `mtt edit` / `mtt close` (смена полей/статуса)
- [ ] e2_t11 — первый `testscript`-сценарий e2e: init → add → list → show

## e3 — Фаза 2: иерархия, зависимости, ready  `[ ]`

- [ ] e3_t1 — `internal/core`: индекс задач в память, обход иерархии
- [ ] e3_t2 — `depends_on`: добавление/снятие, валидация существования
- [ ] e3_t3 — детект циклов зависимостей
- [ ] e3_t4 — вычисление `ready` + команда `mtt ready`
- [ ] e3_t5 — `mtt tree` (иерархический вывод)

## e4 — Фаза 3: enforcement flow  `[ ]`

(Модель типов/flow уже введена в Фазе 1; здесь — только применение переходов.)

- [ ] e4_t1 — валидация перехода статуса по `transitions` типа (+ показ текстовых условий)
- [ ] e4_t2 — `mtt status <id> <new>` с проверкой перехода
- [ ] e4_t3 — `mtt types` (просмотр типов/flow из конфига)

## e5 — Фаза 4: комментарии (дерево)  `[ ]`

- [ ] e5_t1 — `mtt comment add <id> [--reply <cid>]`
- [ ] e5_t2 — вывод дерева комментариев в `show`
- [ ] e5_t3 — **догфудинг**: перевести этот трекер на сам mtt

## Далее (крупно)

- e6 — Фаза 5: база знаний + текстовый поиск  _(низкий приоритет: у beads уже есть аналог)_
- e7 — Фаза 6: текстовый/ASCII Гант, богатый list/query
- e8 — Фаза 7: `mtt serve` (web UI, Гант SVG, браузер БЗ)
- e9 — Фаза 8: hook внешнего индексатора
- release — goreleaser, кросс-бинарники по тегам
