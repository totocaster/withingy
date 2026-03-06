package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/stats"
)

var statsDailyFn = defaultStatsDaily

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.AddCommand(statsDailyCmd)
	statsDailyCmd.Flags().String("date", "", "Calendar date in YYYY-MM-DD")
	statsDailyCmd.Flags().Bool("text", false, "Human-readable output")
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Aggregated Withings statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var statsDailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Aggregate activity, sleep, and workouts for one day",
	RunE: func(cmd *cobra.Command, args []string) error {
		dateFlag, err := cmd.Flags().GetString("date")
		if err != nil {
			return err
		}
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		date, err := parseStatsDate(dateFlag)
		if err != nil {
			return err
		}
		report, err := statsDailyFn(cmd.Context(), date)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatStatsText(report))
			return nil
		}
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return nil
	},
}

func defaultStatsDaily(ctx context.Context, date time.Time) (*stats.DailyReport, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := stats.NewService(client)
	return service.Daily(ctx, date)
}

func parseStatsDate(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("date must be YYYY-MM-DD")
	}
	return parsed, nil
}

func formatStatsText(report *stats.DailyReport) string {
	if report == nil {
		return "No stats available."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Withings Stats - %s\n", report.Date)
	fmt.Fprintf(&b, "\nSummary\n")
	fmt.Fprintf(&b, "  Steps: %d\n", report.Summary.Steps)
	fmt.Fprintf(&b, "  Distance: %.0f m\n", report.Summary.Distance)
	fmt.Fprintf(&b, "  Calories: %.0f\n", report.Summary.Calories)
	fmt.Fprintf(&b, "  Workouts: %d\n", report.Summary.WorkoutCount)
	fmt.Fprintf(&b, "  Workout Calories: %.0f\n", report.Summary.WorkoutCals)
	fmt.Fprintf(&b, "  Sleep Sessions: %d\n", report.Summary.SleepSessions)
	fmt.Fprintf(&b, "  Sleep Hours: %.1f\n", report.Summary.SleepHours)
	if report.Summary.SleepScore != nil {
		fmt.Fprintf(&b, "  Sleep Score: %.0f\n", *report.Summary.SleepScore)
	}

	if report.Activity != nil {
		fmt.Fprintf(&b, "\nActivity\n")
		fmt.Fprintf(&b, "  Active Time: %s\n", formatSecondDuration(report.Activity.Active))
		fmt.Fprintf(&b, "  Intense Time: %s\n", formatSecondDuration(report.Activity.Intense))
	}

	if len(report.Workouts) > 0 {
		fmt.Fprintf(&b, "\nWorkouts\n")
		for _, workout := range report.Workouts {
			fmt.Fprintf(&b, "  %s  %s  %s\n", formatTimestampShort(workout.Start), formatDuration(workout.Start, workout.End), workout.ID)
		}
	}

	if len(report.Sleep) > 0 {
		fmt.Fprintf(&b, "\nSleep\n")
		for _, session := range report.Sleep {
			fmt.Fprintf(&b, "  %s  %s  %s\n", session.Date, formatDuration(session.Start, session.End), session.ID)
		}
	}

	return strings.TrimSpace(b.String())
}
