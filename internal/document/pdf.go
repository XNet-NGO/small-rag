package document

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ParsePDF extracts text from PDF files
func ParsePDF(data []byte) (string, error) {
	// Create reader from bytes
	reader := bytes.NewReader(data)
	
	// Open PDF
	pdfReader, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	
	// Get number of pages
	numPages := pdfReader.NumPage()
	if numPages == 0 {
		return "", fmt.Errorf("PDF has no pages")
	}
	
	var textBuilder strings.Builder
	
	// Extract text from each page
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		page := pdfReader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}
		
		// Get page text
		text, err := page.GetPlainText(nil)
		if err != nil {
			// Skip pages that fail to extract
			continue
		}
		
		textBuilder.WriteString(text)
		textBuilder.WriteString("\n")
	}
	
	result := textBuilder.String()
	
	// Clean up text
	result = strings.ReplaceAll(result, "\r\n", "\n")
	result = strings.ReplaceAll(result, "\r", "\n")
	
	// Remove excessive whitespace
	lines := strings.Split(result, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	
	return strings.Join(cleaned, "\n"), nil
}
