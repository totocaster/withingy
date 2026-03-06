package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/measures"
)

func TestWeightListText(t *testing.T) {
	orig := weightListFn
	t.Cleanup(func() { weightListFn = orig })

	weightListFn = func(_ context.Context, opts *api.ListOptions) (*measures.WeightListResult, error) {
		require.NotNil(t, opts)
		require.NotNil(t, opts.Start)
		require.NotNil(t, opts.End)
		return &measures.WeightListResult{
			Weights: []measures.WeightEntry{
				{
					GroupID:  99,
					TakenAt:  time.Date(2026, 3, 2, 6, 45, 0, 0, time.UTC),
					WeightKG: 69.8,
				},
			},
		}, nil
	}

	output := runCLICommand(t, []string{
		"weight", "list",
		"--start", "2026-03-01",
		"--end", "2026-03-03",
		"--text",
	}, "")
	require.Contains(t, output, "Taken At")
	require.Contains(t, output, "69.80 kg")
	require.Contains(t, output, "99")
}

func TestWeightLatestText(t *testing.T) {
	orig := weightLatestFn
	t.Cleanup(func() { weightLatestFn = orig })

	weightLatestFn = func(context.Context) (*measures.WeightEntry, error) {
		return &measures.WeightEntry{
			GroupID:  101,
			TakenAt:  time.Date(2026, 3, 3, 7, 0, 0, 0, time.UTC),
			WeightKG: 68.9,
		}, nil
	}

	output := runCLICommand(t, []string{"weight", "latest", "--text"}, "")
	require.Contains(t, output, "Taken At:")
	require.Contains(t, output, "68.90 kg")
	require.Contains(t, output, "Group ID: 101")
}

func TestWeightTodayText(t *testing.T) {
	orig := weightListFn
	t.Cleanup(func() { weightListFn = orig })

	weightListFn = func(_ context.Context, opts *api.ListOptions) (*measures.WeightListResult, error) {
		require.NotNil(t, opts)
		require.NotNil(t, opts.Start)
		require.NotNil(t, opts.End)
		return &measures.WeightListResult{
			Weights: []measures.WeightEntry{
				{
					GroupID:  77,
					TakenAt:  time.Date(2026, 3, 7, 7, 5, 0, 0, time.UTC),
					WeightKG: 70.1,
				},
			},
		}, nil
	}

	output := runCLICommand(t, []string{"weight", "today", "--text"}, "")
	require.Contains(t, output, "Taken At")
	require.Contains(t, output, "70.10 kg")
	require.Contains(t, output, "77")
}
