package auth

import (
	"context"
	"encoding/json"
	"errors"
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
        // Ensure server is properly shut down after token exchange (short timeout)
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = server.Shutdown(shutdownCtx)
        return exchangeErr
    case err := <-errChan:
        // Ensure server is properly shut down on error
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = server.Shutdown(shutdownCtx)
        return err
    case <-ctx.Done():
        // Ensure server is properly shut down on context cancellation
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = server.Shutdown(shutdownCtx)
        return ctx.Err()
    }
}

// startCallbackServer starts a local HTTP server to handle the OAuth 2.0 callback.
func startCallbackServer(server *http.Server, codeChan chan<- string, errChan chan<- error) {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Get the authorization code from the query parameters.
        code := r.URL.Query().Get("code")
        if code == "" {
            errMsg := "Authorization code not found in callback request."
            http.Error(w, errMsg, http.StatusBadRequest)
            errChan <- fmt.Errorf("%s", errMsg)
            return
        }

        // Send a success message to the user's browser.
        fmt.Fprintf(w, "OAuth authentication successful! You can close this window now.")
        codeChan <- code

        // Shutdown the server after sending the code
        go func() {
            time.Sleep(100 * time.Millisecond) // Give time for the response to be sent
            _ = server.Shutdown(context.Background())
        }()
    })

    server.Handler = mux
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

	// Debug: Check if OAuth response already contains a project_id
	if existingProjectID, ok := tokens["project_id"].(string); ok && existingProjectID != "" {
		fmt.Fprintf(os.Stderr, " WARNING  OAuth response already contains project_id: %s (this will be overridden)\n", existingProjectID)
		// Remove it so we can detect the correct project
		delete(tokens, "project_id")
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

	// Detect and save project ID using the newly acquired access token
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if projectID, err := detectAndSaveProjectID(ctx, tokens); err == nil && projectID != "" {
		tokens["project_id"] = projectID
		fmt.Fprintf(os.Stderr, " SUCCESS  Detected Google Cloud Project: %s\n", projectID)
	} else {
		// Don't fail the entire auth flow, just warn the user
		fmt.Fprintf(os.Stderr, " WARNING  Could not auto-detect project ID: %v\n", err)
		fmt.Fprintf(os.Stderr, "          You can set it manually later:\n")
		fmt.Fprintf(os.Stderr, "          • Use: export AISH_GEMINI_PROJECT=YOUR_PROJECT_ID\n")
		fmt.Fprintf(os.Stderr, "          • Or: gcloud config set project YOUR_PROJECT_ID\n")
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

// detectAndSaveProjectID detects the Google Cloud project ID using the newly acquired access token.
// It tries multiple methods to find accessible projects and returns the most suitable one.
func detectAndSaveProjectID(ctx context.Context, tokens map[string]interface{}) (string, error) {
	// Get the access token from the tokens map
	accessToken, ok := tokens["access_token"].(string)
	if !ok || strings.TrimSpace(accessToken) == "" {
		return "", errors.New("no access token found in OAuth response")
	}

	// Try to list projects directly using the access token
	fmt.Fprintf(os.Stderr, " DEBUG  Attempting to list projects with OAuth token...\n")
	projects, err := listProjectsWithToken(ctx, accessToken)
	if err == nil && len(projects) > 0 {
		fmt.Fprintf(os.Stderr, " DEBUG  Found %d accessible projects:\n", len(projects))
		for _, p := range projects {
			fmt.Fprintf(os.Stderr, "         - %s (%s)\n", p.ProjectID, p.Name)
		}

		var selectedProjectID string

		// Use PickDefaultProject to select the best candidate
		projectID := PickDefaultProject(projects)
		if projectID != "" {
			fmt.Fprintf(os.Stderr, " DEBUG  PickDefaultProject selected: %s\n", projectID)
			selectedProjectID = projectID
		} else if len(projects) > 0 && strings.TrimSpace(projects[0].ProjectID) != "" {
			// If no default, use the first available project
			fmt.Fprintf(os.Stderr, " DEBUG  Using first available project: %s\n", projects[0].ProjectID)
			selectedProjectID = projects[0].ProjectID
		}

		if selectedProjectID != "" {
			// Enable required Google Cloud APIs for the selected project
			if err := enableGeminiAPIs(ctx, selectedProjectID, accessToken); err != nil {
				// Log the error but don't fail the auth flow
				fmt.Fprintf(os.Stderr, " WARNING  API enablement encountered issues: %v\n", err)
			}
			return selectedProjectID, nil
		}
	} else {
		if err != nil {
			fmt.Fprintf(os.Stderr, " WARNING  Failed to list projects with OAuth token: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, " WARNING  No projects found with OAuth token\n")
		}
	}

	// Don't fall back to gcloud auto-detection during OAuth flow
	// since we want to use only projects accessible with the OAuth token
	return "", errors.New("no accessible Google Cloud projects found with current OAuth credentials")
}

// listProjectsWithToken lists projects using a specific access token
func listProjectsWithToken(ctx context.Context, accessToken string) ([]GCPProject, error) {
	client := &http.Client{Timeout: 20 * time.Second}

	// Use Cloud Resource Manager v1 API with lifecycleState filter
	endpoint := "https://cloudresourcemanager.googleapis.com/v1/projects?filter=lifecycleState:ACTIVE"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to list projects: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		Projects []GCPProject `json:"projects"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse projects response: %w", err)
	}

	return response.Projects, nil
}

// enableGeminiAPIs enables the required Google Cloud APIs for Gemini
func enableGeminiAPIs(ctx context.Context, projectID string, accessToken string) error {
	// Required APIs for Gemini for Google Cloud
	requiredAPIs := []string{
		"cloudaicompanion.googleapis.com", // Gemini for Google Cloud API
	}

	fmt.Fprintf(os.Stderr, " INFO  Enabling required Google Cloud APIs for project %s...\n", projectID)

	// Use Google Service Usage API to enable services
	client := &http.Client{Timeout: 30 * time.Second}

	for _, api := range requiredAPIs {
		fmt.Fprintf(os.Stderr, "       • Enabling %s...\n", api)

		// Use the Service Usage API v1 to enable a single service
		endpoint := fmt.Sprintf("https://serviceusage.googleapis.com/v1/projects/%s/services/%s:enable",
			url.PathEscape(projectID), url.PathEscape(api))

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
		if err != nil {
			return fmt.Errorf("failed to create enable API request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, " WARNING  Failed to enable %s: %v\n", api, err)
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == 200 || resp.StatusCode == 201 {
			fmt.Fprintf(os.Stderr, " SUCCESS  ✓ %s enabled successfully\n", api)
		} else if resp.StatusCode == 409 {
			// 409 Conflict usually means the API is already enabled
			fmt.Fprintf(os.Stderr, " INFO     ✓ %s is already enabled\n", api)
		} else if resp.StatusCode == 403 {
			// Permission denied - user doesn't have serviceusage.services.enable permission
			fmt.Fprintf(os.Stderr, " WARNING  Insufficient permissions to enable %s\n", api)
			fmt.Fprintf(os.Stderr, "          Please enable it manually:\n")
			fmt.Fprintf(os.Stderr, "          gcloud services enable %s --project=%s\n", api, projectID)
		} else {
			// Other errors
			var errResp struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			json.Unmarshal(body, &errResp)
			fmt.Fprintf(os.Stderr, " WARNING  Failed to enable %s: %s\n", api, errResp.Error.Message)
			fmt.Fprintf(os.Stderr, "          Please enable it manually:\n")
			fmt.Fprintf(os.Stderr, "          gcloud services enable %s --project=%s\n", api, projectID)
		}
	}

	return nil // Don't fail the auth flow even if API enablement fails
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
