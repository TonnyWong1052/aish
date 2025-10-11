package classification

import (
	"strings"
	"testing"
)

// BenchmarkClassifier_Classify benchmarks the error classification performance
func BenchmarkClassifier_Classify(b *testing.B) {
	classifier := NewClassifier()

	benchmarks := []struct {
		name     string
		exitCode int
		stdout   string
		stderr   string
	}{
		{
			name:     "command not found",
			exitCode: 127,
			stdout:   "",
			stderr:   "bash: unknowncmd: command not found",
		},
		{
			name:     "permission denied",
			exitCode: 126,
			stdout:   "",
			stderr:   "permission denied: /restricted/file",
		},
		{
			name:     "syntax error",
			exitCode: 2,
			stdout:   "",
			stderr:   "SyntaxError: Unexpected token",
		},
		{
			name:     "network error",
			exitCode: 1,
			stdout:   "",
			stderr:   "curl: (6) Could not resolve host: example.com",
		},
		{
			name:     "file not found",
			exitCode: 1,
			stdout:   "",
			stderr:   "cat: nonexistent.txt: No such file or directory",
		},
		{
			name:     "large stderr output",
			exitCode: 1,
			stdout:   "",
			stderr:   strings.Repeat("Error line\n", 1000),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = classifier.Classify(bm.exitCode, bm.stdout, bm.stderr)
			}
		})
	}
}

// BenchmarkClassifier_ClassifyParallel benchmarks concurrent classification
func BenchmarkClassifier_ClassifyParallel(b *testing.B) {
	classifier := NewClassifier()
	exitCode := 127
	stdout := ""
	stderr := "bash: unknowncmd: command not found"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = classifier.Classify(exitCode, stdout, stderr)
		}
	})
}

// BenchmarkClassifier_MultiplePatterns benchmarks classification with multiple pattern matches
func BenchmarkClassifier_MultiplePatterns(b *testing.B) {
	classifier := NewClassifier()

	// Create a complex error message that might match multiple patterns
	complexError := `
Error: Connection failed
bash: command not found
Permission denied
SyntaxError: unexpected token
ImportError: No module named 'numpy'
fatal: not a git repository
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = classifier.Classify(1, "", complexError)
	}
}

// BenchmarkNewClassifier benchmarks classifier instantiation
func BenchmarkNewClassifier(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewClassifier()
	}
}

// BenchmarkClassifier_EdgeCases benchmarks edge case handling
func BenchmarkClassifier_EdgeCases(b *testing.B) {
	classifier := NewClassifier()

	cases := []struct {
		name     string
		exitCode int
		stdout   string
		stderr   string
	}{
		{
			name:     "empty output",
			exitCode: 0,
			stdout:   "",
			stderr:   "",
		},
		{
			name:     "very long single line",
			exitCode: 1,
			stdout:   strings.Repeat("a", 10000),
			stderr:   "",
		},
		{
			name:     "unicode characters",
			exitCode: 1,
			stdout:   "",
			stderr:   "エラー: ファイルが見つかりません 文件未找到 파일을 찾을 수 없습니다",
		},
		{
			name:     "special characters",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: Invalid regex pattern: [a-z]{1,",
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = classifier.Classify(tc.exitCode, tc.stdout, tc.stderr)
			}
		})
	}
}

// BenchmarkClassifier_MemoryAllocation benchmarks memory allocation patterns
func BenchmarkClassifier_MemoryAllocation(b *testing.B) {
	classifier := NewClassifier()
	stderr := "bash: unknowncmd: command not found"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = classifier.Classify(127, "", stderr)
	}
}
