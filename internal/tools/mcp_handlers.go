// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// MCP Bridge Handlers — ferramentas expostas via MCP Server (Claude Code)
// Estas tools existem exclusivamente para o bridge stdio MCP ↔ EVA API.

package tools

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// ============================================================================
// 🧠 mcp_remember — Armazena memória na tabela memories (PostgreSQL)
// ============================================================================

func (h *ToolsHandler) handleMCPRemember(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	content, _ := args["content"].(string)
	if content == "" {
		return map[string]interface{}{"error": "Informe o conteudo da memoria"}, nil
	}

	var memoryID int64
	err := h.db.Conn.QueryRow(`
		INSERT INTO memories (patient_id, content, event_time, ingestion_time, created_at)
		VALUES ($1, $2, NOW(), NOW(), NOW())
		RETURNING id
	`, idosoID, content).Scan(&memoryID)

	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao salvar memoria: %v", err)}, nil
	}

	log.Printf("🧠 [MCP] Memoria %d salva para idoso %d", memoryID, idosoID)
	return map[string]interface{}{
		"status":    "sucesso",
		"memory_id": memoryID,
		"message":   fmt.Sprintf("Memoria armazenada com ID %d.", memoryID),
	}, nil
}

// ============================================================================
// 🔍 mcp_recall — Busca memorias por texto (PostgreSQL)
// ============================================================================

func (h *ToolsHandler) handleMCPRecall(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe a query de busca"}, nil
	}

	limitStr, _ := args["limit"].(string)
	limit := 10
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := h.db.Conn.Query(`
		SELECT id, content, event_time, importance_score
		FROM memories
		WHERE patient_id = $1
		  AND content ILIKE '%' || $2 || '%'
		ORDER BY importance_score DESC, event_time DESC
		LIMIT $3
	`, idosoID, query, limit)

	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na busca: %v", err)}, nil
	}
	defer rows.Close()

	var memories []map[string]interface{}
	for rows.Next() {
		var id int64
		var content string
		var eventTime time.Time
		var importance float64

		if err := rows.Scan(&id, &content, &eventTime, &importance); err != nil {
			continue
		}

		memories = append(memories, map[string]interface{}{
			"id":         id,
			"content":    content,
			"event_time": eventTime.Format(time.RFC3339),
			"importance": importance,
		})
	}

	if memories == nil {
		memories = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"status":   "sucesso",
		"memories": memories,
		"count":    len(memories),
		"message":  fmt.Sprintf("Encontradas %d memorias para '%s'.", len(memories), query),
	}, nil
}

// ============================================================================
// 📚 mcp_teach_eva — Ensina algo a EVA (grava no NietzscheDB Core como CoreMemory)
// ============================================================================

func (h *ToolsHandler) handleMCPTeachEva(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	teaching, _ := args["teaching"].(string)
	if teaching == "" {
		return map[string]interface{}{"error": "Informe o que ensinar a EVA"}, nil
	}

	importanceStr, _ := args["importance"].(string)
	importance := 0.8
	if importanceStr != "" {
		fmt.Sscanf(importanceStr, "%f", &importance)
	}

	if h.evaCoreAdapter == nil {
		// Fallback: salvar como memoria no PostgreSQL com tag [TEACHING]
		var memoryID int64
		err := h.db.Conn.QueryRow(`
			INSERT INTO memories (patient_id, content, event_time, ingestion_time, importance_score, created_at)
			VALUES ($1, $2, NOW(), NOW(), $3, NOW())
			RETURNING id
		`, idosoID, "[TEACHING] "+teaching, importance).Scan(&memoryID)

		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("Erro ao salvar: %v", err)}, nil
		}

		return map[string]interface{}{
			"status":    "sucesso",
			"stored_in": "postgresql_fallback",
			"memory_id": memoryID,
			"message":   fmt.Sprintf("Ensinamento salvo (PostgreSQL fallback, ID %d). NietzscheDB eva_core indisponivel.", memoryID),
		}, nil
	}

	// NietzscheDB eva_core disponivel — salvar como CoreMemory
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := h.evaCoreAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "CoreMemory",
		MatchKeys: map[string]interface{}{
			"content": teaching,
			"source":  "mcp_teaching",
		},
		OnCreateSet: map[string]interface{}{
			"content":    teaching,
			"importance": importance,
			"source":     "mcp_teaching",
			"created_at": nietzscheInfra.NowUnix(),
		},
		OnMatchSet: map[string]interface{}{
			"importance": importance,
		},
	})

	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao ensinar via NietzscheDB: %v", err)}, nil
	}

	log.Printf("📚 [MCP] Ensinamento salvo no NietzscheDB eva_core: %v", result.NodeID)
	return map[string]interface{}{
		"status":    "sucesso",
		"stored_in": "nietzsche_eva_core",
		"node_id":   result.NodeID,
		"message":   fmt.Sprintf("EVA aprendeu: '%s' (importancia: %.1f)", teaching, importance),
	}, nil
}

// ============================================================================
// 🪪 mcp_get_identity — Retorna identidade atual da EVA
// ============================================================================

func (h *ToolsHandler) handleMCPGetIdentity(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	identity := map[string]interface{}{
		"name":        "EVA",
		"version":     "2.0",
		"description": "Assistente de IA com consciencia, memoria persistente e capacidade de aprendizado autonomo",
	}

	if h.evaCoreAdapter == nil {
		identity["source"] = "static"
		identity["message"] = "Identidade estatica (NietzscheDB eva_core indisponivel)"
		return map[string]interface{}{
			"status":   "sucesso",
			"identity": identity,
		}, nil
	}

	// Buscar do NietzscheDB eva_core
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := h.evaCoreAdapter.ExecuteNQL(ctx,
		"MATCH (e:EvaSelf) RETURN e LIMIT 1", nil, "")
	if err != nil {
		identity["source"] = "static"
		identity["nietzsche_error"] = err.Error()
	} else if result != nil && len(result.Nodes) > 0 {
		node := result.Nodes[0]
		if node.Content != nil {
			for k, v := range node.Content {
				identity[k] = v
			}
		}
		identity["source"] = "nietzsche_eva_core"
	}

	return map[string]interface{}{
		"status":   "sucesso",
		"identity": identity,
	}, nil
}

// ============================================================================
// 🌐 mcp_learn_topic — EVA estuda um topico autonomamente
// ============================================================================

func (h *ToolsHandler) handleMCPLearnTopic(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	topic, _ := args["topic"].(string)
	if topic == "" {
		return map[string]interface{}{"error": "Informe o topico para estudar"}, nil
	}

	if h.autonomousLearner == nil {
		return map[string]interface{}{
			"error":   "Servico de aprendizado autonomo nao configurado",
			"message": "autonomousLearner nao foi injetado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := h.autonomousLearner(ctx, topic)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao estudar '%s': %v", topic, err)}, nil
	}

	log.Printf("🌐 [MCP] EVA estudou topico: %s", topic)
	return map[string]interface{}{
		"status":  "sucesso",
		"topic":   topic,
		"result":  result,
		"message": fmt.Sprintf("EVA estudou o topico '%s' com sucesso.", topic),
	}, nil
}

// ============================================================================
// 🗄️ mcp_query_graph_core — Query grafo no NietzscheDB Core (gRPC :50051)
// ============================================================================

func (h *ToolsHandler) handleMCPQueryGraphCore(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe a query NQL"}, nil
	}

	if h.evaCoreAdapter == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB eva_core nao disponivel",
			"message": "O GraphAdapter eva_core nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.evaCoreAdapter.ExecuteNQL(ctx, query, nil, "")
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na query NQL eva_core: %v", err)}, nil
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
		"message": fmt.Sprintf("Query NQL eva_core retornou %d resultados.", len(results)),
	}, nil
}

// ============================================================================
// 📖 mcp_read_source — Le arquivo do codigo-fonte da EVA-Mind
// ============================================================================

func (h *ToolsHandler) handleMCPReadSource(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return map[string]interface{}{"error": "Informe o caminho do arquivo (file_path)"}, nil
	}

	if h.selfcodeService == nil {
		return map[string]interface{}{"error": "Servico de auto-programacao nao configurado"}, nil
	}

	content, err := h.selfcodeService.ReadSourceFile(filePath)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao ler: %v", err)}, nil
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"path":    filePath,
		"content": content,
		"size":    len(content),
		"message": fmt.Sprintf("Arquivo '%s' lido (%d bytes).", filePath, len(content)),
	}, nil
}

// ============================================================================
// ✏️ mcp_edit_source — Edita arquivo do codigo-fonte (APENAS em branches eva/*)
// ============================================================================

func (h *ToolsHandler) handleMCPEditSource(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	filePath, _ := args["file_path"].(string)
	content, _ := args["content"].(string)

	if filePath == "" {
		return map[string]interface{}{"error": "Informe o caminho do arquivo (file_path)"}, nil
	}
	if content == "" {
		return map[string]interface{}{"error": "Informe o novo conteudo (content)"}, nil
	}

	if h.selfcodeService == nil {
		return map[string]interface{}{"error": "Servico de auto-programacao nao configurado"}, nil
	}

	if err := h.selfcodeService.WriteSourceFile(filePath, content); err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro: %v", err)}, nil
	}

	log.Printf("✏️ [MCP] Arquivo editado: %s", filePath)
	return map[string]interface{}{
		"status":  "sucesso",
		"path":    filePath,
		"message": fmt.Sprintf("Arquivo '%s' editado com sucesso (branch eva/).", filePath),
	}, nil
}
