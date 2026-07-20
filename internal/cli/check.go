package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
)

// newCheckCmd builds `mtt check`: a read-only repo-wide reference integrity sweep.
// Exit 7 when any dangling reference is found; unverified (url / no-KB) is not a
// failure.
func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify references across the repository (exit 7 on dangling)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			notes, err := yaml.NewKnowledgeStore(root).ListNotes()
			if err != nil {
				return err
			}
			findings := core.CheckRefs(tasks, notes, true) // the YAML KB is always wired
			dangling := 0
			for _, f := range findings {
				if f.Status == core.RefDangling {
					dangling++
				}
			}
			if jsonFlag(cmd) {
				out := make([]refCheckJSON, 0, len(findings))
				for _, f := range findings {
					out = append(out, toRefCheckJSON(f))
				}
				if err := writeJSON(cmd.OutOrStdout(), out); err != nil {
					return err
				}
			} else {
				var b strings.Builder
				for _, f := range findings {
					fmt.Fprintf(&b, "%s:%s → %s:%s   [%s]\n", f.CarrierKind, f.CarrierID, f.Ref.Kind, f.Ref.ID, f.Status)
				}
				fmt.Fprintf(&b, "%d dangling, %d unverified across %d entities\n", dangling, len(findings)-dangling, countCarriers(findings))
				if _, err := fmt.Fprint(cmd.OutOrStdout(), b.String()); err != nil {
					return err
				}
			}
			if dangling > 0 {
				return core.ErrDanglingRefs
			}
			return nil
		},
	}
}

// refCheckJSON is the `mtt check --json` shape: carrier + ref + status. NOTE the
// name is refCheckJSON, NOT checkJSON — json.go already has a checkJSON (the gate
// command-result view), so reusing it would collide.
type refCheckJSON struct {
	Carrier struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"carrier"`
	Ref    refJSON `json:"ref"`
	Status string  `json:"status"`
}

func toRefCheckJSON(f core.CheckFinding) refCheckJSON {
	var j refCheckJSON
	j.Carrier.Kind, j.Carrier.ID = string(f.CarrierKind), f.CarrierID
	j.Ref = refJSON{Kind: string(f.Ref.Kind), ID: f.Ref.ID, Label: f.Ref.Label, Status: string(f.Status)}
	j.Status = string(f.Status)
	return j
}

func countCarriers(fs []core.CheckFinding) int {
	seen := map[string]bool{}
	for _, f := range fs {
		seen[string(f.CarrierKind)+":"+f.CarrierID] = true
	}
	return len(seen)
}
