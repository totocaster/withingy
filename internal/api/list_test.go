package api

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListOptionsValidate(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(-time.Hour)
	opts := ListOptions{Start: &start, End: &end}
	err := opts.Validate()
	require.Error(t, err)

	opts = ListOptions{Limit: -1}
	err = opts.Validate()
	require.Error(t, err)

	end = start.Add(24 * time.Hour)
	opts = ListOptions{Start: &start, End: &end, Limit: 50}
	require.NoError(t, opts.Validate())
}

func TestListOptionsApply(t *testing.T) {
	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
	end := start.Add(24 * time.Hour)
	opts := ListOptions{
		Start:     &start,
		End:       &end,
		Limit:     25,
		NextToken: "abc",
	}
	values := opts.Apply(nil)
	require.Equal(t, url.Values{
		"start":     []string{start.UTC().Format(time.RFC3339Nano)},
		"end":       []string{end.UTC().Format(time.RFC3339Nano)},
		"limit":     []string{"25"},
		"nextToken": []string{"abc"},
	}, values)
}

func TestPageHasNext(t *testing.T) {
	page := Page[int]{NextToken: "  "}
	require.False(t, page.HasNext())
	page.NextToken = "token"
	require.True(t, page.HasNext())
}
