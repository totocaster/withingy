package recovery

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/toto/withingy/internal/api"
)

const recoveryPath = "/recovery"

// Service fetches recovery data from WHOOP's API.
type Service struct {
	client interface {
		GetJSON(ctx context.Context, path string, query url.Values, dest any) error
	}
}

// NewService constructs a recovery Service backed by the shared API client.
func NewService(client *api.Client) *Service {
	return &Service{client: client}
}

// ListResult captures a single page of recoveries.
type ListResult struct {
	Recoveries []Recovery `json:"recoveries"`
	NextToken  string     `json:"next_token,omitempty"`
}

// Recovery contains the recovery metrics for a specific cycle.
type Recovery struct {
	CycleID    int64         `json:"cycle_id"`
	SleepID    string        `json:"sleep_id"`
	UserID     *int64        `json:"user_id,omitempty"`
	ScoreState string        `json:"score_state"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
	Score      RecoveryScore `json:"score"`
}

// RecoveryScore represents the physiological metrics returned by WHOOP.
type RecoveryScore struct {
	UserCalibrating  bool     `json:"user_calibrating"`
	RecoveryScore    *float64 `json:"recovery_score,omitempty"`
	RestingHeartRate *float64 `json:"resting_heart_rate,omitempty"`
	HRVRMSSDMilli    *float64 `json:"hrv_rmssd_milli,omitempty"`
	RespiratoryRate  *float64 `json:"respiratory_rate,omitempty"`
	Spo2Percentage   *float64 `json:"spo2_percentage,omitempty"`
	SkinTempCelsius  *float64 `json:"skin_temp_celsius,omitempty"`
	CycleStrain      *float64 `json:"cycle_strain,omitempty"`
}

// List retrieves a page of recoveries.
func (s *Service) List(ctx context.Context, opts *api.ListOptions) (*ListResult, error) {
	if opts == nil {
		opts = &api.ListOptions{}
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	query := opts.Apply(nil)
	var resp struct {
		Records   []recoveryRecord `json:"records"`
		NextToken string           `json:"next_token"`
	}
	if err := s.client.GetJSON(ctx, recoveryPath, query, &resp); err != nil {
		return nil, fmt.Errorf("fetch recoveries: %w", err)
	}
	recoveries := make([]Recovery, len(resp.Records))
	for i, record := range resp.Records {
		rec, err := convertRecord(record)
		if err != nil {
			return nil, err
		}
		recoveries[i] = rec
	}
	return &ListResult{Recoveries: recoveries, NextToken: strings.TrimSpace(resp.NextToken)}, nil
}

// GetByCycle returns the recovery score associated with a given cycle ID.
func (s *Service) GetByCycle(ctx context.Context, cycleID string) (*Recovery, error) {
	if strings.TrimSpace(cycleID) == "" {
		return nil, fmt.Errorf("cycle id is required")
	}
	path := fmt.Sprintf("/cycle/%s/recovery", strings.TrimSpace(cycleID))
	var record recoveryRecord
	if err := s.client.GetJSON(ctx, path, nil, &record); err != nil {
		return nil, fmt.Errorf("fetch recovery for cycle %s: %w", cycleID, err)
	}
	rec, err := convertRecord(record)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

type recoveryRecord struct {
	CycleID    int64          `json:"cycle_id"`
	SleepID    string         `json:"sleep_id"`
	UserID     *int64         `json:"user_id"`
	ScoreState string         `json:"score_state"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
	Score      *recoveryScore `json:"score"`
}

type recoveryScore struct {
	UserCalibrating  bool     `json:"user_calibrating"`
	RecoveryScore    *float64 `json:"recovery_score"`
	RestingHeartRate *float64 `json:"resting_heart_rate"`
	HRVRMSSDMilli    *float64 `json:"hrv_rmssd_milli"`
	RespiratoryRate  *float64 `json:"respiratory_rate"`
	Spo2Percentage   *float64 `json:"spo2_percentage"`
	SkinTempCelsius  *float64 `json:"skin_temp_celsius"`
	CycleStrain      *float64 `json:"cycle_strain"`
}

func convertRecord(record recoveryRecord) (Recovery, error) {
	created, err := parseTime(record.CreatedAt)
	if err != nil {
		return Recovery{}, fmt.Errorf("parse created_at: %w", err)
	}
	updated, err := parseTime(record.UpdatedAt)
	if err != nil {
		return Recovery{}, fmt.Errorf("parse updated_at: %w", err)
	}

	score := RecoveryScore{}
	if record.Score != nil {
		score = RecoveryScore{
			UserCalibrating:  record.Score.UserCalibrating,
			RecoveryScore:    record.Score.RecoveryScore,
			RestingHeartRate: record.Score.RestingHeartRate,
			HRVRMSSDMilli:    record.Score.HRVRMSSDMilli,
			RespiratoryRate:  record.Score.RespiratoryRate,
			Spo2Percentage:   record.Score.Spo2Percentage,
			SkinTempCelsius:  record.Score.SkinTempCelsius,
			CycleStrain:      record.Score.CycleStrain,
		}
	}

	return Recovery{
		CycleID:    record.CycleID,
		SleepID:    record.SleepID,
		UserID:     record.UserID,
		ScoreState: record.ScoreState,
		CreatedAt:  created,
		UpdatedAt:  updated,
		Score:      score,
	}, nil
}

func parseTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("timestamps must be RFC3339: %q", value)
}
