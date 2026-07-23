package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Exact gate strings — byte-for-byte copies of .mtt/config.yaml. A mangled or
// inverted gate must fail HERE, not at runtime.
const (
	cmdEntrySwitch = `git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})`
	cmdEntryGuard  = `test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on this branch — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }`
	cmdSpecGlob    = `ls docs/superpowers/specs/{{.ID}}-*.md`
	cmdPlanGlob    = `ls docs/superpowers/plans/{{.ID}}-*.md`
	cmdMakeCheck   = `make check`
	cmdCleanTree   = `out=$(git status --porcelain -- ":(exclude).mtt") && test -z "$out" || { echo "working tree not clean - commit your code/docs first (.mtt is swept by the move itself)" >&2; false; }`
	cmdChangelog   = `git diff --quiet main...HEAD -- cmd internal pkg go.mod go.sum || git diff --name-only main...HEAD -- CHANGELOG.md | grep -q . || { echo "code changed but CHANGELOG.md has no entry - add one under [Unreleased] (pure refactor? bypass: mtt do submit --no-run --who ... --why ...)" >&2; false; }`
	cmdDeclineBack = `git switch task/{{.ID}}`
	cmdSwitchMain  = `git switch main`
	cmdDeliverLog  = `git log -n 200 --format=%s | grep "^{{.ID}}: " || { echo "no squash commit \"{{.ID}}: …\" on local main: git pull first, and check the PR/merge title started with \"{{.ID}}: \"" >&2; false; }`
	cmdCancelGuard = `test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }`
	cmdPostCommit  = `git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt`
	// deliver/cancel commit ON main (their pre-gate switches there), so their
	// post add-pathspec is narrowed to the task file (+audit.log when present) —
	// an uncommitted config.yaml edit must not ride the auto-commit/push to main
	// past review (SEC2, c3).
	cmdPostCommitMain = `a=.mtt/tasks/{{.ID}}.yaml; test -f .mtt/audit.log && a="$a .mtt/audit.log"; git add -- $a && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- $a`
	cmdPushBranch     = `git push -u origin task/{{.ID}}`
	cmdPushMain       = `git push origin main`
	// approve also opens/updates the PR (idempotent, config-only) — c1 pushed the
	// branch, t27 opens the PR. Byte-matches the .mtt/config.yaml approve post[2].
	cmdPrCreate = `[ -n "$(gh pr list --head task/{{.ID}} --state open --json number --jq ".[].number")" ] || { t="{{.ID}}: $(mtt show {{.ID}} --json | jq -r ".title // empty")"; if test -f docs/superpowers/pr/{{.ID}}.md; then gh pr create --base main --head task/{{.ID}} --title "$t" --body-file docs/superpowers/pr/{{.ID}}.md; else gh pr create --base main --head task/{{.ID}} --title "$t" --body "Automated PR for {{.ID}} — see: mtt show {{.ID}}"; fi; }`
)

// TestRepoDogfoodConfig is the SOLE guard of this repo's committed
// .mtt/config.yaml (Config.Validate runs on add/types, never on Load). It
// pins the root to THIS repo (go.mod beside .mtt), copies the committed
// config into a temp root, and loads it THERE — the gitignored
// .mtt/config.local.yaml overlay can neither redden nor mask the committed
// artifact — then asserts the full flow-v2 shape for both types.
func TestRepoDogfoodConfig(t *testing.T) {
	root, err := FindRoot(".")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("found root %q is not this repo (no go.mod): %v", root, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".mtt", "config.yaml"))
	if err != nil {
		t.Fatalf("read committed config: %v", err)
	}
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".mtt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".mtt", "config.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, settings, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if len(cfg.Types) != 2 {
		t.Fatalf("types = %d, want 2 (task, chore)", len(cfg.Types))
	}
	task, chore := cfg.Types[0], cfg.Types[1]
	if task.Name != "task" || chore.Name != "chore" {
		t.Fatalf("type names = %q, %q; want task, chore", task.Name, chore.Name)
	}
	if !task.Default || chore.Default {
		t.Fatalf("default flags: task=%v chore=%v, want true/false", task.Default, chore.Default)
	}
	if settings.Prefixes["task"] != "t" || settings.Prefixes["chore"] != "c" {
		t.Fatalf("prefixes = %v, want task:t chore:c", settings.Prefixes)
	}
	if !settings.Require.Who {
		t.Fatalf("require.who must be true")
	}
	if settings.CommandTimeout != 5*time.Minute {
		t.Fatalf("global command_timeout = %v, want the 5m code default (D8)", settings.CommandTimeout)
	}

	taskKinds := map[mtt.StatusName]mtt.StatusKind{
		"tbd": mtt.KindInitial, "speccing": mtt.KindActive,
		"spec_review": mtt.KindActive, "spec_human_review": mtt.KindActive,
		"spec_fix": mtt.KindActive, "planning": mtt.KindActive,
		"plan_review": mtt.KindActive, "plan_human_review": mtt.KindActive,
		"plan_fix": mtt.KindActive, "implementing": mtt.KindActive,
		"impl_review": mtt.KindActive, "impl_fix": mtt.KindActive,
		"approved": mtt.KindActive,
		"done":     mtt.KindTerminal, "cancelled": mtt.KindTerminal,
	}
	choreKinds := map[mtt.StatusName]mtt.StatusKind{
		"tbd": mtt.KindInitial, "implementing": mtt.KindActive,
		"impl_review": mtt.KindActive, "impl_fix": mtt.KindActive,
		"approved": mtt.KindActive,
		"done":     mtt.KindTerminal, "cancelled": mtt.KindTerminal,
	}
	assertKinds(t, task, taskKinds)
	assertKinds(t, chore, choreKinds)

	if got := len(task.Transitions); got != 27 {
		t.Fatalf("task transitions = %d, want 27", got)
	}
	if got := len(chore.Transitions); got != 11 {
		t.Fatalf("chore transitions = %d, want 11", got)
	}

	// every edge auto-commits .mtt via a post: command (t21); approve (→approved)
	// also pushes the task branch and deliver (approved→done) pushes main (c1) — a
	// dropped or drifted block reddens on the exact literal.
	for _, ty := range []mtt.Type{task, chore} {
		for _, tr := range ty.Transitions {
			// deliver (approved→done) and every cancel (→cancelled) commit on main
			// and must use the narrowed pathspec; all other edges keep the broad one.
			wantPost := cmdPostCommit
			if tr.To == "cancelled" || (tr.From == "approved" && tr.To == "done") {
				wantPost = cmdPostCommitMain
			}
			if len(tr.Post) < 1 || tr.Post[0].Run != wantPost {
				t.Fatalf("%s %s->%s post[0] = %+v, want %q first", ty.Name, tr.From, tr.To, tr.Post, wantPost)
			}
			switch {
			case tr.To == "approved": // approve: push the branch for the PR, then open the PR (t27)
				if len(tr.Post) != 3 || tr.Post[1].Run != cmdPushBranch || tr.Post[2].Run != cmdPrCreate {
					t.Fatalf("%s %s->approved post = %+v, want [commit, %q, %q]", ty.Name, tr.From, tr.Post, cmdPushBranch, cmdPrCreate)
				}
			case tr.From == "approved" && tr.To == "done": // deliver: push main
				if len(tr.Post) != 2 || tr.Post[1].Run != cmdPushMain {
					t.Fatalf("%s deliver post = %+v, want [commit, %q]", ty.Name, tr.Post, cmdPushMain)
				}
			case tr.To == "cancelled": // cancel: push main too (symmetry with deliver, c5)
				if len(tr.Post) != 2 || tr.Post[1].Run != cmdPushMain {
					t.Fatalf("%s cancel %s->cancelled post = %+v, want [commit, %q]", ty.Name, tr.From, tr.Post, cmdPushMain)
				}
			default:
				if len(tr.Post) != 1 {
					t.Fatalf("%s %s->%s post = %+v, want single commit only", ty.Name, tr.From, tr.To, tr.Post)
				}
			}
		}
	}

	// entry edges: name/current + exact two-command pipeline (both types).
	for _, tc := range []struct {
		typ mtt.Type
		to  mtt.StatusName
	}{{task, "speccing"}, {chore, "implementing"}} {
		e := edge(t, tc.typ, "tbd", tc.to)
		if e.Name != "start" || e.Current != mtt.CurrentSet {
			t.Fatalf("%s entry = {name:%q current:%q}, want {start set}", tc.typ.Name, e.Name, e.Current)
		}
		assertRuns(t, tc.typ, e, cmdEntrySwitch, cmdEntryGuard)
	}

	// spec/plan submit edges: exact glob gates.
	for _, sp := range [][2]mtt.StatusName{
		{"speccing", "spec_review"}, {"spec_fix", "spec_review"},
	} {
		assertRuns(t, task, namedEdge(t, task, sp[0], sp[1], "submit"), cmdSpecGlob, cmdCleanTree)
	}
	for _, sp := range [][2]mtt.StatusName{
		{"planning", "plan_review"}, {"plan_fix", "plan_review"},
	} {
		assertRuns(t, task, namedEdge(t, task, sp[0], sp[1], "submit"), cmdPlanGlob, cmdCleanTree)
	}

	// impl submit edges: clean tree, then the CHANGELOG check, then make check
	// with the 10m per-command timeout (D8, t31), both types.
	for _, tc := range []mtt.Type{task, chore} {
		for _, from := range []mtt.StatusName{"implementing", "impl_fix"} {
			e := namedEdge(t, tc, from, "impl_review", "submit")
			assertRuns(t, tc, e, cmdCleanTree, cmdChangelog, cmdMakeCheck)
			if e.Commands[2].Timeout != 10*time.Minute {
				t.Fatalf("%s %s->impl_review make-check timeout = %v, want 10m", tc.Name, from, e.Commands[2].Timeout)
			}
		}
	}

	// delivery tail (both types): approve, decline-back-to-branch, deliver.
	for _, tc := range []mtt.Type{task, chore} {
		// approve pushes the branch and opens the PR — the tree must be fully
		// committed or the PR ships incomplete work (t31).
		assertRuns(t, tc, namedEdge(t, tc, "impl_review", "approved", "approve"), cmdCleanTree)
		assertRuns(t, tc, namedEdge(t, tc, "approved", "impl_fix", "decline"), cmdDeclineBack)
		d := namedEdge(t, tc, "approved", "done", "deliver")
		if d.Current != mtt.CurrentClear {
			t.Fatalf("%s deliver current = %q, want clear", tc.Name, d.Current)
		}
		assertRuns(t, tc, d, cmdSwitchMain, cmdDeliverLog)
	}

	// full cancel matrix (no forward-trap): every non-terminal except _review pairs.
	taskCancels := []mtt.StatusName{"tbd", "speccing", "planning", "implementing", "spec_fix", "plan_fix", "impl_fix", "approved"}
	choreCancels := []mtt.StatusName{"tbd", "implementing", "impl_fix", "approved"}
	assertCancels(t, task, taskCancels)
	assertCancels(t, chore, choreCancels)

	// descriptions are load-bearing (self-instructing runbook): all present,
	// plus the two key instruction strings.
	for _, tc := range []mtt.Type{task, chore} {
		for _, s := range tc.Statuses {
			if strings.TrimSpace(s.Description) == "" {
				t.Fatalf("%s status %q has no description", tc.Name, s.Name)
			}
		}
		for _, tr := range tc.Transitions {
			if strings.TrimSpace(tr.Description) == "" {
				t.Fatalf("%s edge %s->%s has no description", tc.Name, tr.From, tr.To)
			}
		}
	}
	if s, _ := chore.StatusByName("impl_review"); !strings.Contains(s.Description, "it must be a") {
		t.Fatalf("chore impl_review description lost the type-boundary police line: %q", s.Description)
	}
	if d := namedEdge(t, task, "approved", "done", "deliver"); !strings.Contains(d.Description, "pull main") {
		t.Fatalf("task deliver description lost the pull-main hint: %q", d.Description)
	}
	// both entry edges remind you to pull main first (a stale base makes conflict-prone PRs; c6).
	for _, tc := range []struct {
		typ mtt.Type
		to  mtt.StatusName
	}{{task, "speccing"}, {chore, "implementing"}} {
		if e := namedEdge(t, tc.typ, "tbd", tc.to, "start"); !strings.Contains(e.Description, "pull main first") {
			t.Fatalf("%s start description lost the pull-main-first reminder: %q", tc.typ.Name, e.Description)
		}
	}
	if s, _ := task.StatusByName("speccing"); !strings.Contains(s.Description, "superpowers:brainstorming") {
		t.Fatalf("task speccing description lost the brainstorm step: %q", s.Description)
	}
	for _, tc := range []mtt.Type{task, chore} {
		if s, _ := tc.StatusByName("approved"); !strings.Contains(s.Description, "gh pr create") {
			t.Fatalf("%s approved description lost the PR command: %q", tc.Name, s.Description)
		}
		if d := namedEdge(t, tc, "approved", "done", "deliver"); !strings.Contains(d.Description, "mtt note add") {
			t.Fatalf("%s deliver description lost the KB-capture reminder: %q", tc.Name, d.Description)
		}
	}
}

func assertKinds(t *testing.T, typ mtt.Type, want map[mtt.StatusName]mtt.StatusKind) {
	t.Helper()
	if len(typ.Statuses) != len(want) {
		t.Fatalf("%s statuses = %d, want %d", typ.Name, len(typ.Statuses), len(want))
	}
	for name, kind := range want {
		s, ok := typ.StatusByName(name)
		if !ok {
			t.Fatalf("%s status %q missing", typ.Name, name)
		}
		if s.Kind != kind {
			t.Fatalf("%s status %q kind = %q, want %q", typ.Name, name, s.Kind, kind)
		}
	}
}

func edge(t *testing.T, typ mtt.Type, from, to mtt.StatusName) mtt.Transition {
	t.Helper()
	tr, ok := typ.FindTransition(from, to)
	if !ok {
		t.Fatalf("%s edge %s -> %s missing", typ.Name, from, to)
	}
	return tr
}

func namedEdge(t *testing.T, typ mtt.Type, from, to mtt.StatusName, name string) mtt.Transition {
	t.Helper()
	tr := edge(t, typ, from, to)
	if tr.Name != name {
		t.Fatalf("%s edge %s->%s name = %q, want %q", typ.Name, from, to, tr.Name, name)
	}
	return tr
}

func assertRuns(t *testing.T, typ mtt.Type, tr mtt.Transition, want ...string) {
	t.Helper()
	if len(tr.Commands) != len(want) {
		t.Fatalf("%s edge %s->%s: %d commands, want %d", typ.Name, tr.From, tr.To, len(tr.Commands), len(want))
	}
	for i, w := range want {
		if tr.Commands[i].Run != w {
			t.Fatalf("%s edge %s->%s cmd[%d] = %q, want %q", typ.Name, tr.From, tr.To, i, tr.Commands[i].Run, w)
		}
	}
}

func assertCancels(t *testing.T, typ mtt.Type, froms []mtt.StatusName) {
	t.Helper()
	for _, from := range froms {
		e := namedEdge(t, typ, from, "cancelled", "cancel")
		if e.Current != mtt.CurrentClear {
			t.Fatalf("%s cancel from %s: current = %q, want clear", typ.Name, from, e.Current)
		}
		assertRuns(t, typ, e, cmdSwitchMain, cmdCancelGuard)
	}
}
