package embedding

import (
	"crypto/sha256"
	"fmt"
	"math"
)

// MockEngine generates deterministic mock embeddings for testing
// In Phase 2, this will be replaced with real Qwen3-0.6B embeddings
type MockEngine struct {
	dims int
}

func NewMockEngine(dims int) *MockEngine {
	if dims <= 0 {
		dims = 384 // Default for Qwen3-0.6B
	}
	return &MockEngine{dims: dims}
}

// Embed generates a deterministic embedding for the given text
func (e *MockEngine) Embed(text string) ([]float32, error) {
	if text == "" {
		return make([]float32, e.dims), nil
	}

	// Use SHA256 hash of text as seed for reproducible embeddings
	hash := sha256.Sum256([]byte(text))
	
	// Generate deterministic float32 values from hash
	embedding := make([]float32, e.dims)
	
	for i := 0; i < e.dims; i++ {
		// Use different bytes from hash for each dimension
		byteIndex := (i * 4) % len(hash)
		
		// Convert 4 bytes to uint32, then to float32
		var val uint32
		for j := 0; j < 4; j++ {
			val = (val << 8) | uint32(hash[(byteIndex+j)%len(hash)])
		}
		
		// Convert to float in range [-1, 1]
		f := float32(val) / float32(math.MaxUint32)
		embedding[i] = (f * 2) - 1
	}
	
	// Normalize to unit vector (L2 norm = 1)
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	
	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *MockEngine) EmbedBatch(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	
	for i, text := range texts {
		emb, err := e.Embed(text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = emb
	}
	
	return embeddings, nil
}

// GetDimensions returns the embedding dimension
func (e *MockEngine) GetDimensions() int {
	return e.dims
}

// GetModelID returns the model identifier
func (e *MockEngine) GetModelID() string {
	return "mock-embedding-384"
}
