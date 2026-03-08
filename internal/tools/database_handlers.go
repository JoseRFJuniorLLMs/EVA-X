// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	nietzsche "nietzsche-sdk"
)

// ============================================================================
// 🗄️ NietzscheDB — Query SELECT Direto
// ============================================================================

func (h *ToolsHandler) handleQueryNietzscheDB(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe a query NQL"}, nil
	}

	normalized := strings.TrimSpace(strings.ToUpper(query))

	// SECURITY: Bloquear mutacoes — apenas MATCH/RETURN permitidos
	isRead := strings.HasPrefix(normalized, "MATCH") || strings.HasPrefix(normalized, "SELECT") || strings.HasPrefix(normalized, "WITH")
	if !isRead {
		return map[string]interface{}{
			"error":   "Apenas queries de leitura (MATCH ... RETURN) sao permitidas por seguranca",
			"message": "Use NQL MATCH para consultar dados. Mutacoes nao sao permitidas via MCP.",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Execute via NietzscheDB NQL
	result, err := h.db.NQL(ctx, query, nil)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na query NQL: %v", err)}, nil
	}

	var results []map[string]interface{}
	if result != nil {
		for _, node := range result.Nodes {
			row := map[string]interface{}{
				"id":        node.ID,
				"node_type": node.NodeType,
				"energy":    node.Energy,
			}
			if node.Content != nil {
				for k, v := range node.Content {
					row[k] = fmt.Sprintf("%v", v)
				}
			}
			results = append(results, row)
			if len(results) >= 100 {
				break
			}
		}
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"rows":    results,
		"count":   len(results),
		"message": fmt.Sprintf("Query NQL retornou %d resultados.", len(results)),
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// 🔗 NIETZSCHEDB GRAPH — Query NQL
// ============================================================================

func (h *ToolsHandler) handleQueryGraph(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe a query NQL"}, nil
	}

	if h.graphAdapter == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB GraphAdapter nao disponivel",
			"message": "O GraphAdapter nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.graphAdapter.ExecuteNQL(ctx, query, nil, "")
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na query NQL: %v", err)}, nil
	}

	var results []map[string]interface{}
	if result != nil {
		for _, node := range result.Nodes {
			row := map[string]interface{}{
				"id":        node.ID,
				"node_type": node.NodeType,
				"energy":    node.Energy,
			}
			if node.Content != nil {
				for k, v := range node.Content {
					row[k] = fmt.Sprintf("%v", v)
				}
			}
			results = append(results, row)
			if len(results) >= 100 {
				break
			}
		}
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"rows":    results,
		"count":   len(results),
		"message": fmt.Sprintf("Query NQL retornou %d resultados.", len(results)),
	}, nil
}

// ============================================================================
// 🔍 NIETZSCHEDB — Busca Vetorial
// ============================================================================

func (h *ToolsHandler) handleQueryVector(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
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

	if h.vectorAdapter == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB VectorAdapter nao disponivel",
			"message": "O VectorAdapter nao foi configurado no ToolsHandler",
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

	// 2. Buscar no NietzscheDB via VectorAdapter
	points, err := h.vectorAdapter.Search(ctx, collection, vector, limit, idosoID)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na busca vetorial: %v", err)}, nil
	}

	// 3. Formatar resultados
	var results []map[string]interface{}
	for _, point := range points {
		item := map[string]interface{}{
			"id":    point.ID,
			"score": point.Score,
		}
		if point.Payload != nil {
			for k, v := range point.Payload {
				item[k] = fmt.Sprintf("%v", v)
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
// 📖 NIETZSCHEDB — Consultar via gRPC SDK (porta 50051)
// ============================================================================

func (h *ToolsHandler) handleQueryNietzsche(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB nao disponivel",
			"message": "O client NietzscheDB gRPC nao foi configurado no ToolsHandler",
		}, nil
	}

	action, _ := args["action"].(string)
	if action == "" {
		action = "stats"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch action {
	case "health":
		if err := h.nietzscheClient.Health(ctx); err != nil {
			return map[string]interface{}{"status": "erro", "error": fmt.Sprintf("HealthCheck falhou: %v", err)}, nil
		}
		return map[string]interface{}{"status": "sucesso", "message": "NietzscheDB esta saudavel"}, nil

	case "stats":
		stats, err := h.nietzscheClient.GetStats(ctx)
		if err != nil {
			return map[string]interface{}{"status": "erro", "error": fmt.Sprintf("GetStats falhou: %v", err)}, nil
		}
		return map[string]interface{}{"status": "sucesso", "stats": stats}, nil

	case "collections":
		cols, err := h.nietzscheClient.ListCollections(ctx)
		if err != nil {
			return map[string]interface{}{"status": "erro", "error": fmt.Sprintf("ListCollections falhou: %v", err)}, nil
		}
		var colList []map[string]interface{}
		for _, c := range cols {
			colList = append(colList, map[string]interface{}{
				"name":       c.Name,
				"dim":        c.Dim,
				"metric":     c.Metric,
				"node_count": c.NodeCount,
				"edge_count": c.EdgeCount,
			})
		}
		if colList == nil {
			colList = []map[string]interface{}{}
		}
		return map[string]interface{}{"status": "sucesso", "collections": colList, "count": len(colList)}, nil

	case "query":
		nql, _ := args["nql"].(string)
		if nql == "" {
			return map[string]interface{}{"error": "Informe a query NQL no campo 'nql'"}, nil
		}
		collection, _ := args["collection"].(string)

		result, err := h.nietzscheClient.Query(ctx, nql, nil, collection)
		if err != nil {
			return map[string]interface{}{"status": "erro", "error": fmt.Sprintf("Query NQL falhou: %v", err)}, nil
		}

		var rows []map[string]interface{}
		for _, node := range result.Nodes {
			row := map[string]interface{}{
				"id":        node.ID,
				"node_type": node.NodeType,
				"energy":    node.Energy,
			}
			if node.Content != nil {
				for k, v := range node.Content {
					row[k] = v
				}
			}
			rows = append(rows, row)
			if len(rows) >= 100 {
				break
			}
		}
		for _, pair := range result.NodePairs {
			row := map[string]interface{}{
				"from_id": pair.From.ID,
				"to_id":   pair.To.ID,
			}
			rows = append(rows, row)
			if len(rows) >= 100 {
				break
			}
		}
		if rows == nil {
			rows = []map[string]interface{}{}
		}
		return map[string]interface{}{
			"status":  "sucesso",
			"rows":    rows,
			"count":   len(rows),
			"explain": result.Explain,
			"message": fmt.Sprintf("Query NQL retornou %d resultados.", len(rows)),
		}, nil

	case "get_node":
		nodeID, _ := args["node_id"].(string)
		collection, _ := args["collection"].(string)
		if nodeID == "" {
			return map[string]interface{}{"error": "Informe o node_id"}, nil
		}
		node, err := h.nietzscheClient.GetNode(ctx, nodeID, collection)
		if err != nil {
			return map[string]interface{}{"status": "erro", "error": fmt.Sprintf("GetNode falhou: %v", err)}, nil
		}
		if !node.Found {
			return map[string]interface{}{"status": "sucesso", "found": false, "message": "Node nao encontrado"}, nil
		}
		return map[string]interface{}{
			"status":    "sucesso",
			"found":     true,
			"id":        node.ID,
			"node_type": node.NodeType,
			"energy":    node.Energy,
			"content":   node.Content,
		}, nil

	case "insert_node":
		collection, _ := args["collection"].(string)
		nodeType, _ := args["node_type"].(string)
		if nodeType == "" {
			nodeType = "Semantic"
		}
		content, _ := args["content"]
		result, err := h.nietzscheClient.InsertNode(ctx, nietzsche.InsertNodeOpts{
			Collection: collection,
			NodeType:   nodeType,
			Content:    content,
		})
		if err != nil {
			return map[string]interface{}{"status": "erro", "error": fmt.Sprintf("InsertNode falhou: %v", err)}, nil
		}
		return map[string]interface{}{
			"status":  "sucesso",
			"node_id": result.ID,
			"message": fmt.Sprintf("Node criado com ID %s", result.ID),
		}, nil

	case "knn_search":
		collection, _ := args["collection"].(string)
		k := uint32(5)
		if kFloat, ok := args["k"].(float64); ok {
			k = uint32(kFloat)
		}
		if h.embedFunc == nil {
			return map[string]interface{}{"error": "EmbedFunc nao configurado — necessario para KNN"}, nil
		}
		queryText, _ := args["query"].(string)
		if queryText == "" {
			return map[string]interface{}{"error": "Informe o texto de busca (query)"}, nil
		}
		vec32, err := h.embedFunc(ctx, queryText)
		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("Erro ao gerar embedding: %v", err)}, nil
		}
		vec64 := make([]float64, len(vec32))
		for i, v := range vec32 {
			vec64[i] = float64(v)
		}
		results, err := h.nietzscheClient.KnnSearch(ctx, collection, vec64, k)
		if err != nil {
			return map[string]interface{}{"status": "erro", "error": fmt.Sprintf("KnnSearch falhou: %v", err)}, nil
		}
		var rows []map[string]interface{}
		for _, r := range results {
			rows = append(rows, map[string]interface{}{
				"id":       r.ID,
				"distance": r.Distance,
			})
		}
		if rows == nil {
			rows = []map[string]interface{}{}
		}
		return map[string]interface{}{
			"status":     "sucesso",
			"collection": collection,
			"rows":       rows,
			"count":      len(rows),
			"message":    fmt.Sprintf("KNN em '%s' retornou %d resultados.", collection, len(rows)),
		}, nil

	default:
		return map[string]interface{}{
			"error":   fmt.Sprintf("Acao '%s' nao reconhecida", action),
			"message": "Acoes disponiveis: health, stats, collections, query, get_node, insert_node, knn_search",
		}, nil
	}
}
