package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Google OAuth public client for desktop/native apps (well-known, non-confidential)
// IMPORTANT: This pair is publicly documented and NOT a user secret.
// We only use it as a last-resort fallback to improve UX when no client is configured.
const (
	DefaultPublicClientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	DefaultPublicClientSecret = "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"
)

// OAuthCredentials represents the minimal structure we need from oauth_creds.json
type OAuthCredentials struct {
	ExpiryDate   int64  `json:"expiry_date"` // 毫秒
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenURI     string `json:"token_uri,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// stateCache holds the in-memory cache for token expiry and file stats
type stateCache struct {
	expiry  time.Time
	modTime time.Time
}

var (
	// Global cache to avoid re-reading the file on every command
	tokenCache *stateCache
	// Mutex to protect cache and file operations from race conditions
	mu sync.Mutex
)

// refreshThreshold returns the proactive refresh window. If the token will
// expire within this duration, we attempt a refresh. Default is 2 hours, and
// can be overridden via env var AISH_GEMINI_REFRESH_THRESHOLD (e.g. "90m", "2h").
func refreshThreshold() time.Duration {
	v := strings.TrimSpace(os.Getenv("AISH_GEMINI_REFRESH_THRESHOLD"))
	if v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 2 * time.Hour
}

// EnsureValidToken is the main entry point. It checks the token's validity
// using an efficient cache and delegates to `gemini-cli auth refresh` if necessary.
func EnsureValidToken(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("auth: failed to get user home directory: %v", err)
	}
	credsPath := filepath.Join(homeDir, ".gemini", "oauth_creds.json")

	// Check if the credentials file exists
	info, err := os.Stat(credsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file doesn't exist, we can't do anything.
			// This is not an error, as the user might not be using a gemini-cli flow.
			return nil
		}
		return fmt.Errorf("auth: cannot stat oauth_creds.json: %v", err)
	}

	// If cache is valid (file hasn't changed), check expiry from cache
	if tokenCache != nil && !info.ModTime().After(tokenCache.modTime) {
		if time.Now().Add(refreshThreshold()).Before(tokenCache.expiry) {
			// Token is still valid according to cache, no action needed
			return nil
		}
	}

	// --- Cache is invalid or token is expired, load fresh data from disk ---
	creds, err := loadCredentials(credsPath)
	if err != nil {
		return fmt.Errorf("auth: failed to load credentials: %v", err)
	}

	// Update cache with the fresh data
	tokenCache = &stateCache{
		expiry:  time.Unix(creds.ExpiryDate/1000, 0),
		modTime: info.ModTime(),
	}

	// Check expiry again with the fresh data
	if time.Now().Add(refreshThreshold()).Before(tokenCache.expiry) {
		return nil // Token is valid
	}

	// --- Token is confirmed將過期，嘗試自動刷新 ---
	// 1) 優先使用本機 gemini-cli
	if hasGeminiCLIInPath() {
		fmt.Fprintln(os.Stderr, "auth: Gemini token is expiring, delegating to `gemini-cli auth refresh`...")
		cmd := exec.CommandContext(ctx, "gemini-cli", "auth", "refresh")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			// 失敗則嘗試 HTTP Refresh 作為後備
			fmt.Fprintf(os.Stderr, "auth: gemini-cli refresh failed, trying HTTP refresh... (%v)\n", err)
			if herr := httpRefreshToken(credsPath); herr != nil {
				// 再嘗試 gcloud ADC 取得 access token
				if tok, gerr := gcloudFetchAccessToken(ctx); gerr == nil {
					if werr := writeAccessTokenAndUpdate(credsPath, tok, 50*time.Minute); werr == nil {
						fmt.Fprintln(os.Stderr, "auth: Token refreshed via gcloud (ADC)")
					} else {
						tokenCache = nil
						return fmt.Errorf("auth: token refresh failed (cli+http), gcloud wrote token but update failed: %v", werr)
					}
				} else {
					tokenCache = nil
					return fmt.Errorf("auth: token refresh failed (cli+http+gcloud). Please ensure gemini-cli is installed and authenticated.\nCLI error: %v\nHTTP error: %v\nGCLOUD error: %v", err, herr, gerr)
				}
			}
		}
	} else {
		// 2) 無 CLI：嘗試使用 oauth_creds.json 進行 HTTP Refresh，失敗則 gcloud ADC
		if err := httpRefreshToken(credsPath); err != nil {
			if tok, gerr := gcloudFetchAccessToken(ctx); gerr == nil {
				if werr := writeAccessTokenAndUpdate(credsPath, tok, 50*time.Minute); werr == nil {
					fmt.Fprintln(os.Stderr, "auth: Token refreshed via gcloud (ADC)")
				} else {
					tokenCache = nil
					return fmt.Errorf("auth: HTTP refresh failed and gcloud write failed: %v", werr)
				}
			} else {
				tokenCache = nil
				return fmt.Errorf("auth: gemini-cli not found in PATH and HTTP refresh failed: %v.\nAlternatively, install gcloud and run: gcloud auth application-default login\nOr install gemini-cli and run: gemini-cli auth login\nDocs: https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/authentication.md#workspace-gca", err)
			}
		}
	}

	// Invalidate the cache so the next check will re-read the updated file
	tokenCache = nil

	fmt.Fprintln(os.Stderr, "auth: Token refresh delegated successfully.")
	return nil
}

// loadCredentials reads the expiry date from oauth_creds.json
func loadCredentials(credsPath string) (*OAuthCredentials, error) {
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", credsPath, err)
	}
	var creds OAuthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %v", credsPath, err)
	}
	return &creds, nil
}

// httpRefreshToken 嘗試讀取 oauth_creds.json 並使用 refresh_token 走標準 OAuth2 Refresh Token 流程
func httpRefreshToken(credsPath string) error {
	// 讀取現有 oauth_creds.json 作為 map，保留未知欄位
	raw := map[string]any{}
	if b, err := os.ReadFile(credsPath); err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &raw)
	}

	// 抽取必要欄位
	refresh := strings.TrimSpace(getString(raw, "refresh_token"))
	if refresh == "" {
		// 後備：透過 struct 解析一次
		creds, err := loadCredentials(credsPath)
		if err != nil {
			return err
		}
		refresh = strings.TrimSpace(creds.RefreshToken)
		if refresh == "" {
			return errors.New("missing refresh_token in oauth_creds.json")
		}
		// 同步部分欄位到 raw
		if getString(raw, "client_id") == "" && creds.ClientID != "" {
			raw["client_id"] = creds.ClientID
		}
		if getString(raw, "client_secret") == "" && creds.ClientSecret != "" {
			raw["client_secret"] = creds.ClientSecret
		}
		if getString(raw, "token_uri") == "" && creds.TokenURI != "" {
			raw["token_uri"] = creds.TokenURI
		}
	}
	tokenURL := strings.TrimSpace(getString(raw, "token_uri"))
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}

	clientID, clientSecret, err := resolveClientCredentials(credsPath, raw)
	if err != nil {
		return err
	}
	if strings.TrimSpace(clientID) == "" {
		return errors.New("unable to resolve OAuth client_id; configure GOOGLE_OAUTH_CLIENT_ID or ensure oauth_creds.json contains id_token")
	}
	// Do not force client_secret here. Some public clients can refresh without a secret.
	// If the token endpoint requires a secret, it will respond with a descriptive error
	// which will be surfaced by formatTokenEndpointError below.

	httpClient := &http.Client{Timeout: 20 * time.Second}

	// 先嘗試符合用戶提供樣例的 JSON 請求體
	jsonBody := map[string]any{
		"client_id":     clientID,
		"refresh_token": refresh,
		"grant_type":    "refresh_token",
	}
	if clientSecret != "" {
		jsonBody["client_secret"] = clientSecret
	}
	jb, _ := json.Marshal(jsonBody)
	req, _ := http.NewRequest("POST", tokenURL, bytes.NewReader(jb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "aish-gemini-refresh/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// 若 JSON 方式不通，回退到 x-www-form-urlencoded（Google 官方標準）
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", refresh)
		form.Set("client_id", clientID)
		if clientSecret != "" {
			form.Set("client_secret", clientSecret)
		}
		req2, _ := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req2.Header.Set("User-Agent", "aish-gemini-refresh/1.0")
		resp2, err2 := httpClient.Do(req2)
		if err2 != nil {
			return err2
		}
		body, _ = io.ReadAll(resp2.Body)
		_ = resp2.Body.Close()
		if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
			return formatTokenEndpointError(resp2.StatusCode, body)
		}
	}

	// 解析 token 回應
	res := map[string]any{}
	if err := json.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("invalid token response: %v", err)
	}
	access := strings.TrimSpace(getString(res, "access_token"))
	if access == "" {
		return errors.New("empty access_token in token response")
	}
	if newRefresh := strings.TrimSpace(getString(res, "refresh_token")); newRefresh != "" {
		refresh = newRefresh
	}
	if scope := strings.TrimSpace(getString(res, "scope")); scope != "" {
		raw["scope"] = scope
	}
	if tokenType := strings.TrimSpace(getString(res, "token_type")); tokenType != "" {
		raw["token_type"] = tokenType
	}
	if idToken := strings.TrimSpace(getString(res, "id_token")); idToken != "" {
		raw["id_token"] = idToken
	}

	// 計算 expiry_date（毫秒），比實際過期時間提前 60 秒
	expiresIn := int64(getNumber(res, "expires_in"))
	// 預設 3600 秒
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	expiryDate := time.Now().Add(time.Duration(expiresIn)*time.Second - time.Minute).UnixMilli()

	// 寫 access_token
	home, _ := os.UserHomeDir()
	accessPath := filepath.Join(home, ".gemini", "access_token")
	if err := os.MkdirAll(filepath.Dir(accessPath), 0o755); err != nil {
		return fmt.Errorf("failed to create ~/.gemini dir: %v", err)
	}
	if err := os.WriteFile(accessPath, []byte(access+"\n"), 0o600); err != nil {
		return fmt.Errorf("failed to write access_token: %v", err)
	}

	// 依用戶提供的格式更新 oauth_creds.json：保留原欄位，覆寫 access_token、refresh_token、expiry_date，移除 expires_in
	raw["access_token"] = access
	raw["refresh_token"] = refresh
	raw["expiry_date"] = expiryDate
	raw["client_id"] = clientID
	if clientSecret != "" {
		raw["client_secret"] = clientSecret
	}
	delete(raw, "expires_in")
	if data, err := json.MarshalIndent(raw, "", "  "); err == nil {
		if err := os.WriteFile(credsPath, data, 0o600); err != nil {
			return fmt.Errorf("failed to update oauth_creds.json: %v", err)
		}
	}
	fmt.Fprintln(os.Stderr, "auth: Token refreshed via HTTP (json-compatible format)")
	return nil
}

func resolveClientCredentials(credsPath string, raw map[string]any) (string, string, error) {
	dir := filepath.Dir(credsPath)
	candidateIDs := []string{strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID"))}
	candidateSecrets := []string{strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"))}

	if dir != "" {
		if id, secret, err := readOAuthClientConfig(dir); err != nil {
			return "", "", err
		} else {
			if id != "" {
				candidateIDs = append(candidateIDs, id)
			}
			if secret != "" {
				candidateSecrets = append(candidateSecrets, secret)
			}
		}
	}

	for _, key := range []string{"client_id", "clientId"} {
		if v := strings.TrimSpace(getString(raw, key)); v != "" {
			candidateIDs = append(candidateIDs, v)
			break
		}
	}
	for _, key := range []string{"client_secret", "clientSecret"} {
		if v := strings.TrimSpace(getString(raw, key)); v != "" {
			candidateSecrets = append(candidateSecrets, v)
			break
		}
	}

	var clientID string
	for _, v := range candidateIDs {
		if v != "" {
			clientID = v
			break
		}
	}

	var clientSecret string
	for _, v := range candidateSecrets {
		if v != "" {
			clientSecret = v
			break
		}
	}

	if clientID == "" {
		if idToken := strings.TrimSpace(getString(raw, "id_token")); idToken != "" {
			if inferred, err := inferClientIDFromIDToken(idToken); err == nil && inferred != "" {
				clientID = inferred
			}
		}
	}

	// Final fallback: if no usable client_secret is present and either
	// 1) client_id is empty, or
	// 2) client_id matches the public client id,
	// then adopt the public desktop client pair. This avoids mismatching
	// a secret for a different client_id.
	if strings.TrimSpace(clientSecret) == "" {
		if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientID) == DefaultPublicClientID {
			clientID = DefaultPublicClientID
			clientSecret = DefaultPublicClientSecret
		}
	}

	return clientID, clientSecret, nil
}

func formatTokenEndpointError(status int, body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		trimmed = "(empty body)"
	}

	var errObj struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &errObj) == nil {
		desc := strings.TrimSpace(errObj.ErrorDescription)
		if desc == "" {
			desc = strings.TrimSpace(errObj.Error)
		}
		lower := strings.ToLower(desc)
		if strings.Contains(lower, "client_secret") && strings.Contains(lower, "missing") {
			return fmt.Errorf("token refresh failed: HTTP %d - %s. This OAuth client requires a client_secret to refresh tokens. Provide GOOGLE_OAUTH_CLIENT_SECRET or ~/.gemini/oauth_client.json, or refresh via gemini-cli.", status, desc)
		}
		if errObj.Error == "invalid_client" || strings.Contains(lower, "invalid client") {
			return fmt.Errorf("token refresh failed: HTTP %d - %s. The client_id may be incorrect for this refresh_token.", status, desc)
		}
		if desc != "" {
			return fmt.Errorf("token refresh failed: HTTP %d - %s", status, desc)
		}
	}

	return fmt.Errorf("token refresh failed: HTTP %d - %s", status, trimmed)
}

func readOAuthClientConfig(dir string) (string, string, error) {
	path := filepath.Join(dir, "oauth_client.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("failed to read %s: %w", path, err)
	}

	cfg := map[string]any{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", "", fmt.Errorf("failed to parse %s: %w", path, err)
	}

	id := strings.TrimSpace(getString(cfg, "client_id"))
	if id == "" {
		id = strings.TrimSpace(getString(cfg, "clientId"))
	}
	secret := strings.TrimSpace(getString(cfg, "client_secret"))
	if secret == "" {
		secret = strings.TrimSpace(getString(cfg, "clientSecret"))
	}

	return id, secret, nil
}

func inferClientIDFromIDToken(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return "", errors.New("invalid id_token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode id_token payload: %w", err)
	}

	claims := map[string]any{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse id_token payload: %w", err)
	}

	if azp, ok := claims["azp"].(string); ok {
		if s := strings.TrimSpace(azp); s != "" {
			return s, nil
		}
	}

	switch aud := claims["aud"].(type) {
	case string:
		if s := strings.TrimSpace(aud); s != "" {
			return s, nil
		}
	case []any:
		for _, v := range aud {
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					return s, nil
				}
			}
		}
	}

	return "", nil
}

// getString 從 map 取得字串欄位
func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		switch s := v.(type) {
		case string:
			return s
		}
	}
	return ""
}

// getNumber 從 map 取得數值（float64/int 型別皆可）
func getNumber(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int64:
			return float64(n)
		case int:
			return float64(n)
		}
	}
	return 0
}

// hasGeminiCLIInPath 檢查是否可執行 gemini-cli
func hasGeminiCLIInPath() bool {
	if _, err := exec.LookPath("gemini-cli"); err == nil {
		return true
	}
	// 嘗試常見安裝位置（macOS/Linux Homebrew 或使用者 go/bin）
	guesses := []string{
		"/usr/local/bin/gemini-cli",
		"/opt/homebrew/bin/gemini-cli",
	}
	usr, _ := user.Current()
	if usr != nil {
		guesses = append(guesses, filepath.Join(usr.HomeDir, "go", "bin", "gemini-cli"))
		guesses = append(guesses, filepath.Join(usr.HomeDir, ".local", "bin", "gemini-cli"))
	}
	for _, g := range guesses {
		if fi, err := os.Stat(g); err == nil && !fi.IsDir() {
			return true
		}
	}
	// Windows 額外嘗試
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("gemini-cli.exe"); err == nil {
			return true
		}
	}
	return false
}

// gcloudFetchAccessToken 嘗試透過 gcloud 取得 ADC access token
func gcloudFetchAccessToken(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return "", errors.New("gcloud not found in PATH")
	}
	// 優先 ADC
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "application-default", "print-access-token")
	var out bytes.Buffer
	var errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		// 後備使用使用者憑證
		cmd2 := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
		out.Reset()
		errb.Reset()
		cmd2.Stdout = &out
		cmd2.Stderr = &errb
		if err2 := cmd2.Run(); err2 != nil {
			return "", fmt.Errorf("gcloud access token fetch failed: %v | %v", err, err2)
		}
	}
	tok := strings.TrimSpace(out.String())
	if tok == "" {
		return "", errors.New("empty token from gcloud")
	}
	return tok, nil
}

// writeAccessTokenAndUpdate 寫入 ~/.gemini/access_token 並嘗試更新 oauth_creds.json
func writeAccessTokenAndUpdate(credsPath, token string, approxTTL time.Duration) error {
	home, _ := os.UserHomeDir()
	accessPath := filepath.Join(home, ".gemini", "access_token")
	if err := os.MkdirAll(filepath.Dir(accessPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(accessPath, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	if b, err := os.ReadFile(credsPath); err == nil {
		m := map[string]any{}
		if json.Unmarshal(b, &m) == nil {
			m["access_token"] = token
			if approxTTL > 0 {
				m["expiry_date"] = time.Now().Add(approxTTL - time.Minute).UnixMilli()
			}
			if nd, err := json.MarshalIndent(m, "", "  "); err == nil {
				_ = os.WriteFile(credsPath, nd, 0o600)
			}
		}
	}
	return nil
}
