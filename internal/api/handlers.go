package api

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xnet-admin-1/small-rag/internal/document"
	"github.com/xnet-admin-1/small-rag/internal/search"
)

// DocumentResponse represents API response for document operations
type DocumentResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code,omitempty"`
}

// handleUploadDocument uploads and indexes a document
func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		respondJSON(w, http.StatusBadRequest, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse form: %v", err),
			Code:    400,
		})
		return
	}

	// Get file
	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, DocumentResponse{
			Success: false,
			Error:   "No file provided",
			Code:    400,
		})
		return
	}
	defer file.Close()

	// Read file data
	fileData, err := io.ReadAll(file)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to read file: %v", err),
			Code:    400,
		})
		return
	}

	// Get title and source
	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}
	source := r.FormValue("source")

	// Parse document
	content, err := document.ParseFile(header.Filename, fileData)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse file: %v", err),
			Code:    400,
		})
		return
	}

	// Calculate content hash to detect duplicates
	hash := md5.Sum([]byte(content))
	contentHash := hex.EncodeToString(hash[:])

	// Check for duplicate
	var existingID string
	err = s.db.QueryRow(
		"SELECT id FROM documents WHERE content_hash = ?",
		contentHash,
	).Scan(&existingID)

	if err == nil {
		// Document already exists
		respondJSON(w, http.StatusConflict, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Document already indexed (ID: %s)", existingID),
			Code:    409,
		})
		return
	} else if err != sql.ErrNoRows {
		respondJSON(w, http.StatusInternalServerError, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Database error: %v", err),
			Code:    500,
		})
		return
	}

	// Create document
	docID := uuid.New().String()
	doc := document.NewDocument(
		docID,
		title,
		source,
		content,
		s.cfg.ChunkSize,
		s.cfg.ChunkOverlap,
	)

	// Chunk document
	if err := doc.Chunk(); err != nil {
		respondJSON(w, http.StatusInternalServerError, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to chunk document: %v", err),
			Code:    500,
		})
		return
	}

	// Save document to database
	_, err = s.db.Exec(
		`INSERT INTO documents (id, title, source, content, content_hash, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.Title, doc.Source, doc.Content, contentHash,
		time.Now(), time.Now(),
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to save document: %v", err),
			Code:    500,
		})
		return
	}

	// Save chunks to database
	for _, chunk := range doc.Chunks {
		_, err := s.db.Exec(
			`INSERT INTO chunks (id, doc_id, chunk_index, text, tokens, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			chunk.ID, chunk.DocID, chunk.Index, chunk.Text, chunk.Tokens,
			time.Now(),
		)
		if err != nil {
			// Log error but continue
			fmt.Printf("Failed to save chunk: %v\n", err)
		}
	}

	// Generate embeddings
	embeddingsCreated := 0
	for _, chunk := range doc.Chunks {
		// Generate embedding
		embedding, err := s.embedding.Embed(chunk.Text)
		if err != nil {
			fmt.Printf("Failed to generate embedding for chunk %s: %v\n", chunk.ID, err)
			continue
		}

		// Encode embedding
		embData := search.EncodeEmbedding(embedding)

		// Save embedding
		_, err = s.db.Exec(
			`INSERT INTO embeddings (id, chunk_id, embedding, model_id, dims, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), chunk.ID, embData, s.cfg.EmbeddingModel, s.cfg.EmbeddingDims,
			time.Now(),
		)
		if err != nil {
			fmt.Printf("Failed to save embedding for chunk %s: %v\n", chunk.ID, err)
			continue
		}

		embeddingsCreated++
	}

	// Return response
	respondJSON(w, http.StatusCreated, DocumentResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":                   doc.ID,
			"title":                doc.Title,
			"source":               doc.Source,
			"chunks_created":       len(doc.Chunks),
			"embeddings_created":   embeddingsCreated,
			"status":               "indexed",
			"created_at":           time.Now(),
		},
	})
}

// handleListDocuments lists all documents
func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0

	// Parse query params
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	// Query documents
	rows, err := s.db.Query(
		`SELECT id, title, source, created_at FROM documents
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Database error: %v", err),
			Code:    500,
		})
		return
	}
	defer rows.Close()

	var documents []map[string]interface{}

	for rows.Next() {
		var id, title, source, createdAt string
		if err := rows.Scan(&id, &title, &source, &createdAt); err != nil {
			continue
		}

		// Count chunks
		var chunkCount int
		s.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE doc_id = ?", id).Scan(&chunkCount)

		documents = append(documents, map[string]interface{}{
			"id":           id,
			"title":        title,
			"source":       source,
			"chunks_count": chunkCount,
			"created_at":   createdAt,
		})
	}

	// Get total count
	var total int
	s.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&total)

	respondJSON(w, http.StatusOK, DocumentResponse{
		Success: true,
		Data: map[string]interface{}{
			"documents": documents,
			"total":     total,
			"limit":     limit,
			"offset":    offset,
		},
	})
}

// handleGetDocument gets document details
func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "doc_id")

	var id, title, source, content, createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, title, source, content, created_at, updated_at
		 FROM documents WHERE id = ?`,
		docID,
	).Scan(&id, &title, &source, &content, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, DocumentResponse{
			Success: false,
			Error:   "Document not found",
			Code:    404,
		})
		return
	} else if err != nil {
		respondJSON(w, http.StatusInternalServerError, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Database error: %v", err),
			Code:    500,
		})
		return
	}

	// Count chunks
	var chunkCount int
	s.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE doc_id = ?", id).Scan(&chunkCount)

	// Truncate content for preview
	contentPreview := content
	if len(contentPreview) > 200 {
		contentPreview = contentPreview[:200] + "..."
	}

	respondJSON(w, http.StatusOK, DocumentResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":               id,
			"title":            title,
			"source":           source,
			"content_preview":  contentPreview,
			"chunks_count":     chunkCount,
			"created_at":       createdAt,
			"updated_at":       updatedAt,
		},
	})
}

// handleDeleteDocument deletes a document
func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "doc_id")

	// Delete document (cascades to chunks and embeddings)
	result, err := s.db.Exec("DELETE FROM documents WHERE id = ?", docID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Database error: %v", err),
			Code:    500,
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		respondJSON(w, http.StatusNotFound, DocumentResponse{
			Success: false,
			Error:   "Document not found",
			Code:    404,
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}


