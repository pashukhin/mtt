package core

import (
	"errors"
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Runner executes a transition's commands in order and reports each result. It is
// defined here (only core uses it), implemented in internal/adapter/exec, and
// faked in tests. A non-zero exit is DATA (recorded in a Check), not a Go error;
// the returned error signals an operational failure (a command could not launch
// or timed out). At this boundary each Command's Run is ALREADY EXPANDED by core
// (see expand.go); the runner only runs and reports.
type Runner interface {
	// Run executes the commands in order, stopping at the first non-zero exit.
	// CONTRACT (compensation relies on it): on an operational failure the returned
	// checks include a Check for the failing command as the LAST element (Exit -1).
	Run(commands []mtt.Command) ([]mtt.Check, error)
	// Compensate runs the given already-expanded commands best-effort: in order,
	// NEVER stopping, NEVER returning an error (an operational failure is recorded
	// as Exit -1). It reports a labeled compensation phase. core passes the
	// reversed, succeeded-only rollbacks.
	Compensate(commands []mtt.Command) []mtt.Check
}

// ErrBlocked is returned when a transition's gate does not pass (a command exited
// non-zero, or the runner failed operationally). The task is left unchanged.
var ErrBlocked = errors.New("mtt: transition blocked by a failed gate")

// ErrInvalidTransition is returned when the requested edge is not in the type's
// flow (no transition from the current status to the target).
var ErrInvalidTransition = errors.New("mtt: transition not allowed by the flow")

// ErrMissingAttribution is returned when the project's require:{who,why} policy
// is unmet on a transition. It is checked before the gate runs (fail fast) and
// aggregates all missing fields; the CLI maps it to exit code 2.
var ErrMissingAttribution = errors.New("mtt: missing required attribution")

// ErrPostAction is returned when a transition's POST phase (post: commands, run
// AFTER persist) fails. Unlike ErrBlocked, the move IS persisted — only the
// finalization failed; the CLI keeps the move and maps it to exit code 5. This is
// the ONLY case where Transition returns a valid task with a non-nil error.
var ErrPostAction = errors.New("mtt: post-action failed after the move")

// PostActionError is the typed form of ErrPostAction: it carries the post commands
// that did NOT complete (the failed one + those never reached) so the CLI can print
// exact recovery steps, plus the underlying Cause. Is() maps it to ErrPostAction, so
// the CLI still exits 5. This is the single case where Transition returns a valid
// task with a non-nil error (the move IS persisted).
type PostActionError struct {
	Remaining []string // unfinished post commands (expanded where available)
	Cause     string   // the underlying failure, verbatim
}

// Error renders the sentinel prefix plus the underlying cause.
func (e *PostActionError) Error() string { return fmt.Sprintf("%s: %s", ErrPostAction, e.Cause) }

// Is reports whether target is ErrPostAction, so errors.Is (and the CLI's exit-5
// mapping) treat a *PostActionError exactly like the plain sentinel.
func (e *PostActionError) Is(target error) bool { return target == ErrPostAction }
