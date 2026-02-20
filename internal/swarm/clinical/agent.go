// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package clinical

import (
	"context"
	"fmt"
	"log"

	"eva/internal/swarm"
)

// Agent implementa o ClinicalSwarm - avaliações PHQ-9, GAD-7, C-SSRS, medicamentos
type Agent struct {
	*swarm.BaseAgent
}

// New cria o ClinicalSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"clinical",
			"Avaliações clínicas PHQ-9, GAD-7, C-SSRS, medicamentos",
			swarm.PriorityHigh,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	// Avaliações clínicas
	for _, assessment := range []struct {
		name string
		desc string
	}{
		{"apply_phq9", "Inicia avaliação de depressão PHQ-9"},
		{"apply_gad7", "Inicia avaliação de ansiedade GAD-7"},
		{"apply_cssrs", "Inicia avaliação de risco suicida C-SSRS"},
	} {
		name := assessment.name
		a.RegisterTool(swarm.ToolDefinition{
			Name:        name,
			Description: assessment.desc,
			Parameters:  map[string]interface{}{},
		}, a.handleStartAssessment(name))
	}

	// Submit responses
	for _, submit := range []struct {
		name string
		desc string
	}{
		{"submit_phq9_response", "Registra resposta PHQ-9"},
		{"submit_gad7_response", "Registra resposta GAD-7"},
		{"submit_cssrs_response", "Registra resposta C-SSRS"},
	} {
		name := submit.name
		a.RegisterTool(swarm.ToolDefinition{
			Name:        name,
			Description: submit.desc,
			Parameters: map[string]interface{}{
				"question_number": map[string]interface{}{
					"type":        "integer",
					"description": "Número da questão",
				},
				"response": map[string]interface{}{
					"type":        "string",
					"description": "Resposta do paciente",
				},
			},
			Required: []string{"question_number", "response"},
		}, a.handleSubmitResponse(name))
	}

	// confirm_medication
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "confirm_medication",
		Description: "Confirma que o idoso tomou o remédio",
		Parameters: map[string]interface{}{
			"medication_name": map[string]interface{}{
				"type":        "string",
				"description": "Nome do medicamento tomado",
			},
		},
		Required: []string{"medication_name"},
	}, a.handleConfirmMedication)

	// open_camera_analysis
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "open_camera_analysis",
		Description: "Ativa a câmera para analisar visualmente um objeto, remédio ou ambiente",
		Parameters:  map[string]interface{}{},
	}, a.handleCameraAnalysis)

	// scan_medication_visual
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "scan_medication_visual",
		Description: "Identificar medicamento pela câmera",
		Parameters: map[string]interface{}{
			"period": map[string]interface{}{
				"type":        "string",
				"description": "Período do dia (manhã, tarde, noite)",
			},
		},
	}, a.handleCameraAnalysis)
}

func (a *Agent) handleStartAssessment(assessmentType string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("🧠 [CLINICAL] Iniciando %s para userID=%d", assessmentType, call.UserID)

		// TODO: Integrar com internal/tools/handlers_scales_response.go
		return &swarm.ToolResult{
			Success:     true,
			Message:     fmt.Sprintf("Iniciando avaliação %s...", assessmentType),
			SuggestTone: "gentil_empático",
			Data: map[string]interface{}{
				"action":          "start_assessment",
				"assessment_type": assessmentType,
				"user_id":         call.UserID,
			},
		}, nil
	}
}

func (a *Agent) handleSubmitResponse(responseType string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		questionNum, _ := call.Args["question_number"].(float64)
		response, _ := call.Args["response"].(string)

		log.Printf("📋 [CLINICAL] %s Q%d=%s userID=%d", responseType, int(questionNum), response, call.UserID)

		// TODO: Integrar com handlers de escala + scoring
		result := &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Resposta registrada (questão %d)", int(questionNum)),
			Data: map[string]interface{}{
				"action":          "submit_response",
				"response_type":   responseType,
				"question_number": int(questionNum),
				"response":        response,
				"user_id":         call.UserID,
			},
		}

		return result, nil
	}
}

func (a *Agent) handleConfirmMedication(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	medName, _ := call.Args["medication_name"].(string)
	log.Printf("💊 [CLINICAL] Medicamento confirmado: %s userID=%d", medName, call.UserID)

	return &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Medicamento '%s' confirmado como tomado!", medName),
		SuggestTone: "positivo_encorajador",
		Data: map[string]interface{}{
			"action":          "confirm_medication",
			"medication_name": medName,
			"user_id":         call.UserID,
		},
		SideEffects: []swarm.SideEffect{
			{Type: "log", Payload: fmt.Sprintf("MED_CONFIRMED: user=%d med=%s", call.UserID, medName)},
		},
	}, nil
}

func (a *Agent) handleCameraAnalysis(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	log.Printf("📷 [CLINICAL] Câmera ativada para análise userID=%d", call.UserID)

	return &swarm.ToolResult{
		Success:     true,
		Message:     "Câmera ativada para análise visual",
		SuggestTone: "instrucional",
		Data: map[string]interface{}{
			"action":  "camera_analysis",
			"user_id": call.UserID,
		},
	}, nil
}
