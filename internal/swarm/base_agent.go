package swarm

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// BaseAgent fornece implementação base para todos os swarm agents
// Embed esta struct e implemente apenas Execute() e os métodos de identidade
type BaseAgent struct {
	name        string
	description string
	priority    Priority
	tools       []ToolDefinition
	toolSet     map[string]bool // lookup rápido O(1)
	deps        *Dependencies
	handlers    map[string]ToolHandler

	// Métricas
	totalCalls   atomic.Int64
	successCalls atomic.Int64
	failedCalls  atomic.Int64
	totalLatency atomic.Int64 // nanoseconds
	lastCall     time.Time
	metricsMu    sync.RWMutex
}

// ToolHandler é a função que executa uma tool específica
type ToolHandler func(ctx context.Context, call ToolCall) (*ToolResult, error)

// NewBaseAgent cria um novo BaseAgent
func NewBaseAgent(name, description string, priority Priority) *BaseAgent {
	return &BaseAgent{
		name:        name,
		description: description,
		priority:    priority,
		tools:       make([]ToolDefinition, 0),
		toolSet:     make(map[string]bool),
		handlers:    make(map[string]ToolHandler),
	}
}

// RegisterTool registra uma tool e seu handler
func (b *BaseAgent) RegisterTool(tool ToolDefinition, handler ToolHandler) {
	b.tools = append(b.tools, tool)
	b.toolSet[tool.Name] = true
	b.handlers[tool.Name] = handler
}

// Name retorna o nome do swarm
func (b *BaseAgent) Name() string { return b.name }

// Description retorna a descrição
func (b *BaseAgent) Description() string { return b.description }

// Priority retorna a prioridade
func (b *BaseAgent) Priority() Priority { return b.priority }

// Tools retorna as definições de tools
func (b *BaseAgent) Tools() []ToolDefinition { return b.tools }

// CanHandle verifica se este swarm pode executar a tool
func (b *BaseAgent) CanHandle(toolName string) bool { return b.toolSet[toolName] }

// Deps retorna as dependências
func (b *BaseAgent) Deps() *Dependencies { return b.deps }

// Init inicializa com dependencies
func (b *BaseAgent) Init(deps *Dependencies) error {
	b.deps = deps
	return nil
}

// Shutdown desliga o agent
func (b *BaseAgent) Shutdown() error { return nil }

// HealthCheck retorna status de saúde
func (b *BaseAgent) HealthCheck() HealthStatus { return HealthOK }

// Metrics retorna métricas do agent
func (b *BaseAgent) Metrics() *AgentMetrics {
	total := b.totalCalls.Load()
	var avgLatency time.Duration
	if total > 0 {
		avgLatency = time.Duration(b.totalLatency.Load() / total)
	}

	b.metricsMu.RLock()
	lastCall := b.lastCall
	b.metricsMu.RUnlock()

	return &AgentMetrics{
		TotalCalls:   total,
		SuccessCalls: b.successCalls.Load(),
		FailedCalls:  b.failedCalls.Load(),
		AvgLatency:   avgLatency,
		LastCallTime: lastCall,
	}
}

// Execute roteia para o handler correto e registra métricas
func (b *BaseAgent) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
	start := time.Now()
	b.totalCalls.Add(1)

	b.metricsMu.Lock()
	b.lastCall = start
	b.metricsMu.Unlock()

	handler, ok := b.handlers[call.Name]
	if !ok {
		b.failedCalls.Add(1)
		return nil, fmt.Errorf("handler não encontrado para tool '%s' no swarm '%s'", call.Name, b.name)
	}

	result, err := handler(ctx, call)

	latency := time.Since(start)
	b.totalLatency.Add(int64(latency))

	if err != nil {
		b.failedCalls.Add(1)
	} else {
		b.successCalls.Add(1)
	}

	return result, err
}
