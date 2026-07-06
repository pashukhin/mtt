package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newTypesCmd builds `mtt types [type]`: show the configured types and their flow.
func newTypesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "types [type]",
		Short: "Show configured task types and their flow",
		Args:  cobra.MaximumNArgs(1),
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
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			out, err := formatTypes(cfg, settings.Prefixes, filter)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), out); err != nil {
				return err
			}
			return nil
		},
	}
}

// formatTypes renders the configured types as human-readable blocks. When filter
// is non-empty, only that type is shown (error if unknown).
func formatTypes(cfg mtt.Config, prefixes map[string]string, filter string) (string, error) {
	var b strings.Builder
	shown := 0
	for _, t := range cfg.Types {
		if filter != "" && string(t.Name) != filter {
			continue
		}
		shown++
		writeTypeBlock(&b, t, prefixes[string(t.Name)])
	}
	if filter != "" && shown == 0 {
		return "", fmt.Errorf("unknown type %q", filter)
	}
	return b.String(), nil
}

// writeTypeBlock appends one type's block to b.
func writeTypeBlock(b *strings.Builder, t mtt.Type, prefix string) {
	rel := "root"
	if len(t.Parents) > 0 {
		ps := make([]string, len(t.Parents))
		for i, p := range t.Parents {
			ps[i] = string(p)
		}
		rel = "parents: " + strings.Join(ps, ", ")
	}
	fmt.Fprintf(b, "%s  (prefix %s · %s", t.Name, prefix, rel)
	if t.Default {
		b.WriteString(" · default")
	}
	b.WriteString(")\n")
	if t.Description != "" {
		fmt.Fprintf(b, "  %s\n", t.Description)
	}
	if len(t.Statuses) > 0 {
		b.WriteString("  statuses:")
		for _, s := range t.Statuses {
			fmt.Fprintf(b, " %s[%s]", s.Name, s.Kind)
		}
		b.WriteString("\n")
	}
	b.WriteString("  transitions:\n")
	for _, tr := range t.Transitions {
		fmt.Fprintf(b, "    %s -> %s", tr.From, tr.To)
		if tr.Description != "" {
			fmt.Fprintf(b, "  # %s", tr.Description)
		}
		b.WriteString("\n")
		for _, c := range tr.Commands {
			if c.Timeout > 0 {
				fmt.Fprintf(b, "        $ %s  (timeout %s)\n", c.Run, c.Timeout)
			} else {
				fmt.Fprintf(b, "        $ %s\n", c.Run)
			}
		}
	}
	b.WriteString("\n")
}
