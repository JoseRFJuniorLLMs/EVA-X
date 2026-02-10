package memory

import (
	"context"
	"database/sql"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"fmt"
	"log"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// RetrievalService busca memórias por similaridade semântica
type RetrievalService struct {
	db       *sql.DB
	embedder *EmbeddingService
	qdrant   *vector.QdrantClient
}

// NewRetrievalService cria um novo serviço de busca
func NewRetrievalService(db *sql.DB, embedder *EmbeddingService, qdrant *vector.QdrantClient) *RetrievalService {
	return &RetrievalService{
		db:       db,
		embedder: embedder,
		qdrant:   qdrant,
	}
}

// SearchResult representa um resultado de busca com score de similaridade
type SearchResult struct {
	Memory     *Memory
	Similarity float64 // 0.0 (nada similar) a 1.0 (idêntico)
}

// Retrieve busca as K memórias mais relevantes para uma query (HÍBRIDO: Postgres + Qdrant)
func (r *RetrievalService) Retrieve(ctx context.Context, idosoID int64, query string, k int) ([]*SearchResult, error) {
	// 1. Gerar embedding da query
	queryEmbedding, err := r.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar embedding: %w", err)
	}

	var allResults []*SearchResult
	seenIDs := make(map[int64]bool)

	// 2. BUSCA NO POSTGRES (pgvector)
	sqlQuery := `
		SELECT id, content, speaker, memory_timestamp, emotion, importance, topics, similarity 
		FROM search_similar_memories(
			$1,  -- idoso_id
			$2,  -- query_embedding
			$3,  -- limit
			$4   -- min_similarity
		)
	`
	rows, err := r.db.QueryContext(ctx, sqlQuery, idosoID, vectorToPostgres(queryEmbedding), k, 0.5)
	if err == nil {
		defer rows.Close()
		log.Printf("🔍 [MEMORY] Postgres Search: Query=\"%s\"", query)
		for rows.Next() {
			var (
				memoryID               int64
				content, speaker       string
				ts                     time.Time
				topics                 string
				importance, similarity float64
				emotion                sql.NullString
			)
			// Nota: a função search_similar_memories deve retornar colunas compatíveis
			err := rows.Scan(&memoryID, &content, &speaker, &ts, &emotion, &importance, &topics, &similarity)
			if err == nil {
				mem := &Memory{
					ID:         memoryID,
					IdosoID:    idosoID,
					Timestamp:  ts,
					Speaker:    speaker,
					Content:    content,
					Emotion:    emotion.String,
					Importance: importance,
					Topics:     parsePostgresArray(topics),
				}
				allResults = append(allResults, &SearchResult{Memory: mem, Similarity: similarity})
				seenIDs[memoryID] = true
			} else {
				log.Printf("⚠️ [MEMORY] Erro ao escanear linha Postgres: %v", err)
			}
		}
	} else {
		log.Printf("⚠️ [MEMORY] Erro busca Postgres: %v", err)
	}

	// 3. BUSCA NO QDRANT (Se disponível)
	if r.qdrant != nil {
		log.Printf("🔍 [MEMORY] Qdrant Search: Query=\"%s\"", query)
		qResults, err := r.qdrant.Search(ctx, "memories", queryEmbedding, uint64(k), nil)
		if err == nil {
			for _, qr := range qResults {
				// Mapear payload do Qdrant para Memory
				p := qr.Payload

				// Nil checks para evitar panic
				var contentStr, speakerStr string
				if contentVal, ok := p["content"].GetKind().(*qdrant.Value_StringValue); ok && contentVal != nil {
					contentStr = contentVal.StringValue
				}
				if speakerVal, ok := p["speaker"].GetKind().(*qdrant.Value_StringValue); ok && speakerVal != nil {
					speakerStr = speakerVal.StringValue
				}

				// Evitar duplicados se já veio do Postgres
				// Nota: Qdrant ID pode não bater com Postgres ID se não sincronizado
				allResults = append(allResults, &SearchResult{
					Memory: &Memory{
						Content: contentStr,
						Speaker: speakerStr,
					},
					Similarity: float64(qr.Score),
				})
			}
		} else {
			log.Printf("⚠️ [MEMORY] Erro busca Qdrant: %v", err)
		}
	}

	return allResults, nil
}

// RetrieveRecent busca memórias recentes (últimos N dias) sem usar embedding
// Útil para contexto temporal imediato
func (r *RetrievalService) RetrieveRecent(ctx context.Context, idosoID int64, days int, limit int) ([]*Memory, error) {
	query := `
		SELECT id, idoso_id, timestamp, speaker, content, emotion, 
		       importance, topics, session_id
		FROM episodic_memories
		WHERE idoso_id = $1
		  AND timestamp > NOW() - INTERVAL '1 day' * $2
		ORDER BY importance DESC, timestamp DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, idosoID, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	log.Printf("🔍 [MEMORY] Busca Recente: Idoso=%d, Dias=%d, Limit=%d", idosoID, days, limit)

	var memories []*Memory

	for rows.Next() {
		memory := &Memory{}
		var topics string

		err := rows.Scan(
			&memory.ID,
			&memory.IdosoID,
			&memory.Timestamp,
			&memory.Speaker,
			&memory.Content,
			&memory.Emotion,
			&memory.Importance,
			&topics,
			&memory.SessionID,
		)

		if err != nil {
			return nil, err
		}

		memory.Topics = parsePostgresArray(topics)
		memories = append(memories, memory)
	}

	return memories, rows.Err()
}

// RetrieveHybrid combina busca semântica + temporal
// Retorna memórias relevantes E recentes
func (r *RetrievalService) RetrieveHybrid(ctx context.Context, idosoID int64, query string, k int) ([]*SearchResult, error) {
	// Buscar memórias semânticas
	semantic, err := r.Retrieve(ctx, idosoID, query, k)
	if err != nil {
		return nil, err
	}

	// Buscar memórias recentes (últimos 3 dias)
	recent, err := r.RetrieveRecent(ctx, idosoID, 3, k/2)
	if err != nil {
		log.Printf("⚠️ Erro ao buscar memórias recentes: %v", err)
		return semantic, nil // Retorna apenas semânticas
	}

	// Mesclar e deduplicar
	seen := make(map[int64]bool)
	var combined []*SearchResult

	// Adicionar semânticas primeiro
	for _, res := range semantic {
		if !seen[res.Memory.ID] {
			combined = append(combined, res)
			seen[res.Memory.ID] = true
		}
	}

	// Adicionar recentes (se não duplicadas)
	for _, mem := range recent {
		if !seen[mem.ID] {
			combined = append(combined, &SearchResult{
				Memory:     mem,
				Similarity: 0.9, // Score artificial alto para recentes
			})
			seen[mem.ID] = true

			if len(combined) >= k {
				break
			}
		}
	}

	return combined, nil
}
