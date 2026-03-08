// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package knowledge

import (
	"context"
	"eva/internal/brainstem/config"
	"eva/internal/cortex/gemini"
	"eva/internal/util"
	"fmt"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// GraphReasoningService orquestra o raciocinio clinico usando NietzscheDB e Gemini Thinking
type GraphReasoningService struct {
	graphAdapter *nietzscheInfra.GraphAdapter
	geminiClient *gemini.Client
	cfg          *config.Config
	context      *ContextService
}

func NewGraphReasoningService(cfg *config.Config, graphAdapter *nietzscheInfra.GraphAdapter, ctxService *ContextService) *GraphReasoningService {
	return &GraphReasoningService{
		graphAdapter: graphAdapter,
		cfg:          cfg,
		context:      ctxService,
	}
}

// AnalyzeGraphContext extrai o contexto do grafo e pede analise do Gemini
func (s *GraphReasoningService) AnalyzeGraphContext(ctx context.Context, idosoID int64, currentTopic string) (string, error) {
	// 1. Extrair Sub-grafo relevante (ultimos nos ativados ou relacionados ao topico)
	graphData, err := s.fetchPatientContext(ctx, idosoID, currentTopic)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar grafo: %w", err)
	}

	if graphData == "" {
		return "", nil // Sem contexto relevante no grafo
	}

	// 2. Construir Prompt de Thinking
	prompt := fmt.Sprintf(`
	Voce e o Modulo de Raciocinio Clinico da EVA (Fractal Zeta Priming Network).

	CONTEXTO DO GRAFO (NietzscheDB):
	%s

	TOPICO ATUAL: "%s"

	TAREFAS:
	1. Analise as relacoes causais no grafo (ex: Dor -> Humor).
	2. Use raciocinio psicanalitico e medico.
	3. Decida: Devemos focar no sintoma fisico, na emocao ou em ambos?

	Responda APENAS com sua linha de raciocinio (Thoughts) e uma sugestao de abordagem tecnica.
	`, graphData, currentTopic)

	// 3. Chamar Gemini Thinking
	analysis, err := gemini.AnalyzeText(s.cfg, prompt)
	if err != nil {
		return "", fmt.Errorf("erro na analise do Gemini: %w", err)
	}

	// Persistir Factual Memory
	if s.context != nil {
		go s.context.SaveAnalysis(context.Background(), idosoID, "GRAPH", analysis)
	}

	return analysis, nil
}

// fetchPatientContext busca nos conectados ao paciente e ao topico recente
// Rewritten: uses BFS from patient node instead of Cypher *1..2 path
func (s *GraphReasoningService) fetchPatientContext(ctx context.Context, idosoID int64, topic string) (string, error) {
	// Find patient node
	patientResult, err := s.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Paciente",
		MatchKeys: map[string]interface{}{
			"id": idosoID,
		},
	})
	if err != nil {
		return "", err
	}

	// BFS from patient up to depth 2
	neighborIDs, err := s.graphAdapter.Bfs(ctx, patientResult.NodeID, 2, "")
	if err != nil {
		return "", err
	}

	thirtyDaysAgo := nietzscheInfra.DaysAgoUnix(30)
	var contextStr string
	count := 0

	for _, nID := range neighborIDs {
		if nID == patientResult.NodeID {
			continue
		}

		node, err := s.graphAdapter.GetNode(ctx, nID, "")
		if err != nil {
			continue
		}

		// Filter by timestamp (last 30 days)
		timestamp := util.ToFloat64(node.Content["timestamp"])
		if timestamp > 0 && timestamp < thirtyDaysAgo {
			continue
		}

		label := node.NodeType
		name, _ := node.Content["name"].(string)
		value, _ := node.Content["value"].(string)

		contextStr += fmt.Sprintf("(Paciente) -[RELATED]-> (%s: %s)\n", label, name)
		if value != "" {
			contextStr += fmt.Sprintf("   Detalhe: %s\n", value)
		}

		count++
		if count >= 10 {
			break
		}
	}

	return contextStr, nil
}

