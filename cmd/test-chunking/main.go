package main

import (
	"fmt"
	"strings"

	"github.com/xnet-admin-1/small-rag/internal/document"
)

func main() {
	fmt.Println("Testing Text Chunking...")
	fmt.Println()

	// Test 1: Small document
	fmt.Println("Test 1: Small Document")
	fmt.Println("=" + strings.Repeat("=", 70))
	smallText := "This is a test document. It has multiple sentences. We will test chunking. Machine learning is great. AI is the future. Natural language processing is important."
	doc1 := document.NewDocument("test-1", "Small Test", "test.txt", smallText, 512, 128)
	err := doc1.Chunk()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Chunking successful\n")
		fmt.Printf("Content length: %d characters\n", len(smallText))
		fmt.Printf("Chunks created: %d\n", len(doc1.Chunks))
		for i, chunk := range doc1.Chunks {
			fmt.Printf("\nChunk %d:\n", i)
			fmt.Printf("  ID: %s\n", chunk.ID)
			fmt.Printf("  Tokens: %d\n", chunk.Tokens)
			fmt.Printf("  Text: %s\n", chunk.Text)
		}
	}
	fmt.Println()

	// Test 2: Large document (simulate)
	fmt.Println("Test 2: Large Document (2000+ tokens)")
	fmt.Println("=" + strings.Repeat("=", 70))
	
	// Generate a large text (~2500 tokens = 10,000 characters)
	var largeTextBuilder strings.Builder
	baseSentence := "Machine learning is a powerful technology that enables computers to learn from data. "
	for i := 0; i < 120; i++ {
		largeTextBuilder.WriteString(baseSentence)
		if i%10 == 0 {
			largeTextBuilder.WriteString("This is sentence number " + fmt.Sprint(i) + ". ")
		}
	}
	largeText := largeTextBuilder.String()
	
	doc2 := document.NewDocument("test-2", "Large Test", "large.txt", largeText, 512, 128)
	err = doc2.Chunk()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Chunking successful\n")
		fmt.Printf("Content length: %d characters (~%d tokens)\n", 
			len(largeText), document.TokenCount(largeText))
		fmt.Printf("Chunks created: %d\n", len(doc2.Chunks))
		fmt.Printf("Target chunk size: 512 tokens\n")
		fmt.Printf("Target overlap: 128 tokens\n\n")
		
		for i, chunk := range doc2.Chunks {
			fmt.Printf("Chunk %d: %d tokens, %d chars\n", 
				i, chunk.Tokens, len(chunk.Text))
			
			// Show first 80 chars
			preview := chunk.Text
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			fmt.Printf("  Preview: %s\n", preview)
			
			// Check overlap with next chunk
			if i < len(doc2.Chunks)-1 {
				nextChunk := doc2.Chunks[i+1]
				// Find common text
				overlapFound := false
				for j := 50; j < len(chunk.Text) && j < len(nextChunk.Text); j++ {
					if strings.Contains(nextChunk.Text, chunk.Text[len(chunk.Text)-j:]) {
						overlapChars := j
						overlapTokens := document.TokenCount(chunk.Text[len(chunk.Text)-j:])
						fmt.Printf("  Overlap with next: ~%d chars (~%d tokens)\n", 
							overlapChars, overlapTokens)
						overlapFound = true
						break
					}
				}
				if !overlapFound {
					fmt.Printf("  Overlap with next: minimal or none\n")
				}
			}
			fmt.Println()
		}
	}

	// Test 3: Edge cases
	fmt.Println("Test 3: Edge Cases")
	fmt.Println("=" + strings.Repeat("=", 70))
	
	// Empty document
	doc3 := document.NewDocument("test-3", "Empty", "empty.txt", "", 512, 128)
	err = doc3.Chunk()
	if err != nil {
		fmt.Printf("Empty document: ❌ Error: %v\n", err)
	} else {
		fmt.Printf("Empty document: ✅ Handled (chunks: %d)\n", len(doc3.Chunks))
	}
	
	// Single sentence
	doc4 := document.NewDocument("test-4", "Single", "single.txt", "Just one sentence.", 512, 128)
	err = doc4.Chunk()
	if err != nil {
		fmt.Printf("Single sentence: ❌ Error: %v\n", err)
	} else {
		fmt.Printf("Single sentence: ✅ Handled (chunks: %d)\n", len(doc4.Chunks))
	}
	
	// Very long sentence (> chunk size)
	longSentence := strings.Repeat("word ", 600) + "."
	doc5 := document.NewDocument("test-5", "Long Sentence", "long.txt", longSentence, 512, 128)
	err = doc5.Chunk()
	if err != nil {
		fmt.Printf("Long sentence: ❌ Error: %v\n", err)
	} else {
		fmt.Printf("Long sentence: ✅ Handled (chunks: %d, tokens: %d)\n", 
			len(doc5.Chunks), document.TokenCount(longSentence))
	}

	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("✅ All chunking tests complete")
}
