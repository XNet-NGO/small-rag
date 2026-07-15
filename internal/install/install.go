package install

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ModelURL  = "https://huggingface.co/PeterAM4/Qwen3-Embedding-0.6B-GGUF/resolve/main/Qwen3-Embedding-0.6B-Q4_K_M-imat.gguf"
	ModelName = "qwen3-embedding-0.6b-q4_k_m.gguf"
	ModelSize = 378_000_000 // ~378MB
)

// Run executes the install command
func Run() error {
	baseDir := getBaseDir()

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║       Small-RAG Install                  ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Printf("\nInstall directory: %s\n\n", baseDir)

	// Step 1: Create directory structure
	fmt.Println("📁 Creating directory structure...")
	dirs := []string{
		filepath.Join(baseDir, "llama", "models"),
		filepath.Join(baseDir, ".small-rag-db"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	fmt.Println("   ✅ Directories created")

	// Step 2: Download model
	modelPath := filepath.Join(baseDir, "llama", "models", ModelName)
	if _, err := os.Stat(modelPath); err == nil {
		fmt.Printf("\n📦 Model already exists: %s\n", ModelName)
		fmt.Println("   ✅ Skipping download")
	} else {
		fmt.Printf("\n⬇️  Downloading model: %s\n", ModelName)
		fmt.Printf("   Source: %s\n", ModelURL)
		if err := downloadModel(modelPath); err != nil {
			return fmt.Errorf("failed to download model: %w", err)
		}
		fmt.Println("\n   ✅ Model downloaded")
	}

	// Step 3: Write default config
	configPath := filepath.Join(baseDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		fmt.Println("\n⚙️  Writing default config...")
		if err := writeDefaultConfig(baseDir); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Println("   ✅ Config created")
	}

	// Step 4: Verify installation
	fmt.Println("\n🔍 Verifying installation...")
	if err := verify(baseDir); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	fmt.Println("\n╔══════════════════════════════════════════╗")
	fmt.Println("║       ✅ Installation Complete!          ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Printf("\nDirectory structure:\n")
	fmt.Printf("  %s/\n", baseDir)
	fmt.Printf("  ├── small-rag              (this binary)\n")
	fmt.Printf("  ├── config.json\n")
	fmt.Printf("  ├── .small-rag-db/\n")
	fmt.Printf("  │   └── small-rag.db       (created on first run)\n")
	fmt.Printf("  └── llama/\n")
	fmt.Printf("      └── models/\n")
	fmt.Printf("          └── %s\n", ModelName)
	fmt.Printf("\nRun: ./small-rag\n")

	return nil
}

// getBaseDir returns the directory where the binary lives
func getBaseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return filepath.Dir(exe)
	}
	return filepath.Dir(exe)
}

// downloadModel downloads the GGUF model with progress
func downloadModel(destPath string) error {
	// Create temp file for download
	tmpPath := destPath + ".tmp"
	defer os.Remove(tmpPath)

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	// Start download
	resp, err := http.Get(ModelURL)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = ModelSize
	}

	// Download with progress
	var downloaded int64
	buf := make([]byte, 32*1024) // 32KB buffer
	lastPrint := time.Now()
	startTime := time.Now()

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("write error: %w", writeErr)
			}
			downloaded += int64(n)

			// Print progress every 500ms
			if time.Since(lastPrint) > 500*time.Millisecond {
				printProgress(downloaded, totalSize, startTime)
				lastPrint = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download error: %w", err)
		}
	}

	// Final progress
	printProgress(downloaded, totalSize, startTime)
	fmt.Println()

	// Close file before rename
	out.Close()

	// Move temp file to final location
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to move downloaded file: %w", err)
	}

	return nil
}

// printProgress prints a progress bar
func printProgress(downloaded, total int64, startTime time.Time) {
	pct := float64(downloaded) / float64(total) * 100
	if pct > 100 {
		pct = 100
	}

	elapsed := time.Since(startTime).Seconds()
	speed := float64(downloaded) / elapsed / 1024 / 1024 // MB/s

	barWidth := 30
	filled := int(pct / 100 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Printf("\r   [%s] %.1f%% (%.0f/%.0f MB) %.1f MB/s",
		bar, pct,
		float64(downloaded)/1024/1024,
		float64(total)/1024/1024,
		speed)
}

// writeDefaultConfig writes a default config.json
func writeDefaultConfig(baseDir string) error {
	config := `{
  "embedding_model": "qwen3-embedding-0.6b",
  "embedding_dims": 1024,
  "chunk_size": 512,
  "chunk_overlap": 128,
  "search_types": ["semantic", "keyword", "hybrid"],
  "min_score": 0.3,
  "default_llm_provider": "openai",
  "default_model": "gpt-4",
  "port": 8765,
  "enable_cache": true,
  "enable_sse": true
}
`
	return os.WriteFile(filepath.Join(baseDir, "config.json"), []byte(config), 0644)
}

// verify checks that all required files exist
func verify(baseDir string) error {
	required := []string{
		filepath.Join(baseDir, "llama", "models", ModelName),
	}

	for _, path := range required {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("missing required file: %s", path)
		}
		fmt.Printf("   ✅ %s\n", filepath.Base(path))
	}

	return nil
}
