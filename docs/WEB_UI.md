# Small-RAG Web UI

## Overview

A lightweight, single-page web application for managing and querying the Small-RAG knowledge base.

**Features:**
- 📄 Document management (upload, list, delete)
- 🔍 Hybrid search (semantic, keyword, combined)
- ✨ RAG queries with streaming responses
- ⚙️ Configuration viewer
- 📊 Real-time status monitoring

## Quick Start

### Access the Web UI

```bash
# Start small-rag server
./small-rag

# Open in browser
http://localhost:8765
```

### Tabs

#### 1. Documents
- Upload documents (PDF, TXT, MD)
- View all indexed documents
- See chunk counts
- Delete documents

#### 2. Search
- Search knowledge base
- Choose search type (hybrid, semantic, keyword)
- Adjust result count and minimum score
- View results with relevance scores

#### 3. RAG Query
- Ask questions about your documents
- Select LLM model (GPT-4, Claude, etc.)
- Adjust temperature
- Stream responses in real-time

#### 4. Settings
- View configuration
- See model information
- Check system settings

## Features

### Document Upload

```
1. Click "Documents" tab
2. Select file (PDF, TXT, MD)
3. Add title (optional)
4. Add source (optional)
5. Click "Upload"
```

**Supported Formats:**
- PDF (with text extraction)
- TXT (plain text)
- MD (Markdown)

### Search

```
1. Click "Search" tab
2. Enter query
3. Choose search type:
   - Hybrid: Combine semantic + keyword
   - Semantic: Vector similarity only
   - Keyword: Full-text search only
4. Adjust parameters:
   - Top K: Number of results
   - Min Score: Minimum relevance
5. Click "Search"
```

**Results Show:**
- Relevance score (0-100%)
- Text snippet
- Source document

### RAG Query

```
1. Click "RAG Query" tab
2. Enter question
3. Select LLM model
4. Adjust temperature (0-2)
5. Click "Ask"
6. Watch response stream in real-time
```

**Response Shows:**
- Streaming text
- Token count
- Generation time

## Architecture

### Frontend Stack

- **HTML5** - Semantic markup
- **CSS3** - Responsive design, dark theme
- **Vanilla JavaScript** - No dependencies
- **Fetch API** - HTTP requests
- **EventSource API** - SSE streaming

### API Integration

All requests to `http://localhost:8765/api/v1/`

**Endpoints Used:**
- `GET /health` - Server status
- `POST /documents` - Upload
- `GET /documents` - List
- `DELETE /documents/{id}` - Delete
- `POST /search` - Search
- `POST /rag/query` - RAG (streaming)
- `GET /config` - Configuration

### Styling

**Dark Theme:**
- Background: #1a1a1a
- Surface: #2d2d2d
- Accent: #4a9eff (blue)
- Success: #4ade80 (green)
- Error: #f87171 (red)

### Responsive

- Desktop: ≥1024px
- Tablet: 768px-1023px
- Mobile: <768px

## Browser Support

- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)
- Mobile browsers

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Tab | Switch tabs |
| Enter | Execute search |
| Ctrl+Enter | Execute RAG query |
| Escape | Clear messages |

## Performance

- **Load Time:** <1 second
- **Search:** <100ms
- **RAG Response:** Streaming (2-5s first token)

## Accessibility

- Semantic HTML5
- ARIA labels
- Keyboard navigation
- Color contrast (WCAG AA)
- Focus indicators

## Development

### File Structure

```
web/
└── index.html          Single-page application
```

### Embedding in Binary

To embed the web UI in the Go binary:

```go
//go:embed web/index.html
var webUI embed.FS

func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
    data, _ := webUI.ReadFile("web/index.html")
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Write(data)
}
```

### Customization

Edit `web/index.html` to customize:
- Colors (CSS variables at top)
- Tabs and features
- API endpoints
- Styling

## Troubleshooting

### Web UI Not Loading

1. Check server is running: `curl http://localhost:8765/health`
2. Check browser console for errors (F12)
3. Try hard refresh (Ctrl+Shift+R)

### API Requests Failing

1. Ensure server is running on :8765
2. Check CORS headers in response
3. Verify API endpoint URLs

### Streaming Not Working

1. Check browser supports EventSource API
2. Verify /rag/query endpoint is working
3. Check network tab for SSE events

## Future Enhancements

1. **Dark/Light Theme Toggle**
2. **Export Results** (CSV, JSON, PDF)
3. **Search History**
4. **Saved Queries**
5. **Analytics Dashboard**
6. **Bulk Upload**
7. **Advanced Filters**
8. **API Key Management**
9. **User Authentication**
10. **Multi-language Support**

## API Reference

See `docs/API.md` for complete API documentation.

## Support

For issues or questions:
- Check browser console (F12)
- Review API responses
- Check server logs
