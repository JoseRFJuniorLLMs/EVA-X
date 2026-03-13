// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package productivity

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/swarm"

	nietzsche "nietzsche-sdk"
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

			// Persist appointment to NietzscheDB (best-effort)
			a.persistAppointment(ctx, call)

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

		if action == "capture" {
			a.persistTask(ctx, call)
		}

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

		switch action {
		case "remember":
			a.persistSpacedMemory(ctx, call)
		case "review":
			a.persistMemoryReview(ctx, call)
		}

		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Memória %s executada", action),
			Data:    map[string]interface{}{"action": call.Name, "args": call.Args},
		}, nil
	}
}

// ── NietzscheDB Persistence ──────────────────────────────────────────────────

// graphAdapter extracts the *GraphAdapter from Dependencies (best-effort).
func (a *Agent) graphAdapter() *nietzscheInfra.GraphAdapter {
	deps := a.Deps()
	if deps == nil || deps.Graph == nil {
		return nil
	}
	ga, ok := deps.Graph.(*nietzscheInfra.GraphAdapter)
	if !ok {
		return nil
	}
	return ga
}

// findPatientNode locates the patient node in patient_graph by user ID.
func (a *Agent) findPatientNode(ctx context.Context, ga *nietzscheInfra.GraphAdapter, userID int64) string {
	nql := `MATCH (n:Semantic) WHERE n.patient_id = $uid OR n.user_id = $uid RETURN n LIMIT 1`
	qr, err := ga.ExecuteNQL(ctx, nql, map[string]interface{}{"uid": userID}, "patient_graph")
	if err != nil || len(qr.Nodes) == 0 {
		return ""
	}
	return qr.Nodes[0].ID
}

// ── FIX 4.1: Appointments ────────────────────────────────────────────────────

// persistAppointment stores an Appointment node in patient_graph and links it
// to the patient via HAS_APPOINTMENT. Best-effort: errors are logged only.
func (a *Agent) persistAppointment(ctx context.Context, call swarm.ToolCall) {
	ga := a.graphAdapter()
	if ga == nil {
		log.Printf("⚠️ [PRODUCTIVITY] Graph not available — appointment not persisted")
		return
	}

	now := time.Now().UTC()
	apptType, _ := call.Args["type"].(string)
	description, _ := call.Args["description"].(string)
	scheduledTime, _ := call.Args["timestamp"].(string)

	// MergeNode: find or create the Appointment (match on description + scheduled_time)
	mergeResult, err := ga.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "patient_graph",
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label":     "Appointment",
			"description":    description,
			"scheduled_time": scheduledTime,
		},
		OnCreateSet: map[string]interface{}{
			"node_label":     "Appointment",
			"type":           apptType,
			"description":    description,
			"scheduled_time": scheduledTime,
			"status":         "scheduled",
			"patient_id":     call.UserID,
			"timestamp":      now.Format(time.RFC3339),
		},
		OnMatchSet: map[string]interface{}{
			"status":    "scheduled",
			"timestamp": now.Format(time.RFC3339),
		},
		Energy: 0.8,
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to persist appointment: %v", err)
		return
	}
	log.Printf("✅ [PRODUCTIVITY] Appointment node %s (created=%v)", mergeResult.NodeID, mergeResult.Created)

	// Link patient → appointment via HAS_APPOINTMENT edge
	patientID := a.findPatientNode(ctx, ga, call.UserID)
	if patientID == "" {
		log.Printf("⚠️ [PRODUCTIVITY] Patient node not found for userID=%d — edge not created", call.UserID)
		return
	}

	_, err = ga.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		Collection: "patient_graph",
		FromNodeID: patientID,
		ToNodeID:   mergeResult.NodeID,
		EdgeType:   "Association",
		OnCreateSet: map[string]interface{}{
			"edge_label": "HAS_APPOINTMENT",
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to create HAS_APPOINTMENT edge: %v", err)
		return
	}
	log.Printf("✅ [PRODUCTIVITY] HAS_APPOINTMENT edge: %s → %s", patientID, mergeResult.NodeID)
}

// ── FIX 4.1: Tasks ──────────────────────────────────────────────────────────

// persistTask stores a Task node in patient_graph and links it to the patient
// via HAS_TASK. Best-effort: errors are logged only.
func (a *Agent) persistTask(ctx context.Context, call swarm.ToolCall) {
	ga := a.graphAdapter()
	if ga == nil {
		log.Printf("⚠️ [PRODUCTIVITY] Graph not available — task not persisted")
		return
	}

	now := time.Now().UTC()
	rawInput, _ := call.Args["raw_input"].(string)
	taskContext, _ := call.Args["context"].(string)
	nextAction, _ := call.Args["next_action"].(string)
	dueDate, _ := call.Args["due_date"].(string)
	project, _ := call.Args["project"].(string)

	// MergeNode: find or create the Task (match on raw_input)
	mergeResult, err := ga.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "patient_graph",
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label": "Task",
			"raw_input":  rawInput,
		},
		OnCreateSet: map[string]interface{}{
			"node_label":  "Task",
			"raw_input":   rawInput,
			"context":     taskContext,
			"next_action": nextAction,
			"due_date":    dueDate,
			"project":     project,
			"status":      "pending",
			"patient_id":  call.UserID,
			"timestamp":   now.Format(time.RFC3339),
		},
		OnMatchSet: map[string]interface{}{
			"next_action": nextAction,
			"due_date":    dueDate,
			"status":      "pending",
			"timestamp":   now.Format(time.RFC3339),
		},
		Energy: 0.7,
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to persist task: %v", err)
		return
	}
	log.Printf("✅ [PRODUCTIVITY] Task node %s (created=%v)", mergeResult.NodeID, mergeResult.Created)

	// Link patient → task via HAS_TASK edge
	patientID := a.findPatientNode(ctx, ga, call.UserID)
	if patientID == "" {
		log.Printf("⚠️ [PRODUCTIVITY] Patient node not found for userID=%d — edge not created", call.UserID)
		return
	}

	_, err = ga.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		Collection: "patient_graph",
		FromNodeID: patientID,
		ToNodeID:   mergeResult.NodeID,
		EdgeType:   "Association",
		OnCreateSet: map[string]interface{}{
			"edge_label": "HAS_TASK",
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to create HAS_TASK edge: %v", err)
		return
	}
	log.Printf("✅ [PRODUCTIVITY] HAS_TASK edge: %s → %s", patientID, mergeResult.NodeID)
}

// ── FIX 4.2: Spaced Repetition ──────────────────────────────────────────────

// persistSpacedMemory stores a SpacedMemory node and links it to the patient
// via REMEMBERS. Best-effort: errors are logged only.
func (a *Agent) persistSpacedMemory(ctx context.Context, call swarm.ToolCall) {
	ga := a.graphAdapter()
	if ga == nil {
		log.Printf("⚠️ [PRODUCTIVITY] Graph not available — spaced memory not persisted")
		return
	}

	now := time.Now().UTC()
	content, _ := call.Args["content"].(string)
	category, _ := call.Args["category"].(string)
	trigger, _ := call.Args["trigger"].(string)
	importance, _ := call.Args["importance"].(float64) // JSON numbers decode as float64
	if importance == 0 {
		importance = 3 // default mid-importance
	}
	if category == "" {
		category = "general"
	}

	// First review in 1 day (SM-2 initial interval)
	nextReview := now.Add(24 * time.Hour)

	nodeResult, err := ga.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType:   "Semantic",
		Energy:     float32(0.5 + (importance * 0.1)), // 0.6–1.0 based on importance
		Collection: "patient_graph",
		Content: map[string]interface{}{
			"node_label":    "SpacedMemory",
			"content":       content,
			"category":      category,
			"importance":    importance,
			"trigger":       trigger,
			"next_review":   nextReview.Format(time.RFC3339),
			"interval_days": 1,
			"repetitions":   0,
			"ease_factor":   2.5,
			"status":        "active",
			"patient_id":    call.UserID,
			"timestamp":     now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to persist spaced memory: %v", err)
		return
	}
	log.Printf("✅ [PRODUCTIVITY] SpacedMemory node %s created", nodeResult.ID)

	// Link patient → memory via REMEMBERS edge
	patientID := a.findPatientNode(ctx, ga, call.UserID)
	if patientID == "" {
		log.Printf("⚠️ [PRODUCTIVITY] Patient node not found for userID=%d — edge not created", call.UserID)
		return
	}

	edgeID, err := ga.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:       patientID,
		To:         nodeResult.ID,
		EdgeType:   "Association",
		Weight:     importance / 5.0, // normalize to 0–1
		Collection: "patient_graph",
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to create REMEMBERS edge: %v", err)
		return
	}
	log.Printf("✅ [PRODUCTIVITY] REMEMBERS edge: %s → %s (edge=%s)", patientID, nodeResult.ID, edgeID)
}

// persistMemoryReview updates an existing SpacedMemory node with SM-2 algorithm
// results and creates a ReviewEvent node linked via REVIEWED_ON.
func (a *Agent) persistMemoryReview(ctx context.Context, call swarm.ToolCall) {
	ga := a.graphAdapter()
	if ga == nil {
		log.Printf("⚠️ [PRODUCTIVITY] Graph not available — memory review not persisted")
		return
	}

	now := time.Now().UTC()
	remembered, _ := call.Args["remembered"].(bool)
	quality, _ := call.Args["quality"].(float64) // 0–5 scale
	if quality == 0 && remembered {
		quality = 4 // good recall default
	}

	// Find the most recent active SpacedMemory for this user that needs review
	nql := `MATCH (n:Semantic) WHERE n.node_label = "SpacedMemory" AND n.patient_id = $uid AND n.status = "active" RETURN n LIMIT 1`
	qr, err := ga.ExecuteNQL(ctx, nql, map[string]interface{}{"uid": call.UserID}, "patient_graph")
	if err != nil || len(qr.Nodes) == 0 {
		log.Printf("⚠️ [PRODUCTIVITY] No active SpacedMemory found for userID=%d", call.UserID)
		return
	}

	memoryNode := qr.Nodes[0]

	// Extract current SM-2 values from node content
	repetitions := 0
	intervalDays := 1.0
	easeFactor := 2.5
	if v, ok := memoryNode.Content["repetitions"].(float64); ok {
		repetitions = int(v)
	}
	if v, ok := memoryNode.Content["interval_days"].(float64); ok {
		intervalDays = v
	}
	if v, ok := memoryNode.Content["ease_factor"].(float64); ok {
		easeFactor = v
	}

	// SM-2 algorithm update
	if quality >= 3 {
		repetitions++
		switch repetitions {
		case 1:
			intervalDays = 1
		case 2:
			intervalDays = 6
		default:
			intervalDays = intervalDays * easeFactor
		}
		easeFactor = easeFactor + (0.1 - (5-quality)*(0.08+(5-quality)*0.02))
		if easeFactor < 1.3 {
			easeFactor = 1.3
		}
	} else {
		// Failed recall: reset to beginning
		repetitions = 0
		intervalDays = 1
	}

	nextReview := now.Add(time.Duration(intervalDays*24) * time.Hour)

	// Update the SpacedMemory node via MergeNode (match on existing content)
	memContent, _ := memoryNode.Content["content"].(string)
	_, err = ga.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "patient_graph",
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label": "SpacedMemory",
			"content":    memContent,
			"patient_id": call.UserID,
		},
		OnMatchSet: map[string]interface{}{
			"next_review":   nextReview.Format(time.RFC3339),
			"interval_days": intervalDays,
			"repetitions":   repetitions,
			"ease_factor":   easeFactor,
			"last_reviewed": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to update SpacedMemory: %v", err)
		return
	}
	log.Printf("✅ [PRODUCTIVITY] SpacedMemory updated: next_review=%s interval=%.1f days reps=%d",
		nextReview.Format(time.RFC3339), intervalDays, repetitions)

	// Create ReviewEvent node
	reviewNode, err := ga.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType:   "Episodic",
		Energy:     0.5,
		Collection: "patient_graph",
		Content: map[string]interface{}{
			"node_label":  "ReviewEvent",
			"remembered":  remembered,
			"quality":     quality,
			"patient_id":  call.UserID,
			"timestamp":   now.Format(time.RFC3339),
			"memory_node": memoryNode.ID,
		},
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to create ReviewEvent node: %v", err)
		return
	}

	// Link SpacedMemory → ReviewEvent via REVIEWED_ON edge
	edgeID, err := ga.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:       memoryNode.ID,
		To:         reviewNode.ID,
		EdgeType:   "Association",
		Weight:     quality / 5.0,
		Collection: "patient_graph",
	})
	if err != nil {
		log.Printf("❌ [PRODUCTIVITY] Failed to create REVIEWED_ON edge: %v", err)
		return
	}
	log.Printf("✅ [PRODUCTIVITY] REVIEWED_ON edge: %s → %s (edge=%s)", memoryNode.ID, reviewNode.ID, edgeID)
}
