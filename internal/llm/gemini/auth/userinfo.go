package auth

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

// GetAuthenticatedEmail returns the Google account email associated with the
// currently saved OAuth access token (stored in AISH config directory).
// It calls Google's UserInfo endpoint using the saved access token.
func GetAuthenticatedEmail(ctx context.Context) (string, error) {
    token, err := getAccessTokenForGCP()
    if err != nil {
        return "", err
    }

    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    b, _ := io.ReadAll(resp.Body)

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return "", fmt.Errorf("userinfo failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
    }

    var m struct{ Email string `json:"email"` }
    if err := json.Unmarshal(b, &m); err == nil {
        if s := strings.TrimSpace(m.Email); s != "" {
            return s, nil
        }
    }
    return "", fmt.Errorf("email not found in userinfo response")
}

