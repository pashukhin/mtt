package mtt

// AcceptsParent reports whether a task of type t may sit under a parent whose
// type is named parentType — i.e. parentType is one of t.Parents. A root type
// (empty Parents) accepts no parent, so this also rejects giving an epic a parent.
func (t Type) AcceptsParent(parentType TypeName) bool {
	for _, p := range t.Parents {
		if p == parentType {
			return true
		}
	}
	return false
}

// StatusKind returns the category of the named status within t's flow, or false
// when the status is not part of the flow (e.g. config drift on a stored task).
// Status identity is per-flow, so the lookup stays name-agnostic at the call site.
func (t Type) StatusKind(status StatusName) (StatusKind, bool) {
	for _, s := range t.Statuses {
		if s.Name == status {
			return s.Kind, true
		}
	}
	return "", false
}

// StatusByName returns the named status within t's flow, or false when absent.
// Used to surface a status's Description as inline guidance for the agent.
func (t Type) StatusByName(status StatusName) (Status, bool) {
	for _, s := range t.Statuses {
		if s.Name == status {
			return s, true
		}
	}
	return Status{}, false
}

// TransitionsFrom returns every transition leaving the given status, in config
// (definition) order. Empty for a terminal status (no outgoing edges) or an
// unknown one — so callers can render "next moves" without special-casing.
func (t Type) TransitionsFrom(status StatusName) []Transition {
	var out []Transition
	for _, e := range t.Transitions {
		if e.From == status {
			out = append(out, e)
		}
	}
	return out
}
