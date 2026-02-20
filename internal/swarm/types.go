// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package swarm

import (
	"context"
	"time"
)

// Priority define a prioridade de execução de um swarm agent
type Priority int

const (
	PriorityLow      Priority = 0
	PriorityMedium   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "LOW"
	case PriorityMedium:
		return "MEDIUM"
	case PriorityHigh:
		return "HIGH"
	case PriorityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// HealthStatus representa o estado de saúde de um swarm agent
type HealthStatus int

const (
	HealthOK       HealthStatus = 0
	HealthDegraded HealthStatus = 1
	HealthDown     HealthStatus = 2
)

// ToolDefinition define uma ferramenta Gemini function_declaration
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Required    []string               `json:"required,omitempty"`
}

// ToolCall representa uma chamada de ferramenta recebida do LLM
type ToolCall struct {
	Name      string                 `json:"name"`
	Args      map[string]interface{} `json:"args"`
	UserID    int64
	SessionID string
	Context   *ConversationContext
}

// ConversationContext carrega o estado emocional e clínico do paciente
type ConversationContext struct {
	PatientID       int64
	PatientName     string
	EmotionalState  string
	LacanState      map[string]interface{}
	PersonalityType int
	ActiveSession   string
	Metadata        map[string]interface{}
}

// ToolResult é o resultado da execução de uma ferramenta
type ToolResult struct {
	Success     bool                   `json:"success"`
	Data        interface{}            `json:"data,omitempty"`
	Message     string                 `json:"message"`
	SuggestTone string                 `json:"suggest_tone,omitempty"`
	Handoff     *HandoffRequest        `json:"handoff,omitempty"`
	SideEffects []SideEffect           `json:"side_effects,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// HandoffRequest para transferir execução entre swarms
type HandoffRequest struct {
	TargetSwarm string                 `json:"target_swarm"`
	ToolCall    ToolCall               `json:"tool_call"`
	Reason      string                 `json:"reason"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Priority    Priority               `json:"priority"`
}

// SideEffect representa efeitos colaterais (notificações, logs, alertas)
type SideEffect struct {
	Type    string      `json:"type"` // "notification", "log", "alert", "metric"
	Payload interface{} `json:"payload"`
}

// AgentMetrics para observabilidade
type AgentMetrics struct {
	TotalCalls     int64         `json:"total_calls"`
	SuccessCalls   int64         `json:"success_calls"`
	FailedCalls    int64         `json:"failed_calls"`
	AvgLatency     time.Duration `json:"avg_latency"`
	LastCallTime   time.Time     `json:"last_call_time"`
	CircuitOpen    bool          `json:"circuit_open"`
}

// AlertFunc envia alerta real para cuidadores (push + email + SMS)
type AlertFunc func(ctx context.Context, userID int64, reason, severity string) error

// Dependencies agrupa dependências compartilhadas entre swarms
type Dependencies struct {
	DB           interface{} // *database.DB
	Nietzsche    interface{} // *nietzscheInfra.Client
	Graph        interface{} // *nietzscheInfra.GraphAdapter
	Vector       interface{} // *nietzscheInfra.VectorAdapter
	Push         interface{} // *push.FirebaseService
	Config       interface{} // *config.Config
	GoogleAPIKey string
	Krylov       interface{} // *memory.KrylovMemoryManager
	AlertFamily  AlertFunc   // Envia notificacao real para familia/cuidadores
}

// SwarmAgent é a interface que todo swarm agent deve implementar
type SwarmAgent interface {
	// Identidade
	Name() string
	Description() string
	Priority() Priority

	// Capacidades
	Tools() []ToolDefinition
	CanHandle(toolName string) bool

	// Execução
	Execute(ctx context.Context, call ToolCall) (*ToolResult, error)

	// Lifecycle
	Init(deps *Dependencies) error
	Shutdown() error

	// Observabilidade
	HealthCheck() HealthStatus
	Metrics() *AgentMetrics
}
