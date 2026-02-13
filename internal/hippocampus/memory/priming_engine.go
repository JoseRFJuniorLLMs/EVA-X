package memory

import (
	"context"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"fmt"
)

// PrimingEngine substitui RetrievalService no modelo FZPN
type PrimingEngine struct {
	client *graph.Neo4jClient
}

// NewPrimingEngine cria novo motor de priming
func NewPrimingEngine(client *graph.Neo4jClient) *PrimingEngine {
	return &PrimingEngine{client: client}
}

// Prime realiza a busca "Fractal" puxando nós conectados
func (p *PrimingEngine) Prime(ctx context.Context, idosoID int64, queryText string) ([]string, error) {
	// 1. Busca por palavras-chave (Significantes/Topicos) na query
	// (Simplificação: busca textual exata ou contains)

	cypher := `
		MATCH (p:Person {id: $idosoId})-[:EXPERIENCED]->(e:Event)
		WHERE e.content CONTAINS $text OR e.speaker = 'user'
		
		// Encontrar nós conectados (Significantes, Tópicos)
		OPTIONAL MATCH (e)-[:RELATED_TO|EVOCA]->(related)
		
		// Calcular "peso" baseado nas conexões (Simula ativação neural)
		WITH e, related, count(related) as weight
		ORDER BY weight DESC, e.timestamp DESC
		LIMIT 5
		
		RETURN e.content as content
	`

	params := map[string]interface{}{
		"idosoId": idosoID,
		"text":    queryText,
	}

	records, err := p.client.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("priming search failed: %w", err)
	}

	var results []string
	for _, record := range records {
		content, _ := record.Get("content")
		if str, ok := content.(string); ok {
			results = append(results, str)
		}
	}

	return results, nil
}
