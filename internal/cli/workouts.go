package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/workouts"
)

var (
	workoutsListFn = defaultWorkoutsList
	workoutsViewFn = defaultWorkoutsView
)

func init() {
	rootCmd.AddCommand(workoutsCmd)
	workoutsCmd.AddCommand(workoutsListCmd)
	workoutsCmd.AddCommand(workoutsTodayCmd)
	workoutsCmd.AddCommand(workoutsViewCmd)
	workoutsCmd.AddCommand(workoutsExportCmd)
	addListFlags(workoutsListCmd)
	addListFlags(workoutsExportCmd)
	workoutsListCmd.Flags().Bool("text", false, "Human-readable output")
	workoutsTodayCmd.Flags().Bool("text", false, "Human-readable output")
	workoutsViewCmd.Flags().Bool("text", false, "Human-readable output")
	workoutsExportCmd.Flags().String("format", "jsonl", "Export format: jsonl or csv")
	workoutsExportCmd.Flags().String("output", "-", "Output path ('-' for stdout)")
}

var workoutsCmd = &cobra.Command{
	Use:   "workouts",
	Short: "Workout-related commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var workoutsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workouts within an optional time range",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := parseListOptions(cmd)
		if err != nil {
			return err
		}
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		result, err := workoutsListFn(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatWorkoutsText(result))
			return nil
		}
		payload, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return nil
	},
}

var workoutsTodayCmd = &cobra.Command{
	Use:   "today",
	Short: "List today's workouts quickly",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := todayRangeOptions(25)
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		result, err := workoutsListFn(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatWorkoutsText(result))
			return nil
		}
		payload, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return nil
	},
}

var workoutsViewCmd = &cobra.Command{
	Use:   "view <workout-id>",
	Short: "Show a single workout by start timestamp",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		workout, err := workoutsViewFn(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatWorkoutDetailText(workout))
			return nil
		}
		payload, err := json.MarshalIndent(workout, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return nil
	},
}

var workoutsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export workouts over a time range",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := parseListOptions(cmd)
		if err != nil {
			return err
		}
		format, err := cmd.Flags().GetString("format")
		if err != nil {
			return err
		}
		outputPath, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}
		return exportWorkouts(cmd.Context(), opts, format, outputPath, cmd.OutOrStdout())
	},
}

func defaultWorkoutsList(ctx context.Context, opts *api.ListOptions) (*workouts.ListResult, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := workouts.NewService(client)
	return service.List(ctx, opts)
}

func defaultWorkoutsView(ctx context.Context, id string) (*workouts.Workout, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := workouts.NewService(client)
	return service.Get(ctx, id)
}

func formatWorkoutsText(result *workouts.ListResult) string {
	if result == nil || len(result.Workouts) == 0 {
		return "No workouts found."
	}
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Start\tDuration\tCategory\tCalories\tDistance\tID")
	for _, w := range result.Workouts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			formatTimestampShort(w.Start),
			formatDuration(w.Start, w.End),
			formatIntPtr(w.Category),
			formatOptionalFloat(w.Calories, "%.0f"),
			formatOptionalFloat(w.Distance, "%.0f m"),
			w.ID,
		)
	}
	tw.Flush()
	return strings.TrimSpace(b.String())
}

func formatWorkoutDetailText(w *workouts.Workout) string {
	if w == nil {
		return "Workout not found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ID: %s\n", w.ID)
	fmt.Fprintf(&b, "Start: %s\n", formatTimestamp(w.Start))
	fmt.Fprintf(&b, "End: %s\n", formatTimestamp(w.End))
	fmt.Fprintf(&b, "Duration: %s\n", formatDuration(w.Start, w.End))
	fmt.Fprintf(&b, "Date: %s\n", safeValue(w.Date))
	fmt.Fprintf(&b, "Timezone: %s\n", safeValue(w.Timezone))
	fmt.Fprintf(&b, "Category: %s\n", formatIntPtr(w.Category))
	fmt.Fprintf(&b, "Model: %s\n", formatIntPtr(w.Model))
	fmt.Fprintf(&b, "Calories: %s\n", formatOptionalFloat(w.Calories, "%.0f"))
	fmt.Fprintf(&b, "Distance: %s\n", formatOptionalFloat(w.Distance, "%.0f m"))
	fmt.Fprintf(&b, "Elevation: %s\n", formatOptionalFloat(w.Elevation, "%.0f m"))
	fmt.Fprintf(&b, "Steps: %s\n", formatIntPtr(w.Steps))
	fmt.Fprintf(&b, "Device ID: %s", safeValue(w.DeviceID))
	return strings.TrimSpace(b.String())
}

func exportWorkouts(ctx context.Context, opts *api.ListOptions, format, outputPath string, stdout io.Writer) error {
	result, err := workoutsListFn(ctx, opts)
	if err != nil {
		return err
	}

	writer, closeFn, err := openOutput(outputPath, stdout)
	if err != nil {
		return err
	}
	defer closeFn()

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jsonl":
		enc := json.NewEncoder(writer)
		for _, workout := range result.Workouts {
			if err := enc.Encode(workout); err != nil {
				return err
			}
		}
		return nil
	case "csv":
		cw := csv.NewWriter(writer)
		if err := cw.Write([]string{"id", "start", "end", "date", "timezone", "category", "model", "calories", "distance", "elevation", "steps", "device_id"}); err != nil {
			return err
		}
		for _, workout := range result.Workouts {
			if err := cw.Write([]string{
				workout.ID,
				workout.Start.Format(timeFormatRFC3339UTC),
				workout.End.Format(timeFormatRFC3339UTC),
				workout.Date,
				workout.Timezone,
				intPtrCSV(workout.Category),
				intPtrCSV(workout.Model),
				floatPtrCSV(workout.Calories),
				floatPtrCSV(workout.Distance),
				floatPtrCSV(workout.Elevation),
				intPtrCSV(workout.Steps),
				workout.DeviceID,
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	default:
		return fmt.Errorf("unsupported export format %q", format)
	}
}

const timeFormatRFC3339UTC = "2006-01-02T15:04:05Z07:00"

func formatTimestampShort(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func formatOptionalFloat(value *float64, format string) string {
	if value == nil {
		return "n/a"
	}
	return fmt.Sprintf(format, *value)
}

func openOutput(outputPath string, stdout io.Writer) (io.Writer, func() error, error) {
	if outputPath == "-" {
		return stdout, func() error { return nil }, nil
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, nil, err
	}
	return file, file.Close, nil
}

func intPtrCSV(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func floatPtrCSV(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%f", *value)
}
