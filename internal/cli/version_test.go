package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	orig := currentBuild
	SetBuildInfo("v1.2.3", "abcdef0", "2026-03-04T00:00:00Z")
	t.Cleanup(func() {
		currentBuild = orig
		rootCmd.Version = currentBuild.version
	})

	output := runCLICommand(t, []string{"version"}, "")
	require.Contains(t, output, "withingy v1.2.3")
	require.Contains(t, output, "commit: abcdef0")
	require.Contains(t, output, "built: 2026-03-04T00:00:00Z")

	output = runCLICommand(t, []string{"--version"}, "")
	require.Contains(t, output, "withingy v1.2.3")
}
