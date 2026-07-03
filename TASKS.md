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
разбивка ниже — ориентир, план её уточнит. Инварианты: типы/иерархия — из конфига (без
литералов в коде); ID/slug минтит **адаптер**; в наборе типов есть дефолтный `task`; в flow —
`tbd→in_progress→done`; хранилище — за портом, `core` не импортирует `adapter/*`.

- [ ] e2_t1 — планирование Фазы 1 (superpowers), сверка с инвариантами DESIGN.md
- [ ] e2_t2 — контракт `pkg/mtt`: домен-типы (`Task` c `history[]`+`refs[]`, `Comment` c `refs[]`,
      `Ref` {kind,id,label}, `Type`, `Flow`, `Status` c `kind`, `Transition`, `Config`); базовый
      `TaskStore` + опциональные capability-интерфейсы (`HistoryStore`, `DependencyStore`,
      `CommentStore`, `SearchStore`), `Capabilities()`, `ErrUnsupported` + `pkg/mtt/CLAUDE.md`
      (порядок полей = порядок сериализации)
- [ ] e2_t3 — конфиг: тип (`name/parent/statuses(c kind)/transitions`; `prefix` — поле YAML),
      валидация инвариантов (дефолт `task`; статусы-якоря `tbd`/`in_progress`/`done` с категориями;
      ровно один `initial`, ≥1 `terminal`, в дефолте ещё `cancelled`); дефолтный шаблон
- [ ] e2_t4 — `mtt init`: запись дефолтного `.mtt/config.yaml`
- [ ] e2_t5 — `internal/adapter/yaml`: реализация `TaskStore` **и всех capability-интерфейсов**
      (референс) — **минтинг ID** (`<prefix><N>` по цепочке родителей, `max+1`, `O_EXCL`),
      детерминированная сериализация, атомарная запись (temp+rename), поиск корня `.mtt/`,
      загрузка конфига + `.../yaml/CLAUDE.md`
- [ ] e2_t6 — `internal/core`: usecase-слой (add/list/show/edit/close); валидация parent-типа;
      создаёт логическую задачу, ID запрашивает у `TaskStore` + `internal/core/CLAUDE.md`
- [ ] e2_t7 — golden-тесты сериализации задачи и конфига (флаг `-update`)
- [ ] e2_t8 — `mtt add` (тип из конфига, `--parent`, `--title`)
- [ ] e2_t9 — `mtt list` (фильтры: статус/тип/родитель; стабильный порядок) + `mtt show <id>`
- [ ] e2_t10 — `mtt edit` / `mtt close` (смена полей/статуса)
- [ ] e2_t11 — первый `testscript`-сценарий e2e: init → add → list → show

## e3 — Фаза 2: иерархия, зависимости, ready  `[ ]`

(Зависимости — capability `DependencyStore`; при отсутствии у адаптера — `ErrUnsupported`.)

- [ ] e3_t1 — `internal/core`: индекс задач в память, обход иерархии
- [ ] e3_t2 — `depends_on`: добавление/снятие, валидация существования
- [ ] e3_t3 — детект циклов зависимостей
- [ ] e3_t4 — вычисление `ready` + команда `mtt ready`
- [ ] e3_t5 — `mtt tree` (иерархический вывод)
- [ ] e3_t6 — резолв `refs` вида `task`/`comment` (верификация существования) + backlinks в `show`

## e4 — Фаза 3: flow-enforcement + исполняемые переходы (killer-фича)  `[ ]`

(Модель типов/flow уже введена в Фазе 1; здесь — применение переходов и исполнение команд.)

- [ ] e4_t1 — валидация перехода статуса по `transitions` типа (+ показ `description`)
- [ ] e4_t2 — порт `Runner` (в `core`) + `internal/adapter/exec` (запуск команд; таймаут,
      cwd=корень); фейк для тестов
- [ ] e4_t3 — исполнение `commands` перехода по порядку, гейтинг по exit-кодам (переход
      блокируется на первом ненулевом); флаг `--no-run`
- [ ] e4_t4 — запись перехода в `history` задачи (from→to, at, by, результаты `checks`), append-only
      (capability `HistoryStore`; при отсутствии — мягкая деградация)
- [ ] e4_t5 — `mtt advance <id> --to <status>` — мета-обход до цели (прогрессирующие рёбра,
      стоп на развилке, защита от циклов, не в чужой терминал); режимы `--stop`(деф)/`--atomic`/
      `--force`; `mtt start`/`mtt done` — алиасы; `mtt status <id> <new>` — одиночный переход
- [ ] e4_t6 — `mtt types` (типы/flow из конфига) + `mtt caps` (возможности текущего бэкенда)
- [ ] e4_t7 — `ready`/`list`/завершённость — **по категории** статуса (не по литералу `done`)

## e5 — Фаза 4: комментарии (дерево)  `[ ]`

- [ ] e5_t1 — `mtt comment add <id> [--reply <cid>]`
- [ ] e5_t2 — вывод дерева комментариев в `show`
- [ ] e5_t3 — **догфудинг**: перевести этот трекер на сам mtt

## Далее (крупно)

- e6 — Фаза 5: KB (`KnowledgeStore`) + текстовый поиск; резолв `refs` вида `note`; `mtt check`
  (висячие ссылки) + backlinks  _(KB — низкий приоритет; у beads есть аналог)_
- e7 — Фаза 6: текстовый/ASCII Гант, богатый list/query
- e8 — Фаза 7: `mtt-ui` (опц., отдельный бинарь: web UI, Гант SVG, браузер БЗ)
- e9 — Фаза 8: hook внешнего индексатора
- later — реконструкция наблюдаемого графа статусов из `history` задач (read-only агрегация);
  явное версионирование/миграции flow (пока хватает git-истории конфига)
- release — goreleaser, кросс-бинарники по тегам
