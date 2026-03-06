package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/paths"
)

func runCLICommand(t *testing.T, args []string, input string) string {
	t.Helper()
	var buf strings.Builder
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	reader := strings.NewReader(input)
	rootCmd.SetIn(reader)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := Execute(ctx)
	rootCmd.SetIn(os.Stdin)
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(nil)
	resetCommandFlags(rootCmd)
	require.NoError(t, err)
	return buf.String()
}

func setTestConfigDir(t *testing.T) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "withingy")
	paths.SetConfigDirOverride(dir)
	t.Cleanup(func() { paths.SetConfigDirOverride("") })
}

func getConfigDir(t *testing.T) string {
	t.Helper()
	dir, err := paths.ConfigDir()
	require.NoError(t, err)
	return dir
}

func resetCommandFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, child := range cmd.Commands() {
		resetCommandFlags(child)
	}
}

func intPtr(v int) *int { return &v }

func floatPtr(v float64) *float64 { return &v }

func int64Ptr(v int64) *int64 { return &v }
