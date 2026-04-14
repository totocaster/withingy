package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/activity"
	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/config"
	"github.com/toto/withingy/internal/paths"
	"github.com/toto/withingy/internal/tokens"
)

var (
	diagGatherFn      = gatherDiagnostics
	diagHealthCheckFn = defaultDiagHealthCheck
	diagAPIProbeFn    = defaultDiagAPIProbe
)

func init() {
	rootCmd.AddCommand(diagCmd)
	diagCmd.Flags().Bool("text", false, "Human-readable output")
}

var diagCmd = &cobra.Command{
	Use:   "diag",
	Short: "Print configuration, token, and API health diagnostics",
	RunE: func(cmd *cobra.Command, args []string) error {
		textMode, err := cmd.Flags().GetBool("text")
		if err != nil {
			return err
		}
		report, err := diagGatherFn(cmd.Context())
		if err != nil {
			return err
		}
		if textMode {
			fmt.Fprintln(cmd.OutOrStdout(), formatDiagText(report))
			return nil
		}
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return nil
	},
}

type diagReport struct {
	Clock  diagClockInfo  `json:"clock"`
	Config diagConfigInfo `json:"config"`
	Tokens diagTokenInfo  `json:"tokens"`
	API    diagAPIInfo    `json:"api"`
}

type diagClockInfo struct {
	LocalNow     string `json:"local_now"`
	UTCNow       string `json:"utc_now"`
	TimezoneName string `json:"timezone_name"`
	OffsetMin    int    `json:"offset_min"`
}

type diagConfigInfo struct {
	Path            string `json:"path,omitempty"`
	Exists          bool   `json:"exists"`
	LastModified    string `json:"last_modified,omitempty"`
	ClientIDSet     bool   `json:"client_id_set"`
	ClientSecretSet bool   `json:"client_secret_set"`
	APIBaseURL      string `json:"api_base_url,omitempty"`
	OAuthBaseURL    string `json:"oauth_base_url,omitempty"`
	RedirectURI     string `json:"redirect_uri,omitempty"`
	Scopes          string `json:"scopes,omitempty"`
	Error           string `json:"error,omitempty"`
}

type diagTokenInfo struct {
	Path            string   `json:"path,omitempty"`
	Exists          bool     `json:"exists"`
	LastModified    string   `json:"last_modified,omitempty"`
	Scope           []string `json:"scope,omitempty"`
	ExpiresAt       string   `json:"expires_at,omitempty"`
	ExpiresIn       string   `json:"expires_in,omitempty"`
	Status          string   `json:"status"`
	HasRefreshToken bool     `json:"has_refresh_token"`
	Error           string   `json:"error,omitempty"`
}

type diagAPIInfo struct {
	Status     string `json:"status"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
	CheckedAt  string `json:"checked_at,omitempty"`
	Error      string `json:"error,omitempty"`
	APIBaseURL string `json:"api_base_url,omitempty"`
}

func gatherDiagnostics(ctx context.Context) (*diagReport, error) {
	clockInfo := gatherClockInfo()
	cfgInfo := gatherConfigInfo()
	tokenInfo := gatherTokenInfo()

	apiInfo := diagAPIInfo{APIBaseURL: cfgInfo.APIBaseURL}
	if latency, err := diagHealthCheckFn(ctx); err != nil {
		apiInfo.Status = "error"
		apiInfo.Error = err.Error()
	} else {
		apiInfo.Status = "ok"
		apiInfo.LatencyMS = latency.Milliseconds()
		apiInfo.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}

	return &diagReport{
		Clock:  clockInfo,
		Config: cfgInfo,
		Tokens: tokenInfo,
		API:    apiInfo,
	}, nil
}

func gatherClockInfo() diagClockInfo {
	now := time.Now()
	zoneName, offsetSec := now.Zone()
	return diagClockInfo{
		LocalNow:     now.Format(time.RFC3339),
		UTCNow:       now.UTC().Format(time.RFC3339),
		TimezoneName: zoneName,
		OffsetMin:    offsetSec / 60,
	}
}

func gatherConfigInfo() diagConfigInfo {
	info := diagConfigInfo{}
	if path, err := paths.ConfigFile(); err != nil {
		info.Error = fmt.Sprintf("resolve config path: %v", err)
	} else {
		info.Path = path
		if stat, err := os.Stat(path); err == nil {
			info.Exists = true
			info.LastModified = stat.ModTime().UTC().Format(time.RFC3339)
		} else if !errors.Is(err, os.ErrNotExist) {
			info.Error = fmt.Sprintf("stat config: %v", err)
		}
	}

	cfg, err := config.LoadAllowEmpty()
	if err != nil {
		if info.Error == "" {
			info.Error = err.Error()
		}
		return info
	}

	info.ClientIDSet = strings.TrimSpace(cfg.ClientID) != ""
	info.ClientSecretSet = strings.TrimSpace(cfg.ClientSecret) != ""
	info.APIBaseURL = strings.TrimSpace(cfg.APIBaseURL)
	info.OAuthBaseURL = strings.TrimSpace(cfg.OAuthBaseURL)
	info.RedirectURI = strings.TrimSpace(cfg.RedirectURI)
	info.Scopes = strings.TrimSpace(cfg.Scopes)
	return info
}

func gatherTokenInfo() diagTokenInfo {
	info := diagTokenInfo{Status: "missing"}
	store, err := tokens.NewStore("")
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.Path = store.Path()
	if stat, err := os.Stat(info.Path); err == nil {
		info.Exists = true
		info.LastModified = stat.ModTime().UTC().Format(time.RFC3339)
	} else if !errors.Is(err, os.ErrNotExist) {
		info.Error = fmt.Sprintf("stat tokens: %v", err)
	}

	token, err := store.Load()
	if err != nil {
		info.Error = err.Error()
		return info
	}
	if token == nil {
		return info
	}

	info.Status = "valid"
	if time.Until(token.ExpiresAt) <= 0 {
		info.Status = "expired"
	}
	info.ExpiresAt = token.ExpiresAt.Local().Format(time.RFC3339)
	info.ExpiresIn = humanizeDuration(time.Until(token.ExpiresAt))
	info.HasRefreshToken = strings.TrimSpace(token.RefreshToken) != ""
	if len(token.Scope) > 0 {
		scopeCopy := append([]string{}, token.Scope...)
		sort.Strings(scopeCopy)
		info.Scope = scopeCopy
	}
	return info
}

func defaultDiagHealthCheck(ctx context.Context) (time.Duration, error) {
	client, err := apiClientFactory()
	if err != nil {
		return 0, err
	}
	start := time.Now()
	if err := diagAPIProbeFn(ctx, client); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

func defaultDiagAPIProbe(ctx context.Context, client *api.Client) error {
	service := activity.NewService(client)
	_, err := service.List(ctx, todayRangeOptions(1))
	return err
}

func formatDiagText(report *diagReport) string {
	if report == nil {
		return "No diagnostics collected."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Clock\n")
	fmt.Fprintf(&b, "  Local Now: %s\n", safeValue(report.Clock.LocalNow))
	fmt.Fprintf(&b, "  UTC Now: %s\n", safeValue(report.Clock.UTCNow))
	fmt.Fprintf(&b, "  Timezone: %s\n", safeValue(report.Clock.TimezoneName))
	fmt.Fprintf(&b, "  Offset Minutes: %d\n", report.Clock.OffsetMin)

	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "Config\n")
	fmt.Fprintf(&b, "  Path: %s\n", safeValue(report.Config.Path))
	fmt.Fprintf(&b, "  Exists: %s\n", formatBool(report.Config.Exists))
	if report.Config.LastModified != "" {
		fmt.Fprintf(&b, "  Last Modified: %s\n", report.Config.LastModified)
	}
	fmt.Fprintf(&b, "  Client ID set: %s\n", formatBool(report.Config.ClientIDSet))
	fmt.Fprintf(&b, "  Client Secret set: %s\n", formatBool(report.Config.ClientSecretSet))
	if report.Config.APIBaseURL != "" {
		fmt.Fprintf(&b, "  API Base URL: %s\n", report.Config.APIBaseURL)
	}
	if report.Config.OAuthBaseURL != "" {
		fmt.Fprintf(&b, "  OAuth Base URL: %s\n", report.Config.OAuthBaseURL)
	}
	if report.Config.RedirectURI != "" {
		fmt.Fprintf(&b, "  Redirect URI: %s\n", report.Config.RedirectURI)
	}
	if report.Config.Scopes != "" {
		fmt.Fprintf(&b, "  Scopes: %s\n", report.Config.Scopes)
	}
	if report.Config.Error != "" {
		fmt.Fprintf(&b, "  Error: %s\n", report.Config.Error)
	}

	fmt.Fprintf(&b, "\nTokens\n")
	fmt.Fprintf(&b, "  Path: %s\n", safeValue(report.Tokens.Path))
	fmt.Fprintf(&b, "  Exists: %s\n", formatBool(report.Tokens.Exists))
	if report.Tokens.LastModified != "" {
		fmt.Fprintf(&b, "  Last Modified: %s\n", report.Tokens.LastModified)
	}
	fmt.Fprintf(&b, "  Status: %s\n", report.Tokens.Status)
	if len(report.Tokens.Scope) > 0 {
		fmt.Fprintf(&b, "  Scopes: %s\n", strings.Join(report.Tokens.Scope, " "))
	}
	if report.Tokens.ExpiresAt != "" {
		fmt.Fprintf(&b, "  Expires At: %s\n", report.Tokens.ExpiresAt)
		fmt.Fprintf(&b, "  Expires In: %s\n", report.Tokens.ExpiresIn)
	}
	fmt.Fprintf(&b, "  Refresh Token Stored: %s\n", formatBool(report.Tokens.HasRefreshToken))
	if report.Tokens.Error != "" {
		fmt.Fprintf(&b, "  Error: %s\n", report.Tokens.Error)
	}

	fmt.Fprintf(&b, "\nAPI\n")
	fmt.Fprintf(&b, "  Status: %s\n", safeValue(report.API.Status))
	if report.API.LatencyMS > 0 {
		fmt.Fprintf(&b, "  Latency: %dms\n", report.API.LatencyMS)
	}
	if report.API.CheckedAt != "" {
		fmt.Fprintf(&b, "  Checked At: %s\n", report.API.CheckedAt)
	}
	if report.API.APIBaseURL != "" {
		fmt.Fprintf(&b, "  Base URL: %s\n", report.API.APIBaseURL)
	}
	if report.API.Error != "" {
		fmt.Fprintf(&b, "  Error: %s\n", report.API.Error)
	}

	return strings.TrimSpace(b.String())
}
