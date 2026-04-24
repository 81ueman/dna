package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCommand builds the top-level dna command.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dna",
		Short: "Differential Network Analysis prototype",
		Long: "dna is a prototype implementation of Differential Network Analysis " +
			"for reporting forwarding-behavior changes caused by network changes.",
		SilenceUsage: true,
	}

	cmd.AddCommand(newDiffCommand())

	return cmd
}

func newDiffCommand() *cobra.Command {
	var opts diffOptions

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare old and new network snapshots",
		Long: "Compare old and new network snapshots and report differential " +
			"reachability facts. This scaffold only defines the command shape.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(opts)
		},
	}

	cmd.Flags().StringVar(&opts.topology, "topology", "", "path to a Containerlab topology file")
	cmd.Flags().StringVar(&opts.oldConfigs, "old-configs", "", "path to the old configuration snapshot directory")
	cmd.Flags().StringVar(&opts.newConfigs, "new-configs", "", "path to the new configuration snapshot directory")

	return cmd
}

type diffOptions struct {
	topology   string
	oldConfigs string
	newConfigs string
}

func runDiff(opts diffOptions) error {
	return fmt.Errorf("dna diff is not implemented yet")
}
