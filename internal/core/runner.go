package core

import (
	"errors"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Runner executes a transition's commands in order and reports each result. It is
// defined here (only core uses it), implemented in internal/adapter/exec, and
// faked in tests. A non-zero exit is DATA (recorded in a Check), not a Go error;
// the returned error signals an operational failure (a command could not launch
// or timed out).
type Runner interface {
	Run(commands []string) ([]mtt.Check, error)
}

// ErrBlocked is returned when a transition's gate does not pass (a command exited
// non-zero, or the runner failed operationally). The task is left unchanged.
var ErrBlocked = errors.New("mtt: transition blocked by a failed gate")

// ErrInvalidTransition is returned when the requested edge is not in the type's
// flow (no transition from the current status to the target).
var ErrInvalidTransition = errors.New("mtt: transition not allowed by the flow")
