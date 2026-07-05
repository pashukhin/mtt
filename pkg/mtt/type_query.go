package mtt

// AcceptsParent reports whether a task of type t may sit under a parent whose
// type is named parentType — i.e. parentType is one of t.Parents. A root type
// (empty Parents) accepts no parent, so this also rejects giving an epic a parent.
func (t Type) AcceptsParent(parentType string) bool {
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
func (t Type) StatusKind(status string) (StatusKind, bool) {
	for _, s := range t.Statuses {
		if s.Name == status {
			return s.Kind, true
		}
	}
	return "", false
}
