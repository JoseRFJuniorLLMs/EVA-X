// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consciousness

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ============================================================================
// Agent Runtime — Phase 6 do Cognitive Operating System (COS)
// ============================================================================
//
// O Agent Runtime e o subsistema que gere agentes cognitivos dentro do COS.
// Agentes sao processos autonomos que:
//
//   - Subscrevem ao ThoughtBus para receber estimulos relevantes
//   - Publicam resultados de volta no ThoughtBus
//   - Tem contexto proprio (memoria local, estado, permissoes)
//   - Podem ser criados, pausados, resumidos e destruidos em runtime
//
// Diferenca entre Daemons e Agents:
//   - Daemons sao processos de MANUTENCAO (background, periodicos)
//   - Agents sao processos de ACCAO (reactivos, orientados a objectivos)
//
// O Agent Runtime integra com o Swarm System existente, permitindo que
// Swarm Agents participem no barramento cognitivo.
//
// Ciencia: Minsky (1986) Society of Mind, Brooks (1991) Subsumption Architecture
// Engineering: Registry pattern + ThoughtBus pub/sub + context propagation

// AgentState estado do ciclo de vida do agente
type AgentState string

const (
	AgentIdle     AgentState = "idle"
	AgentActive   AgentState = "active"
	AgentBusy     AgentState = "busy"
	AgentPaused   AgentState = "paused"
	AgentRetired  AgentState = "retired"
)

// CognitiveAgent interface para agentes que participam no COS
type CognitiveAgent interface {
	// Identidade
	AgentID() string
	AgentName() string
	AgentDescription() string

	// Capacidades
	SubscribedTypes() []ThoughtType // Tipos de pensamentos que o agente quer receber
	CanProcess(event ThoughtEvent) bool

	// Processamento
	Process(ctx context.Context, event ThoughtEvent) (*ThoughtEvent, error)

	// Lifecycle
	OnActivate(ctx context.Context) error
	OnDeactivate() error

	// Observabilidade
	CurrentState() AgentState
	AgentStatistics() map[string]interface{}
}

// AgentContext contexto de execucao de um agente
type AgentContext struct {
	AgentID     string                 `json:"agent_id"`
	SessionID   string                 `json:"session_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Priority    int                    `json:"priority"`     // 0=low, 3=critical
	Memory      map[string]interface{} `json:"memory"`       // Memoria local do agente
	Permissions []string               `json:"permissions"`  // Permissoes de accao
	CreatedAt   time.Time              `json:"created_at"`
}

// AgentRegistration registo de um agente no runtime
type AgentRegistration struct {
	Agent     CognitiveAgent
	Context   *AgentContext
	Cancel    context.CancelFunc
	Activated time.Time
}

// AgentRuntime gere o ciclo de vida de agentes cognitivos
type AgentRuntime struct {
	bus      *ThoughtBus
	agents   map[string]*AgentRegistration
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc

	// Metricas
	totalRegistered   atomic.Int64
	totalActivated    atomic.Int64
	totalDeactivated  atomic.Int64
	totalProcessed    atomic.Int64
	totalErrors       atomic.Int64

	// Rate limiting
	maxAgents   int
	maxBusyTime time.Duration
}

// NewAgentRuntime cria o runtime de agentes cognitivos
func NewAgentRuntime(bus *ThoughtBus) *AgentRuntime {
	return &AgentRuntime{
		bus:         bus,
		agents:      make(map[string]*AgentRegistration),
		maxAgents:   50,
		maxBusyTime: 30 * time.Second,
	}
}

// Start inicia o runtime
func (ar *AgentRuntime) Start(ctx context.Context) {
	ar.ctx, ar.cancel = context.WithCancel(ctx)

	// Subscrever ao ThoughtBus como dispatcher global
	if ar.bus != nil {
		ar.bus.Subscribe(Global, ar.dispatch)
	}

	// Goroutine de health check periodico
	go ar.healthCheckLoop(ar.ctx)

	log.Info().Int("max_agents", ar.maxAgents).Msg("[AgentRuntime] Runtime de agentes iniciado")
}

// Stop para o runtime e desactiva todos os agentes
func (ar *AgentRuntime) Stop() {
	if ar.cancel != nil {
		ar.cancel()
	}

	ar.mu.Lock()
	defer ar.mu.Unlock()

	for id, reg := range ar.agents {
		if reg.Cancel != nil {
			reg.Cancel()
		}
		if err := reg.Agent.OnDeactivate(); err != nil {
			log.Error().Err(err).Str("agent", id).Msg("[AgentRuntime] Erro ao desactivar agente")
		}
	}

	log.Info().Int("agents", len(ar.agents)).Msg("[AgentRuntime] Runtime parado")
}

// Register regista um agente no runtime
func (ar *AgentRuntime) Register(agent CognitiveAgent, agentCtx *AgentContext) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if len(ar.agents) >= ar.maxAgents {
		log.Warn().Str("agent", agent.AgentName()).Msg("[AgentRuntime] Limite de agentes atingido")
		return nil
	}

	id := agent.AgentID()
	if id == "" {
		id = uuid.New().String()
	}

	if agentCtx == nil {
		agentCtx = &AgentContext{
			AgentID:   id,
			Memory:    make(map[string]interface{}),
			CreatedAt: time.Now(),
		}
	}

	ar.agents[id] = &AgentRegistration{
		Agent:   agent,
		Context: agentCtx,
	}
	ar.totalRegistered.Add(1)

	log.Info().
		Str("agent_id", id).
		Str("name", agent.AgentName()).
		Msg("[AgentRuntime] Agente registado")

	return nil
}

// Activate activa um agente (comeca a processar eventos)
func (ar *AgentRuntime) Activate(agentID string) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	reg, ok := ar.agents[agentID]
	if !ok {
		return nil
	}

	agentCtx, cancel := context.WithCancel(ar.ctx)
	reg.Cancel = cancel
	reg.Activated = time.Now()

	go func() {
		if err := reg.Agent.OnActivate(agentCtx); err != nil {
			log.Error().Err(err).Str("agent", agentID).Msg("[AgentRuntime] Erro ao activar agente")
		}
	}()

	ar.totalActivated.Add(1)

	log.Info().Str("agent_id", agentID).Msg("[AgentRuntime] Agente activado")
	return nil
}

// Deactivate desactiva um agente
func (ar *AgentRuntime) Deactivate(agentID string) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	reg, ok := ar.agents[agentID]
	if !ok {
		return nil
	}

	if reg.Cancel != nil {
		reg.Cancel()
	}
	if err := reg.Agent.OnDeactivate(); err != nil {
		log.Error().Err(err).Str("agent", agentID).Msg("[AgentRuntime] Erro ao desactivar agente")
	}

	ar.totalDeactivated.Add(1)

	log.Info().Str("agent_id", agentID).Msg("[AgentRuntime] Agente desactivado")
	return nil
}

// Unregister remove um agente do runtime
func (ar *AgentRuntime) Unregister(agentID string) {
	ar.Deactivate(agentID)

	ar.mu.Lock()
	defer ar.mu.Unlock()
	delete(ar.agents, agentID)

	log.Info().Str("agent_id", agentID).Msg("[AgentRuntime] Agente removido")
}

// dispatch encaminha ThoughtEvents para agentes interessados
func (ar *AgentRuntime) dispatch(event ThoughtEvent) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	for _, reg := range ar.agents {
		agent := reg.Agent
		if agent.CurrentState() != AgentActive {
			continue
		}

		// Verificar se o agente esta interessado neste tipo de evento
		interested := false
		for _, t := range agent.SubscribedTypes() {
			if t == event.Type || t == Global {
				interested = true
				break
			}
		}

		if !interested || !agent.CanProcess(event) {
			continue
		}

		// Processar em goroutine isolada com timeout
		go func(a CognitiveAgent, e ThoughtEvent) {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Str("agent", a.AgentName()).
						Interface("panic", r).
						Msg("[AgentRuntime] Panic em agente (recuperado)")
					ar.totalErrors.Add(1)
				}
			}()

			ctx, cancel := context.WithTimeout(ar.ctx, ar.maxBusyTime)
			defer cancel()

			result, err := a.Process(ctx, e)
			if err != nil {
				ar.totalErrors.Add(1)
				log.Error().Err(err).Str("agent", a.AgentName()).Msg("[AgentRuntime] Erro ao processar evento")
				return
			}

			ar.totalProcessed.Add(1)

			// Se o agente produziu output, publicar no ThoughtBus
			if result != nil && ar.bus != nil {
				ar.bus.Publish(*result)
			}
		}(agent, event)
	}
}

// healthCheckLoop verifica periodicamente a saude dos agentes
func (ar *AgentRuntime) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ar.checkAgentHealth()
		}
	}
}

func (ar *AgentRuntime) checkAgentHealth() {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	for id, reg := range ar.agents {
		state := reg.Agent.CurrentState()

		// Detectar agentes bloqueados (busy por muito tempo)
		if state == AgentBusy && !reg.Activated.IsZero() {
			busyDuration := time.Since(reg.Activated)
			if busyDuration > ar.maxBusyTime*2 {
				log.Warn().
					Str("agent_id", id).
					Dur("busy_for", busyDuration).
					Msg("[AgentRuntime] Agente potencialmente bloqueado")
			}
		}
	}
}

// ListAgents retorna lista de agentes registados
func (ar *AgentRuntime) ListAgents() []map[string]interface{} {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	list := make([]map[string]interface{}, 0, len(ar.agents))
	for id, reg := range ar.agents {
		list = append(list, map[string]interface{}{
			"id":          id,
			"name":        reg.Agent.AgentName(),
			"description": reg.Agent.AgentDescription(),
			"state":       string(reg.Agent.CurrentState()),
			"subscribed":  reg.Agent.SubscribedTypes(),
			"activated":   reg.Activated,
		})
	}
	return list
}

// GetStatistics retorna metricas do runtime
func (ar *AgentRuntime) GetStatistics() map[string]interface{} {
	ar.mu.RLock()
	agentCount := len(ar.agents)
	activeCount := 0
	for _, reg := range ar.agents {
		if reg.Agent.CurrentState() == AgentActive {
			activeCount++
		}
	}
	ar.mu.RUnlock()

	return map[string]interface{}{
		"engine":            "agent_runtime",
		"total_registered":  ar.totalRegistered.Load(),
		"total_activated":   ar.totalActivated.Load(),
		"total_deactivated": ar.totalDeactivated.Load(),
		"total_processed":   ar.totalProcessed.Load(),
		"total_errors":      ar.totalErrors.Load(),
		"current_agents":    agentCount,
		"active_agents":     activeCount,
		"max_agents":        ar.maxAgents,
	}
}

// ============================================================================
// SwarmBridgeAgent — Adaptador para conectar Swarm Agents ao COS
// ============================================================================
// Permite que swarm agents existentes participem no ThoughtBus sem
// modificar o codigo original do swarm system.

// SwarmBridgeAgent adapta um swarm agent para o COS
type SwarmBridgeAgent struct {
	id          string
	name        string
	description string
	state       AgentState
	mu          sync.Mutex

	// Configuracao
	subscribedTypes []ThoughtType

	// Callback para o Swarm Orchestrator
	onIntent func(event ThoughtEvent) (*ThoughtEvent, error)

	// Metricas
	processed atomic.Int64
	errors    atomic.Int64
}

// NewSwarmBridgeAgent cria um bridge entre swarm e COS
func NewSwarmBridgeAgent(name, description string, onIntent func(ThoughtEvent) (*ThoughtEvent, error)) *SwarmBridgeAgent {
	return &SwarmBridgeAgent{
		id:              uuid.New().String(),
		name:            name,
		description:     description,
		state:           AgentIdle,
		subscribedTypes: []ThoughtType{Intent}, // Swarm agents respondem a intencoes
		onIntent:        onIntent,
	}
}

func (a *SwarmBridgeAgent) AgentID() string          { return a.id }
func (a *SwarmBridgeAgent) AgentName() string         { return a.name }
func (a *SwarmBridgeAgent) AgentDescription() string  { return a.description }
func (a *SwarmBridgeAgent) SubscribedTypes() []ThoughtType { return a.subscribedTypes }

func (a *SwarmBridgeAgent) CanProcess(event ThoughtEvent) bool {
	return event.Type == Intent
}

func (a *SwarmBridgeAgent) Process(ctx context.Context, event ThoughtEvent) (*ThoughtEvent, error) {
	if a.onIntent == nil {
		return nil, nil
	}
	a.processed.Add(1)
	result, err := a.onIntent(event)
	if err != nil {
		a.errors.Add(1)
	}
	return result, err
}

func (a *SwarmBridgeAgent) OnActivate(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = AgentActive
	return nil
}

func (a *SwarmBridgeAgent) OnDeactivate() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = AgentRetired
	return nil
}

func (a *SwarmBridgeAgent) CurrentState() AgentState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

func (a *SwarmBridgeAgent) AgentStatistics() map[string]interface{} {
	return map[string]interface{}{
		"id":        a.id,
		"name":      a.name,
		"state":     string(a.state),
		"processed": a.processed.Load(),
		"errors":    a.errors.Load(),
	}
}
