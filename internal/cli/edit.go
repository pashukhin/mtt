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

// newEditCmd builds `mtt edit <id>`: edit a task's non-flow fields.
func newEditCmd() *cobra.Command {
	var title, desc, priority string
	var noRun bool
	cmd := &cobra.Command{
		Use:   "edit [<id>]",
		Short: "Edit a task's title, description, and/or priority (the current task when the id is omitted)",
		Long: `Edit a task's title, description, and/or priority (the current task when the id is
omitted). Status is not editable here — it moves through the flow ('mtt status').

Editing the title or description re-derives #hashtags: a tag whose #hashtag leaves
the text is dropped, a newly-typed one is added, and tags set via 'mtt tag add'
survive. There is no --tag here; use 'mtt tag add/rm' for tags not in the text.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var p core.EditParams
			if cmd.Flags().Changed("title") {
				p.Title = &title
			}
			if cmd.Flags().Changed("description") {
				p.Description = &desc
			}
			if cmd.Flags().Changed("priority") {
				pr, err := parsePriority(priority)
				if err != nil {
					return err
				}
				p.Priority = &pr
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			cfg, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			if p.Events, err = eventOptions(cmd, noRun, settings.Author); err != nil {
				return err
			}
			ev, closeOut, err := newEventEmitter(cmd, root, cfg, settings)
			if err != nil {
				return err
			}
			defer closeOut()
			editor := core.NewEditor(yaml.NewTaskStore(root), time.Now, ev)
			id, err := resolveTaskID(root, argOrEmpty(args))
			if err != nil {
				return err
			}
			task, err := editor.Edit(id, p)
			if err != nil && !errors.As(err, new(*core.PostActionError)) {
				if errors.Is(err, mtt.ErrNotFound) {
					return taskNotFound(id)
				}
				return err
			}
			evErr := err // nil, or the finalization failure — render the edit either way (it IS saved)
			if jsonFlag(cmd) {
				if werr := writeJSON(cmd.OutOrStdout(), toTaskJSON(task)); werr != nil {
					return werr
				}
			} else if _, werr := fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", task.ID); werr != nil {
				return werr
			}
			renderPostRecovery(cmd, evErr, mutationSavedLine)
			return evErr
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&desc, "description", "", "new description")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority: high|medium|low (empty string clears it)")
	addNoRunFlag(cmd, &noRun)
	return cmd
}
