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
	"github.com/xnet-admin-1/small-rag/internal/install"
)

var version = "0.1.0-dev"

func main() {
	// Check for subcommands first
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			cmdInstall()
			return
		case "version":
			fmt.Println("small-rag", version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// Default: run server
	cmdServe()
}

func printUsage() {
	fmt.Printf(`small-rag v%s - Self-contained portable RAG system

Usage:
  small-rag              Start the RAG server
  small-rag install      Download model and set up directories
  small-rag version      Show version
  small-rag help         Show this help

Server flags:
  -port int        HTTP server port (default 8765)
  -data-dir string Data directory (default: next to binary)
  -debug           Enable debug logging

Directory structure (after install):
  ./small-rag              Main binary
  ./config.json            Configuration
  ./.small-rag-db/         SQLite database
  ./llama/
      └── models/
          └── %s
`, version, install.ModelName)
}

func cmdInstall() {
	if err := install.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Installation failed: %v\n", err)
		os.Exit(1)
	}
}

func cmdServe() {
	// Flags
	var (
		port  = flag.Int("port", 8765, "HTTP server port")
		debug = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	// Resolve base directory (next to binary)
	baseDir := getBaseDir()

	// Data directory is .small-rag-db/ next to binary
	dataDir := filepath.Join(baseDir, ".small-rag-db")

	// Fallback: if .small-rag-db doesn't exist, try ~/.small-rag (legacy)
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		home, _ := os.UserHomeDir()
		legacyDir := filepath.Join(home, ".small-rag")
		if _, err := os.Stat(legacyDir); err == nil {
			dataDir = legacyDir
		}
	}

	// Create data directory if needed
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	log.Printf("small-rag v%s", version)
	log.Printf("Base directory: %s", baseDir)
	log.Printf("Data directory: %s", dataDir)
	if *debug {
		log.Printf("Debug mode enabled")
	}

	// Load config (check base dir first, then data dir)
	configPath := filepath.Join(baseDir, "config.json")
	var cfg *config.Config
	var err error
	if _, statErr := os.Stat(configPath); statErr == nil {
		cfg, err = config.LoadFromFile(configPath)
	} else {
		cfg, err = config.Load(dataDir)
	}
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set model path relative to binary
	modelPath := filepath.Join(baseDir, "llama", "models", install.ModelName)

	// Also check ~/.small-rag/models/ as fallback
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		home, _ := os.UserHomeDir()
		altPath := filepath.Join(home, ".small-rag", "models", install.ModelName)
		if _, err := os.Stat(altPath); err == nil {
			modelPath = altPath
		}
	}

	cfg.ModelPath = modelPath

	// Initialize database
	database, err := db.Open(filepath.Join(dataDir, "small-rag.db"))
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Initialize schema
	if err := db.InitSchema(database); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	log.Printf("Database initialized")

	// Start API server
	server := api.NewServer(database, cfg)
	log.Printf("Starting server on port %d", *port)
	if err := server.Start(*port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// getBaseDir returns the directory containing the binary
func getBaseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
