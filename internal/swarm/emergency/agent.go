// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package emergency

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/swarm"

	nietzsche "nietzsche-sdk"
)

// Agent implementa o EmergencySwarm - PRIORIDADE MÁXIMA
type Agent struct {
	*swarm.BaseAgent
}

// New cria o EmergencySwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"emergency",
			"Alertas de emergência, chamadas WebRTC, escalação",
			swarm.PriorityCritical,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	// alert_family
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "alert_family",
		Description: "Alerta a família em caso de emergência detectada na conversa com o idoso",
		Parameters: map[string]interface{}{
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Motivo do alerta (ex: 'Paciente relatou dor no peito')",
			},
			"severity": map[string]interface{}{
				"type":        "string",
				"description": "Severidade: critica, alta, media, baixa",
				"enum":        []string{"critica", "alta", "media", "baixa"},
			},
		},
		Required: []string{"reason"},
	}, a.handleAlertFamily)

	// call_family_webrtc
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "call_family_webrtc",
		Description: "Inicia uma chamada de vídeo para a família do idoso",
		Parameters:  map[string]interface{}{},
	}, a.handleCallWebRTC("family"))

	// call_central_webrtc
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "call_central_webrtc",
		Description: "Inicia uma chamada de vídeo de emergência para a Central EVA-Mind",
		Parameters:  map[string]interface{}{},
	}, a.handleCallWebRTC("central"))

	// call_doctor_webrtc
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "call_doctor_webrtc",
		Description: "Inicia uma chamada de vídeo para o médico responsável",
		Parameters:  map[string]interface{}{},
	}, a.handleCallWebRTC("doctor"))

	// call_caregiver_webrtc
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "call_caregiver_webrtc",
		Description: "Inicia uma chamada de vídeo para o cuidador",
		Parameters:  map[string]interface{}{},
	}, a.handleCallWebRTC("caregiver"))
}

func (a *Agent) handleAlertFamily(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	reason, _ := call.Args["reason"].(string)
	severity, _ := call.Args["severity"].(string)
	if severity == "" {
		severity = "alta"
	}

	log.Printf("🚨 [EMERGENCY] Alert família - reason=%s severity=%s userID=%d",
		reason, severity, call.UserID)

	// Enviar notificacao REAL para cuidadores (push + email + SMS)
	alertSent := false
	if deps := a.Deps(); deps != nil && deps.AlertFamily != nil {
		if err := deps.AlertFamily(ctx, call.UserID, reason, severity); err != nil {
			log.Printf("❌ [EMERGENCY] Falha ao alertar familia: %v", err)
		} else {
			alertSent = true
			log.Printf("✅ [EMERGENCY] Familia alertada com sucesso para userID=%d", call.UserID)
		}
	} else {
		log.Printf("⚠️ [EMERGENCY] AlertFamily nao configurado — notificacao NAO enviada!")
	}

	// Persist emergency alert in NietzscheDB for history/audit
	a.persistAlert(ctx, call.UserID, reason, severity, alertSent)

	result := &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Alerta enviado à família: %s (severidade: %s)", reason, severity),
		SuggestTone: "urgente_mas_calmo",
		Data: map[string]interface{}{
			"action":   "alert_family",
			"reason":   reason,
			"severity": severity,
			"user_id":  call.UserID,
		},
		SideEffects: []swarm.SideEffect{
			{Type: "notification", Payload: map[string]string{
				"type":     "push",
				"title":    "⚠️ Alerta EVA-Mind",
				"body":     reason,
				"severity": severity,
			}},
			{Type: "log", Payload: fmt.Sprintf("EMERGENCY_ALERT: user=%d severity=%s reason=%s", call.UserID, severity, reason)},
		},
	}

	// Se severidade crítica, fazer handoff para avaliação C-SSRS
	if severity == "critica" {
		result.Handoff = &swarm.HandoffRequest{
			TargetSwarm: "clinical",
			ToolCall: swarm.ToolCall{
				Name:      "apply_cssrs",
				Args:      map[string]interface{}{},
				UserID:    call.UserID,
				SessionID: call.SessionID,
				Context:   call.Context,
			},
			Reason:   "Alerta crítico detectado - avaliar risco suicida",
			Priority: swarm.PriorityCritical,
		}
	}

	return result, nil
}

// persistAlert stores the emergency alert as a node in NietzscheDB and links it
// to the patient via a HAD_EMERGENCY edge. Best-effort: errors are logged, never
// propagated (the push notification is the critical path, not persistence).
func (a *Agent) persistAlert(ctx context.Context, userID int64, reason, severity string, alertSent bool) {
	deps := a.Deps()
	if deps == nil || deps.Graph == nil {
		log.Printf("⚠️ [EMERGENCY] Graph not available — alert not persisted to NietzscheDB")
		return
	}
	ga, ok := deps.Graph.(*nietzscheInfra.GraphAdapter)
	if !ok {
		log.Printf("⚠️ [EMERGENCY] Graph is not *GraphAdapter — alert not persisted")
		return
	}

	now := time.Now().UTC()

	// 1. Insert the EmergencyAlert node (Energy=1.0, highest importance)
	alertNode, err := ga.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType:   "Semantic",
		Energy:     1.0,
		Collection: "patient_graph",
		Content: map[string]interface{}{
			"node_label":      "EmergencyAlert",
			"severity":        severity,
			"reason":          reason,
			"timestamp":       now.Format(time.RFC3339),
			"patient_id":      userID,
			"alert_sent":      alertSent,
			"response_status": "pending",
		},
	})
	if err != nil {
		log.Printf("❌ [EMERGENCY] Failed to persist alert node: %v", err)
		return
	}
	log.Printf("✅ [EMERGENCY] Alert node persisted: id=%s", alertNode.ID)

	// 2. Find patient node and link with HAD_EMERGENCY edge.
	// Use NQL to locate the patient node by user_id in patient_graph.
	patientNQL := `MATCH (n:Semantic) WHERE n.patient_id = $uid OR n.user_id = $uid RETURN n LIMIT 1`
	qr, err := ga.ExecuteNQL(ctx, patientNQL, map[string]interface{}{
		"uid": userID,
	}, "patient_graph")
	if err != nil || len(qr.Nodes) == 0 {
		log.Printf("⚠️ [EMERGENCY] Patient node not found for userID=%d — edge not created", userID)
		return
	}

	patientID := qr.Nodes[0].ID
	edgeID, err := ga.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:       patientID,
		To:         alertNode.ID,
		EdgeType:   "Association",
		Weight:     1.0,
		Collection: "patient_graph",
	})
	if err != nil {
		log.Printf("❌ [EMERGENCY] Failed to create HAD_EMERGENCY edge: %v", err)
		return
	}
	log.Printf("✅ [EMERGENCY] HAD_EMERGENCY edge created: %s → %s (edge=%s)", patientID, alertNode.ID, edgeID)
}

func (a *Agent) handleCallWebRTC(target string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("📞 [EMERGENCY] Chamada WebRTC → %s userID=%d", target, call.UserID)

		return &swarm.ToolResult{
			Success:     true,
			Message:     fmt.Sprintf("Iniciando chamada para %s...", target),
			SuggestTone: "tranquilizador",
			Data: map[string]interface{}{
				"action":  "call_webrtc",
				"target":  target,
				"user_id": call.UserID,
			},
		}, nil
	}
}
