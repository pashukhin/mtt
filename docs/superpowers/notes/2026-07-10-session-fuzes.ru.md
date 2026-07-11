# Затравки (fuzes) для следующих сессий — s009 / s009.5 (s008.97 ✅ и s008.98 ✅ смёржены)

> Human-facing, по-русски (осознанное исключение из правила «заметки — английские»): готовые session-start
> промпты для более простой модели. Использование: скопировать блок нужной сессии целиком в новый чат.
> Написаны 2026-07-10 по итогам аналитической сессии — контекст в
> [выжимке](2026-07-10-analysis-session-summary.ru.md); пункты R*/U*/S*/A*/SEC*/T*, на которые ссылаются
> затравки, — в трёх заметках рядом. После мёржа каждой сессии затравка следующей может требовать
> актуализации (версия/статус в первой строке — сверить с реальностью).
>
> **Актуализация 2026-07-10:** s008.97 (hardening) и s008.98 (named transitions + edge-verb sugar) **смёржены**
> (версия `0.8.98-dev`). Затравка s009 ниже **обновлена** под пересмотренный флоу (flow-granularity note §9,
> решения C–G) — старый линейный `speccing → planning → …` заменён двухзвенкой phase→session с per-artifact
> двухстадийным ревью на именованных рёбрах. Затравка s008.97 оставлена как исторический артефакт (✅ done).

## Затравка s008.97 (dogfood hardening, chore) — ✅ СМЁРЖЕНА (PR #21), артефакт

```
Продолжаем mtt. Сессии 001–008.95 смёржены в main, версия 0.8.9-dev, make check + CI зелёные. 2026-07-10
прошла аналитическая сессия: роадмап перегруппирован (s008.97 hardening → s009 dogfood → s009.5 release
positioning → тег v0.9.0), все находки — в трёх заметках docs/superpowers/notes/ с ID-пунктами (R*/U* —
positioning-and-agent-ux; S*/A* — s009-readiness-and-architecture-audit; SEC*/T* — debt-security-tests).
Каждый пункт самодостаточен: симптом → репро → фикс-скетч с якорями file:line → acceptance. Общайся по-русски.

Прочитай сначала (в порядке): CLAUDE.md → AGENTS.md → NEXT_SESSION.md (секция «Next task — chore 008.97» —
это твой скоуп; + «Carry-over lessons» 008.9/008.7) → docs/superpowers/notes/2026-07-09-positioning-and-
agent-ux-analysis.md (пункты U2, U3, U4, U5 + §6 «Strengths — do not regress») → docs/superpowers/notes/
2026-07-09-s009-readiness-and-architecture-audit.md (пункты A1, A2, S7 + «Verified CLEAN» — это базлайн, его
НЕ «чинить») → docs/superpowers/notes/2026-07-10-debt-security-tests-triage.md (пункты T6, T7) → TASKS.md
(e5_t1d) → sessions/README.md (строка 008.97) → CLI_REFERENCE.md (таблица exit-кодов). Убедись, что
superpowers-скиллы активны.

Это chore с УЖЕ принятыми решениями — полный brainstorming не нужен: сразу writing-plans по пунктам
(каждый пункт заметки = задача плана со своим acceptance), отдай план на адверсариальное субагент-ревью,
затем строго TDD до зелёного make check. Единственная микро-развилка — форма U2 (хвост вывода упавшей
команды по умолчанию vs подсказка -v/--log-file в ошибке): рекомендация — хвост ~10 строк + подсказка;
если сомневаешься — спроси, не гадай. Ветка feat/s008.97-hardening от свежего main; ветка → PR → CI green
→ сквош в main. Версия 0.8.9-dev → 0.8.97-dev (point-session → патч).

Scope (фиксированный, не расширять): U2 (blocked gate показывает ПОЧЕМУ упал); A1 (ошибки List/Get несут
путь битого файла; тесты на corrupt + zero-byte task-файл — это же T1); A2 (маппинг Status.Default в
ymlStatus + toDomain + golden с двумя initial); U3 (add --json эмитит созданную задачу через taskJSON;
history-массив в show --json); U4 (Use: "status [<id>] <new-status>", verb sugar в root Long, подсказка
`mtt init` в ошибке «no .mtt/»); U5 (тэглайн root.go:22 называет гейт-фичу, README консистентен); rm -rf
bin/.mtt; T6 (вынести per-command-timeout e2e из git-гейтед скрипта structured_commands.txt); T7 (тест
-v-стриминга). НЕ трогать: U1/rollback-паттерн в DESIGN (это s009), позиционные правки README (это s009.5).

Heed «Carry-over lessons»: форматтер-ловушка show.go vs format.go (грепни перед правкой — урок 008.7);
golangci unused (символ объявляй там, где ВПЕРВЫЕ используется); testscript без пайпов; e2e ассертит
отношения, не позиции; make check ПЕРЕД каждым коммитом; отражай новое поведение в --help. Docs-sync tick:
CLI_REFERENCE.md/.ru (add --json, status usage, подсказка init, хвост гейта), DESIGN.md/.ru (фраза «commands'
own output is hidden by default» — уточнить про хвост при блоке), README/.ru (тэглайн), TASKS e5_t1d ✅,
sessions/README 008.97 ✅, NEXT_SESSION (+carry-over 008.97), создай и заполни sessions/008.97_hardening.md
из 000_template.md.

После s008.97 → s009 dogfood. PARKED — не делать: advance/start/done + режимы + роли; node-level
status-actions; кросс-рёберная компенсация. Фанатично: SOLID/DRY/KISS/TDD/DDD/clean-arch + self-check из
AGENTS.md.
```

## Затравка s009 (dogfood)

```
Продолжаем mtt. Сессии 001–008.98 смёржены в main, версия 0.8.98-dev, make check + CI зелёные. s009 —
dogfood/self-host. Общайся по-русски.

ВАЖНО про флоу: он ПЕРЕСМОТРЕН в обсуждении 2026-07-10 ПОСЛЕ s008.98 (named transitions + edge-verb sugar).
Актуальные решения — flow-granularity note §9 (пункты C–G); они РАСШИРЯЮТ decision A (§8) и МЕНЯЮТ типы
(двухзвенка вместо трёх). Спека s009 и sessions/009_dogfood.md УСТАРЕЛИ (писались до A и до §9) — бери их как
БАЗЛАЙН, не как авторитет. Сессия начинается с брейнсторма флоу (добить открытые детали §9) + реконсиляции
доков, НЕ с кода. Догфуд — это ещё и ПЕРВЫЙ реальный прогон s008.98 (edge-verbs) на живом флоу.

Прочитай сначала (в порядке): CLAUDE.md → AGENTS.md → docs/superpowers/notes/2026-07-09-flow-granularity-
for-dogfood.md ЦЕЛИКОМ (§9 «Decided in the s009 flow discussion» C–G — твой АКТУАЛЬНЫЙ флоу; §8 A/B — контекст;
§1–7 — грамматика: что вешать на статус/ребро/гейт) → docs/superpowers/notes/2026-07-09-s009-readiness-and-
architecture-audit.md Part A (S1–S8 — стартовый чеклист; S2 идемпотентная ветка, S3 приоритеты миграции, S4/S5
процесс; S7 bin/.mtt УЖЕ сделан в 008.97) → спека s009 (docs/superpowers/specs/2026-07-09-session-009-dogfood-
design.md — БАЗЛАЙН для реконсиляции, НЕ авторитет) → sessions/009_dogfood.md → NEXT_SESSION.md («Where we are»:
s008.98; «Carry-over lessons» 008.98/008.9/008) → DESIGN.md («Flow: executable transitions»; блок «Shipped
(s008.98, named transitions)») → TASKS.md (e5_t2) → docs/superpowers/notes/2026-07-10-debt-security-tests-
triage.md (SEC2). Убедись, что superpowers-скиллы активны.

Флоу (§9 C–G — это ОСНОВА; открытые детали добей брейнстормом):
- (C) Двухзвенка: типы phase(p) / session(s, default). НЕТ step/subtask — rich-session делает инкремент-тир
  избыточным (сабтаск добавляется, только если реализация честно расщепляется).
- (D) session = двухстадийный per-artifact review: для КАЖДОГО артефакта (design, plan, implementation)
  do → <stage>_review (адверсариальный агент) → <stage>_human_review (человек) → next; declined → <stage>_fix
  → <stage>_review (bounce считает переделки — history-as-signal). Форки approve/decline — ИМЕНОВАННЫЕ рёбра
  (s008.98!); имя ребра ДИЗЪЮНКТНО с именем статуса (инвариант — значит approve/decline/rework/submit, НЕ
  review/done).
- (E) human-review = advisory + require:{who,why} (ролей нет — запаркованы; require делает самоаппрув видимым
  в history как by:агент). НЕ распарковывай роли — это записанный think-item (первый реальный триггер).
- (F) artifact-гейт = дешёвый прокси `git status --porcelain | grep -q .` (есть незакоммиченное) там, где
  проверяемо; иначе инструкция + judgment-ребро approve/decline (§6: date-slug доки под {{.ID}} не гейтятся).
- (G) CI = make check В флоу на implementation-review-рёбрах (repo-global, ключ не нужен); CD ВНЕ флоу (релиз
  = событие milestone, тег → release.yml, user-triggered v0.9.0 в s009.5) — на рёбрах сессии его НЕТ.
- Ветка + current:set на входном ребре сессии, идемпотентно `git switch -c feat/{{.ID}} || git switch
  feat/{{.ID}}` (S2; БЕЗ rollback — паттерн git branch -D СЛОМАН, U1); current:clear на approve→done.
- phase: коарс tbd → in_progress → done (+cancelled), опц. self-ref «все сессии-дети терминальны» (§4/B — скупо).

Ещё ОТКРЫТО (реши в брейнсторме, хвост §9): точные имена рёбер + расстановка гейтов по трём стадиям; берёт ли
phase self-ref-гейт в v1; set/clear-рёбра; складываем ли v1 две подстадии ревью (_review + _human_review) в
ОДНУ ради меньшего числа статусов (start-simpler-and-enrich vs полный шейп сразу). Реши это → реконсиль спеку
Q3 + sessions/009_dogfood.md + TestRepoDogfoodConfig-assertions под итог → добавь строки SEC2 («в гейтах —
только read-only команды mtt») и S4 («.mtt-мутации коммитятся с session-PR; после закрытия — git add .mtt +
commit») → writing-plans → адверсариальное субагент-ревью плана → строго TDD. Если неоднозначно — спроси.

Ветка feat/s009-dogfood от свежего main (её ещё НЕТ — создай). Версия 0.8.98-dev → 0.9.0-dev (полная сессия
→ минор).

Scope: committed .mtt/config.yaml (типы phase/session; флоу выше); forward-only миграция бэклога через
./bin/mtt add (Phase-4 сессии: references/comments/profiles/coding-demo + НОВАЯ «dangerous-ops attribution» —
elevated-пункт в TASKS Later; голые Phase-5…8 эпики С --priority low — S3); ПЕРЕД коммитом tasks/*.yaml
прогони mtt roadmap руками, проверь порядок глазами (S3: равный ранг = recency desc + ID-строка, зависит от
секунд); TestRepoDogfoodConfig (единственный страж коммитнутого конфига — Validate НЕ зовётся на Load;
проверь КАЖДЫЙ тип, включая новые именованные рёбра/инварианты); e2e dogfood.txt на scratch-конфиге с
fake-командами ([!exec:git] skip; git symbolic-ref для unborn branch — урок 007; `mtt types` ДО первого хода
— он валидирует, §9-precondition); TASKS.md заморозить (баннер + e5_t2 ✅).

Heed «Carry-over lessons» (особенно 008.98): роутинг новой формы в СТАРЫЙ путь — ядро (core.Transitioner) НЕ
трогать; «core untouched» безопасно УСЛОВНО на инвариантах — путь перехода НЕ ревалидирует (валидация на
add/types, не на Load); два адверс-ревью (спека, потом план) каждый ловит дефект self-review; named-edge verb
≠ имя статуса; testscript без пайпов; e2e ассертит отношения, не позиции; make check перед каждым коммитом.
Docs-sync tick: DESIGN.md/.ru (заметка «Dogfooding / self-host» + bootstrap-caveats из спеки), CLI_REFERENCE.md/
.ru (минимально), TASKS e5_t2 ✅ + заморозка, sessions/README 009 ✅, NEXT_SESSION (+carry-over 009, next =
s009.5), sessions/009_dogfood.md Done. После s009 → s009.5 release positioning → тег v0.9.0. PARKED — не делать:
advance/start/done + режимы + роли (даже под human-review — только advisory+require); node-level status-actions;
кросс-рёберная компенсация; monotonic minting / lost-update (один общий брейншторм ПОЗЖЕ). Фанатично:
SOLID/DRY/KISS/TDD/DDD/clean-arch + self-check из AGENTS.md.
```

## Затравка s009.5 (release positioning, chore)

```
Продолжаем mtt. s009 (dogfood) смёржен: mtt ведёт свою разработку сам (.mtt/ коммитнут, TASKS.md заморожен),
версия 0.9.0-dev, make check + CI зелёные. s009.5 — последний chore перед ПЕРВЫМ публичным тегом v0.9.0:
релизная поверхность позиционирования + два мелких контрактных фикса. Общайся по-русски. DOGFOOD-ПРАВИЛО:
эта сессия ведётся через сам mtt — добавь её задачей (mtt add ... --parent <Phase-4 id>), веди через flow
сессии (глаголы-рёбра из конфига, который зафиксировал s009 — смотри `mtt types`; НЕ линейный
speccing→done — флоу двухстадийный per-artifact, см. flow-granularity §9), .mtt-мутации коммить в PR сессии.

Прочитай сначала (в порядке): CLAUDE.md → AGENTS.md → docs/superpowers/notes/2026-07-09-positioning-and-
agent-ux-analysis.md ЦЕЛИКОМ (R0–R3 — твой основной скоуп; §2 — таблица «harness hooks vs mtt»; §3 —
таблица правок скана; §4 — драфт AGENTS.md-сниппета; Appendix — фактура) → docs/superpowers/notes/
2026-07-09-s009-readiness-and-architecture-audit.md (A5, A7) → docs/superpowers/notes/2026-07-10-debt-
security-tests-triage.md (SEC3, T5, строка про config-review-as-code) → docs/superpowers/notes/
2026-07-10-launch-plan.md (что публикуется после тега) → NEXT_SESSION.md → TASKS.md (e5_t2a) → RELEASING.md.
Убедись, что superpowers-скиллы активны.

Решения приняты — brainstorming не нужен: writing-plans по пунктам → адверсариальное субагент-ревью → TDD
для кодовой части. Ветка feat/s009.5-release-positioning от свежего main → PR → CI green → сквош. Версию НЕ
бампай (прецедент 008.95: release-prep почти version-neutral; тег v0.9.0 создаёт ЧЕЛОВЕК после мёржа — только
подготовь; если решишь иначе — спроси).

Scope: R1 — подсекция «Why not harness hooks?» в README + DESIGN «Positioning» (таблица из §2: per-type /
in-repo-data / harness-portable / весь lifecycle / audit history); R2 — правки конкурентного скана DESIGN.md
СТРОГО по таблице §3 (beads: custom statuses global + bd gate create как async-wait + embedded Dolt default +
bidirectional sync, Dolt primary — bet #2 не оспорена; Backlog.md ~6k★ + DoD-чеклист не executable; osmove
5★ — демote; дата скана); R0 — 1–2 цифры problem-validation в README (issue #25305 «75% rework», 19/19
false-positive); R3 — copy-paste сниппет «For your AGENTS.md / CLAUDE.md» в README (драфт в §4 — проверь,
что каждая команда сниппета существует и работает); A7 — pkg/mtt.ErrUnsupported (godoc Delete уже ссылается
на него!) + осознанный маппинг в exitCode + строка в CLI_REFERENCE; A5 — реши exit-код stale current (1 vs 4;
рекомендация: обернуть ErrNotFound → 4) + тесты T5 (stale current в bare-verb sugar и mtt use); SEC3 —
строка честности про Windows («gates: documented, not CI-tested») в README/RELEASING; строка «ревьюйте диффы
.mtt/config.yaml как код» в AGENTS.md/README. БИЛИНГВА ОБЯЗАТЕЛЬНА: README.ru, DESIGN.ru, CLI_REFERENCE.ru —
синхронно, в ту же сессию.

Heed: формулировки питча НЕ выдумывай — бери из заметки 1 (они выверены по источникам 2026-07-09); зеркаль
.ru-доки по абзацам; make check перед каждым коммитом; отражай ErrUnsupported/exit-код в --help и таблице
exit-кодов. Docs-sync tick: TASKS e5_t2a ✅ (файл заморожен — только галочка), sessions/README 009.5 ✅,
NEXT_SESSION (+carry-over 009.5, next = references s010 + «после мёржа человек тегает v0.9.0, затем
launch-план»), sessions/009.5_*.md Done. После s009.5 → тег v0.9.0 (человек) → публикации по
launch-plan.md → s010 references. PARKED — не делать: advance и всё из списка; подпись релизов
(SEC4) — после v0.9.0. Фанатично: SOLID/DRY/KISS/TDD/DDD/clean-arch + self-check из AGENTS.md.
```