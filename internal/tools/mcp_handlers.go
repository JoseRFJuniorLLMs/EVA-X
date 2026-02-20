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
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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
// 📚 mcp_teach_eva — Ensina algo a EVA (grava no Neo4j Core como CoreMemory)
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

	if h.neo4jCoreDriver == nil {
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
			"message":   fmt.Sprintf("Ensinamento salvo (PostgreSQL fallback, ID %d). Neo4j Core indisponivel.", memoryID),
		}, nil
	}

	// Neo4j Core disponivel — salvar como CoreMemory
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := h.neo4jCoreDriver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			CREATE (m:CoreMemory {
				content: $content,
				importance: $importance,
				source: 'mcp_teaching',
				created_at: datetime()
			})
			RETURN id(m) AS id
		`, map[string]interface{}{
			"content":    teaching,
			"importance": importance,
		})
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, err
		}
		return record.Values[0], nil
	})

	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao ensinar via Neo4j Core: %v", err)}, nil
	}

	log.Printf("📚 [MCP] Ensinamento salvo no Neo4j Core: %v", result)
	return map[string]interface{}{
		"status":    "sucesso",
		"stored_in": "neo4j_core",
		"node_id":   result,
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

	if h.neo4jCoreDriver == nil {
		identity["source"] = "static"
		identity["message"] = "Identidade estatica (Neo4j Core indisponivel)"
		return map[string]interface{}{
			"status":   "sucesso",
			"identity": identity,
		}, nil
	}

	// Buscar do Neo4j Core
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := h.neo4jCoreDriver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	records, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			MATCH (e:EvaSelf)
			OPTIONAL MATCH (e)-[:HAS_MEMORY]->(m:CoreMemory)
			RETURN e, count(m) AS memory_count
			LIMIT 1
		`, nil)
		if err != nil {
			return nil, err
		}
		return res.Collect(ctx)
	})

	if err != nil {
		identity["source"] = "static"
		identity["neo4j_error"] = err.Error()
	} else if recs, ok := records.([]*neo4j.Record); ok && len(recs) > 0 {
		rec := recs[0]
		if node, ok := rec.Values[0].(neo4j.Node); ok {
			for k, v := range node.Props {
				identity[k] = v
			}
		}
		if count, ok := rec.Values[1].(int64); ok {
			identity["core_memories_count"] = count
		}
		identity["source"] = "neo4j_core"
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
// 🗄️ mcp_query_neo4j_core — Query Cypher no Neo4j Core (:7688)
// ============================================================================

func (h *ToolsHandler) handleMCPQueryNeo4jCore(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe a query Cypher"}, nil
	}

	// SECURITY: Apenas leitura (MATCH/RETURN)
	normalized := strings.TrimSpace(strings.ToUpper(query))
	dangerous := []string{"CREATE", "DELETE", "DETACH", "SET ", "REMOVE", "MERGE", "DROP", "CALL"}
	for _, d := range dangerous {
		if strings.Contains(normalized, d) {
			return map[string]interface{}{"error": fmt.Sprintf("Operacao '%s' nao permitida — apenas leitura", d)}, nil
		}
	}

	if h.neo4jCoreDriver == nil {
		return map[string]interface{}{
			"error":   "Neo4j Core (porta 7688) nao disponivel",
			"message": "O driver Neo4j Core nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Parse params se fornecido
	var cyParams map[string]interface{}
	if paramsStr, ok := args["params"].(string); ok && paramsStr != "" {
		// Parse JSON params string
		cyParams = map[string]interface{}{}
		// Simple: just pass nil if we can't parse (user can use inline params)
	}
	_ = cyParams

	session := h.neo4jCoreDriver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	records, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}
		return res.Collect(ctx)
	})

	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na query Neo4j Core: %v", err)}, nil
	}

	recs := records.([]*neo4j.Record)
	var results []map[string]interface{}
	for _, rec := range recs {
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
		"message": fmt.Sprintf("Query Neo4j Core retornou %d resultados.", len(results)),
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
