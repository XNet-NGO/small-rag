// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 xnet-admin-1

package mcp

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/xnet-admin-1/small-rag/internal/config"
	dbpkg "github.com/xnet-admin-1/small-rag/internal/db"
	"github.com/xnet-admin-1/small-rag/internal/document"
	"github.com/xnet-admin-1/small-rag/internal/embedding"
	"github.com/xnet-admin-1/small-rag/internal/llm"
	"github.com/xnet-admin-1/small-rag/internal/search"
)

// Server implements the MCP protocol over stdio using JSON-RPC 2.0.
type Server struct {
	db        *sql.DB
	cfg       *config.Config
	embedding *embedding.Engine
	search    *search.Engine
	llm       *llm.Client
	logger    *log.Logger
}

// JSON-RPC types

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error object.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	errCodeParse          = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
	errCodeInternal       = -32603
)

// NewServer creates a new MCP server.
func NewServer(db *sql.DB, cfg *config.Config, emb *embedding.Engine, llmClient *llm.Client) *Server {
	return &Server{
		db:        db,
		cfg:       cfg,
		embedding: emb,
		search:    search.NewEngine(db),
		llm:       llmClient,
		logger:    log.New(os.Stderr, "[mcp] ", log.LstdFlags),
	}
}

// Run starts the MCP server, reading from stdin and writing to stdout.
func (s *Server) Run() error {
	// Initialize embedding engine.
	// Temporarily redirect stdout to stderr during init because the embedding
	// library prints loading messages to stdout, which would corrupt the
	// JSON-RPC protocol stream.
	origStdout := os.Stdout
	os.Stdout = os.Stderr
	if err := s.embedding.Initialize(); err != nil {
		s.logger.Printf("WARNING: embedding engine initialization failed: %v", err)
		s.logger.Printf("Search and indexing tools will not work without the embedding model.")
	}
	os.Stdout = origStdout

	s.logger.Printf("MCP server started, reading from stdin")

	scanner := bufio.NewScanner(os.Stdin)
	// Allow large messages (16MB)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeResponse(Response{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &RPCError{
					Code:    errCodeParse,
					Message: "Parse error",
					Data:    err.Error(),
				},
			})
			continue
		}

		resp := s.handleRequest(&req)
		s.writeResponse(resp)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin read error: %w", err)
	}

	s.logger.Printf("MCP server shutting down (stdin closed)")
	return nil
}

func (s *Server) writeResponse(resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.logger.Printf("ERROR: failed to marshal response: %v", err)
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

func (s *Server) handleRequest(req *Request) Response {
	s.logger.Printf("Received method: %s", req.Method)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    errCodeMethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

// --- initialize ---

func (s *Server) handleInitialize(req *Request) Response {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "small-rag",
			"version": "0.1.0",
		},
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// --- tools/list ---

func (s *Server) handleToolsList(req *Request) Response {
	tools := []map[string]interface{}{
		{
			"name":        "rag_search",
			"description": "Search the knowledge base using semantic, keyword, or hybrid search.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query.",
					},
					"top_k": map[string]interface{}{
						"type":        "integer",
						"description": "Number of results to return (default 5).",
					},
					"search_type": map[string]interface{}{
						"type":        "string",
						"description": "Search type: semantic, keyword, or hybrid (default hybrid).",
						"enum":        []string{"semantic", "keyword", "hybrid"},
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "rag_query",
			"description": "Search the knowledge base and generate an answer using LLM (or return formatted results if no LLM configured).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The question to answer.",
					},
					"top_k": map[string]interface{}{
						"type":        "integer",
						"description": "Number of search results to use as context (default 3).",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "document_index",
			"description": "Index a document from text content into the knowledge base.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The document text content to index.",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "The document title.",
					},
				},
				"required": []string{"content", "title"},
			},
		},
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// --- tools/call ---

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(req *Request) Response {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	switch params.Name {
	case "rag_search":
		return s.toolRagSearch(req.ID, params.Arguments)
	case "rag_query":
		return s.toolRagQuery(req.ID, params.Arguments)
	case "document_index":
		return s.toolDocumentIndex(req.ID, params.Arguments)
	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: fmt.Sprintf("Unknown tool: %s", params.Name),
			},
		}
	}
}

// --- rag_search tool ---

type ragSearchArgs struct {
	Query      string `json:"query"`
	TopK       int    `json:"top_k"`
	SearchType string `json:"search_type"`
}

func (s *Server) toolRagSearch(id interface{}, args json.RawMessage) Response {
	var a ragSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "Invalid arguments for rag_search",
				Data:    err.Error(),
			},
		}
	}

	if a.Query == "" {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "query is required",
			},
		}
	}
	if a.TopK <= 0 {
		a.TopK = 5
	}
	if a.SearchType == "" {
		a.SearchType = "hybrid"
	}

	// Generate query embedding for semantic/hybrid search
	var queryEmbedding []float32
	if a.SearchType != "keyword" {
		emb, err := s.embedding.Embed(a.Query)
		if err != nil {
			return Response{
				JSONRPC: "2.0",
				ID:      id,
				Error: &RPCError{
					Code:    errCodeInternal,
					Message: "Failed to generate query embedding",
					Data:    err.Error(),
				},
			}
		}
		queryEmbedding = emb
	}

	results, err := s.search.Search(a.Query, queryEmbedding, a.TopK, a.SearchType, s.cfg.MinScore)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInternal,
				Message: "Search failed",
				Data:    err.Error(),
			},
		}
	}

	// Build response items
	items := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		// Lookup document source
		source := ""
		doc, err := dbpkg.GetDocument(s.db, r.DocID)
		if err == nil && doc != nil {
			source = doc.Source
		}

		items = append(items, map[string]interface{}{
			"text":   r.Text,
			"score":  r.Score,
			"doc_id": r.DocID,
			"source": source,
		})
	}

	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": mustJSON(items),
				},
			},
		},
	}
}

// --- rag_query tool ---

type ragQueryArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

func (s *Server) toolRagQuery(id interface{}, args json.RawMessage) Response {
	var a ragQueryArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "Invalid arguments for rag_query",
				Data:    err.Error(),
			},
		}
	}

	if a.Query == "" {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "query is required",
			},
		}
	}
	if a.TopK <= 0 {
		a.TopK = 3
	}

	// Generate query embedding
	queryEmbedding, err := s.embedding.Embed(a.Query)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInternal,
				Message: "Failed to generate query embedding",
				Data:    err.Error(),
			},
		}
	}

	// Search
	results, err := s.search.Search(a.Query, queryEmbedding, a.TopK, "hybrid", s.cfg.MinScore)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInternal,
				Message: "Search failed",
				Data:    err.Error(),
			},
		}
	}

	// Build sources
	sources := make([]map[string]interface{}, 0, len(results))
	var contextParts []string
	for _, r := range results {
		title := ""
		doc, err := dbpkg.GetDocument(s.db, r.DocID)
		if err == nil && doc != nil {
			title = doc.Title
		}
		sources = append(sources, map[string]interface{}{
			"doc_id": r.DocID,
			"title":  title,
			"score":  r.Score,
		})
		contextParts = append(contextParts, r.Text)
	}

	// If LLM is configured, generate answer
	var answer string
	var tokensUsed int

	if s.llm != nil {
		contextText := strings.Join(contextParts, "\n\n---\n\n")
		prompt := fmt.Sprintf("Based on the following context, answer the question.\n\nContext:\n%s\n\nQuestion: %s\n\nAnswer:", contextText, a.Query)

		resp, err := s.llm.ChatCompletion(context.Background(), llm.ChatRequest{
			Messages: []llm.Message{
				{Role: "user", Content: prompt},
			},
			Temperature: 0.3,
			MaxTokens:   1024,
		})
		if err != nil {
			s.logger.Printf("LLM call failed, falling back to context: %v", err)
			answer = formatContextAsAnswer(a.Query, contextParts)
		} else if len(resp.Choices) > 0 {
			answer = resp.Choices[0].Message.Content
			tokensUsed = resp.Usage.TotalTokens
		} else {
			answer = formatContextAsAnswer(a.Query, contextParts)
		}
	} else {
		// No LLM configured - return formatted search results
		answer = formatContextAsAnswer(a.Query, contextParts)
	}

	result := map[string]interface{}{
		"answer":      answer,
		"sources":     sources,
		"tokens_used": tokensUsed,
	}

	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": mustJSON(result),
				},
			},
		},
	}
}

// --- document_index tool ---

type documentIndexArgs struct {
	Content string `json:"content"`
	Title   string `json:"title"`
}

func (s *Server) toolDocumentIndex(id interface{}, args json.RawMessage) Response {
	var a documentIndexArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "Invalid arguments for document_index",
				Data:    err.Error(),
			},
		}
	}

	if a.Content == "" {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "content is required",
			},
		}
	}
	if a.Title == "" {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "title is required",
			},
		}
	}

	// Create document
	docID := uuid.New().String()

	// Save document to DB
	doc := &dbpkg.Document{
		ID:      docID,
		Title:   a.Title,
		Source:  "mcp",
		Content: a.Content,
	}
	if err := dbpkg.SaveDocument(s.db, doc); err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInternal,
				Message: "Failed to save document",
				Data:    err.Error(),
			},
		}
	}

	// Chunk the document
	docObj := document.NewDocument(docID, a.Title, "mcp", a.Content, s.cfg.ChunkSize, s.cfg.ChunkOverlap)
	if err := docObj.Chunk(); err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &RPCError{
				Code:    errCodeInternal,
				Message: "Failed to chunk document",
				Data:    err.Error(),
			},
		}
	}

	// Save chunks and embeddings
	chunksCreated := 0
	for _, chunk := range docObj.Chunks {
		// Save chunk
		dbChunk := &dbpkg.Chunk{
			ID:         chunk.ID,
			DocID:      docID,
			ChunkIndex: chunk.Index,
			Text:       chunk.Text,
			Tokens:     chunk.Tokens,
		}
		if err := dbpkg.SaveChunk(s.db, dbChunk); err != nil {
			s.logger.Printf("Failed to save chunk %s: %v", chunk.ID, err)
			continue
		}

		// Generate embedding
		emb, err := s.embedding.Embed(chunk.Text)
		if err != nil {
			s.logger.Printf("Failed to embed chunk %s: %v", chunk.ID, err)
			continue
		}

		// Save embedding
		embID := uuid.New().String()
		dbEmb := &dbpkg.Embedding{
			ID:        embID,
			ChunkID:   chunk.ID,
			Embedding: search.EncodeEmbedding(emb),
			ModelID:   s.cfg.EmbeddingModel,
			Dims:      len(emb),
		}
		if err := dbpkg.SaveEmbedding(s.db, dbEmb); err != nil {
			s.logger.Printf("Failed to save embedding for chunk %s: %v", chunk.ID, err)
			continue
		}

		chunksCreated++
	}

	result := map[string]interface{}{
		"doc_id":         docID,
		"chunks_created": chunksCreated,
	}

	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": mustJSON(result),
				},
			},
		},
	}
}

// --- resources/list ---

func (s *Server) handleResourcesList(req *Request) Response {
	rows, err := s.db.Query(`SELECT id, title FROM documents ORDER BY created_at DESC`)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    errCodeInternal,
				Message: "Failed to list documents",
				Data:    err.Error(),
			},
		}
	}
	defer rows.Close()

	resources := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}
		resources = append(resources, map[string]interface{}{
			"uri":      fmt.Sprintf("rag://documents/%s", id),
			"name":     title,
			"mimeType": "text/plain",
		})
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"resources": resources,
		},
	}
}

// --- resources/read ---

type resourceReadParams struct {
	URI string `json:"uri"`
}

func (s *Server) handleResourcesRead(req *Request) Response {
	var params resourceReadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	// Parse URI: rag://documents/{doc_id}
	const prefix = "rag://documents/"
	if !strings.HasPrefix(params.URI, prefix) {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: fmt.Sprintf("Invalid resource URI: %s", params.URI),
			},
		}
	}

	docID := strings.TrimPrefix(params.URI, prefix)
	if docID == "" {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "Document ID is required in URI",
			},
		}
	}

	doc, err := dbpkg.GetDocument(s.db, docID)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    errCodeInternal,
				Message: "Document not found",
				Data:    err.Error(),
			},
		}
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":      params.URI,
					"mimeType": "text/plain",
					"text":     doc.Content,
				},
			},
		},
	}
}

// --- helpers ---

func formatContextAsAnswer(query string, contextParts []string) string {
	if len(contextParts) == 0 {
		return "No relevant information found in the knowledge base."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Based on the knowledge base, here are the relevant passages for: %q\n\n", query))
	for i, part := range contextParts {
		sb.WriteString(fmt.Sprintf("--- Result %d ---\n%s\n\n", i+1, part))
	}
	return sb.String()
}

func mustJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
