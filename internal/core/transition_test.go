package core

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// fakeRunner is the test double for the Runner port: it records the commands it
// was handed and returns canned checks/error without spawning a process.
type fakeRunner struct {
	checks       []mtt.Check
	err          error
	called       bool
	gotCmds      []mtt.Command
	compCmds     []mtt.Command // commands passed to Compensate (nil = never called)
	compChecks   []mtt.Check   // canned Compensate result (nil = all succeed)
	failSubstr   string        // when set (and no canned checks/err): derive one check per command, exit 1 if Run contains it — lets the empty pre-gate pass while the post phase fails (t21)
	postOpErr    error         // when set, Run returns this operational error ONLY for a non-empty command slice (the post phase; the empty pre-gate passes)
	postOpChecks []mtt.Check   // checks returned alongside postOpErr (nil = none recorded; lets a test drive the "failing check last" index)
}

func (f *fakeRunner) Run(commands []mtt.Command) ([]mtt.Check, error) {
	f.called = true
	f.gotCmds = commands
	if f.postOpErr != nil && len(commands) > 0 {
		return f.postOpChecks, f.postOpErr
	}
	if f.checks == nil && f.err == nil && f.failSubstr != "" {
		out := make([]mtt.Check, len(commands))
		for i, c := range commands {
			exit := 0
			if strings.Contains(c.Run, f.failSubstr) {
				exit = 1
			}
			out[i] = mtt.Check{Cmd: c.Run, Exit: exit}
		}
		return out, nil
	}
	return f.checks, f.err
}

func (f *fakeRunner) Compensate(commands []mtt.Command) []mtt.Check {
	f.compCmds = commands
	if f.compChecks != nil {
		return f.compChecks
	}
	out := make([]mtt.Check, len(commands))
	for i, c := range commands {
		out[i] = mtt.Check{Cmd: c.Run, Exit: 0}
	}
	return out
}

// rbCmd is a command with a leaf compensator, for compensation tests.
func rbCmd(run, rollback string) mtt.Command {
	return mtt.Command{Run: run, Rollback: &mtt.Command{Run: rollback}}
}

// flowCfgA is flowCfg with explicit Commands on the tbd→in_progress edge (index 0).
func flowCfgA(cmdsA []mtt.Command) mtt.Config {
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Commands = cmdsA
	return cfg
}

// strCmds wraps bare command strings as Commands (no per-command timeout).
func strCmds(ss []string) []mtt.Command {
	if ss == nil {
		return nil
	}
	out := make([]mtt.Command, len(ss))
	for i, s := range ss {
		out[i] = mtt.Command{Run: s}
	}
	return out
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
					{From: "tbd", To: "in_progress", Commands: strCmds(cmdsA)},
					{From: "tbd", To: "cancelled"},
					{From: "in_progress", To: "done", Commands: strCmds(cmdsB)},
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
	if runner.compCmds != nil {
		t.Fatalf("compensated on a successful transition: %+v", runner.compCmds)
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

func TestTransitionWritesWhy(t *testing.T) {
	store := newMemStore(baseTask())
	tr := NewTransitioner(store, flowCfg(nil, nil), &fakeRunner{}, testClock)

	got, err := tr.Transition("t1", "in_progress", TransitionOptions{By: "alice", Why: "start work"})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if got.History[len(got.History)-1].Why != "start work" {
		t.Fatalf("history Why = %q, want %q", got.History[len(got.History)-1].Why, "start work")
	}
}

func TestTransitionMissingAttributionAggregates(t *testing.T) {
	store := newMemStore(baseTask())
	tr := NewTransitioner(store, flowCfg(nil, nil), &fakeRunner{}, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{RequireWho: true, RequireWhy: true})
	if !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("err = %v, want ErrMissingAttribution", err)
	}
	if !strings.Contains(err.Error(), "who") || !strings.Contains(err.Error(), "why") {
		t.Fatalf("error must name both missing fields, got: %v", err)
	}
}

func TestTransitionMissingAttributionSkipsGate(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{} // gate has commands; must NOT be reached
	tr := NewTransitioner(store, flowCfg([]string{"make test"}, nil), runner, testClock)

	if _, err := tr.Transition("t1", "in_progress", TransitionOptions{RequireWho: true}); !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("err = %v, want ErrMissingAttribution", err)
	}
	if runner.called {
		t.Fatal("gate ran; attribution must be checked before the gate (fail fast)")
	}
}

func TestTransitionNoRunDoesNotBypassAttribution(t *testing.T) {
	store := newMemStore(baseTask())
	tr := NewTransitioner(store, flowCfg(nil, nil), &fakeRunner{}, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{NoRun: true, RequireWhy: true})
	if !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("--no-run must not bypass attribution; err = %v", err)
	}
}

func TestTransitionExpandsPlaceholders(t *testing.T) {
	store := newMemStore(baseTask()) // t1, type task, status tbd
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "git checkout -b task/t1", Exit: 0}}}
	tr := NewTransitioner(store, flowCfg([]string{"git checkout -b task/{{.ID}}"}, nil), runner, testClock)

	if _, err := tr.Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "git checkout -b task/t1" {
		t.Fatalf("runner got %+v, want expanded 'git checkout -b task/t1'", runner.gotCmds)
	}
}

func TestTransitionExpandsFromTo(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{}
	tr := NewTransitioner(store, flowCfg([]string{"echo {{.From}} {{.To}}"}, nil), runner, testClock)

	if _, err := tr.Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if runner.gotCmds[0].Run != "echo tbd in_progress" {
		t.Fatalf("expanded = %q, want 'echo tbd in_progress' (From = pre-move status)", runner.gotCmds[0].Run)
	}
}

func TestTransitionUnknownPlaceholderErrors(t *testing.T) {
	store := newMemStore(baseTask())
	tr := NewTransitioner(store, flowCfg([]string{"echo {{.Title}}"}, nil), &fakeRunner{}, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if err == nil || errors.Is(err, ErrBlocked) {
		t.Fatalf("want a plain expansion error (not ErrBlocked), got %v", err)
	}
	reloaded, _ := store.Get("t1")
	if reloaded.Status != "tbd" || len(reloaded.History) != 0 {
		t.Fatalf("task changed on an expansion error: %+v", reloaded)
	}
}

func TestTransitionNoRunSkipsExpansion(t *testing.T) {
	store := newMemStore(baseTask())
	// A template that would fail expansion; --no-run must skip expansion + gate.
	tr := NewTransitioner(store, flowCfg([]string{"echo {{.Title}}"}, nil), &fakeRunner{}, testClock)

	// --no-run forces who+why (t5); supply them so this test exercises expansion-skip.
	got, err := tr.Transition("t1", "in_progress", TransitionOptions{NoRun: true, By: "a", Why: "bypass"})
	if err != nil {
		t.Fatalf("--no-run must skip expansion; err = %v", err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
}

func TestTransitionNoRunBypassesRunner(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{err: errors.New("must not be called")}
	tr := NewTransitioner(store, flowCfg([]string{"make test"}, nil), runner, testClock)

	// --no-run forces who+why (t5); supply them so this test exercises runner-bypass.
	got, err := tr.Transition("t1", "in_progress", TransitionOptions{NoRun: true, By: "a", Why: "bypass"})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if runner.called {
		t.Fatalf("runner was called under --no-run")
	}
	if got.Status != "in_progress" || len(got.History) != 1 || len(got.History[0].Checks) != 0 {
		t.Fatalf("no-run result = %+v", got)
	}
	if runner.compCmds != nil {
		t.Fatalf("--no-run must skip compensation; got %+v", runner.compCmds)
	}
}

func TestTransitionCompensatesSucceededInReverse(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{checks: []mtt.Check{
		{Cmd: "c1", Exit: 0}, {Cmd: "c2", Exit: 0}, {Cmd: "c3", Exit: 1},
	}}
	// c3 (the FAILING command) also carries a rollback (r3); it must NOT run —
	// this guards the non-zero-branch failIdx (rollbacksBefore starts at failIdx-1).
	cfg := flowCfgA([]mtt.Command{rbCmd("c1", "r1"), rbCmd("c2", "r2"), rbCmd("c3", "r3")})
	tr := NewTransitioner(store, cfg, runner, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if len(runner.compCmds) != 2 || runner.compCmds[0].Run != "r2" || runner.compCmds[1].Run != "r1" {
		t.Fatalf("compensated %+v, want [r2 r1] (reverse over succeeded; r3 excluded)", runner.compCmds)
	}
	reloaded, _ := store.Get("t1")
	if reloaded.Status != "tbd" || len(reloaded.History) != 0 {
		t.Fatalf("task changed on a blocked+compensated transition: %+v", reloaded)
	}
	if !strings.Contains(err.Error(), "compensated 2 commands") {
		t.Fatalf("block error missing compensation summary: %v", err)
	}
}

func TestTransitionFirstCommandFailNoCompensation(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "c1", Exit: 1}}}
	cfg := flowCfgA([]mtt.Command{rbCmd("c1", "r1")})
	tr := NewTransitioner(store, cfg, runner, testClock)

	if _, err := tr.Transition("t1", "in_progress", TransitionOptions{}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if runner.compCmds != nil {
		t.Fatalf("compensated %+v, want none (first command failed)", runner.compCmds)
	}
}

func TestTransitionOperationalErrorCompensates(t *testing.T) {
	store := newMemStore(baseTask())
	// c1 ok, c2 operational failure (recorded last as -1 per the Runner CONTRACT).
	runner := &fakeRunner{
		checks: []mtt.Check{{Cmd: "c1", Exit: 0}, {Cmd: "c2", Exit: -1}},
		err:    errors.New(`command "c2" timed out`),
	}
	cfg := flowCfgA([]mtt.Command{rbCmd("c1", "r1"), rbCmd("c2", "r2")})
	tr := NewTransitioner(store, cfg, runner, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if len(runner.compCmds) != 1 || runner.compCmds[0].Run != "r1" {
		t.Fatalf("compensated %+v, want [r1] (succeeded-before-failure only)", runner.compCmds)
	}
}

func TestTransitionBestEffortCompensatorFailureKeepsBlocked(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{
		checks:     []mtt.Check{{Cmd: "c1", Exit: 0}, {Cmd: "c2", Exit: 1}},
		compChecks: []mtt.Check{{Cmd: "r1", Exit: 1}}, // the compensator itself fails
	}
	cfg := flowCfgA([]mtt.Command{rbCmd("c1", "r1"), {Run: "c2"}})
	tr := NewTransitioner(store, cfg, runner, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("a failed compensator must not change the outcome; err = %v", err)
	}
	if !strings.Contains(err.Error(), "compensated 1 command (1 failed)") {
		t.Fatalf("summary should report the failed compensator: %v", err)
	}
}

func TestTransition_NoRunForcesWhoAndWhy(t *testing.T) {
	store := newMemStore(baseTask()) // t1 @ tbd
	cfg := flowCfg(nil, nil)         // edge tbd→in_progress: no commands, no require
	tr := NewTransitioner(store, cfg, &fakeRunner{}, testClock)

	// (b) missing why (By present) → error mentions why
	_, err := tr.Transition("t1", "in_progress", TransitionOptions{By: "alice", NoRun: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "why") {
		t.Fatalf("no-run without why: want ErrMissingAttribution mentioning why, got %v", err)
	}

	// (b′) non-vacuous who: RequireWho=false AND By="" → who forced solely by --no-run
	_, err = tr.Transition("t1", "in_progress", TransitionOptions{Why: "bypass ci", NoRun: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "who") {
		t.Fatalf("no-run without who: want ErrMissingAttribution mentioning who, got %v", err)
	}

	// success: both present → moves, Why recorded
	got, err := tr.Transition("t1", "in_progress", TransitionOptions{By: "alice", Why: "bypass ci", NoRun: true})
	if err != nil {
		t.Fatalf("no-run with who+why: unexpected error %v", err)
	}
	if w := got.History[len(got.History)-1].Why; w != "bypass ci" {
		t.Fatalf("Why not recorded: got %q", w)
	}
}

func TestTransition_PerEdgeRequireUnionsWithGlobal(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Require = mtt.Require{Why: true} // tbd→in_progress requires why
	tr := NewTransitioner(store, cfg, &fakeRunner{}, testClock)

	// global who + edge why → both required; give only who → missing why
	_, err := tr.Transition("t1", "in_progress", TransitionOptions{By: "alice", RequireWho: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "why") {
		t.Fatalf("union: want missing why, got %v", err)
	}
}

func TestTransition_PostRunsAfterPersist(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil) // edge0 tbd→in_progress, no pre-commands
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo hi"})
	runner := &fakeRunner{}
	got, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "echo hi" {
		t.Fatalf("post not run: %+v", runner.gotCmds)
	}
	if reloaded, _ := store.Get("t1"); reloaded.Status != "in_progress" {
		t.Fatalf("not persisted: %q", reloaded.Status)
	}
}

func TestTransition_PostFailureKeepsMove(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"false"})
	runner := &fakeRunner{failSubstr: "false"} // empty pre-gate passes; the post "false" fails
	_, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrPostAction) {
		t.Fatalf("want ErrPostAction, got %v", err)
	}
	var pe *PostActionError
	if !errors.As(err, &pe) || len(pe.Remaining) != 1 || pe.Remaining[0] != "false" {
		t.Fatalf("Remaining should be [false]; pe=%+v", pe)
	}
	if reloaded, _ := store.Get("t1"); reloaded.Status != "in_progress" {
		t.Fatalf("post failure must not roll back; status = %q", reloaded.Status)
	}
}

func TestTransition_PostExpandsPlaceholders(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo {{.ID}}"})
	runner := &fakeRunner{}
	if _, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "echo t1" {
		t.Fatalf("post placeholder not expanded: %+v", runner.gotCmds)
	}
}

func TestTransition_PostExpandErrorIsPostAction(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo {{.Nope}}"}) // unknown field → template error
	_, err := NewTransitioner(store, cfg, &fakeRunner{}, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrPostAction) {
		t.Fatalf("expand error must be ErrPostAction, got %v", err)
	}
	var pe *PostActionError
	if !errors.As(err, &pe) || len(pe.Remaining) != 1 || pe.Remaining[0] != "echo {{.Nope}}" {
		t.Fatalf("expand-error Remaining should be the raw post run; got %+v", pe)
	}
}

func TestTransition_PostActionErrorRemaining(t *testing.T) {
	// 3 post commands; the 2nd fails -> Remaining = the failed + the untried (2 of 3).
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo one", "boom-two", "echo three"})
	runner := &fakeRunner{failSubstr: "boom"} // empty pre-gate passes; post "boom-two" fails
	_, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError, got %T (%v)", err, err)
	}
	if !errors.Is(err, ErrPostAction) {
		t.Fatalf("PostActionError must map to ErrPostAction")
	}
	want := []string{"boom-two", "echo three"}
	if !slices.Equal(pe.Remaining, want) {
		t.Fatalf("Remaining = %v, want %v", pe.Remaining, want)
	}
	if !strings.Contains(pe.Cause, "boom-two") {
		t.Fatalf("Cause = %q, want it to name the failed command", pe.Cause)
	}
}

func TestTransition_PostActionOperationalZeroChecks(t *testing.T) {
	// Operational error in the POST phase with NO recorded checks -> Remaining = all
	// expanded (guarded against len(pchecks)==0, no panic). postOpErr fires only for a
	// non-empty command slice, so the empty pre-gate passes and only the post errors.
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo a", "echo b"})
	runner := &fakeRunner{postOpErr: errors.New("boom timeout")}
	_, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError, got %T (%v)", err, err)
	}
	if want := []string{"echo a", "echo b"}; !slices.Equal(pe.Remaining, want) {
		t.Fatalf("Remaining = %v, want %v (all, guarded against len(pchecks)==0)", pe.Remaining, want)
	}
}

func TestTransition_PostActionOperationalFailingCheckLast(t *testing.T) {
	// Operational error WITH recorded checks: the failing command is last (Runner
	// CONTRACT), so i=len(pchecks)-1>0 -> Remaining = that command + those untried.
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo a", "echo b", "echo c"})
	runner := &fakeRunner{
		postOpErr:    errors.New("boom timeout"),
		postOpChecks: []mtt.Check{{Cmd: "echo a", Exit: 0}, {Cmd: "echo b", Exit: -1}}, // b failed operationally, last recorded
	}
	_, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError, got %T (%v)", err, err)
	}
	if want := []string{"echo b", "echo c"}; !slices.Equal(pe.Remaining, want) {
		t.Fatalf("Remaining = %v, want %v (failing check b + untried c)", pe.Remaining, want)
	}
}

func TestTransition_InvalidMoveOutOfTerminalReadsCleanly(t *testing.T) {
	// A move requested out of a terminal status: empty allowedTargets -> no dangling list.
	store := newMemStore(func() mtt.Task { tk := baseTask(); tk.Status = "done"; return tk }())
	cfg := flowCfg(nil, nil)
	_, err := NewTransitioner(store, cfg, &fakeRunner{}, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
	if strings.Contains(err.Error(), "allowed from done: )") || strings.HasSuffix(err.Error(), ": ") {
		t.Fatalf("terminal message has a dangling empty list: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("terminal message should say so: %q", err.Error())
	}
}

func TestTransition_NotInFlowStatusIsNotCalledTerminal(t *testing.T) {
	// A hand-broken / config-drift status is NOT in the flow at all — it has no
	// outgoing edges, but calling it "terminal" is a wrong diagnosis (c14).
	store := newMemStore(func() mtt.Task { tk := baseTask(); tk.Status = "gone"; return tk }())
	cfg := flowCfg(nil, nil)
	_, err := NewTransitioner(store, cfg, &fakeRunner{}, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
	if strings.Contains(err.Error(), "terminal") {
		t.Fatalf("a not-in-flow status must not be diagnosed as terminal: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "gone") || !strings.Contains(err.Error(), "flow") {
		t.Fatalf("message should name the status and the flow drift: %q", err.Error())
	}
}

func TestTransition_NoRunSkipsPost(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo hi"})
	runner := &fakeRunner{}
	// --no-run forces who+why (t5); supply them.
	got, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress",
		TransitionOptions{NoRun: true, By: "a", Why: "b"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if runner.called {
		t.Fatal("--no-run must skip the post phase")
	}
	if got.Status != "in_progress" {
		t.Fatalf("persist must still happen; status = %q", got.Status)
	}
}

func TestTransition_NoPostUnchanged(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{}
	if _, err := NewTransitioner(store, flowCfg(nil, nil), runner, testClock).Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// The pre-gate calls runner.Run(nil) even for a zero-command edge (it sets
	// `called`), so assert on gotCmds, NOT `called`: with no post, the only Run is
	// the empty pre-gate → gotCmds is nil/len 0. (A post phase would overwrite it.)
	if len(runner.gotCmds) != 0 {
		t.Fatalf("no post → post runner not invoked; got %+v", runner.gotCmds)
	}
}

func TestTransitionRunsEffectivePost(t *testing.T) {
	// type post_defaults [D]; edge0 (tbd→in_progress) inherits and owns [E];
	// a second move (in_progress→done) opts out with own [F].
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].PostDefaults = strCmds([]string{"echo D"})
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo E"})
	for i, e := range cfg.Types[0].Transitions {
		if e.From == "in_progress" && e.To == "done" {
			cfg.Types[0].Transitions[i].Post = strCmds([]string{"echo F"})
			cfg.Types[0].Transitions[i].SkipPostDefaults = true
		}
	}
	runner := &fakeRunner{}
	tr := NewTransitioner(store, cfg, runner, testClock)
	if _, err := tr.Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("move1: %v", err)
	}
	if len(runner.gotCmds) != 2 || runner.gotCmds[0].Run != "echo D" || runner.gotCmds[1].Run != "echo E" {
		t.Fatalf("inheriting edge post = %+v, want [echo D, echo E]", runner.gotCmds)
	}
	if _, err := tr.Transition("t1", "done", TransitionOptions{}); err != nil {
		t.Fatalf("move2: %v", err)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "echo F" {
		t.Fatalf("opted-out edge post = %+v, want [echo F]", runner.gotCmds)
	}
}
