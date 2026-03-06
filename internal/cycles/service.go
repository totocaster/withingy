package cycles

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/toto/withingy/internal/api"
)

const cyclesPath = "/cycle"

// Service fetches cycle data from the WHOOP API.
type Service struct {
	client interface {
		GetJSON(ctx context.Context, path string, query url.Values, dest any) error
	}
}

// NewService returns a cycle Service backed by the shared API client.
func NewService(client *api.Client) *Service {
	return &Service{client: client}
}

// ListResult contains a page of cycles and the accompanying pagination cursor.
type ListResult struct {
	Cycles    []Cycle `json:"cycles"`
	NextToken string  `json:"next_token,omitempty"`
}

// Cycle captures the WHOOP cycle object with parsed timestamps.
type Cycle struct {
	ID             int64     `json:"id"`
	UserID         *int64    `json:"user_id,omitempty"`
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ScoreState     string    `json:"score_state"`
	TimezoneOffset string    `json:"timezone_offset"`
	Score          Score     `json:"score"`
}

// Score represents a cycle's strain metrics.
type Score struct {
	Strain           float64  `json:"strain"`
	Kilojoule        *float64 `json:"kilojoule,omitempty"`
	AverageHeartRate *int     `json:"average_heart_rate,omitempty"`
	MaxHeartRate     *int     `json:"max_heart_rate,omitempty"`
}

// List retrieves a page of cycles using the shared list options.
func (s *Service) List(ctx context.Context, opts *api.ListOptions) (*ListResult, error) {
	if opts == nil {
		opts = &api.ListOptions{}
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	query := opts.Apply(nil)
	var resp struct {
		Records   []cycleRecord `json:"records"`
		NextToken string        `json:"next_token"`
	}
	if err := s.client.GetJSON(ctx, cyclesPath, query, &resp); err != nil {
		return nil, fmt.Errorf("fetch cycles: %w", err)
	}
	cycles := make([]Cycle, len(resp.Records))
	for i, record := range resp.Records {
		cycle, err := convertRecord(record)
		if err != nil {
			return nil, err
		}
		cycles[i] = cycle
	}
	return &ListResult{Cycles: cycles, NextToken: strings.TrimSpace(resp.NextToken)}, nil
}

// Get returns a single cycle by ID.
func (s *Service) Get(ctx context.Context, id string) (*Cycle, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("cycle id is required")
	}
	path := fmt.Sprintf("%s/%s", cyclesPath, strings.TrimSpace(id))
	var record cycleRecord
	if err := s.client.GetJSON(ctx, path, nil, &record); err != nil {
		return nil, fmt.Errorf("fetch cycle %s: %w", id, err)
	}
	cycle, err := convertRecord(record)
	if err != nil {
		return nil, err
	}
	return &cycle, nil
}

type cycleRecord struct {
	ID             int64       `json:"id"`
	UserID         *int64      `json:"user_id"`
	Start          string      `json:"start"`
	End            string      `json:"end"`
	CreatedAt      string      `json:"created_at"`
	UpdatedAt      string      `json:"updated_at"`
	ScoreState     string      `json:"score_state"`
	TimezoneOffset string      `json:"timezone_offset"`
	Score          *cycleScore `json:"score"`
}

type cycleScore struct {
	Strain           float64  `json:"strain"`
	Kilojoule        *float64 `json:"kilojoule"`
	AverageHeartRate *int     `json:"average_heart_rate"`
	MaxHeartRate     *int     `json:"max_heart_rate"`
}

func convertRecord(rec cycleRecord) (Cycle, error) {
	start, err := parseTimestamp("start", rec.Start)
	if err != nil {
		return Cycle{}, err
	}
	end, err := parseTimestampAllowBlank(rec.End)
	if err != nil {
		return Cycle{}, fmt.Errorf("parse end: %w", err)
	}
	created, err := parseTimestampAllowBlank(rec.CreatedAt)
	if err != nil {
		return Cycle{}, fmt.Errorf("parse created_at: %w", err)
	}
	updated, err := parseTimestampAllowBlank(rec.UpdatedAt)
	if err != nil {
		return Cycle{}, fmt.Errorf("parse updated_at: %w", err)
	}

	score := Score{}
	if rec.Score != nil {
		score = Score{
			Strain:           rec.Score.Strain,
			Kilojoule:        rec.Score.Kilojoule,
			AverageHeartRate: rec.Score.AverageHeartRate,
			MaxHeartRate:     rec.Score.MaxHeartRate,
		}
	}

	return Cycle{
		ID:             rec.ID,
		UserID:         rec.UserID,
		Start:          start,
		End:            end,
		CreatedAt:      created,
		UpdatedAt:      updated,
		ScoreState:     rec.ScoreState,
		TimezoneOffset: rec.TimezoneOffset,
		Score:          score,
	}, nil
}

func parseTimestamp(field, value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, fmt.Errorf("cycle %s is missing", field)
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cycle %s must be RFC3339", field)
}

func parseTimestampAllowBlank(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("value %q must be RFC3339", value)
}
