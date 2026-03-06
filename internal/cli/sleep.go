package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/sleep"
)

var (
	sleepListFn = defaultSleepList
	sleepViewFn = defaultSleepView
)

func init() {
	rootCmd.AddCommand(sleepCmd)
	sleepCmd.AddCommand(sleepListCmd)
	sleepCmd.AddCommand(sleepTodayCmd)
	sleepCmd.AddCommand(sleepViewCmd)
	addListFlags(sleepListCmd)
	sleepListCmd.Flags().Bool("text", false, "Human-readable output")
	sleepTodayCmd.Flags().Bool("text", false, "Human-readable output")
	sleepViewCmd.Flags().Bool("text", false, "Human-readable output")
}

var sleepCmd = &cobra.Command{
	Use:   "sleep",
	Short: "Sleep-related commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var sleepListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sleep summaries within an optional time range",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := parseListOptions(cmd)
		if err != nil {
			return err
		}
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		result, err := sleepListFn(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatSleepListText(result))
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

var sleepTodayCmd = &cobra.Command{
	Use:   "today",
	Short: "Show today's sleep summaries",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := todayRangeOptions(7)
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		result, err := sleepListFn(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatSleepListText(result))
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

var sleepViewCmd = &cobra.Command{
	Use:   "view <sleep-id>",
	Short: "Show a single sleep summary by date",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		sess, err := sleepViewFn(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatSleepDetailText(sess))
			return nil
		}
		payload, err := json.MarshalIndent(sess, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return nil
	},
}

func defaultSleepList(ctx context.Context, opts *api.ListOptions) (*sleep.ListResult, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := sleep.NewService(client)
	return service.List(ctx, opts)
}

func defaultSleepView(ctx context.Context, id string) (*sleep.Session, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := sleep.NewService(client)
	return service.Get(ctx, id)
}

func formatSleepListText(result *sleep.ListResult) string {
	if result == nil || len(result.Sleeps) == 0 {
		return "No sleep summaries found."
	}
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Date\tStart\tDuration\tScore\tAvg HR\tAvg RR")
	for _, sess := range result.Sleeps {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			sess.Date,
			formatTimestampShort(sess.Start),
			formatDuration(sess.Start, sess.End),
			formatOptionalFloat(sess.Data.SleepScore, "%.0f"),
			formatOptionalFloat(sess.Data.AverageHeartRate, "%.0f"),
			formatOptionalFloat(sess.Data.AverageRespRate, "%.1f"),
		)
	}
	tw.Flush()
	return strings.TrimSpace(b.String())
}

func formatSleepDetailText(sess *sleep.Session) string {
	if sess == nil {
		return "Sleep summary not found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ID: %s\n", sess.ID)
	fmt.Fprintf(&b, "Date: %s\n", sess.Date)
	fmt.Fprintf(&b, "Start: %s\n", formatTimestamp(sess.Start))
	fmt.Fprintf(&b, "End: %s\n", formatTimestamp(sess.End))
	fmt.Fprintf(&b, "Duration: %s\n", formatDuration(sess.Start, sess.End))
	fmt.Fprintf(&b, "Timezone: %s\n", safeValue(sess.Timezone))
	fmt.Fprintf(&b, "Sleep Score: %s\n", formatOptionalFloat(sess.Data.SleepScore, "%.0f"))
	fmt.Fprintf(&b, "Time In Bed: %s\n", formatSecondsPtr(sess.Data.TimeInBedSeconds))
	fmt.Fprintf(&b, "Light Sleep: %s\n", formatSecondsPtr(sess.Data.LightSleepSeconds))
	fmt.Fprintf(&b, "Deep Sleep: %s\n", formatSecondsPtr(sess.Data.DeepSleepSeconds))
	fmt.Fprintf(&b, "REM Sleep: %s\n", formatSecondsPtr(sess.Data.REMSleepSeconds))
	fmt.Fprintf(&b, "Wake Duration: %s\n", formatSecondsPtr(sess.Data.WakeUpDurationSeconds))
	fmt.Fprintf(&b, "Average HR: %s\n", formatOptionalFloat(sess.Data.AverageHeartRate, "%.0f"))
	fmt.Fprintf(&b, "Average RR: %s\n", formatOptionalFloat(sess.Data.AverageRespRate, "%.1f"))
	fmt.Fprintf(&b, "Snoring: %s", formatSecondsPtr(sess.Data.SnoringSeconds))
	return strings.TrimSpace(b.String())
}

func formatSecondsPtr(value *int) string {
	if value == nil {
		return "n/a"
	}
	return (time.Duration(*value) * time.Second).String()
}
