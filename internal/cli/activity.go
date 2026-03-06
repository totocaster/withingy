package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/activity"
	"github.com/toto/withingy/internal/api"
)

var (
	activityListFn = defaultActivityList
	activityGetFn  = defaultActivityGet
)

func init() {
	rootCmd.AddCommand(activityCmd)
	activityCmd.AddCommand(activityListCmd)
	activityCmd.AddCommand(activityTodayCmd)
	activityCmd.AddCommand(activityViewCmd)
	addListFlags(activityListCmd)
	activityListCmd.Flags().Bool("text", false, "Human-readable output")
	activityTodayCmd.Flags().Bool("text", false, "Human-readable output")
	activityViewCmd.Flags().Bool("text", false, "Human-readable output")
}

var activityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Daily activity summaries",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var activityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List activity summaries within an optional date range",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := parseListOptions(cmd)
		if err != nil {
			return err
		}
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		result, err := activityListFn(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatActivityListText(result))
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

var activityTodayCmd = &cobra.Command{
	Use:   "today",
	Short: "Show today's activity summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := todayRangeOptions(1)
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		result, err := activityListFn(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatActivityListText(result))
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

var activityViewCmd = &cobra.Command{
	Use:   "view <date>",
	Short: "Show one activity summary by date (YYYY-MM-DD)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		day, err := activityGetFn(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatActivityDetailText(day))
			return nil
		}
		payload, err := json.MarshalIndent(day, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return nil
	},
}

func defaultActivityList(ctx context.Context, opts *api.ListOptions) (*activity.ListResult, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := activity.NewService(client)
	return service.List(ctx, opts)
}

func defaultActivityGet(ctx context.Context, date string) (*activity.Day, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := activity.NewService(client)
	return service.Get(ctx, date)
}

func formatActivityListText(result *activity.ListResult) string {
	if result == nil || len(result.Activities) == 0 {
		return "No activity summaries found."
	}
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Date\tSteps\tDistance\tCalories\tActive\tIntense")
	for _, day := range result.Activities {
		fmt.Fprintf(tw, "%s\t%d\t%.0f m\t%.0f\t%s\t%s\n",
			day.Date,
			day.Steps,
			day.Distance,
			day.Calories,
			formatSecondDuration(day.Active),
			formatSecondDuration(day.Intense),
		)
	}
	tw.Flush()
	return strings.TrimSpace(b.String())
}

func formatActivityDetailText(day *activity.Day) string {
	if day == nil {
		return "Activity summary not found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Date: %s\n", day.Date)
	fmt.Fprintf(&b, "Timezone: %s\n", safeValue(day.Timezone))
	fmt.Fprintf(&b, "Steps: %d\n", day.Steps)
	fmt.Fprintf(&b, "Distance: %.0f m\n", day.Distance)
	fmt.Fprintf(&b, "Calories: %.0f\n", day.Calories)
	fmt.Fprintf(&b, "Total Calories: %.0f\n", day.TotalCalories)
	fmt.Fprintf(&b, "Elevation: %.0f m\n", day.Elevation)
	fmt.Fprintf(&b, "Soft Activity: %s\n", formatSecondDuration(day.Soft))
	fmt.Fprintf(&b, "Moderate Activity: %s\n", formatSecondDuration(day.Moderate))
	fmt.Fprintf(&b, "Intense Activity: %s\n", formatSecondDuration(day.Intense))
	fmt.Fprintf(&b, "Active Time: %s\n", formatSecondDuration(day.Active))
	fmt.Fprintf(&b, "Device ID: %s", safeValue(day.DeviceID))
	return strings.TrimSpace(b.String())
}

func formatSecondDuration(seconds int) string {
	if seconds <= 0 {
		return "n/a"
	}
	return (time.Duration(seconds) * time.Second).String()
}
