// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consciousness

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ============================================================================
// Graph Evolution Engine — Phase 5 do Cognitive Operating System (COS)
// ============================================================================
//
// O Graph Evolution Engine implementa operacoes de evolucao autonoma sobre
// o grafo cognitivo da EVA. O grafo nao e estatico — ele evolui:
//
//   - Node Fusion:     conceitos similares sao fundidos num unico
//   - Node Fission:    conceitos sobrecarregados sao divididos
//   - Concept Abstraction: abstraccoes sao criadas a partir de exemplos
//   - Hierarchy Building:  relacoes hierarquicas sao detectadas e criadas
//   - Edge Pruning:    conexoes fracas sao podadas (sinapse elimination)
//
// O engine e event-driven: subscreve ao ThoughtBus e propoe evolucoes
// quando detecta oportunidades. Publicacoes para NietzscheDB via callbacks.
//
// Ciencia: Quillian (1968) Semantic Networks, Lakoff (1987) Categories
// Engineering: ThoughtBus-driven evolution with NietzscheDB bridge

// EvolutionType tipos de evolucao do grafo
type EvolutionType string

const (
	Fusion      EvolutionType = "fusion"       // Merge 2+ conceitos similares
	Fission     EvolutionType = "fission"      // Split 1 conceito sobrecarregado
	Abstraction EvolutionType = "abstraction"  // Criar conceito abstracto
	Pruning     EvolutionType = "pruning"      // Remover conexoes fracas
	Emergence   EvolutionType = "emergence"    // Novo conceito emergente
)

// EvolutionProposal proposta de evolucao do grafo
type EvolutionProposal struct {
	ID           string        `json:"id"`
	Type         EvolutionType `json:"type"`
	Description  string        `json:"description"`
	SourceNodes  []string      `json:"source_nodes"`            // Nodes envolvidos
	TargetNode   string        `json:"target_node,omitempty"`   // Node resultado (para fusion)
	Confidence   float64       `json:"confidence"`              // 0.0-1.0
	EnergyImpact float64       `json:"energy_impact"`           // Impacto estimado na energia do grafo
	Timestamp    time.Time     `json:"timestamp"`
	Applied      bool          `json:"applied"`
}

// GraphEvolutionEngine motor de evolucao do grafo cognitivo
type GraphEvolutionEngine struct {
	bus          *ThoughtBus
	memoryKernel *MemoryKernel
	ctx          context.Context
	mu           sync.RWMutex

	// Configuracao
	fusionThreshold      float64 // Similaridade minima para fusao
	fissionThreshold     int     // Max associacoes antes de fissao
	abstractionThreshold int     // Min exemplos para abstracao
	pruneThreshold       float64 // Min forca para manter conexao

	// Historico de evolucoes
	proposals  []*EvolutionProposal
	proposalMu sync.Mutex

	// Metricas
	totalFusions      atomic.Int64
	totalFissions     atomic.Int64
	totalAbstractions atomic.Int64
	totalPruned       atomic.Int64
	totalEmergences   atomic.Int64

	// Internal tracking
	totalStoreEvents atomic.Int64

	// Callbacks para NietzscheDB
	onFusion      func(sourceIDs []string, mergedContent string) (string, error) // Returns new node ID
	onFission     func(sourceID string, parts []string) ([]string, error)        // Returns new node IDs
	onAbstraction func(concept string, examples []string) (string, error)        // Returns abstract node ID
	onPrune       func(edgeID string) error
}

// GraphEvolutionConfig configuracao do engine de evolucao
type GraphEvolutionConfig struct {
	FusionThreshold      float64
	FissionThreshold     int
	AbstractionThreshold int
	PruneThreshold       float64
}

// DefaultGraphEvolutionConfig retorna configuracao padrao
func DefaultGraphEvolutionConfig() GraphEvolutionConfig {
	return GraphEvolutionConfig{
		FusionThreshold:      0.85,
		FissionThreshold:     15,
		AbstractionThreshold: 3,
		PruneThreshold:       0.05,
	}
}

// NewGraphEvolutionEngine cria o motor de evolucao do grafo
func NewGraphEvolutionEngine(bus *ThoughtBus, mk *MemoryKernel, cfg GraphEvolutionConfig) *GraphEvolutionEngine {
	return &GraphEvolutionEngine{
		bus:                  bus,
		memoryKernel:         mk,
		fusionThreshold:      cfg.FusionThreshold,
		fissionThreshold:     cfg.FissionThreshold,
		abstractionThreshold: cfg.AbstractionThreshold,
		pruneThreshold:       cfg.PruneThreshold,
		proposals:            make([]*EvolutionProposal, 0),
	}
}

// Start inicia o engine de evolucao
func (ge *GraphEvolutionEngine) Start(ctx context.Context) {
	ge.ctx = ctx

	// Subscrever ao ThoughtBus para detectar oportunidades de evolucao
	if ge.bus != nil {
		ge.bus.Subscribe(Memory, ge.handleMemoryEvent)
		ge.bus.Subscribe(Reflection, ge.handleReflectionEvent)
	}

	// Goroutine de evolucao periodica
	go ge.evolutionLoop(ctx)

	log.Info().
		Float64("fusion_threshold", ge.fusionThreshold).
		Int("fission_threshold", ge.fissionThreshold).
		Msg("[GraphEvolution] Motor de evolucao iniciado")
}

// SetCallbacks define callbacks para persistir evolucoes no NietzscheDB
func (ge *GraphEvolutionEngine) SetCallbacks(
	onFusion func(sourceIDs []string, mergedContent string) (string, error),
	onFission func(sourceID string, parts []string) ([]string, error),
	onAbstraction func(concept string, examples []string) (string, error),
	onPrune func(edgeID string) error,
) {
	ge.onFusion = onFusion
	ge.onFission = onFission
	ge.onAbstraction = onAbstraction
	ge.onPrune = onPrune
}

// ProposeFusion propoe fusao de dois conceitos similares
func (ge *GraphEvolutionEngine) ProposeFusion(sourceIDs []string, confidence float64, description string) *EvolutionProposal {
	proposal := &EvolutionProposal{
		ID:          uuid.New().String(),
		Type:        Fusion,
		Description: description,
		SourceNodes: sourceIDs,
		Confidence:  confidence,
		Timestamp:   time.Now(),
	}

	ge.proposalMu.Lock()
	ge.proposals = append(ge.proposals, proposal)
	ge.proposalMu.Unlock()

	log.Debug().
		Strs("sources", sourceIDs).
		Float64("confidence", confidence).
		Msg("[GraphEvolution] Fusao proposta")

	return proposal
}

// ProposeFission propoe divisao de um conceito sobrecarregado
func (ge *GraphEvolutionEngine) ProposeFission(sourceID string, parts []string, confidence float64) *EvolutionProposal {
	proposal := &EvolutionProposal{
		ID:          uuid.New().String(),
		Type:        Fission,
		Description: "Conceito sobrecarregado dividido em sub-conceitos",
		SourceNodes: []string{sourceID},
		Confidence:  confidence,
		Timestamp:   time.Now(),
	}

	ge.proposalMu.Lock()
	ge.proposals = append(ge.proposals, proposal)
	ge.proposalMu.Unlock()

	return proposal
}

// DetectFusionCandidates analisa o MemoryKernel para encontrar candidatos a fusao
func (ge *GraphEvolutionEngine) DetectFusionCandidates() []*EvolutionProposal {
	if ge.memoryKernel == nil {
		return nil
	}

	pairs := ge.memoryKernel.FindFusionCandidates(ge.fusionThreshold)

	proposals := make([]*EvolutionProposal, 0, len(pairs))
	for _, pair := range pairs {
		proposal := &EvolutionProposal{
			ID:          uuid.New().String(),
			Type:        Fusion,
			Description: "Alta associacao mutua detectada",
			SourceNodes: []string{pair[0], pair[1]},
			Confidence:  ge.fusionThreshold, // conservative: at least threshold
			Timestamp:   time.Now(),
		}
		proposals = append(proposals, proposal)
	}

	return proposals
}

// DetectFissionCandidates encontra conceitos sobrecarregados para divisao
func (ge *GraphEvolutionEngine) DetectFissionCandidates() []*EvolutionProposal {
	if ge.memoryKernel == nil {
		return nil
	}

	overloaded := ge.memoryKernel.FindOverloadedTraces(ge.fissionThreshold)

	proposals := make([]*EvolutionProposal, 0, len(overloaded))
	for _, item := range overloaded {
		confidence := float64(item.Count) / float64(ge.fissionThreshold*2)
		if confidence > 1.0 {
			confidence = 1.0
		}
		proposal := &EvolutionProposal{
			ID:          uuid.New().String(),
			Type:        Fission,
			Description: "Conceito com excesso de associacoes",
			SourceNodes: []string{item.ID},
			Confidence:  confidence,
			Timestamp:   time.Now(),
		}
		proposals = append(proposals, proposal)
	}

	return proposals
}

// PruneWeakConnections remove associacoes fracas de todas as memorias
func (ge *GraphEvolutionEngine) PruneWeakConnections() int {
	if ge.memoryKernel == nil {
		return 0
	}

	pruned := ge.memoryKernel.PruneWeakAssociations(ge.pruneThreshold)

	if pruned > 0 {
		ge.totalPruned.Add(int64(pruned))
		log.Info().Int("pruned", pruned).Msg("[GraphEvolution] Conexoes fracas podadas")
	}

	return pruned
}

// DetectEmergentConcepts detecta conceitos que emergem de padroes de co-activacao
func (ge *GraphEvolutionEngine) DetectEmergentConcepts() []*EvolutionProposal {
	if ge.memoryKernel == nil {
		return nil
	}

	clusters := ge.memoryKernel.FindEmergentClusters(ge.abstractionThreshold)

	proposals := make([]*EvolutionProposal, 0, len(clusters))
	for _, cluster := range clusters {
		proposal := &EvolutionProposal{
			ID:           uuid.New().String(),
			Type:         Emergence,
			Description:  "Cluster de conceitos inter-conectados detectado",
			SourceNodes:  cluster,
			Confidence:   math.Min(1.0, float64(len(cluster))/5.0),
			EnergyImpact: 0.3,
			Timestamp:    time.Now(),
		}
		proposals = append(proposals, proposal)
	}

	return proposals
}

// ApplyProposal aplica uma proposta de evolucao
func (ge *GraphEvolutionEngine) ApplyProposal(proposalID string) error {
	ge.proposalMu.Lock()
	var target *EvolutionProposal
	for _, p := range ge.proposals {
		if p.ID == proposalID {
			target = p
			break
		}
	}
	ge.proposalMu.Unlock()

	if target == nil || target.Applied {
		return nil
	}

	switch target.Type {
	case Fusion:
		ge.totalFusions.Add(1)
		if ge.onFusion != nil {
			_, err := ge.onFusion(target.SourceNodes, target.Description)
			if err != nil {
				return err
			}
		}
	case Fission:
		ge.totalFissions.Add(1)
	case Abstraction:
		ge.totalAbstractions.Add(1)
	case Emergence:
		ge.totalEmergences.Add(1)
	}

	target.Applied = true

	// Publicar evento de evolucao
	if ge.bus != nil {
		ge.bus.Publish(ThoughtEvent{
			Source:   "graph_evolution",
			Type:     Reflection,
			Payload:  map[string]interface{}{"evolution": string(target.Type), "nodes": target.SourceNodes},
			Salience: target.Confidence * 0.5,
		})
	}

	return nil
}

// handleMemoryEvent reage a eventos de memoria para detectar oportunidades
func (ge *GraphEvolutionEngine) handleMemoryEvent(event ThoughtEvent) {
	// Memoria store events podem trigger deteccao de fusao
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return
	}
	action, _ := payload["action"].(string)
	if action == "store" {
		// Periodicamente verificar fusao apos novos stores
		ge.totalStoreEvents.Add(1)
	}
}

// handleReflectionEvent reage a reflexoes para trigger evolucao
func (ge *GraphEvolutionEngine) handleReflectionEvent(event ThoughtEvent) {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return
	}
	if alert, ok := payload["hubs"]; ok {
		_ = alert // Hub detection pode triggerar fission
	}
}

// evolutionLoop corre periodicamente para detectar e aplicar evolucoes
func (ge *GraphEvolutionEngine) evolutionLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ge.runEvolutionCycle()
		}
	}
}

// runEvolutionCycle executa um ciclo completo de deteccao e aplicacao
func (ge *GraphEvolutionEngine) runEvolutionCycle() {
	// 1. Prune weak connections
	pruned := ge.PruneWeakConnections()

	// 2. Detect fusion candidates
	fusionProposals := ge.DetectFusionCandidates()

	// 3. Detect fission candidates
	fissionProposals := ge.DetectFissionCandidates()

	// 4. Detect emergent concepts
	emergentProposals := ge.DetectEmergentConcepts()

	// 5. Auto-apply high-confidence proposals
	allProposals := make([]*EvolutionProposal, 0)
	allProposals = append(allProposals, fusionProposals...)
	allProposals = append(allProposals, fissionProposals...)
	allProposals = append(allProposals, emergentProposals...)

	applied := 0
	for _, p := range allProposals {
		if p.Confidence > 0.8 { // Auto-apply apenas alta confianca
			ge.proposalMu.Lock()
			ge.proposals = append(ge.proposals, p)
			ge.proposalMu.Unlock()
			if err := ge.ApplyProposal(p.ID); err != nil {
				log.Error().Err(err).Str("type", string(p.Type)).Msg("[GraphEvolution] Falha ao aplicar proposta")
			} else {
				applied++
			}
		}
	}

	if pruned > 0 || applied > 0 {
		log.Info().
			Int("pruned", pruned).
			Int("proposals", len(allProposals)).
			Int("applied", applied).
			Msg("[GraphEvolution] Ciclo de evolucao completo")
	}
}

// GetStatistics retorna metricas do engine de evolucao
func (ge *GraphEvolutionEngine) GetStatistics() map[string]interface{} {
	ge.proposalMu.Lock()
	pendingProposals := 0
	for _, p := range ge.proposals {
		if !p.Applied {
			pendingProposals++
		}
	}
	ge.proposalMu.Unlock()

	return map[string]interface{}{
		"engine":              "graph_evolution",
		"total_fusions":       ge.totalFusions.Load(),
		"total_fissions":      ge.totalFissions.Load(),
		"total_abstractions":  ge.totalAbstractions.Load(),
		"total_pruned":        ge.totalPruned.Load(),
		"total_emergences":    ge.totalEmergences.Load(),
		"pending_proposals":   pendingProposals,
		"fusion_threshold":    ge.fusionThreshold,
		"fission_threshold":   ge.fissionThreshold,
	}
}

