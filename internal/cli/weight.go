package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/measures"
)

var (
	weightListFn   = defaultWeightList
	weightLatestFn = defaultWeightLatest
)

func init() {
	rootCmd.AddCommand(weightCmd)
	weightCmd.AddCommand(weightListCmd)
	weightCmd.AddCommand(weightTodayCmd)
	weightCmd.AddCommand(weightLatestCmd)
	addListFlags(weightListCmd)
	weightListCmd.Flags().Bool("text", false, "Human-readable output")
	weightTodayCmd.Flags().Bool("text", false, "Human-readable output")
	weightLatestCmd.Flags().Bool("text", false, "Human-readable output")
}

var weightCmd = &cobra.Command{
	Use:   "weight",
	Short: "Weight-focused body measurements",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var weightListCmd = &cobra.Command{
	Use:   "list",
	Short: "List weight measurements over a date range (defaults to last 30 days)",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := parseListOptions(cmd)
		if err != nil {
			return err
		}
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		result, err := weightListFn(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatWeightListText(result))
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

var weightTodayCmd = &cobra.Command{
	Use:   "today",
	Short: "Show today's weight measurements",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := todayRangeOptions(25)
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		result, err := weightListFn(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatWeightListText(result))
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

var weightLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Show the most recent weight measurement",
	RunE: func(cmd *cobra.Command, args []string) error {
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		entry, err := weightLatestFn(cmd.Context())
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatWeightEntryText(entry))
			return nil
		}
		payload, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return nil
	},
}

func defaultWeightList(ctx context.Context, opts *api.ListOptions) (*measures.WeightListResult, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := measures.NewService(client)
	return service.WeightList(ctx, opts)
}

func defaultWeightLatest(ctx context.Context) (*measures.WeightEntry, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := measures.NewService(client)
	return service.LatestWeight(ctx)
}

func formatWeightListText(result *measures.WeightListResult) string {
	if result == nil || len(result.Weights) == 0 {
		return "No weight measurements found."
	}
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Taken At\tWeight\tGroup ID")
	for _, entry := range result.Weights {
		fmt.Fprintf(tw, "%s\t%.2f kg\t%d\n",
			formatTimestampShort(entry.TakenAt),
			entry.WeightKG,
			entry.GroupID,
		)
	}
	tw.Flush()
	return strings.TrimSpace(b.String())
}

func formatWeightEntryText(entry *measures.WeightEntry) string {
	if entry == nil {
		return "No weight measurements found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Taken At: %s\n", formatTimestamp(entry.TakenAt))
	fmt.Fprintf(&b, "Weight: %.2f kg\n", entry.WeightKG)
	fmt.Fprintf(&b, "Group ID: %d", entry.GroupID)
	return strings.TrimSpace(b.String())
}
