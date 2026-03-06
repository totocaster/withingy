package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/toto/withingy/internal/config"
	"github.com/toto/withingy/internal/tokens"
)

const (
	authPath      = "/oauth2_user/authorize2"
	signaturePath = "/v2/signature"
	tokenPath     = "/v2/oauth2"
)

var defaultScopes = []string{
	"user.metrics",
	"user.activity",
}

// Flow orchestrates OAuth interactions with the Withings API.
type Flow struct {
	cfg        *config.Config
	store      *tokens.Store
	httpClient *http.Client
	now        func() time.Time
}

// NewFlow returns a Flow with sane defaults.
func NewFlow(cfg *config.Config, store *tokens.Store) *Flow {
	return &Flow{
		cfg:   cfg,
		store: store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		now: time.Now,
	}
}

func (f *Flow) authURLBase() string {
	return strings.TrimRight(f.cfg.OAuthBaseURL, "/")
}

func (f *Flow) apiBaseURL() string {
	return strings.TrimRight(f.cfg.APIBaseURL, "/")
}

func (f *Flow) signatureEndpoint() string {
	return f.apiBaseURL() + signaturePath
}

func (f *Flow) tokenEndpoint() string {
	return f.apiBaseURL() + tokenPath
}

// BuildAuthURL creates the URL users should open to authorize the CLI.
func (f *Flow) BuildAuthURL(redirectURI, state string, _ *PKCE) (string, error) {
	base := f.authURLBase() + authPath
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse auth url: %w", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", f.cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", f.scopeString())
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeCode swaps an authorization code for tokens and persists them.
func (f *Flow) ExchangeCode(ctx context.Context, code, redirectURI string, _ *PKCE) (*tokens.Token, error) {
	if code == "" {
		return nil, errors.New("authorization code is empty")
	}

	nonce, err := f.getNonce(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("action", "requesttoken")
	form.Set("client_id", f.cfg.ClientID)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("nonce", nonce)
	form.Set("redirect_uri", redirectURI)
	form.Set("signature", signValues(f.cfg.ClientSecret, "requesttoken", f.cfg.ClientID, nonce))

	token, err := f.postToken(ctx, form)
	if err != nil {
		return nil, err
	}
	if err := f.store.Save(token); err != nil {
		return nil, err
	}
	return token, nil
}

// Refresh looks up the stored refresh token and obtains fresh access credentials.
func (f *Flow) Refresh(ctx context.Context) (*tokens.Token, error) {
	current, err := f.store.Load()
	if err != nil {
		return nil, err
	}
	if current == nil || current.RefreshToken == "" {
		return nil, errors.New("no refresh token available")
	}

	nonce, err := f.getNonce(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("action", "requesttoken")
	form.Set("client_id", f.cfg.ClientID)
	form.Set("grant_type", "refresh_token")
	form.Set("nonce", nonce)
	form.Set("refresh_token", current.RefreshToken)
	form.Set("signature", signValues(f.cfg.ClientSecret, "requesttoken", f.cfg.ClientID, nonce))

	token, err := f.postToken(ctx, form)
	if err != nil {
		return nil, err
	}
	if err := f.store.Save(token); err != nil {
		return nil, err
	}
	return token, nil
}

// Logout clears the locally cached tokens.
func (f *Flow) Logout(_ context.Context) error {
	return f.store.Clear()
}

func (f *Flow) getNonce(ctx context.Context) (string, error) {
	timestamp := strconv.FormatInt(f.now().Unix(), 10)
	form := url.Values{}
	form.Set("action", "getnonce")
	form.Set("client_id", f.cfg.ClientID)
	form.Set("timestamp", timestamp)
	form.Set("signature", signValues(f.cfg.ClientSecret, "getnonce", f.cfg.ClientID, timestamp))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.signatureEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build nonce request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute nonce request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body := ioReadLimited(resp.Body)
		return "", fmt.Errorf("nonce request failed: %d %s: %s", resp.StatusCode, resp.Status, body)
	}

	var payload struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
		Body   struct {
			Nonce string `json:"nonce"`
		} `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode nonce response: %w", err)
	}
	if payload.Status != 0 {
		if payload.Error != "" {
			return "", fmt.Errorf("nonce request failed: status %d: %s", payload.Status, payload.Error)
		}
		return "", fmt.Errorf("nonce request failed: status %d", payload.Status)
	}
	if strings.TrimSpace(payload.Body.Nonce) == "" {
		return "", errors.New("nonce response missing nonce")
	}
	return payload.Body.Nonce, nil
}

func (f *Flow) postToken(ctx context.Context, form url.Values) (*tokens.Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.tokenEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body := ioReadLimited(resp.Body)
		return nil, fmt.Errorf("token request failed: %d %s: %s", resp.StatusCode, resp.Status, body)
	}

	var payload struct {
		Status int           `json:"status"`
		Error  string        `json:"error"`
		Body   tokenResponse `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if payload.Status != 0 {
		if payload.Error != "" {
			return nil, fmt.Errorf("token request failed: status %d: %s", payload.Status, payload.Error)
		}
		return nil, fmt.Errorf("token request failed: status %d", payload.Status)
	}
	if payload.Body.AccessToken == "" || payload.Body.RefreshToken == "" {
		return nil, errors.New("token response missing access or refresh token")
	}

	expiresIn := payload.Body.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3 * 60 * 60
	}
	tokenType := strings.TrimSpace(payload.Body.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}

	return &tokens.Token{
		AccessToken:  payload.Body.AccessToken,
		RefreshToken: payload.Body.RefreshToken,
		TokenType:    tokenType,
		Scope:        parseScopes(payload.Body.Scope),
		ExpiresAt:    f.now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func (f *Flow) scopeString() string {
	if trimmed := strings.TrimSpace(f.cfg.Scopes); trimmed != "" {
		return trimmed
	}
	return strings.Join(defaultScopes, ",")
}

type tokenResponse struct {
	UserID       *int64 `json:"userid"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	CSRFToken    string `json:"csrf_token"`
}

func signValues(secret string, values ...string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join(values, ",")))
	return hex.EncodeToString(mac.Sum(nil))
}

func parseScopes(raw string) []string {
	normalized := strings.NewReplacer(",", " ", "\t", " ", "\n", " ").Replace(raw)
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(fields))
	scopes := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		scopes = append(scopes, field)
	}
	return scopes
}

func ioReadLimited(body io.Reader) string {
	const limit = 4 * 1024
	buf := make([]byte, limit)
	n, _ := body.Read(buf)
	return string(buf[:n])
}
