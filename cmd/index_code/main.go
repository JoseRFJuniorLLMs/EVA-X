// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// index_code indexes all EVA-Mind .go files AND .md docs into Qdrant for semantic search.
// Uses full Go AST parsing (structs with fields, method signatures, interfaces, constants).
// Run: go run cmd/index_code/main.go [basePath]
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/infrastructure/vector"
	"eva/internal/cortex/selfawareness"
	"eva/internal/hippocampus/knowledge"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}

	qdrantClient, err := vector.NewQdrantClient(cfg.QdrantHost, cfg.QdrantPort)
	if err != nil {
		log.Fatalf("Qdrant connect failed: %v", err)
	}
	defer qdrantClient.Close()

	embedSvc, err := knowledge.NewEmbeddingService(cfg, qdrantClient)
	if err != nil {
		log.Fatalf("Embedding service failed: %v", err)
	}

	svc := selfawareness.NewSelfAwarenessService(nil, qdrantClient, embedSvc, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	basePath := "."
	if len(os.Args) > 1 {
		basePath = os.Args[1]
	}

	// 1. Index Go source files (AST parsing)
	fmt.Printf("=== Indexing Go files (AST) from: %s ===\n", basePath)
	start := time.Now()

	indexed, err := svc.IndexCodebase(ctx, basePath)
	if err != nil {
		log.Fatalf("Code indexing failed: %v", err)
	}
	fmt.Printf("Code: indexed %d .go files in %v\n\n", indexed, time.Since(start).Round(time.Second))

	// 2. Index Markdown documentation
	fmt.Printf("=== Indexing .md docs from: %s ===\n", basePath)
	startDocs := time.Now()

	docsIndexed, err := svc.IndexDocs(ctx, basePath)
	if err != nil {
		log.Printf("WARNING: Docs indexing failed: %v", err)
	} else {
		fmt.Printf("Docs: indexed %d .md chunks in %v\n\n", docsIndexed, time.Since(startDocs).Round(time.Second))
	}

	fmt.Printf("=== DONE! Total: %d code + %d docs in %v ===\n",
		indexed, docsIndexed, time.Since(start).Round(time.Second))
}
