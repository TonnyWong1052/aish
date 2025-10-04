package ollama

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/prompt"
	"github.com/firebase/genkit/go/genkit"
	ollamaPlugin "github.com/firebase/genkit/go/plugins/ollama"
)

// OllamaProvider implements the llm.Provider interface using Genkit.
type OllamaProvider struct {
	cfg     config.ProviderConfig
	pm      *prompt.Manager
	genkit  *genkit.Genkit
	adapter *llm.GenkitAdapter
}

// NewProvider creates a new OllamaProvider using Genkit.
func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
	ctx := context.Background()

	// Initialize Genkit with Ollama plugin
	g := genkit.Init(ctx,
		genkit.WithPlugins(&ollamaPlugin.Ollama{
			ServerAddress: cfg.APIEndpoint, // http://localhost:11434
		}),
	)

	// Create adapter with model name
	// User can specify full model name with prefix (e.g., "ollama/llama3.3")
	// or just base name (will default to ollama/ prefix)
	modelName := cfg.Model
	if !strings.Contains(modelName, "/") {
		// No prefix provided, use ollama/ as default
		modelName = "ollama/" + modelName
	}
	// If user explicitly provided a prefix (contains /), use it as-is
	adapter := llm.NewGenkitAdapter(g, modelName)

	return &OllamaProvider{
		cfg:     cfg,
		pm:      pm,
		genkit:  g,
		adapter: adapter,
	}, nil
}

func init() {
	llm.RegisterProvider("ollama", NewProvider)
}

// GetSuggestion implements the llm.Provider interface.
func (p *OllamaProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
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
		return nil, fmt.Errorf("Ollama generation failed: %w", err)
	}

	return parseSuggestionResponse(response)
}

// GetEnhancedSuggestion implements the llm.Provider interface with enhanced context.
func (p *OllamaProvider) GetEnhancedSuggestion(ctx context.Context, enhancedCtx llm.EnhancedCapturedContext, lang string) (*llm.Suggestion, error) {
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
		return nil, fmt.Errorf("Ollama enhanced generation failed: %w", err)
	}

	return parseSuggestionResponse(response)
}

// GenerateCommand implements the llm.Provider interface.
func (p *OllamaProvider) GenerateCommand(ctx context.Context, promptText string, lang string) (string, error) {
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
		return "", fmt.Errorf("Ollama command generation failed: %w", err)
	}

	// Extract command from response
	if cmd := extractPlausibleCommand(response); cmd != "" {
		return cmd, nil
	}
	return "", fmt.Errorf("no plausible command found in response")
}

// VerifyConnection implements the llm.Provider interface.
func (p *OllamaProvider) VerifyConnection(ctx context.Context) ([]string, error) {
	// Test generation using Genkit
	if err := p.adapter.TestGeneration(ctx); err != nil {
		return nil, fmt.Errorf("Ollama connection verification failed (ensure Ollama is running at %s): %w", p.cfg.APIEndpoint, err)
	}

	// Return common Ollama models
	return []string{
		"llama3.3",
		"llama3.1",
		"codellama",
		"mistral",
	}, nil
}

// Helper functions
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
