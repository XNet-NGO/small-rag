package llm

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// LocalEngine manages a llama-server subprocess for chat inference
type LocalEngine struct {
	cmd       *exec.Cmd
	modelPath string
	modelName string
	port      string
	ready     bool
	mu        sync.Mutex
}

// NewLocalEngine creates a new local LLM engine
func NewLocalEngine() *LocalEngine {
	return &LocalEngine{
		port: "8767",
	}
}

// IsLoaded returns whether a model is currently loaded
func (e *LocalEngine) IsLoaded() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ready
}

// ModelName returns the name of the currently loaded model
func (e *LocalEngine) ModelName() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.modelName
}

// URL returns the base URL for the running server
func (e *LocalEngine) URL() string {
	return "http://127.0.0.1:" + e.port + "/v1"
}

// LoadModel starts llama-server with the specified model
func (e *LocalEngine) LoadModel(modelPath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found: %s", modelPath)
	}

	// Kill existing server
	e.stopLocked()

	// Find llama-server binary
	llamaServer, err := exec.LookPath("llama-server")
	if err != nil {
		// Try common locations
		for _, p := range []string{"/usr/local/bin/llama-server", "/usr/bin/llama-server"} {
			if _, err := os.Stat(p); err == nil {
				llamaServer = p
				break
			}
		}
		if llamaServer == "" {
			return fmt.Errorf("llama-server not found in PATH")
		}
	}

	// Start llama-server
	e.cmd = exec.Command(
		llamaServer,
		"-m", modelPath,
		"--port", e.port,
		"--no-webui",
		"--log-disable",
		"-c", "4096",
		"-t", "4",
		"--jinja",
	)
	e.cmd.Stdout = nil
	e.cmd.Stderr = nil

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	// Wait for server to be ready
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get("http://127.0.0.1:" + e.port + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				e.modelPath = modelPath
				name := filepath.Base(modelPath)
				if len(name) > 5 && name[len(name)-5:] == ".gguf" {
					name = name[:len(name)-5]
				}
				e.modelName = name
				e.ready = true
				log.Printf("Chat model ready: %s (port %s)", e.modelName, e.port)
				return nil
			}
		}
	}

	// Timeout - kill the process
	e.stopLocked()
	return fmt.Errorf("llama-server failed to start within 30s")
}

// GenerateChat sends a chat completion request to the managed server
// This is handled by the HTTP client pointing at our managed server
// The server.go code uses s.llmClient which we update the URL for

// Close stops the llama-server
func (e *LocalEngine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()
}

func (e *LocalEngine) stopLocked() {
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
		e.cmd.Wait()
		e.cmd = nil
	}
	e.ready = false
	e.modelName = ""
	e.modelPath = ""
}
