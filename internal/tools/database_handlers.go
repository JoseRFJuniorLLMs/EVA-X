// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
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

	// Detectar tipo de operação
	isSelect := strings.HasPrefix(normalized, "SELECT") || strings.HasPrefix(normalized, "WITH")

	if isSelect {
		// SELECT — retorna linhas
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

	// INSERT, UPDATE, DELETE, CREATE, ALTER, etc — executa e retorna rows affected
	result, err := h.db.Conn.Exec(query)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao executar: %v", err)}, nil
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("🗄️ [SQL] Executado: %s → %d rows affected", query[:min(len(query), 80)], rowsAffected)

	return map[string]interface{}{
		"status":        "sucesso",
		"rows_affected": rowsAffected,
		"message":       fmt.Sprintf("Comando executado. %d linhas afetadas.", rowsAffected),
	}, nil
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
	dangerous := []string{"CREATE", "DELETE", "DETACH", "SET ", "REMOVE", "MERGE", "DROP"}
	for _, d := range dangerous {
		if strings.Contains(normalized, d) {
			return map[string]interface{}{"error": fmt.Sprintf("Operação '%s' não permitida — apenas leitura", d)}, nil
		}
	}

	// O Neo4j client está no campo Dependencies do orchestrator ou precisa ser injetado
	// Por enquanto, retornar que Neo4j está disponível via selfawareness agent
	return map[string]interface{}{
		"status":  "info",
		"message": "Para queries Neo4j, use 'query_my_database' do serviço de autoconhecimento ou 'search_my_code'. O acesso Cypher direto será implementado em breve.",
		"query":   query,
	}, nil
}

// ============================================================================
// 🔍 QDRANT — Busca Vetorial
// ============================================================================

func (h *ToolsHandler) handleQueryQdrant(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	query, _ := args["query"].(string)
	limitFloat, _ := args["limit"].(float64)
	limit := int(limitFloat)
	if limit <= 0 {
		limit = 10
	}

	if collection == "" && query == "" {
		return map[string]interface{}{"error": "Informe collection e/ou query"}, nil
	}

	// Usar o serviço de autoconhecimento para busca
	return map[string]interface{}{
		"status":     "info",
		"collection": collection,
		"query":      query,
		"limit":      limit,
		"message":    "Para buscas vetoriais, use 'search_knowledge' ou 'list_my_collections' do serviço de autoconhecimento. O acesso Qdrant direto será implementado em breve.",
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
