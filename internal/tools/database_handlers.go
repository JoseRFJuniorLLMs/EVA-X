// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ============================================================================
// 🗄️ POSTGRESQL — Query SELECT Direto
// ============================================================================

func (h *ToolsHandler) handleQueryPostgreSQL(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe a query SQL"}, nil
	}

	normalized := strings.TrimSpace(strings.ToUpper(query))

	// SECURITY: Bloquear DML/DDL — apenas SELECT/WITH permitidos
	isSelect := strings.HasPrefix(normalized, "SELECT") || strings.HasPrefix(normalized, "WITH")
	if !isSelect {
		return map[string]interface{}{
			"error":   "Apenas queries SELECT sao permitidas por seguranca",
			"message": "Use SELECT ou WITH para consultar dados. INSERT/UPDATE/DELETE/CREATE/ALTER/DROP nao sao permitidos via MCP.",
		}, nil
	}

	// SELECT — retorna linhas
	{
		rows, err := h.db.Conn.Query(query)
		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("Erro na query: %v", err)}, nil
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("Erro ao ler colunas: %v", err)}, nil
		}

		var results []map[string]interface{}
		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				continue
			}

			row := make(map[string]interface{})
			for i, col := range columns {
				val := values[i]
				switch v := val.(type) {
				case []byte:
					row[col] = string(v)
				default:
					row[col] = v
				}
			}
			results = append(results, row)

			if len(results) >= 100 {
				break
			}
		}

		return map[string]interface{}{
			"status":  "sucesso",
			"columns": columns,
			"rows":    results,
			"count":   len(results),
			"message": fmt.Sprintf("Query retornou %d resultados.", len(results)),
		}, nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// 🔗 NEO4J — Query Cypher (via Bolt, usando o driver existente)
// ============================================================================

func (h *ToolsHandler) handleQueryNeo4j(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe a query Cypher"}, nil
	}

	// SECURITY: Apenas MATCH/RETURN permitido (read-only)
	normalized := strings.TrimSpace(strings.ToUpper(query))
	dangerous := []string{"CREATE", "DELETE", "DETACH", "SET ", "REMOVE", "MERGE", "DROP", "CALL"}
	for _, d := range dangerous {
		if strings.Contains(normalized, d) {
			return map[string]interface{}{"error": fmt.Sprintf("Operacao '%s' nao permitida — apenas leitura", d)}, nil
		}
	}

	if h.neo4jClient == nil {
		return map[string]interface{}{
			"error":   "Neo4j (porta 7687) nao disponivel",
			"message": "O client Neo4j nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	records, err := h.neo4jClient.ExecuteRead(ctx, query, nil)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na query Neo4j: %v", err)}, nil
	}

	var results []map[string]interface{}
	for _, rec := range records {
		row := map[string]interface{}{}
		for i, key := range rec.Keys {
			row[key] = fmt.Sprintf("%v", rec.Values[i])
		}
		results = append(results, row)
		if len(results) >= 100 {
			break
		}
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"rows":    results,
		"count":   len(results),
		"message": fmt.Sprintf("Query Neo4j retornou %d resultados.", len(results)),
	}, nil
}

// ============================================================================
// 🔍 QDRANT — Busca Vetorial
// ============================================================================

func (h *ToolsHandler) handleQueryQdrant(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	query, _ := args["query"].(string)

	// limit pode vir como string (MCP) ou float64 (JSON)
	limit := 5
	if limitStr, ok := args["limit"].(string); ok && limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	} else if limitFloat, ok := args["limit"].(float64); ok {
		limit = int(limitFloat)
	}
	if limit <= 0 || limit > 50 {
		limit = 5
	}

	if collection == "" {
		return map[string]interface{}{"error": "Informe o nome da colecao (collection)"}, nil
	}
	if query == "" {
		return map[string]interface{}{"error": "Informe o texto de busca (query)"}, nil
	}

	if h.qdrantClient == nil {
		return map[string]interface{}{
			"error":   "Qdrant nao disponivel",
			"message": "O client Qdrant nao foi configurado no ToolsHandler",
		}, nil
	}
	if h.embedFunc == nil {
		return map[string]interface{}{
			"error":   "Servico de embeddings nao disponivel",
			"message": "EmbedFunc nao configurado — necessario para converter texto em vetor",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Gerar embedding do texto de busca
	vector, err := h.embedFunc(ctx, query)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao gerar embedding: %v", err)}, nil
	}

	// 2. Buscar no Qdrant
	points, err := h.qdrantClient.Search(ctx, collection, vector, uint64(limit), nil)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na busca Qdrant: %v", err)}, nil
	}

	// 3. Formatar resultados
	var results []map[string]interface{}
	for _, point := range points {
		item := map[string]interface{}{
			"score": point.Score,
		}
		if point.Id != nil {
			item["id"] = fmt.Sprintf("%v", point.Id)
		}
		if point.Payload != nil {
			for k, v := range point.Payload {
				item[k] = v.GetStringValue()
			}
		}
		results = append(results, item)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"status":     "sucesso",
		"collection": collection,
		"rows":       results,
		"count":      len(results),
		"message":    fmt.Sprintf("Busca vetorial em '%s' retornou %d resultados.", collection, len(results)),
	}, nil
}

// ============================================================================
// 📖 NIETZSCHEDB — Consultar API
// ============================================================================

func (h *ToolsHandler) handleQueryNietzsche(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	endpoint, _ := args["endpoint"].(string)
	if endpoint == "" {
		endpoint = "/api/quotes/random"
	}

	// Construir URL
	baseURL := "http://localhost:3000" // NietzscheDB na mesma VM
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	url := baseURL + endpoint

	// Non-blocking
	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("❌ [NIETZSCHE] Erro: %v", err)
			if h.NotifyFunc != nil {
				h.NotifyFunc(idosoID, "nietzsche_error", map[string]interface{}{"error": err.Error()})
			}
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var result interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			result = string(body)
		}

		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "nietzsche_result", map[string]interface{}{
				"endpoint": endpoint,
				"data":     result,
			})
		}
	}()

	return map[string]interface{}{
		"status":   "consultando",
		"endpoint": endpoint,
		"message":  fmt.Sprintf("Consultando NietzscheDB: %s", endpoint),
	}, nil
}
