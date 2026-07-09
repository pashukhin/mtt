# Затравки (fuzes) для следующих сессий — s008.97 / s009 / s009.5

> Human-facing, по-русски (осознанное исключение из правила «заметки — английские»): готовые session-start
> промпты для более простой модели. Использование: скопировать блок нужной сессии целиком в новый чат.
> Написаны 2026-07-10 по итогам аналитической сессии — контекст в
> [выжимке](2026-07-10-analysis-session-summary.ru.md); пункты R*/U*/S*/A*/SEC*/T*, на которые ссылаются
> затравки, — в трёх заметках рядом. После мёржа каждой сессии затравка следующей может требовать
> актуализации (версия/статус в первой строке — сверить с реальностью).

## Затравка s008.97 (dogfood hardening, chore)

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
Продолжаем mtt. Сессии 001–008.97 смёржены в main, версия 0.8.97-dev, make check + CI зелёные. s009 —
dogfood/self-host. ВАЖНО: спека s009 СУЩЕСТВУЕТ (docs/superpowers/specs/2026-07-09-session-009-dogfood-
design.md), но она ПРОТИВОРЕЧИТ более позднему решению A — сессия начинается с реконсиляции спеки, не с
кода. Общайся по-русски.

Прочитай сначала (в порядке): CLAUDE.md → AGENTS.md → docs/superpowers/notes/2026-07-09-s009-readiness-
and-architecture-audit.md, Part A (S1–S8 — это твой стартовый чеклист) → docs/superpowers/notes/
2026-07-09-flow-granularity-for-dogfood.md (решения A и B в §8) → спека s009 (выше) → sessions/009_dogfood.md
→ NEXT_SESSION.md («Next task», абзац про s009; «Carry-over lessons» 008.9/008.6/008/007) → DESIGN.md
(«Flow: executable transitions», «Dependencies», «Priorities and roadmap») → TASKS.md (e5_t2) →
docs/superpowers/notes/2026-07-10-debt-security-tests-triage.md (SEC2) → CLI_REFERENCE.md. Убедись, что
superpowers-скиллы активны.

Шаг 0 — реконсиляция (docs-коммит ДО плана): перепиши в спеке Q3 и sessions/009_dogfood.md под решение A —
session-flow tbd → speccing → planning → in_progress → review → done (+cancelled); speccing/planning —
отдельные статусы; description на КАЖДОМ ребре и статусе (это ранбук агента — s008.95 печатает их);
ветка + current:set на ребре tbd → speccing командой `git switch -c feat/{{.ID}} || git switch feat/{{.ID}}`
(S2: идемпотентно; БЕЗ rollback на этом ребре — задокументированный паттерн git branch -D СЛОМАН, см. U1);
make check на → review И на → done (S5: зафиксируй в спеке цену — сессия из 5 шагов ≈ 7 прогонов, принято);
поправь assertions TestRepoDogfoodConfig (ветка проверяется на ребре → speccing); добавь строку SEC2 («в
гейтах — только read-only команды mtt») и S4 («.mtt-мутации коммитятся с session-PR; после mtt done —
git add .mtt + commit»). Исправь сломанный rollback-пример в DESIGN.md/.ru (U1: `git checkout - && git
branch -D …`). Затем writing-plans → адверсариальное субагент-ревью плана → TDD. Если неоднозначно — спроси.
Старая ветка feat/s009-dogfood полностью влита в main — удали и создай заново от свежего main. Версия
0.8.97-dev → 0.9.0-dev (полная сессия → минор).

Scope (по спеке после реконсиляции): committed .mtt/config.yaml с типами phase(p)/session(s, default)/
step(t); forward-only миграция бэклога через ./bin/mtt add (Phase-4 сессии: references high, comments,
profiles, coding-demo low + НОВАЯ сессия «dangerous-ops attribution» — см. elevated-пункт в TASKS Later;
голые Phase-5…8 эпики С --priority low — S3); ПЕРЕД коммитом tasks/*.yaml прогони mtt roadmap руками и
проверь порядок глазами (S3: тай-брейк равного ранга — recency desc + ID-строка, порядок зависит от секунд);
TestRepoDogfoodConfig (единственный страж коммитнутого конфига — Validate НЕ зовётся на Load); e2e
dogfood.txt на scratch-конфиге с fake-командами ([!exec:git] skip; git symbolic-ref для unborn branch —
урок 007); TASKS.md заморозить (баннер + e5_t2 ✅).

Heed «Carry-over lessons»: валидация конфига живёт на add/types, не на Load; testscript без пайпов; e2e
ассертит отношения, не позиции; make check перед каждым коммитом. Docs-sync tick: DESIGN.md/.ru (заметка
«Dogfooding / self-host» + bootstrap-caveats из спеки), CLI_REFERENCE.md/.ru (минимально), TASKS e5_t2 ✅ +
заморозка, sessions/README 009 ✅, NEXT_SESSION (+carry-over 009, next = s009.5), sessions/009_dogfood.md
Done. После s009 → s009.5 release positioning → тег v0.9.0. PARKED — не делать: advance/start/done + режимы
+ роли; node-level status-actions; кросс-рёберная компенсация; monotonic minting / lost-update (один общий
брейншторм ПОЗЖЕ). Фанатично: SOLID/DRY/KISS/TDD/DDD/clean-arch + self-check из AGENTS.md.
```

## Затравка s009.5 (release positioning, chore)

```
Продолжаем mtt. s009 (dogfood) смёржен: mtt ведёт свою разработку сам (.mtt/ коммитнут, TASKS.md заморожен),
версия 0.9.0-dev, make check + CI зелёные. s009.5 — последний chore перед ПЕРВЫМ публичным тегом v0.9.0:
релизная поверхность позиционирования + два мелких контрактных фикса. Общайся по-русски. DOGFOOD-ПРАВИЛО:
эта сессия ведётся через сам mtt — добавь её задачей (mtt add ... --parent <Phase-4 id>), веди через flow
(mtt speccing → ... → mtt done), .mtt-мутации коммить в PR сессии.

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