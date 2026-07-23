# Ship the flagship flow: a flow-authoring guide (`t57`)

Status: spec (decision record). Type: task (`t57`). Branch: `task/t57`. Tags: `dx`, `flow`.

## Context / problem

The 2026-07-22 audit dry-ran a pilot migration by hand-copying this repo's ~320-line `.mtt/config.yaml` into
a foreign repo. The **whole pipeline works end-to-end** (spec/plan/impl reviews, the exit-5 PR tail, the
`deliver` main-push). But **nothing ships the ability to do that**:

- `mtt init` templates stop at `coding` (a `make lint`/`make test` gate, **no git integration** — no
  branch/auto-commit/PR/deliver machinery), and the whole set is dev-skewed;
- there is **no how-to-adapt guide**: what a foreign project must change (the gate command, artifact paths,
  `gh`+`jq` deps, the `task/<id>` branch name, the `config.local.yaml` gitignore, attribution) is tribal
  knowledge locked in this repo's committed config.

So adoption today = "copy our config and reverse-engineer it." `t57` closes the **documentation** gap.

## Framing decision (the load-bearing one)

**mtt's product is the executable-transition *engine*, not our flow.** The essence: a flow **binds the
transitions of your config-defined entities to external actions/artifacts**. The entity is anything (a task, a
document, a request, an experiment, a release, a ticket); the action/artifact is anything (running a script,
writing a file, a git operation, an HTTP call, a notification). **Running a test suite is just one instance** —
the *engine* is a generic gated-flow engine, and a flow is **per-project data** (`.mtt/config.yaml`). This is
the architecture (DESIGN: "There are no commands by default — the user hangs them for their own project";
type/status names are config, never code literals).

Nuance to state explicitly (so a reader moving between docs feels no whiplash): the **product's headline**
(README/DESIGN positioning) is the *agent gate* — "an agent can't type `done` without passing your checks" —
a dev/agent wedge. That is the go-to-market emphasis; it does **not** narrow the *engine*, which is
domain-neutral. The guide teaches the engine; coding is one worked domain.

Therefore our dogfood flow (branch → auto-commit → PR → deliver) is shipped as a **worked example explicitly
labelled "a sample: adapt or replace"**, never as an integral or mandated part of the tool. Our flow is
deeply opinionated — it assumes GitHub + `gh` + `jq` + the `task/<id>` branch model + a direct push to `main`
— which is itself the proof that it cannot be "the mtt flow."

## Resolved decisions (brainstorm, 2026-07-23)

1. **Guide-first; defer the shipped git-flagship template.** `t57` ships the adaptation **guide** only. The
   one-command `mtt init --template <flow>` for our git-integration flow is **deferred to `t24`** (global/default
   `post:` actions): the dogfood config repeats the same `post:` block heavily — **24×** for the auto-commit
   block, **14×** for the main-push block, across **12** `cancel` edges and the rest; `t24` collapses that
   duplication, so a *clean* flagship template should be minted only after it. The pilot (`t59`) proceeds via
   the guide (copy + adapt — exactly what the audit already proved works).
2. **Tight scope: flow authoring only.** The guide is strictly about authoring the flow in `.mtt/config.yaml`.
   Generic agent-usage docs (`t46`) and the settings/hooks scaffold (`t52`) stay **out**, reached by
   cross-link, not absorbed.
3. **A new top-level bilingual doc.** `FLOW_GUIDE.md` ↔ `FLOW_GUIDE.ru.md`, alongside
   `README`/`DESIGN`/`CLI_REFERENCE` (human-facing adoption content → bilingual per AGENTS.md; English is the
   source of truth, keep in sync).
4. **Universal *runnable* templates are split to a sibling task (`t62`).** Shipping domain-neutral
   `mtt init --template <name>` samples (content-review / approval / generic-script-gate) is code
   (template YAML + `templates.go` registration + goldens + e2e + per-template docs) and deserves its own
   design — a **different deliverable unit** from the guide. `t62` ships them (and later absorbs the git-flagship
   after `t24`). `t57` still carries the generality **in prose**: a domain-neutral spine plus 2-3 cross-domain
   example configs as illustrative snippets, and forward-links `t62`.

## Deliverable

- `FLOW_GUIDE.md` (EN, source of truth) + `FLOW_GUIDE.ru.md` (RU mirror).
- Discoverability: a one-line pointer to the guide from `README.md`/`README.ru.md` (and, if it fits, a "see
  FLOW_GUIDE" note where `CLI_REFERENCE` describes the flow/`types`) — no content duplication.
- **Register the new bilingual pair** `FLOW_GUIDE.md ↔ FLOW_GUIDE.ru.md` in the "Bilingual docs" list in
  **both** `AGENTS.md` (Documentation language) and the root `CLAUDE.md` (Docs language) — the rule is stated
  in both, so keep them in sync (the "parallel occurrences" trap).
- A forward-link to `t62` for the runnable universal templates.
- The diff is **docs-only** (plus the two-line language-rule registration). No engine change (the machinery is
  shipped); no new `init` template; no agent-usage or hooks docs.

## Guide outline (EN; the RU mirror follows the same structure)

1. **Mental model (domain-neutral spine)** — a flow binds each transition of your entities to external
   actions/artifacts; the engine gates the transition on your checks. Enumerate the generality: an *entity* is
   anything, an *action/artifact* is anything (script / file / git / HTTP / notification), and "run the tests"
   is one instance. "Your flow ≠ our flow." Point at `mtt init` templates as starting points and `mtt types`
   as the inspector.
2. **A minimal flow from scratch** — the smallest **valid** flow is **3 statuses**, one of each kind:
   `initial → active → terminal` (a 2-status flow is rejected by `Config.Validate` — every flow needs ≥1
   initial, ≥1 active, ≥1 terminal, and `kind` is derived from topology). Show one type, one `initial→active`
   gate, mirroring the `default` template's shape.
3. **The graph** — types (root vs hierarchy via `parents`), statuses + `kind` (initial/active/terminal) and
   the topology invariants (kind = topology; ≥1 of each; ≤1 default type/status), transitions (from/to), and
   **named edges** driving `mtt <edge>` / `mtt do`.
4. **Gates: executable transitions** — `commands:` (all exit 0 or the move is **blocked**, exit 3); the
   **global `command_timeout`** (adapter setting in `config.yaml`, **default 5m**, overridable in
   `config.local.yaml`) **and** the optional per-command `timeout` override (the dogfood `make check` gate sets
   `timeout: 10m` precisely because the 5m default would SIGKILL it — a real adopter footgun to call out);
   placeholders `{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}` + the shell-safety note. **Trust caution:** a flow
   config runs **arbitrary shell with your privileges** — commands are trusted config (like a Makefile), never
   network input, and ids are charset-validated at load; only run/commit a config you trust. Show a gate as
   **any external action** first (e.g. `./gate.sh`), then `make lint`/`make test` as *one* domain's instance.
5. **Finalization: `post:` actions** — run **after** the status is persisted (git add/commit, push, PR — or a
   deploy, a notification, an archive); the two-phase failure model (a gate failure blocks — exit 3, nothing
   persisted; a `post:` failure keeps the move — exit 5, print the remaining commands). **`rollback:`
   compensators** for side-effecting gates — taught from the DESIGN illustration `git checkout -b task/{{.ID}}`
   with `rollback: git branch -D task/{{.ID}}`, **clearly labelled as an illustration** (note: our own dogfood
   config does *not* use rollback — see §8).
6. **Attribution & guards** — `require: {who, why}` (global + per-edge, tighten-only), and — critically for a
   copy-adopter — **how `by` is resolved**: `--who`/`--by` > `MTT_BY` > `author:` in the gitignored
   `config.local.yaml`. Our shipped config sets `require: {who}`, so an adopter who copies it and sets none of
   these hits **exit 2** on the very first move; the guide must front-load this. Also the `current`-pointer
   effect (`Transition.Current: set|clear`).
7. **Flows for any domain (illustrative snippets)** — 2-3 short **prose** example configs showing the engine is
   not dev-specific: a content-review flow (draft → review → published, gate = a link/style check, post =
   publish), an approval/sign-off flow (requested → approved → done, gate = validate a request file), and a
   generic script-gated flow (a placeholder external action). Each is a **valid** (≥ initial/active/terminal)
   flow. Forward-link `t62` for the runnable versions (`mtt init --template …`).
8. **The git-integration pattern (worked example = our dogfood flow)** — a walk-through of this repo's real,
   committed config: branch on `start` via the **idempotent**
   `git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})` (**no rollback** — it is
   re-entrant by construction), auto-commit `.mtt` on every move, `approve` → `git push` + `gh pr create`
   (idempotent), `deliver`/`cancel` → push `main` with a **narrowed pathspec**. **Honest caveats:** it assumes
   GitHub + `gh` + `jq` + the `task/<id>` model + direct push to `main`; a short "how to swap for GitLab MR /
   no-PR / trunk-based / a non-development lifecycle." The heavy `post:` duplication (24×/14×) is shown as-is
   with a note that `t24` will let it be declared once and `t62` will ship the clean template.
9. **Adaptation checklist** (from the audit's gap list) — the concrete "copy our config, then change these":
   the gate command (your build/checks, not necessarily a Makefile), artifact paths
   (`docs/superpowers/pr/<id>.md`, spec/plan paths), tool deps (`gh`+`jq`), the branch name, the `.gitignore`
   for `config.local.yaml`, and attribution (`author:` in `config.local.yaml` — see §6).
10. **Exercise your flow locally** — the run loop that makes "author a *working* flow" real: `mtt init` a
    scratch project → `mtt add` a task → `mtt <status>`/`mtt do` a move and watch the gate (`▶`/`✓`/`✗`); use
    `--no-run` to iterate past a side-effecting gate; `-v` / `--log-file` to debug a failing gate; `mtt types`
    to inspect the graph + gates + edge verbs.
11. **Validate your flow (structure)** — `Config.Validate` runs on `mtt add`/`mtt types`; enumerate the
    structural invariants it enforces (kind↔topology, ≥1 of each kind, unique names, resolvable transition
    refs, ≤1 default) so the reader knows what will be rejected *before* runtime. (Reference integrity — `mtt
    check` over `depends_on`/refs — is a **separate** concern that `t58` strengthens; do not conflate it with
    structural flow validation.)
12. **Neighbours (out of scope here)** — brief cross-links: runnable universal templates (`t62`), agent-usage
    docs (`t46`), and the settings/hooks scaffold like `sessionStart → mtt prime` (`t52`).

## Verification (a docs task)

No new test harness. The guide is **anchored on live, already-tested artifacts** so it cannot silently rot:

- the git-integration worked example **is** this repo's committed `.mtt/config.yaml` (guarded by
  `TestRepoDogfoodConfig`, and exercised by the flow itself every session) — quote/reference it rather than
  inventing a parallel config;
- the minimal-flow and cross-domain snippets mirror the shipped `default`/`coding` templates and `demo/` (a
  runnable, tested coding-template walkthrough) — reference `demo/` for an executable end-to-end;
- **every hand-authored snippet in the guide (the minimal flow + the 2-3 cross-domain configs) is run through
  `mtt types` / a scratch `mtt init` while writing**, so an *invalid* flow can never ship in the guide (this is
  the exact class of error — a sub-3-status flow — that a static prose review would miss);
- every mechanism claim is cross-checked against DESIGN.md's "Shipped" blocks and CLI_REFERENCE before
  writing. The EN and RU versions are kept consistent (English is the source of truth).

## Out of scope / deferred

- Runnable universal `mtt init --template <name>` samples — **split to `t62`**.
- The git-integration flagship template — **deferred to `t24`** then shipped via `t62` (DRY the `post:`
  duplication first).
- Any change to the executable-transition engine — it is shipped and unchanged.
- Agent-usage docs (`t46`) and the settings/hooks scaffold (`t52`) — cross-linked, not built.

## Risks / notes

- **Size.** 12 sections + a full RU mirror is on the large side for one docs task. Acceptable, but if it
  balloons during writing, §7 (the 3 snippets) and §8 (the git walkthrough) are the compressible parts —
  trim depth there before cutting a mechanism.

## Success criteria

- A newcomer understands mtt as a **generic** gated-flow engine (not a dev tool) and can author a working flow
  (types/statuses/transitions + a gate) from the guide alone, then adapt the git-integration example to a
  non-GitHub / non-PR / non-dev process using the adaptation checklist — including setting attribution and a
  gate timeout so their first move doesn't hit exit 2 or a 5m SIGKILL.
- The guide never mandates our flow; our config appears only as a labelled, adaptable sample, described
  **accurately** (idempotent `git switch`, no rollback on start).
- Docs-only diff, `make check` green, EN/RU in sync, README/CLI_REFERENCE point at the guide, the language rule
  registers the new pair, `t62` linked.
