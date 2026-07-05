package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newTreeCmd builds `mtt tree [id]`: render the epic → task → subtask hierarchy.
func newTreeCmd() *cobra.Command {
	var (
		statuses []string
		kinds    []string
		depth    int
	)
	cmd := &cobra.Command{
		Use:   "tree [id]",
		Short: "Show the task hierarchy",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("provide at most one task id (example: mtt tree e1)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			kindVals, err := parseKinds(kinds)
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			cfg, _, err := yaml.Load(root)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			idx := core.NewIndex(tasks)
			var roots []mtt.Task
			if len(args) == 1 {
				t, ok := idx.Get(args[0])
				if !ok {
					return fmt.Errorf("task %q not found", args[0])
				}
				roots = []mtt.Task{t}
			} else {
				roots = idx.Roots()
			}
			f := core.ListFilter{Statuses: statuses, Kinds: kindVals}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderTree(idx, roots, f, cfg, depth))
			return err
		},
	}
	cmd.Flags().StringArrayVar(&statuses, "status", nil, "filter by status (repeatable)")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "filter by status category: initial|active|terminal (repeatable)")
	cmd.Flags().IntVar(&depth, "depth", 0, "limit visible levels (0 = unlimited)")
	return cmd
}

// parseKinds validates the --kind values against the closed StatusKind vocabulary.
func parseKinds(vals []string) ([]mtt.StatusKind, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	out := make([]mtt.StatusKind, 0, len(vals))
	for _, v := range vals {
		k := mtt.StatusKind(v)
		if !k.Valid() {
			return nil, fmt.Errorf("invalid --kind %q: want initial|active|terminal", v)
		}
		out = append(out, k)
	}
	return out, nil
}

// renderTree renders the forest rooted at roots as an ASCII tree. With a filter,
// keep-ancestors semantics apply: a node shows iff it matches or any descendant
// matches (non-matching ancestors remain as the path). maxDepth <= 0 is
// unlimited; maxDepth n shows n levels below (and including) each root.
func renderTree(x core.Index, roots []mtt.Task, f core.ListFilter, cfg mtt.Config, maxDepth int) string {
	keep := map[string]bool{}
	for _, r := range roots {
		markVisible(x, r.ID, f, cfg, keep, map[string]bool{})
	}
	var b strings.Builder
	var walk func(t mtt.Task, prefix string, isLast, root bool, level int, seen map[string]bool)
	walk = func(t mtt.Task, prefix string, isLast, root bool, level int, seen map[string]bool) {
		if !keep[t.ID] || seen[t.ID] {
			return
		}
		seen[t.ID] = true
		if root {
			fmt.Fprintf(&b, "%s\n", taskLine(t))
		} else {
			branch := "├─ "
			if isLast {
				branch = "└─ "
			}
			fmt.Fprintf(&b, "%s%s%s\n", prefix, branch, taskLine(t))
		}
		if maxDepth > 0 && level+1 > maxDepth {
			return
		}
		kids := visibleChildren(x, t.ID, keep)
		childPrefix := prefix
		if !root {
			if isLast {
				childPrefix += "   "
			} else {
				childPrefix += "│  "
			}
		}
		for i, c := range kids {
			walk(c, childPrefix, i == len(kids)-1, false, level+1, seen)
		}
	}
	for _, r := range roots {
		walk(r, "", true, true, 1, map[string]bool{})
	}
	return b.String()
}

// markVisible memoizes into keep whether id should appear: it matches the filter
// or some descendant does. seen guards against cycles in hand-broken data.
func markVisible(x core.Index, id string, f core.ListFilter, cfg mtt.Config, keep, seen map[string]bool) bool {
	if seen[id] {
		return keep[id]
	}
	seen[id] = true
	t, ok := x.Get(id)
	visible := ok && core.Match(t, f, cfg)
	for _, c := range x.Children(id) {
		if markVisible(x, c.ID, f, cfg, keep, seen) {
			visible = true
		}
	}
	keep[id] = visible
	return visible
}

// visibleChildren returns id's direct children that survive the keep set.
func visibleChildren(x core.Index, id string, keep map[string]bool) []mtt.Task {
	all := x.Children(id)
	out := make([]mtt.Task, 0, len(all))
	for _, c := range all {
		if keep[c.ID] {
			out = append(out, c)
		}
	}
	return out
}
