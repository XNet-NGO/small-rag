package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Engine handles embedding generation using llama.cpp server
type Engine struct {
	modelPath  string
	dims       int
	mu         sync.Mutex
	cache      map[string][]float32
	serverCmd  *exec.Cmd
	serverURL  string
	httpClient *http.Client
	started    bool
}

// NewEngine creates a new embedding engine
func NewEngine(modelPath string, dims int) *Engine {
	return &Engine{
		modelPath: modelPath,
		dims:      dims,
		cache:     make(map[string][]float32),
		serverURL: "http://127.0.0.1:8766",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		started: false,
	}
}

// Initialize starts the llama.cpp server
func (e *Engine) Initialize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return nil
	}

	// Check if model exists
	if _, err := os.Stat(e.modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", e.modelPath)
	}

	fmt.Printf("Starting llama.cpp server...\n")
	fmt.Printf("Model: %s\n", filepath.Base(e.modelPath))
	
	// Find llama-server binary
	llamaServer, err := exec.LookPath("llama-server")
	if err != nil {
		// Try common locations
		possiblePaths := []string{
			"/usr/local/bin/llama-server",
			"/usr/bin/llama-server",
			filepath.Join(os.Getenv("HOME"), ".local/bin/llama-server"),
			"./llama-server",
		}
		
		found := false
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				llamaServer = path
				found = true
				break
			}
		}
		
		if !found {
			return fmt.Errorf("llama-server not found in PATH. Please install llama.cpp")
		}
	}

	// Start llama.cpp server in embedding mode
	e.serverCmd = exec.Command(
		llamaServer,
		"-m", e.modelPath,
		"--port", "8766",
		"--embedding",
		"-c", "512",       // Context size
		"-t", "4",         // Threads
		"--log-disable",   // Disable logging
	)

	// Capture stderr for debugging
	stderr, err := e.serverCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the server
	if err := e.serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	// Read stderr in background
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				break
			}
			if n > 0 {
				// Silently discard or log if needed
				_ = buf[:n]
			}
		}
	}()

	// Wait for server to be ready
	fmt.Printf("Waiting for server to start")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.serverCmd.Process.Kill()
			return fmt.Errorf("server startup timeout")
		case <-ticker.C:
			fmt.Printf(".")
			// Try to connect
			resp, err := e.httpClient.Get(e.serverURL + "/health")
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == 200 {
					fmt.Printf(" ✅\n")
					e.started = true
					fmt.Printf("Server ready at %s\n", e.serverURL)
					fmt.Printf("Embedding dimensions: %d\n", e.dims)
					return nil
				}
			}
		}
	}
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

	// Ensure server is running
	if !e.started {
		if err := e.Initialize(); err != nil {
			return nil, err
		}
	}

	// Make embedding request
	reqBody := map[string]interface{}{
		"input": text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", e.serverURL+"/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response (OpenAI format)
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	embedding := result.Data[0].Embedding

	// Normalize
	embedding = normalize(embedding)

	// Ensure correct dimensions
	if len(embedding) != e.dims {
		if len(embedding) < e.dims {
			padded := make([]float32, e.dims)
			copy(padded, embedding)
			embedding = padded
		} else {
			embedding = embedding[:e.dims]
		}
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

// normalize converts vector to unit length
func normalize(vec []float32) []float32 {
	var norm float32 = 0
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm > 0 {
		result := make([]float32, len(vec))
		for i, v := range vec {
			result[i] = v / norm
		}
		return result
	}

	return vec
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

// Close stops the llama.cpp server
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.serverCmd != nil && e.serverCmd.Process != nil {
		fmt.Println("Stopping llama.cpp server...")
		e.serverCmd.Process.Kill()
		e.serverCmd.Wait()
		e.started = false
	}

	return nil
}
