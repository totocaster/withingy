package cli

import (
	"time"

	"github.com/toto/withingy/internal/api"
)

func todayRangeOptions(limit int) *api.ListOptions {
	now := time.Now().In(time.Local)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24 * time.Hour)
	startUTC := start.UTC()
	endUTC := end.UTC()
	if limit <= 0 || limit > 25 {
		limit = 25
	}
	return &api.ListOptions{Start: &startUTC, End: &endUTC, Limit: limit}
}
