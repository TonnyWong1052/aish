package auth

import (
	"bytes"
	"context"
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

// Google OAuth 公開客戶端（用於桌面/本地應用）－固定值
// 注意：此 client_id 與 client_secret 為公開客戶端用途，非用戶個人密鑰。
const (
	fixedClientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	fixedClientSecret = "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"
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
		if time.Now().Add(5 * time.Minute).Before(tokenCache.expiry) {
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
	if time.Now().Add(5 * time.Minute).Before(tokenCache.expiry) {
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
	// 強制使用固定的 client_id 與 client_secret（依使用者要求）
	clientID := fixedClientID
	clientSecret := fixedClientSecret

	httpClient := &http.Client{Timeout: 20 * time.Second}

	// 先嘗試符合用戶提供樣例的 JSON 請求體
	jsonBody := map[string]any{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"refresh_token": refresh,
		"grant_type":    "refresh_token",
	}
	jb, _ := json.Marshal(jsonBody)
	req, _ := http.NewRequest("POST", tokenURL, bytes.NewReader(jb))
	req.Header.Set("Content-Type", "application/json")
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
		form.Set("client_secret", clientSecret)
		req2, _ := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp2, err2 := httpClient.Do(req2)
		if err2 != nil {
			return err2
		}
		body, _ = io.ReadAll(resp2.Body)
		_ = resp2.Body.Close()
		if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
			return fmt.Errorf("token endpoint %s returned %d: %s", tokenURL, resp2.StatusCode, string(body))
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
	delete(raw, "expires_in")
	if data, err := json.MarshalIndent(raw, "", "  "); err == nil {
		if err := os.WriteFile(credsPath, data, 0o600); err != nil {
			return fmt.Errorf("failed to update oauth_creds.json: %v", err)
		}
	}
	fmt.Fprintln(os.Stderr, "auth: Token refreshed via HTTP (json-compatible format)")
	return nil
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
