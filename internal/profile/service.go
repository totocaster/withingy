package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/toto/withingy/internal/api"
)

const (
	profilePath         = "/user/profile/basic"
	bodyMeasurementPath = "/user/measurement/body"
)

// Summary contains user profile basics plus body measurements.
type Summary struct {
	UserID         string     `json:"user_id"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Locale         string     `json:"locale"`
	Timezone       string     `json:"timezone"`
	MembershipTier string     `json:"membership_tier"`
	HeightCm       *float64   `json:"height_cm,omitempty"`
	HeightIn       *float64   `json:"height_in,omitempty"`
	WeightKg       *float64   `json:"weight_kg,omitempty"`
	WeightLb       *float64   `json:"weight_lb,omitempty"`
	MaxHeartRate   *int       `json:"max_heart_rate,omitempty"`
	UpdatedAt      *time.Time `json:"measurement_updated_at,omitempty"`
}

// Service fetches profile data.
type Service struct {
	client interface {
		GetJSON(ctx context.Context, path string, query url.Values, dest any) error
	}
}

// NewService constructs a profile Service.
func NewService(client *api.Client) *Service {
	return &Service{client: client}
}

// Fetch retrieves and combines the profile + body measurement data.
func (s *Service) Fetch(ctx context.Context) (*Summary, error) {
	var basic profileResponse
	if err := s.client.GetJSON(ctx, profilePath, nil, &basic); err != nil {
		return nil, fmt.Errorf("fetch profile: %w", err)
	}
	var measurements bodyMeasurementResponse
	if err := s.client.GetJSON(ctx, bodyMeasurementPath, nil, &measurements); err != nil {
		return nil, fmt.Errorf("fetch body measurement: %w", err)
	}

	return mergeProfile(&basic, &measurements), nil
}

type profileResponse struct {
	UserID         stringID `json:"user_id"`
	FirstName      string   `json:"first_name"`
	LastName       string   `json:"last_name"`
	Email          string   `json:"email"`
	Locale         string   `json:"locale"`
	Timezone       string   `json:"timezone"`
	MembershipTier string   `json:"membership_tier"`
	DisplayName    string   `json:"display_name"`
}

type bodyMeasurementResponse struct {
	HeightCm     *float64 `json:"height_cm"`
	HeightMeter  *float64 `json:"height_meter"`
	HeightInches *float64 `json:"height_in"`
	WeightKg     *float64 `json:"weight_kg"`
	WeightLbs    *float64 `json:"weight_lbs"`
	MaxHeartRate *int     `json:"max_heart_rate"`
	UpdatedAtRaw string   `json:"updated_at"`
}

func mergeProfile(profile *profileResponse, body *bodyMeasurementResponse) *Summary {
	summary := &Summary{
		UserID:         profile.UserID.String(),
		Name:           displayName(profile),
		Email:          profile.Email,
		Locale:         profile.Locale,
		Timezone:       profile.Timezone,
		MembershipTier: profile.MembershipTier,
	}

	if body != nil {
		summary.HeightCm = firstFloat(body.HeightCm, convertMetersToCm(body.HeightMeter))
		summary.HeightIn = firstFloat(body.HeightInches, convertCmToIn(summary.HeightCm))
		summary.WeightKg = firstFloat(body.WeightKg, convertLbsToKg(body.WeightLbs))
		summary.WeightLb = firstFloat(body.WeightLbs, convertKgToLbs(summary.WeightKg))
		summary.MaxHeartRate = body.MaxHeartRate
		if ts := parseTime(body.UpdatedAtRaw); ts != nil {
			summary.UpdatedAt = ts
		}
	}

	return summary
}

func displayName(resp *profileResponse) string {
	if resp.DisplayName != "" {
		return resp.DisplayName
	}
	name := strings.TrimSpace(strings.Join([]string{resp.FirstName, resp.LastName}, " "))
	if name != "" {
		return name
	}
	return resp.Email
}

func firstFloat(values ...*float64) *float64 {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func convertMetersToCm(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cm := *value * 100
	return &cm
}

func convertCmToIn(value *float64) *float64 {
	if value == nil {
		return nil
	}
	in := *value / 2.54
	return &in
}

func convertLbsToKg(value *float64) *float64 {
	if value == nil {
		return nil
	}
	kg := *value / 2.20462262
	return &kg
}

func convertKgToLbs(value *float64) *float64 {
	if value == nil {
		return nil
	}
	lbs := *value * 2.20462262
	return &lbs
}

func parseTime(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02T15:04:05", raw); err == nil {
		return &t
	}
	return nil
}

// MarshalJSON ensures JSON omits nil fields (already handled by omitempty) but keeping custom conversions.
func (s *Summary) MarshalJSON() ([]byte, error) {
	type Alias Summary
	return json.Marshal((*Alias)(s))
}

type stringID string

func (id *stringID) UnmarshalJSON(data []byte) error {
	if id == nil {
		return fmt.Errorf("stringID: nil receiver")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}
	if data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*id = stringID(raw)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return err
	}
	*id = stringID(num.String())
	return nil
}

func (id stringID) String() string {
	return string(id)
}
