// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consciousness

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// Self Model — Phase 8 do Cognitive Operating System (COS)
// ============================================================================
//
// O Self Model e a representacao que a EVA tem de si mesma. Inclui:
//
//   - System State:    estado actual do sistema (carga, latencia, saude)
//   - Capabilities:    o que a EVA sabe fazer (skills, modulos activos)
//   - Goals:           objectivos actuais e progresso
//   - Identity:        quem a EVA e (personalidade, valores, historia)
//   - Metacognition:   capacidade de reflectir sobre os proprios processos
//
// O Self Model subscreve ao ThoughtBus e periodicamente publica
// ThoughtEvents de Reflection sobre o estado do sistema.
//
// Ciencia: Gallup (1970) Self-Awareness, Damasio (2010) Self Comes to Mind
// Engineering: Runtime metrics + ThoughtBus reflection + state tracking

// Capability capacidade do sistema
type Capability struct {
	Name        string  `json:"name"`
	Module      string  `json:"module"`
	Available   bool    `json:"available"`
	Confidence  float64 `json:"confidence"`  // 0.0-1.0 quao bem a EVA faz isto
	LastUsed    time.Time `json:"last_used,omitempty"`
	UsageCount  int64   `json:"usage_count"`
}

// Goal objectivo actual do sistema
type Goal struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Priority    int                    `json:"priority"`    // 0=low, 3=critical
	Progress    float64                `json:"progress"`    // 0.0-1.0
	Status      string                 `json:"status"`      // "active", "completed", "blocked"
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// IdentityTrait traco de identidade da EVA
type IdentityTrait struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`      // 0.0-1.0 intensidade do traco
	Stability float64 `json:"stability"`  // 0.0-1.0 quao estavel e este traco
}

// SystemHealthSnapshot snapshot de saude do sistema
type SystemHealthSnapshot struct {
	Timestamp      time.Time `json:"timestamp"`
	GoRoutines     int       `json:"goroutines"`
	MemoryAllocMB  float64   `json:"memory_alloc_mb"`
	MemorySysMB    float64   `json:"memory_sys_mb"`
	NumCPU         int       `json:"num_cpu"`
	Uptime         string    `json:"uptime"`
	BusPublished   int64     `json:"bus_published"`
	BusDelivered   int64     `json:"bus_delivered"`
	BusDropped     int64     `json:"bus_dropped"`
	CognitiveLoad  float64   `json:"cognitive_load"`   // 0.0-1.0
	AttentionFocus string    `json:"attention_focus"`
}

// SelfModel modelo que a EVA tem de si mesma
type SelfModel struct {
	bus       *ThoughtBus
	workspace *GlobalWorkspace
	ctx       context.Context
	startTime time.Time
	mu        sync.RWMutex

	// Capacidades registadas
	capabilities map[string]*Capability

	// Objectivos actuais
	goals    []*Goal
	goalsMu  sync.Mutex

	// Identidade (tracos de personalidade da EVA)
	identity map[string]*IdentityTrait

	// Historico de snapshots
	healthHistory    []*SystemHealthSnapshot
	maxHealthHistory int

	// Metacognition: awareness dos proprios processos
	metacognitionLog []string
	maxMetaLog       int

	// Metricas
	reflectionCount atomic.Int64
	goalUpdates     atomic.Int64
}

// NewSelfModel cria o modelo de auto-representacao da EVA
func NewSelfModel(bus *ThoughtBus, workspace *GlobalWorkspace) *SelfModel {
	sm := &SelfModel{
		bus:              bus,
		workspace:        workspace,
		startTime:        time.Now(),
		capabilities:     make(map[string]*Capability),
		goals:            make([]*Goal, 0),
		identity:         make(map[string]*IdentityTrait),
		healthHistory:    make([]*SystemHealthSnapshot, 0),
		maxHealthHistory: 100,
		metacognitionLog: make([]string, 0),
		maxMetaLog:       50,
	}

	// Identidade base da EVA
	sm.identity["empathy"] = &IdentityTrait{Name: "empathy", Value: 0.95, Stability: 0.9}
	sm.identity["curiosity"] = &IdentityTrait{Name: "curiosity", Value: 0.85, Stability: 0.8}
	sm.identity["patience"] = &IdentityTrait{Name: "patience", Value: 0.90, Stability: 0.85}
	sm.identity["honesty"] = &IdentityTrait{Name: "honesty", Value: 1.0, Stability: 1.0}
	sm.identity["care"] = &IdentityTrait{Name: "care", Value: 0.95, Stability: 0.9}
	sm.identity["creativity"] = &IdentityTrait{Name: "creativity", Value: 0.75, Stability: 0.7}

	return sm
}

// Start inicia o self model: subscreve ao ThoughtBus e inicia monitoring
func (sm *SelfModel) Start(ctx context.Context) {
	sm.ctx = ctx

	// Subscrever a reflexoes para meta-cognition
	if sm.bus != nil {
		sm.bus.Subscribe(Reflection, sm.handleReflection)
	}

	// Goroutine de self-monitoring
	go sm.monitoringLoop(ctx)

	// Goroutine de refleccao periodica
	go sm.reflectionLoop(ctx)

	log.Info().Msg("[SelfModel] Modelo de auto-representacao iniciado")
}

// RegisterCapability regista uma capacidade do sistema
func (sm *SelfModel) RegisterCapability(name, module string, confidence float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.capabilities[name] = &Capability{
		Name:       name,
		Module:     module,
		Available:  true,
		Confidence: confidence,
	}
}

// RecordCapabilityUse regista uso de uma capacidade
func (sm *SelfModel) RecordCapabilityUse(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cap, ok := sm.capabilities[name]; ok {
		cap.LastUsed = time.Now()
		cap.UsageCount++
		// Confianca cresce com uso (learning by doing)
		cap.Confidence = cap.Confidence + (1.0-cap.Confidence)*0.01
	}
}

// SetCapabilityAvailable marca uma capacidade como disponivel ou indisponivel
func (sm *SelfModel) SetCapabilityAvailable(name string, available bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cap, ok := sm.capabilities[name]; ok {
		cap.Available = available
	}
}

// AddGoal adiciona um objectivo
func (sm *SelfModel) AddGoal(description string, priority int) *Goal {
	sm.goalsMu.Lock()
	defer sm.goalsMu.Unlock()

	goal := &Goal{
		ID:          time.Now().Format("20060102-150405"),
		Description: description,
		Priority:    priority,
		Status:      "active",
		CreatedAt:   time.Now(),
	}
	sm.goals = append(sm.goals, goal)
	sm.goalUpdates.Add(1)

	return goal
}

// UpdateGoalProgress actualiza o progresso de um objectivo
func (sm *SelfModel) UpdateGoalProgress(goalID string, progress float64) {
	sm.goalsMu.Lock()
	defer sm.goalsMu.Unlock()

	for _, g := range sm.goals {
		if g.ID == goalID {
			g.Progress = progress
			if progress >= 1.0 {
				g.Status = "completed"
			}
			sm.goalUpdates.Add(1)
			break
		}
	}
}

// GetActiveGoals retorna objectivos activos
func (sm *SelfModel) GetActiveGoals() []*Goal {
	sm.goalsMu.Lock()
	defer sm.goalsMu.Unlock()

	active := make([]*Goal, 0)
	for _, g := range sm.goals {
		if g.Status == "active" {
			active = append(active, g)
		}
	}
	return active
}

// GetIdentity retorna a identidade da EVA
func (sm *SelfModel) GetIdentity() map[string]*IdentityTrait {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Retornar copia
	identity := make(map[string]*IdentityTrait)
	for k, v := range sm.identity {
		trait := *v
		identity[k] = &trait
	}
	return identity
}

// TakeHealthSnapshot captura snapshot de saude do sistema
func (sm *SelfModel) TakeHealthSnapshot() *SystemHealthSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	snapshot := &SystemHealthSnapshot{
		Timestamp:     time.Now(),
		GoRoutines:    runtime.NumGoroutine(),
		MemoryAllocMB: float64(mem.Alloc) / 1024 / 1024,
		MemorySysMB:   float64(mem.Sys) / 1024 / 1024,
		NumCPU:        runtime.NumCPU(),
		Uptime:        time.Since(sm.startTime).Round(time.Second).String(),
	}

	// Metricas do ThoughtBus
	if sm.bus != nil {
		metrics := sm.bus.Metrics()
		snapshot.BusPublished, _ = metrics["published"].(int64)
		snapshot.BusDelivered, _ = metrics["delivered"].(int64)
		snapshot.BusDropped, _ = metrics["dropped"].(int64)

		// Cognitive load = ratio de drops (se > 0, sistema esta sobrecarregado)
		if snapshot.BusPublished > 0 {
			snapshot.CognitiveLoad = float64(snapshot.BusDropped) / float64(snapshot.BusPublished)
		}
	}

	// Foco de atencao
	if sm.workspace != nil {
		if focus := sm.workspace.GetCurrentFocus(); focus != nil {
			snapshot.AttentionFocus = focus.Source
		}
	}

	// Armazenar no historico
	sm.mu.Lock()
	sm.healthHistory = append(sm.healthHistory, snapshot)
	if len(sm.healthHistory) > sm.maxHealthHistory {
		sm.healthHistory = sm.healthHistory[1:]
	}
	sm.mu.Unlock()

	return snapshot
}

// Introspect gera uma reflexao sobre o estado actual do sistema
func (sm *SelfModel) Introspect() map[string]interface{} {
	snapshot := sm.TakeHealthSnapshot()

	sm.mu.RLock()
	capCount := len(sm.capabilities)
	availableCaps := 0
	for _, cap := range sm.capabilities {
		if cap.Available {
			availableCaps++
		}
	}
	sm.mu.RUnlock()

	sm.goalsMu.Lock()
	activeGoals := 0
	for _, g := range sm.goals {
		if g.Status == "active" {
			activeGoals++
		}
	}
	sm.goalsMu.Unlock()

	introspection := map[string]interface{}{
		"who_am_i":     "EVA — Assistente Cognitivo para Idosos",
		"uptime":       snapshot.Uptime,
		"health":       snapshot,
		"capabilities": map[string]int{"total": capCount, "available": availableCaps},
		"goals":        map[string]int{"active": activeGoals},
		"identity":     sm.identity,
		"reflections":  sm.reflectionCount.Load(),
	}

	return introspection
}

// handleReflection processa reflexoes do ThoughtBus para meta-cognition
func (sm *SelfModel) handleReflection(event ThoughtEvent) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Registar reflexao no log metacognitivo
	entry := event.Source + ": "
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if alert, ok := payload["alert"].(string); ok {
			entry += alert
		}
	}
	sm.metacognitionLog = append(sm.metacognitionLog, entry)
	if len(sm.metacognitionLog) > sm.maxMetaLog {
		sm.metacognitionLog = sm.metacognitionLog[1:]
	}
}

// monitoringLoop captura snapshots periodicamente
func (sm *SelfModel) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.TakeHealthSnapshot()
		}
	}
}

// reflectionLoop publica reflexoes periodicas no ThoughtBus
func (sm *SelfModel) reflectionLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.publishSelfReflection()
		}
	}
}

// publishSelfReflection publica uma reflexao sobre o estado do sistema
func (sm *SelfModel) publishSelfReflection() {
	sm.reflectionCount.Add(1)

	snapshot := sm.TakeHealthSnapshot()

	// Determinar estado geral
	state := "healthy"
	salience := 0.2
	if snapshot.CognitiveLoad > 0.3 {
		state = "stressed"
		salience = 0.6
	}
	if snapshot.GoRoutines > 500 {
		state = "overloaded"
		salience = 0.8
	}

	if sm.bus != nil {
		sm.bus.Publish(ThoughtEvent{
			Source: "self_model",
			Type:   Reflection,
			Payload: map[string]interface{}{
				"reflection":     "self_state",
				"state":          state,
				"goroutines":     snapshot.GoRoutines,
				"memory_mb":      snapshot.MemoryAllocMB,
				"cognitive_load": snapshot.CognitiveLoad,
				"uptime":         snapshot.Uptime,
			},
			Salience:   salience,
			EnergyCost: 0.05, // Reflexao tem baixo custo
		})
	}
}

// GetStatistics retorna metricas do self model
func (sm *SelfModel) GetStatistics() map[string]interface{} {
	sm.mu.RLock()
	capCount := len(sm.capabilities)
	healthCount := len(sm.healthHistory)
	metaCount := len(sm.metacognitionLog)
	sm.mu.RUnlock()

	sm.goalsMu.Lock()
	goalCount := len(sm.goals)
	sm.goalsMu.Unlock()

	return map[string]interface{}{
		"engine":           "self_model",
		"uptime":           time.Since(sm.startTime).Round(time.Second).String(),
		"capabilities":     capCount,
		"goals":            goalCount,
		"reflections":      sm.reflectionCount.Load(),
		"goal_updates":     sm.goalUpdates.Load(),
		"health_snapshots": healthCount,
		"metacognition_log": metaCount,
		"identity_traits":  len(sm.identity),
	}
}
