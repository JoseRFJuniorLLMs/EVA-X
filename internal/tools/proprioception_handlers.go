// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Proprioception Handlers — EVA cognitive self-awareness tools.
// Phase 1: brain_scan, feel_the_graph (read-only)
// Phase 2: internalize_memory (controlled write with confirmation)
// Phase 3: reorganize_thoughts, tool response awareness, latency monitoring

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	nietzsche "nietzsche-sdk"
)

// ============================================================================
// Session Write Log — Phase 2.3
// ============================================================================

var (
	sessionLogMu   sync.Mutex
	sessionLogFile *os.File
)

type SessionWriteEntry struct {
	Timestamp  string  `json:"timestamp"`
	Collection string  `json:"collection"`
	Content    string  `json:"content"`
	Valence    float64 `json:"valence"`
	NodeID     string  `json:"node_id"`
	SessionID  string  `json:"session_id"`
	Confirmed  bool    `json:"confirmed"`
}

func logSessionWrite(entry SessionWriteEntry) {
	sessionLogMu.Lock()
	defer sessionLogMu.Unlock()

	if sessionLogFile == nil {
		var err error
		sessionLogFile, err = os.OpenFile("session_writes.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[PROPRIOCEPTION] Failed to open session_writes.jsonl: %v", err)
			return
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[PROPRIOCEPTION] Failed to marshal session write: %v", err)
		return
	}
	sessionLogFile.Write(append(data, '\n'))
}

// ============================================================================
// Latency Monitoring — Phase 3.3
// ============================================================================

var (
	latencyMu  sync.Mutex
	latencyLog = make(map[string][]float64)
)

const (
	latencyMaxEntries = 100
	latencyAlertMs    = 500.0
)

// RecordToolLatency records the latency of a tool call and alerts if above threshold.
func RecordToolLatency(toolName string, elapsedMs float64) {
	latencyMu.Lock()
	defer latencyMu.Unlock()

	entries := latencyLog[toolName]
	entries = append(entries, elapsedMs)
	if len(entries) > latencyMaxEntries {
		entries = entries[1:]
	}
	latencyLog[toolName] = entries

	if elapsedMs > latencyAlertMs {
		log.Printf("[LATENCY-ALERT] %s: %.0fms (threshold: %.0fms)", toolName, elapsedMs, latencyAlertMs)
	}
}

// GetLatencyReport returns latency statistics for all monitored tools.
func GetLatencyReport() map[string]interface{} {
	latencyMu.Lock()
	defer latencyMu.Unlock()

	report := make(map[string]interface{})
	for tool, entries := range latencyLog {
		if len(entries) == 0 {
			continue
		}
		var sum float64
		maxVal := 0.0
		above := 0
		for _, e := range entries {
			sum += e
			if e > maxVal {
				maxVal = e
			}
			if e > latencyAlertMs {
				above++
			}
		}
		avg := sum / float64(len(entries))
		report[tool] = map[string]interface{}{
			"count":           len(entries),
			"avg_ms":          fmt.Sprintf("%.1f", avg),
			"max_ms":          fmt.Sprintf("%.1f", maxVal),
			"above_threshold": above,
		}
	}
	return report
}

// ============================================================================
// 1. brain_scan — Proprioceptive snapshot
// ============================================================================

func (h *ToolsHandler) handleBrainScan(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	start := time.Now()

	if h.nietzscheClient == nil {
		return map[string]interface{}{"error": "NietzscheDB client não disponível"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get collections list via gRPC
	collections, err := h.nietzscheClient.ListCollections(ctx)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao listar collections: %v", err)}, nil
	}

	var totalNodes, totalEdges uint64
	collectionData := make([]map[string]interface{}, 0, len(collections))

	for _, col := range collections {
		totalNodes += col.NodeCount
		totalEdges += col.EdgeCount
		collectionData = append(collectionData, map[string]interface{}{
			"name":       col.Name,
			"node_count": col.NodeCount,
			"edge_count": col.EdgeCount,
			"dimension":  col.Dim,
			"metric":     col.Metric,
		})
	}

	elapsed := time.Since(start)
	RecordToolLatency("brain_scan", float64(elapsed.Milliseconds()))

	return map[string]interface{}{
		"status":            "sucesso",
		"total_nodes":       totalNodes,
		"total_edges":       totalEdges,
		"total_collections": len(collections),
		"collections":       collectionData,
		"latency_ms":        elapsed.Milliseconds(),
		"message": fmt.Sprintf("O meu grafo contém %d nós e %d edges em %d collections.",
			totalNodes, totalEdges, len(collections)),
	}, nil
}

// ============================================================================
// 2. feel_the_graph — Proprioceptive read (full-text search)
// ============================================================================

func (h *ToolsHandler) handleFeelTheGraph(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	start := time.Now()

	collection, _ := args["collection"].(string)
	query, _ := args["query"].(string)

	if collection == "" || query == "" {
		return map[string]interface{}{"error": "Informe collection e query"}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{"error": "NietzscheDB client não disponível"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Full-text search — most relevant for natural language queries
	results, err := h.nietzscheClient.FullTextSearch(ctx, query, collection, 3)
	if err != nil {
		log.Printf("[PROPRIOCEPTION] feel_the_graph full-text error: %v", err)
		results = nil
	}

	nodes := make([]map[string]interface{}, 0, 3)
	for _, r := range results {
		nodes = append(nodes, map[string]interface{}{
			"id":    r.NodeID,
			"score": r.Score,
		})
	}

	elapsed := time.Since(start)
	RecordToolLatency("feel_the_graph", float64(elapsed.Milliseconds()))

	return map[string]interface{}{
		"status":     "sucesso",
		"nodes":      nodes,
		"count":      len(nodes),
		"latency_ms": elapsed.Milliseconds(),
		"message":    fmt.Sprintf("Encontrei %d nós próximos de '%s' em %s.", len(nodes), query, collection),
	}, nil
}

// ============================================================================
// 3. internalize_memory — Controlled write with confirmation
// ============================================================================

func (h *ToolsHandler) handleInternalizeMemory(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	start := time.Now()

	content, _ := args["content"].(string)
	valenceStr, _ := args["valence"].(string)
	confirm, _ := args["confirm"].(bool)

	if content == "" {
		return map[string]interface{}{"error": "Informe o conteúdo da memória"}, nil
	}

	// Phase 2.2: Confirmation required
	if !confirm {
		return map[string]interface{}{
			"status":  "aguardando_confirmacao",
			"message": "EVA deve anunciar o que vai guardar e receber confirmação do utilizador antes de chamar esta tool com confirm=true.",
		}, nil
	}

	var valence float64
	fmt.Sscanf(valenceStr, "%f", &valence)
	if valence < -1.0 {
		valence = -1.0
	}
	if valence > 1.0 {
		valence = 1.0
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{"error": "NietzscheDB client não disponível"}, nil
	}

	collection := "eva_mind"
	sessionID := fmt.Sprintf("session_%d_%d", idosoID, time.Now().Unix())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build content map
	contentMap := map[string]interface{}{
		"text":       content,
		"valence":    valence,
		"session_id": sessionID,
		"source":     "proprioception",
		"created_at": time.Now().Format(time.RFC3339),
	}

	// Magnitude encodes valence intensity: neutral=0.3, strong=0.6
	magnitude := 0.3 + abs64(valence)*0.3
	coords := make([]float64, 128)
	coords[0] = magnitude

	result, err := h.nietzscheClient.InsertNode(ctx, nietzsche.InsertNodeOpts{
		Content:    contentMap,
		NodeType:   "Episodic",
		Energy:     0.8,
		Collection: collection,
		Coords:     coords,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro ao guardar memória: %v", err)}, nil
	}

	nodeID := result.ID

	// Phase 2.3: Log the write
	logSessionWrite(SessionWriteEntry{
		Timestamp:  time.Now().Format(time.RFC3339),
		Collection: collection,
		Content:    content,
		Valence:    valence,
		NodeID:     nodeID,
		SessionID:  sessionID,
		Confirmed:  true,
	})

	elapsed := time.Since(start)
	RecordToolLatency("internalize_memory", float64(elapsed.Milliseconds()))

	truncContent := content
	if len(truncContent) > 50 {
		truncContent = truncContent[:50]
	}
	log.Printf("[PROPRIOCEPTION] Memory internalized: %s (valence=%.2f) -> %s", truncContent, valence, nodeID)

	return map[string]interface{}{
		"status":     "sucesso",
		"node_id":    nodeID,
		"collection": collection,
		"valence":    valence,
		"session_id": sessionID,
		"latency_ms": elapsed.Milliseconds(),
		"message":    fmt.Sprintf("Memória guardada em %s com valência %.2f.", collection, valence),
	}, nil
}

// ============================================================================
// 4. reorganize_thoughts — Trigger AgencyEngine sleep
// ============================================================================

func (h *ToolsHandler) handleReorganizeThoughts(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	start := time.Now()

	targetCollection, _ := args["target_collection"].(string)
	if targetCollection == "" {
		return map[string]interface{}{"error": "Informe target_collection"}, nil
	}

	// Safety: never reorganize core collections
	coreCollections := map[string]bool{
		"eva_core":           true,
		"eva_self_knowledge": true,
		"eva_codebase":       true,
		"knowledge_galaxies": true,
	}
	if coreCollections[targetCollection] {
		return map[string]interface{}{
			"error": fmt.Sprintf("Não é permitido reorganizar a collection core '%s'.", targetCollection),
		}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{"error": "NietzscheDB client não disponível"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sleepResult, err := h.nietzscheClient.TriggerSleep(ctx, nietzsche.SleepOpts{
		Noise:      0.01,
		AdamSteps:  50,
		Collection: targetCollection,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Erro na reconsolidação: %v", err)}, nil
	}

	elapsed := time.Since(start)
	RecordToolLatency("reorganize_thoughts", float64(elapsed.Milliseconds()))

	return map[string]interface{}{
		"status":           "reorganizacao_concluida",
		"collection":       targetCollection,
		"hausdorff_before": sleepResult.HausdorffBefore,
		"hausdorff_after":  sleepResult.HausdorffAfter,
		"hausdorff_delta":  sleepResult.HausdorffDelta,
		"nodes_perturbed":  sleepResult.NodesPerturbed,
		"committed":        sleepResult.Committed,
		"latency_ms":       elapsed.Milliseconds(),
		"message": fmt.Sprintf("Reconsolidação de '%s' concluída. Hausdorff: %.4f → %.4f (Δ%.4f). %d nós perturbados.",
			targetCollection, sleepResult.HausdorffBefore, sleepResult.HausdorffAfter, sleepResult.HausdorffDelta, sleepResult.NodesPerturbed),
	}, nil
}

// ============================================================================
// Phase 3.1: Tool Response Awareness — inject tool results back into context
// ============================================================================

// FormatToolResponse creates a natural-language summary of a tool call result
// for injection into the EVA session context. EVA can then comment on the
// result in voice: "Adicionei 1500 nós, o grafo aqueceu ligeiramente."
func FormatToolResponse(toolName string, result map[string]interface{}, elapsedMs int64) string {
	switch toolName {
	case "brain_scan":
		totalNodes := result["total_nodes"]
		totalEdges := result["total_edges"]
		nCols := result["total_collections"]
		return fmt.Sprintf("[Resultado brain_scan: %v nós, %v edges em %v collections. Latência: %dms]",
			totalNodes, totalEdges, nCols, elapsedMs)

	case "feel_the_graph":
		count := result["count"]
		msg, _ := result["message"].(string)
		return fmt.Sprintf("[Resultado feel_the_graph: %v nós encontrados. %s Latência: %dms]",
			count, msg, elapsedMs)

	case "internalize_memory":
		nodeID, _ := result["node_id"].(string)
		valence := result["valence"]
		return fmt.Sprintf("[Memória internalizada: node=%s, valência=%v. Latência: %dms]",
			nodeID, valence, elapsedMs)

	case "reorganize_thoughts":
		col, _ := result["collection"].(string)
		nodes := result["nodes_perturbed"]
		return fmt.Sprintf("[Reconsolidação de '%s' concluída: %v nós perturbados. Latência: %dms]",
			col, nodes, elapsedMs)

	default:
		if msg, ok := result["message"].(string); ok {
			return fmt.Sprintf("[Resultado %s: %s. Latência: %dms]", toolName, msg, elapsedMs)
		}
		return fmt.Sprintf("[Resultado %s: OK. Latência: %dms]", toolName, elapsedMs)
	}
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
