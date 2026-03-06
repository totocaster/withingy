package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/api"
)

const (
	listFlagStart  = "start"
	listFlagEnd    = "end"
	listFlagLimit  = "limit"
	listFlagCursor = "cursor"
)

func addListFlags(cmd *cobra.Command) {
	cmd.Flags().String(listFlagStart, "", "Start timestamp (RFC3339 or YYYY-MM-DD, UTC if date only)")
	cmd.Flags().String(listFlagEnd, "", "End timestamp (RFC3339 or YYYY-MM-DD, UTC if date only)")
	cmd.Flags().Int(listFlagLimit, 0, "Maximum records to return (0 leaves the API default)")
	cmd.Flags().String(listFlagCursor, "", "Opaque cursor token to resume pagination")
}

func parseListOptions(cmd *cobra.Command) (*api.ListOptions, error) {
	startVal, err := cmd.Flags().GetString(listFlagStart)
	if err != nil {
		return nil, err
	}
	endVal, err := cmd.Flags().GetString(listFlagEnd)
	if err != nil {
		return nil, err
	}
	limit, err := cmd.Flags().GetInt(listFlagLimit)
	if err != nil {
		return nil, err
	}
	cursor, err := cmd.Flags().GetString(listFlagCursor)
	if err != nil {
		return nil, err
	}

	opts := &api.ListOptions{Limit: limit}
	if strings.TrimSpace(cursor) != "" {
		opts.NextToken = cursor
	}
	if strings.TrimSpace(startVal) != "" {
		ts, err := parseTimeFlag(startVal)
		if err != nil {
			return nil, fmt.Errorf("invalid --start: %w", err)
		}
		opts.Start = &ts
	}
	if strings.TrimSpace(endVal) != "" {
		ts, err := parseTimeFlag(endVal)
		if err != nil {
			return nil, fmt.Errorf("invalid --end: %w", err)
		}
		opts.End = &ts
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}
	return opts, nil
}

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02",
}

func parseTimeFlag(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			if layout == "2006-01-02" {
				return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
			}
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%q must be RFC3339 or YYYY-MM-DD", value)
}
