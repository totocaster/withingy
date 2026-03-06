package measures

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

func TestServiceListParsesAndFiltersMeasureGroups(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)
	category := CategoryReal

	fake := &fakeClient{response: struct {
		MeasureGroups []measureGroupRecord `json:"measuregrps"`
		More          int                  `json:"more"`
		Offset        int64                `json:"offset"`
	}{
		MeasureGroups: []measureGroupRecord{
			{
				GroupID:  102,
				Date:     time.Date(2026, 2, 2, 7, 0, 0, 0, time.UTC).Unix(),
				Category: CategoryReal,
				Measures: []measureRecord{
					{Value: 70550, Type: TypeWeight, Unit: -3},
					{Value: 182, Type: TypeFatRatio, Unit: -1},
				},
			},
			{
				GroupID:  101,
				Date:     time.Date(2026, 1, 31, 7, 0, 0, 0, time.UTC).Unix(),
				Category: CategoryReal,
				Measures: []measureRecord{
					{Value: 69000, Type: TypeWeight, Unit: -3},
				},
			},
			{
				GroupID:  103,
				Date:     time.Date(2026, 2, 2, 8, 0, 0, 0, time.UTC).Unix(),
				Category: CategoryObjective,
				Measures: []measureRecord{
					{Value: 68000, Type: TypeWeight, Unit: -3},
				},
			},
		},
		More:   1,
		Offset: 55,
	}}

	svc := &Service{client: fake, now: func() time.Time { return end }}
	result, err := svc.List(context.Background(), &Query{
		Range:    &api.ListOptions{Start: &start, End: &end},
		Types:    []int{TypeWeight, TypeFatRatio},
		Category: &category,
	})
	require.NoError(t, err)
	require.Equal(t, measurePath, fake.lastPath)
	require.Equal(t, "getmeas", fake.lastForm.Get("action"))
	require.Equal(t, strconv.FormatInt(start.Unix(), 10), fake.lastForm.Get("startdate"))
	require.Equal(t, strconv.FormatInt(end.Unix(), 10), fake.lastForm.Get("enddate"))
	require.Equal(t, strconv.Itoa(CategoryReal), fake.lastForm.Get("category"))
	require.Equal(t, "1,6", fake.lastForm.Get("meastype"))
	require.True(t, result.More)
	require.Equal(t, int64(55), result.Offset)
	require.Len(t, result.Groups, 1)
	require.Equal(t, int64(102), result.Groups[0].ID)
	require.Len(t, result.Groups[0].Measures, 2)
	require.Equal(t, "weight", result.Groups[0].Measures[0].Code)
	require.InDelta(t, 70.55, result.Groups[0].Measures[0].Value, 0.0001)
	require.Equal(t, "%", result.Groups[0].Measures[1].Unit)
	require.InDelta(t, 18.2, result.Groups[0].Measures[1].Value, 0.0001)
}

func TestServiceWeightListAndLatestSortNewestFirst(t *testing.T) {
	fake := &fakeClient{response: struct {
		MeasureGroups []measureGroupRecord `json:"measuregrps"`
	}{
		MeasureGroups: []measureGroupRecord{
			{
				GroupID:  1,
				Date:     time.Date(2026, 1, 10, 7, 0, 0, 0, time.UTC).Unix(),
				Category: CategoryReal,
				Measures: []measureRecord{{Value: 70000, Type: TypeWeight, Unit: -3}},
			},
			{
				GroupID:  2,
				Date:     time.Date(2026, 1, 12, 7, 0, 0, 0, time.UTC).Unix(),
				Category: CategoryReal,
				Measures: []measureRecord{{Value: 104600, Type: TypeWeight, Unit: -3}},
			},
		},
	}}

	svc := &Service{
		client: fake,
		now:    func() time.Time { return time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC) },
	}

	list, err := svc.WeightList(context.Background(), &api.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Weights, 2)
	require.Equal(t, int64(2), list.Weights[0].GroupID)
	require.Equal(t, 104.6, list.Weights[0].WeightKG)

	latest, err := svc.LatestWeight(context.Background())
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, int64(2), latest.GroupID)
	require.Equal(t, 104.6, latest.WeightKG)
}

func TestParseTypesAcceptsAliasesAndCodes(t *testing.T) {
	types, err := ParseTypes("weight, fat-ratio,76,weight")
	require.NoError(t, err)
	require.Equal(t, []int{TypeWeight, TypeFatRatio, TypeMuscleMass}, types)
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
