package httpx

import (
	"net/http"
	"time"

	"github.com/toto/withingy/internal/debuglog"
)

// NewTransport returns the shared transport settings used for Withings API calls.
func NewTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = false
	return transport
}

// NewClient constructs an HTTP client with the shared transport and optional debug logging.
func NewClient(timeout time.Duration, component string) *http.Client {
	transport := NewTransport()

	roundTripper := http.RoundTripper(transport)
	if logger := debuglog.Default(); logger.Enabled() {
		roundTripper = &loggingRoundTripper{
			component: component,
			next:      roundTripper,
			logger:    logger,
		}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: roundTripper,
	}
}

type loggingRoundTripper struct {
	component string
	next      http.RoundTripper
	logger    *debuglog.Logger
}

func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := l.next.RoundTrip(req)

	fields := map[string]any{
		"component":   l.component,
		"method":      req.Method,
		"scheme":      req.URL.Scheme,
		"host":        req.URL.Host,
		"path":        req.URL.Path,
		"duration_ms": time.Since(start).Milliseconds(),
	}
	if contentType := req.Header.Get("Content-Type"); contentType != "" {
		fields["content_type"] = contentType
	}

	if err != nil {
		fields["error"] = err.Error()
		l.logger.Log("http.round_trip.error", fields)
		return nil, err
	}

	fields["status_code"] = resp.StatusCode
	fields["proto"] = resp.Proto
	if requestID := resp.Header.Get("X-Request-Id"); requestID != "" {
		fields["request_id"] = requestID
	}
	if serverTime, ok := debuglog.ParseResponseDate(resp); ok {
		fields["response_date"] = serverTime.Format(time.RFC3339)
		fields["clock_skew_ms"] = serverTime.Sub(start.UTC()).Milliseconds()
	}
	l.logger.Log("http.round_trip", fields)
	return resp, nil
}
