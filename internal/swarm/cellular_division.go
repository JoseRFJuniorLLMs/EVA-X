// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package swarm

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// CellularSwarm implementa crescimento organico de agentes inspirado em divisao celular
// Agentes se multiplicam quando carga > threshold e se retraem quando ociosos
// Analogia: Meristemas (pontos de crescimento) em plantas
type CellularSwarm struct {
	orchestrator *Orchestrator
	agents       []*ManagedAgent
	divisions    int // Numero de divisoes ja realizadas
	maxDivisions int // Maximo de divisoes permitidas

	// Regras de divisao (L-System simplificado)
	divisionRules map[string][]string // agentType -> [filhoA, filhoB]

	// Metricas
	loadHistory  []float64
	mu           sync.RWMutex
}

// ManagedAgent agente gerenciado com metricas de carga
type ManagedAgent struct {
	Agent     SwarmAgent
	Load      float64   // 0-1 carga atual
	Born      time.Time // Quando foi criado
	Parent    string    // Nome do agente pai (se dividido)
	Generation int      // Geracao (0 = original, 1 = filho, 2 = neto...)
}

// DivisionEvent evento de divisao celular
type DivisionEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ParentAgent  string    `json:"parent_agent"`
	ChildAgents  []string  `json:"child_agents"`
	TriggerLoad  float64   `json:"trigger_load"`
	Generation   int       `json:"generation"`
}

// CellularStats estatisticas do swarm celular
type CellularStats struct {
	TotalAgents    int               `json:"total_agents"`
	TotalDivisions int               `json:"total_divisions"`
	MaxDivisions   int               `json:"max_divisions"`
	AvgLoad        float64           `json:"avg_load"`
	Agents         []AgentStatus     `json:"agents"`
}

// AgentStatus status de um agente
type AgentStatus struct {
	Name       string  `json:"name"`
	Load       float64 `json:"load"`
	Generation int     `json:"generation"`
	Health     string  `json:"health"`
}

// NewCellularSwarm cria um swarm com capacidade de divisao celular
func NewCellularSwarm(orchestrator *Orchestrator) *CellularSwarm {
	cs := &CellularSwarm{
		orchestrator: orchestrator,
		agents:       make([]*ManagedAgent, 0),
		maxDivisions: 3, // Max 3 divisoes (1->2->4->8)
		divisionRules: map[string][]string{
			// Regras de divisao por tipo de agente
			"emergency":  {"emergency", "clinical"},      // Emergencia divide em emergencia + clinico
			"clinical":   {"clinical", "wellness"},       // Clinico divide em clinico + wellness
			"wellness":   {"wellness"},                   // Wellness nao divide mais (folha)
			"google":     {"google", "productivity"},     // Google divide em google + produtividade
			"productivity": {"productivity"},             // Produtividade nao divide
			"entertainment": {"entertainment"},           // Entertainment nao divide
			"kids":       {"kids"},                       // Kids nao divide
			"external":   {"external"},                   // External nao divide
		},
	}

	return cs
}

// RegisterAgent registra um agente existente no gerenciamento celular
func (cs *CellularSwarm) RegisterAgent(agent SwarmAgent) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.agents = append(cs.agents, &ManagedAgent{
		Agent:      agent,
		Load:       0.0,
		Born:       time.Now(),
		Generation: 0,
	})
}

// UpdateLoad atualiza a carga de um agente
func (cs *CellularSwarm) UpdateLoad(agentName string, load float64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, ma := range cs.agents {
		if ma.Agent.Name() == agentName {
			ma.Load = load
			break
		}
	}

	cs.loadHistory = append(cs.loadHistory, load)
	if len(cs.loadHistory) > 100 {
		cs.loadHistory = cs.loadHistory[1:]
	}
}

// GrowIfNeeded verifica se algum agente precisa dividir
func (cs *CellularSwarm) GrowIfNeeded(ctx context.Context) []DivisionEvent {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.divisions >= cs.maxDivisions {
		return nil
	}

	var events []DivisionEvent

	for _, ma := range cs.agents {
		if ma.Load > 0.8 {
			// Agente sobrecarregado -> tentar dividir
			rules, hasRules := cs.divisionRules[ma.Agent.Name()]
			if !hasRules || len(rules) <= 1 {
				continue // Agente folha, nao pode dividir
			}

			event := DivisionEvent{
				Timestamp:   time.Now(),
				ParentAgent: ma.Agent.Name(),
				TriggerLoad: ma.Load,
				Generation:  ma.Generation + 1,
			}

			for _, childType := range rules {
				event.ChildAgents = append(event.ChildAgents, childType)
			}

			events = append(events, event)
			cs.divisions++

			log.Printf("[CELLULAR] Divisao: %s (carga=%.1f%%) -> %v (geracao %d, divisao %d/%d)",
				ma.Agent.Name(), ma.Load*100, rules, ma.Generation+1,
				cs.divisions, cs.maxDivisions)

			break // Uma divisao por ciclo
		}
	}

	return events
}

// ShrinkIfIdle remove agentes ociosos de geracoes > 0 (nao remove originais)
func (cs *CellularSwarm) ShrinkIfIdle() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	removed := 0
	active := make([]*ManagedAgent, 0, len(cs.agents))

	for _, ma := range cs.agents {
		if ma.Generation > 0 && ma.Load < 0.1 {
			// Agente filho ocioso -> retrair
			log.Printf("[CELLULAR] Retracao: agente %s (geracao %d) removido por ociosidade",
				ma.Agent.Name(), ma.Generation)
			removed++
			continue
		}
		active = append(active, ma)
	}

	cs.agents = active
	return removed
}

// MeasureOverallLoad retorna carga media de todos os agentes
func (cs *CellularSwarm) MeasureOverallLoad() float64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if len(cs.agents) == 0 {
		return 0
	}

	totalLoad := 0.0
	for _, ma := range cs.agents {
		totalLoad += ma.Load
	}

	return totalLoad / float64(len(cs.agents))
}

// GetStats retorna estatisticas completas do swarm celular
func (cs *CellularSwarm) GetStats() *CellularStats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	stats := &CellularStats{
		TotalAgents:    len(cs.agents),
		TotalDivisions: cs.divisions,
		MaxDivisions:   cs.maxDivisions,
		AvgLoad:        cs.MeasureOverallLoadUnsafe(),
	}

	for _, ma := range cs.agents {
		health := "healthy"
		if ma.Load > 0.8 {
			health = "overloaded"
		} else if ma.Load > 0.6 {
			health = "busy"
		} else if ma.Load < 0.1 {
			health = "idle"
		}

		stats.Agents = append(stats.Agents, AgentStatus{
			Name:       ma.Agent.Name(),
			Load:       ma.Load,
			Generation: ma.Generation,
			Health:     health,
		})
	}

	return stats
}

// MeasureOverallLoadUnsafe sem lock (chamador deve ter lock)
func (cs *CellularSwarm) MeasureOverallLoadUnsafe() float64 {
	if len(cs.agents) == 0 {
		return 0
	}

	totalLoad := 0.0
	for _, ma := range cs.agents {
		totalLoad += ma.Load
	}

	return totalLoad / float64(len(cs.agents))
}

// GetStatistics retorna estatisticas no formato padrao
func (cs *CellularSwarm) GetStatistics() map[string]interface{} {
	stats := cs.GetStats()

	agentList := make([]map[string]interface{}, len(stats.Agents))
	for i, a := range stats.Agents {
		agentList[i] = map[string]interface{}{
			"name":       a.Name,
			"load":       fmt.Sprintf("%.0f%%", a.Load*100),
			"generation": a.Generation,
			"health":     a.Health,
		}
	}

	return map[string]interface{}{
		"engine":          "cellular_swarm",
		"total_agents":    stats.TotalAgents,
		"total_divisions": stats.TotalDivisions,
		"max_divisions":   stats.MaxDivisions,
		"avg_load":        fmt.Sprintf("%.0f%%", stats.AvgLoad*100),
		"agents":          agentList,
		"status":          "active",
	}
}
