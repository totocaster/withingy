package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/activity"
	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/measures"
	"github.com/toto/withingy/internal/sleep"
	"github.com/toto/withingy/internal/workouts"
)

func TestHPXExportEmitsHypercontextRecords(t *testing.T) {
	origMeasures := measuresListFn
	origSleep := sleepListFn
	origWorkouts := workoutsListFn
	origActivity := activityListFn
	origNow := hpxNow
	t.Cleanup(func() {
		measuresListFn = origMeasures
		sleepListFn = origSleep
		workoutsListFn = origWorkouts
		activityListFn = origActivity
		hpxNow = origNow
	})

	hpxNow = func() time.Time {
		return time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	}

	measuresListFn = func(_ context.Context, query *measures.Query) (*measures.ListResult, error) {
		require.NotNil(t, query)
		require.NotNil(t, query.Range)
		require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local).UTC(), query.Range.Start.UTC())
		require.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.Local).UTC(), query.Range.End.UTC())
		return &measures.ListResult{
			Groups: []measures.Group{
				{
					ID:           123,
					TakenAt:      time.Date(2026, 3, 2, 7, 15, 0, 0, time.UTC),
					CategoryName: "real",
					DeviceID:     "scale-1",
					Measures: []measures.Measure{
						{Type: measures.TypeWeight, Code: "weight", Name: "Weight", Value: 70.4, Unit: "kg"},
						{Type: measures.TypeFatRatio, Code: "fat-ratio", Name: "Fat Ratio", Value: 18.5, Unit: "%"},
						{Type: measures.TypeMuscleMass, Code: "muscle-mass", Name: "Muscle Mass", Value: 32.1, Unit: "kg"},
					},
				},
				{
					ID:           124,
					TakenAt:      time.Date(2026, 3, 3, 7, 15, 0, 0, time.UTC),
					CategoryName: "real",
					Measures: []measures.Measure{
						{Type: measures.TypeTemperature, Code: "temperature", Name: "Temperature", Value: 36.6, Unit: "C"},
					},
				},
			},
		}, nil
	}

	sleepListFn = func(_ context.Context, opts *api.ListOptions) (*sleep.ListResult, error) {
		require.NotNil(t, opts)
		require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local).UTC(), opts.Start.UTC())
		require.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.Local).UTC(), opts.End.UTC())
		return &sleep.ListResult{
			Sleeps: []sleep.Session{
				{
					Date:     "2026-03-02",
					Start:    time.Date(2026, 3, 1, 22, 30, 0, 0, time.FixedZone("JST", 9*60*60)),
					End:      time.Date(2026, 3, 2, 6, 45, 0, 0, time.FixedZone("JST", 9*60*60)),
					Timezone: "Asia/Tokyo",
					Data: sleep.Score{
						TimeInBedSeconds:      intPtr(27900),
						LightSleepSeconds:     intPtr(14400),
						DeepSleepSeconds:      intPtr(5400),
						REMSleepSeconds:       intPtr(8100),
						WakeUpDurationSeconds: intPtr(1800),
						SleepScore:            floatPtr(82),
						AverageHeartRate:      floatPtr(52),
						AverageRespRate:       floatPtr(13.2),
					},
				},
			},
		}, nil
	}

	workoutsListFn = func(_ context.Context, opts *api.ListOptions) (*workouts.ListResult, error) {
		require.NotNil(t, opts)
		require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local).UTC(), opts.Start.UTC())
		require.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.Local).UTC(), opts.End.UTC())
		return &workouts.ListResult{
			Workouts: []workouts.Workout{
				{
					ID:       "1710000000",
					Start:    time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC),
					End:      time.Date(2026, 3, 2, 10, 15, 0, 0, time.UTC),
					Date:     "2026-03-02",
					Timezone: "Asia/Tokyo",
					Category: intPtr(16),
					Calories: floatPtr(640),
					Distance: floatPtr(12400),
					Steps:    intPtr(15000),
				},
			},
		}, nil
	}

	activityListFn = func(_ context.Context, opts *api.ListOptions) (*activity.ListResult, error) {
		require.NotNil(t, opts)
		require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local).UTC(), opts.Start.UTC())
		require.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.Local).UTC(), opts.End.UTC())
		return &activity.ListResult{
			Activities: []activity.Day{
				{
					Date:          "2026-03-02",
					Timezone:      "Asia/Tokyo",
					Steps:         12345,
					Distance:      9876,
					Calories:      2200,
					TotalCalories: 2500,
					Active:        3600,
				},
			},
		}, nil
	}

	output := runCLICommand(t, []string{"--hpx", "--since", "2026-03-01", "--until", "2026-03-03"}, "")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 17)

	records := decodeHPXRecords(t, lines)
	require.Equal(t, "signpost", records[0]["table"])

	weightRecord := findHPXRecord(t, records, "metric", "body.weight_kg")
	require.Equal(t, hpxSource, weightRecord["source"])
	require.Equal(t, "2026-03-02", weightRecord["date"])

	weightMeta := weightRecord["meta"].(map[string]any)
	require.Equal(t, "scale-1", weightMeta["device_id"])
	require.Len(t, weightMeta["extra_measures"], 1)

	sleepDuration := findHPXRecord(t, records, "metric", "sleep.duration_ms")
	require.Equal(t, "2026-03-02", sleepDuration["date"])
	require.NotEmpty(t, sleepDuration["signpost_id"])

	sleepMeta := sleepDuration["meta"].(map[string]any)
	require.EqualValues(t, 82, sleepMeta["sleep_score"])

	workoutRecord := findHPXRecord(t, records, "metric", "workout.duration_ms")
	require.Equal(t, "2026-03-02", workoutRecord["date"])

	activityDoc := findHPXSummaryDoc(t, records, "withings-activity-2026-03-02")
	activityMeta := activityDoc["meta"].(map[string]any)
	require.EqualValues(t, 12345, activityMeta["steps"])

	measureSummary := findHPXSummaryDoc(t, records, "withings-measure-124-summary")
	require.Equal(t, "summary", measureSummary["kind"])
}

func TestParseHPXOptionsLastWindowAndLimit(t *testing.T) {
	origNow := hpxNow
	t.Cleanup(func() { hpxNow = origNow })

	hpxNow = func() time.Time {
		return time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	}

	cmd := &cobra.Command{Use: "withingy"}
	cmd.Flags().String(hpxFlagSince, "", "")
	cmd.Flags().String(hpxFlagUntil, "", "")
	cmd.Flags().String(hpxFlagLast, "", "")
	cmd.Flags().Int(hpxFlagLimit, 0, "")
	require.NoError(t, cmd.Flags().Set(hpxFlagLast, "10d"))
	require.NoError(t, cmd.Flags().Set(hpxFlagLimit, "5"))

	opts, err := parseHPXOptions(cmd)
	require.NoError(t, err)
	require.Equal(t, 5, opts.limit)
	require.Equal(t, time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC), opts.rangeOpts.Start.UTC())
	require.Equal(t, time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC), opts.rangeOpts.End.UTC())
}

func decodeHPXRecords(t *testing.T, lines []string) []map[string]any {
	t.Helper()
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		records = append(records, record)
	}
	return records
}

func findHPXRecord(t *testing.T, records []map[string]any, table, key string) map[string]any {
	t.Helper()
	for _, record := range records {
		if record["table"] == table && record["key"] == key {
			return record
		}
	}
	t.Fatalf("record %s/%s not found", table, key)
	return nil
}

func findHPXSummaryDoc(t *testing.T, records []map[string]any, originID string) map[string]any {
	t.Helper()
	for _, record := range records {
		if record["table"] == "document" && record["origin_id"] == originID {
			return record
		}
	}
	t.Fatalf("summary document %s not found", originID)
	return nil
}
