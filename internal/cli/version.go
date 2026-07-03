package cli

import "github.com/spf13/cobra"

// newVersionCmd prints the build version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the mtt version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Println(version)
			return nil
		},
	}
}
