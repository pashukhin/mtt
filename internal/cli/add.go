package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newAddCmd builds `mtt add [title]`: create a task.
func newAddCmd() *cobra.Command {
	var (
		typeName  string
		parent    string
		noParent  bool
		desc      string
		priority  string
		dependsOn []string
		tags      []string
		refVals   []string
		noRun     bool
	)
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Create a task",
		Long: `Create a task. Provide a title (positional) and/or --description; at least one is
required.

#hashtags in the title or description are extracted into the task's tags, and --tag
adds explicit tags — both merged into one normalized, deduplicated, sorted set. Edit
the text later ('mtt edit') to change text-derived tags, or 'mtt tag add/rm' for
explicit ones.`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("too many arguments (got %d): wrap a multi-word title in quotes (mtt add \"fix login\"), and pass multiple --tag/--depends-on values comma-separated (--tag a,b) or by repeating the flag (--tag a --tag b) — not space-separated", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			cfg, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}
			title := ""
			if len(args) == 1 {
				title = args[0]
			}
			prio, err := parsePriority(priority)
			if err != nil {
				return err
			}
			tagVals, err := toTags(tags)
			if err != nil {
				return err
			}
			depIDs := make([]mtt.TaskID, len(dependsOn))
			for i, d := range dependsOn {
				depIDs[i] = mtt.TaskID(d)
			}
			refs, err := parseRefFlags(refVals)
			if err != nil {
				return err
			}
			evOpts, err := eventOptions(cmd, noRun, settings.Author)
			if err != nil {
				return err
			}
			ev, closeOut, err := newEventEmitter(cmd, root, cfg, settings)
			if err != nil {
				return err
			}
			defer closeOut()
			adder := core.NewAdder(yaml.NewTaskStore(root), cfg, time.Now, ev)
			task, err := adder.Add(core.AddParams{Title: title, TypeName: mtt.TypeName(typeName), Parent: mtt.TaskID(parent), NoParent: noParent, Description: desc, Priority: prio, DependsOn: depIDs, Tags: tagVals, Refs: refs, Events: evOpts})
			if err != nil && !errors.As(err, new(*core.PostActionError)) {
				return err
			}
			evErr := err // nil, or the finalization failure — render the task either way (it IS saved)
			for _, r := range refs {
				warnIfNotOK(cmd, r, verifyOne(root, r))
			}
			if jsonFlag(cmd) {
				if werr := writeJSON(cmd.OutOrStdout(), toTaskJSON(task)); werr != nil {
					return werr
				}
			} else if _, werr := fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", task.ID); werr != nil {
				return werr
			}
			renderPostRecovery(cmd, evErr, mutationSavedLine)
			return evErr
		},
	}
	cmd.Flags().StringVar(&typeName, "type", "", "task type (default: the config's default type)")
	cmd.Flags().StringVar(&parent, "parent", "", "place under an existing parent task (by id)")
	cmd.Flags().BoolVar(&noParent, "no-parent", false, "create a parent-requiring type at top level (conscious exception)")
	cmd.Flags().StringVar(&desc, "description", "", "task description")
	cmd.Flags().StringVar(&priority, "priority", "", "task priority: high|medium|low (default: unset)")
	cmd.Flags().StringSliceVar(&dependsOn, "depends-on", nil, "ids this task depends on (repeatable, comma-separated)")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "add a tag (repeatable, comma-separated; #hashtags in the title/description are also picked up)")
	cmd.Flags().StringArrayVar(&refVals, "ref", nil, "add a reference <kind>:<target> (repeatable)")
	addNoRunFlag(cmd, &noRun)
	cmd.MarkFlagsMutuallyExclusive("parent", "no-parent")
	return cmd
}
