package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/google/uuid"
)

const (
	// CALLBACK_HOST is the hostname for the local callback server.
	CALLBACK_HOST = "localhost"
	// AUTH_ENDPOINT is the Google OAuth 2.0 authorization endpoint.
	AUTH_ENDPOINT = "https://accounts.google.com/o/oauth2/auth"
	// TOKEN_ENDPOINT is the Google OAuth 2.0 token endpoint.
	TOKEN_ENDPOINT = "https://oauth2.googleapis.com/token"
)

var (
	// SCOPES are the required OAuth scopes for the application.
	SCOPES = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
)

// GenerateOAuthURL creates the Google OAuth 2.0 authorization URL.
func GenerateOAuthURL(redirectPort int) (string, string, error) {
	state := uuid.New().String()
	params := url.Values{}
	params.Set("client_id", DefaultPublicClientID)
	params.Set("redirect_uri", fmt.Sprintf("http://%s:%d", CALLBACK_HOST, redirectPort))
	params.Set("scope", strings.Join(SCOPES, " "))
	params.Set("response_type", "code")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("include_granted_scopes", "true")
	params.Set("state", state)

	authURL := fmt.Sprintf("%s?%s", AUTH_ENDPOINT, params.Encode())
	return authURL, state, nil
}

// StartWebAuthFlow orchestrates the web-based OAuth 2.0 authorization flow.
func StartWebAuthFlow(ctx context.Context) error {
	// Channel to receive the authorization code from the callback server.
	codeChan := make(chan string)
	errChan := make(chan error)

	// 1. Find an available port and start the local callback server.
	port, err := findAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to find an available port: %w", err)
	}
	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}

	go startCallbackServer(server, codeChan, errChan)
	defer server.Shutdown(ctx)

	// 2. Generate the OAuth URL.
	authURL, _, err := GenerateOAuthURL(port)
	if err != nil {
		return fmt.Errorf("failed to generate OAuth URL: %w", err)
	}

	// 3. Open the URL in the user's default browser.
	fmt.Fprintf(os.Stderr, "Your browser has been opened to visit:\n\n%s\n\n", authURL)
	if err := openBrowser(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open browser automatically. Please open the URL manually.\n")
	}

	// 4. Wait for the authorization code or an error from the callback server.
	select {
	case code := <-codeChan:
		// 5. Exchange the authorization code for an access token.
		exchangeErr := exchangeCodeForToken(ctx, code, port)
		// Ensure server is properly shut down after token exchange
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		return exchangeErr
	case err := <-errChan:
		// Ensure server is properly shut down on error
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		return err
	case <-ctx.Done():
		// Ensure server is properly shut down on context cancellation
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		return ctx.Err()
	}
}

// startCallbackServer starts a local HTTP server to handle the OAuth 2.0 callback.
func startCallbackServer(server *http.Server, codeChan chan<- string, errChan chan<- error) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get the authorization code from the query parameters.
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := "Authorization code not found in callback request."
			http.Error(w, errMsg, http.StatusBadRequest)
			errChan <- fmt.Errorf(errMsg)
			return
		}

		// Send a success message to the user's browser.
		fmt.Fprintf(w, "OAuth authentication successful! You can close this window now.")
		codeChan <- code

		// Shutdown the server after sending the code
		go func() {
			time.Sleep(100 * time.Millisecond) // Give time for the response to be sent
			server.Shutdown(context.Background())
		}()
	})

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		errChan <- err
	}
}

// exchangeCodeForToken exchanges the authorization code for an access and refresh token.
func exchangeCodeForToken(ctx context.Context, code string, port int) error {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", DefaultPublicClientID)
	data.Set("client_secret", DefaultPublicClientSecret)
	data.Set("redirect_uri", fmt.Sprintf("http://%s:%d", CALLBACK_HOST, port))
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", TOKEN_ENDPOINT, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokens map[string]interface{}
	if err := json.Unmarshal(body, &tokens); err != nil {
		return fmt.Errorf("failed to unmarshal token response: %w", err)
	}

	// Ensure all I/O is completed before returning
	err = saveTokens(tokens)
	if err != nil {
		return err
	}

	// Give a moment for any buffered output to be flushed
	time.Sleep(100 * time.Millisecond)

	return nil
}

// saveTokens saves the access and refresh tokens to the aish config directory.
func saveTokens(tokens map[string]interface{}) error {
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get aish config path: %w", err)
	}
	configDir := filepath.Dir(configPath)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create aish config directory: %w", err)
	}

	// Add expiry_date to the tokens map.
	if expiresIn, ok := tokens["expires_in"].(float64); ok {
		tokens["expiry_date"] = time.Now().Add(time.Duration(expiresIn) * time.Second).UnixMilli()
	}

	// Write to gemini_oauth_creds.json.
	credsPath := filepath.Join(configDir, "gemini_oauth_creds.json")
	credsData, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal gemini_oauth_creds.json: %w", err)
	}
	if err := os.WriteFile(credsPath, credsData, 0600); err != nil {
		return fmt.Errorf("failed to write gemini_oauth_creds.json: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Authentication credentials saved to %s\n", credsPath)

	// Ensure the message is flushed to stderr
	os.Stderr.Sync()

	return nil
}

// findAvailablePort finds an available TCP port on the local machine.
func findAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// openBrowser opens the specified URL in the user's default browser.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
