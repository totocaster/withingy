package workouts

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/toto/withingy/internal/api"
)

const workoutsPath = "/v2/measure"

// Service fetches Withings workout data.
type Service struct {
	client interface {
		PostFormJSON(ctx context.Context, path string, form url.Values, dest any) error
	}
	now func() time.Time
}

// NewService creates a workout Service backed by the shared API client.
func NewService(client *api.Client) *Service {
	return &Service{client: client, now: time.Now}
}

// ListResult captures workouts returned by Withings.
type ListResult struct {
	Workouts []Workout `json:"workouts"`
}

// Workout represents the CLI-friendly Withings workout model.
type Workout struct {
	ID        string     `json:"id"`
	Start     time.Time  `json:"start"`
	End       time.Time  `json:"end"`
	Date      string     `json:"date,omitempty"`
	Timezone  string     `json:"timezone,omitempty"`
	Category  *int       `json:"category,omitempty"`
	Model     *int       `json:"model,omitempty"`
	Modified  *time.Time `json:"modified,omitempty"`
	Attrib    *int       `json:"attrib,omitempty"`
	DeviceID  string     `json:"device_id,omitempty"`
	Calories  *float64   `json:"calories,omitempty"`
	Distance  *float64   `json:"distance,omitempty"`
	Elevation *float64   `json:"elevation,omitempty"`
	Steps     *int       `json:"steps,omitempty"`
}

// List retrieves workouts based on the provided time range.
func (s *Service) List(ctx context.Context, opts *api.ListOptions) (*ListResult, error) {
	if opts == nil {
		opts = defaultWorkoutOptions(s.now())
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	rangeStart, rangeEnd := resolvedWorkoutRange(opts, s.now())
	start, end := rangeStart.UTC().Unix(), rangeEnd.UTC().Unix()
	form := url.Values{}
	form.Set("action", "getworkouts")
	form.Set("startdate", strconv.FormatInt(start, 10))
	form.Set("enddate", strconv.FormatInt(end, 10))

	var body struct {
		Series []workoutRecord `json:"series"`
	}
	if err := s.client.PostFormJSON(ctx, workoutsPath, form, &body); err != nil {
		return nil, fmt.Errorf("fetch workouts: %w", err)
	}

	workouts := make([]Workout, 0, len(body.Series))
	for _, record := range body.Series {
		workout := convertWorkout(record)
		if workout.Start.Before(rangeStart) || !workout.Start.Before(rangeEnd) {
			continue
		}
		workouts = append(workouts, workout)
	}
	return &ListResult{Workouts: workouts}, nil
}

// Get fetches a single workout by its synthetic ID (start timestamp).
func (s *Service) Get(ctx context.Context, id string) (*Workout, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("workout id is required")
	}
	startTS, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("workout id must be a Unix start timestamp")
	}

	start := time.Unix(startTS, 0).Add(-24 * time.Hour)
	end := time.Unix(startTS, 0).Add(24 * time.Hour)
	opts := &api.ListOptions{Start: &start, End: &end}
	result, err := s.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	for _, workout := range result.Workouts {
		if workout.ID == id {
			copy := workout
			return &copy, nil
		}
	}
	return nil, nil
}

type workoutRecord struct {
	StartDate int64        `json:"startdate"`
	EndDate   int64        `json:"enddate"`
	Date      string       `json:"date"`
	Timezone  string       `json:"timezone"`
	Category  *int         `json:"category"`
	Model     *int         `json:"model"`
	Modified  *int64       `json:"modified"`
	Attrib    *int         `json:"attrib"`
	DeviceID  string       `json:"deviceid"`
	Data      *workoutData `json:"data"`
}

type workoutData struct {
	Calories  *float64 `json:"calories"`
	Distance  *float64 `json:"distance"`
	Elevation *float64 `json:"elevation"`
	Steps     *int     `json:"steps"`
}

func convertWorkout(record workoutRecord) Workout {
	workout := Workout{
		ID:       strconv.FormatInt(record.StartDate, 10),
		Start:    time.Unix(record.StartDate, 0),
		End:      time.Unix(record.EndDate, 0),
		Date:     record.Date,
		Timezone: record.Timezone,
		Category: record.Category,
		Model:    record.Model,
		Attrib:   record.Attrib,
		DeviceID: record.DeviceID,
	}
	if record.Modified != nil {
		t := time.Unix(*record.Modified, 0)
		workout.Modified = &t
	}
	if record.Data != nil {
		workout.Calories = record.Data.Calories
		workout.Distance = record.Data.Distance
		workout.Elevation = record.Data.Elevation
		workout.Steps = record.Data.Steps
	}
	return workout
}

func defaultWorkoutOptions(now time.Time) *api.ListOptions {
	end := now
	start := now.Add(-7 * 24 * time.Hour)
	return &api.ListOptions{Start: &start, End: &end}
}

func resolvedWorkoutRange(opts *api.ListOptions, now time.Time) (time.Time, time.Time) {
	if opts == nil || opts.Start == nil || opts.End == nil {
		opts = defaultWorkoutOptions(now)
	}
	return opts.Start.UTC(), opts.End.UTC()
}
