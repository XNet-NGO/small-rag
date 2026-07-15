package api

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	streaming := r.URL.Query().Get("stream") == "true"

	// Helper to respond with error in the appropriate format
	respondErr := func(status int, msg string) {
		if streaming {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			evt, _ := json.Marshal(map[string]interface{}{"type": "error", "error": msg})
			fmt.Fprintf(w, "data: %s\n\n", evt)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		} else {
			respondJSON(w, status, DocumentResponse{Success: false, Error: msg, Code: status})
		}
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		respondErr(400, fmt.Sprintf("Failed to parse form: %v", err))
		return
	}

	// Get file
	file, header, err := r.FormFile("file")
	if err != nil {
		respondErr(400, "No file provided")
		return
	}
	defer file.Close()

	// Read file data
	fileData, err := io.ReadAll(file)
	if err != nil {
		respondErr(400, fmt.Sprintf("Failed to read file: %v", err))
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
		respondErr(400, fmt.Sprintf("Failed to parse file: %v", err))
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
		respondErr(409, fmt.Sprintf("Document already indexed (ID: %s)", existingID))
		return
	} else if err != sql.ErrNoRows {
		respondErr(500, fmt.Sprintf("Database error: %v", err))
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
		respondErr(500, fmt.Sprintf("Failed to chunk document: %v", err))
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
		respondErr(500, fmt.Sprintf("Failed to save document: %v", err))
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
	totalChunks := len(doc.Chunks)
	startEmbed := time.Now()

	// Set up streaming progress if requested
	var flusher http.Flusher
	if streaming {
		f, ok := w.(http.Flusher)
		if ok {
			flusher = f
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			// Send initial event
			evt, _ := json.Marshal(map[string]interface{}{
				"type": "start", "total_chunks": totalChunks, "doc_id": doc.ID, "title": doc.Title,
			})
			fmt.Fprintf(w, "data: %s\n\n", evt)
			flusher.Flush()
		} else {
			streaming = false
		}
	}

	for i, chunk := range doc.Chunks {
		// Log progress every 10 chunks for large documents
		if totalChunks > 10 && (i%10 == 0 || i == totalChunks-1) {
			elapsed := time.Since(startEmbed)
			rate := float64(0)
			if i > 0 {
				rate = float64(i) / elapsed.Seconds()
			}
			remaining := time.Duration(0)
			if rate > 0 {
				remaining = time.Duration(float64(totalChunks-i)/rate) * time.Second
			}
			log.Printf("Embedding progress: %d/%d chunks (%.1f chunks/sec, ~%v remaining)",
				i+1, totalChunks, rate, remaining.Round(time.Second))

			// Send SSE progress event
			if streaming {
				evt, _ := json.Marshal(map[string]interface{}{
					"type": "progress", "current": i + 1, "total": totalChunks,
					"rate": fmt.Sprintf("%.1f", rate),
					"eta":  remaining.Round(time.Second).String(),
				})
				fmt.Fprintf(w, "data: %s\n\n", evt)
				flusher.Flush()
			}
		}

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
	if totalChunks > 10 {
		log.Printf("Embedding complete: %d/%d in %v", embeddingsCreated, totalChunks, time.Since(startEmbed).Round(time.Millisecond))
	}

	// Send final response
	if streaming {
		evt, _ := json.Marshal(map[string]interface{}{
			"type": "done", "doc_id": doc.ID, "title": doc.Title,
			"chunks_created": totalChunks, "embeddings_created": embeddingsCreated,
			"duration_ms": time.Since(startEmbed).Milliseconds(),
		})
		fmt.Fprintf(w, "data: %s\n\n", evt)
		flusher.Flush()
		return
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

// handleSearch performs hybrid search
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, DocumentResponse{
			Success: false,
			Error:   "POST required",
			Code:    405,
		})
		return
	}

	var req struct {
		Query      string  `json:"query"`
		SearchType string  `json:"search_type"` // semantic, keyword, hybrid
		TopK       int     `json:"top_k"`
		MinScore   float32 `json:"min_score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
			Code:    400,
		})
		return
	}

	// Defaults
	if req.SearchType == "" {
		req.SearchType = "hybrid"
	}
	if req.TopK == 0 {
		req.TopK = 10
	}
	if req.MinScore == 0 {
		req.MinScore = 0.3
	}

	// Get query embedding (only for semantic/hybrid search)
	var queryEmbedding []float32
	if req.SearchType != "keyword" {
		embedding, err := s.embedding.Embed(req.Query)
		if err != nil {
			// Fall back to keyword search
			req.SearchType = "keyword"
		} else {
			queryEmbedding = embedding
		}
	}

	// Perform search
	results, err := s.searchEngine.Search(req.Query, queryEmbedding, req.TopK, req.SearchType, req.MinScore)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, DocumentResponse{
			Success: false,
			Error:   fmt.Sprintf("Search failed: %v", err),
			Code:    500,
		})
		return
	}

	respondJSON(w, http.StatusOK, DocumentResponse{
		Success: true,
		Data: map[string]interface{}{
			"query":       req.Query,
			"search_type": req.SearchType,
			"count":       len(results),
			"results":     results,
		},
	})
}


