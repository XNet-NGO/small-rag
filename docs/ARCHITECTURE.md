# Small-RAG Architecture

## Table of Contents

1. [System Overview](#system-overview)
2. [High-Level Architecture](#high-level-architecture)
3. [Component Architecture](#component-architecture)
4. [Data Flow](#data-flow)
5. [Database Design](#database-design)
6. [API Layer](#api-layer)
7. [Embedding Pipeline](#embedding-pipeline)
8. [Search Strategy](#search-strategy)
9. [Concurrency Model](#concurrency-model)
10. [Deployment Architecture](#deployment-architecture)

---

## System Overview

Small-RAG is a **self-contained, portable RAG (Retrieval-Augmented Generation) system** designed as a single Go binary with zero external dependencies.

### Core Principles

1. **Self-Contained** - Single binary, no external services
2. **Portable** - All data in `~/.small-rag/`, moves between machines
3. **Agent-First** - REST API optimized for autonomous agent tool calling
4. **Local-First** - CPU-based embeddings, no cloud calls
5. **Efficient** - Fast startup, low memory footprint

### Technology Stack

```
┌─────────────────────────────────────┐
│         Small-RAG Binary (30MB)     │
├─────────────────────────────────────┤
│  Language: Go 1.25                  │
│  HTTP: Chi router + net/http        │
│  Database: SQLite (pure-Go)         │
│  Embeddings: Qwen3-0.6B (GGUF)      │
│  Streaming: Server-Sent Events      │
└─────────────────────────────────────┘
```

---

## High-Level Architecture

### System Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                    External Agents                           │
│              (AX, other tools, clients)                      │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       │ HTTP/REST
                       ▼
┌──────────────────────────────────────────────────────────────┐
│                  Small-RAG Server                            │
│                   (:8765)                                    │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │            HTTP API Layer (Chi Router)                │  │
│  │  - /health                                            │  │
│  │  - /api/v1/documents (CRUD)                          │  │
│  │  - /api/v1/search                                    │  │
│  │  - /api/v1/rag/query (streaming SSE)                │  │
│  │  - /api/v1/tools/search_and_rag (agent tool)        │  │
│  │  - /api/v1/config                                   │  │
│  └────────────────────────────────────────────────────────┘  │
│                       ▲                                       │
│                       │                                       │
│  ┌────────────────────┴────────────────────────────────────┐  │
│  │          Business Logic Layer                          │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │  │
│  │  │   Document   │  │   Embedding  │  │   Search     │ │  │
│  │  │   Manager    │  │   Engine     │  │   Engine     │ │  │
│  │  │              │  │              │  │              │ │  │
│  │  │ - Upload     │  │ - Load GGUF  │  │ - Semantic   │ │  │
│  │  │ - Chunk      │  │ - Generate   │  │ - Keyword    │ │  │
│  │  │ - Store      │  │ - Cache      │  │ - Hybrid     │ │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │  │
│  │                                                        │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │  │
│  │  │     RAG      │  │   LLM        │  │  Config      │ │  │
│  │  │   Engine     │  │   Router     │  │  Manager     │ │  │
│  │  │              │  │              │  │              │ │  │
│  │  │ - Context    │  │ - Provider   │  │ - Load/Save  │ │  │
│  │  │ - Prompt     │  │ - Models     │  │ - Defaults   │ │  │
│  │  │ - Stream     │  │ - Streaming  │  │ - Validation │ │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │  │
│  └────────────────────────────────────────────────────────┘  │
│                       ▲                                       │
│                       │                                       │
│  ┌────────────────────┴────────────────────────────────────┐  │
│  │         Data Access Layer                              │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │  │
│  │  │   Document  │  │   Chunk      │  │  Embedding   │ │  │
│  │  │   DAO        │  │   DAO        │  │   DAO        │ │  │
│  │  │              │  │              │  │              │ │  │
│  │  │ - CRUD       │  │ - CRUD       │  │ - Store      │ │  │
│  │  │ - Search     │  │ - Search     │  │ - Retrieve   │ │  │
│  │  │ - Index      │  │ - Index      │  │ - Similarity │ │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │  │
│  └────────────────────────────────────────────────────────┘  │
│                       ▲                                       │
│                       │                                       │
│                   SQLite                                      │
│              (pure-Go driver)                                 │
│                                                              │
└──────────────────────────────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────┐
│              Data Directory (~/.small-rag/)                  │
│  ├── small-rag.db (SQLite database)                         │
│  ├── models/                                                │
│  │   └── qwen3-embedding-0.6b-q4_k_m.gguf                 │
│  ├── config.json                                           │
│  └── logs/                                                 │
└──────────────────────────────────────────────────────────────┘
```

---

## Component Architecture

### 1. HTTP API Layer (`internal/api/`)

**Responsibility:** Handle HTTP requests, routing, middleware

**Components:**
- `server.go` - HTTP server setup, route registration
- `handlers.go` - Request handlers for each endpoint

**Key Features:**
- Chi router for efficient routing
- Middleware: logging, recovery, CORS
- Request validation
- Response formatting (JSON, SSE)
- Error handling

**Endpoints:**
```
GET    /health
POST   /api/v1/documents
GET    /api/v1/documents
GET    /api/v1/documents/{id}
DELETE /api/v1/documents/{id}
POST   /api/v1/search
POST   /api/v1/rag/query
POST   /api/v1/tools/search_and_rag
GET    /api/v1/config
```

### 2. Document Manager (`internal/document/`)

**Responsibility:** Handle document upload, parsing, chunking

**Components:**
- `manager.go` - Orchestrate document lifecycle
- `parser.go` - Parse different file formats (PDF, TXT, MD)
- `chunker.go` - Split documents into chunks

**Key Features:**
- Multi-format support (PDF, TXT, MD)
- Configurable chunk size (default: 512 tokens)
- Overlap handling (default: 128 tokens)
- Duplicate detection (content hash)
- Metadata extraction

**Flow:**
```
Upload → Parse → Chunk → Hash → Store → Embed
```

### 3. Embedding Engine (`internal/embedding/`)

**Responsibility:** Generate vector embeddings for text

**Components:**
- `engine.go` - Orchestrate embedding generation
- `model.go` - Load and manage GGUF model
- `cache.go` - Cache embeddings in memory

**Key Features:**
- Load GGUF model (Qwen3-Embedding-0.6B)
- Batch embedding (multiple texts at once)
- CPU-based inference
- In-memory caching
- Configurable dimensions (384 default)

**Performance:**
- Model load: 100-200ms
- Per-chunk embedding: 150-300ms
- Cache hit: <1ms

### 4. Search Engine (`internal/search/`)

**Responsibility:** Execute search queries (semantic + keyword)

**Components:**
- `engine.go` - Orchestrate search
- `semantic.go` - Vector similarity search
- `keyword.go` - Full-text search (FTS5)
- `ranker.go` - Combine and rank results

**Search Types:**
```
Semantic:  Vector similarity (cosine)
Keyword:   Full-text search (FTS5)
Hybrid:    Combine both (weighted average)
```

**Ranking:**
```
Score = (semantic_score * semantic_weight) + 
        (keyword_score * keyword_weight)
```

### 5. RAG Engine (`internal/rag/`)

**Responsibility:** Orchestrate RAG query (search + generation)

**Components:**
- `engine.go` - Main RAG orchestration
- `context.go` - Build LLM context from results
- `prompt.go` - Build prompts
- `stream.go` - Handle streaming responses

**RAG Flow:**
```
1. Embed query (CPU)
2. Search for relevant chunks
3. Retrieve top-K results
4. Build context from results
5. Call LLM with context
6. Stream response (SSE)
```

### 6. LLM Router (`internal/llm/`)

**Responsibility:** Route to different LLM providers

**Components:**
- `router.go` - Provider selection and routing
- `openai.go` - OpenAI API client
- `bedrock.go` - AWS Bedrock client
- `ollama.go` - Local Ollama client
- `stream.go` - Streaming response handling

**Supported Providers:**
- OpenAI (GPT-4, GPT-3.5)
- AWS Bedrock (Claude, Llama, etc.)
- Ollama (local models)
- OpenAI-compatible endpoints

### 7. Data Access Layer (`internal/db/`)

**Responsibility:** Database operations

**Components:**
- `db.go` - SQLite connection management
- `schema.go` - Schema definition and queries
- `dao.go` - Data access objects

**Entities:**
- `Document` - Source documents
- `Chunk` - Document fragments
- `Embedding` - Vector embeddings
- `Settings` - Configuration

### 8. Configuration Manager (`internal/config/`)

**Responsibility:** Load and manage configuration

**Components:**
- `config.go` - Configuration struct and methods

**Settings:**
- Embedding model
- Chunk size/overlap
- Search parameters
- LLM defaults
- Feature flags

---

## Data Flow

### Document Upload Flow

```
User Upload
    ↓
HTTP POST /documents
    ↓
DocumentManager.Upload()
    ├─ Parse file (PDF/TXT/MD)
    ├─ Extract text
    ├─ Calculate content hash
    ├─ Store document metadata
    └─ Return document ID
    ↓
DocumentManager.Chunk()
    ├─ Split text into chunks (512 tokens)
    ├─ Add overlap (128 tokens)
    ├─ Store chunks with doc reference
    └─ Return chunk IDs
    ↓
EmbeddingEngine.Generate()
    ├─ Load GGUF model (if not cached)
    ├─ For each chunk:
    │  ├─ Embed text (CPU)
    │  ├─ Get 384-dim vector
    │  └─ Store embedding
    └─ Return embedding count
    ↓
HTTP Response
{
  "id": "doc-123",
  "chunks_created": 12,
  "embeddings_created": 12,
  "status": "indexed"
}
```

### Search Flow

```
User Query
    ↓
HTTP POST /search
{
  "query": "What is machine learning?",
  "top_k": 5,
  "search_type": "hybrid"
}
    ↓
SearchEngine.Search()
    ├─ Embed query (CPU)
    ├─ Execute semantic search
    │  ├─ Compute cosine similarity
    │  ├─ Get top-K by similarity
    │  └─ Return results with scores
    ├─ Execute keyword search
    │  ├─ FTS5 full-text search
    │  ├─ Get matching chunks
    │  └─ Return results with scores
    ├─ Hybrid ranking
    │  ├─ Normalize scores
    │  ├─ Combine (0.7 * semantic + 0.3 * keyword)
    │  ├─ Sort by combined score
    │  └─ Return top-K
    └─ Enrich results
       ├─ Get document metadata
       ├─ Add source info
       └─ Return to client
    ↓
HTTP Response
{
  "query": "...",
  "results": [
    {
      "chunk_id": "chunk-123",
      "doc_id": "doc-abc",
      "text": "Machine learning is...",
      "score": 0.87,
      "source": "ml-guide.pdf"
    },
    ...
  ],
  "search_time_ms": 45
}
```

### RAG Query Flow

```
User Query
    ↓
HTTP POST /rag/query
{
  "query": "Summarize ML concepts",
  "model": "gpt-4",
  "stream": true
}
    ↓
RAGEngine.Query()
    ├─ Search for relevant chunks
    │  ├─ Embed query
    │  ├─ Hybrid search
    │  └─ Get top-3 results
    ├─ Build context
    │  ├─ Format chunks
    │  ├─ Add metadata
    │  └─ Create context string
    ├─ Build prompt
    │  ├─ System prompt
    │  ├─ Context
    │  ├─ User query
    │  └─ Format for LLM
    ├─ Call LLM
    │  ├─ Route to provider (OpenAI/Bedrock/Ollama)
    │  ├─ Send request
    │  ├─ Get streaming response
    │  └─ Stream to client (SSE)
    └─ Log and store
       ├─ Save query
       ├─ Save response
       └─ Update metrics
    ↓
HTTP Response (text/event-stream)
data: {"type":"context","chunks":3}
data: {"type":"delta","text":"Machine learning"}
data: {"type":"delta","text":" is a powerful"}
data: {"type":"done","total_tokens":342}
```

---

## Database Design

### Schema Overview

```
documents
├── id (TEXT PRIMARY KEY)
├── title (TEXT)
├── source (TEXT)
├── content (TEXT)
├── content_hash (TEXT UNIQUE)
├── created_at (DATETIME)
└── updated_at (DATETIME)

chunks
├── id (TEXT PRIMARY KEY)
├── doc_id (TEXT FK → documents.id)
├── chunk_index (INTEGER)
├── text (TEXT)
├── tokens (INTEGER)
├── created_at (DATETIME)
└── UNIQUE(doc_id, chunk_index)

embeddings
├── id (TEXT PRIMARY KEY)
├── chunk_id (TEXT FK → chunks.id)
├── embedding (BLOB)  -- float32 array
├── model_id (TEXT)
├── dims (INTEGER)
└── created_at (DATETIME)

chunks_fts (Virtual FTS5)
├── chunk_id
├── text
└── content=chunks

settings
├── key (TEXT PRIMARY KEY)
├── value (TEXT)
└── updated_at (DATETIME)
```

### Indexes

```
idx_chunks_doc_id           ON chunks(doc_id)
idx_embeddings_chunk_id     ON embeddings(chunk_id)
idx_embeddings_model        ON embeddings(model_id)
idx_documents_created       ON documents(created_at)
```

### Triggers

```
chunks_ai    - Auto-insert into FTS on chunk insert
chunks_ad    - Auto-delete from FTS on chunk delete
chunks_au    - Auto-update FTS on chunk update
```

---

## API Layer

### Request/Response Format

**Standard JSON Request:**
```json
{
  "query": "search term",
  "top_k": 5,
  "search_type": "hybrid"
}
```

**Standard JSON Response:**
```json
{
  "success": true,
  "data": {...},
  "metadata": {
    "timestamp": "2026-07-14T20:30:00Z",
    "duration_ms": 45
  }
}
```

**Error Response:**
```json
{
  "success": false,
  "error": "Error message",
  "code": 400
}
```

### Streaming Response (SSE)

```
data: {"type":"context","chunks":3}
data: {"type":"delta","text":"Response text"}
data: {"type":"done","total_tokens":342}
```

---

## Embedding Pipeline

### Model Loading

```
1. Check cache (in-memory)
   ├─ Hit: Return cached model
   └─ Miss: Continue
2. Check disk (~/.small-rag/models/)
   ├─ Found: Load from disk
   └─ Not found: Download
3. Load GGUF file
   ├─ Parse GGUF format
   ├─ Load weights into memory
   ├─ Initialize inference engine
   └─ Cache in memory
```

### Embedding Generation

```
Input: Text chunk (512 tokens)
    ↓
Tokenize
    ├─ Split into tokens
    ├─ Add special tokens
    └─ Pad to max length
    ↓
Forward Pass (CPU)
    ├─ Load model (if not cached)
    ├─ Run inference
    ├─ Get output embeddings
    └─ Extract last layer
    ↓
Post-Processing
    ├─ Normalize (L2 norm)
    ├─ Convert to float32
    └─ Return 384-dim vector
    ↓
Output: float32[384]
```

### Batch Embedding

```
Inputs: [chunk1, chunk2, chunk3, ...]
    ↓
For each chunk (sequential or batch):
    ├─ Embed chunk
    ├─ Store embedding
    └─ Update progress
    ↓
Outputs: [embedding1, embedding2, ...]
```

---

## Search Strategy

### Semantic Search

```
Query: "What is machine learning?"
    ↓
Embed query (same model as chunks)
    → query_vector: float32[384]
    ↓
For each chunk embedding:
    ├─ Compute cosine similarity
    │  similarity = dot(query, chunk) / (norm(query) * norm(chunk))
    ├─ Score range: [-1, 1], normalized to [0, 1]
    └─ Keep score
    ↓
Sort by similarity (descending)
    ↓
Return top-K results
```

### Keyword Search

```
Query: "What is machine learning?"
    ↓
FTS5 Query
    ├─ Parse query terms
    ├─ Search in chunks_fts
    ├─ Get matching chunks
    └─ Calculate BM25 scores
    ↓
Sort by BM25 score (descending)
    ↓
Return top-K results
```

### Hybrid Ranking

```
Semantic Results: [chunk1(0.92), chunk2(0.87), ...]
Keyword Results:  [chunk3(0.95), chunk1(0.78), ...]
    ↓
Normalize scores to [0, 1]
    ↓
For each chunk:
    combined_score = (semantic_score * 0.7) + (keyword_score * 0.3)
    ↓
Sort by combined score (descending)
    ↓
Return top-K results
```

---

## Concurrency Model

### Goroutine Strategy

```
Main Server
├── HTTP Server (1 goroutine)
│   ├─ Listen on :8765
│   ├─ Accept connections
│   └─ Dispatch to handlers
│
├── Request Handlers (N goroutines)
│   ├─ Each request: 1 goroutine
│   ├─ Read request
│   ├─ Call business logic
│   └─ Write response
│
├── Embedding Engine (1 goroutine per batch)
│   ├─ Load model once (cached)
│   ├─ Process chunks sequentially
│   └─ Store embeddings
│
└── Database (connection pool)
    ├─ SQLite connections (pooled)
    ├─ Concurrent reads (allowed)
    ├─ Serialized writes (WAL mode)
    └─ PRAGMA busy_timeout
```

### Concurrency Patterns

**Read-Heavy Operations (Search):**
- Concurrent reads allowed
- Multiple goroutines can search simultaneously
- SQLite handles locking

**Write Operations (Upload):**
- Sequential writes
- WAL mode ensures isolation
- Busy timeout prevents deadlocks

**Embedding Generation:**
- Sequential (CPU-bound)
- Model loaded once, reused
- Caching prevents reloading

---

## Deployment Architecture

### Standalone Binary

```
User Machine
├── ./small-rag (30MB binary)
├── ~/.small-rag/ (data directory)
│   ├── small-rag.db
│   ├── models/
│   ├── config.json
│   └── logs/
└── Port :8765
```

### With AX

```
User Machine
├── ./ax (autonomous agent)
├── ./small-rag (RAG server)
└── Communication: HTTP localhost:8765
```

### Docker

```
Docker Container
├── Base: Alpine Linux
├── Binary: small-rag
├── Models: Pre-loaded in image
├── Volume: ~/.small-rag/ (host mount)
└── Port: 8765 (exposed)
```

### Cloud Deployment

```
AWS/GCP/Azure
├── VM or Container
├── small-rag binary
├── Persistent volume (for data)
├── Load balancer (optional)
└── API gateway (optional)
```

---

## Error Handling

### Error Categories

1. **Request Validation**
   - Missing required fields
   - Invalid data types
   - Out-of-range values

2. **Resource Not Found**
   - Document not found
   - Chunk not found
   - Model not found

3. **Processing Errors**
   - File parsing failed
   - Embedding generation failed
   - LLM API error

4. **System Errors**
   - Database error
   - Disk space error
   - Memory error

### Error Response

```json
{
  "success": false,
  "error": "Document not found",
  "code": 404,
  "details": {
    "doc_id": "doc-123",
    "timestamp": "2026-07-14T20:30:00Z"
  }
}
```

---

## Performance Characteristics

### Latency

| Operation | Time | Notes |
|-----------|------|-------|
| Startup | 100-200ms | Load model |
| Embed chunk (512 tokens) | 150-300ms | CPU inference |
| Search 1M embeddings | 50-100ms | SQLite + vector |
| RAG query (first token) | 2-5s | Wait for LLM |

### Memory Usage

| Component | RAM |
|-----------|-----|
| Embedding model | 1.2 GB |
| SQLite (100K embeddings) | 200 MB |
| Server + buffers | 300 MB |
| **Total** | **1.7 GB** |

### Storage

| Component | Size |
|-----------|------|
| Binary | 30 MB |
| Model | 379 MB |
| SQLite (100K chunks) | 500 MB |
| **Total** | **~1 GB** |

---

## Security Considerations

### Data Protection

- **At Rest:** SQLite (optional encryption)
- **In Transit:** HTTPS (optional)
- **In Memory:** Cleared on shutdown

### API Security

- **Authentication:** Optional Bearer token
- **Authorization:** Basic (all endpoints or none)
- **Rate Limiting:** Optional (per IP)

### Privacy

- **No External Calls:** Embeddings local
- **No Telemetry:** No tracking
- **No Cloud:** Offline-capable

---

## Scalability

### Horizontal Scaling

- Multiple instances behind load balancer
- Shared data directory (NFS/S3)
- Stateless servers

### Vertical Scaling

- Larger embeddings cache
- More CPU threads
- Larger RAM for model caching

### Database Scaling

- SQLite limits: ~100GB per database
- For larger: Migrate to PostgreSQL + pgvector

---

## Future Enhancements

1. **Advanced Search**
   - Reranking (cross-encoder)
   - Query expansion
   - Semantic clustering

2. **Multi-Model Support**
   - Multiple embedding models
   - Model switching per query
   - Fine-tuned models

3. **Caching**
   - Query result caching
   - Embedding caching
   - LLM response caching

4. **Monitoring**
   - Metrics (Prometheus)
   - Tracing (OpenTelemetry)
   - Logging (structured)

5. **Administration**
   - Web UI for management
   - Bulk operations
   - Analytics dashboard
