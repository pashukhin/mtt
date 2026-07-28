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

When the project opts into #hashtag extraction (extract_hashtags: true — OFF by
default), editing the title or description re-derives text tags: one whose #hashtag
leaves the text is dropped and a newly-typed one is added, while 'mtt tag add' tags
survive. With extraction off, a text edit changes no tags. There is no --tag here;
use 'mtt tag add/rm'.`,
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
			p.ExtractHashtags = settings.ExtractHashtags
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
			return finishMutation(cmd, err, func() error {
				if jsonFlag(cmd) {
					return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
				}
				_, werr := fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", task.ID)
				return werr
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&desc, "description", "", "new description")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority: high|medium|low (empty string clears it)")
	addNoRunFlag(cmd, &noRun)
	return cmd
}
