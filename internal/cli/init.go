package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
)

// newInitCmd builds `mtt init`: write the starter .mtt/config.yaml.
func newInitCmd() *cobra.Command {
	var (
		tmpl  string
		force bool
		name  string
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a project (.mtt/config.yaml)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			base, err := baseDir(cmd)
			if err != nil {
				return err
			}
			projectName := name
			if projectName == "" {
				projectName = filepath.Base(base)
			}
			if err := yaml.Init(base, tmpl, projectName, force); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tmpl, "template", "default", "starter template: default|coding")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config")
	cmd.Flags().StringVar(&name, "name", "", "project name (default: current directory name)")
	return cmd
}
