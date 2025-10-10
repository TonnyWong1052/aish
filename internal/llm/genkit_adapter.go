package llm

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// GenkitAdapter 封裝 Genkit 生成邏輯，提供統一介面給 providers 使用
type GenkitAdapter struct {
	g         *genkit.Genkit
	modelName string
}

// NewGenkitAdapter 建立新的 Genkit adapter 實例
func NewGenkitAdapter(g *genkit.Genkit, modelName string) *GenkitAdapter {
	return &GenkitAdapter{
		g:         g,
		modelName: modelName,
	}
}

// Generate 使用 Genkit 生成文字回應
func (a *GenkitAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	resp, err := genkit.Generate(ctx, a.g,
		ai.WithPrompt(prompt),
		ai.WithModelName(a.modelName),
	)
	if err != nil {
		return "", fmt.Errorf("genkit generate failed: %w", err)
	}

	if resp == nil {
		return "", fmt.Errorf("genkit returned nil response")
	}

	return resp.Text(), nil
}

// GenerateStructured 使用 Genkit 生成結構化輸出
// 這個函數使用 Go generics 提供類型安全的結構化輸出
func GenerateStructured[T any](ctx context.Context, a *GenkitAdapter, prompt string) (*T, error) {
	result, _, err := genkit.GenerateData[T](ctx, a.g,
		ai.WithPrompt(prompt),
		ai.WithModelName(a.modelName),
	)
	if err != nil {
		return nil, fmt.Errorf("genkit structured generate failed: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("genkit returned nil structured result")
	}

	return result, nil
}

// TestGeneration 測試生成功能，用於驗證連線
func (a *GenkitAdapter) TestGeneration(ctx context.Context) error {
	_, err := a.Generate(ctx, "Hello")
	return err
}
