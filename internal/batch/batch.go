package batch

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status represents the current state of a batch job
type Status string

const (
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// DocumentInput represents a document to be indexed in a batch
type DocumentInput struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

// DocumentResult represents the result of indexing a single document
type DocumentResult struct {
	DocID  string `json:"doc_id"`
	Title  string `json:"title"`
	Chunks int    `json:"chunks"`
	Error  string `json:"error,omitempty"`
}

// BatchJob tracks the state of a batch indexing operation
type BatchJob struct {
	ID        string           `json:"batch_id"`
	Status    Status           `json:"status"`
	Total     int              `json:"total"`
	Completed int              `json:"completed"`
	Failed    int              `json:"failed"`
	Results   []DocumentResult `json:"results,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

// Manager manages batch indexing jobs
type Manager struct {
	jobs map[string]*BatchJob
	mu   sync.RWMutex
}

// NewManager creates a new batch Manager
func NewManager() *Manager {
	return &Manager{
		jobs: make(map[string]*BatchJob),
	}
}

// CreateBatch creates a new batch job and returns it
func (m *Manager) CreateBatch(docs []DocumentInput) *BatchJob {
	job := &BatchJob{
		ID:        uuid.New().String(),
		Status:    StatusProcessing,
		Total:     len(docs),
		Completed: 0,
		Failed:    0,
		Results:   []DocumentResult{},
		CreatedAt: time.Now(),
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	return job
}

// GetBatch returns the current status of a batch job
func (m *Manager) GetBatch(id string) (*BatchJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("batch job not found: %s", id)
	}
	return job, nil
}

// ProcessBatch processes each document in the batch using the provided callback.
// This is intended to be run as a goroutine.
func (m *Manager) ProcessBatch(job *BatchJob, docs []DocumentInput, processFunc func(path, title string) (string, int, error)) {
	for _, doc := range docs {
		docID, chunks, err := processFunc(doc.Path, doc.Title)

		m.mu.Lock()
		if err != nil {
			job.Failed++
			job.Results = append(job.Results, DocumentResult{
				Title: doc.Title,
				Error: err.Error(),
			})
		} else {
			job.Completed++
			job.Results = append(job.Results, DocumentResult{
				DocID:  docID,
				Title:  doc.Title,
				Chunks: chunks,
			})
		}
		m.mu.Unlock()
	}

	m.mu.Lock()
	if job.Failed == job.Total {
		job.Status = StatusFailed
	} else {
		job.Status = StatusCompleted
	}
	m.mu.Unlock()
}
