package main

import (
	"fmt"
	"time"
	"github.com/xnet-admin-1/small-rag/internal/llamacpp"
)

func main() {
	modelPath := "/home/user-x/.small-rag/models/qwen3-embedding-0.6b-q4_k_m.gguf"
	
	fmt.Println("Loading model...")
	start := time.Now()
	model, err := llamacpp.LoadModel(modelPath, 512, 4)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer model.Close()
	fmt.Printf("Model loaded in %v\n", time.Since(start))
	fmt.Printf("Embedding dims: %d\n", model.EmbeddingDims())

	// Generate embedding
	fmt.Println("\nGenerating embedding for 'hello world'...")
	start = time.Now()
	emb, err := model.Embed("hello world")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Done in %v\n", time.Since(start))
	fmt.Printf("Embedding length: %d\n", len(emb))
	fmt.Printf("First 5 values: %v\n", emb[:5])

	// Second embedding
	fmt.Println("\nGenerating embedding for 'machine learning'...")
	start = time.Now()
	emb2, err := model.Embed("machine learning is a subset of AI")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Done in %v\n", time.Since(start))
	fmt.Printf("First 5 values: %v\n", emb2[:5])
}
