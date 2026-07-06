package mtt

// CurrentAction is what a transition does to the personal "current task" pointer
// when traversed — a closed domain vocabulary (a value object, like StatusKind),
// not a name. The empty action is the default: most edges leave the pointer alone.
type CurrentAction string

// The current-pointer actions. Empty means "no effect".
const (
	CurrentSet   CurrentAction = "set"   // take the task into work: it becomes current
	CurrentClear CurrentAction = "clear" // release it: the pointer is cleared
)

// Valid reports whether a is one of the defined actions (empty is valid — no effect).
func (a CurrentAction) Valid() bool {
	switch a {
	case "", CurrentSet, CurrentClear:
		return true
	default:
		return false
	}
}
