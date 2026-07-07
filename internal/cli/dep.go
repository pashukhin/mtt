package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newDepCmd builds `mtt dep` with add/rm/list subcommands.
func newDepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage blocking dependencies (depends_on)",
	}
	cmd.AddCommand(newDepAddCmd(), newDepRmCmd(), newDepListCmd())
	return cmd
}

func twoIDs(usage string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != 2 {
			return errors.New(usage)
		}
		return nil
	}
}

func oneID(usage string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New(usage)
		}
		return nil
	}
}

func newDepAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <id> <depends-on-id>",
		Short: "Add a blocking dependency",
		Args:  twoIDs("provide two task ids (example: mtt dep add t2 t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, dep := mtt.TaskID(args[0]), mtt.TaskID(args[1])
			task, err := core.NewDependencyEditor(yaml.NewTaskStore(root), time.Now).AddDependency(id, dep)
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "now %s depends on %s\n", id, dep)
			return err
		},
	}
}

func newDepRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id> <depends-on-id>",
		Short: "Remove a blocking dependency",
		Args:  twoIDs("provide two task ids (example: mtt dep rm t2 t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, dep := mtt.TaskID(args[0]), mtt.TaskID(args[1])
			task, err := core.NewDependencyEditor(yaml.NewTaskStore(root), time.Now).RemoveDependency(id, dep)
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s no longer depends on %s\n", id, dep)
			return err
		},
	}
}

func newDepListCmd() *cobra.Command {
	var (
		tree   bool
		cycles bool
	)
	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List a task's dependencies and dependents",
		Args:  oneID("provide exactly one task id (example: mtt dep list t2)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			g := core.NewDepGraph(tasks)
			id := mtt.TaskID(args[0])
			if _, ok := g.Get(id); !ok {
				return taskNotFound(id)
			}
			if cycles {
				return writeDepCycles(cmd, g)
			}
			if jsonFlag(cmd) {
				if tree {
					return writeJSON(cmd.OutOrStdout(), buildDepTreeJSON(g, id))
				}
				return writeJSON(cmd.OutOrStdout(), buildDepListJSON(g, id))
			}
			if tree {
				_, err = fmt.Fprint(cmd.OutOrStdout(), renderDepTree(g, id))
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderDepList(g, id))
			return err
		},
	}
	cmd.Flags().BoolVar(&tree, "tree", false, "show the transitive dependency tree")
	cmd.Flags().BoolVar(&cycles, "cycles", false, "report dependency cycles project-wide")
	return cmd
}

// renderDepTree renders id's transitive blockers as an ASCII tree, cycle-safe.
func renderDepTree(g core.DepGraph, id mtt.TaskID) string {
	root, ok := g.Get(id)
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", taskLine(root))
	var walk func(cur mtt.TaskID, prefix string, seen map[mtt.TaskID]bool)
	walk = func(cur mtt.TaskID, prefix string, seen map[mtt.TaskID]bool) {
		deps := g.DependsOn(cur)
		for i, dep := range deps {
			last := i == len(deps)-1
			branch, childPrefix := "├─ ", prefix+"│  "
			if last {
				branch, childPrefix = "└─ ", prefix+"   "
			}
			if t, ok := g.Get(dep); ok {
				fmt.Fprintf(&b, "%s%s%s\n", prefix, branch, taskLine(t))
			} else {
				fmt.Fprintf(&b, "%s%s%s  (missing)\n", prefix, branch, dep)
			}
			if seen[dep] {
				continue // cycle guard (hand-broken data)
			}
			seen[dep] = true
			walk(dep, childPrefix, seen)
		}
	}
	walk(id, "", map[mtt.TaskID]bool{id: true})
	return b.String()
}

// depTreeJSON is the nested machine-readable transitive tree.
type depTreeJSON struct {
	taskJSON
	Missing   bool          `json:"missing,omitempty"`
	DependsOn []depTreeJSON `json:"depends_on,omitempty"`
}

// buildDepTreeJSON builds the nested transitive tree, cycle-safe.
func buildDepTreeJSON(g core.DepGraph, id mtt.TaskID) depTreeJSON {
	var build func(cur mtt.TaskID, seen map[mtt.TaskID]bool) depTreeJSON
	build = func(cur mtt.TaskID, seen map[mtt.TaskID]bool) depTreeJSON {
		node := depTreeJSON{}
		t, ok := g.Get(cur)
		if !ok {
			node.taskJSON = taskJSON{ID: string(cur)}
			node.Missing = true
			return node
		}
		node.taskJSON = toTaskJSON(t)
		for _, dep := range g.DependsOn(cur) {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			node.DependsOn = append(node.DependsOn, build(dep, seen))
		}
		return node
	}
	return build(id, map[mtt.TaskID]bool{id: true})
}

// writeDepCycles reports the project's dependency cycles (or "no cycles").
func writeDepCycles(cmd *cobra.Command, g core.DepGraph) error {
	cycles := g.Cycles()
	if jsonFlag(cmd) {
		out := make([][]string, 0, len(cycles))
		for _, cyc := range cycles {
			chain := make([]string, len(cyc))
			for i, cid := range cyc {
				chain[i] = string(cid)
			}
			out = append(out, chain)
		}
		return writeJSON(cmd.OutOrStdout(), out)
	}
	if len(cycles) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "no cycles")
		return err
	}
	var b strings.Builder
	for _, cyc := range cycles {
		chain := make([]string, len(cyc))
		for i, cid := range cyc {
			chain[i] = string(cid)
		}
		fmt.Fprintf(&b, "cycle: %s\n", strings.Join(chain, " -> "))
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
	return err
}

// renderDepList renders a task's direct blockers ("depends on") and its computed
// dependents ("required by"). Dangling blockers are flagged (missing).
func renderDepList(g core.DepGraph, id mtt.TaskID) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s depends on:\n", id)
	deps := g.DependsOn(id)
	if len(deps) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, dep := range deps {
		if t, ok := g.Get(dep); ok {
			fmt.Fprintf(&b, "  %s\n", taskLine(t))
		} else {
			fmt.Fprintf(&b, "  %s  (missing)\n", dep)
		}
	}
	b.WriteString("required by:\n")
	dependents := g.Dependents(id)
	if len(dependents) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, t := range dependents {
		fmt.Fprintf(&b, "  %s\n", taskLine(t))
	}
	return b.String()
}

// depListJSON is the machine-readable view of `dep list <id>`.
type depListJSON struct {
	ID         string       `json:"id"`
	DependsOn  []depRefJSON `json:"depends_on"`
	RequiredBy []taskJSON   `json:"required_by"`
}

// depRefJSON is one blocker: the resolved task view, or an id flagged missing.
type depRefJSON struct {
	ID      string    `json:"id"`
	Missing bool      `json:"missing,omitempty"`
	Task    *taskJSON `json:"task,omitempty"`
}

// buildDepListJSON builds the flat dep-list view; slices are non-nil so an empty
// result marshals to [] (never null).
func buildDepListJSON(g core.DepGraph, id mtt.TaskID) depListJSON {
	out := depListJSON{ID: string(id), DependsOn: make([]depRefJSON, 0), RequiredBy: make([]taskJSON, 0)}
	for _, dep := range g.DependsOn(id) {
		entry := depRefJSON{ID: string(dep)}
		if t, ok := g.Get(dep); ok {
			v := toTaskJSON(t)
			entry.Task = &v
		} else {
			entry.Missing = true
		}
		out.DependsOn = append(out.DependsOn, entry)
	}
	for _, t := range g.Dependents(id) {
		out.RequiredBy = append(out.RequiredBy, toTaskJSON(t))
	}
	return out
}
