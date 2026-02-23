// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"fmt"
	"log"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	nietzsche "nietzsche-sdk"
)

// ConflictSynthesisService stores narrative conflicts (thesis vs antithesis)
// as opposing nodes in NietzscheDB and computes Riemannian midpoint synthesis.
//
// Lacanian contradictions between demand (what the patient says) and desire
// (what signals indicate) are persisted so future sessions can query conflict
// history. The synthesis node sits at a lower hyperbolic depth (closer to disk
// center = more abstract) and enters the prompt as a "suggested resolution".
type ConflictSynthesisService struct {
	graph    *nietzscheInfra.GraphAdapter
	manifold *nietzscheInfra.ManifoldAdapter
	client   *nietzscheInfra.Client
}

// ConflictSynthesisResult holds the outcome of a Riemannian conflict synthesis.
type ConflictSynthesisResult struct {
	ThesisNodeID     string  `json:"thesis_node_id"`
	AntithesisNodeID string  `json:"antithesis_node_id"`
	SynthesisNodeID  string  `json:"synthesis_node_id"`
	SynthesisDepth   float64 `json:"synthesis_depth"` // lower = more abstract
	NearestDistance  float64 `json:"nearest_distance"`
	PromptFragment   string  `json:"prompt_fragment"` // text for prompt enrichment
}

// NewConflictSynthesisService creates a new service. All adapters are optional;
// if nil, the service gracefully degrades (returns nil results).
func NewConflictSynthesisService(
	graph *nietzscheInfra.GraphAdapter,
	manifold *nietzscheInfra.ManifoldAdapter,
	client *nietzscheInfra.Client,
) *ConflictSynthesisService {
	return &ConflictSynthesisService{
		graph:    graph,
		manifold: manifold,
		client:   client,
	}
}

// SetGraphAdapter allows late injection of the graph adapter.
func (cs *ConflictSynthesisService) SetGraphAdapter(g *nietzscheInfra.GraphAdapter) {
	cs.graph = g
}

// SetManifoldAdapter allows late injection of the manifold adapter.
func (cs *ConflictSynthesisService) SetManifoldAdapter(m *nietzscheInfra.ManifoldAdapter) {
	cs.manifold = m
}

// SetClient allows late injection of the NietzscheDB client.
func (cs *ConflictSynthesisService) SetClient(c *nietzscheInfra.Client) {
	cs.client = c
}

// SynthesizeConflict stores a detected contradiction as thesis/antithesis nodes
// in NietzscheDB, computes the Riemannian midpoint, and returns a synthesis
// node that can enrich the prompt. Returns nil (no error) if NietzscheDB is
// unavailable -- existing behavior is preserved as fallback.
func (cs *ConflictSynthesisService) SynthesizeConflict(
	ctx context.Context,
	patientID int64,
	thesis string, // what the patient said (previous text)
	antithesis string, // what current signals indicate (current text)
	contradiction string, // the detected contradiction text from GrandAutre
) (*ConflictSynthesisResult, error) {
	// Graceful degradation: if adapters are missing, return nil (fallback to LLM-only)
	if cs.graph == nil || cs.manifold == nil {
		return nil, nil
	}

	now := nietzscheInfra.NowUnix()
	collection := "" // use graph adapter's default (patient_graph)

	// 1. MERGE Person node
	personResult, err := cs.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "Person",
		MatchKeys: map[string]interface{}{"id": patientID},
	})
	if err != nil {
		return nil, fmt.Errorf("conflict synthesis: merge Person failed: %w", err)
	}

	// 2. Store THESIS node (what the patient said)
	thesisNode, err := cs.graph.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType:   "ConflictThesis",
		Collection: collection,
		Content: map[string]interface{}{
			"patient_id":    patientID,
			"text":          thesis,
			"role":          "thesis",
			"detected_at":   now,
			"contradiction": contradiction,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("conflict synthesis: insert thesis failed: %w", err)
	}

	// 3. Store ANTITHESIS node (what signals indicate)
	antithesisNode, err := cs.graph.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType:   "ConflictAntithesis",
		Collection: collection,
		Content: map[string]interface{}{
			"patient_id":    patientID,
			"text":          antithesis,
			"role":          "antithesis",
			"detected_at":   now,
			"contradiction": contradiction,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("conflict synthesis: insert antithesis failed: %w", err)
	}

	// 4. Link Person -> Thesis and Person -> Antithesis
	cs.graph.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:     personResult.NodeID,
		To:       thesisNode.ID,
		EdgeType: "HAS_CONFLICT",
	})
	cs.graph.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:     personResult.NodeID,
		To:       antithesisNode.ID,
		EdgeType: "HAS_CONFLICT",
	})

	// 5. Compute Riemannian midpoint synthesis (Poincare ball)
	synthesisResult, err := cs.manifold.SynthesizePair(
		ctx, thesisNode.ID, antithesisNode.ID, "patient_graph",
	)
	if err != nil {
		log.Printf("[SYNTHESIS] Riemannian synthesis failed (non-fatal): %v", err)
		// Non-fatal: we still have thesis/antithesis stored
		return &ConflictSynthesisResult{
			ThesisNodeID:     thesisNode.ID,
			AntithesisNodeID: antithesisNode.ID,
			PromptFragment:   buildFallbackPromptFragment(thesis, antithesis, contradiction),
		}, nil
	}

	// 6. Store SYNTHESIS node with the midpoint coordinates
	synthesisNode, err := cs.graph.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType:   "ConflictSynthesis",
		Collection: collection,
		Content: map[string]interface{}{
			"patient_id":      patientID,
			"thesis":          thesis,
			"antithesis":      antithesis,
			"contradiction":   contradiction,
			"nearest_node_id": synthesisResult.NearestNodeID,
			"nearest_dist":    synthesisResult.NearestDistance,
			"detected_at":     now,
		},
		Coords: synthesisResult.SynthesisCoords,
	})
	if err != nil {
		log.Printf("[SYNTHESIS] Insert synthesis node failed (non-fatal): %v", err)
		return &ConflictSynthesisResult{
			ThesisNodeID:     thesisNode.ID,
			AntithesisNodeID: antithesisNode.ID,
			PromptFragment:   buildFallbackPromptFragment(thesis, antithesis, contradiction),
		}, nil
	}

	// 7. Create edges: Thesis -SYNTHESIZED_INTO-> Synthesis, Antithesis -SYNTHESIZED_INTO-> Synthesis
	cs.graph.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:     thesisNode.ID,
		To:       synthesisNode.ID,
		EdgeType: "SYNTHESIZED_INTO",
	})
	cs.graph.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:     antithesisNode.ID,
		To:       synthesisNode.ID,
		EdgeType: "SYNTHESIZED_INTO",
	})

	// 8. Read depth of synthesis node (lower = more abstract = closer to disk center)
	synthDepth := synthesisNode.Depth

	result := &ConflictSynthesisResult{
		ThesisNodeID:     thesisNode.ID,
		AntithesisNodeID: antithesisNode.ID,
		SynthesisNodeID:  synthesisNode.ID,
		SynthesisDepth:   float64(synthDepth),
		NearestDistance:  synthesisResult.NearestDistance,
		PromptFragment:   buildSynthesisPromptFragment(thesis, antithesis, contradiction, float64(synthDepth)),
	}

	log.Printf("[SYNTHESIS] Conflict synthesized for patient %d: thesis=%s antithesis=%s depth=%.3f",
		patientID, thesisNode.ID, antithesisNode.ID, synthDepth)

	return result, nil
}

// QueryConflictHistory retrieves past conflict syntheses for a patient.
// Enables cross-session continuity: "In previous sessions, we noticed..."
func (cs *ConflictSynthesisService) QueryConflictHistory(
	ctx context.Context, patientID int64, limit int,
) ([]ConflictSynthesisResult, error) {
	if cs.graph == nil {
		return nil, nil
	}

	nql := `MATCH (p:Person)-[:HAS_CONFLICT]->(t:ConflictThesis)-[:SYNTHESIZED_INTO]->(s:ConflictSynthesis)
		WHERE p.id = $patientId
		RETURN s
		ORDER BY s.detected_at DESC
		LIMIT $limit`

	qr, err := cs.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"patientId": patientID,
		"limit":     limit,
	}, "")
	if err != nil {
		return nil, fmt.Errorf("query conflict history: %w", err)
	}

	var results []ConflictSynthesisResult
	for _, node := range qr.Nodes {
		r := ConflictSynthesisResult{
			SynthesisNodeID: node.ID,
			SynthesisDepth:  float64(node.Depth),
		}
		if t, ok := node.Content["thesis"].(string); ok {
			r.PromptFragment = buildSynthesisPromptFragment(
				t,
				nodeContentStr(node.Content, "antithesis"),
				nodeContentStr(node.Content, "contradiction"),
				float64(node.Depth),
			)
		}
		results = append(results, r)
	}

	return results, nil
}

// BuildConflictHistoryContext formats past conflict syntheses for prompt injection.
func (cs *ConflictSynthesisService) BuildConflictHistoryContext(
	ctx context.Context, patientID int64,
) string {
	history, err := cs.QueryConflictHistory(ctx, patientID, 3)
	if err != nil || len(history) == 0 {
		return ""
	}

	ctx_str := "\nHISTORICO DE CONFLITOS (Sinteses Riemannianas):\n"
	for i, h := range history {
		ctx_str += fmt.Sprintf("%d. %s\n", i+1, h.PromptFragment)
	}
	ctx_str += "-> Use este historico para abordar contradicoes recorrentes.\n\n"
	return ctx_str
}

// --- helpers ---

func buildSynthesisPromptFragment(thesis, antithesis, contradiction string, depth float64) string {
	abstractLevel := "concreto"
	if depth < 0.3 {
		abstractLevel = "muito abstrato (nucleo)"
	} else if depth < 0.6 {
		abstractLevel = "abstrato"
	}

	return fmt.Sprintf(
		"SINTESE RIEMANNIANA (profundidade=%.2f, nivel=%s): "+
			"Tese: '%s' vs Antitese: '%s'. "+
			"Contradicao: %s. "+
			"A sintese sugere que ambas posicoes coexistem em um nivel mais profundo.",
		depth, abstractLevel,
		truncate(thesis, 80), truncate(antithesis, 80),
		contradiction,
	)
}

func buildFallbackPromptFragment(thesis, antithesis, contradiction string) string {
	return fmt.Sprintf(
		"CONTRADICAO DETECTADA: Tese: '%s' vs Antitese: '%s'. %s",
		truncate(thesis, 80), truncate(antithesis, 80), contradiction,
	)
}

func nodeContentStr(content map[string]interface{}, key string) string {
	if v, ok := content[key].(string); ok {
		return v
	}
	return ""
}
