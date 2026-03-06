package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceFetchMergesProfileAndBody(t *testing.T) {
	fake := &fakeClient{
		responses: map[string]any{
			profilePath: profileResponse{
				UserID:         stringID("user-123"),
				FirstName:      "Ada",
				LastName:       "Lovelace",
				Email:          "ada@example.com",
				Locale:         "en_US",
				Timezone:       "America/New_York",
				MembershipTier: "pro",
			},
			bodyMeasurementPath: bodyMeasurementResponse{
				HeightCm:     floatPtr(170),
				WeightKg:     floatPtr(65),
				MaxHeartRate: intPtr(185),
				UpdatedAtRaw: "2026-03-04T00:00:00Z",
			},
		},
	}

	svc := &Service{client: fake}
	sum, err := svc.Fetch(context.Background())
	require.NoError(t, err)
	require.Equal(t, "user-123", sum.UserID)
	require.Equal(t, "Ada Lovelace", sum.Name)
	require.Equal(t, "ada@example.com", sum.Email)
	require.Equal(t, "America/New_York", sum.Timezone)
	require.NotNil(t, sum.HeightIn)
	require.NotNil(t, sum.WeightLb)
	require.NotNil(t, sum.UpdatedAt)
	require.Equal(t, 185, *sum.MaxHeartRate)
}

func TestServiceHandlesMetersAndPounds(t *testing.T) {
	fake := &fakeClient{
		responses: map[string]any{
			profilePath: profileResponse{UserID: stringID("user"), Email: "x@y.com"},
			bodyMeasurementPath: bodyMeasurementResponse{
				HeightMeter: floatPtr(1.8),
				WeightLbs:   floatPtr(150),
			},
		},
	}
	svc := &Service{client: fake}
	sum, err := svc.Fetch(context.Background())
	require.NoError(t, err)
	require.InDelta(t, 180, *sum.HeightCm, 0.001)
	require.InDelta(t, 68.038, *sum.WeightKg, 0.001)
}

type fakeClient struct {
	responses map[string]any
	err       error
}

func (f *fakeClient) GetJSON(ctx context.Context, path string, _ url.Values, dest any) error {
	if f.err != nil {
		return f.err
	}
	data, ok := f.responses[path]
	if !ok {
		return fmt.Errorf("unexpected path %s", path)
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, dest)
}

func floatPtr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}

func TestStringIDUnmarshalJSONHandlesNumbers(t *testing.T) {
	var resp profileResponse
	payload := []byte(`{"user_id":12345}`)
	require.NoError(t, json.Unmarshal(payload, &resp))
	require.Equal(t, "12345", resp.UserID.String())
}
