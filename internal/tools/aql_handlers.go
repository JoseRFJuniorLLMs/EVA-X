// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AQL Tool Handlers — expose AQL (Agent Query Language) cognitive verbs
// as EVA voice/text tools. These handlers bridge EVA's tool system with
// the AQL executor, allowing the LLM to express cognitive intent directly.
//
// TODO: The handler signature func(idosoID, args) does not carry a context.Context.
// All handlers create context.WithTimeout(context.Background(), ...) as a workaround.
// Ideally, the tool dispatch layer should propagate the caller's context so that
// cancellation (e.g. client disconnect) is respected. Until then, each handler
// enforces its own timeout to prevent unbounded gRPC calls.

package tools

import (
	"context"
	"fmt"
	"time"

	"eva/internal/cortex/aql"
)

// ============================================================================
// aql_execute — Execute raw AQL statement
// ============================================================================

func (h *ToolsHandler) handleAqlExecute(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{
			"error":   "AQL executor nao disponivel",
			"message": "O AQL executor nao foi configurado no ToolsHandler. Configure via SetAqlExecutor().",
		}, nil
	}

	statement, _ := args["statement"].(string)
	if statement == "" {
		return map[string]interface{}{
			"error":   "Informe o statement AQL",
			"message": "Exemplo: RECALL \"quantum physics\" LIMIT 5",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.ExecuteRaw(ctx, statement)
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("Erro AQL: %v", err),
			"message": "Verifique a sintaxe do statement AQL",
		}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// aql_recall — Semantic memory retrieval
// ============================================================================

func (h *ToolsHandler) handleAqlRecall(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{"error": "AQL executor nao disponivel"}, nil
	}

	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe o texto de busca (query)"}, nil
	}

	collection, _ := args["collection"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.Execute(ctx, &aql.Statement{
		Verb:       aql.VerbRecall,
		Query:      query,
		Collection: collection,
		Limit:      limit,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("RECALL falhou: %v", err)}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// aql_imprint — Write new knowledge
// ============================================================================

func (h *ToolsHandler) handleAqlImprint(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{"error": "AQL executor nao disponivel"}, nil
	}

	content, _ := args["content"].(string)
	if content == "" {
		return map[string]interface{}{"error": "Informe o conteudo a imprimir (content)"}, nil
	}

	collection, _ := args["collection"].(string)
	epistemicType, _ := args["epistemic_type"].(string)
	linkTo, _ := args["link_to"].(string)

	energy := float32(0)
	if e, ok := args["energy"].(float64); ok && e > 0 {
		energy = float32(e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.Execute(ctx, &aql.Statement{
		Verb:       aql.VerbImprint,
		Content:    content,
		Collection: collection,
		Epistemic:  aql.EpistemicType(epistemicType),
		Energy:     energy,
		LinkTo:     linkTo,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("IMPRINT falhou: %v", err)}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// aql_associate — Create/reinforce connections
// ============================================================================

func (h *ToolsHandler) handleAqlAssociate(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{"error": "AQL executor nao disponivel"}, nil
	}

	from, _ := args["from"].(string)
	to, _ := args["to"].(string)
	if from == "" || to == "" {
		return map[string]interface{}{"error": "Informe from e to (UUIDs dos nodes)"}, nil
	}

	collection, _ := args["collection"].(string)
	edgeType, _ := args["edge_type"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.Execute(ctx, &aql.Statement{
		Verb:       aql.VerbAssociate,
		From:       from,
		To:         to,
		Collection: collection,
		EdgeType:   edgeType,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("ASSOCIATE falhou: %v", err)}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// aql_trace — Path finding between nodes
// ============================================================================

func (h *ToolsHandler) handleAqlTrace(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{"error": "AQL executor nao disponivel"}, nil
	}

	from, _ := args["from"].(string)
	to, _ := args["to"].(string)
	if from == "" || to == "" {
		return map[string]interface{}{"error": "Informe from e to (UUIDs dos nodes)"}, nil
	}

	collection, _ := args["collection"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.Execute(ctx, &aql.Statement{
		Verb:       aql.VerbTrace,
		From:       from,
		To:         to,
		Collection: collection,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("TRACE falhou: %v", err)}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// aql_resonate — Wave diffusion / activation spreading
// ============================================================================

func (h *ToolsHandler) handleAqlResonate(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{"error": "AQL executor nao disponivel"}, nil
	}

	query, _ := args["query"].(string)
	if query == "" {
		return map[string]interface{}{"error": "Informe o texto semente (query)"}, nil
	}

	collection, _ := args["collection"].(string)
	depth := 3
	if d, ok := args["depth"].(float64); ok && d > 0 {
		depth = int(d)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.Execute(ctx, &aql.Statement{
		Verb:       aql.VerbResonate,
		Query:      query,
		Collection: collection,
		Depth:      depth,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("RESONATE falhou: %v", err)}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// aql_dream — Creative recombination / sleep cycle
// ============================================================================

func (h *ToolsHandler) handleAqlDream(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{"error": "AQL executor nao disponivel"}, nil
	}

	topic, _ := args["topic"].(string)
	collection, _ := args["collection"].(string)

	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.Execute(ctx, &aql.Statement{
		Verb:       aql.VerbDream,
		Topic:      topic,
		Collection: collection,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("DREAM falhou: %v", err)}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// aql_distill — Extract patterns (PageRank analysis)
// ============================================================================

func (h *ToolsHandler) handleAqlDistill(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{"error": "AQL executor nao disponivel"}, nil
	}

	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.Execute(ctx, &aql.Statement{
		Verb:       aql.VerbDistill,
		Collection: collection,
		Limit:      limit,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("DISTILL falhou: %v", err)}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// aql_fade — Intentional forgetting
// ============================================================================

func (h *ToolsHandler) handleAqlFade(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.aqlExecutor == nil {
		return map[string]interface{}{"error": "AQL executor nao disponivel"}, nil
	}

	collection, _ := args["collection"].(string)

	var nodeIDs []string
	if rawIDs, ok := args["node_ids"].([]interface{}); ok {
		for _, raw := range rawIDs {
			if id, ok := raw.(string); ok && id != "" {
				nodeIDs = append(nodeIDs, id)
			}
		}
	}
	if len(nodeIDs) == 0 {
		return map[string]interface{}{"error": "Informe node_ids para decay"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.aqlExecutor.Execute(ctx, &aql.Statement{
		Verb:       aql.VerbFade,
		NodeIDs:    nodeIDs,
		Collection: collection,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("FADE falhou: %v", err)}, nil
	}

	return aqlResultToMap(result), nil
}

// ============================================================================
// Helpers
// ============================================================================

func aqlResultToMap(result *aql.CognitiveResult) map[string]interface{} {
	if result == nil {
		return map[string]interface{}{"status": "sucesso", "nodes": []interface{}{}, "count": 0}
	}

	nodes := make([]map[string]interface{}, 0, len(result.Nodes))
	for _, n := range result.Nodes {
		node := map[string]interface{}{
			"id":        n.ID,
			"content":   n.Content,
			"node_type": n.NodeType,
			"energy":    n.Energy,
		}
		if n.Magnitude > 0 {
			node["magnitude"] = n.Magnitude
		}
		if n.Metadata != nil {
			for k, v := range n.Metadata {
				node[k] = v
			}
		}
		nodes = append(nodes, node)
	}

	edges := make([]map[string]interface{}, 0, len(result.Edges))
	for _, e := range result.Edges {
		edges = append(edges, map[string]interface{}{
			"source":    e.Source,
			"target":    e.Target,
			"edge_type": e.EdgeType,
			"weight":    e.Weight,
		})
	}

	return map[string]interface{}{
		"status":       "sucesso",
		"verb":         result.Metadata.Verb,
		"nodes":        nodes,
		"edges":        edges,
		"count":        result.Metadata.Count,
		"avg_energy":   result.Metadata.AvgEnergy,
		"execution_ms": result.Metadata.ExecutionMs,
		"side_effects": result.Metadata.SideEffects,
		"message":      fmt.Sprintf("AQL %s: %d resultados em %dms.", result.Metadata.Verb, result.Metadata.Count, result.Metadata.ExecutionMs),
	}
}
