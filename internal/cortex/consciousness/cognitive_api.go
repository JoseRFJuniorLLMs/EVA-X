// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consciousness

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// Cognitive APIs — Phase 7 do Cognitive Operating System (COS)
// ============================================================================
//
// APIs REST que expoe o estado do COS para monitoramento, debugging e
// integracao com ferramentas externas.
//
// Endpoints:
//   GET  /cos/status          — Estado geral do COS
//   GET  /cos/thought/stream  — Stream de pensamentos (SSE)
//   POST /cos/thought/publish — Publicar pensamento externo
//   GET  /cos/attention       — Estado do attention scheduler
//   GET  /cos/memory          — Estatisticas do kernel de memoria
//   GET  /cos/daemons         — Estado dos daemons cognitivos
//   GET  /cos/agents          — Lista de agentes registados
//   GET  /cos/evolution       — Estado do graph evolution engine
//   GET  /cos/self            — Introspeccao do self model
//   GET  /cos/health          — Health snapshot do sistema

// CognitiveAPI expoe o COS via REST
type CognitiveAPI struct {
	bus             *ThoughtBus
	workspace       *GlobalWorkspace
	memoryKernel    *MemoryKernel
	daemons         *DaemonManager
	agentRuntime    *AgentRuntime
	graphEvolution  *GraphEvolutionEngine
	selfModel       *SelfModel
}

// NewCognitiveAPI cria a API cognitiva
func NewCognitiveAPI(
	bus *ThoughtBus,
	workspace *GlobalWorkspace,
	mk *MemoryKernel,
	daemons *DaemonManager,
	agents *AgentRuntime,
	evolution *GraphEvolutionEngine,
	selfModel *SelfModel,
) *CognitiveAPI {
	return &CognitiveAPI{
		bus:            bus,
		workspace:      workspace,
		memoryKernel:   mk,
		daemons:        daemons,
		agentRuntime:   agents,
		graphEvolution: evolution,
		selfModel:      selfModel,
	}
}

// RouteRegistrar funcao que regista uma rota (compativel com gorilla/mux e http.ServeMux)
type RouteRegistrar func(pattern string, handler func(http.ResponseWriter, *http.Request))

// RegisterRoutes regista as rotas da COS API usando um registrador de rotas
func (api *CognitiveAPI) RegisterRoutes(register RouteRegistrar) {
	register("/cos/status", api.handleStatus)
	register("/cos/thought/publish", api.handlePublishThought)
	register("/cos/attention", api.handleAttention)
	register("/cos/memory", api.handleMemory)
	register("/cos/memory/store", api.handleMemoryStore)
	register("/cos/memory/query", api.handleMemoryQuery)
	register("/cos/daemons", api.handleDaemons)
	register("/cos/agents", api.handleAgents)
	register("/cos/evolution", api.handleEvolution)
	register("/cos/self", api.handleSelf)
	register("/cos/health", api.handleHealth)

	log.Info().Msg("[CognitiveAPI] Rotas COS registadas (/cos/*)")
}

// handleStatus GET /cos/status — Estado geral do COS
func (api *CognitiveAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := map[string]interface{}{
		"engine": "cognitive_operating_system",
		"version": "1.0.0",
		"status":  "active",
	}

	if api.bus != nil {
		status["thought_bus"] = api.bus.Metrics()
	}
	if api.workspace != nil {
		status["global_workspace"] = api.workspace.GetStatistics()
	}
	if api.memoryKernel != nil {
		status["memory_kernel"] = api.memoryKernel.GetStatistics()
	}
	if api.daemons != nil {
		status["daemons"] = api.daemons.GetStatistics()
	}
	if api.agentRuntime != nil {
		status["agent_runtime"] = api.agentRuntime.GetStatistics()
	}
	if api.graphEvolution != nil {
		status["graph_evolution"] = api.graphEvolution.GetStatistics()
	}
	if api.selfModel != nil {
		status["self_model"] = api.selfModel.GetStatistics()
	}

	writeJSON(w, status)
}

// handlePublishThought POST /cos/thought/publish — Publicar pensamento externo
func (api *CognitiveAPI) handlePublishThought(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.bus == nil {
		http.Error(w, "ThoughtBus not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Source    string      `json:"source"`
		Type     string      `json:"type"`
		Payload  interface{} `json:"payload"`
		Salience float64     `json:"salience"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Source == "" {
		req.Source = "external_api"
	}
	if req.Salience <= 0 {
		req.Salience = 0.5
	}

	thoughtType := ThoughtType(req.Type)
	switch thoughtType {
	case Perception, Inference, Intent, Reflection, Memory, Emotion:
		// Valid
	default:
		thoughtType = Perception // Default to perception
	}

	event := NewThought(req.Source, thoughtType, req.Payload, req.Salience)
	api.bus.Publish(event)

	writeJSON(w, map[string]interface{}{
		"published": true,
		"thought_id": event.ID,
	})
}

// handleAttention GET /cos/attention — Estado do attention scheduler
func (api *CognitiveAPI) handleAttention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := map[string]interface{}{}

	if api.workspace != nil {
		result = api.workspace.GetStatistics()

		focus := api.workspace.GetCurrentFocus()
		if focus != nil {
			result["current_focus"] = map[string]interface{}{
				"source":    focus.Source,
				"type":      string(focus.Type),
				"salience":  focus.Salience,
				"timestamp": focus.Timestamp,
			}
		}
	}

	writeJSON(w, result)
}

// handleMemory GET /cos/memory — Estatisticas do kernel de memoria
func (api *CognitiveAPI) handleMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.memoryKernel == nil {
		writeJSON(w, map[string]interface{}{"status": "unavailable"})
		return
	}

	writeJSON(w, api.memoryKernel.GetStatistics())
}

// handleMemoryStore POST /cos/memory/store — Armazenar memoria
func (api *CognitiveAPI) handleMemoryStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.memoryKernel == nil {
		http.Error(w, "Memory kernel not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Content   string     `json:"content"`
		Zone      string     `json:"zone"`
		Energy    float64    `json:"energy"`
		UserID    string     `json:"user_id"`
		SessionID string     `json:"session_id"`
		Tags      []string   `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	zone := MemoryZone(req.Zone)
	if zone == "" {
		zone = WorkingMemory
	}

	if req.Energy <= 0 {
		req.Energy = 0.5
	}

	trace := &MemoryTrace{
		Zone:      zone,
		Content:   req.Content,
		Energy:    req.Energy,
		Activation: req.Energy,
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Tags:      req.Tags,
	}

	if err := api.memoryKernel.Store(trace); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"stored":   true,
		"trace_id": trace.ID,
		"zone":     string(trace.Zone),
	})
}

// handleMemoryQuery POST /cos/memory/query — Buscar memorias
func (api *CognitiveAPI) handleMemoryQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.memoryKernel == nil {
		http.Error(w, "Memory kernel not available", http.StatusServiceUnavailable)
		return
	}

	query := &MemoryQuery{Limit: 10}

	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(query); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
	} else {
		// GET params
		query.UserID = r.URL.Query().Get("user_id")
		query.Zone = MemoryZone(r.URL.Query().Get("zone"))
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				query.Limit = limit
			}
		}
	}

	result := api.memoryKernel.Retrieve(query)

	writeJSON(w, map[string]interface{}{
		"traces":           result.Traces,
		"count":            len(result.Traces),
		"spread_activated": result.SpreadActivated,
		"query_time_ms":    result.QueryTime.Milliseconds(),
	})
}

// handleDaemons GET /cos/daemons — Estado dos daemons cognitivos
func (api *CognitiveAPI) handleDaemons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.daemons == nil {
		writeJSON(w, map[string]interface{}{"status": "unavailable"})
		return
	}

	writeJSON(w, api.daemons.GetStatistics())
}

// handleAgents GET /cos/agents — Lista de agentes registados
func (api *CognitiveAPI) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.agentRuntime == nil {
		writeJSON(w, map[string]interface{}{"status": "unavailable"})
		return
	}

	result := map[string]interface{}{
		"statistics": api.agentRuntime.GetStatistics(),
		"agents":     api.agentRuntime.ListAgents(),
	}

	writeJSON(w, result)
}

// handleEvolution GET /cos/evolution — Estado do graph evolution engine
func (api *CognitiveAPI) handleEvolution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.graphEvolution == nil {
		writeJSON(w, map[string]interface{}{"status": "unavailable"})
		return
	}

	writeJSON(w, api.graphEvolution.GetStatistics())
}

// handleSelf GET /cos/self — Introspeccao do self model
func (api *CognitiveAPI) handleSelf(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.selfModel == nil {
		writeJSON(w, map[string]interface{}{"status": "unavailable"})
		return
	}

	writeJSON(w, api.selfModel.Introspect())
}

// handleHealth GET /cos/health — Health snapshot do sistema
func (api *CognitiveAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.selfModel == nil {
		writeJSON(w, map[string]interface{}{"status": "unavailable"})
		return
	}

	snapshot := api.selfModel.TakeHealthSnapshot()
	writeJSON(w, snapshot)
}

// writeJSON helper para escrever resposta JSON
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("[CognitiveAPI] Erro ao serializar resposta")
	}
}
