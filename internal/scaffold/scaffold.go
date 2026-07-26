// Package scaffold writes agent-facing artifacts — config/hooks (t52) and docs
// (t46) — into a project. It is onboarding infrastructure: it writes files
// outside .mtt/ (via internal/fsutil), never through a TaskStore/KnowledgeStore,
// and carries no pkg/mtt domain. The Target interface is the extension seam, with
// two producers — HookTargets() (agent-tool config; only "claude", codex/gemini
// deferred until verified) and DocTargets() (the AGENTS.md runbook + CLAUDE.md/
// GEMINI.md pointers) — both consumed by Run.
package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pashukhin/mtt/internal/fsutil"
)

// Action is what Merge/Run did to a target's file.
type Action string

// The three merge outcomes (also the human/JSON labels).
const (
	Created   Action = "created"
	Merged    Action = "merged"
	Unchanged Action = "unchanged"
)

// Result is one target's scaffold outcome.
type Result struct {
	Name   string
	Path   string
	Action Action
}

// Target is a scaffold target (a file + a pure merge from current to desired
// bytes + a name for reporting). Merge is pure.
type Target interface {
	Name() string
	RelPath() string
	Merge(existing []byte, exists bool) (content []byte, action Action, err error)
}

// HookTargets is the set of agent-tool config targets (t52; the hooks facet).
func HookTargets() []Target { return []Target{claudeTarget{}} }

// Run scaffolds each target under root and returns per-target results. It is
// write-as-you-go: a read/merge/write failure aborts the loop and returns the
// results collected so far alongside a wrapped error. Only os.ErrNotExist marks
// a file absent; any other read error propagates.
func Run(root string, targets []Target) ([]Result, error) {
	var results []Result
	for _, t := range targets {
		path := filepath.Join(root, filepath.FromSlash(t.RelPath()))
		// #nosec G304 -- path is <root>/<target RelPath> under a locally-resolved
		// project root (projectRoot/baseDir), not network/untrusted input.
		existing, err := os.ReadFile(path)
		exists := true
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exists, existing = false, nil
			} else {
				return results, fmt.Errorf("%s: read %s: %w", t.Name(), path, err)
			}
		}
		content, action, err := t.Merge(existing, exists)
		if err != nil {
			return results, fmt.Errorf("%s: %s: %w", t.Name(), path, err)
		}
		if action != Unchanged {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return results, fmt.Errorf("%s: mkdir %s: %w", t.Name(), filepath.Dir(path), err)
			}
			if err := fsutil.AtomicWrite(path, content, 0o644); err != nil {
				return results, fmt.Errorf("%s: write %s: %w", t.Name(), path, err)
			}
		}
		results = append(results, Result{Name: t.Name(), Path: path, Action: action})
	}
	return results, nil
}
