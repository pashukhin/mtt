// Package scaffold writes agent-facing configuration (hooks + permissions) for
// supported harnesses. It is onboarding infrastructure: it writes files outside
// .mtt/ (via internal/fsutil), never through a TaskStore/KnowledgeStore, and
// carries no pkg/mtt domain. The Harness interface + Registry are the extension
// seam; only "claude" is registered (codex/gemini deferred until verified).
package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pashukhin/mtt/internal/fsutil"
)

// Action is what Merge/Run did to a harness's file.
type Action string

// The three merge outcomes (also the human/JSON labels).
const (
	Created   Action = "created"
	Merged    Action = "merged"
	Unchanged Action = "unchanged"
)

// Result is the per-harness scaffold outcome.
type Result struct {
	Harness string
	Path    string
	Action  Action
}

// Harness is a scaffold target (one agent tool's config). Merge is pure.
type Harness interface {
	Name() string
	RelPath() string
	Merge(existing []byte, exists bool) (content []byte, action Action, err error)
}

// Registry is the set of supported harnesses (the single extension point).
func Registry() []Harness { return []Harness{claudeHarness{}} }

// Run scaffolds each harness under root and returns per-harness results. It is
// write-as-you-go: a read/merge/write failure aborts the loop and returns the
// results collected so far alongside a wrapped error. Only os.ErrNotExist marks
// a file absent; any other read error propagates.
func Run(root string, harnesses []Harness) ([]Result, error) {
	var results []Result
	for _, h := range harnesses {
		path := filepath.Join(root, filepath.FromSlash(h.RelPath()))
		// #nosec G304 -- path is <root>/<harness RelPath> under a locally-resolved
		// project root (projectRoot/baseDir), not network/untrusted input.
		existing, err := os.ReadFile(path)
		exists := true
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exists, existing = false, nil
			} else {
				return results, fmt.Errorf("%s: read %s: %w", h.Name(), path, err)
			}
		}
		content, action, err := h.Merge(existing, exists)
		if err != nil {
			return results, fmt.Errorf("%s: %s: %w", h.Name(), path, err)
		}
		if action != Unchanged {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return results, fmt.Errorf("%s: mkdir %s: %w", h.Name(), filepath.Dir(path), err)
			}
			if err := fsutil.AtomicWrite(path, content, 0o644); err != nil {
				return results, fmt.Errorf("%s: write %s: %w", h.Name(), path, err)
			}
		}
		results = append(results, Result{Harness: h.Name(), Path: path, Action: action})
	}
	return results, nil
}
