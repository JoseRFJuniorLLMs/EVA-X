package brain

import (
	"context"
	"database/sql"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"eva-mind/internal/brainstem/push"
	"eva-mind/internal/cortex/lacan"
	ps "eva-mind/internal/cortex/personality"
	"eva-mind/internal/hippocampus/knowledge"
	"eva-mind/internal/hippocampus/memory"
	"fmt"
)

// Service encapsulates the cognitive functions of EVA
// AUDIT FIX 2026-01-27: Adicionado neo4jClient e graphStore para salvar no Neo4j
type Service struct {
	db                 *sql.DB
	qdrantClient       *vector.QdrantClient
	neo4jClient        *graph.Neo4jClient  // AUDIT FIX: Adicionado para salvar no Neo4j
	graphStore         *memory.GraphStore  // AUDIT FIX: Store para Neo4j
	fdpnEngine         *lacan.FDPNEngine
	personalityService *ps.PersonalityService
	zetaRouter         *ps.ZetaRouter
	pushService        *push.FirebaseService
	embeddingService   *memory.EmbeddingService

	knowledgeEmbedder *knowledge.EmbeddingService
	unifiedRetrieval  *lacan.UnifiedRetrieval
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
) *Service {
	var graphStore *memory.GraphStore
	if neo4j != nil {
		graphStore = memory.NewGraphStore(neo4j, nil)
	}

	return &Service{
		db:                 db,
		qdrantClient:       qdrant,
		neo4jClient:        neo4j,       // AUDIT FIX
		graphStore:         graphStore,  // AUDIT FIX
		personalityService: personalitySvc,
		zetaRouter:         zeta,
		pushService:        push,
		embeddingService:   embedder,
		unifiedRetrieval:   unified,
	}
}

// GetSystemPrompt gera o prompt inicial unificado (RSI)
func (s *Service) GetSystemPrompt(ctx context.Context, idosoID int64) (string, error) {
	if s.unifiedRetrieval == nil {
		return "", fmt.Errorf("unified retrieval not initialized")
	}
	return s.unifiedRetrieval.GetPromptForGemini(ctx, idosoID, "", "")
}

// ProcessUserSpeech handles user transcription in real-time (FDPN Hook)
