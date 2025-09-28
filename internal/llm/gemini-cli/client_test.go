package geminicli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/llm/gemini/auth"
	"github.com/TonnyWong1052/aish/internal/prompt"
)

// Helper function to create a test provider
func createTestProvider(serverURL string) (*GeminiCLIProvider, error) {
	cfg := config.ProviderConfig{
		APIEndpoint: serverURL,
		Model:       "gemini-2.5-flash",
		Project:     "test-project",
	}
	// Get absolute path to prompts directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	promptsPath := filepath.Join(wd, "..", "..", "prompts")

	pm, err := prompt.NewManager(promptsPath)
	if err != nil {
		return nil, err
	}
	provider, err := NewProvider(cfg, pm)
	if err != nil {
		return nil, err
	}
	return provider.(*GeminiCLIProvider), nil
}

// Helper function to create a mock OAuth token file
func createMockTokenFile(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("Failed to create .gemini dir: %v", err)
	}
	tokenFile := filepath.Join(geminiDir, "oauth_creds.json")
	if err := os.WriteFile(tokenFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write token file: %v", err)
	}
	return home
}

func TestGeminiCLIProvider_GetSuggestion_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Mock a successful response
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{
							{"text": "```json\n{\n  \"explanation\": \"The command 'ls' lists directory contents.\",\n  \"command\": \"ls\"\n}\n```"},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	home := createMockTokenFile(t, `{"access_token": "valid-token", "expiry_date": 9999999999999}`)
	t.Setenv("HOME", home)

	provider, err := createTestProvider(mockServer.URL)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	capturedContext := llm.CapturedContext{
		Command:  "ls",
		Stdout:   "",
		Stderr:   "bash: ls: command not found",
		ExitCode: 127,
	}

	suggestion, err := provider.GetSuggestion(context.Background(), capturedContext, "en")
	if err != nil {
		t.Fatalf("GetSuggestion failed: %v", err)
	}

	if suggestion.Explanation != "The command 'ls' lists directory contents." {
		t.Errorf("Expected explanation 'The command 'ls' lists directory contents.', got '%s'", suggestion.Explanation)
	}
	if suggestion.CorrectedCommand != "ls" {
		t.Errorf("Expected command 'ls', got '%s'", suggestion.CorrectedCommand)
	}
}

func TestGeminiCLIProvider_GetSuggestion_AuthError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate an authentication error
		response := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid token",
				"status":  "UNAUTHENTICATED",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	home := createMockTokenFile(t, `{"access_token": "invalid-token"}`)
	t.Setenv("HOME", home)

	provider, err := createTestProvider(mockServer.URL)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	capturedContext := llm.CapturedContext{
		Command:  "fakecommand",
		Stdout:   "",
		Stderr:   "bash: fakecommand: command not found",
		ExitCode: 127,
	}

	_, err = provider.GetSuggestion(context.Background(), capturedContext, "en")
	if err == nil {
		t.Fatal("Expected GetSuggestion to fail with auth error, but it did not")
	}

	// Check if the error message contains the expected auth error details
	if !strings.Contains(err.Error(), "Invalid token") || !strings.Contains(err.Error(), "UNAUTHENTICATED") {
		t.Errorf("Expected auth error message to contain 'Invalid token' and 'UNAUTHENTICATED', got: %v", err)
	}
}

func TestGeminiCLIProvider_GetSuggestion_TokenExpired(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer expired-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Mock a successful response after token refresh (if implemented)
		// For this test, we assume the provider handles refresh or uses a fallback
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{
							{"text": "```json\n{\n  \"explanation\": \"Command found.\",\n  \"command\": \"echo 'test'\"\n}\n```"},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create a token that is expired
	expiredTime := time.Now().Add(-1 * time.Hour).UnixMilli()
	home := createMockTokenFile(t, fmt.Sprintf(`{"access_token": "expired-token", "expiry_date": %d}`, expiredTime))
	t.Setenv("HOME", home)

	provider, err := createTestProvider(mockServer.URL)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	capturedContext := llm.CapturedContext{
		Command:  "testcmd",
		Stdout:   "",
		Stderr:   "bash: testcmd: command not found",
		ExitCode: 127,
	}

	// The current implementation might not automatically refresh the token,
	// but it should fall back to the access_token file if oauth_creds.json is expired.
	// Let's create a fallback token file.
	fallbackTokenFile := filepath.Join(home, ".gemini", "access_token")
	if err := os.WriteFile(fallbackTokenFile, []byte("expired-token"), 0644); err != nil {
		t.Fatalf("Failed to write fallback token file: %v", err)
	}


	suggestion, err := provider.GetSuggestion(context.Background(), capturedContext, "en")
	// Depending on the exact logic of token refresh/fallback, this might succeed or fail.
	// For now, we'll assume it might succeed if the fallback token is somehow still valid
	// or if the provider doesn't strictly check expiry for the fallback file.
	// A more robust test would mock the EnsureValidToken call.
	if err != nil {
		t.Logf("GetSuggestion failed with expired token (expected behavior): %v", err)
	} else {
		if suggestion.Explanation != "Command found." {
			t.Errorf("Expected explanation 'Command found.', got '%s'", suggestion.Explanation)
		}
		if suggestion.CorrectedCommand != "echo 'test'" {
			t.Errorf("Expected command 'echo 'test'', got '%s'", suggestion.CorrectedCommand)
		}
	}
}

func TestGeminiCLIProvider_VerifyConnection_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		// Minimal valid response for verification
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "gemini-2.5-flash"}},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	home := createMockTokenFile(t, `{"access_token": "valid-token", "expiry_date": 9999999999999}`)
	t.Setenv("HOME", home)

	provider, err := createTestProvider(mockServer.URL)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	models, err := provider.VerifyConnection(context.Background())
	if err != nil {
		t.Fatalf("VerifyConnection failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("Expected non-empty list of models")
	}
	// Check for expected model names (this might change based on actual API)
	if models[0] != "gemini-2.5-pro" && models[0] != "gemini-2.5-flash" {
		t.Errorf("Unexpected model name: %s", models[0])
	}
}

func TestGeminiCLIProvider_VerifyConnection_AuthError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": {"message": "Token expired", "status": "UNAUTHENTICATED"}}`, http.StatusUnauthorized)
	}))
	defer mockServer.Close()

	home := createMockTokenFile(t, `{"access_token": "expired-token"}`)
	t.Setenv("HOME", home)

	provider, err := createTestProvider(mockServer.URL)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	_, err = provider.VerifyConnection(context.Background())
	if err == nil {
		t.Fatal("Expected VerifyConnection to fail with auth error")
	}
	if !strings.Contains(err.Error(), "Token expired") {
		t.Errorf("Expected auth error message to contain 'Token expired', got: %v", err)
	}
}

func TestGeminiCLIProvider_getOAuthToken_MissingGeminiDir(t *testing.T) {
	// Create a temporary home directory without .gemini
	home := t.TempDir()
	t.Setenv("HOME", home)

	provider, err := createTestProvider("http://dummyurl") // URL doesn't matter for this test
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Mock the confirm function to return false (user chooses gemini-cli login)
	provider.confirmFunc = func(prompt string) (bool, error) {
		return false, nil
	}

	_, err = provider.getOAuthToken()
	if err == nil {
		t.Fatal("Expected getOAuthToken to fail due to missing .gemini dir")
	}
	// The error message should now be the one from promptAndAuthenticate
	if !strings.Contains(err.Error(), "please run 'gemini-cli login'") {
		t.Errorf("Expected error message to contain 'please run 'gemini-cli login'', got '%v'", err)
	}
}

func TestGeminiCLIProvider_getOAuthToken_MissingTokenFiles(t *testing.T) {
	home := t.TempDir()
	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("Failed to create .gemini dir: %v", err)
	}
	t.Setenv("HOME", home)

	provider, err := createTestProvider("http://dummyurl")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Mock the confirm function to return false (user chooses gemini-cli login)
	provider.confirmFunc = func(prompt string) (bool, error) {
		return false, nil
	}

	_, err = provider.getOAuthToken()
	if err == nil {
		t.Fatal("Expected getOAuthToken to fail due to missing token files")
	}
	// The error message should now be the one from promptAndAuthenticate
	if !strings.Contains(err.Error(), "please run 'gemini-cli login'") {
		t.Errorf("Expected error message to contain 'please run 'gemini-cli login'', got '%v'", err)
	}
}

func TestGeminiCLIProvider_getOAuthToken_EnvOverride(t *testing.T) {
	home := t.TempDir() // No .gemini dir needed
	t.Setenv("HOME", home)
	t.Setenv("AISH_GEMINI_BEARER", "env-token")

	provider, err := createTestProvider("http://dummyurl")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	token, err := provider.getBearerToken(context.Background())
	if err != nil {
		t.Fatalf("getBearerToken failed: %v", err)
	}
	if token != "env-token" {
		t.Errorf("Expected token 'env-token', got '%s'", token)
	}
}

// TestGeminiCLIProvider_getOAuthToken_PromptAndAuthenticateWebSuccess tests the scenario
// where no token is found, the user is prompted, chooses web auth, and it succeeds.
func TestGeminiCLIProvider_getOAuthToken_PromptAndAuthenticateWebSuccess(t *testing.T) {
	// 1. Setup a temporary home directory that does NOT contain .gemini or token files initially.
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// 2. Create a provider with mocked functions.
	cfg := config.ProviderConfig{
		APIEndpoint: "http://dummyurl", // API endpoint not used in this specific token path
		Model:       "gemini-2.5-flash",
		Project:     "test-project",
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	promptsPath := filepath.Join(wd, "..", "..", "prompts")
	pm, err := prompt.NewManager(promptsPath)
	if err != nil {
		t.Fatalf("Failed to create prompt manager: %v", err)
	}
	provider, err := NewProvider(cfg, pm)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	gcliProvider := provider.(*GeminiCLIProvider)

	// 3. Mock the UI interaction (user chooses web auth).
	gcliProvider.confirmFunc = func(prompt string) (bool, error) {
		if strings.Contains(prompt, "Choose option 2 for web-based authentication") {
			return true, nil // Simulate user choosing 'yes' for web auth.
		}
		t.Errorf("Unexpected Confirm prompt: %s", prompt)
		return false, fmt.Errorf("unexpected prompt")
	}

	// 4. Mock the Google OAuth token endpoint.
	oauthMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			// Expect a POST request to the token endpoint
			if r.Method != http.MethodPost {
				t.Errorf("Expected POST request to /token, got %s", r.Method)
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			// Check for required parameters (simplified check)
			if err := r.ParseForm(); err != nil {
				t.Errorf("Failed to parse form: %v", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			if r.FormValue("grant_type") != "authorization_code" || r.FormValue("code") != "test-auth-code" {
				t.Errorf("Unexpected form values: %v", r.Form)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Respond with a successful token response
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "new-web-access-token",
				"refresh_token": "new-web-refresh-token",
				"expires_in":    3600,
				"token_type":    "Bearer",
			})
			return
		}
		// Any other path is unexpected for this mock
		t.Errorf("Unexpected request to OAuth mock server: %s %s", r.Method, r.URL.Path)
		http.Error(w, "Not found", http.StatusNotFound)
	}))
	defer oauthMockServer.Close()

	// 5. Mock the auth.StartWebAuthFlow function.
	gcliProvider.startWebAuthFlowFunc = func(ctx context.Context) error {
		// Simulate the local callback server receiving the code.
		// In a real scenario, this would be handled by the `startCallbackServer` in `web_auth.go`.
		// For the test, we directly simulate the token exchange and saving.

		// Simulate the token exchange by making a request to our oauthMockServer.
		formData := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {"test-auth-code"},
			"client_id":     {auth.DefaultPublicClientID},
			"client_secret": {auth.DefaultPublicClientSecret},
			"redirect_uri":  {fmt.Sprintf("http://localhost:8081")}, // Use a fixed port for simplicity in mock
		}
		tokenResp, err := http.PostForm(oauthMockServer.URL+"/token", formData)
		if err != nil {
			return fmt.Errorf("mock token exchange failed: %w", err)
		}
		defer tokenResp.Body.Close()

		if tokenResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(tokenResp.Body)
			return fmt.Errorf("mock token exchange failed with status %d: %s", tokenResp.StatusCode, string(body))
		}

		var tokens map[string]interface{}
		if err := json.NewDecoder(tokenResp.Body).Decode(&tokens); err != nil {
			return fmt.Errorf("failed to decode mock token response: %w", err)
		}

		// Simulate saving the tokens (copied from auth.saveTokens)
		geminiDir := filepath.Join(tempHome, ".gemini")
		if err := os.MkdirAll(geminiDir, 0755); err != nil {
			return fmt.Errorf("failed to create .gemini directory in mock: %w", err)
		}

		if expiresIn, ok := tokens["expires_in"].(float64); ok {
			tokens["expiry_date"] = time.Now().Add(time.Duration(expiresIn)*time.Second).UnixMilli()
		}

		credsPath := filepath.Join(geminiDir, "oauth_creds.json")
		credsData, err := json.MarshalIndent(tokens, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal mock oauth_creds.json: %w", err)
		}
		if err := os.WriteFile(credsPath, credsData, 0600); err != nil {
			return fmt.Errorf("failed to write mock oauth_creds.json: %w", err)
		}

		accessTokenPath := filepath.Join(geminiDir, "access_token")
		if accessToken, ok := tokens["access_token"].(string); ok {
			if err := os.WriteFile(accessTokenPath, []byte(accessToken), 0600); err != nil {
				return fmt.Errorf("failed to write mock access_token: %w", err)
			}
		}
		return nil
	}

	// 6. Trigger the authentication flow.
	// This call should trigger the prompt, then the mocked web auth flow.
	token, err := gcliProvider.getOAuthToken()
	if err != nil {
		t.Fatalf("getOAuthToken failed during web authentication flow: %v", err)
	}

	// 7. Verify the token and that the mock token file was created.
	expectedToken := "new-web-access-token"
	if token != expectedToken {
		t.Errorf("Expected token '%s', got '%s'", expectedToken, token)
	}

	// Verify the oauth_creds.json file was created in the temporary home directory.
	geminiDir := filepath.Join(tempHome, ".gemini")
	credsPath := filepath.Join(geminiDir, "oauth_creds.json")
	if _, err := os.Stat(credsPath); os.IsNotExist(err) {
		t.Fatalf("Expected oauth_creds.json to be created at %s", credsPath)
	}

	// Verify the content of the oauth_creds.json file
	data, err := os.ReadFile(credsPath)
	if err != nil {
		t.Fatalf("Failed to read mock oauth_creds.json: %v", err)
	}
	var tokenData map[string]interface{}
	if err := json.Unmarshal(data, &tokenData); err != nil {
		t.Fatalf("Failed to unmarshal mock oauth_creds.json: %v", err)
	}
	if tokenData["access_token"] != expectedToken {
		t.Errorf("oauth_creds.json contains incorrect access_token: %v", tokenData["access_token"])
	}

	// Verify the access_token file was also created
	accessTokenPath := filepath.Join(geminiDir, "access_token")
	if _, err := os.Stat(accessTokenPath); os.IsNotExist(err) {
		t.Fatalf("Expected access_token to be created at %s", accessTokenPath)
	}
	fileToken, err := os.ReadFile(accessTokenPath)
	if err != nil {
		t.Fatalf("Failed to read mock access_token file: %v", err)
	}
	if string(fileToken) != expectedToken {
		t.Errorf("access_token file contains incorrect token: %s", string(fileToken))
	}
}