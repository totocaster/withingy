package cli

import (
	"context"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "withingy",
	Short:         "Unofficial Withings data CLI written in Go",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute runs the root command with the provided context.
func Execute(ctx context.Context) error {
	rootCmd.SetContext(ctx)
	return rootCmd.ExecuteContext(ctx)
}
