package auth

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/TonnyWong1052/aish/internal/config"
)

// GCPProject 表示來自 Cloud Resource Manager v1 的專案資料
type GCPProject struct {
    ProjectID      string `json:"projectId"`
    Name           string `json:"name"`            // v1: display name; v3: resource name (projects/123)
    DisplayName    string `json:"displayName,omitempty"`
    ProjectNumber  string `json:"projectNumber"`
    LifecycleState string `json:"lifecycleState,omitempty"` // v1
    State          string `json:"state,omitempty"`          // v3
}

// ListActiveProjects 使用保存在 AISH 設定目錄的 OAuth 存取權杖，
// 呼叫 Cloud Resource Manager v1 列出目前帳號的 ACTIVE 專案。
// 注意：此函式假設剛完成 Web 認證，token 新鮮且具備 cloud-platform scope。
func ListActiveProjects(ctx context.Context) ([]GCPProject, error) {
    token, err := getAccessTokenForGCP()
    if err != nil {
        return nil, err
    }

    // 構造請求，支援分頁（若回傳 nextPageToken 則繼續拉取）
    endpoint := "https://cloudresourcemanager.googleapis.com/v1/projects"
    query := url.Values{}
    query.Set("filter", "lifecycleState:ACTIVE")

    client := &http.Client{}
    var projects []GCPProject
    nextToken := ""

	   for i := 0; i < 5; i++ { // 安全上限，避免意外循環
	       var u string
	       if nextToken == "" {
	           u = fmt.Sprintf("%s?%s", endpoint, query.Encode())
	       } else {
	           q := url.Values{}
	           q.Set("filter", "lifecycleState:ACTIVE")
	           q.Set("pageToken", nextToken)
	           u = fmt.Sprintf("%s?%s", endpoint, q.Encode())
	       }

        req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
        if err != nil {
            return nil, err
        }
        req.Header.Set("Authorization", "Bearer "+token)

        resp, err := client.Do(req)
        if err != nil {
            return nil, err
        }
        body, _ := io.ReadAll(resp.Body)
        _ = resp.Body.Close()

        if resp.StatusCode < 200 || resp.StatusCode >= 300 {
            // 將錯誤內容傳回，方便上層顯示提示並退回手動輸入
            return nil, fmt.Errorf("failed to list projects: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
        }

        var m struct {
            Projects      []GCPProject `json:"projects"`
            NextPageToken string       `json:"nextPageToken"`
        }
        if err := json.Unmarshal(body, &m); err != nil {
            return nil, fmt.Errorf("invalid response: %w", err)
        }
        for _, p := range m.Projects {
            if strings.EqualFold(p.LifecycleState, "ACTIVE") {
                projects = append(projects, p)
            }
        }
        if m.NextPageToken == "" {
            break
        }
        nextToken = m.NextPageToken
    }

    return projects, nil
}

// SearchProjectsV3 透過 Cloud Resource Manager v3 的 projects.search 列出可見專案（POST）
// 參考：POST https://cloudresourcemanager.googleapis.com/v3/projects:search
// 請求體：{"query":"state:ACTIVE","pageSize":N,"pageToken":"..."}
func SearchProjectsV3(ctx context.Context) ([]GCPProject, error) {
    token, err := getAccessTokenForGCP()
    if err != nil {
        return nil, err
    }

    endpoint := "https://cloudresourcemanager.googleapis.com/v3/projects:search"

    client := &http.Client{Timeout: 20 * time.Second}
    var projects []GCPProject
    pageToken := ""

    for i := 0; i < 5; i++ {
        body := map[string]any{
            "query": "state:ACTIVE",
        }
        if pageToken != "" {
            body["pageToken"] = pageToken
        }
        jb, _ := json.Marshal(body)
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(jb)))
        if err != nil {
            return nil, err
        }
        req.Header.Set("Authorization", "Bearer "+token)
        req.Header.Set("Content-Type", "application/json")

        resp, err := client.Do(req)
        if err != nil {
            return nil, err
        }
        data, _ := io.ReadAll(resp.Body)
        _ = resp.Body.Close()

        if resp.StatusCode < 200 || resp.StatusCode >= 300 {
            return nil, fmt.Errorf("v3 projects.search failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
        }

        var m struct {
            Projects      []GCPProject `json:"projects"`
            NextPageToken string       `json:"nextPageToken"`
        }
        if err := json.Unmarshal(data, &m); err != nil {
            return nil, fmt.Errorf("v3 invalid response: %w", err)
        }
        for _, p := range m.Projects {
            // v3 的名稱欄位為 displayName，真正的資源名在 name（projects/123）
            // best effort：若需要顯示名稱，保持為空或用 ProjectID 回退
            if strings.EqualFold(p.State, "ACTIVE") || strings.EqualFold(p.LifecycleState, "ACTIVE") {
                projects = append(projects, p)
            }
        }
        if m.NextPageToken == "" {
            break
        }
        pageToken = m.NextPageToken
    }
    return projects, nil
}

// getAccessTokenForGCP 嘗試從 AISH 設定目錄取得 Web 認證後存的 access_token；
// 若不存在，再回退至使用者家目錄的 ~/.gemini/oauth_creds.json 或 access_token。
func getAccessTokenForGCP() (string, error) {
    // 1) 先找 AISH 設定下的 gemini_oauth_creds.json
    if token, ok := readAccessTokenFromAishConfig(); ok {
        return token, nil
    }

    // 2) 回退 ~/.gemini/oauth_creds.json
    home, _ := os.UserHomeDir()
    if home != "" {
        oauthPath := filepath.Join(home, ".gemini", "oauth_creds.json")
        if token, ok := readAccessTokenFromJSON(oauthPath); ok {
            return token, nil
        }
        // 3) 最後嘗試 ~/.gemini/access_token 純文字
        if b, err := os.ReadFile(filepath.Join(home, ".gemini", "access_token")); err == nil {
            s := strings.TrimSpace(string(b))
            if s != "" {
                return s, nil
            }
        }
    }

    return "", errors.New("no OAuth access_token found (please authenticate first)")
}

func readAccessTokenFromAishConfig() (string, bool) {
    cfgPath, err := config.GetConfigPath()
    if err != nil {
        return "", false
    }
    dir := filepath.Dir(cfgPath)
    path := filepath.Join(dir, "gemini_oauth_creds.json")
    return readAccessTokenFromJSON(path)
}

func readAccessTokenFromJSON(path string) (string, bool) {
    b, err := os.ReadFile(path)
    if err != nil || len(b) == 0 {
        return "", false
    }
    m := map[string]any{}
    if err := json.Unmarshal(b, &m); err != nil {
        return "", false
    }
    if v, ok := m["access_token"].(string); ok {
        v = strings.TrimSpace(v)
        if v != "" {
            return v, true
        }
    }
    return "", false
}

// AutoDetectProjectID 嘗試從 GCE/GKE Metadata 或 gcloud 目前設定偵測 project-id
func AutoDetectProjectID(ctx context.Context) (string, error) {
    // 1) Metadata 服務（GCE/GKE）
    {
        req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
        req.Header.Set("Metadata-Flavor", "Google")
        client := &http.Client{Timeout: 1 * time.Second}
        if resp, err := client.Do(req); err == nil {
            b, _ := io.ReadAll(resp.Body)
            _ = resp.Body.Close()
            if resp.StatusCode == 200 {
                if s := strings.TrimSpace(string(b)); s != "" {
                    return s, nil
                }
            }
        }
    }
    // 2) gcloud 當前配置
    {
        ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
        defer cancel()
        cmd := exec.CommandContext(ctx2, "gcloud", "config", "get-value", "project")
        out, err := cmd.Output()
        if err == nil {
            s := strings.TrimSpace(string(out))
            if s != "" && s != "(unset)" {
                return s, nil
            }
        }
    }
    return "", nil
}

// GetProject 透過 Cloud Resource Manager 取得單一專案資訊（支援以 projectId 查詢）
func GetProject(ctx context.Context, projectID string) (*GCPProject, error) {
    token, err := getAccessTokenForGCP()
    if err != nil {
        return nil, err
    }
    u := fmt.Sprintf("https://cloudresourcemanager.googleapis.com/v1/projects/%s", url.PathEscape(strings.TrimSpace(projectID)))
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err := (&http.Client{}).Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    b, _ := io.ReadAll(resp.Body)
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("failed to get project: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
    }
    var p GCPProject
    if err := json.Unmarshal(b, &p); err != nil {
        return nil, fmt.Errorf("invalid project response: %w", err)
    }
    return &p, nil
}

// EnableRequiredAPIs 嘗試在指定專案啟用一組 API 服務。
// 注意：此操作需要 'serviceusage.services.enable' 權限，且目標服務需為公開可啟用。
// 對於無法啟用的服務（例如私有或受限），本函式僅回傳警示錯誤，請上層以 Warning 呈現並不中斷流程。
func EnableRequiredAPIs(ctx context.Context, projectID string, services []string) error {
    // 將 projectId 轉成 projectNumber（Service Usage API 需要）
    p, err := GetProject(ctx, projectID)
    if err != nil {
        return fmt.Errorf("resolve project number failed: %w", err)
    }
    number := strings.TrimSpace(p.ProjectNumber)
    if number == "" {
        return fmt.Errorf("missing projectNumber for project %s", projectID)
    }

    token, err := getAccessTokenForGCP()
    if err != nil {
        return err
    }

    // 批次啟用（若某些服務名稱不合法或無法啟用，由伺服器回應）
    endpoint := fmt.Sprintf("https://serviceusage.googleapis.com/v1/projects/%s/services:batchEnable", url.PathEscape(number))
    body := map[string]any{"serviceIds": services}
    jb, _ := json.Marshal(body)
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(jb)))
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := (&http.Client{}).Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    b, _ := io.ReadAll(resp.Body)
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        // 不中斷主流程：回上層 Warning
        return fmt.Errorf("enable APIs failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
    }

    // 最簡策略：不輪詢長時操作，快速返回；如需要可後續擴充 operation polling。
    return nil
}

// PickDefaultProject 從多個專案中選擇預設候選（優先名稱含 default，再回退第一個）
func PickDefaultProject(list []GCPProject) string {
    for _, p := range list {
        name := strings.ToLower(firstNonEmpty(p.DisplayName, firstNonEmpty(p.Name, p.ProjectID)))
        if strings.Contains(name, "default") {
            if strings.TrimSpace(p.ProjectID) != "" {
                return p.ProjectID
            }
        }
    }
    if len(list) > 0 {
        if strings.TrimSpace(list[0].ProjectID) != "" {
            return list[0].ProjectID
        }
    }
    return ""
}

// firstNonEmpty：本檔內部使用的小工具
func firstNonEmpty(a, b string) string {
    if strings.TrimSpace(a) != "" {
        return a
    }
    return b
}
