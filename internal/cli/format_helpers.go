package cli

import (
	"fmt"
	"strings"
	"time"
)

func safeValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "n/a"
	}
	return value
}

func formatIntPtr(value *int) string {
	if value == nil {
		return "n/a"
	}
	return fmt.Sprintf("%d", *value)
}

func formatFloatPtr(value *float64, precision int) string {
	if value == nil {
		return "n/a"
	}
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, *value)
}

func formatPercent(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", *value*100)
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return t.Local().Format(time.RFC1123)
}

func formatInt64Ptr(value *int64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}

func formatBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func formatDuration(start, end time.Time) string {
	dur := end.Sub(start)
	if start.IsZero() || end.IsZero() || dur <= 0 {
		return "n/a"
	}
	dur = dur.Round(time.Second)
	hours := int(dur.Hours())
	minutes := int(dur.Minutes()) % 60
	seconds := int(dur.Seconds()) % 60
	switch {
	case hours > 0:
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	case minutes > 0:
		if seconds > 0 {
			return fmt.Sprintf("%dm%02ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func formatMillisDuration(value *int64) string {
	if value == nil || *value <= 0 {
		return "n/a"
	}
	dur := time.Duration(*value) * time.Millisecond
	hours := int(dur.Hours())
	minutes := int(dur.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", hours, minutes)
}
