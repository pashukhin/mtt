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

// newTagCmd builds `mtt tag` with add/rm subcommands.
func newTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage a task's tags",
	}
	cmd.AddCommand(newTagAddCmd(), newTagRmCmd())
	return cmd
}

// idAndTags requires an id plus at least one tag.
func idAndTags(usage string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) < 2 {
			return errors.New(usage)
		}
		return nil
	}
}

// toTags normalizes CLI-supplied tags, rejecting an invalid one with a usage error
// (a bare/invalid string never leaks into core). Shared by `add --tag` and `tag`.
func toTags(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		tag, ok := mtt.NormalizeTag(r)
		if !ok {
			return nil, fmt.Errorf("invalid tag %q: want a letter/number token with . _ - (any script), optional leading #", r)
		}
		out = append(out, tag)
	}
	return out, nil
}

func newTagAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <id> <tag>...",
		Short: "Add one or more tags to a task",
		Args:  idAndTags("provide a task id and at least one tag (example: mtt tag add t1 backend urgent)"),
		RunE:  func(cmd *cobra.Command, args []string) error { return runTagEdit(cmd, args, true) },
	}
}

func newTagRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id> <tag>...",
		Short: "Remove one or more tags from a task",
		Args:  idAndTags("provide a task id and at least one tag (example: mtt tag rm t1 urgent)"),
		RunE:  func(cmd *cobra.Command, args []string) error { return runTagEdit(cmd, args, false) },
	}
}

// runTagEdit is the shared add/rm path: parse, mutate via core.TagEditor, render.
func runTagEdit(cmd *cobra.Command, args []string, add bool) error {
	root, err := projectRoot(cmd)
	if err != nil {
		return err
	}
	id := mtt.TaskID(args[0])
	tags, err := toTags(args[1:])
	if err != nil {
		return err
	}
	ed := core.NewTagEditor(yaml.NewTaskStore(root), time.Now)
	var task mtt.Task
	verb := "tagged"
	if add {
		task, err = ed.AddTags(id, tags)
	} else {
		task, err = ed.RemoveTags(id, tags)
		verb = "untagged"
	}
	if err != nil {
		return err
	}
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", verb, id, strings.Join(tags, ", "))
	return err
}
