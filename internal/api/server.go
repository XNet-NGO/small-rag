package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xnet-admin-1/small-rag/internal/config"
	"github.com/xnet-admin-1/small-rag/internal/embedding"
	"github.com/xnet-admin-1/small-rag/internal/search"
)

// Server represents the HTTP API server
type Server struct {
	db           *sql.DB
	cfg          *config.Config
	router       chi.Router
	embedding    *embedding.Engine
	searchEngine *search.Engine
}

// NewServer creates a new API server
func NewServer(db *sql.DB, cfg *config.Config) *Server {
	// Get model path from config, fallback to legacy location
	modelPath := cfg.ModelPath
	if modelPath == "" {
		homeDir, _ := os.UserHomeDir()
		modelPath = filepath.Join(homeDir, ".small-rag/models/qwen3-embedding-0.6b-q4_k_m.gguf")
	}

	// Initialize embedding engine
	embeddingEngine := embedding.NewEngine(modelPath, cfg.EmbeddingDims)

	s := &Server{
		db:           db,
		cfg:          cfg,
		embedding:    embeddingEngine,
		searchEngine: search.NewEngine(db),
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
		r.Post("/tools/search_and_rag", s.handleSearchAndRAG)
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
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, "data: {\"type\":\"context\",\"chunks\":3}\n\n")
	fmt.Fprintf(w, "data: {\"type\":\"delta\",\"text\":\"Sample response\"}\n\n")
	fmt.Fprintf(w, "data: {\"type\":\"done\",\"total_tokens\":100}\n\n")
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": s.cfg})
}

func (s *Server) handleSearchAndRAG(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": map[string]interface{}{"answer": "Not yet implemented"}})
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
<div class="form-group"><label>Title</label><input type="text" id="title"></div>
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
<div class="form-group"><label>Model</label><select id="model"><option>gpt-4</option><option>claude-3-opus</option><option>gpt-3.5-turbo</option></select></div>
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
document.getElementById('clearBtn').addEventListener('click',()=>{document.getElementById('file').value='';document.getElementById('title').value=''});
document.getElementById('searchBtn').addEventListener('click',search);
document.getElementById('clearSearchBtn').addEventListener('click',()=>{document.getElementById('query').value='';document.getElementById('results').innerHTML=''});
document.getElementById('ragBtn').addEventListener('click',rag);
document.getElementById('clearRagBtn').addEventListener('click',()=>{document.getElementById('ragQuery').value='';document.getElementById('ragResponse').textContent='Ask a question to get started...'});
document.getElementById('query').addEventListener('keypress',e=>{if(e.key==='Enter')search()});
document.getElementById('ragQuery').addEventListener('keypress',e=>{if(e.ctrlKey&&e.key==='Enter')rag()});
check();setInterval(check,5000);loadDocs();loadConfig()
});
function switchTab(e){document.querySelectorAll('.tab').forEach(b=>b.classList.remove('active'));e.target.classList.add('active');document.querySelectorAll('.content').forEach(c=>c.classList.remove('active'));document.getElementById(e.target.dataset.tab).classList.add('active')}
async function check(){try{const r=await fetch(API+'/health');const d=await r.json();document.getElementById('ind').classList.remove('off');document.getElementById('stat').textContent='Connected';document.getElementById('info').textContent='Docs: '+d.data.documents_count+' | Embeddings: '+d.data.embeddings_count}catch(e){document.getElementById('ind').classList.add('off');document.getElementById('stat').textContent='Disconnected'}}
function msg(text,type){const m=document.createElement('div');m.className='msg '+type;m.textContent=text;document.getElementById('msgs').appendChild(m);setTimeout(()=>m.remove(),5000)}
async function upload(){const f=document.getElementById('file').files[0];if(!f){msg('Select a file','err');return}const fd=new FormData();fd.append('file',f);fd.append('title',document.getElementById('title').value||f.name);try{document.getElementById('uploadBtn').disabled=true;const r=await fetch(API+'/documents',{method:'POST',body:fd});const d=await r.json();if(d.success){msg('Uploaded: '+d.data.chunks_created+' chunks','ok');document.getElementById('file').value='';document.getElementById('title').value='';loadDocs()}else msg(d.error,'err')}catch(e){msg('Upload failed: '+e.message,'err')}finally{document.getElementById('uploadBtn').disabled=false}}
async function loadDocs(){try{const r=await fetch(API+'/documents');const d=await r.json();const l=document.getElementById('docList');l.innerHTML='';if(!d.data.documents||d.data.documents.length===0){l.innerHTML='<p class="muted">No documents</p>';document.getElementById('count').textContent='0';return}document.getElementById('count').textContent=d.data.documents.length;d.data.documents.forEach(doc=>{const item=document.createElement('div');item.className='item';item.innerHTML='<div><strong>'+doc.title+'</strong><br><small>'+doc.chunks_count+' chunks</small></div><button class="danger" onclick="del('+JSON.stringify(doc.id)+')">Delete</button>';l.appendChild(item)})}catch(e){msg('Failed to load: '+e.message,'err')}}
async function del(id){if(!confirm('Delete?'))return;try{await fetch(API+'/documents/'+id,{method:'DELETE'});msg('Deleted','ok');loadDocs()}catch(e){msg('Delete failed: '+e.message,'err')}}
async function search(){const q=document.getElementById('query').value;if(!q){msg('Enter query','err');return}try{document.getElementById('searchBtn').disabled=true;const r=await fetch(API+'/search',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({query:q,top_k:5,search_type:'hybrid'})});const d=await r.json();const c=document.getElementById('results');c.innerHTML='';if(!d.data.results||d.data.results.length===0){c.innerHTML='<p class="muted">No results</p>';return}d.data.results.forEach(res=>{const item=document.createElement('div');item.className='result';item.innerHTML='<div class="score">'+(res.score*100).toFixed(0)+'%</div><div>'+res.text+'</div>';c.appendChild(item)})}catch(e){msg('Search failed: '+e.message,'err')}finally{document.getElementById('searchBtn').disabled=false}}
async function rag(){var q=document.getElementById('ragQuery').value;if(!q){msg('Enter question','err');return}try{document.getElementById('ragBtn').disabled=true;document.getElementById('ragResponse').textContent='Loading...';var r=await fetch(API+'/rag/query',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({query:q,model:document.getElementById('model').value,stream:true})});var reader=r.body.getReader();var decoder=new TextDecoder();var resp='';var reading=true;while(reading){var chunk=await reader.read();if(chunk.done){reading=false;break}var text=decoder.decode(chunk.value);var lines=text.split('\n');for(var i=0;i<lines.length;i++){var line=lines[i];if(line.startsWith('data: ')){try{var data=JSON.parse(line.substring(6));if(data.type==='delta'){resp+=data.text;document.getElementById('ragResponse').textContent=resp}}catch(pe){}}}}}catch(e){msg('RAG failed: '+e.message,'err')}finally{document.getElementById('ragBtn').disabled=false}}
async function loadConfig(){try{const r=await fetch(API+'/config');const d=await r.json();document.getElementById('config').innerHTML='Model: '+d.data.embedding_model+'<br>Chunk Size: '+d.data.chunk_size+'<br>Overlap: '+d.data.chunk_overlap}catch(e){}}
</script>
</body>
</html>`
