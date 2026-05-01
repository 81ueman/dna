package cli

import (
	"fmt"

	"github.com/81ueman/dna/internal/config"
	"github.com/81ueman/dna/internal/forwarding"
	"github.com/81ueman/dna/internal/model"
	"github.com/81ueman/dna/internal/reachability"
	"github.com/81ueman/dna/internal/topology"
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
			"reachability facts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.topology, "topology", "", "path to a Containerlab topology file")
	cmd.Flags().StringVar(&opts.oldConfigs, "old-configs", "", "path to the old configuration snapshot directory")
	cmd.Flags().StringVar(&opts.newConfigs, "new-configs", "", "path to the new configuration snapshot directory")
	cmd.Flags().StringVar(&opts.parserBackend, "parser-backend", "normalized", "configuration parser backend: normalized or batfish")

	return cmd
}

type diffOptions struct {
	topology      string
	oldConfigs    string
	newConfigs    string
	parserBackend string
}

func runDiff(cmd *cobra.Command, opts diffOptions) error {
	if err := validateDiffOptions(opts); err != nil {
		return err
	}
	if opts.parserBackend == "batfish" {
		return fmt.Errorf("parser backend %q is not implemented yet", opts.parserBackend)
	}

	topo, err := topology.LoadContainerlab(opts.topology, topology.LoadOptions{})
	if err != nil {
		return fmt.Errorf("load topology: %w", err)
	}

	oldReaches, err := computeSnapshotReachability(opts.oldConfigs, topo)
	if err != nil {
		return fmt.Errorf("compute old reachability: %w", err)
	}
	newReaches, err := computeSnapshotReachability(opts.newConfigs, topo)
	if err != nil {
		return fmt.Errorf("compute new reachability: %w", err)
	}

	changes := reachability.Diff(oldReaches, newReaches)
	out := cmd.OutOrStdout()
	if len(changes) == 0 {
		_, err = fmt.Fprintln(out, "No reachability changes.")
		return err
	}
	for _, change := range changes {
		if _, err := fmt.Fprintln(out, reachability.FormatChange(change)); err != nil {
			return err
		}
	}
	return nil
}

func validateDiffOptions(opts diffOptions) error {
	if opts.topology == "" {
		return fmt.Errorf("--topology is required")
	}
	if opts.oldConfigs == "" {
		return fmt.Errorf("--old-configs is required")
	}
	if opts.newConfigs == "" {
		return fmt.Errorf("--new-configs is required")
	}
	switch opts.parserBackend {
	case "normalized", "batfish":
		return nil
	default:
		return fmt.Errorf("unsupported parser backend %q", opts.parserBackend)
	}
}

func computeSnapshotReachability(path string, topo topology.Topology) ([]model.Reach, error) {
	snapshot, err := config.LoadSnapshotDir(path, topo)
	if err != nil {
		return nil, fmt.Errorf("load configs: %w", err)
	}
	rules := forwarding.Rules(snapshot.StaticRoutes, snapshot.ConnectedRoutes)
	reaches, err := reachability.Compute(topo, rules)
	if err != nil {
		return nil, err
	}
	return reaches, nil
}
