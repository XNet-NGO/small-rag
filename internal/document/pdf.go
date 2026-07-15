package document

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ParsePDF extracts text from PDF files
func ParsePDF(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty PDF data")
	}

	// Write to temp file (the pdf library needs a file reader)
	tmpFile, err := os.CreateTemp("", "small-rag-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Open PDF
	f, r, err := pdf.Open(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	// Extract text from all pages
	var buf bytes.Buffer
	totalPages := r.NumPage()

	for i := 1; i <= totalPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		content := strings.TrimSpace(text)
		if content != "" {
			buf.WriteString(content)
			buf.WriteString("\n\n")
		}
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("no text extracted from PDF (may be image-based)")
	}

	return result, nil
}
