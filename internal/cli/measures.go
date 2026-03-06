package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/measures"
)

const (
	measuresTypesFlag      = "types"
	measuresCategoryFlag   = "category"
	measuresLastUpdateFlag = "last-update"
)

var measuresListFn = defaultMeasuresList

func init() {
	rootCmd.AddCommand(measuresCmd)
	measuresCmd.AddCommand(measuresListCmd)
	addListFlags(measuresListCmd)
	measuresListCmd.Flags().String(measuresTypesFlag, "", "Comma-separated measure type aliases or numeric codes")
	measuresListCmd.Flags().Int(measuresCategoryFlag, 0, "Measurement category filter (1 real, 2 objective)")
	measuresListCmd.Flags().Int64(measuresLastUpdateFlag, 0, "Only measurements updated after this Unix timestamp")
	measuresListCmd.Flags().Bool("text", false, "Human-readable output")
}

var measuresCmd = &cobra.Command{
	Use:   "measures",
	Short: "Raw Withings body measurements",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var measuresListCmd = &cobra.Command{
	Use:   "list",
	Short: "List measurement groups from the Withings getmeas endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		query, err := parseMeasuresQuery(cmd)
		if err != nil {
			return err
		}
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}

		result, err := measuresListFn(cmd.Context(), query)
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatMeasureGroupsText(result))
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

func defaultMeasuresList(ctx context.Context, query *measures.Query) (*measures.ListResult, error) {
	client, err := apiClientFactory()
	if err != nil {
		return nil, err
	}
	service := measures.NewService(client)
	return service.List(ctx, query)
}

func parseMeasuresQuery(cmd *cobra.Command) (*measures.Query, error) {
	opts, err := parseListOptions(cmd)
	if err != nil {
		return nil, err
	}

	rawTypes, err := cmd.Flags().GetString(measuresTypesFlag)
	if err != nil {
		return nil, err
	}
	categoryValue, err := cmd.Flags().GetInt(measuresCategoryFlag)
	if err != nil {
		return nil, err
	}
	lastUpdateValue, err := cmd.Flags().GetInt64(measuresLastUpdateFlag)
	if err != nil {
		return nil, err
	}

	query := &measures.Query{Range: opts}
	if strings.TrimSpace(rawTypes) != "" {
		query.Types, err = measures.ParseTypes(rawTypes)
		if err != nil {
			return nil, fmt.Errorf("invalid --types: %w", err)
		}
	}
	if cmd.Flags().Changed(measuresCategoryFlag) {
		query.Category = &categoryValue
	}
	if cmd.Flags().Changed(measuresLastUpdateFlag) {
		query.LastUpdate = &lastUpdateValue
	}
	return query, nil
}

func formatMeasureGroupsText(result *measures.ListResult) string {
	if result == nil || len(result.Groups) == 0 {
		return "No measurements found."
	}

	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Taken At\tCategory\tMeasures\tGroup ID")
	for _, group := range result.Groups {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n",
			formatTimestampShort(group.TakenAt),
			group.CategoryName,
			formatMeasureSummary(group.Measures),
			group.ID,
		)
	}
	tw.Flush()
	return strings.TrimSpace(b.String())
}

func formatMeasureSummary(values []measures.Measure) string {
	if len(values) == 0 {
		return "n/a"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%s=%s", value.Code, formatMeasureValue(value)))
	}
	return strings.Join(parts, ", ")
}

func formatMeasureValue(value measures.Measure) string {
	switch value.Unit {
	case "%":
		return fmt.Sprintf("%.1f %%", value.Value)
	case "kg":
		return fmt.Sprintf("%.2f kg", value.Value)
	case "m":
		return fmt.Sprintf("%.3f m", value.Value)
	case "mmHg":
		return fmt.Sprintf("%.0f mmHg", value.Value)
	case "bpm":
		return fmt.Sprintf("%.0f bpm", value.Value)
	case "C":
		return fmt.Sprintf("%.1f C", value.Value)
	case "m/s":
		return fmt.Sprintf("%.2f m/s", value.Value)
	default:
		return fmt.Sprintf("%.3f", value.Value)
	}
}
