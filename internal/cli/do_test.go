package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestAvailableActionsListsNamedEdges(t *testing.T) {
	typ := mtt.Type{Flow: mtt.Flow{Transitions: []mtt.Transition{
		{From: "review", To: "fix", Name: "decline"},
		{From: "review", To: "done", Name: "approve"},
		{From: "review", To: "tbd"}, // unnamed — not listed
	}}}
	got := availableActions(typ, "review")
	if !strings.Contains(got, "decline") || !strings.Contains(got, "approve") {
		t.Fatalf("availableActions = %q, want decline+approve", got)
	}
	// a status with no named edges says so, no dangling "available: ".
	none := availableActions(typ, "fix")
	if strings.Contains(none, "available:") {
		t.Fatalf("no-named-edges case must not print an empty list: %q", none)
	}
}

func TestDoUnknownEdgeIsInvalidTransition(t *testing.T) {
	// The miss error wraps core.ErrInvalidTransition (exit 6) and reads consistently
	// with the sentinel. Built here the way do.go builds it, to lock the taxonomy
	// without spinning up a project.
	typ := mtt.Type{Flow: mtt.Flow{Transitions: []mtt.Transition{{From: "review", To: "fix", Name: "decline"}}}}
	if _, ok := typ.FindTransitionByName("review", "bogus"); ok {
		t.Fatal("precondition: bogus must miss")
	}
	err := doMissError(typ, "bogus", "review")
	if !errors.Is(err, core.ErrInvalidTransition) {
		t.Fatalf("do miss must map to exit 6 (ErrInvalidTransition): %v", err)
	}
	if !strings.Contains(err.Error(), "not allowed by the flow") {
		t.Fatalf("message must align with the sentinel: %v", err)
	}
}
