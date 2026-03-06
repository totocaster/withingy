package stats

import (
	"context"
	"time"

	"github.com/toto/withingy/internal/activity"
	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/sleep"
	"github.com/toto/withingy/internal/workouts"
)

// Service aggregates Withings resources into higher-level reports.
type Service struct {
	activitySvc activityFetcher
	sleepSvc    sleepFetcher
	workoutSvc  workoutFetcher
}

// NewService wires the stats service to shared API-backed services.
func NewService(client *api.Client) *Service {
	return &Service{
		activitySvc: activity.NewService(client),
		sleepSvc:    sleep.NewService(client),
		workoutSvc:  workouts.NewService(client),
	}
}

// DailyReport summarizes all activity for a single calendar day.
type DailyReport struct {
	Date     string             `json:"date"`
	Start    time.Time          `json:"start"`
	End      time.Time          `json:"end"`
	Activity *activity.Day      `json:"activity,omitempty"`
	Sleep    []sleep.Session    `json:"sleep,omitempty"`
	Workouts []workouts.Workout `json:"workouts,omitempty"`
	Summary  Summary            `json:"summary"`
}

// Summary contains derived statistics for a daily report.
type Summary struct {
	Steps         int      `json:"steps"`
	Distance      float64  `json:"distance"`
	Calories      float64  `json:"calories"`
	WorkoutCount  int      `json:"workout_count"`
	WorkoutCals   float64  `json:"workout_calories"`
	SleepSessions int      `json:"sleep_sessions"`
	SleepHours    float64  `json:"sleep_hours"`
	SleepScore    *float64 `json:"sleep_score,omitempty"`
}

// Daily fetches all Withings resources for the provided date (local timezone).
func (s *Service) Daily(ctx context.Context, date time.Time) (*DailyReport, error) {
	local := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	start := local
	end := local.Add(24 * time.Hour)

	report := &DailyReport{Date: start.Format("2006-01-02"), Start: start, End: end}
	opts := &api.ListOptions{Start: &start, End: &end}

	activityResult, err := s.activitySvc.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	if activityResult != nil {
		for _, day := range activityResult.Activities {
			if day.Date == report.Date {
				copy := day
				report.Activity = &copy
				break
			}
		}
	}

	sleepResult, err := s.sleepSvc.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	if sleepResult != nil {
		report.Sleep = append(report.Sleep, sleepResult.Sleeps...)
	}

	workoutResult, err := s.workoutSvc.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	if workoutResult != nil {
		report.Workouts = append(report.Workouts, workoutResult.Workouts...)
	}

	report.Summary = buildSummary(report)
	return report, nil
}

func buildSummary(report *DailyReport) Summary {
	summary := Summary{
		WorkoutCount:  len(report.Workouts),
		SleepSessions: len(report.Sleep),
	}

	if report.Activity != nil {
		summary.Steps = report.Activity.Steps
		summary.Distance = report.Activity.Distance
		summary.Calories = report.Activity.Calories
	}

	for _, workout := range report.Workouts {
		if workout.Calories != nil {
			summary.WorkoutCals += *workout.Calories
		}
	}

	for _, session := range report.Sleep {
		summary.SleepHours += session.End.Sub(session.Start).Hours()
		if summary.SleepScore == nil && session.Data.SleepScore != nil {
			value := *session.Data.SleepScore
			summary.SleepScore = &value
		}
	}

	return summary
}

type activityFetcher interface {
	List(ctx context.Context, opts *api.ListOptions) (*activity.ListResult, error)
}

type sleepFetcher interface {
	List(ctx context.Context, opts *api.ListOptions) (*sleep.ListResult, error)
}

type workoutFetcher interface {
	List(ctx context.Context, opts *api.ListOptions) (*workouts.ListResult, error)
}
