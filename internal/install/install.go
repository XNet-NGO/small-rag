package install

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Embedding model
	ModelURL  = "https://huggingface.co/second-state/All-MiniLM-L6-v2-Embedding-GGUF/resolve/main/all-MiniLM-L6-v2-Q8_0.gguf"
	ModelName = "all-MiniLM-L6-v2-Q8_0.gguf"
	ModelSize = 25_000_000 // ~25MB

	// Chat/LLM model
	ChatModelURL  = "https://huggingface.co/tiiuae/Falcon-H1-Tiny-90M-Instruct-GGUF/resolve/main/Falcon-H1-Tiny-90M-Instruct-Q8_0.gguf"
	ChatModelName = "falcon-h1-tiny-90m-instruct-Q8_0.gguf"
	ChatModelSize = 94_000_000 // ~94MB

	// llama.cpp shared library release (matches yzma v1.19.0 / llama.cpp b9979+)
	LlamaLibURL  = "https://github.com/hybridgroup/llama-cpp-builder/releases/download/b10361/llama-cpp-shared-libs-linux-x86_64.tar.gz"
	LlamaLibName = "llama-cpp-shared-libs-linux-x86_64.tar.gz"
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
		filepath.Join(baseDir, "lib"),
		filepath.Join(baseDir, "models"),
		filepath.Join(baseDir, ".small-rag-db"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	fmt.Println("   ✅ Directories created")

	// Step 2: Download llama.cpp shared libraries
	libDir := filepath.Join(baseDir, "lib")
	libMarker := filepath.Join(libDir, ".installed")
	if _, err := os.Stat(libMarker); err == nil {
		fmt.Println("\n📦 llama.cpp libraries already installed")
		fmt.Println("   ✅ Skipping download")
	} else {
		fmt.Printf("\n⬇️  Downloading llama.cpp libraries...\n")
		fmt.Printf("   Source: %s\n", LlamaLibURL)
		if err := downloadAndExtract(LlamaLibURL, libDir); err != nil {
			return fmt.Errorf("failed to download llama.cpp libraries: %w", err)
		}
		os.WriteFile(libMarker, []byte("installed"), 0644)
		fmt.Println("   ✅ Libraries installed")
	}

	// Step 3: Download model
	modelPath := filepath.Join(baseDir, "models", ModelName)
	if _, err := os.Stat(modelPath); err == nil {
		fmt.Printf("\n📦 Model already exists: %s\n", ModelName)
		fmt.Println("   ✅ Skipping download")
	} else {
		fmt.Printf("\n⬇️  Downloading model: %s\n", ModelName)
		fmt.Printf("   Source: %s\n", ModelURL)
		if err := downloadFile(modelPath, ModelURL, ModelSize); err != nil {
			return fmt.Errorf("failed to download model: %w", err)
		}
		fmt.Println("\n   ✅ Model downloaded")
	}

	// Step 4: Download chat model
	chatModelPath := filepath.Join(baseDir, "models", ChatModelName)
	if _, err := os.Stat(chatModelPath); err == nil {
		fmt.Printf("\n📦 Chat model already exists: %s\n", ChatModelName)
		fmt.Println("   ✅ Skipping download")
	} else {
		fmt.Printf("\n⬇️  Downloading chat model: %s\n", ChatModelName)
		fmt.Printf("   Source: %s\n", ChatModelURL)
		if err := downloadFile(chatModelPath, ChatModelURL, ChatModelSize); err != nil {
			return fmt.Errorf("failed to download chat model: %w", err)
		}
		fmt.Println("\n   ✅ Chat model downloaded")
	}

	// Step 5: Write default config
	configPath := filepath.Join(baseDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		fmt.Println("\n⚙️  Writing default config...")
		if err := writeDefaultConfig(baseDir); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Println("   ✅ Config created")
	}

	// Step 5: Verify installation
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
	fmt.Printf("  ├── lib/\n")
	fmt.Printf("  │   └── libllama.so (+ other .so files)\n")
	fmt.Printf("  └── models/\n")
	fmt.Printf("      ├── %s  (embeddings)\n", ModelName)
	fmt.Printf("      └── %s  (chat/RAG)\n", ChatModelName)
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

// downloadFile downloads a file with progress bar
func downloadFile(destPath, url string, expectedSize int64) error {
	// Create temp file for download
	tmpPath := destPath + ".tmp"
	defer os.Remove(tmpPath)

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	// Start download
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = expectedSize
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
  "embedding_model": "all-MiniLM-L6-v2",
  "embedding_dims": 384,
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
		filepath.Join(baseDir, "models", ModelName),
		filepath.Join(baseDir, "models", ChatModelName),
		filepath.Join(baseDir, "lib", ".installed"),
	}

	for _, path := range required {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("missing required file: %s", path)
		}
		fmt.Printf("   ✅ %s\n", filepath.Base(path))
	}

	return nil
}

// downloadAndExtract downloads a tar.gz and extracts it to destDir
func downloadAndExtract(url, destDir string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Save to temp file
	tmpFile := filepath.Join(destDir, ".download.tar.gz")
	out, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	written, err := io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpFile)
		return err
	}
	fmt.Printf("   Downloaded %.1f MB\n", float64(written)/1024/1024)

	// Extract using tar
	fmt.Println("   Extracting...")
	cmd := exec.Command("tar", "xzf", tmpFile, "-C", destDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("extraction failed: %s: %w", string(output), err)
	}

	os.Remove(tmpFile)
	return nil
}
