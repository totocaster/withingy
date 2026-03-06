package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/stats"
)

func TestStatsDailyTextShowsEmptyState(t *testing.T) {
	orig := statsDailyFn
	t.Cleanup(func() { statsDailyFn = orig })

	statsDailyFn = func(context.Context, time.Time) (*stats.DailyReport, error) {
		return &stats.DailyReport{
			Date:    "2026-03-07",
			Start:   time.Date(2026, 3, 7, 0, 0, 0, 0, time.Local),
			End:     time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local),
			Summary: stats.Summary{},
		}, nil
	}

	output := runCLICommand(t, []string{"stats", "daily", "--date", "2026-03-07", "--text"}, "")
	require.Contains(t, output, "No data for 2026-03-07.")
	require.NotContains(t, output, "Steps: 0")
}
