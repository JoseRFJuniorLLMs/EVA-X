// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package wellness

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/hippocampus/habits"
	"eva/internal/swarm"
)

// Agent implementa o WellnessSwarm - meditação, respiração, exercícios, hábitos
type Agent struct {
	*swarm.BaseAgent
	habitTracker *habits.HabitTracker
	graph        *nietzscheInfra.GraphAdapter
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

// SetHabitTracker injects the HabitTracker service (called from main.go).
func (a *Agent) SetHabitTracker(tracker *habits.HabitTracker) {
	a.habitTracker = tracker
}

// SetGraphAdapter injects the NietzscheDB GraphAdapter for edge creation (called from main.go).
func (a *Agent) SetGraphAdapter(ga *nietzscheInfra.GraphAdapter) {
	a.graph = ga
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

		switch action {
		case "log":
			return a.handleLogHabit(ctx, call)
		case "water":
			return a.handleLogWater(ctx, call)
		case "stats":
			return a.handleHabitStats(ctx, call)
		case "summary":
			return a.handleHabitSummary(ctx, call)
		default:
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
}

// handleLogHabit registers a habit completion and creates graph edges in eva_mind.
func (a *Agent) handleLogHabit(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	habitName, _ := call.Args["habit_name"].(string)
	success, _ := call.Args["success"].(bool)
	notes, _ := call.Args["notes"].(string)

	if habitName == "" {
		return &swarm.ToolResult{
			Success: false,
			Message: "Nome do hábito é obrigatório",
		}, nil
	}

	if a.habitTracker != nil {
		logEntry, err := a.habitTracker.LogHabit(ctx, call.UserID, habitName, success, "swarm", notes, nil)
		if err != nil {
			log.Printf("⚠️ [WELLNESS] LogHabit failed: %v", err)
			return &swarm.ToolResult{
				Success: false,
				Message: fmt.Sprintf("Erro ao registrar hábito: %v", err),
			}, nil
		}

		var message string
		if success {
			message = fmt.Sprintf("Ótimo! Registrei que você completou '%s'. Continue assim!", habitName)
		} else {
			message = "Entendi, registrei. Não se preocupe, amanhã é um novo dia!"
		}

		return &swarm.ToolResult{
			Success:     true,
			Message:     message,
			SuggestTone: "positivo_encorajador",
			Data: map[string]interface{}{
				"action":   "log_habit",
				"category": "habit",
				"log_id":   logEntry.ID,
				"habit":    habitName,
				"success":  success,
				"user_id":  call.UserID,
			},
		}, nil
	}

	// Fallback: persist via GraphAdapter directly if HabitTracker not available
	if a.graph != nil {
		a.persistHabitViaGraph(call.UserID, habitName, success, notes)
	}

	var message string
	if success {
		message = fmt.Sprintf("Registrei '%s' como completo!", habitName)
	} else {
		message = fmt.Sprintf("Registrei '%s'. Amanhã é um novo dia!", habitName)
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     message,
		SuggestTone: "positivo_encorajador",
		Data: map[string]interface{}{
			"action":   "log_habit",
			"category": "habit",
			"habit":    habitName,
			"success":  success,
			"user_id":  call.UserID,
		},
	}, nil
}

// persistHabitViaGraph creates habit nodes and edges directly via GraphAdapter.
// Used as fallback when HabitTracker is not available.
func (a *Agent) persistHabitViaGraph(userID int64, habitName string, success bool, notes string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	collection := "eva_mind"

	// 1. MergeNode: user profile
	userNode, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: collection,
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label": "UserProfile",
			"idoso_id":   userID,
		},
		OnCreateSet: map[string]interface{}{
			"node_label": "UserProfile",
			"idoso_id":   userID,
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("⚠️ [WELLNESS] MergeNode UserProfile failed: %v", err)
		return
	}

	// 2. MergeNode: habit definition
	habitNode, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: collection,
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label": "Habit",
			"habit_name": habitName,
		},
		OnCreateSet: map[string]interface{}{
			"node_label": "Habit",
			"habit_name": habitName,
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("⚠️ [WELLNESS] MergeNode Habit failed: %v", err)
		return
	}

	// 3. MergeNode: habit completion event (unique per habit_name + date)
	completedStr := "false"
	if success {
		completedStr = "true"
	}
	habitLogNode, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: collection,
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label": "HabitLog",
			"habit_name": habitName,
			"date":       dateStr,
			"idoso_id":   userID,
		},
		OnCreateSet: map[string]interface{}{
			"node_label": "HabitLog",
			"habit_name": habitName,
			"date":       dateStr,
			"idoso_id":   userID,
			"completed":  completedStr,
			"timestamp":  now.Format(time.RFC3339),
			"notes":      notes,
		},
		OnMatchSet: map[string]interface{}{
			"completed": completedStr,
			"timestamp": now.Format(time.RFC3339),
			"notes":     notes,
		},
	})
	if err != nil {
		log.Printf("⚠️ [WELLNESS] MergeNode HabitLog failed: %v", err)
		return
	}

	// 4. Edge: user → habit (TRACKS_HABIT)
	if _, err := a.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		Collection: collection,
		FromNodeID: userNode.NodeID,
		ToNodeID:   habitNode.NodeID,
		EdgeType:   "TRACKS_HABIT",
	}); err != nil {
		log.Printf("⚠️ [WELLNESS] MergeEdge TRACKS_HABIT failed: %v", err)
	}

	// 5. Edge: habit → completion event (COMPLETED_ON)
	if _, err := a.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		Collection: collection,
		FromNodeID: habitNode.NodeID,
		ToNodeID:   habitLogNode.NodeID,
		EdgeType:   "COMPLETED_ON",
	}); err != nil {
		log.Printf("⚠️ [WELLNESS] MergeEdge COMPLETED_ON failed: %v", err)
	}

	log.Printf("✅ [WELLNESS] Graph edges: user(%s) -TRACKS_HABIT-> habit(%s) -COMPLETED_ON-> log(%s)",
		userNode.NodeID[:8], habitNode.NodeID[:8], habitLogNode.NodeID[:8])
}

// handleLogWater registers water intake via HabitTracker.
func (a *Agent) handleLogWater(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	glassesFloat, _ := call.Args["glasses"].(float64)
	glasses := int(glassesFloat)
	if glasses == 0 {
		glasses = 1
	}

	if a.habitTracker != nil {
		logEntry, err := a.habitTracker.LogWater(ctx, call.UserID, glasses, "swarm")
		if err != nil {
			return &swarm.ToolResult{
				Success: false,
				Message: fmt.Sprintf("Erro: %v", err),
			}, nil
		}

		copoStr := "copo"
		if glasses > 1 {
			copoStr = "copos"
		}
		return &swarm.ToolResult{
			Success:     true,
			Message:     fmt.Sprintf("Anotei! %d %s de água. Hidratação é muito importante!", glasses, copoStr),
			SuggestTone: "positivo_encorajador",
			Data: map[string]interface{}{
				"action":  "log_water",
				"log_id":  logEntry.ID,
				"glasses": glasses,
				"user_id": call.UserID,
			},
		}, nil
	}

	// Fallback: log via graph
	if a.graph != nil {
		a.persistHabitViaGraph(call.UserID, "tomar_agua", true, fmt.Sprintf("%d copo(s)", glasses))
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Anotei! %d copo(s) de água!", glasses),
		SuggestTone: "positivo_encorajador",
		Data: map[string]interface{}{
			"action":  "log_water",
			"glasses": glasses,
			"user_id": call.UserID,
		},
	}, nil
}

// handleHabitStats returns habit statistics.
func (a *Agent) handleHabitStats(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.habitTracker == nil {
		return &swarm.ToolResult{
			Success: false,
			Message: "Serviço de hábitos não disponível",
		}, nil
	}

	patterns, err := a.habitTracker.GetAllPatterns(ctx, call.UserID)
	if err != nil {
		return &swarm.ToolResult{
			Success: false,
			Message: fmt.Sprintf("Erro ao buscar padrões: %v", err),
		}, nil
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Encontrei %d hábito(s) rastreados.", len(patterns)),
		SuggestTone: "informativo",
		Data: map[string]interface{}{
			"action":   "habit_stats",
			"patterns": patterns,
			"user_id":  call.UserID,
		},
	}, nil
}

// handleHabitSummary returns daily habit summary.
func (a *Agent) handleHabitSummary(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.habitTracker == nil {
		return &swarm.ToolResult{
			Success: false,
			Message: "Serviço de hábitos não disponível",
		}, nil
	}

	summary, err := a.habitTracker.GetDailySummary(ctx, call.UserID)
	if err != nil {
		return &swarm.ToolResult{
			Success: false,
			Message: fmt.Sprintf("Erro ao buscar resumo: %v", err),
		}, nil
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     "Aqui está o resumo do seu dia!",
		SuggestTone: "informativo",
		Data: map[string]interface{}{
			"action":  "habit_summary",
			"summary": summary,
			"user_id": call.UserID,
		},
	}, nil
}
