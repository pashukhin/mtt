---
title: Recurring architecture decision tests
tags:
    - core
priority: medium
created: "2026-07-23T07:59:10Z"
updated: "2026-07-24T16:50:55Z"
---
- Port-vs-field (GAP #1): can the reference adapter embed it in the aggregate? Yes -> Task field +
  TaskStore.Update, no new port (depends_on, tags, priority, history). No (non-embeddable, e.g. the
  personal current pointer) -> a capability port. Delete cannot be embedded -> base-port method.
- Value objects: closed vocabulary -> type + consts + Valid(), cast in toDomain, validated at the CLI
  boundary, NO smart constructor (StatusKind/Priority/CurrentAction idiom). Open TRANSFORMING
  vocabulary (tags) -> plain []string + pure functions. Named identities -> reject empty, never
  transform.
- Domain-vs-policy for a per-edge property: authored on the specific edge -> domain VO (per-command
  timeout); a runner-wide default -> adapter Settings (global command_timeout).
- Derived graphs (children index, dep graph, roadmap ordering) are computed in core from List, never
  stored, never in pkg/mtt; do not force a shared traversal until a third consumer demands it.
- A pure read needs no core usecase (show/list compose store + pure functions); only mutations get
  usecase structs, clocked via injected now.
- DTO field drops are a silent-bug class: a domain field the DTO does not map dies at Load with green
  tests - test new fields THROUGH Load/toDomain, and audit optional DTO fields when a domain knob
  "does nothing".
- Measured scale posture (2026-07-10, N=5000): list/tree/dep linear (~120ms), gated status O(1) (3ms),
  roadmap ~quadratic (1s; heap fix is t13); the gate path never depends on N.
- Trust model: config is code (Makefile-class); placeholder expansion exposes exactly {ID,Type,From,To}
  via a template struct - free text structurally cannot reach the shell; the binary is zero-network.
- Placeholder shape-safety is per-VALUE-source, not per-field (t66): an exposed template field
  ({{.Type}}) is shell-safe only if its value is constrained. {{.ID}} is safe because the adapter
  validates the id charset at Load (c15); a field read from an unvalidated on-disk value (task type:,
  checked only for non-emptiness) needs its OWN guard even inside the whitelist - the event emitter
  re-resolves the type against the config vocabulary (TypeByName) BEFORE expanding, so a poisoned
  type: never reaches sh -c. Whitelisting the field name is necessary, not sufficient.
- The "mutation kept" contract reaches every CLI call site once a post-persist phase can fail (t21/t28
  post:, t66 events): a *PostActionError means the mutation PERSISTED and only finalization failed.
  Any branch that reads `err != nil` as "it didn't happen" is a latent bug - bulk rm skipped its
  current-pointer cleanup on a failed delete event and left a dangling pointer (t66 review F1). Rule:
  at a mutation call site, distinguish a hard error from a *PostActionError (errors.As) before any
  derived-state cleanup or did-it-happen decision; exit 5 (finalization) must win over a masking
  cleanup error.
