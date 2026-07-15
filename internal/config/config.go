package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents application configuration
type Config struct {
	// Embedding model settings
	EmbeddingModel string `json:"embedding_model"`
	EmbeddingDims  int    `json:"embedding_dims"`

	// Chunking settings
	ChunkSize    int `json:"chunk_size"`
	ChunkOverlap int `json:"chunk_overlap"`

	// Search settings
	SearchTypes []string `json:"search_types"`
	MinScore    float32  `json:"min_score"`

	// LLM settings
	DefaultLLMProvider string `json:"default_llm_provider"`
	DefaultModel       string `json:"default_model"`

	// Server settings
	Port int `json:"port"`

	// Feature flags
	EnableCache bool `json:"enable_cache"`
	EnableSSE   bool `json:"enable_sse"`

	// Runtime paths (set at startup, not serialized)
	ModelPath string `json:"-"`
	LibPath   string `json:"-"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		EmbeddingModel:     "all-MiniLM-L6-v2",
		EmbeddingDims:      384,
		ChunkSize:          1024,
		ChunkOverlap:       256,
		SearchTypes:        []string{"semantic", "keyword", "hybrid"},
		MinScore:           0.3,
		DefaultLLMProvider: "openai",
		DefaultModel:       "gpt-4",
		Port:               8765,
		EnableCache:        true,
		EnableSSE:          true,
	}
}

// Load loads configuration from file or returns default
func Load(dataDir string) (*Config, error) {
	configPath := filepath.Join(dataDir, "config.json")

	// If config exists, load it
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}

	// Otherwise, create default and save
	cfg := DefaultConfig()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)

	return cfg, nil
}

// Save persists configuration to file
func (c *Config) Save(dataDir string) error {
	configPath := filepath.Join(dataDir, "config.json")
	data, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(configPath, data, 0644)
}

// LoadFromFile loads configuration from a specific file path
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
