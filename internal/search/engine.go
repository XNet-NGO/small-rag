package search

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// Result represents a search result
type Result struct {
	ChunkID    string                 `json:"chunk_id"`
	DocID      string                 `json:"doc_id"`
	Text       string                 `json:"text"`
	Score      float32                `json:"score"`
	SearchType string                 `json:"search_type"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Engine handles search operations
type Engine struct {
	db *sql.DB
}

// NewEngine creates a new search engine
func NewEngine(db *sql.DB) *Engine {
	return &Engine{
		db: db,
	}
}

// Search performs hybrid search (semantic + keyword)
func (e *Engine) Search(query string, queryEmbedding []float32, topK int, searchType string, minScore float32) ([]Result, error) {
	switch searchType {
	case "semantic":
		return e.semanticSearch(query, queryEmbedding, topK, minScore)
	case "keyword":
		return e.keywordSearch(query, topK, minScore)
	case "hybrid":
		return e.hybridSearch(query, queryEmbedding, topK, minScore)
	default:
		return nil, fmt.Errorf("unknown search type: %s", searchType)
	}
}

// semanticSearch performs vector similarity search
func (e *Engine) semanticSearch(query string, queryEmbedding []float32, topK int, minScore float32) ([]Result, error) {
	// Retrieve all embeddings from database
	rows, err := e.db.Query(`
		SELECT e.id, e.chunk_id, e.embedding, c.text, c.doc_id
		FROM embeddings e
		JOIN chunks c ON e.chunk_id = c.id
		ORDER BY e.id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer rows.Close()

	var results []Result

	for rows.Next() {
		var id, chunkID, embData, text, docID string
		if err := rows.Scan(&id, &chunkID, &embData, &text, &docID); err != nil {
			continue
		}

		// Decode embedding from BLOB
		embedding := DecodeEmbedding([]byte(embData))

		// Calculate similarity
		similarity := CosineSimilarity(queryEmbedding, embedding)

		if similarity >= minScore {
			results = append(results, Result{
				ChunkID:    chunkID,
				DocID:      docID,
				Text:       text,
				Score:      similarity,
				SearchType: "semantic",
			})
		}
	}

	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// keywordSearch performs full-text search
func (e *Engine) keywordSearch(query string, topK int, minScore float32) ([]Result, error) {
	// Split query into terms
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil, fmt.Errorf("empty query")
	}

	// Build FTS query
	var ftsQuery strings.Builder
	for i, term := range terms {
		if i > 0 {
			ftsQuery.WriteString(" AND ")
		}
		ftsQuery.WriteString(term)
	}

	// Search using FTS5
	rows, err := e.db.Query(`
		SELECT c.id, c.doc_id, c.text
		FROM chunks_fts
		JOIN chunks c ON chunks_fts.chunk_id = c.id
		WHERE chunks_fts MATCH ?
		LIMIT ?
	`, ftsQuery.String(), topK)
	if err != nil {
		return nil, fmt.Errorf("failed to query FTS: %w", err)
	}
	defer rows.Close()

	var results []Result

	for rows.Next() {
		var chunkID, docID, text string
		if err := rows.Scan(&chunkID, &docID, &text); err != nil {
			continue
		}

		// Calculate keyword match score based on term frequency
		score := float32(0.5)
		for _, term := range terms {
			if strings.Contains(strings.ToLower(text), term) {
				score += 0.1
			}
		}
		if score > 1.0 {
			score = 1.0
		}

		if score >= minScore {
			results = append(results, Result{
				ChunkID:    chunkID,
				DocID:      docID,
				Text:       text,
				Score:      score,
				SearchType: "keyword",
			})
		}
	}

	return results, nil
}

// hybridSearch combines semantic and keyword search
func (e *Engine) hybridSearch(query string, queryEmbedding []float32, topK int, minScore float32) ([]Result, error) {
	// Get semantic results
	semanticResults, err := e.semanticSearch(query, queryEmbedding, topK*2, 0)
	if err != nil {
		semanticResults = []Result{}
	}

	// Get keyword results
	keywordResults, err := e.keywordSearch(query, topK*2, 0)
	if err != nil {
		keywordResults = []Result{}
	}

	// Merge results
	resultMap := make(map[string]Result)

	// Add semantic results
	for _, r := range semanticResults {
		r.Score = r.Score * 0.7 // Weight semantic 70%
		resultMap[r.ChunkID] = r
	}

	// Add keyword results (combine scores)
	for _, r := range keywordResults {
		r.Score = r.Score * 0.3 // Weight keyword 30%
		if existing, ok := resultMap[r.ChunkID]; ok {
			// Combine scores
			r.Score = existing.Score + r.Score
			r.SearchType = "hybrid"
		}
		resultMap[r.ChunkID] = r
	}

	// Convert map to slice
	var results []Result
	for _, r := range resultMap {
		if r.Score >= minScore {
			results = append(results, r)
		}
	}

	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}
