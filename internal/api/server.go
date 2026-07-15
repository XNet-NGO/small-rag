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
	"github.com/xnet-admin-1/small-rag/internal/embedding"
	"github.com/xnet-admin-1/small-rag/internal/search"
)

// Server represents the HTTP API server
type Server struct {
	db        *sql.DB
	cfg       *config.Config
	router    chi.Router
	embedding *embedding.Engine
	search    *search.Engine
}

// NewServer creates a new API server
func NewServer(db *sql.DB, cfg *config.Config) *Server {
	s := &Server{
		db:        db,
		cfg:       cfg,
		embedding: embedding.NewEngine("", cfg.EmbeddingDims),
		search:    search.NewEngine(db),
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
	r.Use(corsMiddleware)

	// Static files (web UI)
	r.Get("/", s.handleWebUI)
	r.Get("/index.html", s.handleWebUI)

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

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server
func (s *Server) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// Response types

type HealthResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Status          string `json:"status"`
		Version         string `json:"version"`
		EmbeddingsCount int    `json:"embeddings_count"`
		DocumentsCount  int    `json:"documents_count"`
		UptimeSeconds   int    `json:"uptime_seconds"`
	} `json:"data"`
}

// Handlers

func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Web UI - See web/index.html")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Success: true,
	}

	resp.Data.Status = "ready"
	resp.Data.Version = "0.1.0"

	// Get counts from database
	s.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&resp.Data.DocumentsCount)
	s.db.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&resp.Data.EmbeddingsCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req struct {
		Query      string  `json:"query"`
		TopK       int     `json:"top_k"`
		SearchType string  `json:"search_type"`
		MinScore   float32 `json:"min_score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request",
		})
		return
	}

	// Embed query
	queryEmbedding, err := s.embedding.Embed(req.Query)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to embed query",
		})
		return
	}

	// Search
	results, err := s.search.Search(req.Query, queryEmbedding, req.TopK, req.SearchType, req.MinScore)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Search failed: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"query":   req.Query,
			"results": results,
		},
	})
}

func (s *Server) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Sample streaming response
	fmt.Fprintf(w, "data: {\"type\":\"context\",\"chunks\":3}\n\n")
	fmt.Fprintf(w, "data: {\"type\":\"delta\",\"text\":\"This is a sample response...\"}\n\n")
	fmt.Fprintf(w, "data: {\"type\":\"done\",\"total_tokens\":100}\n\n")
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    s.cfg,
	})
}

func (s *Server) handleSearchAndRAG(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"answer": "Agent tool not yet implemented",
		},
	})
}
