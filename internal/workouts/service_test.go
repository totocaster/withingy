package workouts

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/api"
)

func TestServiceListFiltersRequestedRange(t *testing.T) {
	start := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	inRangeStart := start.Add(2 * time.Hour)
	inRangeEnd := inRangeStart.Add(30 * time.Minute)

	fake := &fakeClient{response: struct {
		Series []workoutRecord `json:"series"`
	}{
		Series: []workoutRecord{
			{
				StartDate: start.Add(-time.Hour).Unix(),
				EndDate:   start.Add(time.Hour).Unix(),
				Date:      "2026-01-12",
			},
			{
				StartDate: inRangeStart.Unix(),
				EndDate:   inRangeEnd.Unix(),
				Date:      "2026-01-13",
				Data:      &workoutData{Steps: intPtr(1234)},
			},
			{
				StartDate: end.Unix(),
				EndDate:   end.Add(20 * time.Minute).Unix(),
				Date:      "2026-01-14",
			},
		},
	}}

	svc := &Service{client: fake, now: func() time.Time { return end }}
	result, err := svc.List(context.Background(), &api.ListOptions{Start: &start, End: &end})
	require.NoError(t, err)
	require.Equal(t, workoutsPath, fake.lastPath)
	require.Equal(t, strconv.FormatInt(start.Unix(), 10), fake.lastForm.Get("startdate"))
	require.Equal(t, strconv.FormatInt(end.Unix(), 10), fake.lastForm.Get("enddate"))
	require.Len(t, result.Workouts, 1)
	require.Equal(t, strconv.FormatInt(inRangeStart.Unix(), 10), result.Workouts[0].ID)
	require.Equal(t, "2026-01-13", result.Workouts[0].Date)
	require.NotNil(t, result.Workouts[0].Steps)
	require.Equal(t, 1234, *result.Workouts[0].Steps)
}

type fakeClient struct {
	response any
	err      error
	lastPath string
	lastForm url.Values
}

func (f *fakeClient) PostFormJSON(_ context.Context, path string, form url.Values, dest any) error {
	f.lastPath = path
	f.lastForm = form
	if f.err != nil {
		return f.err
	}
	data, err := json.Marshal(f.response)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func intPtr(v int) *int {
	return &v
}
