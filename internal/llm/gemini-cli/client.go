package geminicli

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "crypto/tls"
    "crypto/x509"
    "regexp"
	"net/http"
	"os"
	"os/exec"
	"path"
	"powerful-cli/internal/config"
	"powerful-cli/internal/llm"
	"powerful-cli/internal/llm/gemini/auth"
	"powerful-cli/internal/prompt"
	"strings"
	"text/template"
	"time"
)

// GeminiCLIProvider implements the llm.Provider interface for the Gemini CLI.
type GeminiCLIProvider struct {
	cfg    config.ProviderConfig
	pm     *prompt.Manager
	client *http.Client
}

// NewProvider creates a new GeminiCLIProvider.
func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
    // 建立可配置的 HTTP Client（支援自定 CA 與可選跳過驗證）
    tr := &http.Transport{
        Proxy: http.ProxyFromEnvironment,
    }

    // 環境變數控制：AISH_GEMINI_CA_FILE 指定 CA 憑證；AISH_GEMINI_SKIP_TLS_VERIFY 跳過驗證（僅測試用）
    caFile := strings.TrimSpace(os.Getenv("AISH_GEMINI_CA_FILE"))
    skipVerify := func() bool {
        v := strings.TrimSpace(strings.ToLower(os.Getenv("AISH_GEMINI_SKIP_TLS_VERIFY")))
        return v == "1" || v == "true" || v == "yes"
    }()
    if caFile != "" || skipVerify {
        tlsCfg := &tls.Config{}
        if caFile != "" {
            if pem, err := os.ReadFile(caFile); err == nil {
                pool := x509.NewCertPool()
                if pool.AppendCertsFromPEM(pem) {
                    tlsCfg.RootCAs = pool
                }
            }
        }
        if skipVerify { tlsCfg.InsecureSkipVerify = true }
        tr.TLSClientConfig = tlsCfg
    }

    // 允許透過環境變數覆蓋逾時（秒）
    timeout := 30 * time.Second
    if s := strings.TrimSpace(os.Getenv("AISH_GEMINI_TIMEOUT")); s != "" {
        if n, err := time.ParseDuration(s+"s"); err == nil && n > 0 {
            timeout = n
        }
    }
    client := &http.Client{ Timeout: timeout, Transport: tr }

	return &GeminiCLIProvider{
		cfg:    cfg,
		pm:     pm,
		client: client,
	}, nil
}

func init() {
	llm.RegisterProvider("gemini-cli", NewProvider)
}

// GetSuggestion implements the llm.Provider interface using HTTP API.
func (p *GeminiCLIProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
	// Get the prompt template
	promptTemplate, err := p.pm.GetPrompt("get_suggestion", mapLanguage(lang))
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt template: %w", err)
	}

	// Execute template with context data
	data := struct {
		Command  string
		Stdout   string
		Stderr   string
		ExitCode int
	}{
		Command:  capturedContext.Command,
		Stdout:   capturedContext.Stdout,
		Stderr:   capturedContext.Stderr,
		ExitCode: capturedContext.ExitCode,
	}

	var tpl bytes.Buffer
	t := template.Must(template.New("prompt").Parse(promptTemplate))
	if err := t.Execute(&tpl, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	var (
		response string
		httpErr  error
		cliErr   error
	)
	if shouldUseCURL() {
		// Prefer cURL parity first
		response, cliErr = p.generateContentCURL(ctx, tpl.String())
		if cliErr != nil {
			response, httpErr = p.generateContentHTTP(ctx, tpl.String())
			if httpErr != nil {
				return nil, fmt.Errorf("both CURL and HTTP failed (curl: %v) (http: %v)", cliErr, httpErr)
			}
		}
	} else {
		// Default: HTTP first then cURL
		response, httpErr = p.generateContentHTTP(ctx, tpl.String())
		if httpErr != nil {
			response, cliErr = p.generateContentCURL(ctx, tpl.String())
			if cliErr != nil {
				return nil, fmt.Errorf("both HTTP and CURL failed (http: %v) (curl: %v)", httpErr, cliErr)
			}
		}
	}

	// Prefer JSON output
	cleaned := stripCodeFences(response)
	var obj struct {
		Explanation      string `json:"explanation"`
		Command          string `json:"command"`
		CorrectedCommand string `json:"corrected_command"`
		CorrectedCamel   string `json:"correctedCommand"`
	}
	if err := json.Unmarshal([]byte(cleaned), &obj); err == nil {
		cmd := obj.Command
		if cmd == "" {
			cmd = obj.CorrectedCommand
		}
		if cmd == "" {
			cmd = obj.CorrectedCamel
		}
		if strings.TrimSpace(cmd) != "" && strings.TrimSpace(obj.Explanation) != "" {
			return &llm.Suggestion{Explanation: strings.TrimSpace(obj.Explanation), CorrectedCommand: strings.TrimSpace(cmd)}, nil
		}
	}

	// Fallback: heuristic parser
	return p.parseSuggestionResponse(response)
}

// GetEnhancedSuggestion implements the llm.Provider interface with enhanced context.
func (p *GeminiCLIProvider) GetEnhancedSuggestion(ctx context.Context, enhancedCtx llm.EnhancedCapturedContext, lang string) (*llm.Suggestion, error) {
	// Get the enhanced prompt template
	promptTemplate, err := p.pm.GetPrompt("get_enhanced_suggestion", mapLanguage(lang))
	if err != nil {
		// Fall back to regular suggestion if enhanced template doesn't exist
		return p.GetSuggestion(ctx, enhancedCtx.CapturedContext, lang)
	}

	// Create template functions
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
	}

	// Execute template with enhanced context data
	var tpl bytes.Buffer
	t, err := template.New("prompt").Funcs(funcMap).Parse(promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse enhanced template: %w", err)
	}

	if err := t.Execute(&tpl, enhancedCtx); err != nil {
		return nil, fmt.Errorf("failed to execute enhanced template: %w", err)
	}

	var (
		response string
		httpErr  error
		cliErr   error
	)
	if shouldUseCURL() {
		response, cliErr = p.generateContentCURL(ctx, tpl.String())
		if cliErr != nil {
			response, httpErr = p.generateContentHTTP(ctx, tpl.String())
			if httpErr != nil {
				return nil, fmt.Errorf("both CURL and HTTP failed for enhanced suggestion (curl: %v) (http: %v)", cliErr, httpErr)
			}
		}
	} else {
		response, httpErr = p.generateContentHTTP(ctx, tpl.String())
		if httpErr != nil {
			response, cliErr = p.generateContentCURL(ctx, tpl.String())
			if cliErr != nil {
				return nil, fmt.Errorf("both HTTP and CURL failed for enhanced suggestion (http: %v) (curl: %v)", httpErr, cliErr)
			}
		}
	}

	// Prefer JSON output (same parsing logic as regular GetSuggestion)
	cleaned := stripCodeFences(response)
	var obj struct {
		Explanation      string `json:"explanation"`
		Command          string `json:"command"`
		CorrectedCommand string `json:"corrected_command"`
		CorrectedCamel   string `json:"correctedCommand"`
	}
	if err := json.Unmarshal([]byte(cleaned), &obj); err == nil {
		cmd := obj.Command
		if cmd == "" {
			cmd = obj.CorrectedCommand
		}
		if cmd == "" {
			cmd = obj.CorrectedCamel
		}
		if strings.TrimSpace(cmd) != "" && strings.TrimSpace(obj.Explanation) != "" {
			return &llm.Suggestion{Explanation: strings.TrimSpace(obj.Explanation), CorrectedCommand: strings.TrimSpace(cmd)}, nil
		}
	}

	// Fallback: heuristic parser
	return p.parseSuggestionResponse(response)
}

// GenerateCommand implements the llm.Provider interface.
func (p *GeminiCLIProvider) GenerateCommand(ctx context.Context, promptText string, lang string) (string, error) {
	promptTemplate, err := p.pm.GetPrompt("generate_command", mapLanguage(lang))
	if err != nil {
		return "", fmt.Errorf("failed to get prompt template: %w", err)
	}

	data := struct{ Prompt string }{Prompt: promptText}
	var tpl bytes.Buffer
	t := template.Must(template.New("prompt").Parse(promptTemplate))
	if err := t.Execute(&tpl, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	finalPrompt := tpl.String()

	var (
		response string
		httpErr  error
		cliErr   error
	)
	if shouldUseCURL() {
		response, cliErr = p.generateContentCURL(ctx, finalPrompt)
		if cliErr != nil {
			response, httpErr = p.generateContentHTTP(ctx, finalPrompt)
			if httpErr != nil {
				return "", fmt.Errorf("both CURL and HTTP failed (curl: %v) (http: %v)", cliErr, httpErr)
			}
		}
	} else {
		response, httpErr = p.generateContentHTTP(ctx, finalPrompt)
		if httpErr != nil {
			response, cliErr = p.generateContentCURL(ctx, finalPrompt)
			if cliErr != nil {
				return "", fmt.Errorf("both HTTP and CURL failed (http: %v) (curl: %v)", httpErr, cliErr)
			}
		}
	}

	// Prefer JSON output
	cleaned := stripCodeFences(response)
	var obj struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(cleaned), &obj); err == nil && strings.TrimSpace(obj.Command) != "" {
		return strings.TrimSpace(obj.Command), nil
	}

	// Fallback: previous heuristics
	generatedCommand := strings.TrimSpace(response)
	generatedCommand = strings.TrimPrefix(generatedCommand, "`")
	generatedCommand = strings.TrimSuffix(generatedCommand, "`")
	generatedCommand = strings.TrimPrefix(generatedCommand, "bash")
	generatedCommand = strings.TrimSpace(generatedCommand)
	return generatedCommand, nil
}

// VerifyConnection implements the llm.Provider interface.
func (p *GeminiCLIProvider) VerifyConnection(ctx context.Context) ([]string, error) {
	if p.cfg.Project == "" || p.cfg.Project == "YOUR_GEMINI_PROJECT_ID" {
		return nil, errors.New("project ID is missing for gemini-cli")
	}

	// Try to verify using the HTTP API
	err := p.verifyGeminiCLIEndpoint()
	if err != nil {
		return nil, fmt.Errorf("Gemini CLI verification failed: %w", err)
	}

	// Return a specific list of models for gemini-cli
	return []string{
		"gemini-2.5-pro",
		"gemini-2.5-flash",
	}, nil
}

// generateContentHTTP makes HTTP request to Gemini CLI API
func (p *GeminiCLIProvider) generateContentHTTP(ctx context.Context, message string) (string, error) {
	// Ensure the token is valid before making an API call
	if err := auth.EnsureValidToken(ctx); err != nil {
		// Log the error but attempt to proceed; the token might still be usable
		fmt.Fprintf(os.Stderr, "Warning: token refresh check failed: %v\n", err)
	}

	endpoint := strings.TrimSpace(p.cfg.APIEndpoint)
	if endpoint == "" {
		endpoint = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
	}
	url := endpoint
	if !strings.Contains(endpoint, ":generateContent") {
		api := strings.TrimRight(endpoint, "/")
		url = api + ":generateContent"
	}

	// Get Bearer token (env override > oauth files)
	token, err := p.getBearerToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth token: %w", err)
	}

	doReq := func(model string) (string, int, string, error) {
		// Build request body (match simplified reference shape: model, project, request.contents)
		body := map[string]any{
			"model":   model,
			"project": p.cfg.Project,
			"request": map[string]any{
				"contents": []map[string]any{
					{
						"role":  "user",
						"parts": []map[string]string{{"text": message}},
					},
				},
			},
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			return "", 0, "", fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
		if err != nil {
			return "", 0, "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if shouldDebug() {
			fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli HTTP url=%s\n", url)
			fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli HTTP body=%s\n", string(jsonBody))
			fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli HTTP token=%s\n", maskToken(token))
		}

		resp, err := p.client.Do(req)
		if err != nil {
			return "", 0, "", fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return "", resp.StatusCode, string(data), fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(data))
		}

        var response map[string]any
        if err := json.Unmarshal(data, &response); err != nil {
            return "", 200, string(data), fmt.Errorf("failed to decode response: %w", err)
        }
        // Extract text from response (supports top-level and wrapped under "response")
        if txt, ok := parseTextFromAPIResponse(response); ok {
            return txt, 200, string(data), nil
        }
        // Fallback: recursively search for first "text" field anywhere in payload
        if txt, ok := findFirstTextInJSON(response); ok {
            return txt, 200, string(data), nil
        }
        return "", 200, string(data), errors.New("invalid response format")
	}

	// First attempt with configured model
	respText, status, raw, err := doReq(p.cfg.Model)
	if err == nil {
		return respText, nil
	}
	// If 404, try with -001 suffix once (common variant)
	if status == http.StatusNotFound && !strings.HasSuffix(p.cfg.Model, "-001") {
		altModel := p.cfg.Model + "-001"
		if txt, _, _, err2 := doReq(altModel); err2 == nil {
			// cache the working model in memory for this provider instance
			p.cfg.Model = altModel
			return txt, nil
		}
	}
	// Return original error with raw payload to help diagnose
	return "", fmt.Errorf("HTTP %d error: %v\nraw: %s", status, err, raw)
}

// shouldUseCURL 決定是否以 cURL 優先（環境變數 AISH_GEMINI_USE_CURL=true/1/curl/yes）
func shouldUseCURL() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("AISH_GEMINI_USE_CURL")))
	return v == "1" || v == "true" || v == "yes" || v == "curl"
}

// shouldDebug 控制是否輸出除錯資訊（遮蔽敏感資料）
func shouldDebug() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("AISH_GEMINI_DEBUG")))
	return v == "1" || v == "true" || v == "yes" || v == "debug"
}

// maskToken 遮蔽 Bearer token 顯示
func maskToken(tok string) string {
	tok = strings.TrimSpace(tok)
	if len(tok) <= 10 {
		return "***"
	}
	return tok[:6] + "..." + tok[len(tok)-6:]
}

// generateContentCURL 使用 cURL 發送請求，格式對齊使用者提供的範例
func (p *GeminiCLIProvider) generateContentCURL(ctx context.Context, message string) (string, error) {
	// 確保 token 有效
	if err := auth.EnsureValidToken(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: token refresh check failed: %v\n", err)
	}

	endpoint := strings.TrimSpace(p.cfg.APIEndpoint)
	if endpoint == "" {
		endpoint = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
	}
	url := endpoint
	if !strings.Contains(endpoint, ":generateContent") {
		api := strings.TrimRight(endpoint, "/")
		url = api + ":generateContent"
	}

	token, err := p.getBearerToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth token: %w", err)
	}

	// 構造 body（簡化版本，與你最新提供的參考一致）
	body := map[string]any{
		"model":   p.cfg.Model,
		"project": p.cfg.Project,
		"request": map[string]any{
			"contents": []map[string]any{
				{
					"role":  "user",
					"parts": []map[string]string{{"text": message}},
				},
			},
		},
	}
	jb, _ := json.Marshal(body)

	if _, err := exec.LookPath("curl"); err != nil {
		return "", fmt.Errorf("curl not found in PATH")
	}
    cmd := exec.CommandContext(ctx, "curl",
        "--silent", "--show-error",
        "--request", "POST",
        "--url", url,
        "--header", "Authorization: Bearer "+token,
        "--header", "Content-Type: application/json",
        "--data", string(jb),
    )
    // SSL 驗證控制：與 HTTP 客戶端一致
    if v := strings.TrimSpace(strings.ToLower(os.Getenv("AISH_GEMINI_SKIP_TLS_VERIFY"))); v == "1" || v == "true" || v == "yes" {
        cmd.Args = append(cmd.Args, "--insecure")
    }
    if caf := strings.TrimSpace(os.Getenv("AISH_GEMINI_CA_FILE")); caf != "" {
        cmd.Args = append(cmd.Args, "--cacert", caf)
    }
	var out bytes.Buffer
	var errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("curl request failed: %v | %s", err, errb.String())
	}
	if shouldDebug() {
		fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli CURL url=%s\n", url)
		fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli CURL body=%s\n", string(jb))
		fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli CURL token=%s\n", maskToken(token))
		if s := errb.String(); strings.TrimSpace(s) != "" {
			fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli CURL stderr=%s\n", s)
		}
	}

	raw := out.Bytes()
    var response map[string]any
    if err := json.Unmarshal(raw, &response); err != nil {
        return "", fmt.Errorf("failed to decode curl response: %v | %s", err, out.String())
    }
    // 擷取純文字回覆（同時支援包裹在 "response" 下的結構）
    if txt, ok := parseTextFromAPIResponse(response); ok {
        return txt, nil
    }
    if txt, ok := findFirstTextInJSON(response); ok {
        return txt, nil
    }
    return "", errors.New("invalid response format (curl)")

}

// parseTextFromAPIResponse 解析 API 回傳結構，支援頂層或包在 "response" 內的 candidates 結構
func parseTextFromAPIResponse(m map[string]any) (string, bool) {
    root := m
    if r, ok := m["response"].(map[string]any); ok {
        root = r
    }
    // 取第一個 candidate 的第一個 parts.text
    if candidates, ok := root["candidates"].([]any); ok && len(candidates) > 0 {
        if candidate, ok := candidates[0].(map[string]any); ok {
            if content, ok := candidate["content"].(map[string]any); ok {
                if parts, ok := content["parts"].([]any); ok && len(parts) > 0 {
                    // 掃描找到第一個具有 text 的 part
                    for _, p := range parts {
                        if part, ok := p.(map[string]any); ok {
                            if text, ok := part["text"].(string); ok && strings.TrimSpace(text) != "" {
                                return text, true
                            }
                        }
                    }
                }
            }
        }
    }
    return "", false
}

// findFirstTextInJSON 遞迴尋找任意層級的第一個 text 欄位（防守性處理各種回應變體）
func findFirstTextInJSON(v any) (string, bool) {
    switch t := v.(type) {
    case map[string]any:
        // 優先直接拿 text
        if s, ok := t["text"].(string); ok && strings.TrimSpace(s) != "" {
            return s, true
        }
        // 遞迴進入所有子節點
        for _, child := range t {
            if s, ok := findFirstTextInJSON(child); ok {
                return s, true
            }
        }
    case []any:
        for _, item := range t {
            if s, ok := findFirstTextInJSON(item); ok {
                return s, true
            }
        }
    case string:
        // 少數服務直接回傳純文字
        s := strings.TrimSpace(t)
        if s != "" && !regexp.MustCompile(`^\{`).MatchString(s) { // 避免把整包 JSON 當文字
            return s, true
        }
    }
    return "", false
}

// getBearerToken 允許使用環境變數覆蓋（與你在 cURL 測試時使用的 token 完全一致）
func (p *GeminiCLIProvider) getBearerToken(ctx context.Context) (string, error) {
	if s := strings.TrimSpace(os.Getenv("AISH_GEMINI_BEARER")); s != "" {
		return s, nil
	}
	return p.getOAuthToken()
}

// generateContentCLI uses gemini-cli command as fallback
func (p *GeminiCLIProvider) generateContentCLI(ctx context.Context, message string) (string, error) {
	if _, err := exec.LookPath("gemini-cli"); err != nil {
		return "", fmt.Errorf("gemini-cli not found in PATH. Please install gemini-cli and authenticate. Docs: https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/authentication.md#workspace-gca")
	}
	cmd := exec.CommandContext(ctx, "gemini-cli", "p", "--prompt", message)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("gemini-cli command failed: %s", stderr.String())
	}

	return out.String(), nil
}

// getOAuthToken reads OAuth token from ~/.gemini directory
func (p *GeminiCLIProvider) getOAuthToken() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Try access_token file first
	accessTokenPath := path.Join(home, ".gemini", "access_token")
	if data, err := os.ReadFile(accessTokenPath); err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			return token, nil
		}
	}

	// Try oauth_creds.json
	oauthPath := path.Join(home, ".gemini", "oauth_creds.json")
	if data, err := os.ReadFile(oauthPath); err == nil {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err == nil {
			if accessToken, ok := obj["access_token"].(string); ok {
				token := strings.TrimSpace(accessToken)
				if token != "" {
					return token, nil
				}
			}
		}
	}

	return "", errors.New("no OAuth token found in ~/.gemini (access_token or oauth_creds.json)")
}

// verifyGeminiCLIEndpoint verifies connection to Gemini CLI API
func (p *GeminiCLIProvider) verifyGeminiCLIEndpoint() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ensure the token is valid before making a verification call
	if err := auth.EnsureValidToken(ctx); err != nil {
		// Log the error but attempt to proceed
		fmt.Fprintf(os.Stderr, "Warning: token refresh check failed during verification: %v\n", err)
	}

	endpoint := strings.TrimSpace(p.cfg.APIEndpoint)
	if endpoint == "" {
		endpoint = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
	}
	url := endpoint
	if !strings.Contains(endpoint, ":generateContent") {
		api := strings.TrimRight(endpoint, "/")
		url = api + ":generateContent"
	}

	// Get OAuth token
	token, err := p.getOAuthToken()
	if err != nil {
		return err
	}

	// Build verification request using the reference format
	// Use provided model if set; otherwise default to a commonly available one
	model := strings.TrimSpace(p.cfg.Model)
	if model == "" || strings.EqualFold(model, "gemini-pro") {
		model = "gemini-2.5-flash"
	}
	body := map[string]any{
		"model":   model,
		"project": p.cfg.Project,
		"request": map[string]any{
			"contents": []map[string]any{
				{
					"role":  "user",
					"parts": []map[string]string{{"text": "May I have ur model name"}},
				},
			},
			"tools": []map[string]any{
				{
					"functionDeclarations": []map[string]any{
						{
							"name":        "get_current_weather",
							"description": "Get the current weather in a given location",
							"parametersJsonSchema": map[string]any{
								"type": "OBJECT",
								"properties": map[string]any{
									"location": map[string]any{
										"type":        "STRING",
										"description": "The city and state, e.g. San Francisco, CA",
									},
									"unit": map[string]any{
										"type": "STRING",
										"enum": []string{"celsius", "fahrenheit"},
									},
								},
								"required": []string{"location"},
							},
						},
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	// Reuse provider's HTTP client for consistent timeouts
	client := p.client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(data))
	}

	return nil
}

// parseSuggestionResponse parses the response to extract explanation and command
func (p *GeminiCLIProvider) parseSuggestionResponse(response string) (*llm.Suggestion, error) {
	response = strings.TrimSpace(response)

	// Try to find explanation and corrected command
	lines := strings.Split(response, "\n")

	var explanation string
	var correctedCommand string

	explanationFound := false
	commandFound := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for explanation markers
		if strings.Contains(strings.ToLower(line), "explanation") && !explanationFound {
			explanationFound = true
			// Check if explanation is on the same line
			if idx := strings.Index(strings.ToLower(line), "explanation"); idx != -1 {
				afterExplanation := strings.TrimSpace(line[idx+len("explanation"):])
				afterExplanation = strings.TrimLeft(afterExplanation, ":")
				afterExplanation = strings.TrimSpace(afterExplanation)
				if afterExplanation != "" {
					explanation = afterExplanation
				}
			}
			continue
		}

		// Look for command markers
		if (strings.Contains(strings.ToLower(line), "corrected") ||
			strings.Contains(strings.ToLower(line), "command")) && !commandFound {
			commandFound = true
			// Check if command is on the same line
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				cmd := strings.TrimSpace(parts[len(parts)-1])
				cmd = strings.Trim(cmd, "`")
				if cmd != "" && !strings.Contains(strings.ToLower(cmd), "command") {
					correctedCommand = cmd
				}
			}
			continue
		}

		// If we found explanation marker but no explanation yet, this might be it
		if explanationFound && explanation == "" && !strings.Contains(strings.ToLower(line), "command") {
			explanation = line
			explanationFound = false // Reset to look for more
			continue
		}

		// If we found command marker but no command yet, this might be it
		if commandFound && correctedCommand == "" {
			cmd := strings.Trim(line, "`")
			if cmd != "" {
				correctedCommand = cmd
			}
			commandFound = false // Reset to look for more
			continue
		}
	}

	// If we didn't find structured response, try to extract from the full response
	if explanation == "" || correctedCommand == "" {
		// Try to split by common patterns
		if strings.Contains(response, "Explanation:") && strings.Contains(response, "Corrected Command:") {
			parts := strings.Split(response, "Corrected Command:")
			if len(parts) >= 2 {
				correctedCommand = strings.TrimSpace(strings.Trim(parts[1], "`"))
				explanationPart := strings.Split(parts[0], "Explanation:")
				if len(explanationPart) >= 2 {
					explanation = strings.TrimSpace(explanationPart[1])
				}
			}
		} else {
			// Fallback: use the whole response as explanation and try to extract a command
			explanation = response
			// Look for commands in backticks
			if start := strings.Index(response, "`"); start != -1 {
				if end := strings.Index(response[start+1:], "`"); end != -1 {
					correctedCommand = response[start+1 : start+1+end]
				}
			}
		}
	}

	// Final fallbacks
	if explanation == "" {
		explanation = "請檢查命令語法和參數是否正確。"
	}
	if correctedCommand == "" {
		correctedCommand = "echo '無法自動修正命令，請手動檢查'"
	}

	return &llm.Suggestion{
		Explanation:      explanation,
		CorrectedCommand: correctedCommand,
	}, nil
}

// mapLanguage maps user language preferences to template language codes
func mapLanguage(lang string) string {
	switch strings.ToLower(lang) {
	case "chinese", "zh", "中文", "繁體中文":
		return "zh-TW"
	case "english", "en":
		return "en"
	default:
		return "en"
	}
}

// stripCodeFences removes common markdown code fences and json hints.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSpace(s)
		if strings.HasPrefix(strings.ToLower(s), "json") {
			s = strings.TrimSpace(s[4:])
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(strings.Trim(s, "`"))
}
