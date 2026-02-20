// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package brain

import (
	"context"
	"database/sql"
	"eva/internal/brainstem/infrastructure/graph"
	"eva/internal/brainstem/infrastructure/vector"
	"eva/internal/brainstem/push"
	"eva/internal/cortex/lacan"
	ps "eva/internal/cortex/personality"
	"eva/internal/hippocampus/knowledge"
	"eva/internal/hippocampus/memory"
	"eva/internal/memory/ingestion"
	"fmt"
)

// Service encapsulates the cognitive functions of EVA
// AUDIT FIX 2026-01-27: Adicionado neo4jClient e graphStore para salvar no Neo4j
type Service struct {
	db                 *sql.DB
	qdrantClient       *vector.QdrantClient
	neo4jClient        *graph.Neo4jClient // AUDIT FIX: Adicionado para salvar no Neo4j
	graphStore         *memory.GraphStore // AUDIT FIX: Store para Neo4j
	memoryStore        *memory.MemoryStore // AUDIT FIX 2026-02-17: Store para salvar com importância/emoção reais
	fdpnEngine         *lacan.FDPNEngine
	personalityService *ps.PersonalityService
	zetaRouter         *ps.ZetaRouter
	pushService        *push.FirebaseService
	embeddingService   *memory.EmbeddingService

	knowledgeEmbedder *knowledge.EmbeddingService
	unifiedRetrieval  *lacan.UnifiedRetrieval
	ingestionPipeline *ingestion.IngestionPipeline
}

// NewService creates a new Brain service
// AUDIT FIX 2026-01-27: Adicionado neo4jClient para salvar no grafo
func NewService(
	db *sql.DB,
	qdrant *vector.QdrantClient,
	neo4j *graph.Neo4jClient, // AUDIT FIX: Adicionado
	unified *lacan.UnifiedRetrieval,
	personalitySvc *ps.PersonalityService,
	zeta *ps.ZetaRouter,
	push *push.FirebaseService,
	embedder *memory.EmbeddingService,
	ingestionPipeline *ingestion.IngestionPipeline,
) *Service {
	var graphStore *memory.GraphStore
	if neo4j != nil {
		graphStore = memory.NewGraphStore(neo4j, nil)
	}

	var memoryStore *memory.MemoryStore
	if db != nil {
		memoryStore = memory.NewMemoryStore(db, graphStore, qdrant)
	}

	return &Service{
		db:                 db,
		qdrantClient:       qdrant,
		neo4jClient:        neo4j,      // AUDIT FIX
		graphStore:         graphStore, // AUDIT FIX
		memoryStore:        memoryStore, // AUDIT FIX 2026-02-17
		personalityService: personalitySvc,
		zetaRouter:         zeta,
		pushService:        push,
		embeddingService:   embedder,
		unifiedRetrieval:   unified,
		ingestionPipeline:  ingestionPipeline,
	}
}

// GetSystemPrompt gera o prompt inicial unificado (RSI)
func (s *Service) GetSystemPrompt(ctx context.Context, idosoID int64) (string, string, error) {
	if s.unifiedRetrieval == nil {
		return "", "", fmt.Errorf("unified retrieval not initialized")
	}
	return s.unifiedRetrieval.GetPromptForGemini(ctx, idosoID, "", "")
}

// InvalidatePromptCache limpa o cache de prompt para um idoso
func (s *Service) InvalidatePromptCache(idosoID int64) {
	if s.unifiedRetrieval != nil {
		s.unifiedRetrieval.InvalidatePromptCache(idosoID)
	}
}

// ProcessUserSpeech handles user transcription in real-time (FDPN Hook)
