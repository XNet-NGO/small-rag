package embedding

import (
	"fmt"
	"os"
	"sync"

	"github.com/xnet-admin-1/small-rag/internal/llamacpp"
)

// Engine handles embedding generation using llamacpp directly (in-process)
type Engine struct {
	modelPath string
	dims      int
	model     *llamacpp.Model
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

// Initialize loads the GGUF model for embedding generation
func (e *Engine) Initialize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.model != nil {
		return nil
	}

	// Check if model exists
	if _, err := os.Stat(e.modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", e.modelPath)
	}

	fmt.Printf("Loading embedding model: %s\n", e.modelPath)

	model, err := llamacpp.LoadModel(e.modelPath, 512, 4)
	if err != nil {
		return fmt.Errorf("failed to load model: %w", err)
	}

	e.model = model
	e.dims = model.EmbeddingDims()
	fmt.Printf("Embedding model loaded (dims=%d)\n", e.dims)

	return nil
}

// Embed generates embedding for text
func (e *Engine) Embed(text string) ([]float32, error) {
	if len(text) == 0 {
		return make([]float32, e.dims), nil
	}

	// Check cache
	e.mu.Lock()
	if cached, ok := e.cache[text]; ok {
		e.mu.Unlock()
		return cached, nil
	}
	e.mu.Unlock()

	// Ensure model is loaded
	if e.model == nil {
		if err := e.Initialize(); err != nil {
			return nil, err
		}
	}

	// Generate embedding via llamacpp
	embedding, err := e.model.Embed(text)
	if err != nil {
		return nil, fmt.Errorf("embedding generation failed: %w", err)
	}

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
			return nil, fmt.Errorf("batch embedding failed at index %d: %w", i, err)
		}
		embeddings[i] = emb
	}
	return embeddings, nil
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

// Close frees the model
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.model != nil {
		fmt.Println("Freeing embedding model...")
		e.model.Close()
		e.model = nil
	}

	return nil
}
