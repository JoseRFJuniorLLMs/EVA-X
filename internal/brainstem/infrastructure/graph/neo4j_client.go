// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package graph

import (
	"context"
	"eva/internal/brainstem/config"
	"fmt"
	"log"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// sensitiveParamKeys lists parameter names that should be redacted in logs (LGPD compliance)
var sensitiveParamKeys = map[string]bool{
	"cpf": true, "nome": true, "name": true, "telefone": true, "phone": true,
	"email": true, "endereco": true, "address": true, "rg": true,
	"content": true, "text": true, "transcript": true, "speaker": true,
}

// sanitizeParams returns a log-safe version of query parameters
func sanitizeParams(params map[string]interface{}) string {
	if params == nil {
		return "{}"
	}
	parts := make([]string, 0, len(params))
	for k, v := range params {
		if sensitiveParamKeys[strings.ToLower(k)] {
			parts = append(parts, fmt.Sprintf("%s:[REDACTED]", k))
		} else {
			parts = append(parts, fmt.Sprintf("%s:%v", k, v))
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

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
			log.Printf("📥 [NEO4J] Escrita concluída: Query=\"%s\", Params=%s", preview, sanitizeParams(params))
		}
		return summary, err
	})

	return result, err
}

func (c *Neo4jClient) ExecuteWriteAndReturn(ctx context.Context, cypher string, params map[string]interface{}) ([]*neo4j.Record, error) {
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
	log.Printf("📥 [NEO4J] Escrita com retorno concluída: Query=\"%s\", Records=%d", preview, len(records))
	return records, nil
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
func (c *Neo4jClient) GetDriver() neo4j.DriverWithContext {
	if c == nil {
		return nil
	}
	return c.driver
}
