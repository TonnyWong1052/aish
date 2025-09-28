package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/prompt"
	"net/http"
	"strings"
	"text/template"
	"time"
)

// Gemini API structures
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiGenerationRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiCandidate struct {
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

// GeminiApiResponse is the top-level structure for a Gemini API response,
// which wraps the actual generation response.
type GeminiApiResponse struct {
	Response GeminiGenerationResponse `json:"response"`
}

type GeminiGenerationResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
	Error      *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

type GeminiModelsResponse struct {
	Models []struct {
		Name                       string   `json:"name"`
		BaseModelId                string   `json:"baseModelId"`
		Version                    string   `json:"version"`
		DisplayName                string   `json:"displayName"`
		Description                string   `json:"description"`
		InputTokenLimit            int      `json:"inputTokenLimit"`
		OutputTokenLimit           int      `json:"outputTokenLimit"`
		SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	} `json:"models"`
	NextPageToken string `json:"nextPageToken"`
	Error         *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// GeminiProvider implements the llm.Provider interface for Gemini.
type GeminiProvider struct {
	cfg    config.ProviderConfig
	pm     *prompt.Manager
	client *http.Client
}

// NewProvider creates a new GeminiProvider.
func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &GeminiProvider{
		cfg:    cfg,
		pm:     pm,
		client: client,
	}, nil
}

func init() {
	llm.RegisterProvider("gemini", NewProvider)
}

// GetSuggestion implements the llm.Provider interface.
func (p *GeminiProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
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
	response, err := p.generateContent(ctx, tpl.String())
	if err != nil {
		return nil, fmt.Errorf("Gemini API request failed: %w", err)
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
func (p *GeminiProvider) GetEnhancedSuggestion(ctx context.Context, enhancedCtx llm.EnhancedCapturedContext, lang string) (*llm.Suggestion, error) {
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
	response, err := p.generateContent(ctx, tpl.String())
	if err != nil {
		return nil, fmt.Errorf("Gemini API request failed for enhanced suggestion: %w", err)
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
func (p *GeminiProvider) GenerateCommand(ctx context.Context, promptText string, lang string) (string, error) {
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
	response, err := p.generateContent(ctx, tpl.String())
	if err != nil {
		return "", fmt.Errorf("Gemini API request failed: %w", err)
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
	command := strings.TrimSpace(response)
	command = strings.TrimPrefix(command, "`")
	command = strings.TrimSuffix(command, "`")
	command = strings.TrimPrefix(command, "bash")
	command = strings.TrimSpace(command)
	return command, nil
}

// VerifyConnection implements the llm.Provider interface.
func (p *GeminiProvider) VerifyConnection(ctx context.Context) ([]string, error) {
	if p.cfg.APIKey == "" || p.cfg.APIKey == "YOUR_GEMINI_API_KEY" {
		return nil, errors.New("API key is missing for Gemini")
	}

	// Make a request to list models
	var apiURL string
	endpoint := strings.TrimSuffix(p.cfg.APIEndpoint, "/")
	if p.cfg.Project != "" {
		apiURL = fmt.Sprintf("%s/projects/%s/models", endpoint, p.cfg.Project)
	} else {
		apiURL = fmt.Sprintf("%s/models?key=%s", endpoint, p.cfg.APIKey)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var modelsResp GeminiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if modelsResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", modelsResp.Error.Message)
	}

	// Filter for relevant models
	var models []string
	for _, model := range modelsResp.Models {
		// Extract model name without the "models/" prefix
		modelName := strings.TrimPrefix(model.Name, "models/")
		if strings.Contains(modelName, "gemini") {
			models = append(models, modelName)
		}
	}

	if len(models) == 0 {
		// Return some common models if none found
		models = []string{"gemini-pro", "gemini-pro-vision"}
	}

	return models, nil
}

// generateContent makes a content generation request to Gemini API
func (p *GeminiProvider) generateContent(ctx context.Context, message string) (string, error) {
	// Construct the API URL
	modelName := p.cfg.Model
	if modelName == "" {
		modelName = "gemini-pro"
	}

	var apiURL string
	endpoint := strings.TrimSuffix(p.cfg.APIEndpoint, "/")
	if p.cfg.Project != "" {
		apiURL = fmt.Sprintf("%s/projects/%s/models/%s:generateContent",
			endpoint, p.cfg.Project, modelName)
	} else {
		apiURL = fmt.Sprintf("%s/models/%s:generateContent?key=%s",
			endpoint, modelName, p.cfg.APIKey)
	}

	reqBody := GeminiGenerationRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{
						Text: message,
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResponse GeminiApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	completion := apiResponse.Response
	if completion.Error != nil {
		return "", fmt.Errorf("API error: %s", completion.Error.Message)
	}

	if len(completion.Candidates) == 0 {
		return "", errors.New("no response candidates returned")
	}

	if len(completion.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("no content parts in response")
	}

	return completion.Candidates[0].Content.Parts[0].Text, nil
}

// parseSuggestionResponse parses the Gemini response to extract explanation and command
func (p *GeminiProvider) parseSuggestionResponse(response string) (*llm.Suggestion, error) {
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
