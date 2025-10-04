package anthropic

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/prompt"
	"github.com/firebase/genkit/go/genkit"
	anthropicPlugin "github.com/firebase/genkit/go/plugins/compat_oai/anthropic"
	"github.com/openai/openai-go/option"
)

// ClaudeProvider implements the llm.Provider interface using Genkit.
type ClaudeProvider struct {
	cfg     config.ProviderConfig
	pm      *prompt.Manager
	genkit  *genkit.Genkit
	adapter *llm.GenkitAdapter
}

// NewProvider creates a new ClaudeProvider using Genkit.
func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
	ctx := context.Background()

	// Initialize Genkit with Anthropic plugin
	g := genkit.Init(ctx,
		genkit.WithPlugins(&anthropicPlugin.Anthropic{
			Opts: []option.RequestOption{
				option.WithAPIKey(cfg.APIKey),
			},
		}),
	)

	// Create adapter with model name
	// User can specify full model name with prefix (e.g., "openai/gpt-4", "anthropic/claude-3-5-sonnet")
	// or just base name (will default to anthropic/ prefix)
	modelName := cfg.Model
	if !strings.Contains(modelName, "/") {
		// No prefix provided, use anthropic/ as default
		modelName = "anthropic/" + modelName
	}
	// If user explicitly provided a prefix (contains /), use it as-is
	adapter := llm.NewGenkitAdapter(g, modelName)

	return &ClaudeProvider{
		cfg:     cfg,
		pm:      pm,
		genkit:  g,
		adapter: adapter,
	}, nil
}

func init() {
	llm.RegisterProvider("claude", NewProvider)
}

// GetSuggestion implements the llm.Provider interface.
func (p *ClaudeProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
	promptTemplate, err := p.pm.GetPrompt("get_suggestion", mapLanguage(lang))
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt template: %w", err)
	}

	// Execute template
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

	var tpl strings.Builder
	t := template.Must(template.New("prompt").Parse(promptTemplate))
	if err := t.Execute(&tpl, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// Use Genkit adapter to generate
	response, err := p.adapter.Generate(ctx, tpl.String())
	if err != nil {
		return nil, fmt.Errorf("Claude generation failed: %w", err)
	}

	return parseSuggestionResponse(response)
}

// GetEnhancedSuggestion implements the llm.Provider interface with enhanced context.
func (p *ClaudeProvider) GetEnhancedSuggestion(ctx context.Context, enhancedCtx llm.EnhancedCapturedContext, lang string) (*llm.Suggestion, error) {
	promptTemplate, err := p.pm.GetPrompt("get_enhanced_suggestion", mapLanguage(lang))
	if err != nil {
		return p.GetSuggestion(ctx, enhancedCtx.CapturedContext, lang)
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	var tpl strings.Builder
	t, err := template.New("prompt").Funcs(funcMap).Parse(promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	if err := t.Execute(&tpl, enhancedCtx); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	response, err := p.adapter.Generate(ctx, tpl.String())
	if err != nil {
		return nil, fmt.Errorf("Claude enhanced generation failed: %w", err)
	}

	return parseSuggestionResponse(response)
}

// GenerateCommand implements the llm.Provider interface.
func (p *ClaudeProvider) GenerateCommand(ctx context.Context, promptText string, lang string) (string, error) {
	promptTemplate, err := p.pm.GetPrompt("generate_command", mapLanguage(lang))
	if err != nil {
		return "", fmt.Errorf("failed to get prompt template: %w", err)
	}

	data := struct{ Prompt string }{Prompt: promptText}
	var tpl strings.Builder
	t := template.Must(template.New("prompt").Parse(promptTemplate))
	if err := t.Execute(&tpl, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	response, err := p.adapter.Generate(ctx, tpl.String())
	if err != nil {
		return "", fmt.Errorf("Claude command generation failed: %w", err)
	}

	// Extract command from response
	if cmd := extractPlausibleCommand(response); cmd != "" {
		return cmd, nil
	}
	return "", fmt.Errorf("no plausible command found in response")
}

// VerifyConnection implements the llm.Provider interface.
func (p *ClaudeProvider) VerifyConnection(ctx context.Context) ([]string, error) {
	if p.cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is missing for Claude")
	}

	// Test generation using Genkit
	if err := p.adapter.TestGeneration(ctx); err != nil {
		return nil, fmt.Errorf("Claude connection verification failed: %w", err)
	}

	// Return available models
	return []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
	}, nil
}

// Helper functions from original implementation
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

func parseSuggestionResponse(response string) (*llm.Suggestion, error) {
	response = strings.TrimSpace(response)

	var explanation, correctedCommand string
	lines := strings.Split(response, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(line), "explanation") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				explanation = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(strings.ToLower(line), "command") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				correctedCommand = strings.Trim(strings.TrimSpace(parts[1]), "`")
			}
		}
	}

	if explanation == "" {
		explanation = "Please check command syntax and parameters."
	}
	if correctedCommand == "" {
		correctedCommand = "echo 'Unable to auto-correct command'"
	}

	return &llm.Suggestion{
		Explanation:      explanation,
		CorrectedCommand: correctedCommand,
	}, nil
}

func extractPlausibleCommand(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}

	// Check for fenced code blocks
	if idx := strings.Index(s, "```"); idx != -1 {
		end := strings.Index(s[idx+3:], "```")
		if end != -1 {
			block := strings.TrimSpace(s[idx+3 : idx+3+end])
			for _, line := range strings.Split(block, "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					return line
				}
			}
		}
	}

	// Return first non-empty line
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}
