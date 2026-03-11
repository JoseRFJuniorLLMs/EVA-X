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
// Memory Kernel — Phase 1 do Cognitive Operating System (COS)
// ============================================================================
//
// O Memory Kernel e o subsistema de memoria do COS. Gere quatro zonas de
// memoria inspiradas na neurociencia cognitiva:
//
//   - Working Memory  (WM)  — buffer temporario, alta energia, decai rapido
//   - Episodic Memory (EM)  — eventos autobiograficos com contexto temporal
//   - Semantic Memory (SM)  — conhecimento geral, baixa energia, persistente
//   - Procedural Memory (PM) — skills e padroes de acao automatizados
//
// Spreading Activation: quando uma memoria e activada, memorias relacionadas
// sao activadas proporcionalmente a forca da conexao (Hebbian learning).
//
// Ciencia: Tulving (1972) Memory Systems, Anderson (1983) Spreading Activation
// Engineering: ThoughtBus integration, energy-based decay, zone management

// MemoryZone define as zonas de memoria do kernel cognitivo
type MemoryZone string

const (
	WorkingMemory   MemoryZone = "working"
	EpisodicMemory  MemoryZone = "episodic"
	SemanticMemory  MemoryZone = "semantic"
	ProceduralMemory MemoryZone = "procedural"
)

// MemoryTrace representa uma memoria individual no kernel
type MemoryTrace struct {
	ID            string                 `json:"id"`
	Zone          MemoryZone             `json:"zone"`
	Content       string                 `json:"content"`
	Embedding     []float64              `json:"embedding,omitempty"`
	Energy        float64                `json:"energy"`         // 0.0-1.0, decai com tempo
	Activation    float64                `json:"activation"`     // Nivel de activacao actual
	Associations  map[string]float64     `json:"associations"`   // ID -> forca da conexao
	Tags          []string               `json:"tags,omitempty"`
	UserID        string                 `json:"user_id,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	LastAccessed  time.Time              `json:"last_accessed"`
	AccessCount   int64                  `json:"access_count"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryQuery representa um pedido de busca na memoria
type MemoryQuery struct {
	Content    string     `json:"content"`
	Zone       MemoryZone `json:"zone,omitempty"`    // Vazio = todas as zonas
	UserID     string     `json:"user_id,omitempty"`
	MinEnergy  float64    `json:"min_energy"`
	Limit      int        `json:"limit"`
	Embedding  []float64  `json:"embedding,omitempty"`
}

// MemoryResult resultado de uma busca na memoria
type MemoryResult struct {
	Traces          []*MemoryTrace `json:"traces"`
	SpreadActivated int            `json:"spread_activated"` // Quantas memorias foram activadas por spreading
	QueryTime       time.Duration  `json:"query_time"`
}

// MemoryKernel gere as zonas de memoria e integra com o ThoughtBus.
// Subscreve a ThoughtEvents do tipo Memory para activacao e consolidacao.
type MemoryKernel struct {
	bus  *ThoughtBus
	ctx  context.Context

	// Zonas de memoria (in-memory cache — backed by NietzscheDB)
	zones map[MemoryZone]map[string]*MemoryTrace
	mu    sync.RWMutex

	// Configuracao
	workingMemoryCapacity int           // Max items em working memory (Miller's 7+-2)
	decayInterval         time.Duration // Intervalo de decay energetico
	spreadDepth           int           // Profundidade de spreading activation

	// Metricas
	totalStored    atomic.Int64
	totalRetrieved atomic.Int64
	totalDecayed   atomic.Int64
	totalSpread    atomic.Int64

	// Callbacks para persistencia (NietzscheDB bridge)
	onStore    func(trace *MemoryTrace) error
	onRetrieve func(query *MemoryQuery) ([]*MemoryTrace, error)
}

// MemoryKernelConfig configuracao do kernel de memoria
type MemoryKernelConfig struct {
	WorkingMemoryCapacity int
	DecayInterval         time.Duration
	SpreadDepth           int
}

// DefaultMemoryKernelConfig retorna configuracao sensata para producao
func DefaultMemoryKernelConfig() MemoryKernelConfig {
	return MemoryKernelConfig{
		WorkingMemoryCapacity: 7,              // Miller's magical number
		DecayInterval:         30 * time.Second,
		SpreadDepth:           3,
	}
}

// NewMemoryKernel cria o kernel de memoria cognitivo.
// bus: ThoughtBus para comunicacao inter-modulo (obrigatorio)
func NewMemoryKernel(bus *ThoughtBus, cfg MemoryKernelConfig) *MemoryKernel {
	if cfg.WorkingMemoryCapacity <= 0 {
		cfg.WorkingMemoryCapacity = 7
	}
	if cfg.DecayInterval <= 0 {
		cfg.DecayInterval = 30 * time.Second
	}
	if cfg.SpreadDepth <= 0 {
		cfg.SpreadDepth = 3
	}

	mk := &MemoryKernel{
		bus:                   bus,
		zones:                 make(map[MemoryZone]map[string]*MemoryTrace),
		workingMemoryCapacity: cfg.WorkingMemoryCapacity,
		decayInterval:         cfg.DecayInterval,
		spreadDepth:           cfg.SpreadDepth,
	}

	// Inicializar zonas
	mk.zones[WorkingMemory] = make(map[string]*MemoryTrace)
	mk.zones[EpisodicMemory] = make(map[string]*MemoryTrace)
	mk.zones[SemanticMemory] = make(map[string]*MemoryTrace)
	mk.zones[ProceduralMemory] = make(map[string]*MemoryTrace)

	return mk
}

// Start inicia o kernel de memoria:
// 1. Subscreve ao ThoughtBus para eventos de memoria
// 2. Inicia goroutine de decay energetico
func (mk *MemoryKernel) Start(ctx context.Context) {
	mk.ctx = ctx

	// Subscrever a eventos de memoria no ThoughtBus
	if mk.bus != nil {
		mk.bus.Subscribe(Memory, mk.handleMemoryEvent)
		mk.bus.Subscribe(Perception, mk.handlePerceptionEvent)
		log.Info().Msg("[MemoryKernel] Subscrito ao ThoughtBus (Memory + Perception)")
	}

	// Goroutine de decay energetico
	go mk.decayLoop(ctx)

	log.Info().
		Int("wm_capacity", mk.workingMemoryCapacity).
		Dur("decay_interval", mk.decayInterval).
		Int("spread_depth", mk.spreadDepth).
		Msg("[MemoryKernel] Kernel de memoria iniciado")
}

// SetPersistenceCallbacks define funcoes de bridge para NietzscheDB
func (mk *MemoryKernel) SetPersistenceCallbacks(
	onStore func(trace *MemoryTrace) error,
	onRetrieve func(query *MemoryQuery) ([]*MemoryTrace, error),
) {
	mk.onStore = onStore
	mk.onRetrieve = onRetrieve
}

// Store armazena uma memoria no kernel e publica evento no ThoughtBus
func (mk *MemoryKernel) Store(trace *MemoryTrace) error {
	if trace.ID == "" {
		trace.ID = uuid.New().String()
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = time.Now()
	}
	trace.LastAccessed = time.Now()
	if trace.Associations == nil {
		trace.Associations = make(map[string]float64)
	}

	mk.mu.Lock()
	zone := trace.Zone
	if zone == "" {
		zone = WorkingMemory
		trace.Zone = zone
	}
	mk.zones[zone][trace.ID] = trace

	// Working memory capacity enforcement (oldest items evicted)
	if zone == WorkingMemory && len(mk.zones[WorkingMemory]) > mk.workingMemoryCapacity {
		mk.evictOldestFromWorking()
	}
	mk.mu.Unlock()

	mk.totalStored.Add(1)

	// Persistir no NietzscheDB (se callback definido)
	if mk.onStore != nil {
		if err := mk.onStore(trace); err != nil {
			log.Error().Err(err).Str("id", trace.ID).Msg("[MemoryKernel] Falha ao persistir memoria")
		}
	}

	// Publicar evento de memoria no ThoughtBus
	if mk.bus != nil {
		mk.bus.Publish(ThoughtEvent{
			Source:   "memory_kernel",
			Type:     Memory,
			Payload:  map[string]interface{}{"action": "store", "trace_id": trace.ID, "zone": string(zone)},
			Salience: trace.Energy,
		})
	}

	return nil
}

// Retrieve busca memorias no kernel com spreading activation
func (mk *MemoryKernel) Retrieve(query *MemoryQuery) *MemoryResult {
	start := time.Now()
	if query.Limit <= 0 {
		query.Limit = 10
	}

	mk.mu.RLock()

	var candidates []*MemoryTrace

	// Determinar zonas a buscar
	zonesToSearch := []MemoryZone{WorkingMemory, EpisodicMemory, SemanticMemory, ProceduralMemory}
	if query.Zone != "" {
		zonesToSearch = []MemoryZone{query.Zone}
	}

	for _, zone := range zonesToSearch {
		traces, ok := mk.zones[zone]
		if !ok {
			continue
		}
		for _, trace := range traces {
			if query.UserID != "" && trace.UserID != query.UserID {
				continue
			}
			if trace.Energy < query.MinEnergy {
				continue
			}
			candidates = append(candidates, trace)
		}
	}
	mk.mu.RUnlock()

	// Sort por energia * activacao (descendente)
	sortByRelevance(candidates)

	// Limitar resultados
	if len(candidates) > query.Limit {
		candidates = candidates[:query.Limit]
	}

	// Spreading activation: activar memorias associadas
	spreadCount := 0
	if len(candidates) > 0 {
		spreadCount = mk.spreadActivation(candidates, mk.spreadDepth)
	}

	// Actualizar last_accessed e access_count
	mk.mu.Lock()
	for _, trace := range candidates {
		trace.LastAccessed = time.Now()
		trace.AccessCount++
		trace.Activation = math.Min(1.0, trace.Activation+0.1) // Boost activation
	}
	mk.mu.Unlock()

	mk.totalRetrieved.Add(int64(len(candidates)))
	mk.totalSpread.Add(int64(spreadCount))

	// Publicar evento de retrieval no ThoughtBus
	if mk.bus != nil && len(candidates) > 0 {
		mk.bus.Publish(ThoughtEvent{
			Source:   "memory_kernel",
			Type:     Memory,
			Payload:  map[string]interface{}{"action": "retrieve", "count": len(candidates), "spread": spreadCount},
			Salience: candidates[0].Energy, // Saliencia da memoria mais relevante
		})
	}

	return &MemoryResult{
		Traces:          candidates,
		SpreadActivated: spreadCount,
		QueryTime:       time.Since(start),
	}
}

// Associate cria ou reforça associacao entre duas memorias (Hebbian learning)
// "Neurons that fire together wire together"
func (mk *MemoryKernel) Associate(traceID1, traceID2 string, strength float64) {
	mk.mu.Lock()
	defer mk.mu.Unlock()

	// Encontrar ambas as memorias
	var t1, t2 *MemoryTrace
	for _, zone := range mk.zones {
		if trace, ok := zone[traceID1]; ok {
			t1 = trace
		}
		if trace, ok := zone[traceID2]; ok {
			t2 = trace
		}
	}

	if t1 == nil || t2 == nil {
		return
	}

	// Reforcar associacao bidireccional
	if t1.Associations == nil {
		t1.Associations = make(map[string]float64)
	}
	if t2.Associations == nil {
		t2.Associations = make(map[string]float64)
	}

	// Hebbian update: nova forca = old + delta * (1 - old) — saturates at 1.0
	oldStrength1 := t1.Associations[traceID2]
	t1.Associations[traceID2] = oldStrength1 + strength*(1.0-oldStrength1)

	oldStrength2 := t2.Associations[traceID1]
	t2.Associations[traceID1] = oldStrength2 + strength*(1.0-oldStrength2)
}

// Consolidate move memorias de working memory para episodic/semantic
// baseado em criterios de energia e frequencia de acesso
func (mk *MemoryKernel) Consolidate() int {
	mk.mu.Lock()
	defer mk.mu.Unlock()

	consolidated := 0
	working := mk.zones[WorkingMemory]

	for id, trace := range working {
		// Regras de consolidacao:
		// 1. Alta energia + multiplos acessos → episodic (evento significativo)
		// 2. Muitas associacoes → semantic (conhecimento geral)
		// 3. Baixa energia → descarta (esquece)

		if trace.Energy < 0.2 {
			// Energia muito baixa — esquecer
			delete(working, id)
			mk.totalDecayed.Add(1)
			continue
		}

		targetZone := EpisodicMemory
		if len(trace.Associations) > 3 {
			targetZone = SemanticMemory // Muitas conexoes = conhecimento
		}

		if trace.AccessCount > 2 || trace.Energy > 0.7 {
			trace.Zone = targetZone
			mk.zones[targetZone][id] = trace
			delete(working, id)
			consolidated++
		}
	}

	if consolidated > 0 {
		log.Info().Int("count", consolidated).Msg("[MemoryKernel] Memorias consolidadas de WM")

		// Publicar evento de consolidacao
		if mk.bus != nil {
			mk.bus.Publish(ThoughtEvent{
				Source:   "memory_kernel",
				Type:     Memory,
				Payload:  map[string]interface{}{"action": "consolidate", "count": consolidated},
				Salience: 0.4,
			})
		}
	}

	return consolidated
}

// spreadActivation propaga activacao para memorias associadas
// Implementa o modelo de Anderson (1983) de spreading activation
func (mk *MemoryKernel) spreadActivation(seeds []*MemoryTrace, depth int) int {
	if depth <= 0 {
		return 0
	}

	mk.mu.Lock()
	defer mk.mu.Unlock()

	activated := 0
	for _, seed := range seeds {
		for assocID, strength := range seed.Associations {
			// Encontrar memoria associada
			for _, zone := range mk.zones {
				if trace, ok := zone[assocID]; ok {
					// Propagar activacao: activation += seed.activation * strength * decay
					boost := seed.Activation * strength * 0.5 // 50% decay per level
					if boost > 0.05 {                          // Threshold minimo
						trace.Activation = math.Min(1.0, trace.Activation+boost)
						activated++
					}
				}
			}
		}
	}

	return activated
}

// handleMemoryEvent processa eventos de memoria do ThoughtBus
func (mk *MemoryKernel) handleMemoryEvent(event ThoughtEvent) {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return
	}

	action, _ := payload["action"].(string)
	switch action {
	case "activate":
		// Activar memoria especifica
		traceID, _ := payload["trace_id"].(string)
		if traceID != "" {
			mk.activateTrace(traceID, event.Salience)
		}
	case "consolidate_request":
		// Pedido de consolidacao de outro modulo
		mk.Consolidate()
	}
}

// handlePerceptionEvent converte percepcoes em memorias de working memory
func (mk *MemoryKernel) handlePerceptionEvent(event ThoughtEvent) {
	// Percepcoes de alta saliencia sao automaticamente armazenadas na working memory
	if event.Salience < 0.3 {
		return
	}

	content := ""
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if c, ok := payload["content"].(string); ok {
			content = c
		} else if c, ok := payload["text"].(string); ok {
			content = c
		}
	}
	if content == "" {
		return
	}

	trace := &MemoryTrace{
		Zone:      WorkingMemory,
		Content:   content,
		Energy:    event.Salience,
		Activation: event.Salience,
		UserID:    event.UserID,
		SessionID: event.SessionID,
	}
	mk.Store(trace)
}

// activateTrace activa uma memoria especifica e propaga
func (mk *MemoryKernel) activateTrace(traceID string, boost float64) {
	mk.mu.Lock()
	defer mk.mu.Unlock()

	for _, zone := range mk.zones {
		if trace, ok := zone[traceID]; ok {
			trace.Activation = math.Min(1.0, trace.Activation+boost)
			trace.LastAccessed = time.Now()
			trace.AccessCount++
			return
		}
	}
}

// decayLoop reduz energia de todas as memorias periodicamente
// Memorias em working memory decaem mais rapido (Baddeley, 2000)
func (mk *MemoryKernel) decayLoop(ctx context.Context) {
	ticker := time.NewTicker(mk.decayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mk.applyDecay()
		}
	}
}

// applyDecay aplica decay a todas as memorias
func (mk *MemoryKernel) applyDecay() {
	mk.mu.Lock()
	defer mk.mu.Unlock()

	now := time.Now()

	for zone, traces := range mk.zones {
		decayRate := 0.01 // Default: 1% por intervalo

		switch zone {
		case WorkingMemory:
			decayRate = 0.05 // Working memory decai 5x mais rapido
		case EpisodicMemory:
			decayRate = 0.02
		case SemanticMemory:
			decayRate = 0.005 // Semantic memory decai devagar
		case ProceduralMemory:
			decayRate = 0.001 // Procedural quase nao decai
		}

		for id, trace := range traces {
			// Decay baseado em tempo desde ultimo acesso
			timeSince := now.Sub(trace.LastAccessed).Seconds()
			decay := decayRate * (1.0 + timeSince/3600.0) // Accelera com tempo

			trace.Energy = math.Max(0, trace.Energy-decay)
			trace.Activation = math.Max(0, trace.Activation-decay*2)

			// Remover memorias com energia zero
			if trace.Energy <= 0.001 {
				delete(traces, id)
				mk.totalDecayed.Add(1)
			}
		}
	}
}

// evictOldestFromWorking remove a memoria mais antiga da working memory
func (mk *MemoryKernel) evictOldestFromWorking() {
	working := mk.zones[WorkingMemory]
	if len(working) <= mk.workingMemoryCapacity {
		return
	}

	// Encontrar memoria com menor energia
	var lowestID string
	lowestEnergy := math.MaxFloat64

	for id, trace := range working {
		if trace.Energy < lowestEnergy {
			lowestEnergy = trace.Energy
			lowestID = id
		}
	}

	if lowestID != "" {
		// Tentar consolidar antes de descartar
		evicted := working[lowestID]
		if evicted.Energy > 0.3 && evicted.AccessCount > 1 {
			// Promover para episodic antes de remover da WM
			evicted.Zone = EpisodicMemory
			mk.zones[EpisodicMemory][lowestID] = evicted
		}
		delete(working, lowestID)
	}
}

// GetStatistics retorna metricas do kernel de memoria
func (mk *MemoryKernel) GetStatistics() map[string]interface{} {
	mk.mu.RLock()
	defer mk.mu.RUnlock()

	zoneCounts := make(map[string]int)
	totalEnergy := 0.0
	totalTraces := 0

	for zone, traces := range mk.zones {
		zoneCounts[string(zone)] = len(traces)
		for _, trace := range traces {
			totalEnergy += trace.Energy
			totalTraces++
		}
	}

	avgEnergy := 0.0
	if totalTraces > 0 {
		avgEnergy = totalEnergy / float64(totalTraces)
	}

	return map[string]interface{}{
		"engine":          "memory_kernel",
		"zones":           zoneCounts,
		"total_traces":    totalTraces,
		"average_energy":  avgEnergy,
		"total_stored":    mk.totalStored.Load(),
		"total_retrieved": mk.totalRetrieved.Load(),
		"total_decayed":   mk.totalDecayed.Load(),
		"total_spread":    mk.totalSpread.Load(),
		"wm_capacity":     mk.workingMemoryCapacity,
	}
}

// sortByRelevance ordena memorias por energia * activacao (in-place)
func sortByRelevance(traces []*MemoryTrace) {
	// Simple insertion sort (traces are typically small)
	for i := 1; i < len(traces); i++ {
		key := traces[i]
		keyScore := key.Energy * (1.0 + key.Activation)
		j := i - 1
		for j >= 0 && traces[j].Energy*(1.0+traces[j].Activation) < keyScore {
			traces[j+1] = traces[j]
			j--
		}
		traces[j+1] = key
	}
}
