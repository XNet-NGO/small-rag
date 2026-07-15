package embedding

// Engine interface for embedding generation
// Implemented by:
// - *Engine (llama.cpp server-based)
// - *MockEngine (deterministic mock for testing)
type Embedder interface {
	Embed(text string) ([]float32, error)
	EmbedBatch(texts []string) ([][]float32, error)
}
