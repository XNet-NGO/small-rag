package embedding

import (
	"fmt"
	"math"
	"sync"
)

// Engine handles embedding generation
type Engine struct {
	modelPath string
	dims      int
	mu        sync.Mutex
	cache     map[string][]float32
}

// NewEngine creates a new embedding engine
func NewEngine(modelPath string, dims int) *Engine {
	return &Engine{
		modelPath: modelPath,
		dims:      dims,
		cache:     make(map[string][]float32),
	}
}

// Initialize loads the model
func (e *Engine) Initialize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fmt.Printf("Initializing embedding model: %s\n", e.modelPath)
	fmt.Printf("Dimensions: %d\n", e.dims)

	return nil
}

// Embed generates embedding for text
func (e *Engine) Embed(text string) ([]float32, error) {
	if len(text) == 0 {
		return make([]float32, e.dims), nil
	}

	e.mu.Lock()

	// Check cache
	if cached, ok := e.cache[text]; ok {
		e.mu.Unlock()
		return cached, nil
	}

	e.mu.Unlock()

	// Generate deterministic placeholder embedding
	embedding := e.generatePlaceholderEmbedding(text)

	// Cache result
	e.mu.Lock()
	e.cache[text] = embedding
	e.mu.Unlock()

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *Engine) EmbedBatch(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// generatePlaceholderEmbedding creates a deterministic embedding for testing
func (e *Engine) generatePlaceholderEmbedding(text string) []float32 {
	embedding := make([]float32, e.dims)

	// Simple hash-based placeholder
	hash := uint32(0)
	for _, ch := range text {
		hash = hash*31 + uint32(ch)
	}

	// Fill embedding with pseudo-random values based on hash
	for i := 0; i < e.dims; i++ {
		seed := hash + uint32(i)
		seed = seed*1103515245 + 12345
		val := float32((seed/65536)%32768) / 32768.0
		embedding[i] = (val * 2) - 1
	}

	// Normalize to unit vector
	var norm float32 = 0
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}

// CacheSize returns current cache size
func (e *Engine) CacheSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.cache)
}

// ClearCache clears the embedding cache
func (e *Engine) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache = make(map[string][]float32)
}
