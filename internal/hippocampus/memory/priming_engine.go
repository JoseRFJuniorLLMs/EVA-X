// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"fmt"
	"strings"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// PrimingEngine substitui RetrievalService no modelo FZPN
type PrimingEngine struct {
	graph *nietzscheInfra.GraphAdapter
}

// NewPrimingEngine cria novo motor de priming
func NewPrimingEngine(graph *nietzscheInfra.GraphAdapter) *PrimingEngine {
	return &PrimingEngine{graph: graph}
}

// Prime realiza a busca "Fractal" puxando nós conectados
func (p *PrimingEngine) Prime(ctx context.Context, idosoID int64, queryText string) ([]string, error) {
	// 1. Busca por palavras-chave (Significantes/Topicos) na query
	// (Simplificação: busca textual exata ou contains)

	// Start BFS from the Person node, traversing EXPERIENCED edges (depth 2 covers RELATED_TO/EVOCA)
	startNodeID := fmt.Sprintf("Person:%d", idosoID)
	nodeIDs, err := p.graph.BfsWithEdgeType(ctx, startNodeID, "EXPERIENCED", 2, "")
	if err != nil {
		return nil, fmt.Errorf("priming search failed: %w", err)
	}

	// Fetch each node and filter by content match or speaker
	var results []string
	for _, nodeID := range nodeIDs {
		node, err := p.graph.GetNode(ctx, nodeID, "")
		if err != nil || !node.Found {
			continue
		}
		content, hasContent := node.Content["content"].(string)
		if !hasContent {
			continue
		}
		speaker, _ := node.Content["speaker"].(string)
		if strings.Contains(content, queryText) || speaker == "user" {
			results = append(results, content)
		}
		if len(results) >= 5 {
			break
		}
	}

	return results, nil
}
