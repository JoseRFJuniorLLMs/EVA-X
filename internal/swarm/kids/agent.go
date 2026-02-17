// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package kids

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
)

// Agent implementa o KidsSwarm - modo criança gamificado
type Agent struct {
	*swarm.BaseAgent
}

// New cria o KidsSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"kids",
			"Modo criança gamificado - missões, recompensas, aprendizado",
			swarm.PriorityLow,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "kids_mission_create",
		Description: "Criar missão para a criança",
		Parameters: map[string]interface{}{
			"title":      map[string]interface{}{"type": "string", "description": "Nome da missão"},
			"category":   map[string]interface{}{"type": "string", "description": "hygiene, study, chores, health, social, food, sleep"},
			"difficulty": map[string]interface{}{"type": "string", "description": "easy, medium, hard, epic"},
			"due_time":   map[string]interface{}{"type": "string", "description": "Horário limite HH:MM"},
		},
		Required: []string{"title", "category"},
	}, a.handleKids("mission_create"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "kids_mission_complete",
		Description: "Marcar missão como concluída",
		Parameters: map[string]interface{}{
			"title": map[string]interface{}{"type": "string", "description": "Título da missão"},
		},
		Required: []string{"title"},
	}, a.handleKids("mission_complete"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "kids_missions_pending",
		Description: "Ver missões pendentes do dia",
		Parameters:  map[string]interface{}{},
	}, a.handleKids("missions_pending"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "kids_stats",
		Description: "Ver pontos, nível, conquistas",
		Parameters:  map[string]interface{}{},
	}, a.handleKids("stats"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "kids_learn",
		Description: "Ensinar algo novo",
		Parameters: map[string]interface{}{
			"topic":    map[string]interface{}{"type": "string", "description": "Assunto"},
			"category": map[string]interface{}{"type": "string", "description": "animals, science, history, language, math, nature"},
		},
		Required: []string{"topic"},
	}, a.handleKids("learn"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "kids_quiz",
		Description: "Quiz de revisão",
		Parameters:  map[string]interface{}{},
	}, a.handleKids("quiz"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "kids_story",
		Description: "Iniciar história interativa",
		Parameters: map[string]interface{}{
			"theme": map[string]interface{}{
				"type": "string", "description": "Tema",
				"enum": []string{"adventure", "fantasy", "space", "animals", "pirates"},
			},
		},
	}, a.handleKids("story"))
}

func (a *Agent) handleKids(action string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("🎮 [KIDS:%s] %s userID=%d", action, call.Name, call.UserID)

		return &swarm.ToolResult{
			Success:     true,
			Message:     fmt.Sprintf("Kids mode: %s executado!", action),
			SuggestTone: "divertido_energético",
			Data: map[string]interface{}{
				"action":   call.Name,
				"category": "kids",
				"args":     call.Args,
				"user_id":  call.UserID,
			},
		}, nil
	}
}
