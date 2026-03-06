package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/auth"
	"github.com/toto/withingy/internal/config"
	"github.com/toto/withingy/internal/paths"
	"github.com/toto/withingy/internal/tokens"
)

const loginTimeout = 5 * time.Minute

var (
	stateGenerator  = randomState
	openBrowserFunc = auth.OpenBrowser
)

const configTemplate = `# withingy configuration scaffold.
# Replace the empty client ID/secret with your Withings OAuth credentials.
client_id = ""
client_secret = ""
api_base_url = "https://wbsapi.withings.net"
oauth_base_url = "https://account.withings.com"
redirect_uri = "http://127.0.0.1:8735/oauth/callback"
scopes = "user.metrics,user.activity"
`

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)

	authLoginCmd.Flags().Bool("no-browser", false, "Do not automatically open the authorization URL")
	authLoginCmd.Flags().Bool("manual", false, "Skip local callback server and paste the redirected URL manually")
	authLoginCmd.Flags().String("code", "", "Authorization code or redirect URL to exchange directly")
	authStatusCmd.Flags().Bool("text", true, "Render output as human-readable text (default)")
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Withings authentication",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Launch OAuth flow and store tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), loginTimeout)
		defer cancel()

		noBrowser, err := cmd.Flags().GetBool("no-browser")
		if err != nil {
			return err
		}
		manualMode, err := cmd.Flags().GetBool("manual")
		if err != nil {
			return err
		}
		codeFlag, err := cmd.Flags().GetString("code")
		if err != nil {
			return err
		}

		templateCreated, templatePath, err := ensureConfigTemplate()
		if err != nil {
			return err
		}
		if templateCreated {
			cmd.Printf("Created configuration template at %s. Update it with your Withings OAuth client ID and secret.\n", templatePath)
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := tokens.NewStore("")
		if err != nil {
			return err
		}
		flow := auth.NewFlow(cfg, store)

		pkce, err := auth.NewPKCE()
		if err != nil {
			return fmt.Errorf("generate PKCE: %w", err)
		}
		state, err := stateGenerator()
		if err != nil {
			return fmt.Errorf("generate state: %w", err)
		}

		var code string
		var manualInputState string
		redirectURI := cfg.RedirectURI

		switch {
		case codeFlag != "":
			code, manualInputState, err = parseAuthInput(codeFlag)
			if err != nil {
				return err
			}
			manualMode = true
		case manualMode:
			cmd.Println("Manual mode enabled. After approving access, copy the entire redirect URL (with code and state) and paste it when prompted.")
			authURL, err := flow.BuildAuthURL(redirectURI, state, pkce)
			if err != nil {
				return err
			}
			printAuthURL(cmd, authURL)
			if !noBrowser {
				if err := openBrowserFunc(authURL); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to open browser automatically: %v\n", err)
					fmt.Fprintf(cmd.ErrOrStderr(), "Please open the URL above manually.\n")
				}
			}
			code, manualInputState, err = promptAuthInput(cmd)
			if err != nil {
				return err
			}
		default:
			redirectURI, code, err = runAutomaticLogin(ctx, cmd, flow, cfg.RedirectURI, state, pkce, noBrowser)
			if err != nil {
				return err
			}
		}

		if manualMode {
			if manualInputState == "" {
				return errors.New("state missing in pasted URL; please paste the full redirect URL")
			}
			if manualInputState != state {
				return fmt.Errorf("state mismatch; expected %s got %s", state, manualInputState)
			}
		}

		token, err := flow.ExchangeCode(ctx, code, redirectURI, pkce)
		if err != nil {
			return err
		}
		cmd.Printf("Authorization complete.\nAccess token expires: %s\nToken file: %s\n", token.ExpiresAt.Local().Format(time.RFC1123), store.Path())
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show token expiration info",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := cmd.Flags().GetBool("text"); err != nil {
			return err
		}
		store, err := tokens.NewStore("")
		if err != nil {
			return err
		}
		token, err := store.Load()
		if err != nil {
			return err
		}
		if token == nil {
			cmd.Println("No tokens stored. Run `withingy auth login`.")
			return nil
		}
		remaining := time.Until(token.ExpiresAt)
		cmd.Printf("Access token expires at %s (%s from now)\n", token.ExpiresAt.Local().Format(time.RFC1123), humanizeDuration(remaining))
		if len(token.Scope) > 0 {
			cmd.Printf("Scopes: %s\n", strings.Join(token.Scope, " "))
		}
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke tokens and clear local cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := tokens.NewStore("")
		if err != nil {
			return err
		}
		flow := auth.NewFlow(cfg, store)
		if err := flow.Logout(cmd.Context()); err != nil {
			return err
		}
		cmd.Println("Logged out and cleared local tokens.")
		return nil
	},
}

func runAutomaticLogin(ctx context.Context, cmd *cobra.Command, flow *auth.Flow, configuredRedirect, state string, pkce *auth.PKCE, noBrowser bool) (string, string, error) {
	redirectURI, resultCh, shutdown, err := startCallbackServer(ctx, configuredRedirect, state)
	if err != nil {
		return "", "", err
	}
	defer shutdown(context.Background())

	authURL, err := flow.BuildAuthURL(redirectURI, state, pkce)
	if err != nil {
		return "", "", err
	}
	printAuthURL(cmd, authURL)
	if !noBrowser {
		if err := openBrowserFunc(authURL); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to open browser automatically: %v\n", err)
			fmt.Fprintf(cmd.ErrOrStderr(), "Please open the URL above manually.\n")
		}
	}

	select {
	case res := <-resultCh:
		if res.err != nil {
			return "", "", res.err
		}
		if res.state != state {
			return "", "", errors.New("state mismatch from callback")
		}
		return redirectURI, res.code, nil
	case <-ctx.Done():
		return "", "", ctx.Err()
	}
}

func printAuthURL(cmd *cobra.Command, authURL string) {
	cmd.Printf("Open the following URL to authorize withingy:\n%s\n\n", authURL)
}

type callbackResult struct {
	code  string
	state string
	err   error
}

func startCallbackServer(ctx context.Context, redirectURI, expectedState string) (string, <-chan callbackResult, func(context.Context) error, error) {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return "", nil, nil, fmt.Errorf("parse redirect_uri: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil, nil, errors.New("redirect_uri must be http(s)")
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		// missing port -> use 0 (ephemeral)
		host = parsed.Host
		port = "0"
	}
	listenAddr := net.JoinHostPort(host, port)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return "", nil, nil, fmt.Errorf("listen on %s: %w", listenAddr, err)
	}
	actualPort := ln.Addr().(*net.TCPAddr).Port
	parsed.Host = net.JoinHostPort(host, strconv.Itoa(actualPort))

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
	}
	resultCh := make(chan callbackResult, 1)
	mux.HandleFunc(parsed.Path, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		code := query.Get("code")
		state := query.Get("state")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			return
		}
		if state != expectedState {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			resultCh <- callbackResult{err: errors.New("state mismatch"), state: state}
			return
		}

		fmt.Fprintln(w, "Authorization successful! You can close this window.")
		select {
		case resultCh <- callbackResult{code: code, state: state}:
		default:
		}
		go server.Shutdown(context.Background())
	})

	shutdown := func(ctx context.Context) error {
		return server.Shutdown(ctx)
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	go func() {
		server.Serve(ln)
	}()

	return parsed.String(), resultCh, shutdown, nil
}

func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func promptAuthInput(cmd *cobra.Command) (string, string, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	fmt.Fprint(cmd.OutOrStdout(), "Paste redirect URL: ")
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	return parseAuthInput(text)
}

func parseAuthInput(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errors.New("empty input")
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err != nil {
			return "", "", err
		}
		code := u.Query().Get("code")
		state := u.Query().Get("state")
		if code == "" {
			return "", "", errors.New("code missing from URL")
		}
		return code, state, nil
	}
	if strings.Contains(input, "code=") {
		values, err := url.ParseQuery(input)
		if err == nil {
			code := values.Get("code")
			state := values.Get("state")
			if code != "" {
				return code, state, nil
			}
		}
	}
	return input, "", nil
}

func humanizeDuration(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}
	hours := d / time.Hour
	minutes := (d % time.Hour) / time.Minute
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func ensureConfigTemplate() (bool, string, error) {
	path, err := paths.ConfigFile()
	if err != nil {
		return false, "", err
	}
	if _, err := os.Stat(path); err == nil {
		return false, path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, "", fmt.Errorf("check config file: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, "", fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(configTemplate), 0o600); err != nil {
		return false, "", fmt.Errorf("write config template: %w", err)
	}
	return true, path, nil
}
