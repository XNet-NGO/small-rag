package document

import (
	"strings"
	"unicode/utf8"
)

// ParseText parses plain text files (TXT, MD)
func ParseText(data []byte) (string, error) {
	// Convert to string
	text := string(data)
	
	// Validate UTF-8
	if !utf8.ValidString(text) {
		// Try to clean invalid UTF-8
		text = strings.ToValidUTF8(text, "")
	}
	
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	
	// Trim excessive whitespace
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	
	return strings.Join(cleaned, "\n"), nil
}
