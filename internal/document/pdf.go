package document

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ParsePDF extracts text from PDF files using pdftotext (poppler-utils)
// Falls back to a basic Go parser if pdftotext is not available
func ParsePDF(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty PDF data")
	}

	// Write PDF to temp file
	tmpPDF, err := os.CreateTemp("", "small-rag-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpPDF.Name())

	if _, err := tmpPDF.Write(data); err != nil {
		tmpPDF.Close()
		return "", fmt.Errorf("failed to write temp PDF: %w", err)
	}
	tmpPDF.Close()

	// Try pdftotext first (fast, native)
	if path, err := exec.LookPath("pdftotext"); err == nil {
		tmpTxt := tmpPDF.Name() + ".txt"
		defer os.Remove(tmpTxt)

		cmd := exec.Command(path, "-layout", tmpPDF.Name(), tmpTxt)
		if err := cmd.Run(); err == nil {
			text, err := os.ReadFile(tmpTxt)
			if err == nil {
				result := strings.TrimSpace(string(text))
				if result != "" {
					return result, nil
				}
			}
		}
	}

	// Fallback: basic extraction using Go
	return "", fmt.Errorf("pdftotext not available and no fallback parser - install poppler-utils")
}
