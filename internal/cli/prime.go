package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newPrimeCmd builds `mtt prime`: a curated, opt-in, pointer-only KB digest for
// session-start injection. A pure read; no mutation.
func newPrimeCmd() *cobra.Command {
	var minPriority string
	var limit int
	cmd := &cobra.Command{
		Use:   "prime",
		Short: "Print a curated digest of the important KB notes (for session start)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			minPrio := mtt.Priority(minPriority)
			if minPrio == "" || !minPrio.Valid() {
				return fmt.Errorf("invalid --min-priority %q: want high|medium|low", minPriority)
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			notes, err := yaml.NewKnowledgeStore(root).ListNotes()
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			entries, total := core.Prime(notes, core.NewBacklinks(tasks, notes), core.PrimeOptions{MinPriority: minPrio, Limit: limit})
			if jsonFlag(cmd) {
				out := make([]primeJSON, 0, len(entries))
				for _, e := range entries {
					out = append(out, toPrimeJSON(e))
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}
			return writePrime(cmd.OutOrStdout(), entries, total)
		},
	}
	cmd.Flags().StringVar(&minPriority, "min-priority", "high", "include notes at or above this priority: high|medium|low")
	cmd.Flags().IntVar(&limit, "limit", 20, "cap the digest to N notes (<=0 = no cap)")
	return cmd
}

// primeJSON is the machine view of one digest entry (tags forced non-null — the
// toNoteJSON house rule). backlinks is the incoming-reference count.
type primeJSON struct {
	Slug      string   `json:"slug"`
	Title     string   `json:"title,omitempty"`
	Tags      []string `json:"tags"`
	Priority  string   `json:"priority"`
	Backlinks int      `json:"backlinks"`
}

func toPrimeJSON(e core.PrimeEntry) primeJSON {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	return primeJSON{Slug: string(e.Slug), Title: e.Title, Tags: tags, Priority: string(e.Priority), Backlinks: e.Backlinks}
}

// writePrime renders the markdown digest (D4): a header, one pointer line per entry,
// and the "N of M" footer. An empty digest prints a single actionable line.
func writePrime(w io.Writer, entries []core.PrimeEntry, total int) error {
	var b strings.Builder
	if len(entries) == 0 {
		fmt.Fprintln(&b, "# Knowledge base — no important notes (mark one: mtt note edit <slug> --priority high)")
		_, err := fmt.Fprint(w, b.String())
		return err
	}
	b.WriteString("# Knowledge base — important notes\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "- **%s**  [%s]", e.Slug, e.Priority)
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "  (%s)", strings.Join(e.Tags, ", "))
		}
		if e.Title != "" {
			fmt.Fprintf(&b, "  — %s", e.Title)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "(%d of %d important notes shown — `mtt note show <slug>` for detail)\n", len(entries), total)
	_, err := fmt.Fprint(w, b.String())
	return err
}
