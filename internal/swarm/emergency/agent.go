// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package emergency

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
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

	// TODO: Integrar com motor/actions/actions.go AlertFamilyWithSeverity
	// Por enquanto, retorna instrução para o motor layer
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
