# t66 — Commands on task lifecycle events (spec)

Status: draft for adversarial spec review.
Decided in the 2026-07-24 brainstorm (4 scoping decisions with the user). This spec settles
**t66 + t26 + t24 together** (the queue mandate: one precedence model, decided once).

## Problem

Command pipelines exist only on flow **edges** (`commands:` gates + `post:` actions, t21). `add`,
`edit`, `tag`, `dep`, `priority`, `ref`, `rm` and every `note` mutation are not transitions, so
nothing can run on them — which is why non-flow mutations still need manual `git add .mtt && git
commit && git push` (the t26 pain; live incident c7: an unpushed add-commit on main diverged at
squash-merge time; recurring cost in every queue-grooming session since s009).

Meanwhile the per-edge `post:` auto-commit line is duplicated across ~30 edges of the two dogfood
types (the t24 pain), and the t62 flagship template is blocked on collapsing exactly that
duplication.

## Goals

1. **Lifecycle events** — config-authored, post-only command pipelines on create/update/delete of
   tasks **and** notes, firing per entity from the mutation usecases.
2. **Edge `post_defaults`** — a per-type default post list prepended to every edge's `post:`, with
   explicit per-edge opt-out. This resolves t24.
3. **One precedence rule** for both layers: *default first, specifics appended, opt-out only
   explicit*.
4. **Dogfood config rewrite** — events carry the auto-commit(+push) for non-flow mutations (closes
   t26 as config, not code); `post_defaults` collapses the ~30 duplicated edge lines.
5. **Docs + guard tests** per SEC2 (config is code) and the c19 caveat discipline.

## Non-goals

- **Blocking gates (`commands:`) on events** — post-only in v1 (user decision). The config shape
  (`events.<store>.<event>` holding a pipeline block) is forward-compatible: `commands:` can be
  added additively later.
- **Events on flow transitions.** A move's pipeline home is its edge; events hang on the
  *usecase* layer, not the store, so transitions never double-fire (structural, not conventional).
- **Per-type event refinement** (`events` overrides inside a `type:`) — YAGNI now; when it comes,
  it reuses the same precedence rule (goal 3).
- **Config-level (cross-type) `post_defaults`** — per-type is enough for the real duplication
  (2 types); transitions live on types, so their defaults do too.
- **Batching/debounce across CLI invocations** — rejected in the brainstorm (per-entity firing).
- **Event `description:` guidance text** — config comments suffice in v1; additive later.

## Decisions (brainstorm record, 2026-07-24)

1. **Scope:** events cover **all mutations of tasks and notes** — create/update/delete for both
   stores (not just the title's on_create/on_delete; the motivation is killing *every* manual
   `.mtt` commit).
2. **Granularity:** **per entity**, not per CLI invocation. A bulk `mtt rm t1 t2 t3` fires three
   delete events. Domain-pure ({{.ID}} context always exact); bulk noise is acceptable.
3. **Phases:** **post-only**. `rm` already has its own safety (referenced-check, `--force`
   who/why + audit-first); creation validity lives in core. Fail → mutation kept, exit 5 (t21/t28
   semantics).
4. **Shape:** approach 1 — “two symmetric layers”: events for non-flow mutations + `post_defaults`
   for edges. Approach 2 (store-level events subsuming edge posts) rejected: inter-layer
   precedence, conditional context (losing From→To commit messages), double pipelines per move.
   Approach 3 (hardcoded `autocommit: true`) rejected: git in the binary violates
   engine-is-a-dumb-mechanism; no generality for the t59 pilot; leaves t24/t62 unsolved.
   Pseudo-edges (∅→initial) rejected: notes have no flow, update is not a transition, and fake
   edges break the topology invariants (`initial` = no incoming). Events reuse the *pipeline*
   machinery (Command VO → expand → Runner → typed post error), not the status graph.

## Design

### 1. Config surface (domain: `pkg/mtt`)

```yaml
# top-level, sibling of `types:` — optional; zero value = no events
events:
  task:                 # applies to ALL task types
    create:
      post:
        - 'git add .mtt && git commit -m "{{.ID}}: {{.Event}}" -- .mtt'
    update: { post: [...] }
    delete: { post: [...] }
  note:
    create:
      post:
        - 'git add .mtt && git commit -m "note {{.Slug}}: {{.Event}}" -- .mtt'
    update: { post: [...] }
    delete: { post: [...] }
```

Domain types (no maps — deterministic, no serialization tags, DTO-mapped by the adapter):

- `Config.Events Events` (optional; zero value inert).
- `Events { Task EventHooks; Note EventHooks }`.
- `EventHooks { Create EventHook; Update EventHook; Delete EventHook }`.
- `EventHook { Post []Command }` — the same `Command` VO as edges (`run`/`timeout`; scalar or map
  YAML form via the existing DTO custom unmarshal). `rollback:` inside an event post is rejected
  by validation (post pipelines have no compensation phase — same as edge `post:` today, but made
  explicit here because events are a new authoring surface).
- `EventKind` value object (`create`/`update`/`delete`) — the closed vocabulary for dispatch and
  the `{{.Event}}` placeholder; not a config-file field (the YAML keys are the vocabulary).

Events are **domain config** (authored behavior, like `Transition.Commands`), not adapter
`Settings` (execution policy like `command_timeout`). External providers simply have no events —
the field is optional (provider-agnostic mandatory-minimum rule).

### 2. `post_defaults` on a type + per-edge opt-out (resolves t24)

```yaml
types:
  - name: task
    post_defaults:                    # prepended to every edge's post:
      - 'git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt'
    transitions:
      - {from: tbd, to: speccing, name: start, ...}     # own post: gone
      - from: impl_review
        to: approved
        name: approve
        post:                          # appended AFTER the defaults
          - 'git push -u origin task/{{.ID}}'
          - '…gh pr create…'
      - from: approved
        to: done
        name: deliver
        inherit_post: false            # opt-out (SEC2: needs the narrowed pathspec commit)
        post: [narrow add+commit, git push origin main]
```

- Domain: `Type.PostDefaults []Command`; `Transition.SkipPostDefaults bool` (zero value =
  inherit — the Go default-true pattern). YAML authoring key: `inherit_post:` (omitted/`true` →
  inherit; `false` → skip); the DTO maps `inherit_post: false` → `SkipPostDefaults: true`.
- **Precedence rule (the t24 decision, applied to both layers):** *defaults first, edge specifics
  appended, opt-out only explicit.* No key-wise merge, no reordering.
- Effective post is computed by a pure domain primitive `Type.EffectivePost(tr Transition)
  []Command` (sibling of `FindTransition`); `core.Transitioner` calls it where it reads `tr.Post`
  today. Expansion/execution/failure semantics of the combined list are exactly t21/t28
  (one pipeline, one `PostActionError` with `Remaining`).
- `inherit_post: false` with empty `post:` is legal (edge ends up with no post at all).
- Gates (`commands:`) get **no** defaults mechanism — nothing duplicates them today (YAGNI).

### 3. Firing model (core)

A new small core component (working name `core.EventEmitter`) holds the `Events` config section +
the `Runner` and exposes `TaskEvent(kind, task)` / `NoteEvent(kind, note)`. Mutating usecases call
it **immediately after their successful persist**, inside the usecase:

- task **create**: `Adder` after `store.Create`;
- task **update**: `Editor` (edit/priority), `TagEditor` (per task), `DependencyEditor`,
  `RefEditor` — after `store.Update`;
- task **delete**: `Remover` — per id, after audit append + `store.Delete` (audit-first, c18:
  the trail is written *before* destruction; the event fires *after* it — a delete event cannot
  observe or block the removal);
- note **create/update/delete**: `NoteAdder` / `NoteEditor` + `NoteRefEditor` / `NoteRemover`,
  same pattern.

Rules:

- **Fire only on a real persist.** Idempotent no-ops (adding a present tag, removing an absent
  dep) don't call the store and don't fire — no empty-commit noise.
- **Not the store layer, not the CLI.** Store-level firing would make transitions fire `update`
  (rejected — approach 2). CLI-level firing was considered and rejected on a concrete failure:
  bulk `rm` completes all deletions before the CLI regains control, so the *first* per-entity
  auto-commit pipeline would sweep the whole batch and the remaining ones would fail on empty
  commits. Firing inside `RemoveMany`'s loop keeps mutation→pipeline adjacency.
- The emitter is injected into the mutating usecase constructors; a nil/absent emitter is a no-op
  (tests and event-less configs unaffected). `Transitioner` does **not** get one.

### 4. Placeholder context

Two new strict contexts (sibling structs of `cmdContext` — the whitelist stays self-enforcing,
an out-of-context field is a template error):

- task events: `{ID, Type, Event}`;
- note events: `{Slug, Event}`.

`{{.From}}`/`{{.To}}` do not exist on events (an event is not an edge). Shell-safety: `{{.ID}}` is
load-validated since c15 (`idPattern`); `{{.Slug}}` is structurally validated at construction and
re-validated on load and at `notePath` (`noteSlugRe`, kebab ASCII); `{{.Type}}` comes from config
(trusted, SEC2); `{{.Event}}` is a closed VO. No free text is exposed (same s007 policy).

**Expansion timing:** event post expands **after** the persist (for create the ID does not exist
earlier — the adapter mints it). Uniform rule, no per-event asymmetry: any event-pipeline failure
— template or command — is a *finalization* failure (§5). Edge pipelines keep their eager
expansion; this deviation is confined to events and documented.

### 5. Failure semantics

Exactly the t21/t28 contract, extended to events:

- Event post failure (non-zero exit, operational error, or template error): the mutation is
  **kept**; the usecase returns the persisted result plus a typed post-action error carrying the
  unfinished commands (reuse/parallel `core.PostActionError.Remaining`); the CLI prints the exact
  recovery steps + “the change is already saved”; exit **5**.
- Bulk operations stay best-effort: an event failure on one entity does not stop the rest; per-id
  outcomes ride the bulk report; any event failure ⇒ exit 5 (unless a graver code applies).
- No compensation phase for event posts (post-only layer — same as edge `post:`).

### 6. Bypass & attribution

Mutating commands with event hooks gain `--no-run` (skip the event pipeline). Per the t5
discipline, `--no-run` **forces `--who` + `--why`** (aggregate into the existing
`ErrMissingAttribution`, exit 2) — uniformly, whether or not hooks are configured (one rule, no
conditional surprises). Commands: `add`, `edit`, `priority`, `tag add/rm`, `dep add/rm`,
`ref add/rm`, `rm`, `note add/edit/rm`, `note ref add/rm`. The c19 caveat carries over verbatim:
bypass skips ALL event commands — your commands are your responsibility, bypassed or executed.

### 7. Dogfood config rewrite (this repo)

- `events.task.{create,update,delete}.post` and `events.note.{…}.post`:
  `git add .mtt && git commit -m "<id/slug>: {{.Event}}" -- .mtt` followed by a push that is
  main-aware: `[ "$(git branch --show-current)" != main ] || git push origin main` (closing the
  c7 divergence trap on main while staying silent-but-committed for mid-flight backlog adds on a
  task branch, which the deliver-reconcile note already covers; the branch's `.mtt` commit rides
  the task PR).
- Both types get `post_defaults:` with the auto-commit line; `deliver`/`cancel` edges set
  `inherit_post: false` and keep their narrowed-pathspec commit + `git push origin main` (SEC2:
  a dirty `config.yaml` must never ride a main-landing commit); `approve` keeps only its extra
  push + PR lines; every other edge drops its `post:` block entirely (~28 lines → 2).
- `TestRepoDogfoodConfig` extended to pin all of the above (guard test, SEC2).

### 8. Docs & tests

- **DESIGN.md (+ru):** new “Lifecycle events” subsection (model, firing rules, failure semantics,
  the two-layer decision incl. rejected approaches) + `post_defaults`/`inherit_post` in the flow
  section + a Decisions-table row. Sweep ALL parallel occurrences (EN+RU) per the
  design-docs-parallel-occurrences lesson.
- **CLI_REFERENCE (+ru):** `events:` config reference, `post_defaults:`/`inherit_post:`,
  `--no-run` on mutating commands, exit-5 semantics on mutations.
- **FLOW_GUIDE (+ru):** authoring guidance + the no-silent-traps bar for event pipelines.
- **CLAUDE.md:** `internal/core`, `internal/cli`, `internal/adapter/yaml`, `pkg/mtt`.
- **CHANGELOG:** feature entry under [Unreleased].
- **Tests:** TDD throughout — `pkg/mtt` validation (event command validity, rollback-rejection,
  `EffectivePost`); core emitter unit tests (dispatch matrix, no-op ⇒ no fire, failure contract,
  nil emitter); adapter golden/config round-trip (events + post_defaults + inherit_post, scalar
  and map command forms); CLI e2e testscript (add→auto-commit, bulk rm→per-entity commits,
  edit `--no-run` forces who/why, exit 5 on a failing event post, deliver keeps the narrowed
  pathspec); `TestRepoDogfoodConfig`.

## Consequences for the queue

- **t26** — subsumed: auto-commit becomes dogfood config on the event layer (this task ships it).
  Close via `cancel --why "subsumed by t66"` after delivery.
- **t24** — resolved here (`post_defaults` + explicit opt-out + defaults-first ordering). Close
  the same way; t62's flagship template unblocks.
- **t59 pilot** — unblocked once t66 delivers (the pilot needs lifecycle hooks as config).

## Open seams (recorded, not built)

- `commands:` (blocking gates) on events — additive to `EventHook` if a real use case appears.
- Per-type event refinement — reuses the goal-3 precedence rule.
- Event `description:` for guidance parity with edges.
- An `{{.Event}}`-aware guidance surface (`mtt types`-style listing of configured events) — the
  discoverability question rides t15/t42 docs work if needed.
