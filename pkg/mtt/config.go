package mtt

// Config is a whole mtt configuration: project metadata and task types. The only
// mandatory field for a provider is at least one Type; the rest is optional.
type Config struct {
	Version int
	Project Project
	Types   []Type
}

// Project holds project-level metadata.
type Project struct {
	Name string
}

// Type is a task type: its name, optional description, allowed parent types, the
// default marker, and its flow. Mandatory: Name and a Flow whose statuses have a
// name+kind and whose transitions have from/to.
type Type struct {
	Name         TypeName
	Description  string
	Parents      []TypeName
	Default      bool
	PostDefaults []Command // prepended to every edge's Post unless the edge opts out (t66/t24)
	Flow
}

// Flow is a per-type status graph: a closed set of statuses and the transitions
// between them. Status identity is scoped to the flow; there are no cross-flow
// transitions.
type Flow struct {
	Statuses    []Status
	Transitions []Transition
}

// Status is one state in a flow. Kind is a value object; Description is optional.
// Default marks THE entry status when a flow has more than one initial (mirrors
// Type.Default); it is ignored unless the status is initial.
type Status struct {
	Name        StatusName
	Kind        StatusKind
	Description string
	Default     bool
}

// Transition is a directed edge between two statuses of the same flow. Description
// and Commands are optional; Commands run as gates in a later phase.
type Transition struct {
	From        StatusName
	To          StatusName
	Description string
	Name        string // optional edge label — the verb for `mtt <name>` / `mtt do <name>` (empty = unnamed)
	Commands    []Command
	Current     CurrentAction // set|clear the personal current pointer when traversed (empty = no effect)
	Require     Require       // per-edge required attribution (zero = none); unioned with global + --no-run
	Post        []Command     // commands run AFTER persist (finalization, e.g. git commit); non-transactional (t21)

	// SkipPostDefaults opts this edge out of the type's PostDefaults (YAML:
	// inherit_post: false). Zero value = inherit — the t24 precedence rule:
	// defaults first, specifics appended, opt-out only explicit.
	SkipPostDefaults bool
}

// Require is a required-attribution policy: who/why must be supplied. Used as the
// project-global default and as a per-edge (Transition) override; the two are
// unioned (tighten-only) — see core.Transitioner.
type Require struct {
	Who bool
	Why bool
}

// DefaultType returns the type marked default, or the first type when none is
// marked. The bool is false only when there are no types.
func (c Config) DefaultType() (Type, bool) {
	if len(c.Types) == 0 {
		return Type{}, false
	}
	for _, t := range c.Types {
		if t.Default {
			return t, true
		}
	}
	return c.Types[0], true
}

// ChildrenIn returns the types that declare t as a parent — the computed inverse
// of Parents — in config order.
func (t Type) ChildrenIn(c Config) []Type {
	var kids []Type
	for _, other := range c.Types {
		for _, p := range other.Parents {
			if p == t.Name {
				kids = append(kids, other)
				break
			}
		}
	}
	return kids
}

// IsRoot reports whether the type sits at the root level (declares no parents).
func (t Type) IsRoot() bool { return len(t.Parents) == 0 }

// InitialStatus returns the flow's entry status: the initial status marked
// Default, else the first initial in config order. The bool is false when the
// flow has no initial status.
func (t Type) InitialStatus() (Status, bool) {
	var first Status
	found := false
	for _, s := range t.Statuses {
		if s.Kind != KindInitial {
			continue
		}
		if s.Default {
			return s, true
		}
		if !found {
			first, found = s, true
		}
	}
	return first, found
}

// FindTransition returns the edge from -> to in t's flow, if any. The single
// pure edge lookup, shared by core (the gate) and the CLI (reading an edge's
// current action after a move).
func (t Type) FindTransition(from, to StatusName) (Transition, bool) {
	for _, e := range t.Transitions {
		if e.From == from && e.To == to {
			return e, true
		}
	}
	return Transition{}, false
}

// EffectivePost returns the post pipeline that actually runs for edge tr: the
// type's PostDefaults followed by the edge's own Post — unless the edge opts
// out (SkipPostDefaults), then only its own Post. The t24 precedence rule:
// defaults first, specifics appended, opt-out only explicit.
func (t Type) EffectivePost(tr Transition) []Command {
	if tr.SkipPostDefaults || len(t.PostDefaults) == 0 {
		return tr.Post
	}
	out := make([]Command, 0, len(t.PostDefaults)+len(tr.Post))
	out = append(out, t.PostDefaults...)
	return append(out, tr.Post...)
}

// TypeByName returns the type with the given name, or false when absent.
func (c Config) TypeByName(name TypeName) (Type, bool) {
	for _, t := range c.Types {
		if t.Name == name {
			return t, true
		}
	}
	return Type{}, false
}
