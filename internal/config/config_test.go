package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/toto/withingy/internal/config"
	"github.com/toto/withingy/internal/paths"
)

func TestLoadFromFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "withingy")
	paths.SetConfigDirOverride(dir)
	t.Cleanup(func() { paths.SetConfigDirOverride("") })

	cfgPath := filepath.Join(dir, "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o700))
	err := os.WriteFile(cfgPath, []byte(`
client_id = "client-file"
client_secret = "secret-file"
api_base_url = "https://example.com/api"
oauth_base_url = "https://example.com/oauth"
redirect_uri = "http://127.0.0.1:9999/callback"
`), 0o600)
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "client-file", cfg.ClientID)
	require.Equal(t, "secret-file", cfg.ClientSecret)
	require.Equal(t, "https://example.com/api", cfg.APIBaseURL)
	require.Equal(t, "https://example.com/oauth", cfg.OAuthBaseURL)
	require.Equal(t, "http://127.0.0.1:9999/callback", cfg.RedirectURI)
}

func TestLoadEnvOverrides(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "withingy")
	paths.SetConfigDirOverride(dir)
	t.Cleanup(func() { paths.SetConfigDirOverride("") })
	t.Setenv("WITHINGY_CLIENT_ID", "env-id")
	t.Setenv("WITHINGY_CLIENT_SECRET", "env-secret")
	t.Setenv("WITHINGY_API_BASE_URL", "https://env/api")
	t.Setenv("WITHINGY_OAUTH_BASE_URL", "https://env/oauth")
	t.Setenv("WITHINGY_REDIRECT_URI", "http://127.0.0.1:1234/callback")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "env-id", cfg.ClientID)
	require.Equal(t, "env-secret", cfg.ClientSecret)
	require.Equal(t, "https://env/api", cfg.APIBaseURL)
	require.Equal(t, "https://env/oauth", cfg.OAuthBaseURL)
	require.Equal(t, "http://127.0.0.1:1234/callback", cfg.RedirectURI)
}

func TestLoadMissingClientID(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "withingy")
	paths.SetConfigDirOverride(dir)
	t.Cleanup(func() { paths.SetConfigDirOverride("") })
	t.Setenv("WITHINGY_CLIENT_SECRET", "secret-only")

	_, err := config.Load()
	require.Error(t, err)
}
