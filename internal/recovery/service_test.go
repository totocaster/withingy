package recovery

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/api"
)

func TestServiceListParsesRecovery(t *testing.T) {
	response := struct {
		Records   []recoveryRecord `json:"records"`
		NextToken string           `json:"next_token"`
	}{
		Records: []recoveryRecord{
			{
				CycleID:    100,
				SleepID:    "sleep-1",
				UserID:     int64Ptr(5),
				ScoreState: "SCORED",
				CreatedAt:  "2026-03-04T00:00:00Z",
				UpdatedAt:  "2026-03-04T00:10:00Z",
				Score: &recoveryScore{
					UserCalibrating:  true,
					RecoveryScore:    floatPtr(75),
					RestingHeartRate: floatPtr(42),
					HRVRMSSDMilli:    floatPtr(110.5),
				},
			},
		},
		NextToken: "cursor",
	}

	fake := &fakeClient{response: response}
	svc := &Service{client: fake}
	opts := &api.ListOptions{}

	result, err := svc.List(context.Background(), opts)
	require.NoError(t, err)
	require.Equal(t, recoveryPath, fake.lastPath)
	require.Len(t, result.Recoveries, 1)
	rec := result.Recoveries[0]
	require.Equal(t, int64(100), rec.CycleID)
	require.Equal(t, "sleep-1", rec.SleepID)
	require.Equal(t, 75.0, *rec.Score.RecoveryScore)
	require.True(t, rec.Score.UserCalibrating)
}

func TestServiceGetByCycle(t *testing.T) {
	record := recoveryRecord{CycleID: 5, SleepID: "s", ScoreState: "SCORED"}
	fake := &fakeClient{response: record}
	svc := &Service{client: fake}
	rec, err := svc.GetByCycle(context.Background(), "5")
	require.NoError(t, err)
	require.Equal(t, "/cycle/5/recovery", fake.lastPath)
	require.Equal(t, int64(5), rec.CycleID)
}

type fakeClient struct {
	response  any
	err       error
	lastPath  string
	lastQuery url.Values
}

func (f *fakeClient) GetJSON(ctx context.Context, path string, query url.Values, dest any) error {
	f.lastPath = path
	f.lastQuery = query
	if f.err != nil {
		return f.err
	}
	data, err := json.Marshal(f.response)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func int64Ptr(v int64) *int64     { return &v }
func floatPtr(v float64) *float64 { return &v }
