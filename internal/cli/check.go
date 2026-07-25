package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newCheckCmd builds `mtt check`: a read-only repo-wide integrity sweep over
// dangling refs, dangling depends_on, and dependency cycles. Exit 7 on any hard
// finding (dangling-ref / dangling-dep / cycle); unverified (url / no-KB) is soft.
func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify repository integrity: dangling refs/deps + cycles (exit 7 on hard)",
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
			findings := core.CheckIntegrity(tasks, notes, true) // the YAML KB is always wired
			danglingRef, danglingDep, unverified, cycles := 0, 0, 0, 0
			if jsonFlag(cmd) {
				rows := make([]integrityJSON, 0, len(findings))
				for _, f := range findings {
					rows = append(rows, toIntegrityJSON(f))
				}
				if err := writeJSON(cmd.OutOrStdout(), rows); err != nil {
					return err
				}
			} else {
				var b strings.Builder
				for _, f := range findings {
					switch f.Kind {
					case core.IntegrityDanglingRef, core.IntegrityUnverifiedRef:
						fmt.Fprintf(&b, "%s:%s → %s:%s   [%s]\n", f.Ref.CarrierKind, f.Ref.CarrierID, f.Ref.Ref.Kind, f.Ref.Ref.ID, f.Kind)
					case core.IntegrityDanglingDep:
						fmt.Fprintf(&b, "task:%s → depends_on:%s   [%s]\n", f.Dep.Task, f.Dep.Missing, f.Kind)
					case core.IntegrityCycle:
						fmt.Fprintf(&b, "cycle: %s\n", joinIDs(f.Cycle, " -> "))
					}
				}
				for _, f := range findings {
					switch f.Kind {
					case core.IntegrityDanglingRef:
						danglingRef++
					case core.IntegrityDanglingDep:
						danglingDep++
					case core.IntegrityUnverifiedRef:
						unverified++
					case core.IntegrityCycle:
						cycles++
					}
				}
				fmt.Fprintf(&b, "%d dangling, %d unverified, %d cycle across %d entities\n",
					danglingRef+danglingDep, unverified, cycles, countIntegrityCarriers(findings))
				if _, err := fmt.Fprint(cmd.OutOrStdout(), b.String()); err != nil {
					return err
				}
			}
			if integrityHasHard(findings) {
				return core.ErrIntegrity
			}
			return nil
		},
	}
}

// carrierJSON keeps the exact lowercase keys the old refCheckJSON.Carrier used
// (json:"kind"/"id") — an inline struct{Kind,ID string} WITHOUT tags would
// marshal as "Kind"/"ID" and silently regress the check --json carrier shape.
type carrierJSON struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// integrityJSON is the heterogeneous, kind-tagged `mtt check --json` element. A
// consumer switches on `kind`: the ref arms carry carrier/ref/status, the
// dangling-dep arm task/missing, the cycle arm the id chain.
type integrityJSON struct {
	Kind    string       `json:"kind"`
	Carrier *carrierJSON `json:"carrier,omitempty"`
	Ref     *refJSON     `json:"ref,omitempty"`
	Status  string       `json:"status,omitempty"`
	Task    string       `json:"task,omitempty"`
	Missing string       `json:"missing,omitempty"`
	Cycle   []string     `json:"cycle,omitempty"`
}

// toIntegrityJSON fills the union arm matching f.Kind. The ref arms keep the
// byte-identical lowercase carrier/ref/status keys of the old refCheckJSON.
func toIntegrityJSON(f core.IntegrityFinding) integrityJSON {
	j := integrityJSON{Kind: string(f.Kind)}
	switch f.Kind {
	case core.IntegrityDanglingRef, core.IntegrityUnverifiedRef:
		r := f.Ref
		j.Carrier = &carrierJSON{Kind: string(r.CarrierKind), ID: r.CarrierID}
		rv := refJSON{Kind: string(r.Ref.Kind), ID: r.Ref.ID, Label: r.Ref.Label, Status: string(r.Status)}
		j.Ref = &rv
		j.Status = string(r.Status)
	case core.IntegrityDanglingDep:
		j.Task, j.Missing = string(f.Dep.Task), string(f.Dep.Missing)
	case core.IntegrityCycle:
		j.Cycle = idStrings(f.Cycle)
	}
	return j
}

// integrityHasHard reports whether any finding gates (Kind != unverified-ref).
func integrityHasHard(fs []core.IntegrityFinding) bool {
	for _, f := range fs {
		if f.Kind != core.IntegrityUnverifiedRef {
			return true
		}
	}
	return false
}

// countIntegrityCarriers counts the distinct carriers/subjects across all
// findings: a ref carrier (kind:id), a dep finding's task, and each id on any
// cycle chain — deduplicated into one set (the summary's "across N entities").
func countIntegrityCarriers(fs []core.IntegrityFinding) int {
	seen := map[string]bool{}
	for _, f := range fs {
		switch f.Kind {
		case core.IntegrityDanglingRef, core.IntegrityUnverifiedRef:
			seen[string(f.Ref.CarrierKind)+":"+f.Ref.CarrierID] = true
		case core.IntegrityDanglingDep:
			seen["task:"+string(f.Dep.Task)] = true
		case core.IntegrityCycle:
			for _, id := range f.Cycle {
				seen["task:"+string(id)] = true
			}
		}
	}
	return len(seen)
}

// joinIDs joins task ids with sep (DRY with idStrings, the house id→string map).
func joinIDs(ids []mtt.TaskID, sep string) string {
	return strings.Join(idStrings(ids), sep)
}
