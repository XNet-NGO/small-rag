# Small-RAG Web UI Design

## Overview

A lightweight, single-page web application for Small-RAG that provides:
- Document management (upload, list, delete)
- Search interface (hybrid search)
- RAG query with streaming responses
- Configuration management
- Real-time status monitoring

## Technology Stack

**Frontend:**
- HTML5 (semantic markup)
- CSS3 (responsive design, dark theme)
- Vanilla JavaScript (no build step, no dependencies)
- Fetch API (HTTP requests)
- EventSource API (SSE streaming)

**Why:**
- Zero build step
- No npm/node dependencies
- Single HTML file (can be embedded in binary)
- Fast load time
- Works offline with local API

## Design Principles

1. **Minimal** - Single page, essential features only
2. **Fast** - <100ms load, responsive interactions
3. **Local-First** - Works with localhost:8765
4. **Dark Theme** - Easy on eyes, modern look
5. **Responsive** - Works on desktop and tablet
6. **Accessible** - WCAG standards

## UI Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Small-RAG                              Status: Ready ✓     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─ Tabs ────────────────────────────────────────────────┐  │
│  │  [Documents] [Search] [RAG] [Settings]               │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─ Tab Content ──────────────────────────────────────────┐  │
│  │                                                         │  │
│  │  Documents Tab:                                        │  │
│  │  ┌─ Upload ───────────────────────────────────────┐   │  │
│  │  │ [Choose File] [Upload]                         │   │  │
│  │  └─────────────────────────────────────────────────┘   │  │
│  │                                                         │  │
│  │  ┌─ Document List ────────────────────────────────┐   │  │
│  │  │ ML Guide (12 chunks)              [Delete]     │   │  │
│  │  │ Python Tutorial (8 chunks)        [Delete]     │   │  │
│  │  │ ...                                             │   │  │
│  │  └─────────────────────────────────────────────────┘   │  │
│  │                                                         │  │
│  │  Search Tab:                                           │  │
│  │  ┌─ Query ────────────────────────────────────────┐   │  │
│  │  │ [Search input field]      [Search] [Clear]     │   │  │
│  │  │ Type: [Hybrid v]  Min Score: [0.3]            │   │  │
│  │  └─────────────────────────────────────────────────┘   │  │
│  │                                                         │  │
│  │  ┌─ Results ──────────────────────────────────────┐   │  │
│  │  │ Result 1 (0.92)                                │   │  │
│  │  │ Machine learning is a subset of...             │   │  │
│  │  │ [From: ML Guide]                               │   │  │
│  │  │                                                │   │  │
│  │  │ Result 2 (0.87)                                │   │  │
│  │  │ Learning algorithms can be supervised...       │   │  │
│  │  │ [From: ML Guide]                               │   │  │
│  │  └─────────────────────────────────────────────────┘   │  │
│  │                                                         │  │
│  │  RAG Tab:                                              │  │
│  │  ┌─ Query ────────────────────────────────────────┐   │  │
│  │  │ [Question input]                               │   │  │
│  │  │ Model: [gpt-4 v]  Temperature: [0.7]          │   │  │
│  │  │ [Ask] [Clear]                                  │   │  │
│  │  └─────────────────────────────────────────────────┘   │  │
│  │                                                         │  │
│  │  ┌─ Response (Streaming) ─────────────────────────┐   │  │
│  │  │ Machine learning is a powerful technology...   │   │  │
│  │  │ [Tokens: 342] [Time: 2.5s]                     │   │  │
│  │  └─────────────────────────────────────────────────┘   │  │
│  │                                                         │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Component Structure

```
index.html
├── Header
│   ├── Title "Small-RAG"
│   └── Status indicator
├── Tab Navigation
│   ├── Documents tab
│   ├── Search tab
│   ├── RAG tab
│   └── Settings tab
├── Tab Content
│   ├── DocumentsPanel
│   ├── SearchPanel
│   ├── RAGPanel
│   └── SettingsPanel
└── Footer
    └── Version info
```

## Features

### 1. Documents Tab
- Upload documents (PDF, TXT, MD)
- List all documents with chunk counts
- Delete documents
- Progress indicator during upload

### 2. Search Tab
- Hybrid search (semantic, keyword, hybrid)
- Adjustable top-K results
- Minimum score threshold
- Display results with scores
- Show source document

### 3. RAG Tab
- Ask questions
- Select LLM model
- Adjust temperature
- Stream responses (real-time)
- Show token count and timing

### 4. Settings Tab
- View configuration
- Adjust search parameters
- LLM settings
- Cache settings

### 5. Status Bar
- Server status (connected/disconnected)
- Document count
- Embedding count
- Uptime

## API Integration

All requests go to `http://localhost:8765/api/v1/`

**Endpoints Used:**
- `GET /health` - Check status
- `POST /documents` - Upload
- `GET /documents` - List
- `DELETE /documents/{id}` - Delete
- `POST /search` - Search
- `POST /rag/query` - RAG (streaming)
- `GET /config` - Get config

## Streaming Implementation

For RAG responses, use EventSource API:

```javascript
const eventSource = new EventSource(url, {
  method: 'POST',
  body: JSON.stringify(data),
  headers: {'Content-Type': 'application/json'}
});

eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'delta') {
    // Append text to response
    response += data.text;
  } else if (data.type === 'done') {
    // Streaming complete
    eventSource.close();
  }
};
```

## Color Scheme (Dark Theme)

```
Background:     #1a1a1a (dark)
Surface:        #2d2d2d (panels)
Border:         #404040 (subtle)
Text:           #e0e0e0 (light gray)
Accent:         #4a9eff (blue)
Success:        #4ade80 (green)
Error:          #f87171 (red)
Warning:        #fbbf24 (amber)
```

## Responsive Breakpoints

- Desktop: ≥1024px
- Tablet: 768px-1023px
- Mobile: <768px

## Accessibility

- Semantic HTML5
- ARIA labels
- Keyboard navigation
- Color contrast (WCAG AA)
- Focus indicators
- Error messages

## Performance

- Single HTML file (~50KB)
- No external dependencies
- Lazy loading of content
- Efficient DOM updates
- Minimal reflows/repaints

## Browser Support

- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)
- Mobile browsers

## Future Enhancements

1. **Dark/Light Theme Toggle**
2. **Export Results** (CSV, JSON)
3. **Search History**
4. **Saved Queries**
5. **Analytics Dashboard**
6. **Bulk Upload**
7. **Advanced Filters**
8. **API Key Management**
