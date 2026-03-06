package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/config"
	"github.com/toto/withingy/internal/tokens"
)

func TestFlowExchangeCodeAcceptsQuotedUserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())

		switch r.URL.Path {
		case signaturePath:
			require.Equal(t, "getnonce", r.Form.Get("action"))
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"status":0,"body":{"nonce":"nonce-123"}}`))
			require.NoError(t, err)
		case tokenPath:
			require.Equal(t, "requesttoken", r.Form.Get("action"))
			require.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			require.Equal(t, "auth-code", r.Form.Get("code"))
			require.Equal(t, "nonce-123", r.Form.Get("nonce"))
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"status":0,"body":{"userid":"12345","access_token":"access-token","refresh_token":"refresh-token","token_type":"Bearer","expires_in":3600,"scope":"user.metrics,user.activity"}}`))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	store, err := tokens.NewStore(filepath.Join(t.TempDir(), "tokens.json"))
	require.NoError(t, err)

	flow := NewFlow(&config.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		APIBaseURL:   server.URL,
		OAuthBaseURL: "https://account.withings.com",
	}, store)
	flow.httpClient = server.Client()
	flow.now = func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	}

	token, err := flow.ExchangeCode(context.Background(), "auth-code", "http://127.0.0.1:8735/oauth/callback", nil)
	require.NoError(t, err)
	require.Equal(t, "access-token", token.AccessToken)
	require.Equal(t, "refresh-token", token.RefreshToken)
	require.Equal(t, "Bearer", token.TokenType)
	require.Equal(t, []string{"user.metrics", "user.activity"}, token.Scope)
	require.Equal(t, time.Unix(1_700_003_600, 0).UTC(), token.ExpiresAt)

	stored, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, token, stored)
}
