package security

import (
	"regexp"
	"strings"
)

// SensitiveDataSanitizer 敏感數據清理器
type SensitiveDataSanitizer struct {
	patterns []SanitizePattern
	enabled  bool
}

// SanitizePattern 清理模式
type SanitizePattern struct {
	Name        string         `json:"name"`
	Pattern     *regexp.Regexp `json:"-"`
	Replacement string         `json:"replacement"`
	Enabled     bool           `json:"enabled"`
	Priority    int            `json:"priority"`
}

// NewSensitiveDataSanitizer 創建新的敏感數據清理器
func NewSensitiveDataSanitizer() *SensitiveDataSanitizer {
	sanitizer := &SensitiveDataSanitizer{
		enabled:  true,
		patterns: make([]SanitizePattern, 0),
	}
	
	// 添加默認模式
	sanitizer.AddDefaultPatterns()
	
	return sanitizer
}

// AddDefaultPatterns 添加默認的清理模式
func (s *SensitiveDataSanitizer) AddDefaultPatterns() {
	defaultPatterns := []struct {
		name        string
		pattern     string
		replacement string
		priority    int
	}{
		// API 密鑰和令牌
		{"api_key", `(?i)(api[_-]?key|apikey)\s*[:=]\s*["\']?([a-zA-Z0-9._-]{16,})["\']?`, "***REDACTED_API_KEY***", 10},
		{"bearer_token", `(?i)(bearer\s+)([a-zA-Z0-9._-]{20,})`, "$1***REDACTED_TOKEN***", 10},
		{"authorization_header", `(?i)(authorization\s*:\s*)(bearer\s+)?([a-zA-Z0-9._-]{20,})`, "$1$2***REDACTED_AUTH***", 10},
		
		// 密碼
		{"password", `(?i)(password|pwd|pass)\s*[:=]\s*["\']?([^\s"'\n]{4,})["\']?`, "$1=***REDACTED_PASSWORD***", 9},
		{"secret", `(?i)(secret|secret_key)\s*[:=]\s*["\']?([a-zA-Z0-9._-]{8,})["\']?`, "$1=***REDACTED_SECRET***", 9},
		
		// 環境變量
		{"env_var_key", `(?i)([A-Z][A-Z0-9_]*(?:KEY|SECRET|TOKEN|PASSWORD|PWD))\s*[:=]\s*["\']?([^\s"'\n]+)["\']?`, "$1=***REDACTED***", 8},
		
		// 數據庫連接字符串
		{"db_connection", `(?i)(mysql|postgres|mongodb|redis)://[^:]+:([^@/]+)@`, "$1://username:***REDACTED***@", 8},
		
		// 信用卡號碼
		{"credit_card", `\b(?:\d{4}[\s-]?){3}\d{4}\b`, "****-****-****-****", 7},
		
		// 社會安全號碼 (美國)
		{"ssn", `\b\d{3}-\d{2}-\d{4}\b`, "***-**-****", 7},
		
		// 電子郵件地址 (部分遮蔽)
		{"email", `\b([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})\b`, "***@$2", 5},
		
		// IP 地址 (私有網絡除外)
		{"public_ip", `\b(?!(?:10|127|169\.254|172\.(?:1[6-9]|2[0-9]|3[01])|192\.168)\.)(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`, "***IP_ADDRESS***", 4},
		
		// JWT 令牌
		{"jwt_token", `\beyJ[a-zA-Z0-9._-]+\.[a-zA-Z0-9._-]+\.[a-zA-Z0-9._-]+\b`, "***JWT_TOKEN***", 9},
		
		// 私鑰
		{"private_key", `-----BEGIN (?:RSA )?PRIVATE KEY-----[\s\S]*?-----END (?:RSA )?PRIVATE KEY-----`, "***PRIVATE_KEY_REDACTED***", 10},
		
		// AWS 訪���密鑰
		{"aws_access_key", `\b(AKIA[0-9A-Z]{16})\b`, "***AWS_ACCESS_KEY***", 10},
		{"aws_secret_key", `\b([a-zA-Z0-9/+=]{40})\b`, "***AWS_SECRET_KEY***", 9},
		
		// GitHub 令牌
		{"github_token", `\bgh[pousr]_[A-Za-z0-9_]{36,}\b`, "***GITHUB_TOKEN***", 10},
		
		// 常見的 shell 參數
		{"command_args", `(?i)(--?(?:password|pwd|pass|key|secret|token)[\s=])([^\s]+)`, "$1***REDACTED***", 8},
	}
	
	for _, pattern := range defaultPatterns {
		regex, err := regexp.Compile(pattern.pattern)
		if err != nil {
			continue // 跳過無效的正則表達式
		}
		
		s.patterns = append(s.patterns, SanitizePattern{
			Name:        pattern.name,
			Pattern:     regex,
			Replacement: pattern.replacement,
			Enabled:     true,
			Priority:    pattern.priority,
		})
	}
	
	// 按優先級排序
	s.sortPatternsByPriority()
}

// sortPatternsByPriority 按優先級排序模式
func (s *SensitiveDataSanitizer) sortPatternsByPriority() {
	for i := 0; i < len(s.patterns)-1; i++ {
		for j := i + 1; j < len(s.patterns); j++ {
			if s.patterns[i].Priority < s.patterns[j].Priority {
				s.patterns[i], s.patterns[j] = s.patterns[j], s.patterns[i]
			}
		}
	}
}

// Sanitize 清理敏感數據
func (s *SensitiveDataSanitizer) Sanitize(text string) string {
	if !s.enabled || text == "" {
		return text
	}
	
	result := text
	
	for _, pattern := range s.patterns {
		if pattern.Enabled && pattern.Pattern != nil {
			result = pattern.Pattern.ReplaceAllString(result, pattern.Replacement)
		}
	}
	
	return result
}

// SanitizeLines 逐行清理敏感數據
func (s *SensitiveDataSanitizer) SanitizeLines(lines []string) []string {
	if !s.enabled {
		return lines
	}
	
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = s.Sanitize(line)
	}
	
	return result
}

// SanitizeMap 清理 map 中的敏感數據
func (s *SensitiveDataSanitizer) SanitizeMap(data map[string]interface{}) map[string]interface{} {
	if !s.enabled {
		return data
	}
	
	result := make(map[string]interface{})
	
	for key, value := range data {
		sanitizedKey := s.Sanitize(key)
		
		switch v := value.(type) {
		case string:
			result[sanitizedKey] = s.Sanitize(v)
		case map[string]interface{}:
			result[sanitizedKey] = s.SanitizeMap(v)
		case []interface{}:
			result[sanitizedKey] = s.sanitizeSlice(v)
		default:
			// 將其他類型轉換為字符串並清理
			if str := valueToString(v); str != "" {
				result[sanitizedKey] = s.Sanitize(str)
			} else {
				result[sanitizedKey] = v
			}
		}
	}
	
	return result
}

// sanitizeSlice 清理切片中的敏感數據
func (s *SensitiveDataSanitizer) sanitizeSlice(slice []interface{}) []interface{} {
	result := make([]interface{}, len(slice))
	
	for i, item := range slice {
		switch v := item.(type) {
		case string:
			result[i] = s.Sanitize(v)
		case map[string]interface{}:
			result[i] = s.SanitizeMap(v)
		case []interface{}:
			result[i] = s.sanitizeSlice(v)
		default:
			if str := valueToString(v); str != "" {
				result[i] = s.Sanitize(str)
			} else {
				result[i] = v
			}
		}
	}
	
	return result
}

// AddPattern 添加自定義清理模式
func (s *SensitiveDataSanitizer) AddPattern(name, pattern, replacement string, priority int) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	
	s.patterns = append(s.patterns, SanitizePattern{
		Name:        name,
		Pattern:     regex,
		Replacement: replacement,
		Enabled:     true,
		Priority:    priority,
	})
	
	s.sortPatternsByPriority()
	return nil
}

// RemovePattern 移除清理模式
func (s *SensitiveDataSanitizer) RemovePattern(name string) {
	for i, pattern := range s.patterns {
		if pattern.Name == name {
			s.patterns = append(s.patterns[:i], s.patterns[i+1:]...)
			break
		}
	}
}

// EnablePattern 啟用模式
func (s *SensitiveDataSanitizer) EnablePattern(name string) {
	for i := range s.patterns {
		if s.patterns[i].Name == name {
			s.patterns[i].Enabled = true
			break
		}
	}
}

// DisablePattern 禁用模式
func (s *SensitiveDataSanitizer) DisablePattern(name string) {
	for i := range s.patterns {
		if s.patterns[i].Name == name {
			s.patterns[i].Enabled = false
			break
		}
	}
}

// GetPatterns 獲取所有模式
func (s *SensitiveDataSanitizer) GetPatterns() []SanitizePattern {
	patterns := make([]SanitizePattern, len(s.patterns))
	copy(patterns, s.patterns)
	return patterns
}

// SetEnabled 設置清理器啟用狀態
func (s *SensitiveDataSanitizer) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// IsEnabled 檢查清理器是否啟用
func (s *SensitiveDataSanitizer) IsEnabled() bool {
	return s.enabled
}

// ContainsSensitiveData 檢查文本是否包含敏感數據
func (s *SensitiveDataSanitizer) ContainsSensitiveData(text string) bool {
	if !s.enabled || text == "" {
		return false
	}
	
	for _, pattern := range s.patterns {
		if pattern.Enabled && pattern.Pattern != nil {
			if pattern.Pattern.MatchString(text) {
				return true
			}
		}
	}
	
	return false
}

// GetMatchedPatterns 獲取匹配的模式
func (s *SensitiveDataSanitizer) GetMatchedPatterns(text string) []string {
	if !s.enabled || text == "" {
		return nil
	}
	
	var matched []string
	
	for _, pattern := range s.patterns {
		if pattern.Enabled && pattern.Pattern != nil {
			if pattern.Pattern.MatchString(text) {
				matched = append(matched, pattern.Name)
			}
		}
	}
	
	return matched
}

// SanitizeCommandLine 專門清理命令行參數
func (s *SensitiveDataSanitizer) SanitizeCommandLine(command string) string {
	if !s.enabled {
		return command
	}
	
	// 先應用通用清理
	result := s.Sanitize(command)
	
	// 額外的命令行特定清理
	commandPatterns := []struct {
		pattern     string
		replacement string
	}{
		// 常見的命令行參數模式
		{`(?i)(\s-[a-z]*p\s+|--password[\s=])([^\s]+)`, "$1***REDACTED***"},
		{`(?i)(\s-[a-z]*k\s+|--key[\s=])([^\s]+)`, "$1***REDACTED***"},
		{`(?i)(\s-[a-z]*t\s+|--token[\s=])([^\s]+)`, "$1***REDACTED***"},
		{`(?i)(export\s+[A-Z_]*(?:KEY|SECRET|TOKEN|PASSWORD)[^=]*=)([^\s;]+)`, "$1***REDACTED***"},
	}
	
	for _, pattern := range commandPatterns {
		if regex, err := regexp.Compile(pattern.pattern); err == nil {
			result = regex.ReplaceAllString(result, pattern.replacement)
		}
	}
	
	return result
}

// valueToString 將值轉換為字符串
func valueToString(v interface{}) string {
	if v == nil {
		return ""
	}
	
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return ""
	}
}

// GetDefaultSanitizer 獲取默認的全局清理器實例
var defaultSanitizer *SensitiveDataSanitizer

func GetDefaultSanitizer() *SensitiveDataSanitizer {
	if defaultSanitizer == nil {
		defaultSanitizer = NewSensitiveDataSanitizer()
	}
	return defaultSanitizer
}

// Quick helper functions for common use cases

// SanitizeText 快速清理文本
func SanitizeText(text string) string {
	return GetDefaultSanitizer().Sanitize(text)
}

// SanitizeCommand 快速清理命令
func SanitizeCommand(command string) string {
	return GetDefaultSanitizer().SanitizeCommandLine(command)
}

// ContainsSensitive 快速檢查是否包含敏感數據
func ContainsSensitive(text string) bool {
	return GetDefaultSanitizer().ContainsSensitiveData(text)
}