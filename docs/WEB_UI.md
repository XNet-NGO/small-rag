# Small-RAG Web UI

## Overview

Small-RAG includes a built-in web interface accessible at `http://localhost:8765`

## Features

### 1. Documents Tab
- **Upload Documents** - Upload PDF, TXT, or Markdown files
  - Auto-extracts text
  - Creates chunks automatically
  - Generates embeddings
  - Shows progress

- **Document List** - View all indexed documents
  - Shows document title
  - Displays chunk count
  - Delete individual documents

### 2. Search Tab
- **Hybrid Search** - Search across indexed documents
  - Semantic search (vector similarity)
  - Keyword search (full-text)
  - Hybrid ranking (combined)
  
- **Search Results** - Display matching chunks
  - Show relevance score (0-100%)
  - Display chunk text
  - Indicate source document

### 3. Settings Tab
- **Configuration** - View current settings
  - Embedding model
  - Chunk size
  - Chunk overlap
  - Default LLM model
  
- **About** - System information
  - Version
  - API endpoint
  - Documentation links

### 4. Status Bar
- **Connection Status** - Real-time server status
  - Connected/Disconnected indicator
  - Document count
  - Embedding count

## Usage

### Start Server
```bash
./small-rag
# Server ready on :8765
```

### Access Web UI
Open browser: `http://localhost:8765`

### Upload Document
1. Click "Documents" tab
2. Click "Choose File" button
3. Select PDF, TXT, or MD file
4. Enter optional title
5. Click "Upload"
6. Wait for processing

### Search
1. Click "Search" tab
2. Enter search query
3. Adjust search type (Hybrid/Semantic/Keyword)
4. Click "Search"
5. View results with scores

### View Settings
1. Click "Settings" tab
2. See current configuration
3. View system information

## Architecture

### Frontend
- Single HTML file (embedded in binary)
- Vanilla JavaScript (no build step)
- Dark theme
- Responsive design

### Backend Integration
- REST API calls to `http://localhost:8765/api/v1`
- JSON request/response
- Server-Sent Events (SSE) for streaming
- CORS enabled for cross-origin requests

## API Endpoints Used

| Endpoint | Method | Purpose |
|----------|--------|---------|
| /health | GET | Check server status |
| /documents | GET | List all documents |
| /documents | POST | Upload new document |
| /documents/{id} | GET | Get document details |
| /documents/{id} | DELETE | Delete document |
| /search | POST | Search documents |
| /config | GET | Get configuration |

## Styling

### Color Scheme (Dark Theme)
- Background: #1a1a1a
- Surface: #2d2d2d
- Border: #404040
- Text: #e0e0e0
- Accent: #4a9eff (blue)
- Success: #4ade80 (green)
- Error: #f87171 (red)

### Responsive Design
- Desktop: Full layout
- Tablet: Adjusted spacing
- Mobile: Stacked layout

## Browser Support
- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)
- Mobile browsers

## Performance
- Single HTML file (~50KB)
- No external dependencies
- Fast load time
- Efficient DOM updates
- Minimal network requests

## Troubleshooting

### Can't connect to server
- Ensure server is running: `./small-rag`
- Check port 8765 is available
- Check firewall settings

### Upload fails
- Check file format (PDF, TXT, MD)
- Check file size (reasonable limit)
- Check disk space
- Check logs for errors

### Search returns no results
- Try different search query
- Ensure documents are uploaded
- Check search type selection
- Lower minimum score threshold

### Page not loading
- Clear browser cache
- Try different browser
- Check browser console for errors
- Check server logs

## Future Enhancements

- [ ] Dark/Light theme toggle
- [ ] Export results (CSV, JSON)
- [ ] Search history
- [ ] Saved queries
- [ ] Analytics dashboard
- [ ] Bulk upload
- [ ] Advanced filters
- [ ] API key management
- [ ] User authentication
- [ ] Multi-language support

## Development

### Modifying UI
The web UI is embedded in the binary. To modify:

1. Edit `web/index.html` (if standalone file)
2. Or update `htmlUI` constant in `internal/api/server.go`
3. Rebuild: `go build -o small-rag ./cmd/small-rag`

### Standalone Mode
For development, serve HTML from file:
```bash
# In development, serve from web/index.html
# In production, embed in binary
```

### Testing
Test all features:
1. Upload document
2. View in document list
3. Search for content
4. Delete document
5. Check settings

## API Integration

All web UI requests use the REST API:

```javascript
// Example: Upload document
const formData = new FormData();
formData.append('file', file);
formData.append('title', title);

fetch('http://localhost:8765/api/v1/documents', {
  method: 'POST',
  body: formData
})
.then(r => r.json())
.then(data => console.log(data));
```

## Keyboard Shortcuts

- `Enter` in search field: Perform search
- `Ctrl+Enter` in RAG query: Submit question (future)

## Accessibility

- Semantic HTML5
- ARIA labels
- Keyboard navigation
- Color contrast (WCAG AA)
- Focus indicators

## Security Notes

- No sensitive data in localStorage
- API key (if used) not stored in browser
- CORS enabled for localhost only (production)
- No authentication required (local use)

---

**Status:** ✅ Web UI Complete  
**Location:** Embedded in binary + `/web/index.html`  
**Access:** http://localhost:8765  
**Features:** Document management, search, settings
