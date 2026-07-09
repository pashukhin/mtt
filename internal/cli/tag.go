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
		Long: `Manage a task's tags.

The primary way to tag is a #hashtag in the title or description: those are
extracted and merged into the task's tags on 'mtt add' and 'mtt edit' (the text is
left intact). 'mtt tag add/rm' is the secondary, pointed path — for tags not tied
to the text. Tags are a normalized, deduplicated, sorted set (Unicode, lowercased).`,
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
		Long: `Add one or more tags to a task (all in one call). Adding a tag that is already
present is a no-op. Values are normalized (lowercase; letters/digits and . _ -, any
script; an optional leading # is allowed). Tip: for tags that describe the work, put
a #hashtag in the title/description instead — 'mtt add'/'mtt edit' pick those up.`,
		Args: idAndTags("provide a task id and at least one tag (example: mtt tag add t1 backend urgent)"),
		RunE: func(cmd *cobra.Command, args []string) error { return runTagEdit(cmd, args, true) },
	}
}

func newTagRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id> <tag>...",
		Short: "Remove one or more tags from a task",
		Long: `Remove one or more tags from a task (all in one call).

A tag whose #hashtag is still present in the title or description is refused — edit
the text to remove it (the text is authoritative). Removing a tag that is absent is
a no-op.`,
		Args: idAndTags("provide a task id and at least one tag (example: mtt tag rm t1 urgent)"),
		RunE: func(cmd *cobra.Command, args []string) error { return runTagEdit(cmd, args, false) },
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
