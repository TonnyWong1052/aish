package geminicli

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/llm/gemini/auth"
	"github.com/TonnyWong1052/aish/internal/prompt"
	"github.com/TonnyWong1052/aish/internal/ui"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// GeminiCLIProvider implements the llm.Provider interface for the Gemini CLI.
type GeminiCLIProvider struct {
	cfg                  config.ProviderConfig
	pm                   *prompt.Manager
	client               *http.Client
	confirmFunc          func(prompt string) (bool, error)
	startWebAuthFlowFunc func(ctx context.Context) error
}

// NewProvider creates a new GeminiCLIProvider.
func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
	// Create configurable HTTP Client (supports custom CA and optional skip verification)
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	// Environment variable control: AISH_GEMINI_CA_FILE specifies CA certificate; AISH_GEMINI_SKIP_TLS_VERIFY skips verification (test only)
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
		if skipVerify {
			tlsCfg.InsecureSkipVerify = true
		}
		tr.TLSClientConfig = tlsCfg
	}

	// Allow timeout override through environment variables (seconds)
	timeout := 30 * time.Second
	if s := strings.TrimSpace(os.Getenv("AISH_GEMINI_TIMEOUT")); s != "" {
		if n, err := time.ParseDuration(s + "s"); err == nil && n > 0 {
			timeout = n
		}
	}
	client := &http.Client{Timeout: timeout, Transport: tr}

	return &GeminiCLIProvider{
		cfg:                  cfg,
		pm:                   pm,
		client:               client,
		confirmFunc:          ui.Confirm,
		startWebAuthFlowFunc: auth.StartWebAuthFlow,
	}, nil
}

func init() {
	llm.RegisterProvider("gemini-cli", NewProvider)
}

// sanitizeToken normalizes common pasted formats into a raw OAuth token string.
// Accepts inputs like:
//   - "ya29.A0..." (raw token)
//   - "Bearer ya29.A0..."
//   - "Authorization: Bearer ya29.A0..."
//   - "Access Token: ya29.A0..."
//
// and returns only the bare token value.
func sanitizeToken(s string) string {
	s = strings.TrimSpace(s)
	// Strip surrounding quotes if present
	s = strings.Trim(s, "\"'")
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "authorization: bearer "):
		s = strings.TrimSpace(s[len("Authorization: Bearer "):])
	case strings.HasPrefix(lower, "bearer "):
		s = strings.TrimSpace(s[len("Bearer "):])
	case strings.HasPrefix(lower, "access token:"):
		s = strings.TrimSpace(s[len("Access Token:"):])
	}
	// Final trim
	return strings.TrimSpace(s)
}

// buildGenerateContentURL 構造 Cloud Code generateContent 端點 URL，
// 兼容以下輸入形式：
// 1) 空字串 → 預設正式端點
// 2) 完整 URL 且尚未帶 ":generateContent" → 將 ":generateContent" 添加到 Path（而非 Host:Port 之後）
// 3) 已包含 ":generateContent" → 原樣返回
// 4) 非 URL 基底字串 → 附加預設路徑 "/v1internal:generateContent"
func buildGenerateContentURL(endpoint string) (string, error) {
    ep := strings.TrimSpace(endpoint)
    if ep == "" {
        return "https://cloudcode-pa.googleapis.com/v1internal:generateContent", nil
    }
    if strings.Contains(ep, ":generateContent") {
        return ep, nil
    }
    if strings.Contains(ep, "://") {
        u, err := neturl.Parse(ep)
        if err != nil {
            return "", fmt.Errorf("invalid API endpoint: %w", err)
        }
        if u.Path == "" || u.Path == "/" {
            u.Path = "/v1internal:generateContent"
        } else {
            u.Path = strings.TrimRight(u.Path, "/") + ":generateContent"
        }
        return u.String(), nil
    }
    api := strings.TrimRight(ep, "/")
    return api + "/v1internal:generateContent", nil
}

// buildCloudCodeRequestBody builds the JSON payload to match the user's working sample.
// It includes model, project, request.contents and the tools.functionDeclarations schema.
func buildCloudCodeRequestBody(message, model, project string) map[string]any {
	if strings.TrimSpace(model) == "" {
		model = "gemini-2.5-flash"
	}
	return map[string]any{
		"model":   model,
		"project": project,
		"request": map[string]any{
			"contents": []map[string]any{
				{
					"role":  "user",
					"parts": []map[string]string{{"text": message}},
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
	t, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
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
				// CLI fallback
				if resp, cliBinErr := p.generateContentCLI(ctx, tpl.String()); cliBinErr == nil {
					response = resp
				} else if (isAuthError(cliErr) || isAuthError(httpErr)) && allowOfficialFallback() {
					// Optional fallback to official API (requires explicit opt-in)
					if resp, offErr := p.generateContentOfficialAPI(ctx, tpl.String()); offErr == nil {
						response = resp
					} else {
						return nil, fmt.Errorf("HTTP/CURL auth failed; CLI fallback failed; official API fallback failed: %v | curl: %v | http: %v | cli: %v", offErr, cliErr, httpErr, cliBinErr)
					}
				} else {
					return nil, fmt.Errorf("both CURL and HTTP failed (curl: %v) (http: %v)", cliErr, httpErr)
				}
			}
		}
	} else {
		// Default: HTTP first then cURL
		response, httpErr = p.generateContentHTTP(ctx, tpl.String())
		if httpErr != nil {
			response, cliErr = p.generateContentCURL(ctx, tpl.String())
			if cliErr != nil {
				// CLI fallback
				if resp, cliBinErr := p.generateContentCLI(ctx, tpl.String()); cliBinErr == nil {
					response = resp
				} else if (isAuthError(httpErr) || isAuthError(cliErr)) && allowOfficialFallback() {
					// Optional fallback to official API (requires explicit opt-in)
					if resp, offErr := p.generateContentOfficialAPI(ctx, tpl.String()); offErr == nil {
						response = resp
					} else {
						return nil, fmt.Errorf("HTTP/CURL auth failed; CLI fallback failed; official API fallback failed: %v | http: %v | curl: %v | cli: %v", offErr, httpErr, cliErr, cliBinErr)
					}
				} else {
					return nil, fmt.Errorf("both HTTP and CURL failed (http: %v) (curl: %v)", httpErr, cliErr)
				}
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
				if resp, cliBinErr := p.generateContentCLI(ctx, tpl.String()); cliBinErr == nil {
					response = resp
				} else if (isAuthError(cliErr) || isAuthError(httpErr)) && allowOfficialFallback() {
					if resp, offErr := p.generateContentOfficialAPI(ctx, tpl.String()); offErr == nil {
						response = resp
					} else {
						return nil, fmt.Errorf("both CURL and HTTP failed for enhanced suggestion; CLI fallback failed; official API fallback failed: %v | curl: %v | http: %v | cli: %v", offErr, cliErr, httpErr, cliBinErr)
					}
				} else {
					return nil, fmt.Errorf("both CURL and HTTP failed for enhanced suggestion (curl: %v) (http: %v)", cliErr, httpErr)
				}
			}
		}
	} else {
		response, httpErr = p.generateContentHTTP(ctx, tpl.String())
		if httpErr != nil {
			response, cliErr = p.generateContentCURL(ctx, tpl.String())
			if cliErr != nil {
				if resp, cliBinErr := p.generateContentCLI(ctx, tpl.String()); cliBinErr == nil {
					response = resp
				} else if (isAuthError(httpErr) || isAuthError(cliErr)) && allowOfficialFallback() {
					if resp, offErr := p.generateContentOfficialAPI(ctx, tpl.String()); offErr == nil {
						response = resp
					} else {
						return nil, fmt.Errorf("both HTTP and CURL failed for enhanced suggestion; CLI fallback failed; official API fallback failed: %v | http: %v | curl: %v | cli: %v", offErr, httpErr, cliErr, cliBinErr)
					}
				} else {
					return nil, fmt.Errorf("both HTTP and CURL failed for enhanced suggestion (http: %v) (curl: %v)", httpErr, cliErr)
				}
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
				if resp, cliBinErr := p.generateContentCLI(ctx, finalPrompt); cliBinErr == nil {
					response = resp
				} else if (isAuthError(cliErr) || isAuthError(httpErr)) && allowOfficialFallback() {
					if resp, offErr := p.generateContentOfficialAPI(ctx, finalPrompt); offErr == nil {
						response = resp
					} else {
						return "", fmt.Errorf("HTTP/CURL auth failed; CLI fallback failed; official API fallback failed: %v | curl: %v | http: %v | cli: %v", offErr, cliErr, httpErr, cliBinErr)
					}
				} else {
					return "", fmt.Errorf("both CURL and HTTP failed (curl: %v) (http: %v)", cliErr, httpErr)
				}
			}
		}
	} else {
		response, httpErr = p.generateContentHTTP(ctx, finalPrompt)
		if httpErr != nil {
			response, cliErr = p.generateContentCURL(ctx, finalPrompt)
			if cliErr != nil {
				if resp, cliBinErr := p.generateContentCLI(ctx, finalPrompt); cliBinErr == nil {
					response = resp
				} else if (isAuthError(httpErr) || isAuthError(cliErr)) && allowOfficialFallback() {
					if resp, offErr := p.generateContentOfficialAPI(ctx, finalPrompt); offErr == nil {
						response = resp
					} else {
						return "", fmt.Errorf("HTTP/CURL auth failed; CLI fallback failed; official API fallback failed: %v | http: %v | curl: %v | cli: %v", offErr, httpErr, cliErr, cliBinErr)
					}
				} else {
					return "", fmt.Errorf("both HTTP and CURL failed (http: %v) (curl: %v)", httpErr, cliErr)
				}
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

    targetURL, err := buildGenerateContentURL(p.cfg.APIEndpoint)
    if err != nil {
        return "", fmt.Errorf("failed to resolve API endpoint: %w", err)
    }

	// Get Bearer token (env override > oauth files)
	token, err := p.getBearerToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth token: %w", err)
	}

	doReq := func(model string) (string, int, string, error) {
		// Build request body to match user's working sample, including tools
		body := buildCloudCodeRequestBody(message, model, p.cfg.Project)

		jsonBody, err := json.Marshal(body)
		if err != nil {
			return "", 0, "", fmt.Errorf("failed to marshal request: %w", err)
		}

        req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(jsonBody))
        if err != nil {
            return "", 0, "", fmt.Errorf("failed to create request: %w", err)
        }
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
        if shouldDebug() {
            fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli HTTP url=%s\n", targetURL)
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
		// If API returns an explicit error object, surface it as an error instead of
		// attempting to heuristically extract text (prevents showing auth error as suggestion)
		if errObj, ok := response["error"].(map[string]any); ok {
			// Prefer structured message/status if available
			msg := strings.TrimSpace(getStringFromAny(errObj["message"]))
			sts := strings.TrimSpace(getStringFromAny(errObj["status"]))
			if msg == "" {
				msg = "unknown error"
			}
			if sts != "" {
				msg = fmt.Sprintf("%s: %s", sts, msg)
			}
			return "", resp.StatusCode, string(data), fmt.Errorf(msg)
		}
		// Extract text from response (supports top-level and wrapped under "response")
		if txt, ok := parseTextFromAPIResponse(response); ok {
			return txt, 200, string(data), nil
		}
		// Fallback: search only for known text fields to avoid capturing error messages
		if txt, ok := findKnownTextFields(response); ok {
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

// shouldUseCURL determines whether to prioritize cURL (environment variable AISH_GEMINI_USE_CURL=true/1/curl/yes)
func shouldUseCURL() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("AISH_GEMINI_USE_CURL")))
	return v == "1" || v == "true" || v == "yes" || v == "curl"
}

// shouldDebug controls whether to output debug information (masks sensitive data)
func shouldDebug() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("AISH_GEMINI_DEBUG")))
	return v == "1" || v == "true" || v == "yes" || v == "debug"
}

// maskToken masks Bearer token display
func maskToken(tok string) string {
	tok = strings.TrimSpace(tok)
	if len(tok) <= 10 {
		return "***"
	}
	return tok[:6] + "..." + tok[len(tok)-6:]
}

// getStringFromAny 將 interface{} 轉成字串（若為非字串則回傳空字串）
// getStringFromAny converts an interface{} to string; returns empty string for non-strings.
func getStringFromAny(v any) string {
    if s, ok := v.(string); ok {
        return s
    }
    return ""
}

// generateContentCURL uses cURL to send requests, format aligned with user-provided examples
func (p *GeminiCLIProvider) generateContentCURL(ctx context.Context, message string) (string, error) {
	// Ensure token is valid
	if err := auth.EnsureValidToken(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: token refresh check failed: %v\n", err)
	}

    targetURL, err := buildGenerateContentURL(p.cfg.APIEndpoint)
    if err != nil {
        return "", fmt.Errorf("failed to resolve API endpoint: %w", err)
    }

	token, err := p.getBearerToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth token: %w", err)
	}

	// Construct body to match user's working sample (includes tools)
	body := buildCloudCodeRequestBody(message, p.cfg.Model, p.cfg.Project)
	jb, _ := json.Marshal(body)

	if _, err := exec.LookPath("curl"); err != nil {
		return "", fmt.Errorf("curl not found in PATH")
	}
    cmd := exec.CommandContext(ctx, "curl",
        "--silent", "--show-error",
        "--request", "POST",
        "--url", targetURL,
        "--header", "Authorization: Bearer "+token,
        "--header", "Content-Type: application/json",
        "--data", string(jb),
    )
	// Do not set x-goog-user-project header to match user's working sample
	// SSL verification control: consistent with HTTP client
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
        fmt.Fprintf(os.Stderr, "DEBUG aish/gemini-cli CURL url=%s\n", targetURL)
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
	// 若回傳為錯誤物件，直接回傳錯誤，避免將錯誤訊息誤判為模型輸出
	if errObj, ok := response["error"].(map[string]any); ok {
		msg := strings.TrimSpace(getStringFromAny(errObj["message"]))
		sts := strings.TrimSpace(getStringFromAny(errObj["status"]))
		if msg == "" {
			msg = "unknown error"
		}
		if sts != "" {
			msg = fmt.Sprintf("%s: %s", sts, msg)
		}
		return "", fmt.Errorf(msg)
	}
	// Extract plain text response (also supports structure wrapped under "response")
	if txt, ok := parseTextFromAPIResponse(response); ok {
		return txt, nil
	}
	// 僅尋找已知的文字欄位，避免撈到 error.message
	if txt, ok := findKnownTextFields(response); ok {
		return txt, nil
	}
	return "", errors.New("invalid response format (curl)")

}

// parseTextFromAPIResponse parses API response structure, supports top-level or candidates structure wrapped under "response"
func parseTextFromAPIResponse(m map[string]any) (string, bool) {
	root := m
	if r, ok := m["response"].(map[string]any); ok {
		root = r
	}
	// Get first candidate's first parts.text
	if candidates, ok := root["candidates"].([]any); ok && len(candidates) > 0 {
		if candidate, ok := candidates[0].(map[string]any); ok {
			if content, ok := candidate["content"].(map[string]any); ok {
				if parts, ok := content["parts"].([]any); ok && len(parts) > 0 {
					// Scan to find first part with text
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

// findFirstTextInJSON recursively finds the first text field at any level (defensive handling of various response variants)
// findKnownTextFields 僅針對常見的內容欄位（text / content 等）做遞迴搜尋，避免誤把 error.message 視為模型輸出
func findKnownTextFields(v any) (string, bool) {
	switch t := v.(type) {
	case map[string]any:
		// 僅接受常見的文字欄位
		for _, key := range []string{"text", "output_text", "content"} {
			if s, ok := t[key].(string); ok && strings.TrimSpace(s) != "" {
				return s, true
			}
		}
		// 遞迴搜尋子節點
		for k, child := range t {
			// 忽略 error 與 message 關鍵字，避免抓到錯誤訊息
			kl := strings.ToLower(k)
			if kl == "error" || kl == "message" || kl == "status" {
				continue
			}
			if s, ok := findKnownTextFields(child); ok {
				return s, true
			}
		}
	case []any:
		for _, item := range t {
			if s, ok := findKnownTextFields(item); ok {
				return s, true
			}
		}
	}
	return "", false
}

// getBearerToken allows environment variable override (exactly consistent with token used in cURL testing)
func (p *GeminiCLIProvider) getBearerToken(ctx context.Context) (string, error) {
	if s := strings.TrimSpace(os.Getenv("AISH_GEMINI_BEARER")); s != "" {
		return sanitizeToken(s), nil
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

// getOAuthToken reads OAuth token from the aish config directory first, then falls back to the system's .gemini directory.
// It will prompt the user to choose an authentication method if no valid token is found.
func (p *GeminiCLIProvider) getOAuthToken() (string, error) {
	// 1. Try aish-specific token first
	aishConfigPath, err := config.GetConfigPath()
	if err == nil {
		aishConfigDir := filepath.Dir(aishConfigPath)
		aishTokenPath := filepath.Join(aishConfigDir, "gemini_oauth_creds.json")
		if token, err := readTokenFromFile(aishTokenPath); err == nil {
			if shouldDebug() {
				fmt.Fprintln(os.Stderr, "DEBUG aish/gemini-cli token_source=aish_config")
			}
			return token, nil
		}
	}

	// 2. Fallback to system-wide .gemini directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	geminiDir := filepath.Join(home, ".gemini")
	if _, err := os.Stat(geminiDir); err == nil {
		oauthPath := filepath.Join(geminiDir, "oauth_creds.json")
		if token, err := readTokenFromFile(oauthPath); err == nil {
			if shouldDebug() {
				fmt.Fprintln(os.Stderr, "DEBUG aish/gemini-cli token_source=system_gemini_dir")
			}
			return token, nil
		}
		accessTokenPath := filepath.Join(geminiDir, "access_token")
		if data, err := os.ReadFile(accessTokenPath); err == nil {
			token := sanitizeToken(strings.TrimSpace(string(data)))
			if token != "" {
				if shouldDebug() {
					fmt.Fprintln(os.Stderr, "DEBUG aish/gemini-cli token_source=access_token file")
				}
				return token, nil
			}
		}
	}

	// 3. If no token is found, prompt the user to authenticate.
	return p.promptAndAuthenticate()
}

// readTokenFromFile reads and validates a token from a given file path.
func readTokenFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", fmt.Errorf("failed to unmarshal %s: %w", path, err)
	}

	if accessToken, ok := obj["access_token"].(string); ok {
		token := strings.TrimSpace(accessToken)
		if token != "" {
			// Check for expiry if the field exists
			if v, ok := obj["expiry_date"]; ok {
				var expiryMillis int64
				switch n := v.(type) {
				case float64:
					expiryMillis = int64(n)
				case int64:
					expiryMillis = n
				default:
					// No valid expiry, assume it's good
					return token, nil
				}

				// Consider token expired 60 seconds before actual expiry
				if time.Now().After(time.UnixMilli(expiryMillis).Add(-60 * time.Second)) {
					return "", fmt.Errorf("token in %s has expired", path)
				}
			}
			return token, nil
		}
	}

	return "", errors.New("no valid access_token found in file")
}


// promptAndAuthenticate asks the user to choose an authentication method and then authenticates.
func (p *GeminiCLIProvider) promptAndAuthenticate() (string, error) {
	fmt.Fprintln(os.Stderr, "No valid OAuth token found for gemini-cli.")
	fmt.Fprintln(os.Stderr, "Please choose an authentication method:")
	fmt.Fprintln(os.Stderr, "1. Use 'gemini-cli login' (requires gemini-cli to be installed)")
	fmt.Fprintln(os.Stderr, "2. Use web-based authentication (will open a browser)")

	choice, err := p.confirmFunc("Choose option 2 for web-based authentication? (y/n): ")
	if err != nil {
		return "", fmt.Errorf("failed to get user choice: %w", err)
	}

	if choice {
		// User chose web-based authentication
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := p.startWebAuthFlowFunc(ctx); err != nil {
			return "", fmt.Errorf("web-based authentication failed: %w", err)
		}
		// After successful authentication, try reading the token again.
		return p.getOAuthToken() // Recursive call, but now token should exist.
	} else {
		// User chose gemini-cli login
		return "", fmt.Errorf("please run 'gemini-cli login' to authenticate and then try again.")
	}
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

    targetURL, err := buildGenerateContentURL(p.cfg.APIEndpoint)
    if err != nil {
        return fmt.Errorf("failed to resolve API endpoint: %w", err)
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
                    "parts": []map[string]string{{"text": "What is your model name"}},
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

    req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(jsonBody))
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
		explanation = "Please check command syntax and parameters."
	}
	if correctedCommand == "" {
		correctedCommand = "echo 'Unable to auto-correct command, please check manually'"
	}

	return &llm.Suggestion{
		Explanation:      explanation,
		CorrectedCommand: correctedCommand,
	}, nil
}

// mapLanguage maps user language preferences to template language codes
func mapLanguage(lang string) string {
	switch strings.ToLower(lang) {
	case "chinese", "zh", "zh-TW", "zh-CN":
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

// allowOfficialFallback 需顯式設定環境變數 AISH_GEMINI_ALLOW_OFFICIAL_FALLBACK=true 才會啟用官方 API 回退
func allowOfficialFallback() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("AISH_GEMINI_ALLOW_OFFICIAL_FALLBACK")))
	return v == "1" || v == "true" || v == "yes"
}

// isAuthError 判斷錯誤訊息是否為認證相關（401/UNAUTHENTICATED）
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "unauthenticated") || strings.Contains(e, "401") || strings.Contains(e, "authentication")
}

// resolveAPIKey 嘗試從環境變數或設定中取用官方 Gemini API Key
func (p *GeminiCLIProvider) resolveAPIKey() string {
	for _, k := range []string{"AISH_GEMINI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"} {
		if s := strings.TrimSpace(os.Getenv(k)); s != "" {
			return s
		}
	}
	if s := strings.TrimSpace(p.cfg.APIKey); s != "" && s != "YOUR_GEMINI_API_KEY" {
		return s
	}
	return ""
}

// generateContentOfficialAPI 使用官方 Gemini API（需 API Key）作為回退方案
func (p *GeminiCLIProvider) generateContentOfficialAPI(ctx context.Context, message string) (string, error) {
	apiKey := p.resolveAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("no Gemini API key available for fallback (set GEMINI_API_KEY/GOOGLE_API_KEY or configure providers.gemini.api_key)")
	}
	endpoint := strings.TrimSuffix(config.GeminiAPIEndpoint, "/")
	model := p.cfg.Model
	if strings.TrimSpace(model) == "" {
		model = "gemini-pro"
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", endpoint, model, apiKey)

	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{{"text": message}},
			},
		},
	}
	jb, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jb))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fallback API error %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("fallback decode error: %v", err)
	}
	if txt, ok := parseTextFromAPIResponse(m); ok {
		return txt, nil
	}
	if txt, ok := findKnownTextFields(m); ok {
		return txt, nil
	}
	return "", errors.New("fallback invalid response format")
}
