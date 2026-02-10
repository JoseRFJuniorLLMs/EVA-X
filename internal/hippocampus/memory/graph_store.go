package memory

import (
	"context"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"fmt"
	"time"
)

// GraphStore gerencia o armazenamento de memórias em Grafo (Neo4j)
type GraphStore struct {
	client *graph.Neo4jClient
	cfg    *config.Config
}

// NewGraphStore cria um novo gerenciador de memórias em grafo
func NewGraphStore(client *graph.Neo4jClient, cfg *config.Config) *GraphStore {
	return &GraphStore{
		client: client,
		cfg:    cfg,
	}
}

// StoreCausalMemory salva uma memória "explodida" em nós
func (g *GraphStore) StoreCausalMemory(ctx context.Context, memory *Memory) error {
	// 1. Criar nó do Evento Base
	query := `
		MERGE (p:Person {id: $idosoId})
		CREATE (e:Event {
			id: $id,
			content: $content,
			timestamp: datetime($timestamp),
			speaker: $speaker,
			emotion: $emotion,
			importance: $importance,
			sessionId: $sessionId
		})
		CREATE (p)-[:EXPERIENCED]->(e)
	`

	params := map[string]interface{}{
		"idosoId": memory.IdosoID,
		"id":      memory.ID, // Assumindo que ID já foi gerado ou usamos UUID? SQL gera ID. Aqui talvez precisemos gerar.
		// Se memory.ID for 0, precisamos gerar um UUID.
		"content":    memory.Content,
		"timestamp":  memory.Timestamp.Format(time.RFC3339),
		"speaker":    memory.Speaker,
		"emotion":    memory.Emotion,
		"importance": memory.Importance,
		"sessionId":  memory.SessionID,
	}

	// Se ID for zero (novo), gerar UUID ou usar timestamp
	if memory.ID == 0 {
		params["id"] = fmt.Sprintf("%d-%d", memory.IdosoID, time.Now().UnixNano())
	}

	_, err := g.client.ExecuteWrite(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to create base event node: %w", err)
	}

	// 2. Extrair e conectar entidades (Simplificado por agora - idealmente via LLM)
	// Aqui poderíamos conectar Topicos
	if len(memory.Topics) > 0 {
		for _, topic := range memory.Topics {
			topicQuery := `
				MATCH (e:Event {id: $eventId})
				MATCH (p:Person {id: $idosoId})
				
				MERGE (t:Topic {name: $topic})
				ON CREATE SET t.created = datetime()
				
				// Conectar Event -> Topic
				MERGE (e)-[:RELATED_TO]->(t)
				
				// ✅ Conectar Person -> Topic COM CONTADOR
				MERGE (p)-[r:MENTIONED]->(t)
				ON CREATE SET r.count = 1, r.first_mention = datetime()
				ON MATCH SET 
					r.count = r.count + 1,
					r.last_mention = datetime()
			`
			topicParams := map[string]interface{}{
				"eventId": params["id"],
				"idosoId": memory.IdosoID,
				"topic":   topic,
			}
			g.client.ExecuteWrite(ctx, topicQuery, topicParams)
		}
	}

	// 3. Conectar Emoções (Se houver na análise)
	if memory.Emotion != "" && memory.Emotion != "neutro" {
		emotionQuery := `
			MATCH (p:Person {id: $idosoId})
			MERGE (em:Emotion {name: $emotion})
			MERGE (p)-[r:FEELS]->(em)
			ON CREATE SET r.count = 1, r.first_felt = datetime()
			ON MATCH SET 
				r.count = r.count + 1,
				r.last_felt = datetime()
		`
		emotionParams := map[string]interface{}{
			"idosoId": memory.IdosoID,
			"emotion": memory.Emotion,
		}
		g.client.ExecuteWrite(ctx, emotionQuery, emotionParams)
	}

	return nil
}
