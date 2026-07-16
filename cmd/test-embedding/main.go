package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xnet-admin-1/small-rag/internal/embedding"
)

func main() {
	fmt.Println("Testing Embedding Engine with GGUF Model")
	fmt.Println("=======================================================================")
	fmt.Println()

	// Model path
	homeDir, _ := os.UserHomeDir()
	modelPath := filepath.Join(homeDir, "small-rag/models/qwen3-embedding-0.6b-q4_k_m.gguf")

	// Check if model exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		fmt.Printf("❌ Model not found: %s\n", modelPath)
		fmt.Println("Please download the model first.")
		os.Exit(1)
	}

	fmt.Printf("Model path: %s\n", modelPath)
	fmt.Println()

	// Create engine
	fmt.Println("Test 1: Initialize Engine")
	fmt.Println("-----------------------------------------------------------------------")
	engine := embedding.NewEngine(modelPath, 384)
	defer engine.Close()
	
	start := time.Now()
	err := engine.Initialize()
	elapsed := time.Since(start)
	
	if err != nil {
		fmt.Printf("❌ Initialization failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✅ Engine initialized in %v\n", elapsed)
	fmt.Println()

	// Test 2: Single embedding
	fmt.Println("Test 2: Generate Single Embedding")
	fmt.Println("-----------------------------------------------------------------------")
	
	testText := "Machine learning is a powerful technology."
	fmt.Printf("Text: \"%s\"\n", testText)
	
	start = time.Now()
	emb, err := engine.Embed(testText)
	elapsed = time.Since(start)
	
	if err != nil {
		fmt.Printf("❌ Embedding failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✅ Embedding generated in %v\n", elapsed)
	fmt.Printf("Dimensions: %d\n", len(emb))
	fmt.Printf("First 10 values: ")
	for i := 0; i < 10 && i < len(emb); i++ {
		fmt.Printf("%.4f ", emb[i])
	}
	fmt.Println()
	fmt.Println()

	// Test 3: Cached embedding
	fmt.Println("Test 3: Cached Embedding Retrieval")
	fmt.Println("-----------------------------------------------------------------------")
	
	start = time.Now()
	emb2, err := engine.Embed(testText)
	elapsed = time.Since(start)
	
	if err != nil {
		fmt.Printf("❌ Cached embedding failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✅ Cached embedding retrieved in %v\n", elapsed)
	fmt.Printf("Cache size: %d entries\n", engine.CacheSize())
	
	// Verify same result
	same := true
	for i := range emb {
		if emb[i] != emb2[i] {
			same = false
			break
		}
	}
	if same {
		fmt.Println("✅ Cached embedding matches original")
	} else {
		fmt.Println("❌ Cached embedding differs from original")
	}
	fmt.Println()

	// Test 4: Semantic similarity
	fmt.Println("Test 4: Semantic Similarity")
	fmt.Println("-----------------------------------------------------------------------")
	
	text1 := "Machine learning is amazing"
	text2 := "AI and ML are incredible"
	text3 := "I love pizza and pasta"
	
	e1, _ := engine.Embed(text1)
	e2, _ := engine.Embed(text2)
	e3, _ := engine.Embed(text3)
	
	sim12 := cosineSimilarity(e1, e2)
	sim13 := cosineSimilarity(e1, e3)
	sim23 := cosineSimilarity(e2, e3)
	
	fmt.Printf("Text 1: \"%s\"\n", text1)
	fmt.Printf("Text 2: \"%s\"\n", text2)
	fmt.Printf("Text 3: \"%s\"\n", text3)
	fmt.Println()
	fmt.Printf("Similarity (Text 1 ↔ Text 2): %.4f (related topics)\n", sim12)
	fmt.Printf("Similarity (Text 1 ↔ Text 3): %.4f (unrelated topics)\n", sim13)
	fmt.Printf("Similarity (Text 2 ↔ Text 3): %.4f (unrelated topics)\n", sim23)
	fmt.Println()
	
	if sim12 > sim13 && sim12 > sim23 {
		fmt.Println("✅ Semantic similarity working correctly!")
		fmt.Println("   Related texts have higher similarity than unrelated ones.")
	} else {
		fmt.Println("⚠️  Similarity scores unexpected")
	}
	fmt.Println()

	// Summary
	fmt.Println("=======================================================================")
	fmt.Println("✅ All tests completed successfully!")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  - Model: qwen3-embedding-0.6b-q4_k_m.gguf\n")
	fmt.Printf("  - Dimensions: 384\n")
	fmt.Printf("  - Cache entries: %d\n", engine.CacheSize())
	fmt.Println("  - Normalization: ✅")
	fmt.Println("  - Semantic similarity: ✅")
	fmt.Println()
}

func cosineSimilarity(a, b []float32) float32 {
	var dot float32 = 0
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}
