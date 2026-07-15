# Small-RAG API Documentation

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Response Format](#response-format)
4. [Error Handling](#error-handling)
5. [Endpoints](#endpoints)
6. [Examples](#examples)
7. [Streaming](#streaming)
8. [Agent Integration](#agent-integration)

---

## Overview

Small-RAG provides a RESTful API for document management, searching, and RAG queries.

### Base URL

```
http://localhost:8765/api/v1
```

### Supported Content Types

- **Request:** `application/json`, `multipart/form-data`
- **Response:** `application/json`, `text/event-stream`

### API Versions

- Current: `v1`
- URL: `/api/v1/*`

---

## Authentication

### Optional API Key

If enabled, include Bearer token in header:

```http
Authorization: Bearer sk-small-rag-abc123def456
```

### Default

No authentication required (for local use).

---

## Response Format

### Success Response

```json
{
  "success": true,
  "data": {
    "id": "doc-123",
    "title": "Document",
    "chunks": 12
  },
  "metadata": {
    "timestamp": "2026-07-14T20:30:00Z",
    "duration_ms": 45
  }
}
```

### Error Response

```json
{
  "success": false,
  "error": "Error message",
  "code": 400,
  "details": {
    "field": "query",
    "reason": "required"
  }
}
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | OK - Request successful |
| 201 | Created - Resource created |
| 204 | No Content - Successful deletion |
| 400 | Bad Request - Invalid parameters |
| 404 | Not Found - Resource not found |
| 409 | Conflict - Duplicate resource |
| 500 | Server Error - Internal error |

---

## Error Handling

### Error Response Structure

```json
{
  "success": false,
  "error": "descriptive error message",
  "code": 400,
  "details": {
    "field": "parameter_name",
    "reason": "validation_reason"
  }
}
```

### Common Errors

| Code | Error | Reason |
|------|-------|--------|
| 400 | Invalid request | Malformed JSON or missing fields |
| 404 | Not found | Document/chunk/config not found |
| 409 | Conflict | Duplicate document (same content hash) |
| 500 | Internal error | Server error (see logs) |

---

## Endpoints

### 1. Health Check

**Endpoint:** `GET /health`

**Description:** Check server health and statistics

**Response:**

```json
{
  "status": "ready",
  "version": "0.1.0",
  "embeddings_count": 1250,
  "documents_count": 45,
  "uptime_seconds": 3600
}
```

**Status Codes:** 200

---

### 2. Upload Document

**Endpoint:** `POST /documents`

**Description:** Upload and index a document

**Content-Type:** `multipart/form-data`

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| file | file | Yes | Document file (PDF, TXT, MD) |
| title | string | No | Document title |
| source | string | No | Source URL or reference |

**Request Example:**

```bash
curl -X POST http://localhost:8765/api/v1/documents \
  -F "file=@document.pdf" \
  -F "title=Machine Learning Guide" \
  -F "source=https://example.com/ml-guide.pdf"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "doc-abc123",
    "title": "Machine Learning Guide",
    "source": "https://example.com/ml-guide.pdf",
    "chunks_created": 12,
    "embeddings_created": 12,
    "status": "indexed",
    "created_at": "2026-07-14T20:30:00Z"
  }
}
```

**Status Codes:** 201 (Created), 400 (Bad Request), 409 (Conflict)

---

### 3. List Documents

**Endpoint:** `GET /documents`

**Description:** List all documents with pagination

**Query Parameters:**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| limit | integer | 20 | Number of results (max 100) |
| offset | integer | 0 | Pagination offset |
| search | string | - | Search documents by title |

**Request Example:**

```bash
curl "http://localhost:8765/api/v1/documents?limit=10&offset=0"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "documents": [
      {
        "id": "doc-1",
        "title": "ML Guide",
        "source": "file.pdf",
        "chunks_count": 12,
        "created_at": "2026-07-14T20:30:00Z"
      },
      {
        "id": "doc-2",
        "title": "Python Tutorial",
        "source": "tutorial.md",
        "chunks_count": 8,
        "created_at": "2026-07-14T20:25:00Z"
      }
    ],
    "total": 45,
    "limit": 10,
    "offset": 0
  }
}
```

**Status Codes:** 200 (OK)

---

### 4. Get Document

**Endpoint:** `GET /documents/{doc_id}`

**Description:** Get document details and metadata

**Path Parameters:**

| Name | Type | Description |
|------|------|-------------|
| doc_id | string | Document ID |

**Request Example:**

```bash
curl "http://localhost:8765/api/v1/documents/doc-abc123"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "doc-abc123",
    "title": "Machine Learning Guide",
    "source": "https://example.com/ml-guide.pdf",
    "content_preview": "Machine learning is a subset of...",
    "chunks_count": 12,
    "created_at": "2026-07-14T20:30:00Z",
    "updated_at": "2026-07-14T20:30:00Z"
  }
}
```

**Status Codes:** 200 (OK), 404 (Not Found)

---

### 5. Delete Document

**Endpoint:** `DELETE /documents/{doc_id}`

**Description:** Delete document and its chunks/embeddings

**Path Parameters:**

| Name | Type | Description |
|------|------|-------------|
| doc_id | string | Document ID |

**Request Example:**

```bash
curl -X DELETE "http://localhost:8765/api/v1/documents/doc-abc123"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "deleted": true,
    "id": "doc-abc123"
  }
}
```

**Status Codes:** 204 (No Content), 404 (Not Found)

---

### 6. Search

**Endpoint:** `POST /search`

**Description:** Search documents (hybrid: semantic + keyword)

**Content-Type:** `application/json`

**Request Body:**

```json
{
  "query": "What is machine learning?",
  "top_k": 5,
  "search_type": "hybrid",
  "min_score": 0.3,
  "include_metadata": true
}
```

**Parameters:**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| query | string | - | Search query (required) |
| top_k | integer | 5 | Number of results |
| search_type | string | hybrid | semantic, keyword, or hybrid |
| min_score | float | 0.0 | Minimum score threshold |
| include_metadata | boolean | true | Include document metadata |

**Request Example:**

```bash
curl -X POST http://localhost:8765/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "What is machine learning?",
    "top_k": 5,
    "search_type": "hybrid"
  }'
```

**Response:**

```json
{
  "success": true,
  "data": {
    "query": "What is machine learning?",
    "results": [
      {
        "chunk_id": "chunk-123",
        "doc_id": "doc-abc",
        "text": "Machine learning is a subset of artificial intelligence that focuses on learning from data...",
        "score": 0.87,
        "search_type": "hybrid",
        "metadata": {
          "doc_title": "ML Guide",
          "chunk_index": 3,
          "source": "file.pdf"
        }
      },
      {
        "chunk_id": "chunk-124",
        "doc_id": "doc-abc",
        "text": "Learning algorithms can be supervised, unsupervised, or reinforcement-based...",
        "score": 0.82,
        "search_type": "hybrid",
        "metadata": {
          "doc_title": "ML Guide",
          "chunk_index": 4,
          "source": "file.pdf"
        }
      }
    ],
    "total_results": 2,
    "search_time_ms": 45
  }
}
```

**Status Codes:** 200 (OK), 400 (Bad Request)

---

### 7. RAG Query (Streaming)

**Endpoint:** `POST /rag/query`

**Description:** Search and generate answer using LLM (streaming)

**Content-Type:** `application/json`

**Request Body:**

```json
{
  "query": "Summarize the main points about machine learning",
  "top_k": 3,
  "model": "gpt-4",
  "system_prompt": "You are a helpful assistant",
  "temperature": 0.7,
  "stream": true
}
```

**Parameters:**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| query | string | - | Question or prompt (required) |
| top_k | integer | 3 | Number of search results |
| model | string | gpt-4 | LLM model to use |
| system_prompt | string | default | System prompt for LLM |
| temperature | float | 0.7 | LLM temperature (0-1) |
| stream | boolean | true | Stream response (SSE) |

**Request Example:**

```bash
curl -X POST http://localhost:8765/api/v1/rag/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Summarize machine learning",
    "model": "gpt-4",
    "stream": true
  }'
```

**Response (Streaming):**

```
data: {"type":"context","chunks":3,"search_time_ms":45}
data: {"type":"delta","text":"Machine learning"}
data: {"type":"delta","text":" is a powerful"}
data: {"type":"delta","text":" technology that"}
data: {"type":"delta","text":" enables computers"}
data: {"type":"delta","text":" to learn from data."}
data: {"type":"done","total_tokens":342,"generation_time_ms":2500}
```

**Response (Non-Streaming):**

```json
{
  "success": true,
  "data": {
    "query": "Summarize machine learning",
    "answer": "Machine learning is a powerful technology that enables computers to learn from data...",
    "sources": [
      {
        "chunk_id": "chunk-123",
        "doc_id": "doc-abc",
        "doc_title": "ML Guide",
        "score": 0.87
      }
    ],
    "tokens_used": {
      "prompt": 250,
      "completion": 92,
      "total": 342
    }
  }
}
```

**Status Codes:** 200 (OK), 400 (Bad Request)

---

### 8. Agent Tool: Search and RAG

**Endpoint:** `POST /tools/search_and_rag`

**Description:** Combined search + RAG for agent tool calling

**Content-Type:** `application/json`

**Request Body:**

```json
{
  "query": "Find information about ML deployment and provide best practices",
  "top_k": 5,
  "model": "claude-3-opus",
  "include_sources": true
}
```

**Parameters:**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| query | string | - | Search and RAG query (required) |
| top_k | integer | 5 | Number of search results |
| model | string | gpt-4 | LLM model |
| include_sources | boolean | true | Include source documents |

**Request Example:**

```bash
curl -X POST http://localhost:8765/api/v1/tools/search_and_rag \
  -H "Content-Type: application/json" \
  -d '{
    "query": "ML deployment best practices",
    "model": "claude-3-opus",
    "include_sources": true
  }'
```

**Response:**

```json
{
  "success": true,
  "data": {
    "query": "ML deployment best practices",
    "answer": "Based on the documentation, ML deployment best practices include...",
    "sources": [
      {
        "doc_id": "doc-123",
        "doc_title": "ML Operations Guide",
        "chunks": [0, 1, 2],
        "relevance_score": 0.92
      }
    ],
    "tokens_used": 450,
    "generation_time_ms": 2800
  }
}
```

**Status Codes:** 200 (OK), 400 (Bad Request)

---

### 9. Get Configuration

**Endpoint:** `GET /config`

**Description:** Get current configuration

**Request Example:**

```bash
curl "http://localhost:8765/api/v1/config"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "embedding_model": "qwen3-embedding-0.6b",
    "embedding_dims": 384,
    "chunk_size": 512,
    "chunk_overlap": 128,
    "search_types": ["semantic", "keyword", "hybrid"],
    "min_score": 0.3,
    "default_llm_provider": "openai",
    "default_model": "gpt-4",
    "port": 8765,
    "enable_cache": true,
    "enable_sse": true
  }
}
```

**Status Codes:** 200 (OK)

---

## Examples

### Example 1: Upload and Search

```bash
# 1. Upload document
curl -X POST http://localhost:8765/api/v1/documents \
  -F "file=@ml-guide.pdf" \
  -F "title=Machine Learning Guide"

# Response: {"id": "doc-123", "chunks_created": 12}

# 2. Search
curl -X POST http://localhost:8765/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "What is supervised learning?",
    "top_k": 3
  }'

# Response: [{"text": "...", "score": 0.92}, ...]
```

### Example 2: RAG Query with Streaming

```bash
# RAG query with streaming
curl -X POST http://localhost:8765/api/v1/rag/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Explain the difference between supervised and unsupervised learning",
    "model": "gpt-4",
    "stream": true
  }' \
  | grep "data:" | sed 's/data: //' | jq .
```

### Example 3: Agent Tool Integration (AX)

```bash
# AX calls the agent tool
curl -X POST http://localhost:8765/api/v1/tools/search_and_rag \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How do we deploy ML models to production?",
    "model": "claude-3-opus",
    "include_sources": true
  }'
```

---

## Streaming

### Server-Sent Events (SSE)

RAG queries support streaming responses via SSE.

**Enable Streaming:**

```json
{
  "stream": true
}
```

**Event Types:**

| Type | Description | Example |
|------|-------------|---------|
| context | Search context | `{"type":"context","chunks":3}` |
| delta | Text delta | `{"type":"delta","text":"Hello"}` |
| done | Query complete | `{"type":"done","total_tokens":342}` |
| error | Error occurred | `{"type":"error","message":"..."}` |

**Event Format:**

```
data: {"type":"delta","text":"response text"}

```

**Client Implementation (JavaScript):**

```javascript
const eventSource = new EventSource(
  'http://localhost:8765/api/v1/rag/query',
  {
    method: 'POST',
    body: JSON.stringify({query: "..."}),
    headers: {'Content-Type': 'application/json'}
  }
);

eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'delta') {
    console.log(data.text);
  } else if (data.type === 'done') {
    console.log('Done:', data.total_tokens);
    eventSource.close();
  }
};
```

---

## Agent Integration

### AX Tool Definition

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

### AX Integration Example

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
    
    if err != nil {
        return fmt.Sprintf("Error: %v", err)
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    // Format results for LLM
    return formatSearchResults(result)
```

### Workflow Example

```
AX: "Find deployment best practices and create a deployment guide"
  ↓
AX calls: rag_search("ML deployment best practices")
  ↓
Small-RAG returns:
  {
    "results": [
      {"text": "Use containerization...", "score": 0.92},
      {"text": "Monitor model performance...", "score": 0.87},
      {"text": "Set up CI/CD pipelines...", "score": 0.85}
    ]
  }
  ↓
AX includes in LLM context:
  "Based on internal documentation:
   - Use containerization...
   - Monitor model performance...
   - Set up CI/CD pipelines..."
  ↓
LLM generates deployment guide
  ↓
AX spawns coder agent to write scripts
```

---

## Rate Limiting

Currently no rate limiting. Future versions will support:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1626355200
```

---

## Versioning

API follows semantic versioning:

- `v1` - Current stable version
- Future: `v2`, `v3`, etc.

Breaking changes will increment major version.

---

## Support

For issues or questions:
- Check `/docs/ARCHITECTURE.md` for system design
- Review `/README.md` for quick start
- Check logs in `~/.small-rag/logs/`
