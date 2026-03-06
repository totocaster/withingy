package activity

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/toto/withingy/internal/api"
)

const activityPath = "/v2/measure"

// Service fetches Withings daily activity summaries.
type Service struct {
	client interface {
		PostFormJSON(ctx context.Context, path string, form url.Values, dest any) error
	}
	now func() time.Time
}

// NewService constructs an activity Service backed by the shared API client.
func NewService(client *api.Client) *Service {
	return &Service{client: client, now: time.Now}
}

// ListResult contains daily activity summaries.
type ListResult struct {
	Activities []Day `json:"activities"`
}

// Day is a single Withings activity day.
type Day struct {
	Date          string  `json:"date"`
	Timezone      string  `json:"timezone,omitempty"`
	Steps         int     `json:"steps"`
	Distance      float64 `json:"distance,omitempty"`
	Calories      float64 `json:"calories,omitempty"`
	TotalCalories float64 `json:"total_calories,omitempty"`
	Elevation     float64 `json:"elevation,omitempty"`
	Soft          int     `json:"soft_seconds,omitempty"`
	Moderate      int     `json:"moderate_seconds,omitempty"`
	Intense       int     `json:"intense_seconds,omitempty"`
	Active        int     `json:"active_seconds,omitempty"`
	DeviceID      string  `json:"device_id,omitempty"`
	Brand         int     `json:"brand,omitempty"`
}

// List fetches activity summaries for the requested range.
func (s *Service) List(ctx context.Context, opts *api.ListOptions) (*ListResult, error) {
	if opts == nil {
		opts = defaultDateOptions(s.now())
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	start, end := ymdRange(opts, s.now())
	form := url.Values{}
	form.Set("action", "getactivity")
	form.Set("startdateymd", start)
	form.Set("enddateymd", end)

	var body struct {
		Activities []activityRecord `json:"activities"`
	}
	if err := s.client.PostFormJSON(ctx, activityPath, form, &body); err != nil {
		return nil, fmt.Errorf("fetch activity: %w", err)
	}

	days := make([]Day, len(body.Activities))
	for i, record := range body.Activities {
		days[i] = Day{
			Date:          record.Date,
			Timezone:      record.Timezone,
			Steps:         record.Steps,
			Distance:      record.Distance,
			Calories:      record.Calories,
			TotalCalories: record.TotalCalories,
			Elevation:     record.Elevation,
			Soft:          record.Soft,
			Moderate:      record.Moderate,
			Intense:       record.Intense,
			Active:        record.Active,
			DeviceID:      record.DeviceID,
			Brand:         record.Brand,
		}
	}
	return &ListResult{Activities: days}, nil
}

// Get fetches a single activity summary by calendar date (`YYYY-MM-DD`).
func (s *Service) Get(ctx context.Context, date string) (*Day, error) {
	if strings.TrimSpace(date) == "" {
		return nil, fmt.Errorf("date is required")
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("date must be YYYY-MM-DD")
	}
	opts := &api.ListOptions{Start: &t, End: endOfDay(t)}
	result, err := s.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	for _, day := range result.Activities {
		if day.Date == date {
			copy := day
			return &copy, nil
		}
	}
	return nil, nil
}

type activityRecord struct {
	Date          string  `json:"date"`
	Timezone      string  `json:"timezone"`
	Steps         int     `json:"steps"`
	Distance      float64 `json:"distance"`
	Calories      float64 `json:"calories"`
	TotalCalories float64 `json:"totalcalories"`
	Elevation     float64 `json:"elevation"`
	Soft          int     `json:"soft"`
	Moderate      int     `json:"moderate"`
	Intense       int     `json:"intense"`
	Active        int     `json:"active"`
	DeviceID      string  `json:"deviceid"`
	Brand         int     `json:"brand"`
}

func defaultDateOptions(now time.Time) *api.ListOptions {
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(24 * time.Hour)
	start := end.Add(-7 * 24 * time.Hour)
	return &api.ListOptions{Start: &start, End: &end}
}

func ymdRange(opts *api.ListOptions, now time.Time) (string, string) {
	if opts == nil || opts.Start == nil || opts.End == nil {
		opts = defaultDateOptions(now)
	}

	start := opts.Start.In(now.Location()).Format("2006-01-02")
	end := opts.End.Add(-time.Second).In(now.Location()).Format("2006-01-02")
	return start, end
}

func endOfDay(t time.Time) *time.Time {
	end := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Add(24 * time.Hour)
	return &end
}
