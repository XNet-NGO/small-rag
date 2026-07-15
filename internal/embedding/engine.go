package embedding

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"

	"github.com/hybridgroup/yzma/pkg/llama"
)

// Engine handles embedding generation using yzma (llama.cpp via purego)
type Engine struct {
	modelPath string
	libPath   string
	dims      int
	mu        sync.Mutex
	cache     map[string][]float32
	model     llama.Model
	ctx       llama.Context
	vocab     llama.Vocab
	loaded    bool
}

// NewEngine creates a new embedding engine
func NewEngine(modelPath string, dims int) *Engine {
	return &Engine{
		modelPath: modelPath,
		dims:      dims,
		cache:     make(map[string][]float32),
	}
}

// SetLibPath sets the path to llama.cpp shared libraries
func (e *Engine) SetLibPath(path string) {
	e.libPath = path
}

// Initialize loads the llama.cpp library and the GGUF model
func (e *Engine) Initialize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.loaded {
		return nil
	}

	// Check model exists
	if _, err := os.Stat(e.modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s (run 'small-rag install' to download)", e.modelPath)
	}

	// Load llama.cpp shared library
	libPath := e.libPath
	if libPath == "" {
		libPath = os.Getenv("YZMA_LIB")
	}
	if libPath == "" {
		// Try common locations relative to binary
		candidates := []string{
			"./lib",
			"./llama/lib",
			"/usr/local/lib",
			"/usr/lib",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				libPath = p
				break
			}
		}
	}

	if err := llama.Load(libPath); err != nil {
		return fmt.Errorf("failed to load llama.cpp library from %q: %w (run 'small-rag install' to download)", libPath, err)
	}

	// Suppress llama.cpp log output
	llama.LogSet(llama.LogSilent())

	// Initialize backend
	llama.Init()

	// Load model
	fmt.Printf("Loading embedding model: %s\n", e.modelPath)
	mparams := llama.ModelDefaultParams()
	model, err := llama.ModelLoadFromFile(e.modelPath, mparams)
	if err != nil {
		return fmt.Errorf("failed to load model: %w", err)
	}
	e.model = model

	// Get embedding dimensions from model
	e.dims = int(llama.ModelNEmbd(model))

	// Create context with embedding support
	cparams := llama.ContextDefaultParams()
	cparams.NCtx = 256
	cparams.NBatch = 256
	cparams.NThreads = int32(runtime.NumCPU())
	cparams.NThreadsBatch = int32(runtime.NumCPU())
	cparams.Embeddings = 1 // enable embeddings (uint8 bool)

	ctx, err := llama.InitFromModel(model, cparams)
	if err != nil {
		return fmt.Errorf("failed to create context: %w", err)
	}
	e.ctx = ctx

	// Get vocab
	e.vocab = llama.ModelGetVocab(model)

	e.loaded = true
	fmt.Printf("Embedding model loaded (dims=%d)\n", e.dims)

	return nil
}

// Embed generates a normalized embedding vector for the given text
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

	// Ensure initialized
	if !e.loaded {
		if err := e.Initialize(); err != nil {
			return nil, err
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Tokenize
	tokens := llama.Tokenize(e.vocab, text, true, true)
	if len(tokens) == 0 {
		return make([]float32, e.dims), nil
	}

	// Truncate to context size
	if len(tokens) > 256 {
		tokens = tokens[:256]
	}

	// Clear memory
	mem, err := llama.GetMemory(e.ctx)
	if err == nil {
		llama.MemoryClear(mem, true)
	}

	// Create batch and decode
	batch := llama.BatchGetOne(tokens)
	ret, err := llama.Decode(e.ctx, batch)
	if err != nil || ret != 0 {
		return nil, fmt.Errorf("decode failed: ret=%d err=%v", ret, err)
	}

	// Get sequence embeddings
	embedding, err := llama.GetEmbeddingsSeq(e.ctx, 0, int32(e.dims))
	if err != nil || embedding == nil {
		// Fallback: try getting embeddings for last token
		embedding, err = llama.GetEmbeddingsIth(e.ctx, -1, int32(e.dims))
		if err != nil || embedding == nil {
			return nil, fmt.Errorf("failed to get embeddings: %v", err)
		}
	}

	// Copy and normalize
	result := make([]float32, len(embedding))
	copy(result, embedding)
	normalize(result)

	// Cache
	e.cache[text] = result

	return result, nil
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

// Dims returns the embedding dimensions
func (e *Engine) Dims() int {
	return e.dims
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

// Close frees model resources
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ctx != 0 {
		llama.Free(e.ctx)
		e.ctx = 0
	}
	if e.model != 0 {
		llama.ModelFree(e.model)
		e.model = 0
	}
	e.loaded = false
	return nil
}

// normalize converts a vector to unit length
func normalize(vec []float32) {
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
}
