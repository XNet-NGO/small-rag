# Small-RAG: Self-Contained Portable RAG System

**Purpose:** Lightweight, zero-dependency RAG system with REST API for agent integration (AX, etc.)

---

## I. DESIGN PRINCIPLES

### Core Requirements

1. **Self-Contained** - Single binary or minimal dependencies
2. **Portable** - Data directory moves with binary
3. **Zero Runtime Deps** - Like AX: compile and run
4. **Agent-First API** - RESTful, streaming, tool-friendly
5. **Local-First** - CPU inference on commodity hardware
6. **Fast Startup** - <1 second to ready
7. **Memory Efficient** - Run on 4GB RAM machines
8. **Stateless** - No external services required

### Architecture Pattern

```
small-rag/
├── Binary (Go)
├── Embedded SQLite (vector store + metadata)
├── Embedded embedding model (GGUF quantized)
├── REST API server
└── Data directory (portable)
```

---

## II. TECHNOLOGY STACK

### Language: Go (like AX)

**Why:**
- ✅ Single binary compilation
- ✅ Zero runtime dependencies
- ✅ Fast startup (<100ms)
- ✅ Excellent concurrency
- ✅ Native HTTP/WebSocket
- ✅ Cross-platform (linux, macOS, Windows)

### Vector Store: SQLite + BLOB

**Why:**
- ✅ Zero external service
- ✅ Portable (single .db file)
- ✅ Full-text search + vector search
- ✅ ACID transactions
- ✅ Pure-Go driver (modernc.org/sqlite)
- ✅ Scales to 100K+ embeddings

**Schema:**

```sql
-- Documents
CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    title TEXT,
    source TEXT,  -- file path, URL, etc.
    content TEXT,
    created_at DATETIME,
    updated_at DATETIME
);

-- Chunks (document fragments)
CREATE TABLE chunks (
    id TEXT PRIMARY KEY,
    doc_id TEXT REFERENCES documents(id),
    chunk_index INTEGER,
    text TEXT,
    tokens INTEGER,
    created_at DATETIME,
    UNIQUE(doc_id, chunk_index)
);

-- Vector embeddings (stored as BLOB)
CREATE TABLE embeddings (
    id TEXT PRIMARY KEY,
    chunk_id TEXT UNIQUE REFERENCES chunks(id),
    embedding BLOB,  -- float32 array, 384 dims
    model_id TEXT,
    created_at DATETIME
);

-- Search index (FTS5 for full-text)
CREATE VIRTUAL TABLE chunks_fts USING fts5(
    chunk_id UNINDEXED,
    text,
    content=chunks,
    content_rowid=id
);

-- Settings (key-value)
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT
);
```

### Embedding Model: Qwen3-Embedding-0.6B (GGUF)

**Why:**
- ✅ 379MB quantized (Q4_K_M)
- ✅ Runs on CPU (~100-200ms per chunk)
- ✅ 384-dim embeddings
- ✅ Multilingual
- ✅ Fits in 2GB RAM
- ✅ Open-source (Apache 2.0)

### LLM Integration: Provider Agnostic

**Support:**
- OpenAI-compatible (via HTTP)
- AWS Bedrock (via SDK)
- Local (Ollama, llama.cpp)
- Configurable per request

### HTTP Framework: Chi Router

**Why:**
- ✅ Lightweight (~50KB)
- ✅ Middleware support
- ✅ No magic
- ✅ Standard net/http compatible

### Streaming: Server-Sent Events (SSE)

**Why:**
- ✅ Native HTTP (no WebSocket)
- ✅ Easier for agents
- ✅ Built-in browser support
- ✅ Simpler than WebSocket

---

## III. API SPECIFICATION

### Base URL

```
http://localhost:8765/api/v1
```

### 1. Health Check

```http
GET /health

Response 200:
{
  "status": "ready",
  "version": "0.1.0",
  "embeddings_count": 1250,
  "documents_count": 45,
  "uptime_seconds": 3600
}
```

### 2. Upload Document

```http
POST /documents

Content-Type: multipart/form-data

Form fields:
- file: <PDF, TXT, MD>
- title: (optional)
- source: (optional, e.g., "https://...")

Response 201:
{
  "id": "doc-abc123",
  "title": "My Document",
  "chunks_created": 12,
  "embeddings_created": 12,
  "status": "indexed"
}
```

### 3. Search (Hybrid)

```http
POST /search

{
  "query": "What is machine learning?",
  "top_k": 5,
  "search_type": "hybrid",  // hybrid, semantic, keyword
  "min_score": 0.3,
  "include_metadata": true
}

Response 200:
{
  "query": "What is machine learning?",
  "results": [
    {
      "chunk_id": "chunk-123",
      "doc_id": "doc-abc",
      "text": "Machine learning is a subset of AI...",
      "score": 0.87,
      "source": "ml-guide.pdf",
      "page": 5
    },
    ...
  ],
  "search_time_ms": 45
}
```

### 4. RAG Query (Streaming)

```http
POST /rag/query

{
  "query": "Summarize the main points about ML",
  "top_k": 3,
  "model": "claude-3-opus",  // or gpt-4, bedrock:claude-opus-4-6, etc.
  "system_prompt": "You are a helpful assistant",
  "temperature": 0.7,
  "stream": true
}

Response 200 (text/event-stream):
data: {"type":"context","chunks":3}
data: {"type":"delta","text":"Machine learning"}
data: {"type":"delta","text":" is a powerful"}
data: {"type":"delta","text":" technology..."}
data: {"type":"done","total_tokens":342}
```

### 5. Get Document

```http
GET /documents/{doc_id}

Response 200:
{
  "id": "doc-abc123",
  "title": "My Document",
  "source": "file.pdf",
  "chunks_count": 12,
  "created_at": "2026-07-14T20:15:00Z",
  "updated_at": "2026-07-14T20:15:00Z"
}
```

### 6. List Documents

```http
GET /documents?limit=20&offset=0

Response 200:
{
  "documents": [...],
  "total": 45,
  "limit": 20,
  "offset": 0
}
```

### 7. Delete Document

```http
DELETE /documents/{doc_id}

Response 204
```

### 8. Config

```http
GET /config

Response 200:
{
  "embedding_model": "qwen3-embedding-0.6b",
  "embedding_dims": 384,
  "chunk_size": 512,
  "chunk_overlap": 128,
  "search_types": ["semantic", "keyword", "hybrid"],
  "supported_llms": ["openai", "bedrock", "ollama", "openai-compatible"]
}
```

### 9. Agent Tool: Search & RAG

```http
POST /tools/search_and_rag

{
  "query": "Find info about X and answer Y",
  "top_k": 5,
  "model": "claude-3-opus",
  "include_sources": true
}

Response 200:
{
  "query": "...",
  "answer": "Based on the documents, ...",
  "sources": [
    {"doc_id": "doc-123", "title": "...", "chunks": [0, 1, 2]}
  ],
  "tokens_used": 450
}
```

### 10. Batch Index

```http
POST /batch/index

{
  "documents": [
    {"path": "/data/doc1.pdf", "title": "Doc 1"},
    {"path": "/data/doc2.txt", "title": "Doc 2"}
  ]
}

Response 202 (Accepted):
{
  "batch_id": "batch-xyz",
  "status": "processing",
  "total": 2,
  "completed": 0
}

# Poll for status:
GET /batch/{batch_id}

Response 200:
{
  "batch_id": "batch-xyz",
  "status": "completed",
  "total": 2,
  "completed": 2,
  "results": [
    {"doc_id": "doc-1", "chunks": 12},
    {"doc_id": "doc-2", "chunks": 8}
  ]
}
```

---

## IV. AGENT INTEGRATION (AX)

### Tool Definition for AX

```json
{
  "type": "function",
  "function": {
    "name": "rag_search",
    "description": "Search knowledge base and get RAG answer",
    "parameters": {
      "type": "object",
      "required": ["query"],
      "properties": {
        "query": {
          "type": "string",
          "description": "Search query"
        },
        "top_k": {
          "type": "integer",
          "description": "Number of results (default 5)"
        },
        "search_type": {
          "type": "string",
          "enum": ["semantic", "keyword", "hybrid"],
          "description": "Search method (default: hybrid)"
        }
      }
    }
  }
}
```

### AX Agent Integration

```go
// In AX's tool executor
case "rag_search":
    query := args["query"].(string)
    topK := 5
    if k, ok := args["top_k"].(float64); ok {
        topK = int(k)
    }
    
    // Call small-rag API
    resp, err := http.Post(
        "http://localhost:8765/api/v1/search",
        "application/json",
        bytes.NewBufferString(fmt.Sprintf(`{
            "query": %q,
            "top_k": %d,
            "search_type": "hybrid"
        }`, query, topK)),
    )
    
    // Parse response and return results
    return parseSearchResults(resp)
```

### Example AX Workflow

```
User: "What are the best practices for deploying ML models?"

AX (architect):
  1. Task: "Research ML deployment best practices"
  2. Calls: rag_search("ML model deployment best practices")
  3. Gets: 5 relevant chunks from docs
  4. Calls: search_web("ML deployment 2026")
  5. Combines results
  6. Spawns: coder agent with findings

AX (coder):
  1. Task: "Create deployment script based on findings"
  2. Calls: rag_search("Docker Kubernetes deployment examples")
  3. Gets: Code examples from knowledge base
  4. Writes: deployment.yaml
  5. Reports: "Deployment script ready"

Result: Deployment script grounded in both:
  - Your internal knowledge base
  - Web research
  - LLM reasoning
```

---

## V. DATA PORTABILITY

### Directory Structure

```
~/.small-rag/              # Default location
├── small-rag.db           # SQLite (documents, chunks, embeddings)
├── models/
│   ├── qwen3-embedding-0.6b-q4_k_m.gguf  # Embedding model
│   └── config.json        # Model metadata
├── config.json            # App settings
├── cache/
│   └── llm_responses/     # Optional LLM response cache
└── logs/
    └── small-rag.log
```

### Portable Mode

```bash
# Option 1: Next to binary
./small-rag
# Auto-detects: ./data/ or .small-rag/ in same dir

# Option 2: Custom location
./small-rag --data-dir /tmp/rag-session

# Option 3: Environment variable
export SMALL_RAG_DATA=/mnt/usb/rag-data
./small-rag

# Option 4: Docker volume
docker run -v /data/rag:/root/.small-rag small-rag:latest
```

### Migration/Backup

```bash
# Backup everything
tar -czf rag-backup.tar.gz ~/.small-rag/

# Restore
tar -xzf rag-backup.tar.gz -C ~/

# Move to new machine
scp -r ~/.small-rag/ user@newmachine:~/
```

### Data Format

- **Embeddings:** Float32 arrays in BLOB (portable across architectures)
- **Database:** SQLite (portable, no migrations needed)
- **Models:** GGUF format (portable, no conversion needed)

---

## VI. DEPLOYMENT OPTIONS

### Option 1: Standalone Binary

```bash
# Download pre-built binary
wget https://github.com/xnet-admin-1/small-rag/releases/download/v0.1.0/small-rag-linux-amd64
chmod +x small-rag-linux-amd64

# Run
./small-rag-linux-amd64
# Server ready on :8765

# Upload documents
curl -F "file=@doc.pdf" http://localhost:8765/api/v1/documents

# Query
curl -X POST http://localhost:8765/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query":"What is RAG?"}'
```

### Option 2: With AX

```bash
# Start small-rag in background
./small-rag &

# Use in AX
./ax -p "Search the knowledge base and explain ML concepts"

# AX automatically uses rag_search tool
```

### Option 3: Docker

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o small-rag .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/small-rag /usr/local/bin/
COPY --from=builder /app/models/ /root/.small-rag/models/
EXPOSE 8765
CMD ["small-rag"]
```

### Option 4: Embedded in AX

```go
// Future: embed small-rag as library in AX
import "github.com/xnet-admin-1/small-rag/pkg/rag"

// Initialize
kb := rag.NewKnowledgeBase("./data/")

// Use directly (no API)
results := kb.Search("What is X?")
```

---

## VII. PERFORMANCE CHARACTERISTICS

### Latency

| Operation | Time | Notes |
|-----------|------|-------|
| Startup | 100-200ms | Load model into memory |
| Embed chunk (512 tokens) | 150-300ms | CPU inference |
| Search 1M embeddings | 50-100ms | SQLite FTS + vector |
| RAG query (streaming) | 2-5s | First token to LLM |

### Memory Usage

| Component | RAM | Notes |
|-----------|-----|-------|
| Embedding model (loaded) | 1.2 GB | GGUF quantized |
| SQLite (100K embeddings) | 200 MB | In-memory cache |
| Server + buffers | 300 MB | Go runtime |
| **Total** | **~1.7 GB** | Fits on 4GB machine |

### Storage

| Component | Size | Notes |
|-----------|------|-------|
| Binary | 30 MB | Statically compiled |
| Embedding model | 379 MB | GGUF Q4 |
| SQLite (100K chunks) | 500 MB | Embeddings + metadata |
| **Total** | **~1 GB** | Portable USB stick |

---

## VIII. IMPLEMENTATION ROADMAP

### Phase 1: MVP (Week 1-2)

- [x] SQLite schema + Go driver
- [ ] Document upload + chunking
- [ ] Embedding generation (CPU)
- [ ] Search API (hybrid)
- [ ] Health check + basic endpoints

### Phase 2: RAG + Streaming (Week 2-3)

- [ ] LLM provider routing
- [ ] RAG query with streaming
- [ ] SSE response format
- [ ] Agent tool integration
- [ ] Batch indexing

### Phase 3: Polish (Week 3-4)

- [ ] Web UI (optional)
- [ ] Docker build
- [ ] Performance tuning
- [ ] Documentation
- [ ] Release binary

### Phase 4: Integration (Week 4+)

- [ ] AX integration
- [ ] MCP server wrapper
- [ ] Advanced features (reranking, etc.)
- [ ] Multi-model support

---

## IX. COMPARISON TO ALTERNATIVES

| System | Size | Deps | Portable | API | Agents | Cost |
|--------|------|------|----------|-----|--------|------|
| **Small-RAG** | 30MB | 0 | ✅ | REST | ✅ | Free |
| Pinecone | Cloud | ✅ | ❌ | REST | ✅ | $$ |
| Weaviate | 500MB | Docker | ⚠️ | REST | ✅ | $$ |
| Qdrant | 100MB | Rust | ✅ | REST | ✅ | Free |
| Milvus | 1GB+ | K8s | ❌ | REST | ✅ | Free |
| LlamaIndex | 50MB | Python | ⚠️ | Library | ✅ | Free |
| RAGFlow | 1GB+ | Docker | ⚠️ | REST | ⚠️ | Free |

**Small-RAG Advantages:**
- ✅ Smallest binary (30MB)
- ✅ Zero dependencies
- ✅ Fully portable
- ✅ Agent-first design
- ✅ Local-first (no cloud)
- ✅ Fast startup

---

## X. SECURITY & PRIVACY

### No External Calls

- Embeddings computed locally (CPU)
- Search executed locally (SQLite)
- LLM calls optional + configurable
- No telemetry
- No tracking

### API Security

```go
// Optional API key
Authorization: Bearer sk-small-rag-abc123

// Rate limiting
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95

// CORS (for web UI)
Access-Control-Allow-Origin: http://localhost:3000
```

### Data Encryption

- At rest: Optional SQLite encryption (SQLCipher)
- In transit: HTTPS support (with cert)
- In memory: Cleared on shutdown

---

## XI. INTEGRATION WITH AX

### Architecture

```
AX (agent)
    ↓
Calls: rag_search("query")
    ↓
HTTP POST: localhost:8765/api/v1/search
    ↓
Small-RAG (server)
    ↓
1. Embed query (CPU, 0.6B model)
    ↓
2. Search embeddings (SQLite)
    ↓
3. Hybrid ranking (semantic + keyword)
    ↓
4. Return top-K results
    ↓
AX receives results
    ↓
AX includes in LLM context
    ↓
LLM generates grounded answer
```

### Example: AX + Small-RAG Workflow

```
User: "Based on our docs, how do we deploy services?"

1. AX receives task
2. AX calls: rag_search("service deployment")
3. Small-RAG returns:
   - 5 chunks from deployment guide
   - 3 chunks from best practices
   - Metadata: doc IDs, pages, scores

4. AX includes in system context:
   "Based on internal docs:
   - Service deployment follows these steps...
   - Best practices include..."

5. LLM generates deployment guide
6. AX can then:
   - Call coder agent to write scripts
   - Call qa agent to create tests
   - Call devops agent to deploy

Result: Fully grounded, multi-step workflow
```

---

## XII. CONCLUSION

**Small-RAG is designed to be:**

1. **Self-contained** - Single binary, zero deps (like AX)
2. **Portable** - Data moves with binary
3. **Fast** - <100ms startup, 50-100ms search
4. **Lightweight** - 30MB binary, 1.7GB runtime RAM
5. **Agent-first** - REST API optimized for tool calling
6. **Local-first** - No external services required
7. **Production-ready** - ACID transactions, error handling
8. **Extensible** - Plugin architecture for custom features

**Perfect for:**
- Local RAG for AX and other agents
- Portable knowledge bases
- Privacy-first deployments
- Resource-constrained environments
- Offline-first applications

**Next Steps:**
1. Create Go project structure
2. Implement SQLite schema + chunking
3. Integrate GGUF embedding model
4. Build REST API
5. Test with AX integration
