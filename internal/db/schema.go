package db

import (
	"database/sql"
	"fmt"
)

// InitSchema initializes the database schema
func InitSchema(db *sql.DB) error {
	schema := `
-- Documents
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    source TEXT,
    content TEXT,
    content_hash TEXT UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Chunks (document fragments)
CREATE TABLE IF NOT EXISTS chunks (
    id TEXT PRIMARY KEY,
    doc_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    text TEXT NOT NULL,
    tokens INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(doc_id, chunk_index)
);

-- Vector embeddings (stored as BLOB of float32)
CREATE TABLE IF NOT EXISTS embeddings (
    id TEXT PRIMARY KEY,
    chunk_id TEXT UNIQUE NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    embedding BLOB NOT NULL,
    model_id TEXT NOT NULL,
    dims INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Full-text search index
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    chunk_id UNINDEXED,
    text
);

-- Settings (key-value store)
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Search index for performance
CREATE INDEX IF NOT EXISTS idx_chunks_doc_id ON chunks(doc_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_chunk_id ON embeddings(chunk_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_model ON embeddings(model_id);
CREATE INDEX IF NOT EXISTS idx_documents_created ON documents(created_at);

-- Triggers to update FTS index
CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
  INSERT INTO chunks_fts(chunk_id, text) VALUES (new.id, new.text);
END;

CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
  DELETE FROM chunks_fts WHERE chunk_id = old.id;
END;

CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
  UPDATE chunks_fts SET text = new.text WHERE chunk_id = old.id;
END;
`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// Document represents a document in the database
type Document struct {
	ID           string
	Title        string
	Source       string
	Content      string
	ContentHash  string
	CreatedAt    string
	UpdatedAt    string
	ChunksCount  int
}

// SaveDocument inserts a document
func SaveDocument(db *sql.DB, doc *Document) error {
	_, err := db.Exec(
		`INSERT INTO documents (id, title, source, content, content_hash)
		 VALUES (?, ?, ?, ?, ?)`,
		doc.ID, doc.Title, doc.Source, doc.Content, doc.ContentHash,
	)
	return err
}

// GetDocument retrieves a document by ID
func GetDocument(db *sql.DB, id string) (*Document, error) {
	var doc Document
	err := db.QueryRow(
		`SELECT id, title, source, content, content_hash, created_at, updated_at
		 FROM documents WHERE id = ?`,
		id,
	).Scan(&doc.ID, &doc.Title, &doc.Source, &doc.Content, &doc.ContentHash,
		&doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		return nil, err
	}

	// Count chunks
	db.QueryRow(`SELECT COUNT(*) FROM chunks WHERE doc_id = ?`, id).Scan(&doc.ChunksCount)

	return &doc, nil
}

// Chunk represents a document chunk
type Chunk struct {
	ID         string
	DocID      string
	ChunkIndex int
	Text       string
	Tokens     int
	CreatedAt  string
}

// SaveChunk inserts a chunk
func SaveChunk(db *sql.DB, chunk *Chunk) error {
	_, err := db.Exec(
		`INSERT INTO chunks (id, doc_id, chunk_index, text, tokens)
		 VALUES (?, ?, ?, ?, ?)`,
		chunk.ID, chunk.DocID, chunk.ChunkIndex, chunk.Text, chunk.Tokens,
	)
	return err
}

// Embedding represents a vector embedding
type Embedding struct {
	ID        string
	ChunkID   string
	Embedding []byte // Binary float32 array
	ModelID   string
	Dims      int
	CreatedAt string
}

// SaveEmbedding inserts an embedding
func SaveEmbedding(db *sql.DB, emb *Embedding) error {
	_, err := db.Exec(
		`INSERT INTO embeddings (id, chunk_id, embedding, model_id, dims)
		 VALUES (?, ?, ?, ?, ?)`,
		emb.ID, emb.ChunkID, emb.Embedding, emb.ModelID, emb.Dims,
	)
	return err
}

// GetChunksByDocID retrieves all chunks for a document
func GetChunksByDocID(db *sql.DB, docID string) ([]*Chunk, error) {
	rows, err := db.Query(
		`SELECT id, doc_id, chunk_index, text, tokens, created_at
		 FROM chunks WHERE doc_id = ? ORDER BY chunk_index`,
		docID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		var chunk Chunk
		if err := rows.Scan(&chunk.ID, &chunk.DocID, &chunk.ChunkIndex,
			&chunk.Text, &chunk.Tokens, &chunk.CreatedAt); err != nil {
			return nil, err
		}
		chunks = append(chunks, &chunk)
	}

	return chunks, rows.Err()
}

// GetEmbedding retrieves an embedding by chunk ID
func GetEmbedding(db *sql.DB, chunkID string) (*Embedding, error) {
	var emb Embedding
	err := db.QueryRow(
		`SELECT id, chunk_id, embedding, model_id, dims, created_at
		 FROM embeddings WHERE chunk_id = ?`,
		chunkID,
	).Scan(&emb.ID, &emb.ChunkID, &emb.Embedding, &emb.ModelID, &emb.Dims, &emb.CreatedAt)

	return &emb, err
}

// DeleteDocument deletes a document and its chunks/embeddings
func DeleteDocument(db *sql.DB, docID string) error {
	_, err := db.Exec(`DELETE FROM documents WHERE id = ?`, docID)
	return err
}
