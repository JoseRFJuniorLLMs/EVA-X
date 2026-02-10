package graph

import (
	"context"
	"eva-mind/internal/brainstem/config"
	"fmt"
	"log"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Neo4jClient struct {
	driver neo4j.DriverWithContext
}

func NewNeo4jClient(cfg *config.Config) (*Neo4jClient, error) {
	driver, err := neo4j.NewDriverWithContext(
		cfg.Neo4jURI,
		neo4j.BasicAuth(cfg.Neo4jUsername, cfg.Neo4jPassword, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	// Verify connection
	err = driver.VerifyConnectivity(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to neo4j: %w", err)
	}

	log.Println("✅ Conectado ao Neo4j com sucesso")
	return &Neo4jClient{driver: driver}, nil
}

func (c *Neo4jClient) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

func (c *Neo4jClient) ExecuteWrite(ctx context.Context, cypher string, params map[string]interface{}) (any, error) {
	if c == nil || c.driver == nil {
		return nil, fmt.Errorf("neo4j client not initialized or disconnected")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		// Se for uma query que retorna registros, coletar todos
		// Nota: Para operações simples de escrita, muitas vezes não precisamos do retorno,
		// mas se precisarmos, podemos adaptar. Por padrão, retornamos o result summary ou records.
		if result.Err() != nil {
			return nil, result.Err()
		}

		// Consumir para garantir execução
		summary, err := result.Consume(ctx)
		if err == nil {
			preview := cypher
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			log.Printf("📥 [NEO4J] Escrita concluída: Query=\"%s\", Params=%v", preview, params)
		}
		return summary, err
	})

	return result, err
}

func (c *Neo4jClient) ExecuteRead(ctx context.Context, cypher string, params map[string]interface{}) ([]*neo4j.Record, error) {
	if c == nil || c.driver == nil {
		return nil, fmt.Errorf("neo4j client not initialized or disconnected")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		return result.Collect(ctx)
	})

	if err != nil {
		return nil, err
	}

	records := result.([]*neo4j.Record)
	preview := cypher
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}
	log.Printf("🔍 [NEO4J] Leitura concluída: Query=\"%s\", Records=%d", preview, len(records))
	return records, nil
}
