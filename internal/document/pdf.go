package document

import (
	"fmt"
)

// ParsePDF extracts text from PDF files
// NOTE: Full PDF text extraction requires additional dependencies
// For Phase 1 MVP, we support TXT and MD files
// PDF support will be added in Phase 2
func ParsePDF(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty PDF data")
	}

	// For now, return a placeholder
	// In Phase 2, integrate with pdfcpu or another library
	return "", fmt.Errorf("PDF extraction not yet implemented - use TXT or MD files for Phase 1")
}
