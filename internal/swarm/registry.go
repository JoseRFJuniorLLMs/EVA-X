// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package swarm

import (
	"fmt"
	"log"
	"sync"
)

// Registry mantém o registro de todos os swarm agents e mapeia tools → swarms
type Registry struct {
	mu       sync.RWMutex
	swarms   map[string]SwarmAgent   // name → agent
	toolMap  map[string]string       // toolName → swarmName
	ordered  []SwarmAgent            // ordenado por prioridade (desc)
}

// NewRegistry cria um novo registro de swarms
func NewRegistry() *Registry {
	return &Registry{
		swarms:  make(map[string]SwarmAgent),
		toolMap: make(map[string]string),
	}
}

// Register adiciona um swarm agent ao registry
func (r *Registry) Register(agent SwarmAgent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := agent.Name()
	if _, exists := r.swarms[name]; exists {
		return fmt.Errorf("swarm '%s' já registrado", name)
	}

	// Registrar swarm
	r.swarms[name] = agent

	// Mapear cada tool deste swarm
	for _, tool := range agent.Tools() {
		if existing, ok := r.toolMap[tool.Name]; ok {
			return fmt.Errorf("tool '%s' já registrada no swarm '%s', conflito com '%s'", tool.Name, existing, name)
		}
		r.toolMap[tool.Name] = name
	}

	// Recalcular ordem por prioridade
	r.rebuildOrder()

	log.Printf("✅ [SWARM] Registrado: %s (%s) - %d tools, prioridade %s",
		name, agent.Description(), len(agent.Tools()), agent.Priority())

	return nil
}

// FindSwarm retorna o swarm responsável por uma tool (O(1) lookup)
func (r *Registry) FindSwarm(toolName string) (SwarmAgent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	swarmName, ok := r.toolMap[toolName]
	if !ok {
		return nil, fmt.Errorf("nenhum swarm registrado para tool '%s'", toolName)
	}

	return r.swarms[swarmName], nil
}

// GetSwarm retorna um swarm pelo nome
func (r *Registry) GetSwarm(name string) (SwarmAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.swarms[name]
	return agent, ok
}

// AllSwarms retorna todos os swarms ordenados por prioridade
func (r *Registry) AllSwarms() []SwarmAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]SwarmAgent, len(r.ordered))
	copy(result, r.ordered)
	return result
}

// AllTools retorna todas as tool definitions de todos os swarms
// no formato esperado pelo Gemini (function_declarations)
func (r *Registry) AllTools() []interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var declarations []interface{}

	for _, agent := range r.ordered {
		for _, tool := range agent.Tools() {
			params := map[string]interface{}{
				"type":       "object",
				"properties": tool.Parameters,
			}
			if len(tool.Required) > 0 {
				params["required"] = tool.Required
			}

			declarations = append(declarations, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  params,
			})
		}
	}

	// Agrupar em batches de 10 (limite Gemini por function_declarations block)
	var result []interface{}
	batchSize := 10
	for i := 0; i < len(declarations); i += batchSize {
		end := i + batchSize
		if end > len(declarations) {
			end = len(declarations)
		}
		result = append(result, map[string]interface{}{
			"function_declarations": declarations[i:end],
		})
	}

	return result
}

// ToolCount retorna total de tools registradas
func (r *Registry) ToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.toolMap)
}

// SwarmCount retorna total de swarms registrados
func (r *Registry) SwarmCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.swarms)
}

// rebuildOrder reordena swarms por prioridade (maior primeiro)
func (r *Registry) rebuildOrder() {
	r.ordered = make([]SwarmAgent, 0, len(r.swarms))
	for _, agent := range r.swarms {
		r.ordered = append(r.ordered, agent)
	}
	// Bubble sort simples (poucos swarms, ~8)
	for i := 0; i < len(r.ordered); i++ {
		for j := i + 1; j < len(r.ordered); j++ {
			if r.ordered[j].Priority() > r.ordered[i].Priority() {
				r.ordered[i], r.ordered[j] = r.ordered[j], r.ordered[i]
			}
		}
	}
}
