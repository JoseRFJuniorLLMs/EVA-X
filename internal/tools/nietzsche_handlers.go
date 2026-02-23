// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Dialectical Tool Handlers — NietzscheDB multi-manifold operations exposed as
// EVA voice/text tools. These 10 handlers cover Riemannian synthesis (Hegel),
// Schrodinger probabilistic edges, heat-kernel diffusion, Riemannian sleep,
// Zaratustra autonomous evolution, Minkowski causal chains, Klein geodesics,
// and speculative dream exploration.

package tools

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzsche "nietzsche-sdk"
)

// ============================================================================
// 1. nietzsche_hegelian_synthesis — Riemannian synthesis of 2 nodes
// ============================================================================

func (h *ToolsHandler) handleHegelianSynthesis(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	nodeIDA, _ := args["node_id_a"].(string)
	nodeIDB, _ := args["node_id_b"].(string)
	collection, _ := args["collection"].(string)

	if nodeIDA == "" || nodeIDB == "" {
		return map[string]interface{}{"error": "Informe node_id_a e node_id_b"}, nil
	}
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	if h.manifoldAdapter == nil {
		return map[string]interface{}{
			"error":   "ManifoldAdapter nao disponivel",
			"message": "O ManifoldAdapter nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.manifoldAdapter.SynthesizePair(ctx, nodeIDA, nodeIDB, collection)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na sintese Riemanniana: %v", err)}, nil
	}
	if result == nil {
		return map[string]interface{}{
			"status":  "sucesso",
			"message": "Sintese retornou resultado vazio — verifique se os nodes existem na collection",
		}, nil
	}

	log.Printf("[NIETZSCHE] Hegelian synthesis %s + %s em %s -> nearest=%s dist=%.4f",
		nodeIDA, nodeIDB, collection, result.NearestNodeID, result.NearestDistance)

	return map[string]interface{}{
		"status":           "sucesso",
		"synthesis_coords": result.SynthesisCoords,
		"nearest_node":     result.NearestNodeID,
		"nearest_distance": result.NearestDistance,
		"message": fmt.Sprintf("Sintese Hegeliana concluida. Ponto medio Riemanniano proximo ao node %s (dist %.4f).",
			result.NearestNodeID, result.NearestDistance),
	}, nil
}

// ============================================================================
// 2. nietzsche_hegelian_synthesis_multi — Synthesis of 3+ nodes
// ============================================================================

func (h *ToolsHandler) handleHegelianSynthesisMulti(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	// Parse node_ids array
	var nodeIDs []string
	if rawIDs, ok := args["node_ids"].([]interface{}); ok {
		for _, raw := range rawIDs {
			if id, ok := raw.(string); ok && id != "" {
				nodeIDs = append(nodeIDs, id)
			}
		}
	}
	if len(nodeIDs) < 2 {
		return map[string]interface{}{"error": "Informe pelo menos 2 node_ids"}, nil
	}

	if h.manifoldAdapter == nil {
		return map[string]interface{}{
			"error":   "ManifoldAdapter nao disponivel",
			"message": "O ManifoldAdapter nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := h.manifoldAdapter.SynthesizeMemories(ctx, nodeIDs, collection)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na sintese multi-node: %v", err)}, nil
	}
	if result == nil {
		return map[string]interface{}{
			"status":  "sucesso",
			"message": "Sintese retornou resultado vazio — verifique se os nodes existem",
		}, nil
	}

	log.Printf("[NIETZSCHE] Hegelian synthesis multi (%d nodes) em %s -> nearest=%s",
		len(nodeIDs), collection, result.NearestNodeID)

	return map[string]interface{}{
		"status":           "sucesso",
		"node_count":       len(nodeIDs),
		"centroid_coords":  result.SynthesisCoords,
		"nearest_node":     result.NearestNodeID,
		"nearest_distance": result.NearestDistance,
		"message": fmt.Sprintf("Sintese Hegeliana de %d nodes concluida. Centroide proximo a %s (dist %.4f).",
			len(nodeIDs), result.NearestNodeID, result.NearestDistance),
	}, nil
}

// ============================================================================
// 3. nietzsche_schrodinger_edges — List probabilistic edges for a node
// ============================================================================

func (h *ToolsHandler) handleSchrodingerEdges(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	nodeID, _ := args["node_id"].(string)
	collection, _ := args["collection"].(string)

	if nodeID == "" {
		return map[string]interface{}{"error": "Informe o node_id"}, nil
	}
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB client nao disponivel",
			"message": "O client NietzscheDB gRPC nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Query for edges with probability metadata (Schrodinger edges)
	nql := `MATCH (n)-[r]->(m) WHERE n.id = $id AND r.probability IS NOT NULL RETURN r.id, m.id, r.probability, r.decay_rate, r.context_boost, r.edge_type`
	params := map[string]interface{}{
		"id": nodeID,
	}

	result, err := h.nietzscheClient.Query(ctx, nql, params, collection)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao buscar arestas Schrodinger: %v", err)}, nil
	}

	var edges []map[string]interface{}
	if result != nil {
		for _, row := range result.ScalarRows {
			edge := map[string]interface{}{}
			for k, v := range row {
				edge[k] = v
			}
			edges = append(edges, edge)
		}
		// Also check NodePairs for connected nodes
		for _, pair := range result.NodePairs {
			edges = append(edges, map[string]interface{}{
				"from_id": pair.From.ID,
				"to_id":   pair.To.ID,
			})
			if len(edges) >= 100 {
				break
			}
		}
	}

	if edges == nil {
		edges = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"node_id": nodeID,
		"edges":   edges,
		"count":   len(edges),
		"message": fmt.Sprintf("Encontradas %d arestas Schrodinger para node %s.", len(edges), nodeID),
	}, nil
}

// ============================================================================
// 4. nietzsche_collapse_edge — Collapse a Schrodinger edge
// ============================================================================

func (h *ToolsHandler) handleCollapseEdge(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	edgeID, _ := args["edge_id"].(string)
	collection, _ := args["collection"].(string)

	if edgeID == "" {
		return map[string]interface{}{"error": "Informe o edge_id"}, nil
	}
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB client nao disponivel",
			"message": "O client NietzscheDB gRPC nao foi configurado no ToolsHandler",
		}, nil
	}

	// Parse optional context_data
	var contextData map[string]interface{}
	if cd, ok := args["context_data"].(map[string]interface{}); ok {
		contextData = cd
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	collapsed, finalProb, err := h.nietzscheClient.SchrodingerCollapse(ctx, edgeID, collection, contextData)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao colapsar aresta: %v", err)}, nil
	}

	stateMsg := "a aresta permaneceu em superposicao"
	if collapsed {
		stateMsg = "a aresta colapsou em existencia"
	}

	log.Printf("[NIETZSCHE] SchrodingerCollapse edge=%s collapsed=%v prob=%.4f", edgeID, collapsed, finalProb)

	return map[string]interface{}{
		"status":            "sucesso",
		"edge_id":           edgeID,
		"collapsed":         collapsed,
		"final_probability": finalProb,
		"message":           fmt.Sprintf("Colapso Schrodinger concluido: %s (prob final: %.4f).", stateMsg, finalProb),
	}, nil
}

// ============================================================================
// 5. nietzsche_diffuse_memory — Heat kernel diffusion from nodes
// ============================================================================

func (h *ToolsHandler) handleDiffuseMemory(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	// Parse node_ids array
	var nodeIDs []string
	if rawIDs, ok := args["node_ids"].([]interface{}); ok {
		for _, raw := range rawIDs {
			if id, ok := raw.(string); ok && id != "" {
				nodeIDs = append(nodeIDs, id)
			}
		}
	}
	if len(nodeIDs) == 0 {
		return map[string]interface{}{"error": "Informe pelo menos 1 node_id em node_ids"}, nil
	}

	// Parse t_values (diffusion times)
	var tValues []float64
	if rawT, ok := args["t_values"].([]interface{}); ok {
		for _, raw := range rawT {
			if t, ok := raw.(float64); ok {
				tValues = append(tValues, t)
			}
		}
	}
	// Default diffusion times if none provided
	if len(tValues) == 0 {
		tValues = []float64{0.1, 1.0, 10.0}
	}

	// Parse max_hops (maps to KChebyshev polynomial order)
	kChebyshev := uint32(10)
	if mh, ok := args["max_hops"].(float64); ok && mh > 0 {
		kChebyshev = uint32(mh)
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB client nao disponivel",
			"message": "O client NietzscheDB gRPC nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scales, err := h.nietzscheClient.Diffuse(ctx, nodeIDs, nietzsche.DiffuseOpts{
		TValues:    tValues,
		KChebyshev: kChebyshev,
		Collection: collection,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na difusao: %v", err)}, nil
	}

	// Format results per temporal scale
	var scaleResults []map[string]interface{}
	for _, s := range scales {
		activations := make([]map[string]interface{}, 0, len(s.NodeIDs))
		for i, nid := range s.NodeIDs {
			score := 0.0
			if i < len(s.Scores) {
				score = s.Scores[i]
			}
			activations = append(activations, map[string]interface{}{
				"node_id": nid,
				"score":   score,
			})
		}
		scaleResults = append(scaleResults, map[string]interface{}{
			"t":           s.T,
			"activations": activations,
			"node_count":  len(s.NodeIDs),
		})
	}

	if scaleResults == nil {
		scaleResults = []map[string]interface{}{}
	}

	log.Printf("[NIETZSCHE] Diffuse %d sources em %s -> %d scales", len(nodeIDs), collection, len(scaleResults))

	return map[string]interface{}{
		"status":       "sucesso",
		"source_nodes": nodeIDs,
		"scales":       scaleResults,
		"scale_count":  len(scaleResults),
		"message":      fmt.Sprintf("Difusao termica concluida com %d escalas temporais a partir de %d nodes.", len(scaleResults), len(nodeIDs)),
	}, nil
}

// ============================================================================
// 6. nietzsche_trigger_sleep — Riemannian reconsolidation
// ============================================================================

func (h *ToolsHandler) handleTriggerSleep(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	// Parse optional perturbation_amplitude (maps to Noise)
	noise := 0.0 // 0 = server default (0.02)
	if pa, ok := args["perturbation_amplitude"].(float64); ok && pa > 0 {
		noise = pa
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB client nao disponivel",
			"message": "O client NietzscheDB gRPC nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := h.nietzscheClient.TriggerSleep(ctx, nietzsche.SleepOpts{
		Noise:      noise,
		Collection: collection,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro no ciclo de sono: %v", err)}, nil
	}

	statusMsg := "ciclo de sono NAO commitado (delta Hausdorff excedeu threshold)"
	if result.Committed {
		statusMsg = "ciclo de sono commitado com sucesso"
	}

	log.Printf("[NIETZSCHE] TriggerSleep %s: committed=%v nodes_perturbed=%d delta=%.4f",
		collection, result.Committed, result.NodesPerturbed, result.HausdorffDelta)

	return map[string]interface{}{
		"status":           "sucesso",
		"committed":        result.Committed,
		"hausdorff_before": result.HausdorffBefore,
		"hausdorff_after":  result.HausdorffAfter,
		"hausdorff_delta":  result.HausdorffDelta,
		"nodes_perturbed":  result.NodesPerturbed,
		"snapshot_nodes":   result.SnapshotNodes,
		"message":          fmt.Sprintf("Reconsolidacao Riemanniana: %s. %d nodes perturbados, delta H = %.4f.", statusMsg, result.NodesPerturbed, result.HausdorffDelta),
	}, nil
}

// ============================================================================
// 7. nietzsche_invoke_zaratustra — Autonomous evolution
// ============================================================================

func (h *ToolsHandler) handleInvokeZaratustra(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB client nao disponivel",
			"message": "O client NietzscheDB gRPC nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := h.nietzscheClient.InvokeZaratustra(ctx, nietzsche.ZaratustraOpts{
		Collection: collection,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao invocar Zaratustra: %v", err)}, nil
	}
	if result == nil {
		return map[string]interface{}{
			"status":  "sucesso",
			"message": "Zaratustra retornou resultado vazio",
		}, nil
	}

	log.Printf("[NIETZSCHE] Zaratustra %s: nodes=%d elite=%d cycles=%d duration=%dms",
		collection, result.NodesUpdated, result.EliteCount, result.CyclesRun, result.DurationMs)

	return map[string]interface{}{
		"status":             "sucesso",
		"nodes_updated":      result.NodesUpdated,
		"mean_energy_before": result.MeanEnergyBefore,
		"mean_energy_after":  result.MeanEnergyAfter,
		"total_energy_delta": result.TotalEnergyDelta,
		"echoes_created":     result.EchoesCreated,
		"echoes_evicted":     result.EchoesEvicted,
		"total_echoes":       result.TotalEchoes,
		"elite_count":        result.EliteCount,
		"elite_threshold":    result.EliteThreshold,
		"elite_node_ids":     result.EliteNodeIDs,
		"mean_elite_energy":  result.MeanEliteEnergy,
		"mean_base_energy":   result.MeanBaseEnergy,
		"duration_ms":        result.DurationMs,
		"cycles_run":         result.CyclesRun,
		"message": fmt.Sprintf("Zaratustra concluido em %dms: %d nodes atualizados, %d elite (threshold %.2f), %d echoes criados.",
			result.DurationMs, result.NodesUpdated, result.EliteCount, result.EliteThreshold, result.EchoesCreated),
	}, nil
}

// ============================================================================
// 8. nietzsche_causal_chain — Minkowski causal chain
// ============================================================================

func (h *ToolsHandler) handleCausalChain(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	nodeID, _ := args["node_id"].(string)
	collection, _ := args["collection"].(string)
	direction, _ := args["direction"].(string)

	if nodeID == "" {
		return map[string]interface{}{"error": "Informe o node_id"}, nil
	}
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}
	if direction == "" {
		direction = "past"
	}
	if direction != "past" && direction != "future" {
		return map[string]interface{}{"error": "direction deve ser 'past' ou 'future'"}, nil
	}

	// Parse max_depth (default 10)
	maxDepth := uint32(10)
	if md, ok := args["max_depth"].(float64); ok && md > 0 {
		maxDepth = uint32(md)
	}

	if h.manifoldAdapter == nil {
		return map[string]interface{}{
			"error":   "ManifoldAdapter nao disponivel",
			"message": "O ManifoldAdapter nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Use the ManifoldAdapter convenience methods or call client directly for custom depth
	result, err := h.nietzscheClient.CausalChain(ctx, nodeID, maxDepth, direction, collection)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na cadeia causal: %v", err)}, nil
	}
	if result == nil {
		return map[string]interface{}{
			"status":  "sucesso",
			"message": "Cadeia causal vazia — node pode nao ter conexoes causais",
		}, nil
	}

	// Format edges
	var edgeList []map[string]interface{}
	for _, e := range result.Edges {
		edgeList = append(edgeList, map[string]interface{}{
			"edge_id":             e.EdgeID,
			"from_node_id":        e.FromNodeID,
			"to_node_id":          e.ToNodeID,
			"minkowski_interval":  e.MinkowskiInterval,
			"causal_type":         e.CausalType,
			"edge_type":           e.EdgeType,
		})
	}
	if edgeList == nil {
		edgeList = []map[string]interface{}{}
	}

	log.Printf("[NIETZSCHE] CausalChain node=%s dir=%s -> %d chain nodes, %d edges",
		nodeID, direction, len(result.ChainIDs), len(result.Edges))

	return map[string]interface{}{
		"status":     "sucesso",
		"node_id":    nodeID,
		"direction":  direction,
		"chain_ids":  result.ChainIDs,
		"chain_len":  len(result.ChainIDs),
		"edges":      edgeList,
		"edge_count": len(edgeList),
		"message": fmt.Sprintf("Cadeia causal Minkowski (%s): %d nodes na cadeia, %d arestas timelike.",
			direction, len(result.ChainIDs), len(edgeList)),
	}, nil
}

// ============================================================================
// 9. nietzsche_klein_path — Klein geodesic path
// ============================================================================

func (h *ToolsHandler) handleKleinPath(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	fromNodeID, _ := args["from_node_id"].(string)
	toNodeID, _ := args["to_node_id"].(string)
	collection, _ := args["collection"].(string)

	if fromNodeID == "" || toNodeID == "" {
		return map[string]interface{}{"error": "Informe from_node_id e to_node_id"}, nil
	}
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	if h.manifoldAdapter == nil {
		return map[string]interface{}{
			"error":   "ManifoldAdapter nao disponivel",
			"message": "O ManifoldAdapter nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.manifoldAdapter.OptimalPath(ctx, fromNodeID, toNodeID, collection)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro no KleinPath: %v", err)}, nil
	}
	if result == nil {
		return map[string]interface{}{
			"status":  "sucesso",
			"found":   false,
			"message": "Resultado vazio do KleinPath",
		}, nil
	}

	if !result.Found {
		return map[string]interface{}{
			"status":  "sucesso",
			"found":   false,
			"message": fmt.Sprintf("Nenhuma geodesica Klein encontrada entre %s e %s.", fromNodeID, toNodeID),
		}, nil
	}

	log.Printf("[NIETZSCHE] KleinPath %s -> %s: found=%v cost=%.4f path_len=%d",
		fromNodeID, toNodeID, result.Found, result.Cost, len(result.Path))

	return map[string]interface{}{
		"status":   "sucesso",
		"found":    true,
		"path":     result.Path,
		"path_len": len(result.Path),
		"cost":     result.Cost,
		"message": fmt.Sprintf("Geodesica Klein encontrada: %d nodes no caminho, custo hiperbolico %.4f.",
			len(result.Path), result.Cost),
	}, nil
}

// ============================================================================
// 10. nietzsche_dream_explore — Speculative dream exploration
// ============================================================================

func (h *ToolsHandler) handleDreamExplore(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	seedNodeID, _ := args["seed_node_id"].(string)
	collection, _ := args["collection"].(string)

	if seedNodeID == "" {
		return map[string]interface{}{"error": "Informe o seed_node_id"}, nil
	}
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	// Parse optional depth and noise
	depth := 0 // 0 = server default (5)
	if d, ok := args["depth"].(float64); ok && d > 0 {
		depth = int(d)
	}

	noise := 0.0 // 0 = server default (0.05)
	if n, ok := args["noise"].(float64); ok && n > 0 {
		noise = n
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB client nao disponivel",
			"message": "O client NietzscheDB gRPC nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start the dream exploration
	dreamResult, err := h.nietzscheClient.StartDream(ctx, collection, seedNodeID, depth, noise)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao iniciar dream: %v", err)}, nil
	}
	if dreamResult == nil {
		return map[string]interface{}{
			"status":  "sucesso",
			"message": "Dream retornou resultado vazio",
		}, nil
	}

	// Also list pending dreams for context
	pendingDreams, err := h.nietzscheClient.ShowDreams(ctx, collection)
	if err != nil {
		log.Printf("[NIETZSCHE] ShowDreams falhou (nao-fatal): %v", err)
		pendingDreams = []map[string]interface{}{}
	}

	log.Printf("[NIETZSCHE] DreamExplore seed=%s collection=%s dream_id=%s events=%d nodes=%d",
		seedNodeID, collection, dreamResult.DreamID, len(dreamResult.Events), len(dreamResult.Nodes))

	return map[string]interface{}{
		"status":          "sucesso",
		"dream_id":        dreamResult.DreamID,
		"seed_node_id":    seedNodeID,
		"events":          dreamResult.Events,
		"event_count":     len(dreamResult.Events),
		"discovered_nodes": dreamResult.Nodes,
		"node_count":      len(dreamResult.Nodes),
		"pending_dreams":  pendingDreams,
		"message": fmt.Sprintf("Exploracao onírica iniciada (dream_id: %s). %d eventos detectados, %d nodes descobertos. Use APPLY DREAM ou REJECT DREAM para finalizar.",
			dreamResult.DreamID, len(dreamResult.Events), len(dreamResult.Nodes)),
	}, nil
}
