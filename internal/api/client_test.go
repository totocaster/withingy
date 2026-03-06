package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/config"
	"github.com/toto/withingy/internal/tokens"
)

func TestClientRequiresToken(t *testing.T) {
	cfg := &config.Config{APIBaseURL: "https://api.example"}
	client := &Client{
		cfg:        cfg,
		baseURL:    cfg.APIBaseURL,
		store:      &fakeStore{token: nil},
		refresher:  &fakeRefresher{},
		httpClient: http.DefaultClient,
		now:        time.Now,
		sleepFn:    func(context.Context, time.Duration) error { return nil },
		backoff:    func(int) time.Duration { return 0 },
		max429:     0,
		userAgent:  "test",
	}

	err := client.GetJSON(context.Background(), "/foo", nil, nil)
	require.ErrorIs(t, err, ErrNotAuthenticated)
}

func TestClientSetsAuthorizationHeader(t *testing.T) {
	cfg := &config.Config{APIBaseURL: "http://example.com"}
	token := &tokens.Token{
		AccessToken: "abc",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer abc", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := newTestClient(cfg, server, &fakeStore{token: token}, &fakeRefresher{})
	err := client.GetJSON(context.Background(), "/data", nil, nil)
	require.NoError(t, err)
}

func TestClientRefreshesBeforeExpiry(t *testing.T) {
	cfg := &config.Config{APIBaseURL: "http://example.com"}
	expiring := &tokens.Token{
		AccessToken: "old",
		ExpiresAt:   time.Unix(1000, 0).Add(10 * time.Second),
	}
	refreshed := &tokens.Token{
		AccessToken: "new",
		ExpiresAt:   time.Unix(1000, 0).Add(2 * time.Hour),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer new", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	store := &fakeStore{token: expiring}
	refresher := &fakeRefresher{token: refreshed}
	client := newTestClient(cfg, server, store, refresher)
	client.now = func() time.Time { return time.Unix(1000, 0) }

	err := client.GetJSON(context.Background(), "/foo", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, refresher.calls)
}

func TestClientRefreshesOn401(t *testing.T) {
	cfg := &config.Config{APIBaseURL: "http://example.com"}
	token := &tokens.Token{
		AccessToken: "expired",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	refreshed := &tokens.Token{
		AccessToken: "fresh",
		ExpiresAt:   time.Now().Add(2 * time.Hour),
	}

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer fresh", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := newTestClient(cfg, server, &fakeStore{token: token}, &fakeRefresher{token: refreshed})
	err := client.GetJSON(context.Background(), "/foo", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, callCount)
}

func TestClientRetriesOn429(t *testing.T) {
	cfg := &config.Config{APIBaseURL: "http://example.com"}
	token := &tokens.Token{
		AccessToken: "abc",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := newTestClient(cfg, server, &fakeStore{token: token}, &fakeRefresher{})
	client.backoff = func(int) time.Duration { return 0 }
	err := client.GetJSON(context.Background(), "/foo", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 3, callCount)
}

func TestBuildURLMergesQuery(t *testing.T) {
	cfg := &config.Config{APIBaseURL: "https://example.com/api"}
	client := &Client{
		cfg:     cfg,
		baseURL: cfg.APIBaseURL,
	}
	u, err := client.buildURL("/v2/resource", url.Values{"a": []string{"1"}, "b": []string{"2"}})
	require.NoError(t, err)
	require.Equal(t, "https://example.com/api/v2/resource?a=1&b=2", u)
}

func newTestClient(cfg *config.Config, server *httptest.Server, store tokenStore, refresher tokenRefresher) *Client {
	return &Client{
		cfg:        cfg,
		baseURL:    server.URL,
		store:      store,
		refresher:  refresher,
		httpClient: server.Client(),
		now:        time.Now,
		sleepFn:    func(context.Context, time.Duration) error { return nil },
		backoff:    func(int) time.Duration { return 0 },
		max429:     3,
		userAgent:  "test",
	}
}

type fakeStore struct {
	token *tokens.Token
	err   error
}

func (f *fakeStore) Load() (*tokens.Token, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.token, nil
}

type fakeRefresher struct {
	token *tokens.Token
	err   error
	calls int
}

func (f *fakeRefresher) Refresh(ctx context.Context) (*tokens.Token, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.calls++
	if f.token == nil {
		return nil, errors.New("missing token")
	}
	return f.token, nil
}
