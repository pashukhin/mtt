package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// selectorFilterFlags are the list predicates a set-operating command reuses to
// select tasks (the --filter source). Shared by rm and tag add/rm.
var selectorFilterFlags = []string{"status", "type", "kind", "parent", "priority", "tag", "ready"}

// addSelectorFilterFlags registers the filter-source flags on a set-operating
// command (rm, tag add/rm). list/ready keep their own wiring.
func addSelectorFilterFlags(cmd *cobra.Command) {
	cmd.Flags().StringArray("status", nil, "select by status (repeatable)")
	cmd.Flags().StringArray("type", nil, "select by type (repeatable)")
	cmd.Flags().StringArray("kind", nil, "select by status category: initial|active|terminal (repeatable)")
	cmd.Flags().StringArray("priority", nil, "select by priority: high|medium|low (repeatable)")
	cmd.Flags().StringArray("tag", nil, "select by tag (repeatable)")
	cmd.Flags().String("parent", "", "select direct children of this task id")
	cmd.Flags().Bool("ready", false, "select only ready tasks (no open blockers)")
}

// filterActive reports whether any selector filter flag was set (the --filter
// source marker). Flags are already parsed by the time this runs.
func filterActive(cmd *cobra.Command) bool {
	for _, name := range selectorFilterFlags {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			return true
		}
	}
	return false
}

// readSelectorFilter builds a core.ListFilter (and the ready flag) from the
// selector filter flags, validating kinds/priorities/tags at the boundary.
func readSelectorFilter(cmd *cobra.Command) (core.ListFilter, bool, error) {
	statuses, _ := cmd.Flags().GetStringArray("status")
	types, _ := cmd.Flags().GetStringArray("type")
	kinds, _ := cmd.Flags().GetStringArray("kind")
	priorities, _ := cmd.Flags().GetStringArray("priority")
	tags, _ := cmd.Flags().GetStringArray("tag")
	parent, _ := cmd.Flags().GetString("parent")
	ready, _ := cmd.Flags().GetBool("ready")
	kindVals, err := parseKinds(kinds)
	if err != nil {
		return core.ListFilter{}, false, err
	}
	prioVals, err := toPriorities(priorities)
	if err != nil {
		return core.ListFilter{}, false, err
	}
	tagVals, err := toTags(tags)
	if err != nil {
		return core.ListFilter{}, false, err
	}
	return core.ListFilter{
		Statuses: toStatusNames(statuses), Types: toTypeNames(types), Kinds: kindVals,
		Priorities: prioVals, Tags: tagVals, Parent: mtt.TaskID(parent),
	}, ready, nil
}

// selectTaskIDs resolves a task set from exactly one of three mutually exclusive
// sources — explicit positional ids (only if allowExplicitIDs) | stdin "-" | the
// --filter flags — returning deduplicated ids in first-occurrence order. It never
// consults the current-task pointer. A present-but-empty source yields an empty
// slice with no error (the caller does a no-op). >1 or 0 active sources is a usage
// error.
func selectTaskIDs(cmd *cobra.Command, positional []string, allowExplicitIDs bool) ([]mtt.TaskID, error) {
	explicit := stripDash(positional)
	stdinActive := hasDash(positional)
	filterAct := filterActive(cmd)
	explicitActive := allowExplicitIDs && len(explicit) > 0

	sources := 0
	for _, on := range []bool{explicitActive, stdinActive, filterAct} {
		if on {
			sources++
		}
	}
	if sources > 1 {
		return nil, errors.New("choose one source: explicit ids | - (stdin) | --filter flags")
	}
	if sources == 0 {
		return nil, errors.New("no tasks selected: give ids, '-' for stdin, or --filter flags")
	}

	switch {
	case explicitActive:
		return dedupIDs(toTaskIDs(explicit)), nil
	case stdinActive:
		ids, err := readIDsFromStdin(cmd.InOrStdin())
		if err != nil {
			return nil, err
		}
		return dedupIDs(ids), nil
	default: // filterAct
		root, err := projectRoot(cmd)
		if err != nil {
			return nil, err
		}
		cfg, _, err := yaml.Load(root)
		if err != nil {
			return nil, err
		}
		tasks, err := yaml.NewTaskStore(root).List()
		if err != nil {
			return nil, err
		}
		filter, ready, err := readSelectorFilter(cmd)
		if err != nil {
			return nil, err
		}
		if ready {
			tasks = core.Ready(tasks, cfg)
		}
		return dedupIDs(idsOf(core.Select(tasks, filter, cfg))), nil
	}
}

// stripDash returns the positionals with every "-" removed.
func stripDash(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a != "-" {
			out = append(out, a)
		}
	}
	return out
}

// hasDash reports whether "-" (the stdin marker) is among the positionals.
func hasDash(args []string) bool {
	for _, a := range args {
		if a == "-" {
			return true
		}
	}
	return false
}

// toTaskIDs converts strings to task ids (no validation — the mutation checks).
func toTaskIDs(ss []string) []mtt.TaskID {
	out := make([]mtt.TaskID, len(ss))
	for i, s := range ss {
		out[i] = mtt.TaskID(s)
	}
	return out
}

// dedupIDs removes duplicate ids, keeping first-occurrence order.
func dedupIDs(ids []mtt.TaskID) []mtt.TaskID {
	seen := make(map[mtt.TaskID]bool, len(ids))
	out := make([]mtt.TaskID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// readIDsFromStdin reads ids one per line (trimmed; empty lines skipped).
func readIDsFromStdin(r io.Reader) ([]mtt.TaskID, error) {
	var ids []mtt.TaskID
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		ids = append(ids, mtt.TaskID(line))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return ids, nil
}

// idsOf projects a task slice to its ids.
func idsOf(tasks []mtt.Task) []mtt.TaskID {
	out := make([]mtt.TaskID, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

// writeIDs prints ids one per line (the --ids / --dry-run output).
func writeIDs(w io.Writer, ids []mtt.TaskID) error {
	var b strings.Builder
	for _, id := range ids {
		b.WriteString(string(id))
		b.WriteByte('\n')
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}
