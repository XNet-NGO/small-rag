package api

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/xnet-admin-1/small-rag/internal/batch"
	"github.com/xnet-admin-1/small-rag/internal/config"
	"github.com/xnet-admin-1/small-rag/internal/document"
	"github.com/xnet-admin-1/small-rag/internal/embedding"
	"github.com/xnet-admin-1/small-rag/internal/llm"
	"github.com/xnet-admin-1/small-rag/internal/search"
)

// Server represents the HTTP API server
type Server struct {
	db           *sql.DB
	cfg          *config.Config
	router       chi.Router
	embedding    *embedding.Engine
	searchEngine *search.Engine
	batchMgr     *batch.Manager
	llmClient    *llm.Client
	localLLM     *llm.LocalEngine
}

// NewServer creates a new API server
func NewServer(db *sql.DB, cfg *config.Config) *Server {
	// Get model path from config, fallback to legacy location
	modelPath := cfg.ModelPath
	if modelPath == "" {
		homeDir, _ := os.UserHomeDir()
		modelPath = filepath.Join(homeDir, "small-rag/models/qwen3-embedding-0.6b-q4_k_m.gguf")
	}

	// Initialize embedding engine
	embeddingEngine := embedding.NewEngine(modelPath, cfg.EmbeddingDims)
	embeddingEngine.SetLibPath(cfg.LibPath)

	// Initialize LLM client
	llmURL := os.Getenv("SMALL_RAG_LLM_URL")
	if llmURL == "" {
		llmURL = "http://localhost:11434/v1"
	}
	llmKey := os.Getenv("SMALL_RAG_LLM_KEY")
	llmClient := llm.NewClient(llmURL, llmKey)

	s := &Server{
		db:           db,
		cfg:          cfg,
		embedding:    embeddingEngine,
		searchEngine: search.NewEngine(db),
		batchMgr:     batch.NewManager(),
		llmClient:    llmClient,
		localLLM:     llm.NewLocalEngine(),
	}
	s.setupRouter()
	return s
}

// setupRouter configures routes
func (s *Server) setupRouter() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)
	r.Use(corsMiddleware)
	r.Get("/", s.handleWebUI)
	r.Get("/index.html", s.handleWebUI)
	r.Get("/health", s.handleHealth)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/documents", s.handleListDocuments)
		r.Post("/documents", s.handleUploadDocument)
		r.Get("/documents/{doc_id}", s.handleGetDocument)
		r.Delete("/documents/{doc_id}", s.handleDeleteDocument)
		r.Post("/search", s.handleSearch)
		r.Post("/rag/query", s.handleRAGQuery)
		r.Get("/config", s.handleGetConfig)
		r.Get("/models", s.handleListModels)
		r.Post("/models/load", s.handleLoadModel)
		r.Post("/tools/search_and_rag", s.handleSearchAndRAG)
		r.Post("/batch/index", s.handleBatchIndex)
		r.Get("/batch/{batch_id}", s.handleBatchStatus)
	})
	s.router = r
}

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

func (s *Server) Start(port int) error {
	// Initialize embedding engine
	log.Printf("Initializing embedding engine...")
	if err := s.embedding.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize embedding engine: %w", err)
	}
	log.Printf("Embedding engine ready")
	
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

type HealthResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Status          string `json:"status"`
		Version         string `json:"version"`
		EmbeddingsCount int    `json:"embeddings_count"`
		DocumentsCount  int    `json:"documents_count"`
	} `json:"data"`
}

func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Try to serve web/index.html from disk (relative to binary)
	candidates := []string{
		"web/index.html",
		filepath.Join(filepath.Dir(os.Args[0]), "web", "index.html"),
	}
	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil {
			w.Write(data)
			return
		}
	}

	// Fallback to inline HTML
	fmt.Fprint(w, htmlUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{Success: true}
	resp.Data.Status = "ready"
	resp.Data.Version = "0.1.0"
	s.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&resp.Data.DocumentsCount)
	s.db.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&resp.Data.EmbeddingsCount)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}



func (s *Server) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query        string  `json:"query"`
		TopK         int     `json:"top_k"`
		Model        string  `json:"model"`
		SystemPrompt string  `json:"system_prompt"`
		Temperature  float64 `json:"temperature"`
		Stream       *bool   `json:"stream"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if req.Query == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "query is required",
		})
		return
	}

	// Defaults
	if req.TopK == 0 {
		req.TopK = 3
	}
	if req.Model == "" {
		req.Model = s.cfg.DefaultModel
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	stream := true
	if req.Stream != nil {
		stream = *req.Stream
	}

	// Embed the query
	queryEmbedding, err := s.embedding.Embed(req.Query)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Embedding failed: %v", err),
		})
		return
	}

	// Search for relevant chunks
	searchStart := time.Now()
	results, err := s.searchEngine.Search(req.Query, queryEmbedding, req.TopK, "hybrid", 0.3)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Search failed: %v", err),
		})
		return
	}
	searchTimeMs := time.Since(searchStart).Milliseconds()

	// Build context from search results
	var contextParts []string
	for _, result := range results {
		contextParts = append(contextParts, result.Text)
	}
	contextStr := strings.Join(contextParts, "\n---\n")

	// Build system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant. Answer the user's question based on the following context from the knowledge base. If the context doesn't contain relevant information, say so. /no_think"
	}
	fullSystemPrompt := systemPrompt + "\n\nContext:\n---\n" + contextStr + "\n---"

	// Build messages
	messages := []llm.Message{
		{Role: "system", Content: fullSystemPrompt},
		{Role: "user", Content: req.Query},
	}

	chatReq := llm.ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   2048,
	}

	ctx := context.Background()

	if stream {
		// SSE streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   "Streaming not supported",
			})
			return
		}

		// Send context event
		contextEvent, _ := json.Marshal(map[string]interface{}{
			"type":           "context",
			"chunks":         len(results),
			"search_time_ms": searchTimeMs,
		})
		fmt.Fprintf(w, "data: %s\n\n", contextEvent)
		flusher.Flush()

		// Stream LLM response via HTTP client (points at managed llama-server)
		genStart := time.Now()
		tokenCount := 0

		resp, err := s.llmClient.ChatCompletionStream(ctx, chatReq, func(delta string, done bool, err error) {
			if err != nil {
				log.Printf("Stream error: %v", err)
				return
			}
			if done {
				return
			}
			tokenCount++
			deltaEvent, _ := json.Marshal(map[string]interface{}{
				"type": "delta",
				"text": delta,
			})
			fmt.Fprintf(w, "data: %s\n\n", deltaEvent)
			flusher.Flush()
		})

		if err != nil {
			errEvent, _ := json.Marshal(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			fmt.Fprintf(w, "data: %s\n\n", errEvent)
			flusher.Flush()
			return
		}
		_ = resp

		// Send done event
		totalTokens := tokenCount
		genTimeMs := time.Since(genStart).Milliseconds()
		doneEvent, _ := json.Marshal(map[string]interface{}{
			"type":               "done",
			"total_tokens":       totalTokens,
			"generation_time_ms": genTimeMs,
		})
		fmt.Fprintf(w, "data: %s\n\n", doneEvent)
		flusher.Flush()
	} else {
		// Non-streaming response
		resp, err := s.llmClient.ChatCompletion(ctx, chatReq)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("LLM request failed: %v", err),
			})
			return
		}

		answer := ""
		if len(resp.Choices) > 0 {
			answer = resp.Choices[0].Message.Content
		}

		// Build sources
		sources := make([]map[string]interface{}, 0, len(results))
		for _, result := range results {
			source := map[string]interface{}{
				"doc_id": result.DocID,
				"score":  result.Score,
				"text":   result.Text,
			}
			if title, ok := result.Metadata["title"]; ok {
				source["title"] = title
			}
			sources = append(sources, source)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"answer":      answer,
				"sources":     sources,
				"tokens_used": resp.Usage.TotalTokens,
			},
		})
	}
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Return actual runtime values, not just saved config
	actualDims := s.cfg.EmbeddingDims
	if s.embedding != nil && s.embedding.Dims() > 0 {
		actualDims = s.embedding.Dims()
	}

	// Show actual loaded chat model and endpoint
	chatModel := "none loaded"
	llmEndpoint := "not running"
	if s.localLLM.IsLoaded() {
		chatModel = s.localLLM.ModelName()
		llmEndpoint = s.localLLM.URL()
	} else {
		llmURL := os.Getenv("SMALL_RAG_LLM_URL")
		if llmURL != "" {
			llmEndpoint = llmURL
			chatModel = "external"
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"embedding_model": s.cfg.EmbeddingModel,
			"embedding_dims":  actualDims,
			"chunk_size":      s.cfg.ChunkSize,
			"chunk_overlap":   s.cfg.ChunkOverlap,
			"search_types":    s.cfg.SearchTypes,
			"min_score":       s.cfg.MinScore,
			"default_model":   chatModel,
			"llm_endpoint":    llmEndpoint,
			"port":            s.cfg.Port,
			"enable_cache":    s.cfg.EnableCache,
			"enable_sse":      s.cfg.EnableSSE,
		},
	})
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	// Scan chat models directory
	homeDir, _ := os.UserHomeDir()
	chatDir := filepath.Join(homeDir, "small-rag", "models", "chat")

	var models []map[string]interface{}
	entries, err := os.ReadDir(chatDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".gguf") {
				continue
			}
			info, _ := entry.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			// Strip .gguf extension for display name
			displayName := strings.TrimSuffix(name, ".gguf")
			models = append(models, map[string]interface{}{
				"id":       name,
				"name":     displayName,
				"size_mb":  size / 1024 / 1024,
			})
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"models": models},
	})
}

// LoadChatModel loads a chat model (public, for use during startup)
func (s *Server) LoadChatModel(modelPath string) error {
	if err := s.localLLM.LoadModel(modelPath); err != nil {
		return err
	}
	s.llmClient.BaseURL = s.localLLM.URL()
	log.Printf("Chat model loaded: %s (LLM endpoint: %s)", s.localLLM.ModelName(), s.llmClient.BaseURL)
	return nil
}

func (s *Server) handleLoadModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Model == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "model field required"})
		return
	}

	// Find the model file in chat directory
	homeDir, _ := os.UserHomeDir()
	modelPath := filepath.Join(homeDir, "small-rag", "models", "chat", req.Model)
	if !strings.HasSuffix(modelPath, ".gguf") {
		modelPath += ".gguf"
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": "model not found: " + req.Model})
		return
	}

	// Load the model
	log.Printf("Loading chat model: %s", filepath.Base(modelPath))
	if err := s.localLLM.LoadModel(modelPath); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Point the HTTP LLM client at the managed local server
	s.llmClient.BaseURL = s.localLLM.URL()

	log.Printf("Chat model loaded: %s (LLM endpoint: %s)", s.localLLM.ModelName(), s.llmClient.BaseURL)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"model": s.localLLM.ModelName(), "status": "loaded"},
	})
}

func (s *Server) handleSearchAndRAG(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query        string  `json:"query"`
		TopK         int     `json:"top_k"`
		Model        string  `json:"model"`
		SystemPrompt string  `json:"system_prompt"`
		Temperature  float64 `json:"temperature"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if req.Query == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "query is required",
		})
		return
	}

	// Defaults
	if req.TopK == 0 {
		req.TopK = 3
	}
	if req.Model == "" {
		req.Model = s.cfg.DefaultModel
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	// Embed the query
	queryEmbedding, err := s.embedding.Embed(req.Query)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Embedding failed: %v", err),
		})
		return
	}

	// Search for relevant chunks
	results, err := s.searchEngine.Search(req.Query, queryEmbedding, req.TopK, "hybrid", 0.3)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Search failed: %v", err),
		})
		return
	}

	// Build context from search results
	var contextParts []string
	for _, result := range results {
		contextParts = append(contextParts, result.Text)
	}
	contextStr := strings.Join(contextParts, "\n---\n")

	// Build system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant. Answer the user's question based on the following context from the knowledge base. If the context doesn't contain relevant information, say so. /no_think"
	}
	fullSystemPrompt := systemPrompt + "\n\nContext:\n---\n" + contextStr + "\n---"

	// Build messages
	messages := []llm.Message{
		{Role: "system", Content: fullSystemPrompt},
		{Role: "user", Content: req.Query},
	}

	chatReq := llm.ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   2048,
	}

	// Call LLM (non-streaming)
	ctx := context.Background()
	resp, err := s.llmClient.ChatCompletion(ctx, chatReq)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("LLM request failed: %v", err),
		})
		return
	}

	answer := ""
	if len(resp.Choices) > 0 {
		answer = resp.Choices[0].Message.Content
	}

	// Build sources
	sources := make([]map[string]interface{}, 0, len(results))
	for _, result := range results {
		source := map[string]interface{}{
			"doc_id": result.DocID,
			"score":  result.Score,
		}
		if title, ok := result.Metadata["title"]; ok {
			source["title"] = title
		}
		sources = append(sources, source)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"query":       req.Query,
			"answer":      answer,
			"sources":     sources,
			"tokens_used": resp.Usage.TotalTokens,
		},
	})
}

// handleBatchIndex starts a batch document indexing job
func (s *Server) handleBatchIndex(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Documents []batch.DocumentInput `json:"documents"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if len(req.Documents) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "No documents provided",
		})
		return
	}

	job := s.batchMgr.CreateBatch(req.Documents)

	// processFunc reuses the same document pipeline as handleUploadDocument
	processFunc := func(path, title string) (string, int, error) {
		// Read file from disk
		fileData, err := os.ReadFile(path)
		if err != nil {
			return "", 0, fmt.Errorf("failed to read file: %w", err)
		}

		// Determine filename for parser selection
		filename := filepath.Base(path)

		// Parse document content
		content, err := document.ParseFile(filename, fileData)
		if err != nil {
			return "", 0, fmt.Errorf("failed to parse file: %w", err)
		}

		// Calculate content hash for dedup
		hash := md5.Sum([]byte(content))
		contentHash := hex.EncodeToString(hash[:])

		// Check for duplicate
		var existingID string
		err = s.db.QueryRow("SELECT id FROM documents WHERE content_hash = ?", contentHash).Scan(&existingID)
		if err == nil {
			return "", 0, fmt.Errorf("document already indexed (ID: %s)", existingID)
		} else if err != sql.ErrNoRows {
			return "", 0, fmt.Errorf("database error: %w", err)
		}

		// Create and chunk document
		docID := uuid.New().String()
		if title == "" {
			title = filename
		}
		doc := document.NewDocument(docID, title, path, content, s.cfg.ChunkSize, s.cfg.ChunkOverlap)
		if err := doc.Chunk(); err != nil {
			return "", 0, fmt.Errorf("failed to chunk document: %w", err)
		}

		// Save document to database
		_, err = s.db.Exec(
			`INSERT INTO documents (id, title, source, content, content_hash, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			doc.ID, doc.Title, doc.Source, doc.Content, contentHash, time.Now(), time.Now(),
		)
		if err != nil {
			return "", 0, fmt.Errorf("failed to save document: %w", err)
		}

		// Save chunks and generate embeddings
		for _, chunk := range doc.Chunks {
			_, err := s.db.Exec(
				`INSERT INTO chunks (id, doc_id, chunk_index, text, tokens, created_at)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				chunk.ID, chunk.DocID, chunk.Index, chunk.Text, chunk.Tokens, time.Now(),
			)
			if err != nil {
				log.Printf("Failed to save chunk: %v", err)
				continue
			}

			emb, err := s.embedding.Embed(chunk.Text)
			if err != nil {
				log.Printf("Failed to generate embedding for chunk %s: %v", chunk.ID, err)
				continue
			}

			embData := search.EncodeEmbedding(emb)
			_, err = s.db.Exec(
				`INSERT INTO embeddings (id, chunk_id, embedding, model_id, dims, created_at)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				uuid.New().String(), chunk.ID, embData, s.cfg.EmbeddingModel, s.cfg.EmbeddingDims, time.Now(),
			)
			if err != nil {
				log.Printf("Failed to save embedding for chunk %s: %v", chunk.ID, err)
			}
		}

		return docID, len(doc.Chunks), nil
	}

	// Start background processing
	go s.batchMgr.ProcessBatch(job, req.Documents, processFunc)

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"batch_id": job.ID,
			"status":   job.Status,
			"total":    job.Total,
		},
	})
}

// handleBatchStatus returns the current status of a batch job
func (s *Server) handleBatchStatus(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batch_id")

	job, err := s.batchMgr.GetBatch(batchID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    job,
	})
}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

const htmlUI = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Small-RAG</title>
<style>
* {margin:0;padding:0;box-sizing:border-box}
body {font-family:system-ui,Arial;background:#1a1a1a;color:#e0e0e0;line-height:1.6}
.container {max-width:1200px;margin:0 auto;padding:20px}
.header {display:flex;justify-content:space-between;align-items:center;margin-bottom:30px;padding-bottom:20px;border-bottom:1px solid #404040}
.header h1 {font-size:28px;font-weight:600}
.status {display:flex;gap:15px;align-items:center;font-size:14px}
.indicator {width:12px;height:12px;border-radius:50%;background:#4ade80;animation:pulse 2s infinite}
.indicator.off {background:#f87171;animation:none}
@keyframes pulse {0%,100%{opacity:1}50%{opacity:.5}}
.tabs {display:flex;gap:10px;margin-bottom:30px;border-bottom:1px solid #404040;overflow-x:auto}
.tab {padding:12px 24px;background:0;border:0;border-bottom:2px solid transparent;color:#a0a0a0;cursor:pointer;transition:all .3s;white-space:nowrap}
.tab:hover {color:#e0e0e0}
.tab.active {color:#4a9eff;border-bottom-color:#4a9eff}
.content {display:none}
.content.active {display:block}
.card {background:#2d2d2d;border:1px solid #404040;border-radius:8px;padding:20px;margin-bottom:20px}
.title {font-size:16px;font-weight:600;margin-bottom:15px}
.form-group {margin-bottom:15px}
label {display:block;margin-bottom:8px;font-size:14px;color:#a0a0a0}
input,select,textarea {width:100%;padding:10px;background:#1a1a1a;border:1px solid #404040;border-radius:4px;color:#e0e0e0;font-family:inherit}
textarea {resize:vertical;min-height:100px}
button {padding:10px 20px;background:#4a9eff;border:0;border-radius:4px;color:white;cursor:pointer;transition:all .3s}
button:hover {background:#357abd}
button.sec {background:#1a1a1a;border:1px solid #404040;color:#e0e0e0}
button.danger {background:#f87171}
button:disabled {opacity:.5}
.buttons {display:flex;gap:10px;margin-top:15px;flex-wrap:wrap}
.list {display:flex;flex-direction:column;gap:10px}
.item {display:flex;justify-content:space-between;align-items:center;padding:12px;background:#1a1a1a;border:1px solid #404040;border-radius:4px}
.result {padding:15px;background:#1a1a1a;border-left:3px solid #4a9eff;border-radius:4px;margin-bottom:15px}
.score {display:inline-block;padding:2px 8px;background:#4a9eff;border-radius:12px;font-size:12px;font-weight:600;margin-bottom:8px}
.response {padding:15px;background:#1a1a1a;border-radius:4px;min-height:100px;white-space:pre-wrap;word-wrap:break-word}
.msg {padding:12px 15px;border-radius:4px;margin-bottom:15px;font-size:14px}
.msg.ok {background:rgba(74,222,128,.1);border:1px solid #4ade80;color:#4ade80}
.msg.err {background:rgba(248,113,113,.1);border:1px solid #f87171;color:#f87171}
.muted {color:#a0a0a0}
</style>
</head>
<body>
<div class="container">
<div class="header">
<h1>📚 Small-RAG</h1>
<div class="status">
<span class="indicator" id="ind"></span>
<span id="stat">Connecting...</span>
<span id="info" class="muted"></span>
</div>
</div>
<div id="msgs"></div>
<div class="tabs">
<button class="tab active" data-tab="docs">📄 Documents</button>
<button class="tab" data-tab="search">🔍 Search</button>
<button class="tab" data-tab="rag">✨ RAG</button>
<button class="tab" data-tab="settings">⚙️ Settings</button>
</div>
<div id="docs" class="content active">
<div class="card">
<div class="title">Upload Document</div>
<div class="form-group"><label>File</label><input type="file" id="file" accept=".pdf,.txt,.md"></div>
<div class="buttons"><button id="uploadBtn">Upload</button><button class="sec" id="clearBtn">Clear</button></div>
</div>
<div class="card">
<div class="title">Documents (<span id="count">0</span>)</div>
<div id="docList" class="list"></div>
</div>
</div>
<div id="search" class="content">
<div class="card">
<div class="title">Search</div>
<div class="form-group"><label>Query</label><input type="text" id="query"></div>
<div class="buttons"><button id="searchBtn">Search</button><button class="sec" id="clearSearchBtn">Clear</button></div>
</div>
<div class="card">
<div class="title">Results</div>
<div id="results" class="list"></div>
</div>
</div>
<div id="rag" class="content">
<div class="card">
<div class="title">Ask Question</div>
<div class="form-group"><label>Question</label><textarea id="ragQuery" placeholder="Ask a question..."></textarea></div>
<div class="form-group"><label>Model</label><select id="model"><option>loading...</option></select></div>
<div class="buttons"><button id="ragBtn">Ask</button><button class="sec" id="clearRagBtn">Clear</button></div>
</div>
<div class="card">
<div class="title">Response</div>
<div id="ragResponse" class="response muted">Ask a question to get started...</div>
</div>
</div>
<div id="settings" class="content">
<div class="card">
<div class="title">Configuration</div>
<div id="config" class="muted">Loading...</div>
</div>
</div>
</div>
<script>
const API='http://localhost:8765/api/v1';
document.addEventListener('DOMContentLoaded',()=>{
document.querySelectorAll('.tab').forEach(b=>b.addEventListener('click',switchTab));
document.getElementById('uploadBtn').addEventListener('click',upload);
document.getElementById('clearBtn').addEventListener('click',()=>{document.getElementById('file').value=''});
document.getElementById('searchBtn').addEventListener('click',search);
document.getElementById('clearSearchBtn').addEventListener('click',()=>{document.getElementById('query').value='';document.getElementById('results').innerHTML=''});
document.getElementById('ragBtn').addEventListener('click',rag);
document.getElementById('clearRagBtn').addEventListener('click',()=>{document.getElementById('ragQuery').value='';document.getElementById('ragResponse').textContent='Ask a question to get started...'});
document.getElementById('query').addEventListener('keypress',e=>{if(e.key==='Enter')search()});
document.getElementById('ragQuery').addEventListener('keypress',e=>{if(e.ctrlKey&&e.key==='Enter')rag()});
check();setInterval(check,5000);loadDocs();loadConfig();loadModels()
});
function switchTab(e){document.querySelectorAll('.tab').forEach(b=>b.classList.remove('active'));e.target.classList.add('active');document.querySelectorAll('.content').forEach(c=>c.classList.remove('active'));document.getElementById(e.target.dataset.tab).classList.add('active')}
async function check(){try{const r=await fetch(API+'/health');const d=await r.json();document.getElementById('ind').classList.remove('off');document.getElementById('stat').textContent='Connected';document.getElementById('info').textContent='Docs: '+d.data.documents_count+' | Embeddings: '+d.data.embeddings_count}catch(e){document.getElementById('ind').classList.add('off');document.getElementById('stat').textContent='Disconnected'}}
function msg(text,type){const m=document.createElement('div');m.className='msg '+type;m.textContent=text;document.getElementById('msgs').appendChild(m);setTimeout(()=>m.remove(),5000)}
async function upload(){const f=document.getElementById('file').files[0];if(!f){msg('Select a file','err');return}const fd=new FormData();fd.append('file',f);fd.append('title',f.name.replace(/\.[^.]+$/,''));try{document.getElementById('uploadBtn').disabled=true;document.getElementById('uploadBtn').textContent='Processing...';const r=await fetch(API+'/documents?stream=true',{method:'POST',body:fd});var reader=r.body.getReader();var decoder=new TextDecoder();var reading=true;while(reading){var chunk=await reader.read();if(chunk.done){reading=false;break}var text=decoder.decode(chunk.value);var lines=text.split('\n');for(var i=0;i<lines.length;i++){var line=lines[i];if(line.startsWith('data: ')){try{var data=JSON.parse(line.substring(6));if(data.type==='start'){document.getElementById('uploadBtn').textContent='0/'+data.total_chunks+' chunks...'}else if(data.type==='progress'){document.getElementById('uploadBtn').textContent=data.current+'/'+data.total+' ('+data.rate+'/s, ETA '+data.eta+')'}else if(data.type==='done'){msg('Indexed: '+data.chunks_created+' chunks, '+data.embeddings_created+' embeddings in '+(data.duration_ms/1000).toFixed(1)+'s','ok');document.getElementById('file').value='';loadDocs()}else if(data.type==='error'){msg(data.error,'err')}}catch(pe){}}}}}catch(e){msg('Upload failed: '+e.message,'err')}finally{document.getElementById('uploadBtn').disabled=false;document.getElementById('uploadBtn').textContent='Upload'}}
async function loadDocs(){try{const r=await fetch(API+'/documents');const d=await r.json();const l=document.getElementById('docList');l.innerHTML='';if(!d.data.documents||d.data.documents.length===0){l.innerHTML='<p class="muted">No documents</p>';document.getElementById('count').textContent='0';return}document.getElementById('count').textContent=d.data.documents.length;d.data.documents.forEach(doc=>{const item=document.createElement('div');item.className='item';item.innerHTML='<div><strong>'+doc.title+'</strong><br><small>'+doc.chunks_count+' chunks</small></div><button class="danger" onclick="del('+JSON.stringify(doc.id)+')">Delete</button>';l.appendChild(item)})}catch(e){msg('Failed to load: '+e.message,'err')}}
async function del(id){if(!confirm('Delete?'))return;try{await fetch(API+'/documents/'+id,{method:'DELETE'});msg('Deleted','ok');loadDocs()}catch(e){msg('Delete failed: '+e.message,'err')}}
async function search(){const q=document.getElementById('query').value;if(!q){msg('Enter query','err');return}try{document.getElementById('searchBtn').disabled=true;const r=await fetch(API+'/search',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({query:q,top_k:5,search_type:'hybrid'})});const d=await r.json();const c=document.getElementById('results');c.innerHTML='';if(!d.data.results||d.data.results.length===0){c.innerHTML='<p class="muted">No results</p>';return}d.data.results.forEach(res=>{const item=document.createElement('div');item.className='result';item.innerHTML='<div class="score">'+(res.score*100).toFixed(0)+'%</div><div>'+res.text+'</div>';c.appendChild(item)})}catch(e){msg('Search failed: '+e.message,'err')}finally{document.getElementById('searchBtn').disabled=false}}
async function rag(){var q=document.getElementById('ragQuery').value;if(!q){msg('Enter question','err');return}try{document.getElementById('ragBtn').disabled=true;document.getElementById('ragResponse').textContent='Loading...';var r=await fetch(API+'/rag/query',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({query:q,model:document.getElementById('model').value,stream:true})});var reader=r.body.getReader();var decoder=new TextDecoder();var resp='';var reading=true;while(reading){var chunk=await reader.read();if(chunk.done){reading=false;break}var text=decoder.decode(chunk.value);var lines=text.split('\n');for(var i=0;i<lines.length;i++){var line=lines[i];if(line.startsWith('data: ')){try{var data=JSON.parse(line.substring(6));if(data.type==='delta'){resp+=data.text;document.getElementById('ragResponse').textContent=resp}else if(data.type==='done'){document.getElementById('ragResponse').textContent=resp+'\n\n---\nTokens: '+data.total_tokens+' | Time: '+(data.generation_time_ms/1000).toFixed(1)+'s'}}catch(pe){}}}}}catch(e){msg('RAG failed: '+e.message,'err')}finally{document.getElementById('ragBtn').disabled=false}}
async function loadConfig(){try{const r=await fetch(API+'/config');const d=await r.json();document.getElementById('config').innerHTML='<b>Embedding Model:</b> '+d.data.embedding_model+'<br><b>Embedding Dims:</b> '+d.data.embedding_dims+'<br><b>Chunk Size:</b> '+d.data.chunk_size+' tokens<br><b>Chunk Overlap:</b> '+d.data.chunk_overlap+' tokens<br><b>Chat Model:</b> '+d.data.default_model+'<br><b>LLM Endpoint:</b> '+d.data.llm_endpoint+'<br><b>Port:</b> '+d.data.port}catch(e){}}
async function loadModels(){try{const r=await fetch(API+'/models');const d=await r.json();const sel=document.getElementById('model');sel.innerHTML='';if(d.data.models&&d.data.models.length>0){d.data.models.forEach(m=>{const opt=document.createElement('option');opt.value=m.id;opt.textContent=m.name+' ('+m.size_mb+'MB)';sel.appendChild(opt)});sel.addEventListener('change',swapModel)}else{sel.innerHTML='<option>no models found</option>'}}catch(e){document.getElementById('model').innerHTML='<option>error loading models</option>'}}
async function swapModel(){var sel=document.getElementById('model');var model=sel.value;sel.disabled=true;msg('Loading '+model+'...','ok');try{var r=await fetch(API+'/models/load',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({model:model})});var d=await r.json();if(d.success){msg('Model loaded: '+d.data.model,'ok')}else{msg(d.error,'err')}}catch(e){msg('Failed: '+e.message,'err')}finally{sel.disabled=false}}
</script>
</body>
</html>`
