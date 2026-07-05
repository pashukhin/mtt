// Package architecture is a code-form snapshot of mtt's intended domain model:
// the contract surface (domain types + ports + optional capabilities), the core
// usecases and their dependencies, the derived resolved graph, and the open gaps
// — expressed as Go declarations so the whole picture reads at a glance, with
// minimal ambiguity.
//
// This is a DESIGN REFERENCE, not production code. Nothing here is imported by
// the binary; it is a parallel, self-contained view kept in sync with the real
// pkg/mtt contract and internal/core usecases. It compiles on purpose (so the
// signatures stay valid Go and lint-clean) but declares its own types instead of
// importing the real ones — the snapshot may show intended surface the code has
// not grown yet. Function/usecase surfaces are shown as interfaces or as typed
// `var` signatures (no bodies) so the file states shape without behaviour.
//
// Two layers, deliberately different (this is the model's spine):
//
//	Layer A — the CONTRACT / persisted aggregates (pkg/mtt). References across
//	          aggregates are by IDENTITY (typed string ids/names), never by
//	          pointer. This is the serialization + provider boundary: aggregates
//	          stay self-contained, round-trip cleanly, and tolerate config drift
//	          (a status is data, validated lazily, not a live pointer that fails
//	          to load). Canonical DDD: reference other aggregates by identity.
//	Layer B — the RESOLVED graph, built by core from Layer A for traversal
//	          (advance / ready / cycles). Here references ARE pointers — O(1) hops,
//	          compile-time links. Derived, immutable, core-only; NEVER serialized
//	          (the by-name form is the wire form; this form is rebuilt on load).
//
// Stability tiers — each block is tagged:
//
//	T1 — shipped, or the immediate next session (dependencies, s005): precise.
//	T2 — the agent-facing MVP (flow / roles / comments, s006–s009): firm intent.
//	T3 — later phases (KB, search, external adapters): aspirational placeholder.
//
// Layering (dependencies point inward): cli → core → port ← adapter. core imports
// only the domain contract; adapters implement the ports; ID/slug minting and
// serialization live in the adapter; policy (flow, ready, cycles, placement)
// lives in core; the CLI only parses, wires, and formats.
//
// Keeping in sync: when pkg/mtt or internal/core change a T1 signature, update
// the matching block here. T2/T3 blocks are intent and may still move. The
// authoritative prose remains DESIGN.md; this file is its structural index.
package architecture

import (
	"errors"
	"time"
)

// ---------------------------------------------------------------------------
// 1. VALUE OBJECTS — closed vocabularies the code reasons about (never bare
//    strings). Type/status/role NAMES stay open (user config); only these
//    categories are code-level literals.
// ---------------------------------------------------------------------------

// StatusKind is the category of a flow status, fixed by flow topology. [T1]
type StatusKind string

// The three status kinds; every flow needs at least one of each. [T1]
const (
	KindInitial  StatusKind = "initial"  // no incoming, ≥1 outgoing
	KindActive   StatusKind = "active"   // ≥1 incoming, ≥1 outgoing
	KindTerminal StatusKind = "terminal" // ≥1 incoming, no outgoing
)

// RefKind is the closed vocabulary of reference targets. [T1 field / T2–T3 resolution]
type RefKind string

// The four reference kinds. [T1 field]
const (
	RefNote    RefKind = "note"    // resolves only with a KnowledgeStore (T3)
	RefTask    RefKind = "task"    // resolves via TaskStore (T2)
	RefComment RefKind = "comment" // resolves via a CommentStore (T2)
	RefURL     RefKind = "url"     // external; not resolved (optional HEAD later)
)

// Capability names what an adapter can do beyond the mandatory TaskStore. The
// CLI surfaces these via `mtt caps`; core probes support by type assertion. [T2]
type Capability string

// The optional capabilities. Absence yields ErrUnsupported, never a silent skip. [T2]
const (
	CapHistory      Capability = "history"      // HistoryStore
	CapDependencies Capability = "dependencies" // DependencyStore
	CapComments     Capability = "comments"     // CommentStore
	CapSearch       Capability = "search"       // SearchStore
	CapKnowledge    Capability = "knowledge"    // KnowledgeStore
)

// ---------------------------------------------------------------------------
// 2. IDENTITY TYPES — named string identities so the domain's many opaque
//    references cannot be mixed at compile time (a TypeName cannot be passed
//    where a TaskID is wanted). They marshal as plain strings (serialization-
//    transparent) and stay OPAQUE: nothing here parses an id's structure — core
//    never interprets an id's shape (that is adapter-specific). A "smart
//    constructor" therefore does at most normalize / reject empty; a
//    "does this exist?" check is CONTEXTUAL (needs a Config / a store) and lives
//    in Config.Validate / usecases, not in an identity constructor.
// ---------------------------------------------------------------------------

// TaskID is an adapter-minted task identity — opaque (flat per-prefix in YAML,
// PROJ-123 in Jira). Core never parses it. [T1]
type TaskID string

// TypeName is a configured type's name (e.g. epic/task/subtask). [T1]
type TypeName string

// StatusName is a status name. Full status identity is (TypeName, StatusName),
// scoped to one flow — a bare StatusName is not globally unique. [T1]
type StatusName string

// NoteSlug is a knowledge-note identity. [T3]
type NoteSlug string

// NewStatusName illustrates the identity smart-constructor pattern: normalize /
// guard at the boundary, but NOT parse structure and NOT check existence (that is
// contextual). For opaque provider-minted ids (TaskID) a plain conversion is
// enough; this shape matters for user-entered names. [T1]
var NewStatusName func(s string) (StatusName, error)

// ---------------------------------------------------------------------------
// 3. DOMAIN TYPES (LAYER A — contract / persisted aggregates). Pure pkg/mtt
//    values: no serialization tags, no adapter fields (prefix, paths). References
//    across aggregates are by identity; back-references (children, backlinks) are
//    COMPUTED, never stored.
// ---------------------------------------------------------------------------

// Config is a whole project configuration. Mandatory minimum: ≥1 Type. [T1]
type Config struct {
	Version int
	Project Project
	Types   []Type
}

// Project holds project-level metadata. [T1]
type Project struct {
	Name string
}

// Type is a task type: identity + hierarchy (Parents) + flow. Parents defines the
// hierarchy (a type may sit under several parent types); the inverse (children) is
// computed. Default marks the `add`-without-`--type` type (≤1). [T1]
type Type struct {
	Name        TypeName
	Description string
	Parents     []TypeName // allowed parent type names; empty = root level
	Default     bool
	Flow
}

// Flow is a per-type status graph: a closed set of statuses and transitions.
// Status identity is (type, name); there are no cross-flow transitions. [T1]
type Flow struct {
	Statuses    []Status
	Transitions []Transition
}

// Status is one state in a flow. Kind is derived from topology and validated.
// Default marks THE entry status when a flow has >1 initial (must be initial). [T1]
type Status struct {
	Name        StatusName
	Kind        StatusKind
	Description string
	Default     bool
}

// Transition is a directed edge between two statuses of the same flow, referenced
// BY NAME (Layer A). Commands are the local gate augmentation (all must exit 0 or
// the move is blocked); they run behind the Runner port in T2. [T1 fields / T2 exec]
type Transition struct {
	From        StatusName
	To          StatusName
	Description string
	Commands    []string
}

// Task is a single unit of work. Field order == on-disk order (deterministic
// diff). Reserved collections are populated over successive tiers but reserved in
// the model from the start, so the shape never breaks. [T1 shape]
type Task struct {
	ID          TaskID // minted by the adapter; opaque
	Type        TypeName
	Title       string
	Status      StatusName     // validated lazily against the current flow
	Parent      TaskID         // hierarchy edge (forward ref); children computed
	Tags        []string       // reserved; cross-cutting labels          [T3]
	DependsOn   []TaskID       // blocking edges (affects Ready)          [T1/s005]
	Refs        []Ref          // informational verifiable links          [T2/T3]
	Created     time.Time      // domain timestamp; drives list/tree order
	Updated     time.Time      // bumped on every mutation
	Description string         // multi-line allowed
	Comments    []Comment      // tree via nested Replies                 [T2]
	History     []HistoryEntry // append-only transition audit            [T2]
}

// Ref is a structured, verifiable reference — informational, NOT a blocking edge
// (that is DependsOn). ID stays a plain string on purpose: the target is
// heterogeneous (a TaskID, a NoteSlug, or a URL) selected by Kind, so no single
// identity type fits. Verification is capability-aware (note needs a KB). [T2/T3]
type Ref struct {
	Kind  RefKind
	ID    string
	Label string
}

// Comment is a tree node via nested Replies; ID is sequential within the task. [T2]
type Comment struct {
	ID      int
	Author  string
	Created time.Time
	Body    string
	Refs    []Ref
	Replies []Comment
}

// HistoryEntry is one append-only transition record. Role is the roles seam
// (reserved now; routing resolved in T2). By is the subject-identity seam. [T2]
type HistoryEntry struct {
	At     time.Time
	By     string
	Role   string
	From   StatusName
	To     StatusName
	Checks []Check
}

// Check is one gate command's recorded result on a transition. [T2]
type Check struct {
	Cmd  string
	Exit int
}

// Note is a knowledge-base entry (markdown + frontmatter). Notes are versioned:
// saving creates a new version linked to its predecessor. The domain seam is a
// version identity + predecessor link; external KBs use their native versioning. [T3]
type Note struct {
	Slug        NoteSlug
	Version     int
	Predecessor int // 0 for the first version
	Body        string
	Refs        []Ref
	Created     time.Time
}

// ---------------------------------------------------------------------------
// 4. BASE PORT — the mandatory minimum every adapter implements. Pure: no
//    prefix/YAML leaks through it. The adapter mints the ID inside Create.
// ---------------------------------------------------------------------------

// TaskStore is the mandatory-minimum driven port for tasks. [T1]
type TaskStore interface {
	// Create persists a logical task (empty ID); the adapter mints the ID.
	Create(t Task) (Task, error)
	// Get loads a task by ID; ErrNotFound when it does not resolve.
	Get(id TaskID) (Task, error)
	// List returns all tasks; order unspecified — callers impose their own.
	List() ([]Task, error)
	// Update overwrites an existing task by ID; never mints, never creates.
	Update(t Task) (Task, error)
}

// ---------------------------------------------------------------------------
// 5. OPTIONAL CAPABILITY PORTS — atop the base. An adapter implements what it
//    can; core probes by type assertion and degrades with ErrUnsupported.
//
//    KEY DESIGN NOTE (the biggest current gap). For the YAML REFERENCE adapter,
//    DependsOn / History / Comments all live INSIDE the Task and round-trip
//    through TaskStore.Update — so YAML needs NO extra port for them; core reads
//    the fields and applies policy. These capability interfaces exist so that
//    EXTERNAL adapters (Jira, GitHub) can expose the same features when the data
//    is NOT a simple embedded field, and so core can light features up per
//    backend. Decision to lock before s005: core edits the Task field and
//    persists via Update by default, using these interfaces only when an adapter
//    advertises the capability AND cannot embed. (See GAPS.)
// ---------------------------------------------------------------------------

// DependencyStore manages blocking edges when they are not a plain Task field.
// Cycle rejection is a CORE rule, applied before persisting either way. [T1 rule / T2 port]
type DependencyStore interface {
	AddDependency(id, dependsOn TaskID) error
	RemoveDependency(id, dependsOn TaskID) error
	Dependencies(id TaskID) (blocks []TaskID, blockedBy []TaskID, err error)
}

// HistoryStore appends transition records when they are not embedded in the Task.
// The YAML adapter embeds them (writes via Update); this is for external backends. [T2]
type HistoryStore interface {
	AppendHistory(id TaskID, e HistoryEntry) error
	History(id TaskID) ([]HistoryEntry, error)
}

// CommentStore manages the comment tree when not embedded in the Task. [T2]
type CommentStore interface {
	AddComment(taskID TaskID, c Comment, replyTo int) (Comment, error)
	Comments(taskID TaskID) ([]Comment, error)
}

// SearchStore is optional full-text search over tasks (and the KB). No RAG; an
// external indexer may back it via a config hook. [T3]
type SearchStore interface {
	Search(query string) ([]Task, error)
}

// KnowledgeStore is the second independent port (KB, like Confluence atop Jira).
// A pairing = a configured pair of adapters; the two ports can be mixed. [T3]
type KnowledgeStore interface {
	CreateNote(n Note) (Note, error)
	GetNote(slug NoteSlug, version int) (Note, error) // version 0 = latest
	ListNotes() ([]Note, error)
	NoteHistory(slug NoteSlug) ([]Note, error)
}

// CapabilityReporter is implemented by every backend so the CLI (`mtt caps`) and
// core can discover what is available without trial-and-error. [T2]
type CapabilityReporter interface {
	Capabilities() []Capability
}

// ---------------------------------------------------------------------------
// 6. ERROR TAXONOMY — the sentinels that are part of the contract. Adapters wrap
//    with %w; callers match with errors.Is. (Adapter-local errors like
//    ErrNotInitialized / ErrAlreadyInitialized stay in the adapter, not here.)
// ---------------------------------------------------------------------------

// ErrNotFound is returned by TaskStore.Get / KnowledgeStore when an ID/slug does
// not resolve. [T1]
var ErrNotFound = errors.New("mtt: not found")

// ErrUnsupported is returned when a requested optional capability is absent on the
// active backend — an explicit, matchable signal, never a silent no-op. [T2]
var ErrUnsupported = errors.New("mtt: capability not supported by this backend")

// ErrConflict signals a write conflict — notably the known YAML limitation of
// sequential-ID collision across branches (add/add). A namespaced prefix per
// agent is the escape hatch if real concurrency appears. [T1 seam]
var ErrConflict = errors.New("mtt: write conflict")

// ---------------------------------------------------------------------------
// 7. CORE — usecase logic. Depends ONLY on the domain contract above, never on an
//    adapter. Split by mutation vs pure read: mutations are usecase structs with
//    an injected clock and a store; pure reads are plain functions/values with no
//    store and no clock. Shown as interfaces (stateful usecases) and typed vars
//    (pure functions) so dependencies are explicit without bodies.
// ---------------------------------------------------------------------------

// AddParams are the inputs to Add. Parent and NoParent are mutually exclusive
// (enforced at the CLI). [T1]
type AddParams struct {
	Title       string
	TypeName    TypeName
	Parent      TaskID
	NoParent    bool
	Description string
}

// Adder creates a task: resolve type, validate placement (parent exists via
// TaskStore.Get and Type.AcceptsParent), pick the entry status, stamp times, and
// persist via Create (adapter mints the ID). [T1]
type Adder interface {
	Add(p AddParams) (Task, error)
}

// NewAdder wires the Add usecase — the signature shows its dependencies (a store,
// the config, and an injected clock for deterministic tests). [T1]
var NewAdder func(store TaskStore, cfg Config, now func() time.Time) Adder

// EditParams carry the editable non-flow fields; a nil pointer means unchanged.
// Status is deliberately NOT here — it moves through the flow so gates apply. [T1]
type EditParams struct {
	Title       *string
	Description *string
}

// Editor edits title/description only, bumping Updated from the injected clock. [T1]
type Editor interface {
	Edit(id TaskID, p EditParams) (Task, error)
}

// NewEditor wires the Edit usecase. [T1]
var NewEditor func(store TaskStore, now func() time.Time) Editor

// ListFilter holds the list/tree predicates and ordering. Within a field values
// are OR-ed; across fields AND-ed. cfg is consulted only for the Kinds dimension. [T1]
type ListFilter struct {
	Statuses []StatusName
	Types    []TypeName
	Kinds    []StatusKind
	Parent   TaskID
	Sort     SortKey
}

// SortKey selects the list ordering (timestamp descending, ID tiebreak). [T1]
type SortKey string

// The supported sort keys; empty defaults to SortCreated. [T1]
const (
	SortCreated SortKey = "created"
	SortUpdated SortKey = "updated"
)

// Match is the single node predicate (status/type/kind/parent), shared by Select
// and the tree walk (DRY). [T1]
var Match func(t Task, f ListFilter, cfg Config) bool

// Select filters by Match and imposes a deterministic order (Created/Updated desc,
// ID tiebroken as an opaque string). Pure — no store, provider-agnostic. [T1]
var Select func(tasks []Task, f ListFilter, cfg Config) []Task

// Index is the derived hierarchy over a task slice: parent→children, ancestors,
// roots; cycle-safe; orphans (dangling parent) surface as roots. Children are
// COMPUTED, never stored. A small instance of Layer B (over Parent). [T1]
type Index interface {
	Get(id TaskID) (Task, bool)
	Roots() []Task
	Children(id TaskID) []Task
	Ancestors(id TaskID) []Task // root-first breadcrumb, excludes self
}

// NewIndex builds the hierarchy view from a task slice. [T1]
var NewIndex func(tasks []Task) Index

// Ready reports the actionable tasks: status not terminal AND every DependsOn is
// terminal (by Kind category, never a literal). Pure read over the task set +
// config. Conservative: an unresolvable status or a dangling blocker leaves a
// task not-ready. One primitive behind mtt ready and list --ready. [shipped s005]
var Ready func(tasks []Task, cfg Config) []Task

// DependencyEditor mutates DependsOn (add/remove) and persists via
// TaskStore.Update by default (YAML path), rejecting cycles first. A
// DependencyStore is used only when the backend advertises it and cannot embed.
// Shown as a usecase to make the cycle-check ownership explicit (it is CORE).
// Shipped in s005 as a concrete struct (add/rm mutate DependsOn, reject self +
// cycles via DepGraph.Reaches; add and rm both idempotent — duplicate/absent-edge
// are no-ops). [shipped s005]
type DependencyEditor interface {
	AddDependency(id, dependsOn TaskID) (Task, error)
	RemoveDependency(id, dependsOn TaskID) (Task, error)
}

// NewDependencyEditor wires dependency mutation. Shipped (s005) with a YAGNI
// signature — no DependencyStore param: the edge rides Task.DependsOn and
// persists via Update. The capability port is added only when an external
// adapter that cannot embed the field needs it. [shipped s005]
var NewDependencyEditor func(store TaskStore, now func() time.Time) DependencyEditor

// Runner executes a transition's Commands and reports each result. It is defined
// in CORE (only core uses it), implemented in internal/adapter/exec, faked in
// tests. Commands run in order with cwd = project root (held by the exec adapter,
// NOT passed here — keeps core free of filesystem paths) and a per-command
// timeout, aborting on the first non-zero exit. A non-zero exit is DATA (a
// Check), not a Go error; the error signals an operational failure (launch /
// timeout). [shipped s006]
type Runner interface {
	Run(commands []string) ([]Check, error)
}

// Transitioner applies a SINGLE flow edge (mtt status <id> <new>): validate the
// current status → to against the type's transitions, gate on the edge's Commands
// via Runner (ErrBlocked on a non-zero exit; the task is left unchanged), append
// a HistoryEntry, persist via TaskStore.Update. No new port — history rides
// Task.History (GAP #1 rule). A single-edge lookup, NOT ResolvedFlow (that earns
// its keep in s007's multi-edge Advancer). Sentinels ErrBlocked /
// ErrInvalidTransition live in core (flow is core policy); the CLI maps them to
// exit codes 3 / 6. [shipped s006]
type Transitioner interface {
	Transition(id TaskID, to StatusName, opts TransitionOptions) (Task, error)
}

// TransitionOptions carry the roles seam (Role, from --role/MTT_ROLE), the
// subject-identity By (from --by > MTT_BY > config.local `author` — GAP #5
// resolved), and NoRun (bypass the gate). [shipped s006]
type TransitionOptions struct {
	Role  string
	By    string
	NoRun bool
}

// NewTransitioner wires the single-edge usecase (store for load/persist, config
// for the flow, Runner for the gate, injected clock for history). [shipped s006]
var NewTransitioner func(store TaskStore, cfg Config, runner Runner, now func() time.Time) Transitioner

// AdvanceMode selects how far a walk proceeds. [T2]
type AdvanceMode string

// The advance modes. [T2]
const (
	AdvanceStop   AdvanceMode = "stop"   // default: until the first failed gate or fork
	AdvanceAtomic AdvanceMode = "atomic" // all-or-nothing by status
	AdvanceForce  AdvanceMode = "force"  // ignore gates (emergency)
)

// AdvanceOptions parameterize a walk; Role is the roles seam (semantic routing,
// not RBAC) — what a verb means for a role. [T2]
type AdvanceOptions struct {
	Mode AdvanceMode
	Role string
}

// Advancer is the meta-command behind start/done/cancel: it walks a task through
// the flow to a target status, running each edge's gates (via Runner) and
// appending History. start = --to <first active>, done = --to <terminal>. The
// resolver is parameterized by Role (today one implicit role). [T2]
type Advancer interface {
	Advance(id TaskID, toStatus StatusName, opts AdvanceOptions) (Task, error)
}

// NewAdvancer wires the flow walker — signature shows its full dependency set
// (store for load/persist, config for the flow, Runner for gates, injected clock
// for history timestamps). [T2]
var NewAdvancer func(store TaskStore, cfg Config, runner Runner, now func() time.Time) Advancer

// ---------------------------------------------------------------------------
// 8. RESOLVED GRAPH (LAYER B) — core builds this from Layer A for traversal.
//    Here references ARE pointers: O(1) hops, compile-time links, no repeated
//    name lookups. Concrete structs, not interfaces (single, core-internal
//    implementation — an interface would only pay off for provider polymorphism,
//    which the resolved graph does not need; ports stay the interface boundary).
//    Derived, immutable, NEVER serialized — the Layer A by-name form is the wire
//    form and this is rebuilt on load. Index (above) is the shipped instance of
//    this idea over the Parent edge; ResolvedFlow is the s006 instance over the
//    transition edge.
// ---------------------------------------------------------------------------

// ResolvedFlow is the linked status graph core builds from a Flow for advance /
// ready / cycle traversal. [T2]
type ResolvedFlow struct {
	Statuses map[StatusName]*ResolvedStatus
	Initial  []*ResolvedStatus // entry states (≥1)
}

// ResolvedStatus is a status node with real in/out edges (value data embedded). [T2]
type ResolvedStatus struct {
	Status
	Out []*ResolvedEdge
	In  []*ResolvedEdge
}

// ResolvedEdge is a transition with resolved endpoints (pointers, not names). [T2]
type ResolvedEdge struct {
	From     *ResolvedStatus
	To       *ResolvedStatus
	Commands []string
}

// ---------------------------------------------------------------------------
// 9. DEPENDENCY MAP (who depends on what)
//
//	cmd/mtt ─▶ internal/cli ─▶ internal/core ─▶ pkg/mtt (domain + ports)
//	                                                 ▲
//	                     internal/adapter/yaml ──────┘  (implements TaskStore +,
//	                     internal/adapter/exec  ─▶ core.Runner   later, the
//	                                                    optional capabilities)
//
//	- cli: parse → wire adapters from config → call core / pure reads → format.
//	  Pure reads (show/list/tree) may call a TaskStore method directly (no usecase).
//	- core: policy only (placement, ready, cycles, flow). Imports pkg/mtt only.
//	  Builds Layer B graphs on demand; never imports an adapter.
//	- adapter/yaml: the FULL reference provider — mints IDs, maps DTOs, embeds the
//	  optional data in the Task, implements every capability.
//	- adapter/exec: implements core.Runner (transition commands).
//	- External adapters (T3): a Go adapter implementing the pkg/mtt ports, or a
//	  subprocess adapter over a JSON stdin/stdout protocol (no Go import).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 10. GAPS / OPEN QUESTIONS (decide before the surface locks in)
//
//  1. Capability vs field (BIGGEST, decide before s005). Confirm the rule:
//     core mutates the embedded Task field (DependsOn/History/Comments) and
//     persists via Update for the YAML reference; the capability ports are only
//     for external backends that cannot embed. If accepted, s005 adds NO port —
//     only core.DependencyEditor + Ready, mirroring how s004 added --parent with
//     no new port. (Recommendation: accept.)
//     RESOLVED (s005): accepted — DependencyEditor + Ready shipped, no port; the
//     DependencyStore param was dropped from NewDependencyEditor (YAGNI).
//
//  2. Typed-identity retrofit. DONE (chore 004.5). The shipped pkg/mtt/core/
//     adapter/cli now use TaskID/TypeName/StatusName; the YAML DTO keeps plain
//     strings on disk and maps string<->typed at its boundary, and toDomain
//     rejects an empty on-disk id/type/status via the smart constructors. Ref.ID
//     stays string (heterogeneous target); NoteSlug is deferred to the KB tier
//     (no caller yet). Constructors reject empty and do not transform.
//
//  3. Error taxonomy. ErrNotFound is real (T1). Reserve ErrUnsupported and
//     ErrConflict now (so consumers can branch on them) or when first thrown?
//     Reserving early avoids a later breaking change.
//
//  4. Capabilities() shape. []Capability (this snapshot) vs a struct of bools vs a
//     set type. A slice is simplest and forward-compatible; confirm.
//
//  5. Subject identity (By) source. Who is "acting", for history attribution —
//     distinct from --role ("what hat"). RESOLVED (s006): By is written from
//     --by > MTT_BY > the config.local.yaml `author` field (the durable personal
//     default; surfaced via the adapter Settings.Author). role stays flag/env only
//     (per-invocation). A git-independent edit-audit trail (queryable edit history
//     beyond transitions) stays deferred (a dedicated slice).
//
//  6. Resolved-graph generality. Index (Parent edge) and ResolvedFlow (transition
//     edge) and the dependency graph (DependsOn edge) are three instances of one
//     idea. Do they share one traversal primitive (visited-set + injected
//     edge-provider) or stay separate? A shared primitive is DRY but must not
//     force a premature abstraction — revisit when s005 lands the second graph.
//     RESOLVED (s005): DepGraph (over DependsOn) landed and was kept SEPARATE
//     from Index — a shared primitive would be forced (single-parent tree walked
//     upward vs multi-edge DAG walked downward with a computed reverse index).
//     Revisit if a third graph (ResolvedFlow, s006) naturally shares it.
//
//  7. External-adapter authority (T3). For a subprocess/Jira adapter, which flow
//     is authoritative — our config or the backend's native workflow — and how our
//     Commands relate to its transitions. Moot for YAML; decided with the first
//     external adapter.
//
//  8. KB note identity (T3). Note carries Version + Predecessor here; confirm this
//     is the minimal seam and that external KBs can satisfy it via native versions.
// ---------------------------------------------------------------------------
