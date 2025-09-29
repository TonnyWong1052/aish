package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/prompt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"
	"regexp"
)

// OpenAI API structures
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	// Some OpenAI-compatible proxies may default to streaming when the field is omitted.
	// Explicitly include stream:false to force a single JSON response and avoid long-lived connections.
	Stream bool `json:"stream"`
}

type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error interface{} `json:"error,omitempty"`
}

type ModelsResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
	Error interface{} `json:"error,omitempty"`
}

// OpenAIProvider implements the llm.Provider interface for OpenAI.
type OpenAIProvider struct {
	cfg    config.ProviderConfig
	pm     *prompt.Manager
	client *http.Client
}

// NewProvider creates a new OpenAIProvider.
func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
	// Increase timeout to better tolerate slower backends or proxies that buffer/stream
	client := &http.Client{
		Timeout: 90 * time.Second,
	}

	return &OpenAIProvider{
		cfg:    cfg,
		pm:     pm,
		client: client,
	}, nil
}

func init() {
	llm.RegisterProvider("openai", NewProvider)
}

// GetSuggestion implements the llm.Provider interface.
func (p *OpenAIProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
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

	// Make API request
	response, err := p.chatCompletion(ctx, tpl.String())
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %w", err)
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

	// Fallback: heuristic parsing
	return p.parseSuggestionResponse(response)
}

// GetEnhancedSuggestion implements the llm.Provider interface with enhanced context.
func (p *OpenAIProvider) GetEnhancedSuggestion(ctx context.Context, enhancedCtx llm.EnhancedCapturedContext, lang string) (*llm.Suggestion, error) {
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

	// Make API request
	response, err := p.chatCompletion(ctx, tpl.String())
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed for enhanced suggestion: %w", err)
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

	// Fallback: heuristic parsing
	return p.parseSuggestionResponse(response)
}

// GenerateCommand implements the llm.Provider interface.
func (p *OpenAIProvider) GenerateCommand(ctx context.Context, promptText string, lang string) (string, error) {
	// Get the prompt template
	promptTemplate, err := p.pm.GetPrompt("generate_command", mapLanguage(lang))
	if err != nil {
		return "", fmt.Errorf("failed to get prompt template: %w", err)
	}

	// Execute template with prompt data
	data := struct{ Prompt string }{Prompt: promptText}
	var tpl bytes.Buffer
	t := template.Must(template.New("prompt").Parse(promptTemplate))
	if err := t.Execute(&tpl, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Make API request
	response, err := p.chatCompletion(ctx, tpl.String())
	if err != nil {
		return "", fmt.Errorf("OpenAI API request failed: %w", err)
	}

    // Prefer JSON output
    cleaned := stripCodeFences(response)
    var obj struct {
        Command string `json:"command"`
    }
    if err := json.Unmarshal([]byte(cleaned), &obj); err == nil && strings.TrimSpace(obj.Command) != "" {
        return strings.TrimSpace(obj.Command), nil
    }

    // Fallback: extract plausible shell command; if not found, return empty to avoid executing prose
    if cmd := extractPlausibleCommand(response); cmd != "" {
        return cmd, nil
    }
    return "", fmt.Errorf("no plausible command found in provider response")
}

// extractPlausibleCommand tries to extract a shell-like command from free-form text.
// Strategy:
// 1) Prefer last triple-backtick code block, take its first non-empty line not starting with '#'.
// 2) Otherwise scan lines and pick the first that looks like a command (regex heuristics).
// 3) Reject obvious prose (e.g., contains phrases like "I am"/"I cannot answer").
func extractPlausibleCommand(text string) string {
    s := strings.TrimSpace(text)
    if s == "" {
        return ""
    }
    // Reject obvious prose answers
    lower := strings.ToLower(s)
    banned := []string{"i am", "i'm", "i cannot", "i can’t", "large language model", "sorry", "cannot answer", "i don't"}
    for _, b := range banned {
        if strings.Contains(lower, b) {
            return ""
        }
    }
    // Prefer fenced code blocks
    if i := strings.LastIndex(s, "```"); i != -1 {
        // find the matching opening fence
        start := strings.LastIndex(s[:i], "```")
        if start != -1 && start < i {
            block := s[start+3 : i]
            for _, line := range strings.Split(block, "\n") {
                line = strings.TrimSpace(line)
                if line == "" || strings.HasPrefix(line, "#") {
                    continue
                }
                if plausibleCommand(line) {
                    return line
                }
            }
        }
    }
    // Scan lines
    for _, line := range strings.Split(s, "\n") {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        if plausibleCommand(line) {
            return line
        }
    }
    return ""
}

var cmdStartRe = regexp.MustCompile(`^(?i)\s*(sudo\s+)?([a-z][a-z0-9._-]*|\.|\.\.|\./|/|~)(\s|$)`) 

func plausibleCommand(line string) bool {
    l := strings.TrimSpace(line)
    if l == "" {
        return false
    }
    if strings.HasPrefix(l, "bash") {
        l = strings.TrimSpace(strings.TrimPrefix(l, "bash"))
        if l == "" {
            return false
        }
    }
    if !cmdStartRe.MatchString(l) {
        return false
    }
    // Extra guard: avoid lines ending with a period that look like sentences
    if strings.HasSuffix(l, ".") && !strings.ContainsAny(l, "/-'_\"$&|><") {
        return false
    }
    return true
}

// GetAvailableModels fetches all available models from the OpenAI API
func (p *OpenAIProvider) GetAvailableModels(ctx context.Context) ([]string, error) {
	if p.cfg.APIKey == "" {
		return nil, errors.New("API key is missing for OpenAI")
	}

	// 嘗試兩組 URL 變體：
	// 1) 受管 /v1 前綴（預設） 2) 直接使用基底端點（不追加 /v1）
	base := strings.TrimSuffix(p.cfg.APIEndpoint, "/")
	managed := p.resolveURL("/models")
	direct := base + "/models"

	// 順序：若配置顯示省略 /v1，則優先 direct；否則優先 managed
	candidates := []string{managed, direct}
	if p.cfg.OmitV1Prefix {
		candidates = []string{direct, managed}
	}

	var firstErr error
	var lastErr error
	for _, apiURL := range candidates {
		// 先嘗試 POST（必要時回退到 GET）
		postReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader([]byte("{}")))
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to create POST request: %w", err)
			}
			lastErr = firstErr
			continue
		}
		postReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
		postReq.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(postReq)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("request failed: %w", err)
			}
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		// 若 405，回退 GET
		if resp.StatusCode == http.StatusMethodNotAllowed {
			resp.Body.Close()
			getReq, gerr := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if gerr != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to create GET request: %w", gerr)
				}
				lastErr = firstErr
				continue
			}
			getReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
			getReq.Header.Set("Content-Type", "application/json")
			resp, err = p.client.Do(getReq)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("request failed: %w", err)
				}
				lastErr = fmt.Errorf("request failed: %w", err)
				continue
			}
		}

		// 讀取完整 body 以便診斷與重試（避免 decoder 消耗流）
		bodyBytes, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to read response: %w", rerr)
			}
			lastErr = firstErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			// 未授權、金鑰錯誤等，直接回傳（換 URL 也不會修好）
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, firstN(string(bodyBytes), 200))
			}
			// 其他狀況先記錄，嘗試下一個候選 URL
			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, firstN(string(bodyBytes), 200))
			if firstErr == nil {
				firstErr = lastErr
			}
			continue
		}

		// 嘗試解析 JSON
		var modelsResp ModelsResponse
		if err := json.Unmarshal(bodyBytes, &modelsResp); err != nil {
			// 如果像 HTML（以 '<' 開頭）或非 JSON，換下一個候選 URL
			trimmed := strings.TrimSpace(string(bodyBytes))
			if strings.HasPrefix(trimmed, "<") {
				lastErr = fmt.Errorf("non-JSON response (HTML): %s", firstN(trimmed, 120))
				if firstErr == nil {
					firstErr = lastErr
				}
				continue
			}
			lastErr = fmt.Errorf("failed to decode response: %v; body: %s", err, firstN(trimmed, 200))
			if firstErr == nil {
				firstErr = lastErr
			}
			continue
		}

		if modelsResp.Error != nil {
			// 提取錯誤
			var errMsg string
			if errMap, ok := modelsResp.Error.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					errMsg = msg
				}
			}
			if errMsg == "" {
				errMsg = fmt.Sprintf("%v", modelsResp.Error)
			}
			// 金鑰或權限錯誤，直接回傳
			if strings.Contains(strings.ToLower(errMsg), "auth") || strings.Contains(strings.ToLower(errMsg), "key") {
				return nil, fmt.Errorf("API error: %s", errMsg)
			}
			lastErr = fmt.Errorf("API error: %s", errMsg)
			if firstErr == nil {
				firstErr = lastErr
			}
			continue
		}

		// 成功
		var models []string
		for _, model := range modelsResp.Data {
			models = append(models, model.ID)
		}
		// 根據成功的 URL 調整當前 session 的使用策略
		if apiURL == direct {
			p.cfg.OmitV1Prefix = true
		}
		return models, nil
	}

	if firstErr != nil {
		return nil, firstErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("failed to fetch models from all endpoint variants")
}

// VerifyConnection implements the llm.Provider interface.
func (p *OpenAIProvider) VerifyConnection(ctx context.Context) ([]string, error) {
	models, err := p.GetAvailableModels(ctx)
	if err != nil {
		return nil, err
	}

	// Filter for relevant models for verification
	var filteredModels []string
	for _, model := range models {
		if strings.Contains(model, "gpt-") {
			filteredModels = append(filteredModels, model)
		}
	}

	if len(filteredModels) == 0 {
		// Return some common models if none found
		filteredModels = []string{"gpt-4", "gpt-3.5-turbo"}
	}

	return filteredModels, nil
}

// chatCompletion makes a chat completion request to OpenAI API
func (p *OpenAIProvider) chatCompletion(ctx context.Context, message string) (string, error) {
	apiURL := p.resolveURL("/chat/completions")

	reqBody := ChatCompletionRequest{
		Model: p.cfg.Model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: message,
			},
		},
		Temperature: 0.1,
		MaxTokens:   1000,
		Stream:      false, // Explicitly disable streaming to get a single JSON response
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build the request (non-streaming)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	// Only set Authorization if we actually have a key; some proxies reject empty Bearer tokens.
	if strings.TrimSpace(p.cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
	// Request non-streaming JSON responses explicitly (some proxies respect Accept)
	req.Header.Set("Accept", "application/json")

	// Basic retry for transient upstream failures (e.g., 502/503/504) or transport errors
	var resp *http.Response
	var doErr error
	for attempt := 0; attempt < 3; attempt++ {
		// Ensure body can be re-read across retries
		if attempt > 0 {
			// Re-create the request body reader since it is single-use
			req.Body = io.NopCloser(bytes.NewReader(jsonBody))
		}
		resp, doErr = p.client.Do(req)
		if doErr != nil {
			// Backoff and retry on network errors
			time.Sleep(time.Duration(250*(attempt+1)) * time.Millisecond)
			continue
		}
		// Retry on common transient upstream errors
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout {
			// Drain and close body before retry to avoid leaks
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			time.Sleep(time.Duration(250*(attempt+1)) * time.Millisecond)
			continue
		}
		break
	}
	if doErr != nil {
		return "", fmt.Errorf("request failed: %w", doErr)
	}
	defer resp.Body.Close()

	// Read entire body so we can both parse JSON or present helpful text on failure
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", fmt.Errorf("failed to read response: %w", readErr)
	}

	// Attempt JSON decode first (non-streaming)
	var completion ChatCompletionResponse
	if err := json.Unmarshal(body, &completion); err == nil && completion.Object != "" {
		if completion.Error != nil {
			var errMsg string
			if errMap, ok := completion.Error.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					errMsg = msg
				}
			}
			if errMsg == "" {
				errMsg = fmt.Sprintf("%v", completion.Error)
			}
			return "", fmt.Errorf("API error: %s", errMsg)
		}
		if len(completion.Choices) == 0 {
			return "", errors.New("no response choices returned")
		}
		return completion.Choices[0].Message.Content, nil
	}

	// Attempt to parse Server-Sent Events (streaming) format if present
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "data:") || strings.Contains(trimmed, "chat.completion.chunk") {
		var builder strings.Builder
		for _, line := range strings.Split(trimmed, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" || payload == "" {
				continue
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(payload), &chunk); err == nil {
				if len(chunk.Choices) > 0 {
					builder.WriteString(chunk.Choices[0].Delta.Content)
				}
			}
		}
		out := strings.TrimSpace(builder.String())
		if out != "" {
			return out, nil
		}
	}

	// If non-200 or not JSON, return body as a plain string so callers can try heuristic parsing
	if resp.StatusCode != 200 {
		if trimmed == "" {
			return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, firstN(trimmed, 512))
	}

	// 200 but not standard JSON; treat as plain text content
	return strings.TrimSpace(string(body)), nil
}

// parseSuggestionResponse parses the OpenAI response to extract explanation and command
func (p *OpenAIProvider) parseSuggestionResponse(response string) (*llm.Suggestion, error) {
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
	// Remove triple backticks block, optionally starting with ```json
	if s, ok := strings.CutPrefix(s, "```"); ok {
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

// firstN returns at most n bytes of s (safe for logging)
func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// resolveURL intelligently joins the configured API endpoint with a subpath.
func (p *OpenAIProvider) resolveURL(subpath string) string {
	base := strings.TrimSuffix(p.cfg.APIEndpoint, "/")

	// If OmitV1Prefix is true, trust the user's endpoint and just append the subpath if needed.
	// The user is responsible for providing the correct base URL (e.g., "https://api.example.com/v1").
	if p.cfg.OmitV1Prefix {
		if strings.HasSuffix(base, subpath) {
			return base
		}
		return base + subpath
	}

	// For standard mode, we manage the /v1 prefix.
	// First, remove any /v1 the user might have added to avoid duplication.
	base = strings.TrimSuffix(base, "/v1")

	// Now, add our managed /v1 and the subpath.
	return base + "/v1" + subpath
}
