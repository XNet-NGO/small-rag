#!/bin/bash

# small-rag project structure

mkdir -p cmd/small-rag
mkdir -p internal/{api,db,embedding,rag,config}
mkdir -p pkg/{rag,models}
mkdir -p models
mkdir -p docs
mkdir -p tests

# Create initial files
touch cmd/small-rag/main.go
touch internal/api/server.go
touch internal/api/handlers.go
touch internal/db/db.go
touch internal/db/schema.go
touch internal/embedding/embedding.go
touch internal/rag/search.go
touch internal/rag/query.go
touch internal/config/config.go
touch pkg/rag/knowledge_base.go
touch docs/API.md
touch docs/ARCHITECTURE.md
touch README.md
touch .gitignore

echo "✓ Project structure created"
