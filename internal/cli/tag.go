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
to the text. Tags are a normalized, deduplicated, sorted set (Unicode, lowercased).

Bulk: with a "-" (stdin) or a --filter flag, the positionals are TAGS and the tasks
come from the selector (e.g. 'mtt tag add urgent --status tbd', 'mtt tag rm x -').`,
	}
	cmd.AddCommand(newTagAddCmd(), newTagRmCmd())
	return cmd
}

// tagArgs validates positionals for tag add/rm. With a selector marker (a "-" or a
// filter flag) it is bulk — require ≥1 positional (the tag; the ≥1-after-strip
// recheck is in runTagEdit). Without a marker it is the single form — require ≥2
// (id + ≥1 tag).
func tagArgs(usage string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if hasDash(args) || filterActive(cmd) {
			if len(stripDash(args)) < 1 {
				return errors.New("provide at least one tag")
			}
			return nil
		}
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
	c := &cobra.Command{
		Use:   "add <id> <tag>... | <tag>... (- | --filter)",
		Short: "Add one or more tags to a task (or a selected set)",
		Long: `Add one or more tags. Single form: 'mtt tag add <id> <tag>...' (adding a tag that
is already present is a no-op). Bulk form: with a "-" (ids from stdin) or a --filter
flag, the positionals are the tags applied to every selected task. Values are
normalized (lowercase; letters/digits and . _ -, any script; an optional leading #).
Tip: for tags that describe the work, put a #hashtag in the title/description.`,
		Args: tagArgs("provide a task id and at least one tag (example: mtt tag add t1 backend urgent)"),
		RunE: func(cmd *cobra.Command, args []string) error { return runTagEdit(cmd, args, true) },
	}
	addSelectorFilterFlags(c)
	c.Flags().Bool("dry-run", false, "preview the affected tasks without changing them")
	c.Flags().Bool("no-run", false, "skip the configured lifecycle-event pipeline (forces --who and --why)")
	return c
}

func newTagRmCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "rm <id> <tag>... | <tag>... (- | --filter)",
		Short: "Remove one or more tags from a task (or a selected set)",
		Long: `Remove one or more tags. Single form: 'mtt tag rm <id> <tag>...'. Bulk form: with a
"-" (ids from stdin) or a --filter flag, the positionals are the tags removed from
every selected task.

A tag whose #hashtag is still present in the title or description is refused — edit
the text to remove it (the text is authoritative). Removing a tag that is absent is
a no-op.`,
		Args: tagArgs("provide a task id and at least one tag (example: mtt tag rm t1 urgent)"),
		RunE: func(cmd *cobra.Command, args []string) error { return runTagEdit(cmd, args, false) },
	}
	addSelectorFilterFlags(c)
	c.Flags().Bool("dry-run", false, "preview the affected tasks without changing them")
	c.Flags().Bool("no-run", false, "skip the configured lifecycle-event pipeline (forces --who and --why)")
	return c
}

// runTagEdit is the shared add/rm path. A selector marker (a "-" or a filter flag)
// switches to bulk (positionals = tags; tasks from the selector); otherwise it is
// the single form (args[0]=id, args[1:]=tags).
func runTagEdit(cmd *cobra.Command, args []string, add bool) error {
	root, err := projectRoot(cmd)
	if err != nil {
		return err
	}
	cfg, settings, err := yaml.Load(root)
	if err != nil {
		return err
	}
	noRun, _ := cmd.Flags().GetBool("no-run")
	opts, err := eventOptions(cmd, noRun, settings.Author)
	if err != nil {
		return err
	}
	ev, closeOut, err := newEventEmitter(cmd, root, cfg, settings)
	if err != nil {
		return err
	}
	defer closeOut()
	ed := core.NewTagEditor(yaml.NewTaskStore(root), time.Now, ev)

	if !hasDash(args) && !filterActive(cmd) {
		// single (back-compat)
		id := mtt.TaskID(args[0])
		tags, err := toTags(args[1:])
		if err != nil {
			return err
		}
		if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
			return previewBulk(cmd, []mtt.TaskID{id})
		}
		return applyTagSingle(cmd, ed, id, tags, add, opts)
	}

	// bulk
	tags, err := toTags(stripDash(args))
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		return errors.New("provide at least one tag")
	}
	ids, err := selectTaskIDs(cmd, args, false)
	if err != nil {
		return err
	}
	verb := "tagged"
	apply := func(id mtt.TaskID) error { _, _, e := ed.AddTags(id, tags, opts); return e }
	if !add {
		verb = "untagged"
		apply = func(id mtt.TaskID) error { _, _, e := ed.RemoveTags(id, tags, opts); return e }
	}
	return runBulk(cmd, ids, verb, apply)
}

// applyTagSingle is the single-task add/rm. It reports only the tags that
// actually changed, so a no-op (adding a present tag / removing an absent one)
// reads honestly instead of a false "tagged/untagged" (c14).
func applyTagSingle(cmd *cobra.Command, ed *core.TagEditor, id mtt.TaskID, tags []string, add bool, opts core.EventOptions) error {
	var task mtt.Task
	var changed []string
	var err error
	verb := "tagged"
	if add {
		task, changed, err = ed.AddTags(id, tags, opts)
	} else {
		task, changed, err = ed.RemoveTags(id, tags, opts)
		verb = "untagged"
	}
	return finishMutation(cmd, err, func() error {
		if jsonFlag(cmd) {
			return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
		}
		if len(changed) == 0 {
			noop := "no such tag"
			if add {
				noop = "already tagged"
			}
			_, werr := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s (no change)\n", id, noop, strings.Join(tags, ", "))
			return werr
		}
		_, werr := fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", verb, id, strings.Join(changed, ", "))
		return werr
	})
}
