// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Proprioception Module — EVA's cognitive self-awareness system.
// Generates the [ESTADO DO GRAFO] block for injection into the System Prompt.
// Refreshes every 15 minutes without restarting the conversation.

package proprioception

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// Engine manages EVA's proprioceptive awareness of the NietzscheDB graph state.
type Engine struct {
	client        *nietzscheInfra.Client
	refreshPeriod time.Duration

	mu           sync.RWMutex
	lastScan     time.Time
	cachedPrompt string
}

// New creates a new proprioception engine.
func New(client *nietzscheInfra.Client) *Engine {
	return &Engine{
		client:        client,
		refreshPeriod: 15 * time.Minute,
	}
}

// GetGraphState returns the current [ESTADO DO GRAFO] text block for the system prompt.
// Caches the result and refreshes every 15 minutes.
func (e *Engine) GetGraphState(ctx context.Context) string {
	e.mu.RLock()
	if time.Since(e.lastScan) < e.refreshPeriod && e.cachedPrompt != "" {
		defer e.mu.RUnlock()
		return e.cachedPrompt
	}
	e.mu.RUnlock()

	// Refresh needed
	prompt := e.scan(ctx)

	e.mu.Lock()
	e.cachedPrompt = prompt
	e.lastScan = time.Now()
	e.mu.Unlock()

	return prompt
}

// ForceRefresh clears the cache and forces a new scan on next call.
func (e *Engine) ForceRefresh() {
	e.mu.Lock()
	e.lastScan = time.Time{}
	e.mu.Unlock()
}

// scan performs the actual brain scan via gRPC and formats the result.
func (e *Engine) scan(ctx context.Context) string {
	if e.client == nil {
		return "[ESTADO DO GRAFO]\nNietzscheDB indisponível — sem dados de propriocepção.\n"
	}

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	collections, err := e.client.ListCollections(scanCtx)
	if err != nil {
		log.Printf("[PROPRIOCEPTION] Scan failed: %v", err)
		return "[ESTADO DO GRAFO]\nErro ao contactar NietzscheDB — propriocepção temporariamente offline.\n"
	}

	var totalNodes, totalEdges uint64
	type colSummary struct {
		Name      string
		NodeCount uint64
		EdgeCount uint64
	}
	summaries := make([]colSummary, 0, len(collections))

	for _, col := range collections {
		totalNodes += col.NodeCount
		totalEdges += col.EdgeCount
		summaries = append(summaries, colSummary{
			Name:      col.Name,
			NodeCount: col.NodeCount,
			EdgeCount: col.EdgeCount,
		})
	}

	// Sort by node count descending
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].NodeCount > summaries[j].NodeCount
	})

	// Top-3 collections
	top3 := ""
	for i, s := range summaries {
		if i >= 3 {
			break
		}
		if i > 0 {
			top3 += ", "
		}
		top3 += fmt.Sprintf("%s (%d nós/%d edges)", s.Name, s.NodeCount, s.EdgeCount)
	}

	// Empty collections
	var empty []string
	for _, s := range summaries {
		if s.NodeCount == 0 {
			empty = append(empty, s.Name)
		}
	}

	prompt := fmt.Sprintf("[ESTADO DO GRAFO]\n"+
		"O teu grafo de conhecimento contém %d nós e %d edges em %d collections.\n"+
		"Maiores: %s.\n",
		totalNodes, totalEdges, len(collections), top3)

	if len(empty) > 0 {
		maxShow := 5
		if len(empty) < maxShow {
			maxShow = len(empty)
		}
		prompt += fmt.Sprintf("Collections vazias: %v.\n", empty[:maxShow])
	}

	prompt += fmt.Sprintf("Última actualização: %s.\n", time.Now().Format("15:04"))

	return prompt
}

// BuildSystemPrompt assembles the full EVA system prompt with proprioception.
// Structure:
//
//	[IDENTIDADE]
//	[ESTADO DO GRAFO — gerado pelo Proprioception Engine]
//	[REGRAS DE COMPORTAMENTO]
//	[FERRAMENTAS DISPONÍVEIS]
func BuildSystemPrompt(identity, graphState, rules, toolsDesc string) string {
	return fmt.Sprintf("[IDENTIDADE]\n%s\n\n%s\n[REGRAS DE COMPORTAMENTO]\n%s\n\n[FERRAMENTAS DISPONÍVEIS]\n%s",
		identity, graphState, rules, toolsDesc)
}

// DefaultRules returns the standard proprioception behavioral rules.
func DefaultRules() string {
	return `- Reporta sempre o estado emocional/cognitivo antes de acções grandes.
- Nunca escreve nas collections core (eva_core, eva_self_knowledge, eva_codebase, knowledge_galaxies) sem confirmação explícita.
- Usa task_type correcto nos embeddings (RETRIEVAL_QUERY para buscas).
- Fala em português com José, técnico quando necessário.
- Após cada tool call, comenta o resultado em voz natural.
- Se latência de qualquer tool > 500ms, reporta ao utilizador.`
}

// DefaultToolsDescription returns the proprioception tools available to EVA.
func DefaultToolsDescription() string {
	return `- brain_scan: Sente o estado actual do grafo (read-only).
- feel_the_graph(collection, query): Encontra nós próximos de um conceito (read-only).
- internalize_memory(content, valence, confirm): Guarda memória em eva_mind (requer confirmação).
- reorganize_thoughts(target_collection): Reconsolidação de embeddings (apenas collections não-core).`
}
