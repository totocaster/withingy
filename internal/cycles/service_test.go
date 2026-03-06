package cycles

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/api"
)

func TestServiceListParsesCycles(t *testing.T) {
	response := struct {
		Records   []cycleRecord `json:"records"`
		NextToken string        `json:"next_token"`
	}{
		Records: []cycleRecord{
			{
				ID:             1,
				UserID:         int64Ptr(10),
				Start:          "2026-03-03T00:00:00Z",
				End:            "2026-03-03T12:00:00Z",
				CreatedAt:      "2026-03-03T00:05:00Z",
				UpdatedAt:      "2026-03-03T13:00:00Z",
				ScoreState:     "SCORED",
				TimezoneOffset: "-05:00",
				Score: &cycleScore{
					Strain:           12.1,
					Kilojoule:        floatPtr(5000),
					AverageHeartRate: intPtr(120),
					MaxHeartRate:     intPtr(170),
				},
			},
		},
		NextToken: "cursor",
	}

	fake := &fakeClient{response: response}
	svc := &Service{client: fake}
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	opts := &api.ListOptions{Start: &start}

	result, err := svc.List(context.Background(), opts)
	require.NoError(t, err)
	require.Equal(t, cyclesPath, fake.lastPath)
	require.Equal(t, start.Format(time.RFC3339Nano), fake.lastQuery.Get("start"))
	require.Len(t, result.Cycles, 1)
	require.True(t, result.NextToken != "")
	c := result.Cycles[0]
	require.Equal(t, int64(1), c.ID)
	require.Equal(t, "SCORED", c.ScoreState)
	require.Equal(t, 12.1, c.Score.Strain)
	require.Equal(t, 120, *c.Score.AverageHeartRate)
}

func TestServiceListValidatesOptions(t *testing.T) {
	svc := &Service{client: &fakeClient{}}
	opts := &api.ListOptions{Limit: -1}
	_, err := svc.List(context.Background(), opts)
	require.Error(t, err)
}

func TestServiceGetFetchesCycle(t *testing.T) {
	rec := cycleRecord{
		ID:         99,
		Start:      "2026-03-02T00:00:00Z",
		End:        "2026-03-02T10:00:00Z",
		CreatedAt:  "2026-03-02T00:05:00Z",
		UpdatedAt:  "2026-03-02T10:05:00Z",
		ScoreState: "SCORED",
	}
	fake := &fakeClient{response: rec}
	svc := &Service{client: fake}
	cycle, err := svc.Get(context.Background(), "99")
	require.NoError(t, err)
	require.Equal(t, cyclesPath+"/99", fake.lastPath)
	require.Equal(t, int64(99), cycle.ID)
	require.False(t, cycle.End.IsZero())
}

func TestServiceHandlesMissingEnd(t *testing.T) {
	response := struct {
		Records   []cycleRecord `json:"records"`
		NextToken string        `json:"next_token"`
	}{
		Records: []cycleRecord{
			{
				ID:        5,
				Start:     "2026-03-05T00:00:00Z",
				End:       "",
				CreatedAt: "2026-03-05T00:05:00Z",
			},
		},
	}
	fake := &fakeClient{response: response}
	svc := &Service{client: fake}
	result, err := svc.List(context.Background(), &api.ListOptions{})
	require.NoError(t, err)
	require.Len(t, result.Cycles, 1)
	require.True(t, result.Cycles[0].End.IsZero())
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

func intPtr(v int) *int           { return &v }
func int64Ptr(v int64) *int64     { return &v }
func floatPtr(v float64) *float64 { return &v }
