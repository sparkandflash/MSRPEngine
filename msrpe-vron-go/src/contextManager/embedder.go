package contextManager

import (
	"context"

	"github.com/philippgille/chromem-go"
)

// LocalEmbedder handles the creation of text embeddings.
// We port this over from V2 to explicitly inject it into chromem-go fetches,
// preventing the silent panics when chromem-go defaults to an empty embedder.
type LocalEmbedder struct {
	// In production, this connects to the Embedding API (OpenAI/Ollama).
	// For skeleton testing, it provides a safe mock to prevent nil dereference.
}

// NewLocalEmbedder initializes the embedding client.
func NewLocalEmbedder() *LocalEmbedder {
	return &LocalEmbedder{}
}

// AsChromemEmbeddingFunc returns the interface required by chromem-go.
func (le *LocalEmbedder) AsChromemEmbeddingFunc() chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		// Skeleton Mock: Return a dummy vector so chromem-go doesn't crash on insert
		return []float32{0.1, 0.2, 0.3}, nil
	}
}
