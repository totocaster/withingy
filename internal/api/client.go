package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/toto/withingy/internal/auth"
	"github.com/toto/withingy/internal/config"
	"github.com/toto/withingy/internal/debuglog"
	"github.com/toto/withingy/internal/httpx"
	"github.com/toto/withingy/internal/tokens"
)

const (
	tokenRefreshLeeway    = time.Minute
	defaultRequestTimeout = 30 * time.Second
	defaultMax429Retry    = 3
	defaultUserAgent      = "withingy/0.1"
)

// ErrNotAuthenticated is returned when no OAuth token is available locally.
var ErrNotAuthenticated = errors.New("withingy: not authenticated; run 'withingy auth login'")

type tokenStore interface {
	Load() (*tokens.Token, error)
}

type tokenRefresher interface {
	Refresh(ctx context.Context) (*tokens.Token, error)
}

// Client handles authenticated requests against the Withings API.
type Client struct {
	cfg        *config.Config
	store      tokenStore
	refresher  tokenRefresher
	httpClient *http.Client

	tokenMu sync.Mutex
	token   *tokens.Token

	now       func() time.Time
	sleepFn   func(context.Context, time.Duration) error
	backoff   func(int) time.Duration
	max429    int
	baseURL   string
	userAgent string
}

type withingsEnvelope struct {
	Status int             `json:"status"`
	Error  string          `json:"error"`
	Body   json.RawMessage `json:"body"`
}

// ClientOption customizes a Client during construction.
type ClientOption func(*Client)

// WithUserAgent overrides the default HTTP User-Agent header.
func WithUserAgent(agent string) ClientOption {
	return func(c *Client) {
		if trimmed := strings.TrimSpace(agent); trimmed != "" {
			c.userAgent = trimmed
		}
	}
}

// NewClient constructs a Client using the supplied config and token store.
func NewClient(cfg *config.Config, store *tokens.Store, opts ...ClientOption) *Client {
	client := &Client{
		cfg:        cfg,
		store:      store,
		refresher:  auth.NewFlow(cfg, store),
		httpClient: httpx.NewClient(defaultRequestTimeout, "api"),
		now:        time.Now,
		sleepFn:    sleepWithContext,
		backoff:    defaultBackoff,
		max429:     defaultMax429Retry,
		baseURL:    strings.TrimRight(cfg.APIBaseURL, "/"),
		userAgent:  defaultUserAgent,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

// GetJSON performs a GET request against the given API path and unmarshals the JSON response.
func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, dest any) error {
	resp, err := c.do(ctx, http.MethodGet, path, query, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if dest == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

// PostFormJSON performs an authenticated form-encoded POST and unmarshals the Withings `body` payload.
func (c *Client) PostFormJSON(ctx context.Context, path string, form url.Values, dest any) error {
	token, err := c.ensureValidToken(ctx)
	if err != nil {
		return err
	}

	fullURL, err := c.buildURL(path, nil)
	if err != nil {
		return err
	}

	encoded := ""
	if form != nil {
		encoded = form.Encode()
	}

	refreshed := false
	for attempt := 0; attempt <= c.max429; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(encoded))
		if err != nil {
			return err
		}
		headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
		c.setHeaders(req, token, headers)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusUnauthorized && !refreshed {
			resp.Body.Close()
			token, err = c.forceRefresh(ctx)
			if err != nil {
				return err
			}
			refreshed = true
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < c.max429 {
			resp.Body.Close()
			delay := c.backoff(attempt)
			if err := c.sleepFn(ctx, delay); err != nil {
				return err
			}
			continue
		}

		if resp.StatusCode >= 400 {
			bodyText := readLimited(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("withingy api error %d %s: %s", resp.StatusCode, http.StatusText(resp.StatusCode), bodyText)
		}

		defer resp.Body.Close()
		var envelope withingsEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			return fmt.Errorf("decode withings response: %w", err)
		}
		if envelope.Status != 0 {
			msg := strings.TrimSpace(envelope.Error)
			if msg == "" && len(envelope.Body) > 0 {
				msg = strings.TrimSpace(string(envelope.Body))
			}
			if msg == "" {
				msg = "unknown error"
			}
			return fmt.Errorf("withings api status %d: %s", envelope.Status, msg)
		}
		if dest == nil || len(envelope.Body) == 0 {
			return nil
		}
		if err := json.Unmarshal(envelope.Body, dest); err != nil {
			return fmt.Errorf("decode withings body: %w", err)
		}
		return nil
	}

	return errors.New("withingy api error: exhausted retries after 429 responses")
}

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	body io.Reader,
	extraHeaders map[string]string,
) (*http.Response, error) {
	token, err := c.ensureValidToken(ctx)
	if err != nil {
		return nil, err
	}

	fullURL, err := c.buildURL(path, query)
	if err != nil {
		return nil, err
	}

	refreshed := false
	for attempt := 0; attempt <= c.max429; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
		if err != nil {
			return nil, err
		}
		c.setHeaders(req, token, extraHeaders)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized && !refreshed {
			resp.Body.Close()
			token, err = c.forceRefresh(ctx)
			if err != nil {
				return nil, err
			}
			refreshed = true
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < c.max429 {
			resp.Body.Close()
			delay := c.backoff(attempt)
			if err := c.sleepFn(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		if resp.StatusCode >= 400 {
			bodyText := readLimited(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("withingy api error %d %s: %s", resp.StatusCode, http.StatusText(resp.StatusCode), bodyText)
		}

		return resp, nil
	}

	return nil, errors.New("withingy api error: exhausted retries after 429 responses")
}

func (c *Client) setHeaders(req *http.Request, token *tokens.Token, extra map[string]string) {
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if extra != nil {
		for k, v := range extra {
			req.Header.Set(k, v)
		}
	}
}

func (c *Client) ensureValidToken(ctx context.Context) (*tokens.Token, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token == nil {
		token, err := c.store.Load()
		if err != nil {
			return nil, err
		}
		if token == nil {
			return nil, ErrNotAuthenticated
		}
		c.token = token
	}

	remaining := c.token.ExpiresAt.Sub(c.now())
	debuglog.Default().Log("api.token.check", map[string]any{
		"access_token_fp":   debuglog.Fingerprint(c.token.AccessToken),
		"refresh_token_set": strings.TrimSpace(c.token.RefreshToken) != "",
		"expires_at":        c.token.ExpiresAt.UTC().Format(time.RFC3339),
		"remaining_ms":      remaining.Milliseconds(),
		"refresh_leeway_ms": tokenRefreshLeeway.Milliseconds(),
		"refresh_needed":    remaining <= tokenRefreshLeeway,
	})
	if remaining <= tokenRefreshLeeway {
		debuglog.Default().Log("api.token.refresh.start", map[string]any{"reason": "expiry_leeway"})
		token, err := c.refresher.Refresh(ctx)
		if err != nil {
			debuglog.Default().Log("api.token.refresh.error", map[string]any{
				"reason": "expiry_leeway",
				"error":  err.Error(),
			})
			return nil, err
		}
		c.token = token
		debuglog.Default().Log("api.token.refresh.success", map[string]any{
			"reason":            "expiry_leeway",
			"access_token_fp":   debuglog.Fingerprint(token.AccessToken),
			"refresh_token_set": strings.TrimSpace(token.RefreshToken) != "",
			"expires_at":        token.ExpiresAt.UTC().Format(time.RFC3339),
			"remaining_ms":      token.ExpiresAt.Sub(c.now()).Milliseconds(),
		})
	}

	return c.token, nil
}

func (c *Client) forceRefresh(ctx context.Context) (*tokens.Token, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	debuglog.Default().Log("api.token.refresh.start", map[string]any{"reason": "http_401"})
	token, err := c.refresher.Refresh(ctx)
	if err != nil {
		debuglog.Default().Log("api.token.refresh.error", map[string]any{
			"reason": "http_401",
			"error":  err.Error(),
		})
		return nil, err
	}
	c.token = token
	debuglog.Default().Log("api.token.refresh.success", map[string]any{
		"reason":            "http_401",
		"access_token_fp":   debuglog.Fingerprint(token.AccessToken),
		"refresh_token_set": strings.TrimSpace(token.RefreshToken) != "",
		"expires_at":        token.ExpiresAt.UTC().Format(time.RFC3339),
		"remaining_ms":      token.ExpiresAt.Sub(c.now()).Milliseconds(),
	})
	return token, nil
}

func (c *Client) buildURL(path string, query url.Values) (string, error) {
	trimmed := strings.TrimLeft(path, "/")
	full := c.baseURL + "/" + trimmed
	u, err := url.Parse(full)
	if err != nil {
		return "", err
	}
	if query != nil {
		q := u.Query()
		for key, values := range query {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func defaultBackoff(attempt int) time.Duration {
	base := 200 * time.Millisecond
	return base * time.Duration(1<<attempt)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func readLimited(r io.Reader) string {
	const limit = 4 * 1024
	buf := make([]byte, limit)
	n, _ := r.Read(buf)
	return strings.TrimSpace(string(buf[:n]))
}
