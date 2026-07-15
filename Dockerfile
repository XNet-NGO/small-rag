# Multi-stage build
# Stage 1: Build the Go binary
FROM golang:1.26-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /small-rag ./cmd/small-rag

# Stage 2: Runtime
# Use debian slim for glibc compatibility with llama.cpp shared libs
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /small-rag /app/small-rag

# Copy web UI
COPY web/ /app/web/

# Create directories
RUN mkdir -p /app/lib /app/models /app/.small-rag-db

# Environment
ENV YZMA_LIB=/app/lib
ENV SMALL_RAG_LLM_URL=http://host.docker.internal:11434/v1

EXPOSE 8765

# Entry point
ENTRYPOINT ["/app/small-rag"]
CMD ["-port", "8765"]
