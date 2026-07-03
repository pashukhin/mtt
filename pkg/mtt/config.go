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
	Name        string
	Description string
	Parents     []string
	Default     bool
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
type Status struct {
	Name        string
	Kind        StatusKind
	Description string
}

// Transition is a directed edge between two statuses of the same flow. Description
// and Commands are optional; Commands run as gates in a later phase.
type Transition struct {
	From        string
	To          string
	Description string
	Commands    []string
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
