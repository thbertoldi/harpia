package agents

import "context"

type noopEmbedder struct{}

func NewNoopEmbedder() Embedder {
	return &noopEmbedder{}
}

func (e *noopEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	vec := make([]float32, 1536)
	return vec, nil
}
