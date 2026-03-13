// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package clinical

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/swarm"

	nietzsche "nietzsche-sdk"
)

// Agent implementa o ClinicalSwarm - avaliações PHQ-9, GAD-7, C-SSRS, medicamentos
type Agent struct {
	*swarm.BaseAgent
	graph *nietzscheInfra.GraphAdapter // NietzscheDB graph for patient_graph persistence
}

// Init initializes the agent and extracts the GraphAdapter from dependencies.
func (a *Agent) Init(deps *swarm.Dependencies) error {
	if err := a.BaseAgent.Init(deps); err != nil {
		return err
	}
	if deps.Graph != nil {
		if ga, ok := deps.Graph.(*nietzscheInfra.GraphAdapter); ok {
			a.graph = ga
			log.Printf("[CLINICAL] GraphAdapter initialized for NietzscheDB persistence")
		} else {
			log.Printf("[CLINICAL] WARNING: deps.Graph is not *GraphAdapter, clinical data will NOT be persisted")
		}
	} else {
		log.Printf("[CLINICAL] WARNING: deps.Graph is nil, clinical data will NOT be persisted")
	}
	return nil
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

		// Persist assessment in NietzscheDB (patient_graph)
		var persistedNodeID string
		if a.graph != nil {
			nodeID, err := a.persistAssessment(ctx, call.UserID, responseType, int(questionNum), response)
			if err != nil {
				log.Printf("[CLINICAL] WARNING: failed to persist %s in NietzscheDB: %v", responseType, err)
			} else {
				persistedNodeID = nodeID
				log.Printf("[CLINICAL] Persisted %s node=%s for userID=%d", responseType, nodeID, call.UserID)
			}
		}

		result := &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Resposta registrada (questão %d)", int(questionNum)),
			Data: map[string]interface{}{
				"action":          "submit_response",
				"response_type":   responseType,
				"question_number": int(questionNum),
				"response":        response,
				"user_id":         call.UserID,
				"persisted_node":  persistedNodeID,
			},
		}

		return result, nil
	}
}

func (a *Agent) handleConfirmMedication(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	medName, _ := call.Args["medication_name"].(string)
	log.Printf("💊 [CLINICAL] Medicamento confirmado: %s userID=%d", medName, call.UserID)

	// Persist medication confirmation in NietzscheDB (patient_graph)
	var persistedNodeID string
	if a.graph != nil {
		nodeID, err := a.persistMedication(ctx, call.UserID, medName)
		if err != nil {
			log.Printf("[CLINICAL] WARNING: failed to persist medication in NietzscheDB: %v", err)
		} else {
			persistedNodeID = nodeID
			log.Printf("[CLINICAL] Persisted MedicationLog node=%s for userID=%d med=%s", nodeID, call.UserID, medName)
		}
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Medicamento '%s' confirmado como tomado!", medName),
		SuggestTone: "positivo_encorajador",
		Data: map[string]interface{}{
			"action":          "confirm_medication",
			"medication_name": medName,
			"user_id":         call.UserID,
			"persisted_node":  persistedNodeID,
		},
		SideEffects: []swarm.SideEffect{
			{Type: "log", Payload: fmt.Sprintf("MED_CONFIRMED: user=%d med=%s", call.UserID, medName)},
		},
	}, nil
}

func (a *Agent) handleCameraAnalysis(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	log.Printf("[CLINICAL] Camera activated for analysis userID=%d", call.UserID)

	return &swarm.ToolResult{
		Success:     true,
		Message:     "Camera ativada para analise visual. Peça ao idoso para mostrar o objeto à câmera.",
		SuggestTone: "instrucional",
		Data: map[string]interface{}{
			"action":         "camera_analysis",
			"user_id":        call.UserID,
			"activate_video": true, // signals browser to start sending video frames
		},
		SideEffects: []swarm.SideEffect{
			{Type: "browser_command", Payload: `{"command":"start_video_capture","duration_seconds":30}`},
		},
	}, nil
}

// ── NietzscheDB Persistence ─────────────────────────────────────────────────

// scaleTypeFromResponseType extracts the scale name from the submit handler name.
// "submit_phq9_response" → "PHQ9", "submit_gad7_response" → "GAD7", etc.
func scaleTypeFromResponseType(responseType string) string {
	switch responseType {
	case "submit_phq9_response":
		return "PHQ9"
	case "submit_gad7_response":
		return "GAD7"
	case "submit_cssrs_response":
		return "C-SSRS"
	default:
		return responseType
	}
}

// ensurePatientNode finds or creates the Patient node in patient_graph.
// Uses MergeNode with NodeType="Person" and match key "id" = userID,
// consistent with the pattern used across the codebase (e.g. rem_consolidator).
func (a *Agent) ensurePatientNode(ctx context.Context, userID int64) (string, error) {
	result, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Person",
		MatchKeys: map[string]interface{}{
			"id": userID,
		},
		OnCreateSet: map[string]interface{}{
			"id":         userID,
			"created_at": nietzscheInfra.NowUnix(),
			"source":     "clinical_agent",
		},
	})
	if err != nil {
		return "", fmt.Errorf("ensure patient node (userID=%d): %w", userID, err)
	}
	return result.NodeID, nil
}

// persistAssessment creates a ClinicalAssessment node and HAS_ASSESSMENT edge
// from the patient node in patient_graph. Called on each submit_*_response.
func (a *Agent) persistAssessment(ctx context.Context, userID int64, responseType string, questionNum int, response string) (string, error) {
	// 1. Ensure patient node exists
	patientNodeID, err := a.ensurePatientNode(ctx, userID)
	if err != nil {
		return "", err
	}

	scaleType := scaleTypeFromResponseType(responseType)
	now := time.Now()

	// 2. Create ClinicalAssessment node (Semantic + node_label="ClinicalAssessment")
	assessmentNode, err := a.graph.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType: "ClinicalAssessment",
		Content: map[string]interface{}{
			"scale_type":      scaleType,
			"question_number": questionNum,
			"response":        response,
			"patient_id":      userID,
			"timestamp":       float64(now.Unix()),
			"timestamp_iso":   now.Format(time.RFC3339),
			"source":          "clinical_agent",
		},
	})
	if err != nil {
		return "", fmt.Errorf("insert ClinicalAssessment node: %w", err)
	}

	// 3. Create HAS_ASSESSMENT edge from patient to assessment
	_, err = a.graph.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:     patientNodeID,
		To:       assessmentNode.ID,
		EdgeType: "HAS_ASSESSMENT",
	})
	if err != nil {
		// Node was created but edge failed — log but don't fail the tool
		log.Printf("[CLINICAL] WARNING: assessment node %s created but edge failed: %v", assessmentNode.ID, err)
	}

	return assessmentNode.ID, nil
}

// persistMedication creates/merges a MedicationLog node and TOOK_MEDICATION edge
// from the patient node in patient_graph. Uses MergeNode so duplicate confirmations
// on the same day for the same medication are idempotent.
func (a *Agent) persistMedication(ctx context.Context, userID int64, medicationName string) (string, error) {
	// 1. Ensure patient node exists
	patientNodeID, err := a.ensurePatientNode(ctx, userID)
	if err != nil {
		return "", err
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02") // YYYY-MM-DD for daily dedup

	// 2. MergeNode: find or create MedicationLog (dedup by patient_id + date + medication_name)
	medResult, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "MedicationLog",
		MatchKeys: map[string]interface{}{
			"patient_id":      userID,
			"date":            dateStr,
			"medication_name": medicationName,
		},
		OnCreateSet: map[string]interface{}{
			"patient_id":      userID,
			"date":            dateStr,
			"medication_name": medicationName,
			"confirmed":       true,
			"confirmed_at":    float64(now.Unix()),
			"timestamp_iso":   now.Format(time.RFC3339),
			"source":          "clinical_agent",
		},
		OnMatchSet: map[string]interface{}{
			"confirmed":    true,
			"confirmed_at": float64(now.Unix()),
			"updated_iso":  now.Format(time.RFC3339),
		},
	})
	if err != nil {
		return "", fmt.Errorf("merge MedicationLog node: %w", err)
	}

	// 3. Create TOOK_MEDICATION edge from patient to medication log
	if medResult.Created {
		_, err = a.graph.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
			From:     patientNodeID,
			To:       medResult.NodeID,
			EdgeType: "TOOK_MEDICATION",
		})
		if err != nil {
			log.Printf("[CLINICAL] WARNING: MedicationLog node %s created but edge failed: %v", medResult.NodeID, err)
		}
	}

	return medResult.NodeID, nil
}
