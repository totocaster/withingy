package sleep

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/toto/withingy/internal/api"
)

const sleepPath = "/v2/sleep"

// Service fetches Withings sleep summaries.
type Service struct {
	client interface {
		PostFormJSON(ctx context.Context, path string, form url.Values, dest any) error
	}
	now func() time.Time
}

// NewService constructs a sleep Service backed by the shared API client.
func NewService(client *api.Client) *Service {
	return &Service{client: client, now: time.Now}
}

// ListResult captures sleep sessions.
type ListResult struct {
	Sleeps []Session `json:"sleeps"`
}

// Session represents a Withings sleep summary keyed by date.
type Session struct {
	ID       string     `json:"id"`
	Date     string     `json:"date"`
	Start    time.Time  `json:"start"`
	End      time.Time  `json:"end"`
	Timezone string     `json:"timezone,omitempty"`
	Model    *int       `json:"model,omitempty"`
	Modified *time.Time `json:"modified,omitempty"`
	Data     Score      `json:"data"`
}

// Score contains the reported Withings sleep metrics.
type Score struct {
	SleepScore            *float64 `json:"sleep_score,omitempty"`
	TimeInBedSeconds      *int     `json:"time_in_bed_seconds,omitempty"`
	LightSleepSeconds     *int     `json:"light_sleep_seconds,omitempty"`
	DeepSleepSeconds      *int     `json:"deep_sleep_seconds,omitempty"`
	REMSleepSeconds       *int     `json:"rem_sleep_seconds,omitempty"`
	WakeUpDurationSeconds *int     `json:"wakeup_duration_seconds,omitempty"`
	ToSleepSeconds        *int     `json:"to_sleep_seconds,omitempty"`
	ToWakeSeconds         *int     `json:"to_wake_seconds,omitempty"`
	AverageHeartRate      *float64 `json:"average_heart_rate,omitempty"`
	AverageRespRate       *float64 `json:"average_respiratory_rate,omitempty"`
	SnoringSeconds        *int     `json:"snoring_seconds,omitempty"`
}

// List retrieves sleep summaries for the requested range.
func (s *Service) List(ctx context.Context, opts *api.ListOptions) (*ListResult, error) {
	if opts == nil {
		opts = defaultSleepOptions(s.now())
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	start, end := sleepDateRange(opts, s.now())
	form := url.Values{}
	form.Set("action", "getsummary")
	form.Set("startdateymd", start)
	form.Set("enddateymd", end)

	var body struct {
		Series []sleepRecord `json:"series"`
	}
	if err := s.client.PostFormJSON(ctx, sleepPath, form, &body); err != nil {
		return nil, fmt.Errorf("fetch sleep summaries: %w", err)
	}

	sleeps := make([]Session, 0, len(body.Series))
	for _, record := range body.Series {
		sleeps = append(sleeps, convertSleep(record))
	}
	return &ListResult{Sleeps: sleeps}, nil
}

// Get returns a single sleep summary by date (`YYYY-MM-DD`).
func (s *Service) Get(ctx context.Context, id string) (*Session, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("sleep id is required")
	}
	t, err := time.Parse("2006-01-02", id)
	if err != nil {
		return nil, fmt.Errorf("sleep id must be YYYY-MM-DD")
	}
	opts := &api.ListOptions{Start: &t, End: endOfSleepDay(t)}
	result, err := s.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	for _, sess := range result.Sleeps {
		if sess.ID == id {
			copy := sess
			return &copy, nil
		}
	}
	return nil, nil
}

type sleepRecord struct {
	Date      string     `json:"date"`
	StartDate int64      `json:"startdate"`
	EndDate   int64      `json:"enddate"`
	Timezone  string     `json:"timezone"`
	Model     *int       `json:"model"`
	Modified  *int64     `json:"modified"`
	Data      *sleepData `json:"data"`
}

type sleepData struct {
	SleepScore            *float64 `json:"sleep_score"`
	TimeInBedSeconds      *int     `json:"total_sleep_duration"`
	LightSleepSeconds     *int     `json:"lightsleepduration"`
	DeepSleepSeconds      *int     `json:"deepsleepduration"`
	REMSleepSeconds       *int     `json:"remsleepduration"`
	WakeUpDurationSeconds *int     `json:"wakeupduration"`
	ToSleepSeconds        *int     `json:"durationtosleep"`
	ToWakeSeconds         *int     `json:"durationtowakeup"`
	AverageHeartRate      *float64 `json:"hr_average"`
	AverageRespRate       *float64 `json:"rr_average"`
	SnoringSeconds        *int     `json:"snoring"`
}

func convertSleep(record sleepRecord) Session {
	session := Session{
		ID:       record.Date,
		Date:     record.Date,
		Start:    time.Unix(record.StartDate, 0),
		End:      time.Unix(record.EndDate, 0),
		Timezone: record.Timezone,
		Model:    record.Model,
	}
	if record.Modified != nil {
		t := time.Unix(*record.Modified, 0)
		session.Modified = &t
	}
	if record.Data != nil {
		session.Data = Score{
			SleepScore:            record.Data.SleepScore,
			TimeInBedSeconds:      record.Data.TimeInBedSeconds,
			LightSleepSeconds:     record.Data.LightSleepSeconds,
			DeepSleepSeconds:      record.Data.DeepSleepSeconds,
			REMSleepSeconds:       record.Data.REMSleepSeconds,
			WakeUpDurationSeconds: record.Data.WakeUpDurationSeconds,
			ToSleepSeconds:        record.Data.ToSleepSeconds,
			ToWakeSeconds:         record.Data.ToWakeSeconds,
			AverageHeartRate:      record.Data.AverageHeartRate,
			AverageRespRate:       record.Data.AverageRespRate,
			SnoringSeconds:        record.Data.SnoringSeconds,
		}
	}
	return session
}

func defaultSleepOptions(now time.Time) *api.ListOptions {
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(24 * time.Hour)
	start := end.Add(-7 * 24 * time.Hour)
	return &api.ListOptions{Start: &start, End: &end}
}

func sleepDateRange(opts *api.ListOptions, now time.Time) (string, string) {
	if opts == nil || opts.Start == nil || opts.End == nil {
		opts = defaultSleepOptions(now)
	}
	start := opts.Start.In(now.Location()).Format("2006-01-02")
	end := opts.End.Add(-time.Second).In(now.Location()).Format("2006-01-02")
	return start, end
}

func endOfSleepDay(t time.Time) *time.Time {
	end := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Add(24 * time.Hour)
	return &end
}
