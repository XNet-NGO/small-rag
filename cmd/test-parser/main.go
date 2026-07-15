package main

import (
	"fmt"
	"os"

	"github.com/xnet-admin-1/small-rag/internal/document"
)

func main() {
	fmt.Println("Testing document parser...")
	fmt.Println()
	
	// Test TXT file
	fmt.Println("1. Testing TXT file:")
	data, err := os.ReadFile("test_sample.txt")
	if err != nil {
		fmt.Printf("   ❌ Failed to read file: %v\n", err)
	} else {
		text, err := document.ParseFile("test_sample.txt", data)
		if err != nil {
			fmt.Printf("   ❌ Parse failed: %v\n", err)
		} else {
			fmt.Printf("   ✅ Parsed successfully\n")
			fmt.Printf("   Length: %d characters\n", len(text))
			if len(text) > 100 {
				fmt.Printf("   Preview: %s...\n", text[:100])
			} else {
				fmt.Printf("   Content: %s\n", text)
			}
		}
	}
	fmt.Println()
	
	// Test unsupported format
	fmt.Println("2. Testing unsupported format:")
	_, err = document.ParseFile("test.docx", []byte("test"))
	if err != nil {
		fmt.Printf("   ✅ Correctly rejected: %v\n", err)
	} else {
		fmt.Printf("   ❌ Should have failed\n")
	}
	fmt.Println()
	
	// Test document chunking
	fmt.Println("3. Testing document chunking:")
	doc := document.NewDocument("test-1", "Test Document", "test_sample.txt", 
		"This is a test. Machine learning is great. AI is the future.", 512, 128)
	err = doc.Chunk()
	if err != nil {
		fmt.Printf("   ❌ Chunking failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Chunked successfully\n")
		fmt.Printf("   Chunks created: %d\n", len(doc.Chunks))
		for i, chunk := range doc.Chunks {
			fmt.Printf("   Chunk %d: %d tokens, text: %s\n", i, chunk.Tokens, chunk.Text[:min(50, len(chunk.Text))])
		}
	}
	
	fmt.Println()
	fmt.Println("✅ Parser tests complete")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
