package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ListOptions encapsulates common time-range and pagination flags shared by CLI commands.
type ListOptions struct {
	Start     *time.Time
	End       *time.Time
	Limit     int
	NextToken string
}

// Validate ensures the option values are consistent before being sent to the API.
func (o ListOptions) Validate() error {
	if o.Start != nil && o.End != nil && o.End.Before(*o.Start) {
		return fmt.Errorf("end must be greater than or equal to start")
	}
	if o.Limit < 0 {
		return fmt.Errorf("limit must be zero or greater")
	}
	return nil
}

// Apply appends the options to the provided url.Values (creating one if needed).
func (o ListOptions) Apply(values url.Values) url.Values {
	if values == nil {
		values = url.Values{}
	}
	if o.Start != nil {
		values.Set("start", formatTime(*o.Start))
	}
	if o.End != nil {
		values.Set("end", formatTime(*o.End))
	}
	if o.Limit > 0 {
		values.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.NextToken != "" {
		values.Set("nextToken", o.NextToken)
	}
	return values
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// Page represents a paginated response where records are accompanied by a next token.
type Page[T any] struct {
	Records   []T    `json:"records"`
	NextToken string `json:"next_token"`
}

// HasNext indicates whether a follow-up request with nextToken is required.
func (p Page[T]) HasNext() bool {
	return strings.TrimSpace(p.NextToken) != ""
}
