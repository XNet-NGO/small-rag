# Small-RAG Phase 1 Implementation Plan

**Status:** Ready to Start  
**Target:** MVP with Document Upload + Search  
**Timeline:** 1-2 weeks

---

## Phase 1 Goals

Implement core document processing pipeline:
1. Upload documents (PDF, TXT, MD)
2. Parse and extract text
3. Chunk text (512 tokens, 128 overlap)
4. Generate embeddings (Qwen3-0.6B CPU)
5. Store in SQLite
6. Implement hybrid search (semantic + keyword)

---

## Task Breakdown

### Task 1: Document Parsing (2-3 days)

**Files to Create/Update:**
- `internal/document/parser.go` - Document parsing logic
- `internal/document/pdf.go` - PDF parsing
- `internal/document/text.go` - TXT/MD parsing

**Dependencies:**
```go
github.com/ledongthuc/pdf  // PDF parsing
```

**Implementation:**
```go
// internal/document/parser.go
package document

import (
    "fmt"
    "path/filepath"
    "strings"
)

type Parser struct{}

func NewParser() *Parser {
    return &Parser{}
}

func (p *Parser) Parse(filename string, data []byte) (string, error) {
    ext := strings.ToLower(filepath.Ext(filename))
    
    switch ext {
    case ".pdf":
        return parsePDF(data)
    case ".txt", ".md":
        return parseText(data)
    default:
        return "", fmt.Errorf("unsupported file type: %s", ext)
    }
}
```

**Testing:**
- Upload test.pdf → extract text
- Upload test.txt → extract text
- Upload test.md → extract text
- Verify encoding (UTF-8)

---

### Task 2: Text Chunking (1-2 days)

**Files to Create/Update:**
- `internal/document/chunker.go` - Chunking logic

**Implementation:**
```go
// internal/document/chunker.go
package document

import "strings"

type Chunker struct {
    ChunkSize    int // 512 tokens
    ChunkOverlap int // 128 tokens
}

func NewChunker(size, overlap int) *Chunker {
    return &Chunker{
        ChunkSize:    size,
        ChunkOverlap: overlap,
    }
}

func (c *Chunker) Chunk(text string) []string {
    // Simple word-based chunking
    words := strings.Fields(text)
    var chunks []string
    
    for i := 0; i < len(words); i += (c.ChunkSize - c.ChunkOverlap) {
        end := i + c.ChunkSize
        if end > len(words) {
            end = len(words)
        }
        
        chunk := strings.Join(words[i:end], " ")
        chunks = append(chunks, chunk)
        
        if end >= len(words) {
            break
        }
    }
    
    return chunks
}
```

**Testing:**
- Chunk 1000-word document
- Verify chunk size ≈ 512 words
- Verify overlap = 128 words
- Test edge cases (empty, single word, etc.)

---

### Task 3: Embedding Integration (2-3 days)

**Files to Update:**
- `internal/embedding/engine.go` - Load GGUF model

**Dependencies:**
```bash
# Download model
cd ~/.small-rag/models/
wget https://huggingface.co/Qwen/Qwen3-Embedding-0.6B-GGUF/resolve/main/qwen3-embedding-0.6b-q4_k_m.gguf
```

**Implementation:**
```go
// internal/embedding/engine.go
package embedding

import (
    "fmt"
    // Add GGUF loading library
)

type Engine struct {
    modelPath string
    dims      int
    model     interface{} // GGUF model instance
}

func NewEngine(modelPath string, dims int) *Engine {
    return &Engine{
        modelPath: modelPath,
        dims:      dims,
    }
}

func (e *Engine) Load() error {
    // Load GGUF model
    // Initialize inference
    return nil
}

func (e *Engine) Embed(text string) ([]float32, error) {
    // Generate embedding
    // Return 384-dim vector
    return make([]float32, 384), nil
}

func (e *Engine) EmbedBatch(texts []string) ([][]float32, error) {
    // Batch embedding for efficiency
    embeddings := make([][]float32, len(texts))
    for i, text := range texts {
        emb, err := e.Embed(text)
        if err != nil {
            return nil, err
        }
        embeddings[i] = emb
    }
    return embeddings, nil
}
```

**Testing:**
- Load model successfully
- Generate embedding for "hello world"
- Verify output: 384-dim float32 array
- Benchmark: <300ms per chunk on CPU

---

### Task 4: Document Upload Handler (2 days)

**Files to Update:**
- `internal/api/handlers.go` - Complete handleUploadDocument

**Implementation:**
```go
// internal/api/handlers.go
func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
    // 1. Parse multipart form
    if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
        respondJSON(w, 400, map[string]interface{}{"error": "Failed to parse form"})
        return
    }
    
    // 2. Get file
    file, header, err := r.FormFile("file")
    if err != nil {
        respondJSON(w, 400, map[string]interface{}{"error": "No file provided"})
        return
    }
    defer file.Close()
    
    // 3. Read file data
    fileData, err := io.ReadAll(file)
    if err != nil {
        respondJSON(w, 400, map[string]interface{}{"error": "Failed to read file"})
        return
    }
    
    // 4. Parse document
    parser := document.NewParser()
    content, err := parser.Parse(header.Filename, fileData)
    if err != nil {
        respondJSON(w, 400, map[string]interface{}{"error": fmt.Sprintf("Parse failed: %v", err)})
        return
    }
    
    // 5. Chunk text
    chunker := document.NewChunker(512, 128)
    chunks := chunker.Chunk(content)
    
    // 6. Generate embeddings
    embeddings, err := s.embedding.EmbedBatch(chunks)
    if err != nil {
        respondJSON(w, 500, map[string]interface{}{"error": "Embedding failed"})
        return
    }
    
    // 7. Store in database
    docID := uuid.New().String()
    title := r.FormValue("title")
    if title == "" {
        title = header.Filename
    }
    
    // Store document
    _, err = s.db.Exec("INSERT INTO documents (id, title, content) VALUES (?, ?, ?)", 
        docID, title, content)
    if err != nil {
        respondJSON(w, 500, map[string]interface{}{"error": "DB insert failed"})
        return
    }
    
    // Store chunks and embeddings
    for i, chunk := range chunks {
        chunkID := uuid.New().String()
        
        // Store chunk
        _, err = s.db.Exec("INSERT INTO chunks (id, doc_id, chunk_index, text) VALUES (?, ?, ?, ?)",
            chunkID, docID, i, chunk)
        if err != nil {
            continue
        }
        
        // Store embedding
        embBytes := float32ArrayToBytes(embeddings[i])
        _, err = s.db.Exec("INSERT INTO embeddings (id, chunk_id, embedding, model_id, dims) VALUES (?, ?, ?, ?, ?)",
            uuid.New().String(), chunkID, embBytes, "qwen3-0.6b", 384)
    }
    
    // 8. Return response
    respondJSON(w, 201, map[string]interface{}{
        "success": true,
        "data": map[string]interface{}{
            "id": docID,
            "title": title,
            "chunks_created": len(chunks),
            "embeddings_created": len(embeddings),
        },
    })
}
```

**Testing:**
- Upload PDF → verify chunks created
- Upload TXT → verify embeddings stored
- Check database: documents, chunks, embeddings tables
- Verify web UI shows document in list

---

### Task 5: Search Implementation (2-3 days)

**Files to Update:**
- `internal/rag/search.go` - Implement SearchEngine

**Implementation:**
```go
// internal/rag/search.go
package rag

import (
    "database/sql"
    "math"
)

type SearchEngine struct {
    db *sql.DB
}

func NewSearchEngine(db *sql.DB) *SearchEngine {
    return &SearchEngine{db: db}
}

type SearchResult struct {
    ChunkID  string  `json:"chunk_id"`
    DocID    string  `json:"doc_id"`
    Text     string  `json:"text"`
    Score    float32 `json:"score"`
    Metadata map[string]interface{} `json:"metadata"`
}

func (e *SearchEngine) Search(query string, queryEmbedding []float32, topK int, searchType string, minScore float32) ([]SearchResult, error) {
    switch searchType {
    case "semantic":
        return e.semanticSearch(queryEmbedding, topK, minScore)
    case "keyword":
        return e.keywordSearch(query, topK)
    case "hybrid":
        return e.hybridSearch(query, queryEmbedding, topK, minScore)
    default:
        return e.hybridSearch(query, queryEmbedding, topK, minScore)
    }
}

func (e *SearchEngine) semanticSearch(queryEmbedding []float32, topK int, minScore float32) ([]SearchResult, error) {
    // 1. Get all embeddings from database
    rows, err := e.db.Query("SELECT e.id, e.chunk_id, e.embedding, c.text, c.doc_id FROM embeddings e JOIN chunks c ON e.chunk_id = c.id")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var results []SearchResult
    
    // 2. Calculate cosine similarity for each
    for rows.Next() {
        var embID, chunkID, docID, text string
        var embBytes []byte
        
        if err := rows.Scan(&embID, &chunkID, &embBytes, &text, &docID); err != nil {
            continue
        }
        
        // Convert bytes to float32 array
        embedding := bytesToFloat32Array(embBytes)
        
        // Calculate cosine similarity
        score := cosineSimilarity(queryEmbedding, embedding)
        
        if score >= minScore {
            results = append(results, SearchResult{
                ChunkID: chunkID,
                DocID:   docID,
                Text:    text,
                Score:   score,
            })
        }
    }
    
    // 3. Sort by score (descending)
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })
    
    // 4. Return top-K
    if len(results) > topK {
        results = results[:topK]
    }
    
    return results, nil
}

func (e *SearchEngine) keywordSearch(query string, topK int) ([]SearchResult, error) {
    // FTS5 full-text search
    rows, err := e.db.Query(`
        SELECT c.id, c.doc_id, c.text, 
               bm25(chunks_fts) as score
        FROM chunks_fts 
        JOIN chunks c ON chunks_fts.chunk_id = c.id
        WHERE chunks_fts MATCH ?
        ORDER BY score DESC
        LIMIT ?
    `, query, topK)
    
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var results []SearchResult
    for rows.Next() {
        var r SearchResult
        if err := rows.Scan(&r.ChunkID, &r.DocID, &r.Text, &r.Score); err != nil {
            continue
        }
        results = append(results, r)
    }
    
    return results, nil
}

func (e *SearchEngine) hybridSearch(query string, queryEmbedding []float32, topK int, minScore float32) ([]SearchResult, error) {
    // 1. Get semantic results
    semanticResults, err := e.semanticSearch(queryEmbedding, topK*2, minScore)
    if err != nil {
        return nil, err
    }
    
    // 2. Get keyword results
    keywordResults, err := e.keywordSearch(query, topK*2)
    if err != nil {
        return nil, err
    }
    
    // 3. Combine and rerank
    combined := make(map[string]*SearchResult)
    
    // Add semantic results (weight: 0.7)
    for _, r := range semanticResults {
        combined[r.ChunkID] = &SearchResult{
            ChunkID: r.ChunkID,
            DocID:   r.DocID,
            Text:    r.Text,
            Score:   r.Score * 0.7,
        }
    }
    
    // Add keyword results (weight: 0.3)
    for _, r := range keywordResults {
        if existing, ok := combined[r.ChunkID]; ok {
            existing.Score += r.Score * 0.3
        } else {
            combined[r.ChunkID] = &SearchResult{
                ChunkID: r.ChunkID,
                DocID:   r.DocID,
                Text:    r.Text,
                Score:   r.Score * 0.3,
            }
        }
    }
    
    // 4. Convert to slice and sort
    var results []SearchResult
    for _, r := range combined {
        results = append(results, *r)
    }
    
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })
    
    // 5. Return top-K
    if len(results) > topK {
        results = results[:topK]
    }
    
    return results, nil
}

func cosineSimilarity(a, b []float32) float32 {
    var dot, normA, normB float32
    for i := range a {
        dot += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

func bytesToFloat32Array(b []byte) []float32 {
    // Convert []byte to []float32
    // Each float32 is 4 bytes
    result := make([]float32, len(b)/4)
    for i := range result {
        bits := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
        result[i] = math.Float32frombits(bits)
    }
    return result
}

func float32ArrayToBytes(f []float32) []byte {
    // Convert []float32 to []byte
    result := make([]byte, len(f)*4)
    for i, v := range f {
        bits := math.Float32bits(v)
        result[i*4] = byte(bits)
        result[i*4+1] = byte(bits >> 8)
        result[i*4+2] = byte(bits >> 16)
        result[i*4+3] = byte(bits >> 24)
    }
    return result
}
```

**Testing:**
- Upload document
- Search for keywords
- Verify results ranked by relevance
- Test semantic search
- Test keyword search
- Test hybrid search

---

### Task 6: End-to-End Testing (1 day)

**Test Scenarios:**

1. **Upload PDF**
   - Upload test.pdf
   - Verify chunks created
   - Verify embeddings stored
   - Check database

2. **Search**
   - Search for "machine learning"
   - Verify results returned
   - Check relevance scores
   - Test top-K parameter

3. **Web UI**
   - Upload via web UI
   - View document list
   - Search via web UI
   - View search results

4. **Performance**
   - Upload 10MB document
   - Measure time: <30 seconds
   - Search latency: <100ms
   - Memory usage: <2GB

---

## Dependencies to Add

```go
// go.mod additions
require (
    github.com/ledongthuc/pdf v0.0.0-20220302134840-0c2507a12d80
    github.com/google/uuid v1.6.0
    // GGUF loading library (TBD)
)
```

---

## Success Criteria

✅ Upload PDF/TXT/MD documents  
✅ Extract text correctly  
✅ Chunk text (512 tokens, 128 overlap)  
✅ Generate embeddings (384-dim)  
✅ Store in SQLite  
✅ Search works (semantic + keyword + hybrid)  
✅ Web UI functional  
✅ Performance targets met  

---

## Timeline

| Week | Tasks |
|------|-------|
| Week 1 | Tasks 1-3 (parsing, chunking, embedding) |
| Week 2 | Tasks 4-6 (upload, search, testing) |

**Total:** 10-14 days for complete Phase 1

---

## Next Steps

1. Create `internal/document/` package
2. Implement PDF parsing
3. Implement chunking
4. Download Qwen3-0.6B model
5. Integrate embedding generation
6. Complete upload handler
7. Implement search
8. Test end-to-end

---

**Status:** Ready to Start  
**First Task:** Implement document parsing  
**Location:** `/home/user-x/projects/small-rag/`
