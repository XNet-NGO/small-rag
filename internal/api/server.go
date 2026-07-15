package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xnet-admin-1/small-rag/internal/config"
)

// Server represents the HTTP API server
type Server struct {
	db     *sql.DB
	cfg    *config.Config
	router chi.Router
}

// NewServer creates a new API server
func NewServer(db *sql.DB, cfg *config.Config) *Server {
	s := &Server{
		db:  db,
		cfg: cfg,
	}
	s.setupRouter()
	return s
}

// setupRouter configures routes
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)

	// Health check
	r.Get("/health", s.handleHealth)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Documents
		r.Get("/documents", s.handleListDocuments)
		r.Post("/documents", s.handleUploadDocument)
		r.Get("/documents/{doc_id}", s.handleGetDocument)
		r.Delete("/documents/{doc_id}", s.handleDeleteDocument)

		// Search
		r.Post("/search", s.handleSearch)

		// RAG
		r.Post("/rag/query", s.handleRAGQuery)

		// Config
		r.Get("/config", s.handleGetConfig)

		// Tools for agents
		r.Post("/tools/search_and_rag", s.handleSearchAndRAG)
	})

	s.router = r
}

// Start starts the HTTP server
func (s *Server) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// Response types

type HealthResponse struct {
	Status           string `json:"status"`
	Version          string `json:"version"`
	EmbeddingsCount  int    `json:"embeddings_count"`
	DocumentsCount   int    `json:"documents_count"`
	UptimeSeconds    int    `json:"uptime_seconds"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// Handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:  "ready",
		Version: "0.1.0",
	}

	// Get counts from database
	s.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&resp.DocumentsCount)
	s.db.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&resp.EmbeddingsCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement pagination
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"documents": []interface{}{},
		"total":     0,
	})
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement document upload and chunking
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                  "doc-123",
		"title":               "Document",
		"chunks_created":      0,
		"embeddings_created":  0,
		"status":              "pending",
	})
}

func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "doc_id")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           docID,
		"title":        "Document",
		"chunks_count": 0,
	})
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement document deletion
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement search
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":       "search query",
		"results":     []interface{}{},
		"search_time_ms": 0,
	})
}

func (s *Server) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement RAG query with streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send sample events
	fmt.Fprintf(w, "data: {\"type\":\"context\",\"chunks\":3}\n\n")
	fmt.Fprintf(w, "data: {\"type\":\"delta\",\"text\":\"Response...\"}\n\n")
	fmt.Fprintf(w, "data: {\"type\":\"done\",\"total_tokens\":100}\n\n")
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.cfg)
}

func (s *Server) handleSearchAndRAG(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement agent tool
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   "search query",
		"answer":  "Based on documents...",
		"sources": []interface{}{},
		"tokens_used": 0,
	})
}
