package document

import (
	"fmt"
	"io"
	"strings"
)

// Document represents a parsed document
type Document struct {
	ID        string
	Title     string
	Source    string
	Content   string
	Chunks    []Chunk
	ChunkSize int
	Overlap   int
}

// Chunk represents a document chunk
type Chunk struct {
	ID        string
	DocID     string
	Index     int
	Text      string
	Tokens    int
}

// Parser handles different document formats
type Parser interface {
	Parse(data []byte, title string) (string, error)
}

// TextParser handles plain text files
type TextParser struct{}

func (p *TextParser) Parse(data []byte, title string) (string, error) {
	// Use the ParseText function from text.go
	return ParseText(data)
}

// MarkdownParser handles markdown files
type MarkdownParser struct{}

func (p *MarkdownParser) Parse(data []byte, title string) (string, error) {
	// Use the ParseText function (markdown treated as text for now)
	return ParseText(data)
}

// PDFParser handles PDF files
type PDFParser struct{}

func (p *PDFParser) Parse(data []byte, title string) (string, error) {
	// Use the ParsePDF function from pdf.go
	return ParsePDF(data)
}

// GetParser returns appropriate parser for file type
func GetParser(filename string) (Parser, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		return &PDFParser{}, nil
	} else if strings.HasSuffix(strings.ToLower(filename), ".md") {
		return &MarkdownParser{}, nil
	} else if strings.HasSuffix(strings.ToLower(filename), ".txt") {
		return &TextParser{}, nil
	}
	return nil, fmt.Errorf("unsupported file format: %s", filename)
}

// ParseFile parses a file and returns content
func ParseFile(filename string, data []byte) (string, error) {
	parser, err := GetParser(filename)
	if err != nil {
		return "", err
	}
	return parser.Parse(data, filename)
}

// NewDocument creates a new document
func NewDocument(id, title, source, content string, chunkSize, overlap int) *Document {
	return &Document{
		ID:        id,
		Title:     title,
		Source:    source,
		Content:   content,
		Chunks:    []Chunk{},
		ChunkSize: chunkSize,
		Overlap:   overlap,
	}
}

// Chunk splits document into chunks
func (d *Document) Chunk() error {
	if d.ChunkSize <= 0 {
		return fmt.Errorf("chunk size must be positive")
	}

	// Split by sentences first
	sentences := d.splitSentences()

	var chunks []Chunk
	var currentChunk strings.Builder
	var currentTokens int
	var chunkIndex int

	for i, sentence := range sentences {
		sentenceTokens := len(sentence) / 4
		
		// If adding this sentence would exceed chunk size
		if currentTokens+sentenceTokens > d.ChunkSize && currentChunk.Len() > 0 {
			// Save current chunk
			chunk := Chunk{
				ID:     fmt.Sprintf("chunk-%s-%d", d.ID, chunkIndex),
				DocID:  d.ID,
				Index:  chunkIndex,
				Text:   strings.TrimSpace(currentChunk.String()),
				Tokens: currentTokens,
			}
			chunks = append(chunks, chunk)

			// Start new chunk with overlap
			if d.Overlap > 0 && len(chunks) > 0 {
				// Keep last part of previous chunk for overlap
				prevText := chunks[len(chunks)-1].Text
				overlapTokens := (len(prevText) * d.Overlap) / d.ChunkSize
				overlapText := prevText
				if len(prevText) > overlapTokens {
					overlapText = prevText[len(prevText)-overlapTokens:]
				}
				currentChunk.Reset()
				currentChunk.WriteString(overlapText)
				currentChunk.WriteString(" ")
				currentTokens = (len(overlapText) / 4) + sentenceTokens
			} else {
				currentChunk.Reset()
				currentTokens = sentenceTokens
			}

			chunkIndex++
		}

		currentChunk.WriteString(sentence)
		currentChunk.WriteString(" ")
		currentTokens += sentenceTokens

		// Last sentence
		if i == len(sentences)-1 && currentChunk.Len() > 0 {
			chunk := Chunk{
				ID:     fmt.Sprintf("chunk-%s-%d", d.ID, chunkIndex),
				DocID:  d.ID,
				Index:  chunkIndex,
				Text:   strings.TrimSpace(currentChunk.String()),
				Tokens: currentTokens,
			}
			chunks = append(chunks, chunk)
		}
	}

	d.Chunks = chunks
	return nil
}

// splitSentences splits text into sentences
func (d *Document) splitSentences() []string {
	// Simple sentence splitter
	text := d.Content
	
	// Replace common abbreviations to avoid false splits
	text = strings.ReplaceAll(text, "Dr.", "Dr")
	text = strings.ReplaceAll(text, "Mr.", "Mr")
	text = strings.ReplaceAll(text, "Mrs.", "Mrs")
	text = strings.ReplaceAll(text, "Ms.", "Ms")
	text = strings.ReplaceAll(text, "Prof.", "Prof")
	text = strings.ReplaceAll(text, "Inc.", "Inc")
	text = strings.ReplaceAll(text, "Ltd.", "Ltd")
	text = strings.ReplaceAll(text, "Co.", "Co")

	// Split by sentence delimiters
	var sentences []string
	var current strings.Builder

	for i := 0; i < len(text); i++ {
		ch := text[i]
		current.WriteByte(ch)

		// Check for sentence end
		if (ch == '.' || ch == '!' || ch == '?') && i+1 < len(text) {
			// Make sure it's followed by space or end
			if text[i+1] == ' ' || text[i+1] == '\n' {
				s := strings.TrimSpace(current.String())
				if len(s) > 0 {
					sentences = append(sentences, s)
				}
				current.Reset()
				i++ // Skip space
				continue
			}
		}
	}

	// Add remaining text
	if current.Len() > 0 {
		s := strings.TrimSpace(current.String())
		if len(s) > 0 {
			sentences = append(sentences, s)
		}
	}

	// If no sentences found, split by newlines
	if len(sentences) == 0 {
		for _, line := range strings.Split(d.Content, "\n") {
			line = strings.TrimSpace(line)
			if len(line) > 0 {
				sentences = append(sentences, line)
			}
		}
	}

	return sentences
}

// TokenCount estimates token count
func TokenCount(text string) int {
	// Rough estimate: ~4 characters per token
	return len(text) / 4
}

// Reader interface for file reading
type Reader interface {
	Read(p []byte) (n int, err error)
}

// ReadDocument reads and parses a document from reader
func ReadDocument(filename string, reader Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return ParseFile(filename, data)
}
