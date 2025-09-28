package ui

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
)

// ErrorType defines categories of user-facing errors
type ErrorType string

const (
	ConfigurationError ErrorType = "configuration"
	NetworkError       ErrorType = "network"
	AuthenticationError ErrorType = "authentication"
	ProviderError      ErrorType = "provider"
	ValidationError    ErrorType = "validation"
	SystemError        ErrorType = "system"
	UserError          ErrorType = "user"
)

// UserFriendlyError represents an error with enhanced user guidance
type UserFriendlyError struct {
	Type        ErrorType
	Title       string
	Message     string
	Suggestions []string
	HelpLink    string
	DebugInfo   string
	Cause       error
}

// Error implements the error interface
func (e *UserFriendlyError) Error() string {
	return e.Message
}

// ErrorHandler provides enhanced error display and guidance
type ErrorHandler struct {
	debugMode bool
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(debugMode bool) *ErrorHandler {
	return &ErrorHandler{
		debugMode: debugMode,
	}
}

// HandleError displays a user-friendly error with suggestions
func (eh *ErrorHandler) HandleError(err error) {
	if err == nil {
		return
	}

	var userErr *UserFriendlyError
	if uerr, ok := err.(*UserFriendlyError); ok {
		userErr = uerr
	} else {
		// Convert generic error to user-friendly format
		userErr = eh.convertToUserFriendlyError(err)
	}

	eh.displayError(userErr)
}

// CreateConfigurationError creates a configuration-related error
func (eh *ErrorHandler) CreateConfigurationError(message string, suggestions []string) *UserFriendlyError {
	return &UserFriendlyError{
		Type:        ConfigurationError,
		Title:       "Configuration Error",
		Message:     message,
		Suggestions: suggestions,
		HelpLink:    "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md#configuration-problems",
	}
}

// CreateNetworkError creates a network-related error
func (eh *ErrorHandler) CreateNetworkError(message string, suggestions []string) *UserFriendlyError {
	return &UserFriendlyError{
		Type:        NetworkError,
		Title:       "Network Connection Error",
		Message:     message,
		Suggestions: suggestions,
		HelpLink:    "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md#network-connectivity-problems",
	}
}

// CreateAuthenticationError creates an authentication-related error
func (eh *ErrorHandler) CreateAuthenticationError(message string, suggestions []string) *UserFriendlyError {
	return &UserFriendlyError{
		Type:        AuthenticationError,
		Title:       "Authentication Error",
		Message:     message,
		Suggestions: suggestions,
		HelpLink:    "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md#llm-provider-issues",
	}
}

// CreateProviderError creates a provider-related error
func (eh *ErrorHandler) CreateProviderError(message string, suggestions []string) *UserFriendlyError {
	return &UserFriendlyError{
		Type:        ProviderError,
		Title:       "LLM Provider Error",
		Message:     message,
		Suggestions: suggestions,
		HelpLink:    "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md#llm-provider-issues",
	}
}

// CreateValidationError creates a validation-related error
func (eh *ErrorHandler) CreateValidationError(message string, suggestions []string) *UserFriendlyError {
	return &UserFriendlyError{
		Type:        ValidationError,
		Title:       "Input Validation Error",
		Message:     message,
		Suggestions: suggestions,
		HelpLink:    "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md",
	}
}

// displayError renders the error with appropriate styling
func (eh *ErrorHandler) displayError(err *UserFriendlyError) {
	// Error header with appropriate icon
	icon := eh.getErrorIcon(err.Type)

	pterm.Println()
	headerStyle := pterm.NewStyle(pterm.FgRed, pterm.Bold)
	headerStyle.Printf("%s %s\n", icon, err.Title)

	// Error message
	pterm.Println()
	messageStyle := pterm.NewStyle(pterm.FgLightRed)
	messageStyle.Println(err.Message)

	// Suggestions section
	if len(err.Suggestions) > 0 {
		pterm.Println()
		suggestionStyle := pterm.NewStyle(pterm.FgYellow, pterm.Bold)
		suggestionStyle.Println("💡 Suggestions:")

		for i, suggestion := range err.Suggestions {
			pterm.Printf("   %d. %s\n", i+1, suggestion)
		}
	}

	// Help link
	if err.HelpLink != "" {
		pterm.Println()
		linkStyle := pterm.NewStyle(pterm.FgCyan)
		linkStyle.Printf("📚 For more help: %s\n", err.HelpLink)
	}

	// Debug information (only shown in debug mode)
	if eh.debugMode && err.DebugInfo != "" {
		pterm.Println()
		debugStyle := pterm.NewStyle(pterm.FgGray)
		debugStyle.Println("🔍 Debug Information:")
		debugStyle.Println(err.DebugInfo)
	}

	// Original cause (only shown in debug mode)
	if eh.debugMode && err.Cause != nil {
		pterm.Println()
		debugStyle := pterm.NewStyle(pterm.FgGray)
		debugStyle.Println("🐛 Technical Details:")
		debugStyle.Println(err.Cause.Error())
	}

	pterm.Println()
}

// getErrorIcon returns an appropriate icon for each error type
func (eh *ErrorHandler) getErrorIcon(errorType ErrorType) string {
	icons := map[ErrorType]string{
		ConfigurationError:  "⚙️",
		NetworkError:        "🌐",
		AuthenticationError: "🔐",
		ProviderError:       "🤖",
		ValidationError:     "❌",
		SystemError:         "💥",
		UserError:           "👤",
	}

	if icon, exists := icons[errorType]; exists {
		return icon
	}
	return "❗"
}

// convertToUserFriendlyError converts generic errors to user-friendly format
func (eh *ErrorHandler) convertToUserFriendlyError(err error) *UserFriendlyError {
	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// Network-related errors
	if strings.Contains(errMsgLower, "connection refused") ||
		strings.Contains(errMsgLower, "no such host") ||
		strings.Contains(errMsgLower, "network unreachable") {
		return &UserFriendlyError{
			Type:    NetworkError,
			Title:   "Network Connection Error",
			Message: "Unable to connect to the AI service.",
			Suggestions: []string{
				"Check your internet connection",
				"Verify that the API endpoint is accessible",
				"Try switching to a different LLM provider",
				"Check if you're behind a corporate firewall",
			},
			HelpLink:  "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md#network-connectivity-problems",
			DebugInfo: errMsg,
			Cause:     err,
		}
	}

	// Authentication errors
	if strings.Contains(errMsgLower, "unauthorized") ||
		strings.Contains(errMsgLower, "invalid api key") ||
		strings.Contains(errMsgLower, "authentication failed") ||
		strings.Contains(errMsgLower, "401") {
		return &UserFriendlyError{
			Type:    AuthenticationError,
			Title:   "Authentication Failed",
			Message: "Your API credentials are invalid or expired.",
			Suggestions: []string{
				"Check that your API key is correctly configured",
				"Verify your API key is still valid and not expired",
				"For Gemini CLI: run 'gemini-cli auth login' to re-authenticate",
				"For OpenAI: get a new API key from https://platform.openai.com/api-keys",
			},
			HelpLink:  "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md#llm-provider-issues",
			DebugInfo: errMsg,
			Cause:     err,
		}
	}

	// Rate limiting errors
	if strings.Contains(errMsgLower, "rate limit") ||
		strings.Contains(errMsgLower, "quota exceeded") ||
		strings.Contains(errMsgLower, "429") {
		return &UserFriendlyError{
			Type:    ProviderError,
			Title:   "Rate Limit Exceeded",
			Message: "Too many requests sent to the AI service.",
			Suggestions: []string{
				"Wait a few minutes before trying again",
				"Consider upgrading your API plan for higher limits",
				"Switch to Gemini CLI which has higher free limits",
				"Enable caching to reduce API calls: aish config set cache.enabled true",
			},
			HelpLink:  "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md#llm-provider-issues",
			DebugInfo: errMsg,
			Cause:     err,
		}
	}

	// Configuration errors
	if strings.Contains(errMsgLower, "config") ||
		strings.Contains(errMsgLower, "configuration") {
		return &UserFriendlyError{
			Type:    ConfigurationError,
			Title:   "Configuration Error",
			Message: "There's an issue with AISH configuration.",
			Suggestions: []string{
				"Run 'aish init' to reconfigure AISH",
				"Check your configuration with 'aish config show'",
				"Reset configuration: rm -rf ~/.config/aish && aish init",
			},
			HelpLink:  "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md#configuration-problems",
			DebugInfo: errMsg,
			Cause:     err,
		}
	}

	// Generic error fallback
	return &UserFriendlyError{
		Type:    SystemError,
		Title:   "Unexpected Error",
		Message: "An unexpected error occurred.",
		Suggestions: []string{
			"Try the command again",
			"Check the troubleshooting guide for common issues",
			"Report this issue if it persists",
		},
		HelpLink:  "https://github.com/TonnyWong1052/aish/blob/main/docs/TROUBLESHOOTING.md",
		DebugInfo: errMsg,
		Cause:     err,
	}
}

// ShowSuccess displays a success message with styling
func (eh *ErrorHandler) ShowSuccess(message string) {
	successStyle := pterm.NewStyle(pterm.FgGreen, pterm.Bold)
	successStyle.Printf("✅ %s\n", message)
}

// ShowWarning displays a warning message with styling
func (eh *ErrorHandler) ShowWarning(message string) {
	warningStyle := pterm.NewStyle(pterm.FgYellow, pterm.Bold)
	warningStyle.Printf("⚠️  %s\n", message)
}

// ShowInfo displays an informational message with styling
func (eh *ErrorHandler) ShowInfo(message string) {
	infoStyle := pterm.NewStyle(pterm.FgCyan)
	infoStyle.Printf("ℹ️  %s\n", message)
}

// ConfirmAction asks the user to confirm an action
func (eh *ErrorHandler) ConfirmAction(message string) bool {
	confirmStyle := pterm.NewStyle(pterm.FgYellow)
	confirmStyle.Printf("❓ %s (y/N): ", message)

	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// ShowProgress creates a progress indicator for long operations
func (eh *ErrorHandler) ShowProgress(message string) *pterm.SpinnerPrinter {
	spinner, _ := pterm.DefaultSpinner.Start(message)
	return spinner
}

// Global error handler instance
var defaultErrorHandler = NewErrorHandler(false)

// SetDebugMode enables or disables debug mode for error display
func SetDebugMode(enabled bool) {
	defaultErrorHandler.debugMode = enabled
}

// HandleError is a convenience function that uses the default error handler
func HandleError(err error) {
	defaultErrorHandler.HandleError(err)
}

// ShowSuccess is a convenience function that uses the default error handler
func ShowSuccess(message string) {
	defaultErrorHandler.ShowSuccess(message)
}

// ShowWarning is a convenience function that uses the default error handler
func ShowWarning(message string) {
	defaultErrorHandler.ShowWarning(message)
}

// ShowInfo is a convenience function that uses the default error handler
func ShowInfo(message string) {
	defaultErrorHandler.ShowInfo(message)
}

// ConfirmAction is a convenience function that uses the default error handler
func ConfirmAction(message string) bool {
	return defaultErrorHandler.ConfirmAction(message)
}