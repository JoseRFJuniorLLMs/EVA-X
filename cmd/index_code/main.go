// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// index_code indexes all EVA-Mind .go files into Qdrant for semantic search.
// Run: go run cmd/index_code/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"eva-mind/internal/cortex/selfawareness"
	"eva-mind/internal/hippocampus/knowledge"

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Get base path from args or use current directory
	basePath := "."
	if len(os.Args) > 1 {
		basePath = os.Args[1]
	}

	fmt.Printf("Indexing Go files from: %s\n", basePath)
	start := time.Now()

	indexed, err := svc.IndexCodebase(ctx, basePath)
	if err != nil {
		log.Fatalf("Indexing failed: %v", err)
	}

	fmt.Printf("Done! Indexed %d files in %v\n", indexed, time.Since(start).Round(time.Second))
}
