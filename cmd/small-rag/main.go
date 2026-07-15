// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 xnet-admin-1

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/xnet-admin-1/small-rag/internal/api"
	"github.com/xnet-admin-1/small-rag/internal/config"
	"github.com/xnet-admin-1/small-rag/internal/db"
)

var version = "0.1.0-dev"

func main() {
	// Flags
	var (
		dataDir = flag.String("data-dir", "", "Data directory (default: ~/.small-rag)")
		port    = flag.Int("port", 8765, "HTTP server port")
		debug   = flag.Bool("debug", false, "Enable debug logging")
		version = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	// Version
	if *version {
		fmt.Println("small-rag", version)
		os.Exit(0)
	}

	// Resolve data directory
	if *dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get home directory: %v", err)
		}
		*dataDir = filepath.Join(home, ".small-rag")
	}

	// Create data directory
	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	log.Printf("small-rag v%s", version)
	log.Printf("Data directory: %s", *dataDir)

	// Load config
	cfg, err := config.Load(*dataDir)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	database, err := db.Open(filepath.Join(*dataDir, "small-rag.db"))
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Initialize schema
	if err := db.InitSchema(database); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	// Start API server
	server := api.NewServer(database, cfg)
	if err := server.Start(*port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
