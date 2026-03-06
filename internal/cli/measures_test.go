package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/measures"
)

func TestMeasuresListParsesFlagsAndFormatsText(t *testing.T) {
	orig := measuresListFn
	t.Cleanup(func() { measuresListFn = orig })

	measuresListFn = func(_ context.Context, query *measures.Query) (*measures.ListResult, error) {
		require.NotNil(t, query)
		require.Len(t, query.Types, 2)
		require.Equal(t, []int{measures.TypeWeight, measures.TypeFatRatio}, query.Types)
		require.NotNil(t, query.Category)
		require.Equal(t, measures.CategoryReal, *query.Category)
		require.NotNil(t, query.LastUpdate)
		require.Equal(t, int64(1700000000), *query.LastUpdate)
		require.NotNil(t, query.Range)
		require.NotNil(t, query.Range.Start)
		require.NotNil(t, query.Range.End)

		return &measures.ListResult{
			Groups: []measures.Group{
				{
					ID:           42,
					TakenAt:      time.Date(2026, 3, 1, 7, 30, 0, 0, time.UTC),
					Category:     measures.CategoryReal,
					CategoryName: "real",
					Measures: []measures.Measure{
						{Code: "weight", Value: 70.25, Unit: "kg"},
						{Code: "fat-ratio", Value: 18.2, Unit: "%"},
					},
				},
			},
		}, nil
	}

	output := runCLICommand(t, []string{
		"measures", "list",
		"--start", "2026-03-01",
		"--end", "2026-03-02",
		"--types", "weight,fat-ratio",
		"--category", "1",
		"--last-update", "1700000000",
		"--text",
	}, "")
	require.Contains(t, output, "Taken At")
	require.Contains(t, output, "weight=70.25 kg")
	require.Contains(t, output, "fat-ratio=18.2 %")
	require.Contains(t, output, "42")
}
