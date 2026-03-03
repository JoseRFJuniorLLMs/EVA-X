// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Phase V — Cognitive Interference handlers.
// 4 new tools that bridge geodesic navigation with EVA reasoning:
//   1. nietzsche_hydrate_path     — fetches full content for path node IDs
//   2. nietzsche_geodesic_coherence — GCS (Geodesic Coherence Score)
//   3. nietzsche_persist_synthesis — saves EVA synthesis as a new node
//   4. nietzsche_curvature_anomalies — detects sparse high-energy zones

package tools

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	nietzsche "nietzsche-sdk"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// ============================================================================
// 11. nietzsche_hydrate_path — Fetch content for each node in a geodesic path
// ============================================================================

func (h *ToolsHandler) handleHydratePath(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	// Accept path_ids as []interface{} (JSON array of strings)
	rawIDs, ok := args["path_ids"].([]interface{})
	if !ok || len(rawIDs) == 0 {
		return map[string]interface{}{"error": "Informe path_ids (array de UUIDs)"}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error":   "NietzscheDB client nao disponivel",
			"message": "O nietzscheClient nao foi configurado no ToolsHandler",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	nodes := make([]map[string]interface{}, 0, len(rawIDs))
	var failedIDs []string

	for _, raw := range rawIDs {
		id, ok := raw.(string)
		if !ok || id == "" {
			continue
		}

		nr, err := h.nietzscheClient.GetNode(ctx, id, collection)
		if err != nil || !nr.Found {
			failedIDs = append(failedIDs, id)
			continue
		}

		nodeMap := nietzscheInfra.NodeResultToMap(nr)
		nodes = append(nodes, nodeMap)
	}

	log.Printf("[NIETZSCHE] HydratePath: %d/%d nodes hydrated (collection=%s)",
		len(nodes), len(rawIDs), collection)

	return map[string]interface{}{
		"status":      "sucesso",
		"nodes":       nodes,
		"total":       len(nodes),
		"failed_ids":  failedIDs,
		"collection":  collection,
		"message": fmt.Sprintf("Hydratacao completa: %d/%d nodes carregados.",
			len(nodes), len(rawIDs)),
	}, nil
}

// ============================================================================
// 12. nietzsche_geodesic_coherence — Geodesic Coherence Score (GCS)
// ============================================================================
//
// GCS = harmonic mean of:
//   - Semantic Continuity (SC): average cosine similarity between consecutive embeddings
//   - Radial Stability  (RS): 1 − normalised variance of depth along the path
//
// Also detects "ruptures" where consecutive cosine similarity drops below threshold.

func (h *ToolsHandler) handleGeodesicCoherence(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	rawIDs, ok := args["path_ids"].([]interface{})
	if !ok || len(rawIDs) < 2 {
		return map[string]interface{}{"error": "Informe path_ids com pelo menos 2 UUIDs"}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error": "NietzscheDB client nao disponivel",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Fetch all nodes in the path
	type nodeInfo struct {
		id        string
		embedding []float64
		depth     float32
	}
	pathNodes := make([]nodeInfo, 0, len(rawIDs))

	for _, raw := range rawIDs {
		id, ok := raw.(string)
		if !ok || id == "" {
			continue
		}
		nr, err := h.nietzscheClient.GetNode(ctx, id, collection)
		if err != nil || !nr.Found {
			continue
		}
		pathNodes = append(pathNodes, nodeInfo{
			id:        nr.ID,
			embedding: nr.Embedding,
			depth:     nr.Depth,
		})
	}

	if len(pathNodes) < 2 {
		return map[string]interface{}{
			"error": "Menos de 2 nodes validos encontrados no path",
		}, nil
	}

	// --- Semantic Continuity (SC) ---
	cosineThreshold := 0.5
	if ct, ok := args["cosine_threshold"].(float64); ok && ct > 0 {
		cosineThreshold = ct
	}

	var sumCos float64
	pairCount := 0
	ruptures := make([]map[string]interface{}, 0)

	for i := 0; i < len(pathNodes)-1; i++ {
		a := pathNodes[i]
		b := pathNodes[i+1]

		cos := cosineSimilarity(a.embedding, b.embedding)
		sumCos += cos
		pairCount++

		if cos < cosineThreshold {
			ruptures = append(ruptures, map[string]interface{}{
				"from_id":    a.id,
				"to_id":      b.id,
				"cosine":     math.Round(cos*10000) / 10000,
				"position":   i,
				"diagnostic": fmt.Sprintf("Ruptura semantica entre posicao %d e %d (cos=%.4f < %.2f)", i, i+1, cos, cosineThreshold),
			})
		}
	}

	sc := 0.0
	if pairCount > 0 {
		sc = sumCos / float64(pairCount)
	}

	// --- Radial Stability (RS) ---
	var sumDepth, sumDepthSq float64
	for _, n := range pathNodes {
		d := float64(n.depth)
		sumDepth += d
		sumDepthSq += d * d
	}
	n := float64(len(pathNodes))
	meanDepth := sumDepth / n
	variance := (sumDepthSq / n) - (meanDepth * meanDepth)
	if variance < 0 {
		variance = 0
	}
	// Normalise: rs = 1 - stddev/maxPossibleStddev
	// Max depth in Poincare ball = ~7 (practical), use 5 as normalisation constant
	stddev := math.Sqrt(variance)
	rs := 1.0 - math.Min(stddev/5.0, 1.0)

	// --- GCS = harmonic mean ---
	gcs := 0.0
	if sc+rs > 0 {
		gcs = 2.0 * sc * rs / (sc + rs)
	}

	log.Printf("[NIETZSCHE] GCS path(%d nodes): SC=%.4f RS=%.4f GCS=%.4f ruptures=%d",
		len(pathNodes), sc, rs, gcs, len(ruptures))

	return map[string]interface{}{
		"status":               "sucesso",
		"gcs":                  math.Round(gcs*10000) / 10000,
		"semantic_continuity":  math.Round(sc*10000) / 10000,
		"radial_stability":    math.Round(rs*10000) / 10000,
		"path_length":         len(pathNodes),
		"mean_depth":          math.Round(meanDepth*10000) / 10000,
		"depth_stddev":        math.Round(stddev*10000) / 10000,
		"ruptures":            ruptures,
		"rupture_count":       len(ruptures),
		"cosine_threshold":    cosineThreshold,
		"message": fmt.Sprintf("GCS=%.4f (SC=%.4f, RS=%.4f). %d rupturas detectadas em %d nodes.",
			gcs, sc, rs, len(ruptures), len(pathNodes)),
	}, nil
}

// ============================================================================
// 13. nietzsche_persist_synthesis — Save EVA synthesis as a new NietzscheDB node
// ============================================================================

func (h *ToolsHandler) handlePersistSynthesis(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	synthesisText, _ := args["synthesis_text"].(string)
	if synthesisText == "" {
		return map[string]interface{}{"error": "Informe synthesis_text"}, nil
	}

	// Source node IDs that originated this synthesis
	rawSourceIDs, _ := args["source_ids"].([]interface{})
	if len(rawSourceIDs) == 0 {
		return map[string]interface{}{"error": "Informe source_ids (UUIDs dos nodes originais)"}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error": "NietzscheDB client nao disponivel",
		}, nil
	}

	if h.embedFunc == nil {
		return map[string]interface{}{
			"error": "embedFunc nao configurada — impossivel gerar embedding da sintese",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Generate embedding for the synthesis text
	embedding, err := h.embedFunc(ctx, synthesisText)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Erro ao gerar embedding: %v", err),
		}, nil
	}

	// Convert []float32 → []float64 and project into Poincare ball (‖x‖ < 1.0)
	coords := make([]float64, len(embedding))
	var normSq float64
	for i, v := range embedding {
		coords[i] = float64(v)
		normSq += coords[i] * coords[i]
	}
	norm := math.Sqrt(normSq)

	// Target radius in Poincare ball (default 0.7 = moderate depth)
	targetRadius := 0.7
	if tr, ok := args["target_radius"].(float64); ok && tr > 0 && tr < 1.0 {
		targetRadius = tr
	}

	// Project: normalize to unit vector, then scale to target radius
	if norm > 0 {
		scale := targetRadius / norm
		for i := range coords {
			coords[i] *= scale
		}
	}

	// 2. Build content map
	contentMap := map[string]interface{}{
		"synthesis":  synthesisText,
		"node_label": "Synthesis",
		"source_count": len(rawSourceIDs),
		"idoso_id":   idosoID,
		"generated":  time.Now().UTC().Format(time.RFC3339),
	}
	// Optional metadata
	if title, ok := args["title"].(string); ok && title != "" {
		contentMap["title"] = title
	}
	if context_, ok := args["context"].(string); ok && context_ != "" {
		contentMap["context"] = context_
	}

	// Energy for synthesis (default 0.85)
	energy := float32(0.85)
	if e, ok := args["energy"].(float64); ok && e > 0 {
		energy = float32(e)
	}

	// 3. Insert synthesis node
	insertOpts := nietzsche.InsertNodeOpts{
		Coords:     coords,
		Content:    contentMap,
		NodeType:   "Semantic",
		Energy:     energy,
		Collection: collection,
	}

	nr, err := h.nietzscheClient.InsertNode(ctx, insertOpts)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Erro ao inserir synthesis node: %v", err),
		}, nil
	}

	// 4. Create GENERATED_BY edges from synthesis → each source node
	edgesCreated := 0
	var edgeErrors []string

	for _, raw := range rawSourceIDs {
		sourceID, ok := raw.(string)
		if !ok || sourceID == "" {
			continue
		}

		edgeOpts := nietzsche.InsertEdgeOpts{
			From:       nr.ID,
			To:         sourceID,
			EdgeType:   "Association",
			Weight:     0.9,
			Collection: collection,
		}

		_, err := h.nietzscheClient.InsertEdge(ctx, edgeOpts)
		if err != nil {
			edgeErrors = append(edgeErrors, fmt.Sprintf("%s: %v", sourceID, err))
		} else {
			edgesCreated++
		}
	}

	log.Printf("[NIETZSCHE] PersistSynthesis: node=%s edges=%d/%d collection=%s",
		nr.ID, edgesCreated, len(rawSourceIDs), collection)

	result := map[string]interface{}{
		"status":        "sucesso",
		"synthesis_id":  nr.ID,
		"edges_created": edgesCreated,
		"edge_total":    len(rawSourceIDs),
		"energy":        energy,
		"embedding_dim": len(coords),
		"collection":    collection,
		"message": fmt.Sprintf("Sintese persistida: node %s com %d edges GENERATED_BY.",
			nr.ID, edgesCreated),
	}

	if len(edgeErrors) > 0 {
		result["edge_errors"] = edgeErrors
	}

	return result, nil
}

// ============================================================================
// 14. nietzsche_curvature_anomalies — Detect sparse high-energy zones
// ============================================================================
//
// Partitions the Poincare disk into angular×radial zones and identifies
// zones with high average energy but few nodes (anomalies).
// Uses NQL to query all nodes in the collection and analyses their
// coordinates + energy locally.

func (h *ToolsHandler) handleCurvatureAnomalies(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return map[string]interface{}{"error": "Informe a collection"}, nil
	}

	if h.nietzscheClient == nil {
		return map[string]interface{}{
			"error": "NietzscheDB client nao disponivel",
		}, nil
	}

	// Configurable parameters
	angularBins := 8
	if ab, ok := args["angular_bins"].(float64); ok && ab >= 4 {
		angularBins = int(ab)
	}
	radialBins := 3
	if rb, ok := args["radial_bins"].(float64); ok && rb >= 2 {
		radialBins = int(rb)
	}
	// Energy threshold for anomaly (default: 0.7)
	energyThreshold := 0.7
	if et, ok := args["energy_threshold"].(float64); ok && et > 0 {
		energyThreshold = et
	}
	// Max nodes per zone to be considered "sparse" (default: 3)
	sparseMax := 3
	if sm, ok := args["sparse_max"].(float64); ok && sm >= 1 {
		sparseMax = int(sm)
	}
	// Limit NQL results
	limit := 500
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query all nodes (with energy > 0) via NQL
	nql := fmt.Sprintf("MATCH (n:Semantic) RETURN n LIMIT %d", limit)
	qr, err := h.nietzscheClient.Query(ctx, nql, nil, collection)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Erro NQL: %v", err),
		}, nil
	}

	if len(qr.Nodes) == 0 {
		return map[string]interface{}{
			"status":  "sucesso",
			"message": "Nenhum node encontrado na collection.",
		}, nil
	}

	// Zone tracking
	type zone struct {
		totalEnergy float64
		count       int
		nodeIDs     []string
	}
	totalZones := angularBins * radialBins
	zones := make([]zone, totalZones)

	// Radial boundaries (Poincare ball radius < 1)
	radialBoundaries := make([]float64, radialBins+1)
	for i := 0; i <= radialBins; i++ {
		radialBoundaries[i] = float64(i) / float64(radialBins)
	}

	for _, nr := range qr.Nodes {
		if len(nr.Embedding) < 2 {
			continue
		}

		x, y := nr.Embedding[0], nr.Embedding[1]
		r := math.Sqrt(x*x + y*y)
		if r >= 1.0 {
			r = 0.999 // clamp to Poincare disk
		}

		// Determine angular bin
		theta := math.Atan2(y, x)
		if theta < 0 {
			theta += 2 * math.Pi
		}
		aBin := int(theta / (2 * math.Pi) * float64(angularBins))
		if aBin >= angularBins {
			aBin = angularBins - 1
		}

		// Determine radial bin
		rBin := radialBins - 1
		for j := 0; j < radialBins; j++ {
			if r < radialBoundaries[j+1] {
				rBin = j
				break
			}
		}

		idx := aBin*radialBins + rBin
		if idx >= totalZones {
			idx = totalZones - 1
		}
		zones[idx].totalEnergy += float64(nr.Energy)
		zones[idx].count++
		if zones[idx].count <= 5 { // keep max 5 sample IDs per zone
			zones[idx].nodeIDs = append(zones[idx].nodeIDs, nr.ID)
		}
	}

	// Find anomalies: sparse zones with high average energy
	anomalies := make([]map[string]interface{}, 0)

	for i, z := range zones {
		if z.count == 0 || z.count > sparseMax {
			continue
		}
		avgEnergy := z.totalEnergy / float64(z.count)
		if avgEnergy < energyThreshold {
			continue
		}

		aBin := i / radialBins
		rBin := i % radialBins

		anomalies = append(anomalies, map[string]interface{}{
			"zone_index":      i,
			"angular_bin":     aBin,
			"radial_bin":      rBin,
			"angular_range":   fmt.Sprintf("[%.0f, %.0f)", float64(aBin)*360.0/float64(angularBins), float64(aBin+1)*360.0/float64(angularBins)),
			"radial_range":    fmt.Sprintf("[%.2f, %.2f)", radialBoundaries[rBin], radialBoundaries[rBin+1]),
			"node_count":      z.count,
			"avg_energy":      math.Round(avgEnergy*10000) / 10000,
			"sample_node_ids": z.nodeIDs,
			"diagnostic": fmt.Sprintf("Zona angular[%d] radial[%d]: %d nodes com energia media %.4f — regiao esparsa de alta energia",
				aBin, rBin, z.count, avgEnergy),
		})
	}

	log.Printf("[NIETZSCHE] CurvatureAnomalies: %d anomalias em %d zones (collection=%s, %d nodes analisados)",
		len(anomalies), totalZones, collection, len(qr.Nodes))

	return map[string]interface{}{
		"status":           "sucesso",
		"anomalies":        anomalies,
		"anomaly_count":    len(anomalies),
		"total_zones":      totalZones,
		"angular_bins":     angularBins,
		"radial_bins":      radialBins,
		"nodes_analyzed":   len(qr.Nodes),
		"energy_threshold": energyThreshold,
		"sparse_max":       sparseMax,
		"message": fmt.Sprintf("Detetadas %d anomalias de curvatura em %d zones (%d nodes analisados).",
			len(anomalies), totalZones, len(qr.Nodes)),
	}, nil
}

// ============================================================================
// Helpers
// ============================================================================

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if either vector is empty or zero-norm.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	var dot, normA, normB float64
	for i := 0; i < minLen; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
