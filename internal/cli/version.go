package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type buildInfo struct {
	version string
	commit  string
	date    string
}

var currentBuild = buildInfo{
	version: "dev",
	commit:  "none",
	date:    "unknown",
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = currentBuild.version
	rootCmd.SetVersionTemplate("withingy {{.Version}}\n")
}

// SetBuildInfo allows the main package to inject ldflag-provided metadata.
func SetBuildInfo(version, commit, date string) {
	if v := strings.TrimSpace(version); v != "" {
		currentBuild.version = v
	}
	if c := strings.TrimSpace(commit); c != "" {
		currentBuild.commit = c
	}
	if d := strings.TrimSpace(date); d != "" {
		currentBuild.date = d
	}
	rootCmd.Version = currentBuild.version
}

func userAgentString() string {
	version := strings.TrimSpace(currentBuild.version)
	if version == "" {
		version = "dev"
	}
	return fmt.Sprintf("withingy/%s", version)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show build version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "withingy %s\n", currentBuild.version)
		fmt.Fprintf(cmd.OutOrStdout(), "commit: %s\n", currentBuild.commit)
		fmt.Fprintf(cmd.OutOrStdout(), "built: %s\n", currentBuild.date)
		return nil
	},
}
