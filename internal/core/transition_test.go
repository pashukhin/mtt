package core

import (
	"errors"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// fakeRunner is the test double for the Runner port: it records the commands it
// was handed and returns canned checks/error without spawning a process.
type fakeRunner struct {
	checks  []mtt.Check
	err     error
	called  bool
	gotCmds []string
}

func (f *fakeRunner) Run(commands []string) ([]mtt.Check, error) {
	f.called = true
	f.gotCmds = commands
	return f.checks, f.err
}

// flowCfg is a one-type config: tbd →(a)→ in_progress →(b)→ done, plus cancelled.
func flowCfg(cmdsA, cmdsB []string) mtt.Config {
	return mtt.Config{
		Version: 1,
		Types: []mtt.Type{{
			Name:    "task",
			Default: true,
			Flow: mtt.Flow{
				Statuses: []mtt.Status{
					{Name: "tbd", Kind: mtt.KindInitial},
					{Name: "in_progress", Kind: mtt.KindActive},
					{Name: "done", Kind: mtt.KindTerminal},
					{Name: "cancelled", Kind: mtt.KindTerminal},
				},
				Transitions: []mtt.Transition{
					{From: "tbd", To: "in_progress", Commands: cmdsA},
					{From: "tbd", To: "cancelled"},
					{From: "in_progress", To: "done", Commands: cmdsB},
					{From: "in_progress", To: "cancelled"},
				},
			},
		}},
	}
}

var testClock = func() time.Time { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }

func baseTask() mtt.Task {
	return mtt.Task{ID: "t1", Type: "task", Title: "A", Status: "tbd",
		Created: testClock(), Updated: testClock()}
}

func TestTransitionAppliesAndRecordsHistory(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "make lint", Exit: 0}}}
	tr := NewTransitioner(store, flowCfg([]string{"make lint"}, nil), runner, testClock)

	got, err := tr.Transition("t1", "in_progress", TransitionOptions{Role: "impl", By: "grisha"})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
	if len(got.History) != 1 {
		t.Fatalf("history len = %d, want 1", len(got.History))
	}
	h := got.History[0]
	if h.From != "tbd" || h.To != "in_progress" || h.By != "grisha" || h.Role != "impl" {
		t.Fatalf("history entry = %+v", h)
	}
	if len(h.Checks) != 1 || h.Checks[0].Cmd != "make lint" || h.Checks[0].Exit != 0 {
		t.Fatalf("history checks = %+v", h.Checks)
	}
	if !got.Updated.Equal(testClock()) {
		t.Fatalf("updated = %v, want %v", got.Updated, testClock())
	}
}

func TestTransitionBlockedOnFailedGate(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "make test", Exit: 1}}}
	tr := NewTransitioner(store, flowCfg([]string{"make test"}, nil), runner, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	reloaded, _ := store.Get("t1")
	if reloaded.Status != "tbd" {
		t.Fatalf("status = %q, want unchanged tbd", reloaded.Status)
	}
	if len(reloaded.History) != 0 {
		t.Fatalf("history written on block: %+v", reloaded.History)
	}
}

func TestTransitionInvalidEdge(t *testing.T) {
	store := newMemStore(baseTask())
	tr := NewTransitioner(store, flowCfg(nil, nil), &fakeRunner{}, testClock)

	_, err := tr.Transition("t1", "done", TransitionOptions{}) // tbd → done not allowed
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestTransitionNoRunBypassesRunner(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{err: errors.New("must not be called")}
	tr := NewTransitioner(store, flowCfg([]string{"make test"}, nil), runner, testClock)

	got, err := tr.Transition("t1", "in_progress", TransitionOptions{NoRun: true})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if runner.called {
		t.Fatalf("runner was called under --no-run")
	}
	if got.Status != "in_progress" || len(got.History) != 1 || len(got.History[0].Checks) != 0 {
		t.Fatalf("no-run result = %+v", got)
	}
}
