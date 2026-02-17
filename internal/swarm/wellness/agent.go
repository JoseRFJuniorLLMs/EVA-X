// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package wellness

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
)

// Agent implementa o WellnessSwarm - meditação, respiração, exercícios, hábitos
type Agent struct {
	*swarm.BaseAgent
}

// New cria o WellnessSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"wellness",
			"Meditação, respiração, exercícios, rastreamento de hábitos",
			swarm.PriorityMedium,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "guided_meditation",
		Description: "Meditação guiada",
		Parameters: map[string]interface{}{
			"duration": map[string]interface{}{"type": "integer", "description": "Duração em minutos"},
			"theme":    map[string]interface{}{"type": "string", "description": "Tema da meditação"},
		},
	}, a.handleWellness("meditation"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "breathing_exercises",
		Description: "Exercícios de respiração",
		Parameters: map[string]interface{}{
			"technique": map[string]interface{}{"type": "string", "description": "Técnica respiratória"},
		},
	}, a.handleWellness("breathing"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "wim_hof_breathing",
		Description: "Respiração Wim Hof com áudio guiado",
		Parameters: map[string]interface{}{
			"rounds":     map[string]interface{}{"type": "integer", "description": "Rodadas (1-4, padrão 3)"},
			"with_audio": map[string]interface{}{"type": "boolean", "description": "Tocar áudio guiado"},
		},
	}, a.handleWellness("wim_hof"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "pomodoro_timer",
		Description: "Timer Pomodoro para foco",
		Parameters: map[string]interface{}{
			"work_minutes":   map[string]interface{}{"type": "integer", "description": "Tempo de foco (padrão 25)"},
			"break_minutes":  map[string]interface{}{"type": "integer", "description": "Tempo de pausa (padrão 5)"},
			"sessions":       map[string]interface{}{"type": "integer", "description": "Sessões (padrão 4)"},
			"break_activity": map[string]interface{}{"type": "string", "description": "Atividade na pausa"},
		},
	}, a.handleWellness("pomodoro"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "chair_exercises",
		Description: "Exercícios físicos na cadeira",
		Parameters: map[string]interface{}{
			"duration": map[string]interface{}{"type": "integer", "description": "Duração em minutos"},
		},
	}, a.handleWellness("exercise"))

	// Habit tracking
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "log_habit",
		Description: "Registrar sucesso/falha de um hábito",
		Parameters: map[string]interface{}{
			"habit_name": map[string]interface{}{"type": "string", "description": "Nome do hábito"},
			"success":    map[string]interface{}{"type": "boolean", "description": "Completou ou não"},
			"notes":      map[string]interface{}{"type": "string", "description": "Observação"},
		},
		Required: []string{"habit_name", "success"},
	}, a.handleHabit("log"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "log_water",
		Description: "Registrar consumo de água",
		Parameters: map[string]interface{}{
			"glasses": map[string]interface{}{"type": "integer", "description": "Copos de água"},
		},
	}, a.handleHabit("water"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "habit_stats",
		Description: "Ver estatísticas de hábitos",
		Parameters:  map[string]interface{}{},
	}, a.handleHabit("stats"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "habit_summary",
		Description: "Resumo do dia de hábitos",
		Parameters:  map[string]interface{}{},
	}, a.handleHabit("summary"))
}

func (a *Agent) handleWellness(category string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("🧘 [WELLNESS:%s] %s userID=%d", category, call.Name, call.UserID)

		return &swarm.ToolResult{
			Success:     true,
			Message:     fmt.Sprintf("Iniciando %s...", call.Name),
			SuggestTone: "calmo_guiado",
			Data: map[string]interface{}{
				"action":   call.Name,
				"category": category,
				"args":     call.Args,
				"user_id":  call.UserID,
			},
		}, nil
	}
}

func (a *Agent) handleHabit(action string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("📊 [WELLNESS:habit_%s] %s userID=%d", action, call.Name, call.UserID)

		return &swarm.ToolResult{
			Success:     true,
			Message:     fmt.Sprintf("Hábito registrado: %s", call.Name),
			SuggestTone: "positivo_encorajador",
			Data: map[string]interface{}{
				"action":   call.Name,
				"category": "habit",
				"args":     call.Args,
				"user_id":  call.UserID,
			},
		}, nil
	}
}
