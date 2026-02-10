package productivity

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
)

// Agent implementa o ProductivitySwarm - agendamentos, alarmes, GTD, memória
type Agent struct {
	*swarm.BaseAgent
}

// New cria o ProductivitySwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"productivity",
			"Agendamentos, alarmes, GTD, repetição espaçada",
			swarm.PriorityMedium,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	// Scheduling (com flow de confirmação)
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "schedule_appointment",
		Description: "Agenda compromisso (requer confirmação prévia)",
		Parameters: map[string]interface{}{
			"timestamp":   map[string]interface{}{"type": "string", "description": "Data/hora ISO 8601"},
			"type":        map[string]interface{}{"type": "string", "description": "Tipo", "enum": []string{"consulta", "medicamento", "ligacao", "atividade", "outro"}},
			"description": map[string]interface{}{"type": "string", "description": "Descrição"},
		},
		Required: []string{"timestamp", "type", "description"},
	}, a.handleSchedule("create"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "pending_schedule",
		Description: "Registra agendamento pendente de confirmação",
		Parameters: map[string]interface{}{
			"timestamp":   map[string]interface{}{"type": "string", "description": "Data/hora ISO 8601"},
			"type":        map[string]interface{}{"type": "string", "description": "Tipo do agendamento"},
			"description": map[string]interface{}{"type": "string", "description": "Descrição"},
		},
	}, a.handleSchedule("pending"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "confirm_schedule",
		Description: "Confirma ou nega agendamento pendente",
		Parameters: map[string]interface{}{
			"confirmed": map[string]interface{}{"type": "boolean", "description": "true=confirmar, false=cancelar"},
		},
		Required: []string{"confirmed"},
	}, a.handleConfirmSchedule)

	// Alarmes
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "set_alarm",
		Description: "Configurar alarme",
		Parameters: map[string]interface{}{
			"time":        map[string]interface{}{"type": "string", "description": "Horário HH:MM"},
			"label":       map[string]interface{}{"type": "string", "description": "Descrição"},
			"repeat_days": map[string]interface{}{"type": "array", "description": "Dias de repetição"},
		},
		Required: []string{"time"},
	}, a.handleAlarm("set"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "cancel_alarm",
		Description: "Cancelar alarme",
		Parameters: map[string]interface{}{
			"alarm_id": map[string]interface{}{"type": "string", "description": "ID do alarme ou 'all'"},
		},
		Required: []string{"alarm_id"},
	}, a.handleAlarm("cancel"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "list_alarms",
		Description: "Listar alarmes ativos",
		Parameters:  map[string]interface{}{},
	}, a.handleAlarm("list"))

	// GTD
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "capture_task",
		Description: "Capturar preocupação/tarefa e transformar em ação",
		Parameters: map[string]interface{}{
			"raw_input":   map[string]interface{}{"type": "string", "description": "O que o idoso disse"},
			"context":     map[string]interface{}{"type": "string", "description": "Contexto"},
			"next_action": map[string]interface{}{"type": "string", "description": "Ação concreta"},
			"due_date":    map[string]interface{}{"type": "string", "description": "Data sugerida"},
			"project":     map[string]interface{}{"type": "string", "description": "Projeto maior"},
		},
		Required: []string{"raw_input"},
	}, a.handleGTD("capture"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "list_tasks",
		Description: "Listar próximas ações pendentes",
		Parameters: map[string]interface{}{
			"context": map[string]interface{}{"type": "string", "description": "Filtrar por contexto"},
			"limit":   map[string]interface{}{"type": "integer", "description": "Máximo de tarefas"},
		},
	}, a.handleGTD("list"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "complete_task",
		Description: "Marcar tarefa como concluída",
		Parameters: map[string]interface{}{
			"task_description": map[string]interface{}{"type": "string", "description": "Descrição da tarefa"},
		},
	}, a.handleGTD("complete"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "clarify_task",
		Description: "Pedir mais info para definir ação",
		Parameters: map[string]interface{}{
			"task_id":  map[string]interface{}{"type": "string", "description": "ID da tarefa"},
			"question": map[string]interface{}{"type": "string", "description": "Pergunta"},
		},
	}, a.handleGTD("clarify"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "weekly_review",
		Description: "Revisão semanal GTD",
		Parameters:  map[string]interface{}{},
	}, a.handleGTD("review"))

	// Spaced Repetition
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "remember_this",
		Description: "Capturar informação para reforço de memória",
		Parameters: map[string]interface{}{
			"content":    map[string]interface{}{"type": "string", "description": "O que lembrar"},
			"category":   map[string]interface{}{"type": "string", "description": "location, medication, person, event, routine, general"},
			"trigger":    map[string]interface{}{"type": "string", "description": "O que disparou"},
			"importance": map[string]interface{}{"type": "integer", "description": "1-5 (5=crítico)"},
		},
		Required: []string{"content"},
	}, a.handleMemory("remember"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "review_memory",
		Description: "Registrar resultado de reforço",
		Parameters: map[string]interface{}{
			"remembered": map[string]interface{}{"type": "boolean", "description": "Lembrou ou não"},
			"quality":    map[string]interface{}{"type": "integer", "description": "0-5"},
		},
	}, a.handleMemory("review"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "list_memories",
		Description: "Listar memórias sendo reforçadas",
		Parameters: map[string]interface{}{
			"category": map[string]interface{}{"type": "string", "description": "Filtrar categoria"},
			"limit":    map[string]interface{}{"type": "integer", "description": "Limite"},
		},
	}, a.handleMemory("list"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "pause_memory",
		Description: "Pausar reforços de uma memória",
		Parameters: map[string]interface{}{
			"content": map[string]interface{}{"type": "string", "description": "Conteúdo a pausar"},
		},
	}, a.handleMemory("pause"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "memory_stats",
		Description: "Estatísticas de memória",
		Parameters:  map[string]interface{}{},
	}, a.handleMemory("stats"))
}

func (a *Agent) handleSchedule(action string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("📅 [PRODUCTIVITY:schedule_%s] %s userID=%d", action, call.Name, call.UserID)

		result := &swarm.ToolResult{
			Success: true,
			Data:    map[string]interface{}{"action": call.Name, "args": call.Args, "user_id": call.UserID},
		}

		if action == "pending" {
			result.Message = "Agendamento registrado. Deseja confirmar?"
			result.SuggestTone = "pergunta_gentil"
		} else {
			result.Message = fmt.Sprintf("Compromisso agendado: %v", call.Args["description"])
			result.SuggestTone = "confirmação_positiva"
			// Handoff para Google Calendar
			result.Handoff = &swarm.HandoffRequest{
				TargetSwarm: "google",
				ToolCall: swarm.ToolCall{
					Name: "manage_calendar_event",
					Args: map[string]interface{}{
						"action":     "create",
						"summary":    call.Args["description"],
						"start_time": call.Args["timestamp"],
					},
					UserID:    call.UserID,
					SessionID: call.SessionID,
				},
				Reason: "Sincronizar agendamento com Google Calendar",
			}
		}

		return result, nil
	}
}

func (a *Agent) handleConfirmSchedule(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	confirmed, _ := call.Args["confirmed"].(bool)
	log.Printf("📅 [PRODUCTIVITY:confirm] confirmed=%v userID=%d", confirmed, call.UserID)

	if confirmed {
		return &swarm.ToolResult{
			Success:     true,
			Message:     "Agendamento confirmado!",
			SuggestTone: "positivo",
			Data:        map[string]interface{}{"action": "confirm_schedule", "confirmed": true},
		}, nil
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     "Agendamento cancelado.",
		SuggestTone: "compreensivo",
		Data:        map[string]interface{}{"action": "confirm_schedule", "confirmed": false},
	}, nil
}

func (a *Agent) handleAlarm(action string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("⏰ [PRODUCTIVITY:alarm_%s] %s userID=%d", action, call.Name, call.UserID)
		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Alarme %s executado", action),
			Data:    map[string]interface{}{"action": call.Name, "args": call.Args},
		}, nil
	}
}

func (a *Agent) handleGTD(action string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("📋 [PRODUCTIVITY:gtd_%s] %s userID=%d", action, call.Name, call.UserID)
		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("GTD %s executado", action),
			Data:    map[string]interface{}{"action": call.Name, "args": call.Args},
		}, nil
	}
}

func (a *Agent) handleMemory(action string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("🧠 [PRODUCTIVITY:memory_%s] %s userID=%d", action, call.Name, call.UserID)
		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Memória %s executada", action),
			Data:    map[string]interface{}{"action": call.Name, "args": call.Args},
		}, nil
	}
}
