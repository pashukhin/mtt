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

// newDoCmd builds `mtt do [<id>] <edge>`: move a task along the NAMED edge leaving
// its current status — the explicit form of the `mtt <edge> [<id>]` sugar,
// symmetric to `mtt status` for target-status moves. Edge-name only (no status
// fallback); it rides the shared runTransition (gate/attribution/--json inherited).
func newDoCmd() *cobra.Command {
	var noRun bool
	cmd := &cobra.Command{
		Use:   "do [<id>] <edge>",
		Short: "Move a task along a named flow edge out of its current status",
		Long: `Move a task along the NAMED edge leaving its current status — the explicit form
of the 'mtt <edge> [<id>]' shorthand (symmetric to 'mtt status' for moves by target
status). The id is optional (resolves to the current task). Edge names are defined
per flow in the config; 'mtt types' lists them.`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 && len(args) != 2 {
				return errors.New("provide an edge name (and optionally a task id): mtt do [<id>] <edge>")
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
			explicit, edgeName := "", args[0]
			if len(args) == 2 {
				explicit, edgeName = args[0], args[1]
			}
			id, err := resolveTaskID(root, explicit)
			if err != nil {
				return err
			}
			task, err := yaml.NewTaskStore(root).Get(id)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return taskNotFound(id)
				}
				return err
			}
			typ, ok := cfg.TypeByName(task.Type)
			if !ok {
				return fmt.Errorf("unknown type %q for task %q", task.Type, id)
			}
			edge, ok := typ.FindTransitionByName(task.Status, edgeName)
			if !ok {
				return doMissError(typ, edgeName, task.Status)
			}
			return runTransition(cmd, root, cfg, settings, id, edge.To, noRun)
		},
	}
	cmd.Flags().BoolVar(&noRun, "no-run", false, "skip the edge's commands (bypass the gate)")
	return cmd
}

// doMissError reports that `edge` is not a named action out of `from`, wrapping
// core.ErrInvalidTransition (exit 6) and listing the available actions.
func doMissError(typ mtt.Type, edge string, from mtt.StatusName) error {
	return fmt.Errorf("%w: no action %q from status %q%s", core.ErrInvalidTransition, edge, from, availableActions(typ, from))
}

// availableActions renders the named edges leaving `from` for a `mtt do` miss.
func availableActions(typ mtt.Type, from mtt.StatusName) string {
	var names []string
	for _, e := range typ.TransitionsFrom(from) {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}
	if len(names) == 0 {
		return "; no named actions from this status"
	}
	return "; available: " + strings.Join(names, ", ")
}
