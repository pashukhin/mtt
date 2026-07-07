# NEXT_SESSION — primer

A living handoff doc. Update it at the end of each session (what's done / what's next).

## Where we are

- **Phase 0 (scaffold) + sessions 001–006 + 006.5 + 006.7 + 007 + 008 + 008.5 are DONE** (version `0.8.5-dev`, `make check` green).
  **Session 008.5 (dogfood enablers, chore)** shipped `mtt rm <id>` — hard-delete via a new base-port method
  **`TaskStore.Delete`** (the *D* in CRUD; YAML `os.Remove`) + the pure `core.Remover` usecase (reject-if-referenced —
  children via `Index`, dependents via `DepGraph`, deduped — with `--force` to override, leaving dangling refs the
  system already tolerates); `mtt rm` takes an **explicit id** (no current resolution — destructive) and clears a
  stale `current` pointer. **`--depends-on` on `mtt add`** sets `depends_on` at creation, validated + deduped in
  `core.Adder` (no cycle possible — unminted id). **Packaging:** `make install`/`build` stamp the version via
  conditional `-ldflags` (`make build VERSION=…`), a new `make smoke` installs into a throwaway `GOBIN` and runs
  `version`/`--help`, `version.go` now prints to **stdout**, default bumped `0.8.0-dev` → `0.8.5-dev`. **Not-found
  → exit `4` made UNIFORM:** every single-task-by-id path (`rm`/`show`/`edit`/`tree`/`use`/`status`/`dep`) wraps
  `mtt.ErrNotFound` via the `taskNotFound` helper (cli) / `%w` (core), so `exitCode` maps them all to 4. Next:
  **s008.6 priorities + roadmap** (spec already written + subagent-reviewed).
  **Session 008 (rollback/compensation)** shipped an additive per-command `pkg/mtt.Command.Rollback *Command`
  (a **leaf** compensator; `Valid()` rejects a nested rollback), recursive `ymlCommand.rollback` (scalar|map) +
  `toDomain` deep-copy, **eager** rollback expansion (`expandOne`/`expandTemplate` — a bad rollback template is
  exit 1 before any side effect), the `core.Runner` port method **`Compensate`** (best-effort, labeled
  `↩ compensating` phase in the exec adapter), and `core.Transitioner` compensation on a block — the
  succeeded-prefix rollbacks run **in reverse** (single `failIdx` source), the outcome stays `ErrBlocked`
  (exit 3), the task is **unchanged**, **no history**, and the block error carries a `compensated N …` summary;
  `mtt types` shows `↩ <rollback>`. Next: **s008.5 dogfood enablers** (`mtt rm`, `--depends-on`, `make install`).
  **Session 007 (structured commands)** shipped the `pkg/mtt.Command` value object (`{Run, Timeout}`;
  `Transition.Commands` is now `[]Command`), **placeholder expansion** on `run` (`.ID`/`.Type`/`.From`/`.To`
  via `text/template` in `core.Transitioner` — a self-enforcing shape-safe whitelist; `pkg/mtt` stays
  template-agnostic), a **per-command timeout** overriding the global `command_timeout` (resolved in the exec
  `Runner`, global as fallback), back-compat via `ymlCommand.UnmarshalYAML` (bare scalar **or** `{run, timeout}`
  map), `Runner.Run([]mtt.Command)` (Run expanded at the boundary; `Check.Cmd` records the expanded command),
  and `mtt types` showing per-command timeouts. Next: **s008 rollback/compensation**.
  **Session 006.7 (current task / working context)** shipped `mtt use [<id>] [--clear]` (a personal
  git-`HEAD`-for-tasks pointer in `config.local.yaml`), the additive `pkg/mtt.Transition.Current` (`set|clear`)
  rule + the `CurrentStore` capability port (`yaml.NewCurrent` writes `config.local` via a comment-preserving
  `yaml.Node`; the CLI applies set/clear after a move — `core.Transitioner` untouched), the pure
  `Type.FindTransition` primitive, and **omitted-id resolution** to the current task for
  `status`/`mtt <status>`/`show`/`edit` only (never list/tree/dep/ready).
  **Session 006.5 (attribution + verb sugar)** shipped `--why` (durable reason; `HistoryEntry.Why` + DTO +
  `show`), `--who` (mutually-exclusive alias of `--by`), the `mtt <status> <id>` **verb sugar** (fallback-routing
  in `root.RunE` — reuses `core.Transitioner`; unknown arg0 → exit 1; real command wins a clash), and
  **required-attribution** (`require:{who,why}` committed adapter `Settings.Require`, `config.local` tighten-only;
  validated in `core.Transitioner` before the gate, aggregated `ErrMissingAttribution` → **exit 2**; `--no-run`
  no-bypass). `-v`/`--log-file` moved to root-persistent. **Session 006
  (flow gate — the killer feature)** shipped `mtt status <id> <new>` — a **single** gated transition: the
  `core.Runner` port (`Run(commands)`, no `dir`) + `internal/adapter/exec` (per-command timeout, cwd=root,
  cross-platform shell seam) + a fake in tests; `core.Transitioner` (single-edge lookup, gate → `ErrBlocked`,
  append `history`, `Update` — no new port); config-driven `command_timeout` (adapter `Settings`, 5m default);
  `--role`/`--by` (+ env) recorded into history; **exit codes 3 (blocked) / 6 (invalid)** via `Execute() int`;
  and a `history:` section in `mtt show`. **Session 005 (dependencies)** shipped `mtt dep add/rm/list <id>`
  (`--tree`/`--cycles`), `mtt ready`, and `list --ready` over `core.DependencyEditor` (no new port — the edge
  rides `Task.DependsOn` + `TaskStore.Update`), a conservative `core.Ready`, and a derived `core.DepGraph`.
  Phase 2 (e3) is complete and Phase 3 (e4) is underway; **next is session 006.7 — current task (working
  context)** (`current` in `config.local`, set/cleared via a transition property; omitted id → current for
  single-task verbs; `mtt use <id>`), then s007 structured commands. **`advance`/`start`/`done` + roles
  are PARKED** (on-demand — single-edge `status` is the norm). Session 001
  shipped `mtt init [--template default|coding] [--force] [--name]` and `mtt types [<type>]`. Session 002
  shipped `mtt add [title] [--type] [--no-parent] [--description]` and `mtt show <id>`. Session 003 shipped
  `mtt list` (`--status`/`--type`/`--sort created|updated`/`--json`), `mtt edit <id> [--title]
  [--description]`, and the global flags `--dir`/`MTT_DIR`, `--version`, `--json`. **Session 004 (hierarchy)**
  shipped `mtt add --parent <id>` (validated placement, mutually exclusive with `--no-parent`), `mtt tree
  [<id>] [--status/--kind/--depth/--json]` (ASCII, keep-ancestors, nested JSON), `mtt list --parent/--kind`,
  and the `mtt show` lineage breadcrumb — over a derived `core.Index` (computed children/ancestors, cycle-safe)
  and a shared `core.Match` predicate. CI runs on Node 24 action majors.
- **In place now:** the pure `pkg/mtt` contract (`Config/Type/Flow/Status/Transition`, the `StatusKind`
  value object, structural `Config.Validate()`, `DefaultType`/`ChildrenIn`), the YAML adapter's **config
  layer** (`FindRoot`, `HasProject`, embedded `default`/`coding` templates, atomic `Init`, `Load` +
  `config.local.yaml` overlay, DTO↔domain mapping, prefix/one-default checks), the `Task` model + full
  `TaskStore` port (`Create`/`Get`/`List`/`Update`, `pkg/mtt`), `internal/core`'s `Adder`/`Select`/`Editor`
  (add is a mutation usecase; list-select is a pure filter/order function; edit is a mutation usecase, all
  clocked via an injected `now`), the YAML adapter's **flat per-prefix ID minting** (`O_EXCL`) + deterministic
  serialization, and the CLI's `projectRoot`/`baseDir` (DRY root resolution) + `taskJSON` (shared `--json`
  view for `show`/`list`/`edit`). **Not yet:** hierarchy (`--parent`/`tree`), dependencies, flow enforcement,
  comments; also deferred out of 003 — a durable edit-audit trail and the subject-identity (`By`) source
  (see "Open design slice" below). Hierarchy landed in 004; dependencies/`ready`/cycle-detection landed in
  005; **flow enforcement (transition validation + executable `commands` gates via the `Runner` port)** is next.
- Work is organized in **compact sessions** (see [sessions/README.md](sessions/README.md)); next up is
  **session 006** (flow gate: `mtt status <id> <new>` runs & gates the transition's `commands`, writes
  `history`; the `Runner` port + `internal/adapter/exec` + a fake; `--role`/`MTT_ROLE` recorded).
- Repo: <https://github.com/pashukhin/mtt> (public). Branch per session → PR → CI green → merge into `main`.
- Stack: Go 1.23, cobra, `gopkg.in/yaml.v3`, `go-internal/testscript` (e2e); storage — YAML file-per-task.

## The session starts with planning (mandatory)

Before any code — a planning phase (use the superpowers skills: brainstorming/planning). The plan must
account for the key invariants from DESIGN.md:

- **Types and hierarchy are domain (from config); ID/slug is the adapter's job.** The code has NO literals
  for type names or ID structure. Hierarchy comes from a type's `parents` field, stored (not encoded in the
  ID). The ID is minted by `TaskStore` and is **flat, per-prefix** (YAML: `<prefix><N>`, e.g. `e1`/`t17`/`s3`;
  `prefix` is a YAML-adapter field) — decoupled from position, so re-parenting never re-mints it.
- **Invariants (validated on config load, structural & name-agnostic):** `kind` (initial/active/terminal) is
  defined by flow **topology** and validated; **≥1 of each kind per flow** (minimal `initial → active →
  terminal`; a 2-status flow is invalid); **multiple initials allowed**; status identity is per-flow
  `(type, name)` with **no cross-flow transitions**; the default type is marked `default: true` (**no literal
  `task`**); ready/list work by category, never by a literal name.
- **Domain is pure & provider-agnostic (DDD):** `pkg/mtt` carries no serialization tags and no `prefix`
  (adapters map via DTOs); `kind` is a value object (`StatusKind`); references are by identity, back-refs are
  computed; the domain needs only a **mandatory minimum**, so types/flow can later come from an external
  provider (`mtt connect`). A task's type is **immutable** (recategorize = close + create + link via `refs`).
- **Capabilities:** features (history, dependencies, comment tree, search, **KB**) are optional per adapter
  (`Capabilities()` / `ErrUnsupported`); YAML is the reference (does everything), `core` writes to the
  minimum and "lights up" what's available. A task carries append-only `history` (always, in YAML).
- **References:** tasks/comments carry structured verifiable `refs` (`note`/`task`/`comment`/`url`) —
  informational, **≠ `depends_on`**. Verification is capability-aware (note only with a KB). Without a KB,
  knowledge lives in tasks/comments and the links between them.
- **Roles — a seam (not built now):** the semantics of `start`/`done` depend on the role (reviewer vs
  implementer). For now we only reserve `role` in `history` (non-deferrable) and `--role`/`MTT_ROLE`;
  routing and a config `roles` section come later. Roles are semantic routing, not RBAC; role names come from config.
- **Killer feature — executable transitions:** a transition carries `description` + `commands` (all → 0,
  else the transition is blocked). Execution is behind the `Runner` port (`core` defines it,
  `internal/adapter/exec` implements it, tests use a fake). `start`/`done` are the meta-command
  `advance --to` (walk to a target; modes `--stop`(default)/`--atomic`/`--force`; no config DSL). Phase 3.
- **`mtt init`** writes the default `.mtt/config.yaml` (types + flow, no commands). Defaults live in the
  init template, not in logic.
- Consequence for ordering: contract + types + adapter (with ID minting) — phase 1; flow enforcement with
  command execution — phase 3.
- **Positioning (see DESIGN.md → "Positioning vs beads"):** our wedge is per-type flow + zero-footprint +
  adaptivity (a thin agent layer over the existing backend). Dependencies stay simple, the KB is low
  priority; `mtt-ui` is a nice optional default, not the main argument. ID collisions are accepted
  consciously (don't complicate until real parallelism appears).

## What to read first (in order)

1. [CLAUDE.md](CLAUDE.md) — the entry point
2. [AGENTS.md](AGENTS.md) — rules, gate, principles, DoD
3. [DESIGN.md](DESIGN.md) — architecture and decisions
4. [TASKS.md](TASKS.md) — the plan; next up — section **e2 (Phase 1)**

## Activating guards (superpowers)

The plugin is declared in the personal `.claude/settings.local.json` (per-user, gitignored)
(`superpowers@superpowers-marketplace`). Plugins load **at session start**:

1. On opening the project, Claude Code may show a trust prompt for the marketplace
   `obra/superpowers-marketplace` — confirm it (once).
2. If the skills don't appear automatically, run once:
   ```
   /plugin marketplace add obra/superpowers-marketplace
   /plugin install superpowers@superpowers-marketplace
   ```
   (alternative — the official marketplace: `/plugin install superpowers@claude-plugins-official`)
3. Verify the TDD/brainstorming/debugging skills are available, and **use them**.

## Domain-model snapshot (read before s007)

[docs/architecture/model.go](docs/architecture/model.go) — a code-form, tiered (T1/T2/T3) index of the whole
intended contract: domain types + ports + optional capabilities, core usecases with dependencies, the derived
resolved graph, and open gaps. Two decisions locked there that shape s005:

- **s005 adds no new port.** `depends_on` is a `Task` field round-tripped via `TaskStore.Update` (as `parent`
  was in s004); `DependencyStore` is only for external adapters that cannot embed. s005 = core
  `DependencyEditor` + `Ready` + cycle-check, no `pkg/mtt` port method.
- **Chore 004.5: typed-identity retrofit — DONE.** `TaskID`/`TypeName`/`StatusName` are now used across the
  shipped `pkg/mtt`/`core`/`adapter`/`cli`; the YAML DTO keeps plain strings on disk and maps `string↔typed`
  at its boundary (`toDomain` fails fast on a corrupt empty `id`/`type`/`status`). s005 is written against the
  typed contract. Constructors reject empty, no transform; `Ref.ID` stays `string`; `NoteSlug` deferred (KB).

## Next task — session 008.6 (priorities + roadmap)

> **s008.5 (dogfood enablers) is SHIPPED** — see "Where we are" and "Carry-over lessons (008.5)" below; the
> spec is `docs/superpowers/specs/2026-07-07-session-008.5-dogfood-enablers-design.md` and the plan is
> `docs/superpowers/plans/2026-07-07-session-008.5-dogfood-enablers.md`. **Next up is s008.6 priorities +
> roadmap** (e5_t1a): 📋 already **spec'd + subagent-reviewed** — implement from
> `docs/superpowers/specs/2026-07-07-session-008.6-priorities-roadmap-design.md` (write the plan at that
> session's start, in fresh context); a closed `Priority` VO (`--priority` on `add`/`edit`/`list`,
> `--sort priority`, in `show`/`taskJSON`) + **`mtt roadmap [--json]`** — a pure-core priority-guided Kahn
> (dependency hard, priority soft; cycle-safe; `ready`/`blocked_by` annotations), the agent-queryable execution
> order that motivates dogfood. Then **s008.7 tags**, **s008.9 batch & pipeline**, **s009 dogfood**.
> `advance`/`start`/`done` + modes + roles-on-edges stay **PARKED** (single-edge `status` is the norm).

### Carry-over lessons (008.5 — dogfood enablers)
- **A store operation earns a base-port method (the D in CRUD), unlike an embedded field.** `depends_on`/
  `history`/`current` ride `Update` (or a capability port); *delete* cannot be embedded in the aggregate it
  removes, so `TaskStore.Delete` goes on the mandatory-minimum port (YAML `os.Remove`). Read GAP #1 the other
  way: "can the reference adapter embed this?" — no → base-port method.
- **A new interface method's blast radius is every implicit implementer — fold the fakes into the same
  commit.** Adding `Delete` to `TaskStore` broke three `internal/core` test fakes (`fakeStore`/`memStore`/
  `editStore`, and `memStore` is reused by `transition_test.go`); stubbing all three is part of commit #1 or
  the package won't compile. Grep for fakes before an interface change.
- **A uniform exit-code taxonomy is a wrap-with-`%w` pass, and it changes error *text*.** Making not-found →
  exit 4 uniform meant wrapping `mtt.ErrNotFound` in every single-task-by-id path (`taskNotFound` helper in
  cli; `fmt.Errorf("…: %w", …, mtt.ErrNotFound)` in core). The wrapped message (`… : mtt: task not found`)
  keeps the `not found` substring (so substring e2e survive) but breaks exact-wording asserts — two testdata
  scripts (`add_show.txt`, `tree.txt`) needed the new wording. Grep `not found` across tests/testdata first.
- **`rm` is agent-facing: no interactive confirm, no attribution.** A `y/N` prompt hangs an agent (no stdin);
  a deleted task has no `history` to sign, so `--who`/`--why` don't apply — the git commit is the audit.
  Safety comes from reject-if-referenced + `--force`, and an **explicit id** (no current resolution on a
  destructive verb). Reused `Index.Children` + `DepGraph.Dependents` for the referenced-check — no new graph.
- **Conditional-LDFLAGS keeps `make build` predictable.** `VERSION ?=` empty → the code default
  (`internal/cli.version`) ships; `make build VERSION=v0.8.5` stamps via `-X`. A `make smoke` (throwaway
  `GOBIN`, `trap` cleanup, one backslash-joined recipe) install-tests the binary; kept out of `check` (real
  `go install` is non-hermetic). Also fixed `version.go` to print to **stdout** (`cmd.Println` → `OutOrStderr`
  meant `$(mtt version)` was empty).

### Carry-over lessons (008 — rollback / compensation)
- **Additive VO field with a self-referential pointer + a leaf invariant.** A per-command compensator that is
  itself a structured command is `Rollback *Command` (a pointer breaks the infinite-size struct). Keep it a
  **leaf** (`rollback.Rollback == nil`) and enforce that in `Valid()` — the recursion (`expandOne`,
  `ymlCommand.UnmarshalYAML`, `toDomain`) then terminates at one level, and the config-time check catches
  nonsense. Struct comparability survives (pointers are comparable), so existing `cmd == mtt.Command{…}` tests
  keep compiling.
- **Recursive DTO for free via `*ymlCommand`.** Making the `rollback` YAML field a `*ymlCommand` reuses the
  s007 scalar-or-`{run,timeout}` `UnmarshalYAML` for the compensator automatically; a recursive
  `ymlCommand.toDomain()` **deep-copies** it (a fresh `*mtt.Command`, never aliasing the DTO pointer — assert
  this with a mutate-the-DTO test).
- **Compensation is core policy; execution + the labeled phase are the adapter's.** `core.Transitioner` decides
  *what* to compensate (which succeeded, reversed); the exec `Runner.Compensate` runs it **best-effort** and
  owns the `↩ compensating` progress header (core stays free of I/O). One new port method beat a core-side
  `Run`-per-command loop precisely because the header needs the progress writer. Best-effort never changes the
  outcome — still `ErrBlocked` (exit 3), no new code.
- **Derive "which command failed" from a single source.** Compute an explicit `failIdx` (`firstFailure` index
  for a non-zero check; `len(checks)-1` for an operational error) and compensate `expanded[:failIdx]` in
  reverse — do **not** re-infer it as "the last check" at one site and "the first non-zero" at another (they
  coincide only because the exec runner stops at the first non-zero; a divergent Runner would misclassify). The
  failed command's own rollback is then structurally never run.
- **`blocked → no history` is the reconciliation for compensation audit.** A blocked transition did not happen
  (no `from→to`), so `Task.History` (a *transition* journal) gets nothing and the task file is untouched;
  compensation is a side-effect event surfaced only live (progress) + in the block summary. A durable
  side-effect audit stays the parked edit-audit slice.
- **Document the `Runner.Run` operational-failure contract** (the failing command's `Check` is the last
  element, `Exit -1`) since compensation math depends on it; make the test fake replicate it.
- **e2e proves the mechanism, not git.** Rollback is for **arbitrary** commands; the acceptance e2e uses generic
  POSIX `touch`/`rm`/`false` (no `[exec:git]` guard) and a multi-command reverse compensation — precise
  reverse-order / best-effort / succeeded-only / no-history are **unit** tests (the s006/s007 split). The full
  swapped config must be a valid flow (`mtt add` runs `Config.Validate`).
- **Validation runs on `add`/`types`, not `Load`/the gate path** — the pre-existing s006/s007 status quo. Don't
  claim a domain invariant is "enforced at Load"; a config-time invariant only bites where `Config.Validate` is
  actually called.

### Carry-over lessons (007 — structured commands)
- **Domain-vs-policy split for a per-edge property.** The s006 rule "execution policy (`command_timeout`) rides
  the adapter `Settings`, not `pkg/mtt`" does **not** bar a *per-command* timeout from the domain: the global is
  one runner knob (applies to every command, external trackers run none), but a per-command timeout is an
  **authored property of a flow edge**, inseparable from its `run` (which already lives in `Transition.Commands`).
  Test: "is this a runner default, or authored on this specific edge?" Default → adapter `Settings`; authored →
  the domain VO. The runner resolves per-command-else-global, keeping the global out of `core`.
- **Reuse the DTO's mandatory custom `UnmarshalYAML` to parse and keep `toDomain` error-free.** `ymlCommand`
  needed a custom unmarshal anyway (scalar OR map back-compat); folding `time.ParseDuration` into it means a bad
  duration surfaces at `Load` (like `parseCommandTimeout`) and `toDomain` stays a pure, error-free copy — no
  smart constructor, no `toDomain` signature churn (the s006.7 lesson holds). Decode the map branch into a
  string-`Timeout` alias, never back into `ymlCommand` (infinite recursion; yaml.v3 can't decode `30s` into
  `time.Duration`).
- **A `text/template` struct context is a self-enforcing whitelist.** Exposing only `cmdContext{ID,Type,From,To}`
  means `{{.Title}}` (or any free-text/typo) is a template error at `Execute` — the struct's shape *is* the
  injection policy; no shell-quoting, no field allow-list check. Keep templating in `core` (where the policy
  lives), not `pkg/mtt` (which stays template-agnostic, storing the raw template). An expansion error is a plain
  error (exit 1), distinct from a gate block (`ErrBlocked`, exit 3) — a malformed command is a config fault, not
  a failed gate.
- **Expand with the pre-move status.** `.From` must be the status being *left*, so capture `from := t.Status`
  **before** `t.Status = to` and build the context from it; the same `from` feeds the history entry. `--no-run`
  skips expansion together with the gate (nothing to expand if nothing runs).
- **Type-migration task in a green-between-commits plan.** Flipping `Transition.Commands []string → []Command`
  breaks every consumer at once (unavoidable — the field type drives them). Land it as one behavior-preserving
  refactor commit (DTO maps string→`{Run}`, runner/exec/core/`types`/tests updated), *then* add the new YAML
  form + expansion + timeout on top. Each commit stays `make check`-green.
- **`Check.Cmd` records the expanded command** (`git checkout -b task/t1`, not the template) — a truthful audit;
  it falls out for free because `core` passes expanded `Command`s to the runner.
- **e2e: assert an unborn branch via `git symbolic-ref --short HEAD`, not `git branch --list`.** On a fresh
  `git init` with no commits, a newly-created branch is unborn and invisible to `git branch --list` (empty
  stdout); `symbolic-ref` reports it and needs no user config. Guard the one git-shelling script with
  `[!exec:git] skip`. Keep the slow-gate sleep short (~1s): `exec.CommandContext` kills at the timeout but
  `Run()` blocks until the orphaned child closes the inherited output pipe (same as `TestRunTimeout`), so a long
  sleep only slows the test — the block still fires.

### (Completed) session 006.7 brief — current task / working context

- **Create `sessions/006.7_current_task.md` from `sessions/000_template.md`** (design-spec + plan before
  code). Branch `feat/s006.7-current-task`. Refine the plan (superpowers brainstorming/planning), work
  **test-first**; acceptance e2e + `make check` green before the PR. See TASKS.md → **e4_t8a** and DESIGN.md →
  "Working context: the current task".
- Scope (kills id-repetition — git-`HEAD`-for-tasks):
  1. A **`current`** record in `config.local.yaml` (personal, gitignored — the **value**); companion
     `mtt use <id>` (git-checkout-like set) + a way to show the current task.
  2. Set/clear driven by a **transition property** in the **committed** flow (the **rule**) — a new additive
     `pkg/mtt.Transition` field (e.g. `current: set|clear`; name-agnostic; a topology default
     set-on-→active / clear-on-→terminal is a brainstorm option). This is the s006.7 `pkg/mtt` change.
  3. An **omitted id** resolves to `current` **only for single-task direct verbs** (status / `mtt <status>` /
     show / edit / tag) — never for filter/list/stdin/bulk (resolution order: explicit id > filter/stdin >
     current). Composes with the s008.9 selector (its "no source" single-verb case = current).
- **Caveat:** a shared checkout with multiple agents = one `config.local` = one `current` → collision;
  per-agent current ties to the parked subagent-identity think-item (fine for solo / one-agent-per-checkout).
- Reuse the s006.5 shape: the sugar and `mtt status` both funnel through `runTransition`; an omitted id
  resolves before that. Everything typed; string conversion only at the cli/adapter boundary.
- **PARKED (do not build):** `advance`/`start`/`done`/`cancel`, modes, roles-on-edges, config verb→status.
- **After 006.7 → s007 structured commands** (placeholders + per-command timeout — the "work in task terms"
  enabler; `Transition.Commands []string` → a `Command` value object, a domain-shape change), s008 rollback,
  s008.5 dogfood-enablers chore, s008.7 tags (+`#hashtags`), s008.9 batch & pipeline, then dogfood (s009).

### Carry-over lessons (006.7 — current task / working context)
- **A capability port is justified when the data is non-embeddable — that's the GAP #1 test, read the other
  way.** `depends_on`/`history` embed in `Task` and ride `Update`, so YAGNI the port (s005). `current` is a
  personal, gitignored, single-value pointer — *not* task state — so even YAML cannot embed it; it needs a
  separate store. That non-embeddability is exactly what earns `CurrentStore` a port **now** (unlike the parked
  `DependencyStore`). Use "can the reference adapter embed this in the aggregate?" as the port-vs-field test.
- **Interpreting a declared flow effect is dispatch, not policy — apply it at the composition root.** The
  set/clear RULE is domain data (`Transition.Current`); *applying* it (`set`→set, `clear`→clear) makes no
  decision, so the CLI reads the edge (via `Type.FindTransition`) and calls the port after a successful move —
  `core.Transitioner` stays a pure gate. Contrast the gate itself (non-zero → blocked, no history), which *is*
  policy and rightly lives in core. Accepted seam: when the parked `advance` unparks and needs the same, extract
  a shared core apply-edge-effects step (revisit-at-the-second-caller, like DepGraph/Index).
- **Round-trip a human-edited config file with `yaml.Node`, not a struct decode.** `config.local.yaml` carries
  `author` + comments a user wrote. `NewCurrent` reads/mutates/writes only the top-level `current:` key through
  a `yaml.Node` (upsert/delete on the root mapping's `Content`), so comments and other keys survive. A struct
  decode+re-encode would silently drop them. Keep it independent of `Load` (which ignores the unknown key
  non-strictly) — one object owns the pointer (SRP).
- **New value object → mirror `StatusKind`, not a smart constructor.** `CurrentAction` ships as
  `type + consts + Valid()`, validated in `Config.Validate`/`validateFlow`, and cast in `toDomain` — no
  `NewCurrentAction`, no `toDomain` signature churn. The spec suggested a constructor; the codebase idiom
  (`StatusKind`) is cast-then-Validate. Follow the idiom; note the deviation.
- **Defer discovery scaffolding until a consumer exists.** `CapCurrent`/`Capabilities()` were dropped from scope:
  no `Capability` vocabulary exists in code yet, `mtt caps` (e4_t6) is unstarted, and CLI-applies (option ii)
  never type-asserts for the capability — so the const would be dead surface. Ship the port; fold `CapCurrent`
  into `mtt caps`. Guard the port with `var _ mtt.CurrentStore = (*Current)(nil)`.
- **Default type needs `--no-parent` in tests.** The `default` template's default type (`task`) has
  `parents: [epic]`, so `mtt add A` in a fresh default project fails placement — unit/e2e that just want a task
  pass `add A --no-parent` (or ship a single-root-type `-- local.yaml --` like the e2e does).
- **Omitted-id resolution is a shared pre-step, kept off set/bulk verbs.** `resolveTaskID(root, explicit)`
  (explicit id > current; stale/absent → actionable error) is called by `status`/sugar/`show`/`edit` **before**
  the usecase; `list`/`tree`/`dep`/`ready` never call it. The 1-arg sugar (`mtt done`) resolves current first,
  then classifies arg0 against *that* task's flow — mirroring the s006.5 2-arg sugar shape.

### Carry-over lessons (006.5 — attribution + verb sugar)
- **Verb sugar via `root.RunE` fallback, NOT command registration.** Setting `root.Args = cobra.ArbitraryArgs`
  + `root.RunE = runSugar` makes cobra call the root for any unknown first arg; real subcommands still dispatch
  first (so a real command always wins a name clash, for free). `runSugar` classifies via `trySugar` (load
  project, `Get(arg1)`, `Type.StatusKind(arg0)`); a miss returns `unknown command` (exit 1) — do **not**
  swallow it into a confusing transition error. `mtt` no-args → `cmd.Help()`. Reuse this shape for s006.7's
  omitted-id resolution (resolve the id **before** `runTransition`, keep the single shared path).
- **Attribution/policy enforcement lives in `core.Transitioner`, checked before the gate.** `require:{who,why}`
  is adapter `Settings` (like `command_timeout`), passed into `TransitionOptions` as bools; the check
  aggregates missing fields into one `ErrMissingAttribution` (exit 2) **before** `runner.Run`, so `--no-run`
  can't bypass it and both `status` + sugar get it (they share `runTransition` → `Transitioner`). Put the new
  exit code in `exitCode(err)` (mirrors 3/6).
- **`config.local` "tighten-only" = OR with the pre-overlay committed value.** yaml.v3 overlay would let a
  local `who:false` relax a committed `who:true`; capturing committed `require` **before** `decodeInto(local)`
  and OR-combining prevents relaxation while still allowing local to add. Reuse for any future
  committed-vs-local policy where local must not weaken the project rule.
- **Mutually-exclusive aliases: manual check beat `MarkFlagsMutuallyExclusive`.** With `--who`/`--by`
  root-persistent and used by two commands (`status` + sugar), a manual `Changed("who") && Changed("by")` →
  error in the shared `resolveAttribution` is path-uniform and dodges cobra's persistent-flag flag-group
  cross-command subtleties.
- **`formatTask` lives in `show.go`, not `format.go`** (`format.go` is only `taskLine`) — grep before editing.
- **Root-persistent flags only some commands honor is an accepted pattern** (`--json` since s003; `-v`/
  `--log-file` since s006.5) — moving a gate-output flag up to root is fine; remove the now-duplicate local
  flag or cobra panics on the redefinition.

### Open design slice to schedule (not session 006's scope, but don't lose it)
- **Durable, git-independent audit of edits** + **the subject-identity (`By`) source.** `edit` today only
  bumps `updated`; git is the de facto audit trail. A change-log or field versioning (additive) would make
  edit history queryable without git, and needs an identity source for "who" (likely
  `.mtt/config.local.yaml`, distinct from `--role`, which is "what hat"). See DESIGN.md → "Listing and
  editing" / TASKS.md → "Later (coarse)". Schedule a design pass before it's needed for real auditing.

### Carry-over lessons (006 — flow gate)
- **First driven port beyond storage = a fake in tests, a real adapter in `internal/adapter/*`.** `core.Runner`
  is defined in `core` (only it needs it), implemented by `internal/adapter/exec`, and faked in `core` tests —
  no process spawned in unit tests; the exec adapter is unit-tested directly (`true`/`false`/timeout). Reuse
  this shape for any future driven port.
- **Deviate from model.go when the layer is cleaner for it — and update model.go.** `Runner.Run(commands)`
  dropped the `dir` param (the exec adapter holds `cwd=root`), keeping `core` free of filesystem paths. The
  snapshot is a guide, not a contract; sync it when you deviate.
- **A non-zero exit is DATA, not a Go error.** The `Runner` returns `[]Check` (cmd+exit) and reserves the
  Go `error` for operational failures (launch/timeout). `core.Transitioner` decides blocked-vs-applied from the
  checks. Don't conflate "the gate said no" with "the runner broke".
- **Adapter-level settings ride the config layer, not `pkg/mtt`.** `command_timeout` is execution policy (an
  external tracker runs no commands), so it went into the YAML adapter's `Settings{Prefixes, CommandTimeout}`
  from `Load` (like `prefix`), never into the pure domain. Default in code (5m), overridable via config +
  `config.local`. Widening `Load`'s adapter-return is cheap: `_`-discard callers are untouched.
- **Exit-code taxonomy lives in `Execute() int`.** `Execute` maps core sentinels (`ErrBlocked`→3,
  `ErrInvalidTransition`→6, else 1); `main` and the **testscript harness** both do `os.Exit(Execute())` —
  changing that signature silently breaks the e2e harness if `TestMain` isn't updated in the same task (it was).
  testscript `!` asserts non-zero, not a specific code — unit-test the numbers (`exitCode`) separately.
- **e2e configs go in a txtar `-- gated.yaml --` file, `cp`'d over `.mtt/config.yaml` after `init`.** The
  default flow has no `commands`; to exercise a gate, ship a single-type root config with `commands: ["true"]`
  / `["false"]` on edges. `mtt add`'s title is **positional** (`mtt add 'A'`), not a `--title` flag.
- **Single-edge lookup beat `ResolvedFlow` for `status`** — a linear scan of `Type.Transitions` (YAGNI, the
  s005 "don't force the abstraction" lesson again). `ResolvedFlow` earns its keep in s007's multi-edge walk.
- **Sentinels for a new outcome live where the policy is.** `ErrBlocked`/`ErrInvalidTransition` are in `core`
  (flow is core policy), matched via `errors.Is` — mirror of how `ErrNotFound` sits in `pkg/mtt` (port contract).

### Carry-over lessons (005 — dependencies)
- **No new port for an embedded edge** (GAP #1 confirmed): `depends_on` rides `Task.DependsOn` + `TaskStore.Update`
  (like `parent` in 004); `core.DependencyEditor` owns the cycle-check. The `DependencyStore` capability stays
  unimplemented until an external adapter needs it — **YAGNI** the signature too (`NewDependencyEditor(store, now)`,
  no nil capability param). Apply the same to `history`/`comments` in later sessions.
- **Conservative derived-read semantics**: `core.Ready` requires *positive confirmation* — an unresolvable status
  (config drift, `kindOf` false) or a dangling blocker leaves a task **not** ready, mirroring how `Match` fails an
  unresolvable `--kind`. Prefer honest-not-ready over optimistic-ready for anything the data can't confirm.
- **Second derived graph kept separate** (GAP #6 not extracted): `DepGraph` (multi-edge DAG over `depends_on`) and
  `Index` (single-parent tree over `parent`) don't share a traversal primitive — a shared one would be forced.
  Revisit only if a third graph (`ResolvedFlow`, s006) naturally shares it. Both still reuse `lessByRecency` for order.
- **One primitive, two commands**: `mtt ready` and `list --ready` are both `Select(Ready(tasks, cfg), filter, cfg)` —
  readiness AND the list filters compose (both ANDs, order-independent). Extract the subset builder, don't duplicate.
- **e2e can't reach states a later session unlocks**: no status transition exists until s006, so `dep.txt`/`ready.txt`
  prove blocking + cycle rejection, while unblock-on-terminal and `--cycles`-on-a-real-cycle are **unit** fixtures
  (a `done`-blocker task / a hand-built cycle — the CLI's `add` rejects cycles, so it can't build one). Note the gap
  explicitly rather than faking the state.

### Carry-over lessons (001, 002 & 003 — save review loops)
- **CLI output → stdout**: use `fmt.Fprint(cmd.OutOrStdout(), …)`, NOT `cmd.Print/Printf` (those route to
  stderr when no writer is set — breaks pipes and e2e `stdout` asserts). Errors surface via `Execute` (stderr).
- **golangci-lint `unused`** (standard set) fails on unused unexported package-level consts/funcs — declare
  a symbol in the task that first *uses* it, not ahead of time.
- **testscript e2e**: pin `go-internal@v1.14.1` (latest needs Go ≥1.25); use `testscript.Main` (not the
  deprecated `RunMain`); make sure an "uninitialized dir" case is a genuine sibling of the init'd dir (no
  `.mtt` ancestor) or the assertion is vacuous. Drive the real binary; assert stdout vs stderr correctly.
- **testscript assertions must anchor, not substring-match** (a 002 review catch): assert e.g.
  `'t1  task  \[tbd\]'`, not a bare `'task'` — a loose substring can match vacuously against unrelated output.
- **A pure read needs no `core` usecase** (a 002 correction): `show` reads a task directly through the
  `TaskStore` port; only mutations (`add`) go through `core`. 003 applied the same split to `list`:
  `core.Select` is a pure function (no store injected, no usecase struct) the CLI composes with
  `TaskStore.List`, while `edit` (a mutation) got a real usecase (`core.Editor`).
- **New package → its own thin `CLAUDE.md`** (per AGENTS): `internal/core` got one in 002, kept current in
  003 (`Select`/`Editor`); keep doing this for hierarchy in 004.
- **List order is provider-agnostic** (003): the default order is a **domain timestamp** (`Created` desc, or
  `Updated` desc with `--sort updated`), never ID structure — an external adapter's IDs may not sort
  meaningfully. Ties break on the ID as an **opaque string** compare. Order determinism is **unit-tested**
  (`core.Select`); the e2e (`list_edit.txt`) deliberately asserts row **presence**, not sequence, since it
  drives the real binary against wall-clock timestamps that can tie at second resolution.
- **Zero-match `--json` must be `[]`, not `null`**: build the slice with `make([]T, 0, …)` before appending,
  so `encoding/json` marshals an empty result set as `[]` — a `nil` slice marshals to `null`, which most
  JSON consumers don't treat the same as an empty array. (004: nested `tree --json` — top-level slice is
  `make([]T,0,…)`; leaf `children` use `omitempty`.)
- **Derived graphs live in `core`, not the contract** (004): `core.Index` (parent→children/ancestors) is a
  pure value built from `TaskStore.List` — no store, no clock, **not** in `pkg/mtt`. Back-refs are computed,
  cycle-safe (visited-set), orphans surface as roots. Keep the sibling comparator shared with `Select`
  (`lessByRecency`) so tree and list order identically (DRY). Reuse this shape for `depends_on` in 005.
- **One predicate, many consumers** (004): `core.Match` (status/type/kind/parent) backs both `list` and the
  `tree` walk. When a filter needs config (e.g. `--kind` → `Type.StatusKind`), thread `cfg` into the pure
  function (`Select(tasks, ListFilter, cfg)`) — `cfg` is domain data, so the function stays pure.
- **Mutually-exclusive cobra flags**: `cmd.MarkFlagsMutuallyExclusive("parent","no-parent")`; the e2e error
  text to assert is `if any flags in the group`.
- **Tree filter semantics = keep-ancestors** (004 decision): a node shows if it matches or any descendant
  matches; non-matching ancestors stay as the path. Prove render/order by **unit test** (fixed clock); the
  e2e asserts presence.

## Ready-to-paste kickoff prompt (for a new session)

> Продолжаем mtt. Сессии 001–008 (+ 006.5/006.7/007) смёржены в `main`, версия `0.8.0-dev`,
> `make check` + CI зелёные. **s008.6 (priorities + roadmap) уже заспечено и субагент-отревьюено**
> (на `main`, статус 📋 planned) — делаем ПОСЛЕ этой сессии. Общайся по-русски.
>
> Прочитай сначала (в порядке): CLAUDE.md → AGENTS.md → DESIGN.md → NEXT_SESSION.md (секции
> «Where we are», «Next task — session 008.5», «Carry-over lessons (008)» и (007)/(006)/(005)) →
> sessions/README.md (роадмап: 008.5 ← next, затем 008.6 spec'd) → docs/architecture/model.go
> (порт `TaskStore` — Create/Get/List/Update, **без Delete**; `Adder`/`AddParams`, `DependencyEditor`
> — куда едут `rm` и `--depends-on`) → TASKS.md (e5_t1 + think-item «dangerous ops must mandate
> --who/--why» в Later) → sessions/008_rollback.md (свежий образец) → CLI_REFERENCE.md. Убедись,
> что superpowers-скиллы активны.
>
> Делаем **СЕССИЮ 008.5 (dogfood enablers, chore)** на ветке `feat/s008.5-dogfood-enablers` от
> свежего `main`. Порядок: brainstorming → writing-plans → строго test-first до зелёного
> acceptance-e2e + `make check`; ветка → PR → CI green → squash в `main`. Бампни `0.8.0-dev` →
> `0.8.5-dev`.
>
> Scope — ТРИ маленькие однозначные фичи в ОДНУ сессию (батч — предпочтение: не плодить сессии):
> **(1) `mtt rm <id>`** — hard-delete (в отличие от `cancel`). Открытые вопросы для брейнсторма (НЕ
> решай заранее): нужен ли новый метод порта `TaskStore.Delete` (delete — операция стора, НЕ
> embedded-поле → GAP #1 «поле на Update, без порта» его не покрывает → вероятно да, обоснуй); что с
> dangling-ссылками (кто `depends_on` удаляемую; дети с `parent` на неё — система уже терпит dangling:
> Ready консервативен, Index сирот поднимает в корни — но rm решает: отклонять-если-referenced /
> каскад / оставить); опасная операция → требовать ли `--who/--why` и/или `--force`/подтверждение
> (think-item в TASKS); exit-код «not found» (кандидат 4). **(2) `--depends-on` на `mtt add`** —
> задать `depends_on` при создании (валидировать существование целей; цикл на свежей задаче
> невозможен; переиспользуй примитивы `DependencyEditor`; типизировано, конверсия строк только на
> границе cli). **(3) Packaging** — `make install` → `go install ./cmd/mtt` + smoke-тест (бинарь
> ставится, `mtt version`/`--help` работают; версия через `-ldflags`, сейчас `var version` в root.go).
>
> Heed «Carry-over lessons» из NEXT_SESSION, особенно: новый метод порта оправдан, когда данные НЕ
> embeddable (delete — операция стора → новый метод порта, в отличие от depends_on/tags на Update);
> non-zero exit — ДАННЫЕ; CLI-вывод через `fmt.Fprint(cmd.OutOrStdout(), …)`; anchored testscript-
> ассерты; `golangci unused` (символ объявляй там, где ВПЕРВЫЕ используется); zero-match `--json` =
> `[]`; атомарная запись в yaml-адаптере (temp+rename) — удаление тоже атомарно; держи каждый
> `CLAUDE.md` актуальным; docs-sync (DESIGN.md/.ru, CLI_REFERENCE.md/.ru, CLAUDE ×N, model.go —
> `TaskStore.Delete`/`AddParams.DependsOn`, TASKS e5_t1 ✅, sessions/README 008.5 ✅, NEXT_SESSION,
> заполни sessions/008.5_*.md Done).
>
> После s008.5 → **s008.6 priorities + roadmap** по готовой спеке
> `docs/superpowers/specs/2026-07-07-session-008.6-priorities-roadmap-design.md` (план допиши в
> начале той сессии). **PARKED:** advance/start/done/cancel + режимы + роли-на-рёбрах; node-level
> status-actions; кросс-рёберная компенсация. Фанатично: SOLID/DRY/KISS/TDD/DDD/clean-arch +
> self-check из AGENTS.md.
