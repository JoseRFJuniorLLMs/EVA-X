// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"eva/internal/brainstem/database"
	"eva/internal/cortex/scales"
	"eva/internal/motor/actions"
	"fmt"
	"log"
	"time"
)

// handleSubmitPHQ9Response processa resposta de uma pergunta do PHQ-9
func (h *ToolsHandler) handleSubmitPHQ9Response(idosoID int64, sessionID string, questionNumber int, responseValue int, responseText string) (map[string]interface{}, error) {
	log.Printf("[PHQ-9] Recebendo resposta Q%d do paciente %d (sessao: %s)", questionNumber, idosoID, sessionID)
	ctx := context.Background()

	// 1. Buscar assessment ID
	rows, err := h.db.QueryByLabel(ctx, "clinical_assessments",
		" AND n.patient_id = $pid AND n.session_id = $sid AND n.assessment_type = $atype ORDER BY n.created_at DESC",
		map[string]interface{}{"pid": idosoID, "sid": sessionID, "atype": "PHQ-9"}, 1)
	if err != nil || len(rows) == 0 {
		log.Printf("[PHQ-9] Assessment nao encontrado: %v", err)
		return map[string]interface{}{"error": "Sessao PHQ-9 nao encontrada"}, nil
	}
	assessmentID := database.GetInt64(rows[0], "id")

	// 2. Salvar resposta (upsert via Update or Insert)
	questions := scales.GetPHQ9Questions()
	questionText := ""
	if questionNumber > 0 && questionNumber <= len(questions) {
		questionText = questions[questionNumber-1].Text
	}

	// Try update first, if no match then insert
	existingRows, _ := h.db.QueryByLabel(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid AND n.question_number = $qn",
		map[string]interface{}{"aid": assessmentID, "qn": questionNumber}, 1)

	if len(existingRows) > 0 {
		err = h.db.Update(ctx, "clinical_assessment_responses",
			map[string]interface{}{"assessment_id": assessmentID, "question_number": questionNumber},
			map[string]interface{}{"response_value": responseValue, "response_text": responseText, "responded_at": time.Now().Format(time.RFC3339)})
	} else {
		_, err = h.db.Insert(ctx, "clinical_assessment_responses", map[string]interface{}{
			"assessment_id":   assessmentID,
			"question_number": questionNumber,
			"question_text":   questionText,
			"response_value":  responseValue,
			"response_text":   responseText,
			"responded_at":    time.Now().Format(time.RFC3339),
		})
	}
	if err != nil {
		log.Printf("[PHQ-9] Erro ao salvar resposta: %v", err)
		return map[string]interface{}{"error": "Erro ao salvar resposta"}, nil
	}

	// 3. Verificar se completou todas as perguntas
	responsesCount, _ := h.db.Count(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid", map[string]interface{}{"aid": assessmentID})

	// Se completou 9 perguntas, calcular score final
	if responsesCount >= 9 {
		return h.finalizePHQ9Assessment(idosoID, assessmentID, sessionID)
	}

	// 4. Retornar proxima pergunta
	nextQuestion := questionNumber + 1
	if nextQuestion <= len(questions) {
		return map[string]interface{}{
			"status":             "in_progress",
			"session_id":         sessionID,
			"questions_answered": responsesCount,
			"total_questions":    9,
			"next_question": map[string]interface{}{
				"number":  nextQuestion,
				"text":    questions[nextQuestion-1].Text,
				"options": []string{"Nenhuma vez", "Varios dias", "Mais da metade dos dias", "Quase todos os dias"},
			},
		}, nil
	}

	return map[string]interface{}{"error": "Pergunta invalida"}, nil
}

// finalizePHQ9Assessment calcula score final e completa assessment
func (h *ToolsHandler) finalizePHQ9Assessment(idosoID int64, assessmentID int64, sessionID string) (map[string]interface{}, error) {
	log.Printf("[PHQ-9] Finalizando assessment %d para paciente %d", assessmentID, idosoID)
	ctx := context.Background()

	// 1. Buscar todas as respostas
	responseRows, err := h.db.QueryByLabel(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid ORDER BY n.question_number",
		map[string]interface{}{"aid": assessmentID}, 0)
	if err != nil {
		return nil, err
	}

	var responses []scales.PHQ9Response
	for i, row := range responseRows {
		value := int(database.GetInt64(row, "response_value"))
		responses = append(responses, scales.PHQ9Response{
			Question: i + 1,
			Score:    value,
		})
	}

	// 2. Calcular score
	scalesManager := scales.NewClinicalScalesManager(h.db)
	result := scalesManager.CalculatePHQ9Score(responses)

	// 3. Atualizar assessment com resultado
	interpretation := fmt.Sprintf("PHQ-9 Score: %d - %s", result.TotalScore, result.SeverityLevel)
	err = h.db.Update(ctx, "clinical_assessments",
		map[string]interface{}{"id": assessmentID},
		map[string]interface{}{
			"total_score":             result.TotalScore,
			"severity_level":          result.SeverityLevel,
			"clinical_interpretation": interpretation,
			"status":                  "completed",
			"completed_at":            time.Now().Format(time.RFC3339),
		})
	if err != nil {
		log.Printf("[PHQ-9] Erro ao atualizar assessment: %v", err)
		return nil, err
	}

	// 4. Verificar se precisa alertar (risco de suicidio na Q9)
	shouldAlert := false
	alertMessage := ""

	if result.SuicideRisk {
		shouldAlert = true
		alertMessage = fmt.Sprintf("ALERTA: Paciente indicou pensamentos suicidas (Q9). Score PHQ-9: %d (%s)", result.TotalScore, result.SeverityLevel)

		// Alerta critico para familia/equipe
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "suicide_risk_detected", map[string]interface{}{
				"phq9_score":      result.TotalScore,
				"severity":        result.SeverityLevel,
				"session_id":      sessionID,
				"q9_response":     "positive",
				"requires_c_ssrs": true,
			})
		}

		// Recomendar C-SSRS imediato
		log.Printf("[PHQ-9] Risco suicida detectado. Recomendando C-SSRS para paciente %d", idosoID)
	} else if result.SeverityLevel == "severe" || result.SeverityLevel == "moderately_severe" {
		shouldAlert = true
		alertMessage = fmt.Sprintf("ATENCAO: Depressao %s detectada. Score PHQ-9: %d", result.SeverityLevel, result.TotalScore)

		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "high_depression_score", map[string]interface{}{
				"phq9_score": result.TotalScore,
				"severity":   result.SeverityLevel,
				"session_id": sessionID,
			})
		}
	}

	// 5. Retornar resultado
	response := map[string]interface{}{
		"status":          "completed",
		"session_id":      sessionID,
		"assessment_type": "PHQ-9",
		"total_score":     result.TotalScore,
		"severity":        result.SeverityLevel,
		"interpretation":  interpretation,
		"suicide_risk":    result.SuicideRisk,
		"recommendations": result.Recommendations,
	}

	if shouldAlert {
		response["alert"] = true
		response["alert_message"] = alertMessage
	}

	log.Printf("[PHQ-9] Assessment completado. Score: %d, Severidade: %s", result.TotalScore, result.SeverityLevel)

	return response, nil
}

// handleSubmitGAD7Response processa resposta de uma pergunta do GAD-7
func (h *ToolsHandler) handleSubmitGAD7Response(idosoID int64, sessionID string, questionNumber int, responseValue int, responseText string) (map[string]interface{}, error) {
	log.Printf("[GAD-7] Recebendo resposta Q%d do paciente %d (sessao: %s)", questionNumber, idosoID, sessionID)
	ctx := context.Background()

	// 1. Buscar assessment ID
	rows, err := h.db.QueryByLabel(ctx, "clinical_assessments",
		" AND n.patient_id = $pid AND n.session_id = $sid AND n.assessment_type = $atype ORDER BY n.created_at DESC",
		map[string]interface{}{"pid": idosoID, "sid": sessionID, "atype": "GAD-7"}, 1)
	if err != nil || len(rows) == 0 {
		log.Printf("[GAD-7] Assessment nao encontrado: %v", err)
		return map[string]interface{}{"error": "Sessao GAD-7 nao encontrada"}, nil
	}
	assessmentID := database.GetInt64(rows[0], "id")

	// 2. Salvar resposta (upsert via Update or Insert)
	questions := scales.GetGAD7Questions()
	questionText := ""
	if questionNumber > 0 && questionNumber <= len(questions) {
		questionText = questions[questionNumber-1].Text
	}

	existingRows, _ := h.db.QueryByLabel(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid AND n.question_number = $qn",
		map[string]interface{}{"aid": assessmentID, "qn": questionNumber}, 1)

	if len(existingRows) > 0 {
		err = h.db.Update(ctx, "clinical_assessment_responses",
			map[string]interface{}{"assessment_id": assessmentID, "question_number": questionNumber},
			map[string]interface{}{"response_value": responseValue, "response_text": responseText, "responded_at": time.Now().Format(time.RFC3339)})
	} else {
		_, err = h.db.Insert(ctx, "clinical_assessment_responses", map[string]interface{}{
			"assessment_id":   assessmentID,
			"question_number": questionNumber,
			"question_text":   questionText,
			"response_value":  responseValue,
			"response_text":   responseText,
			"responded_at":    time.Now().Format(time.RFC3339),
		})
	}
	if err != nil {
		log.Printf("[GAD-7] Erro ao salvar resposta: %v", err)
		return map[string]interface{}{"error": "Erro ao salvar resposta"}, nil
	}

	// 3. Verificar se completou todas as perguntas
	responsesCount, _ := h.db.Count(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid", map[string]interface{}{"aid": assessmentID})

	// Se completou 7 perguntas, calcular score final
	if responsesCount >= 7 {
		return h.finalizeGAD7Assessment(idosoID, assessmentID, sessionID)
	}

	// 4. Retornar proxima pergunta
	nextQuestion := questionNumber + 1
	if nextQuestion <= len(questions) {
		return map[string]interface{}{
			"status":             "in_progress",
			"session_id":         sessionID,
			"questions_answered": responsesCount,
			"total_questions":    7,
			"next_question": map[string]interface{}{
				"number":  nextQuestion,
				"text":    questions[nextQuestion-1].Text,
				"options": []string{"Nenhuma vez", "Varios dias", "Mais da metade dos dias", "Quase todos os dias"},
			},
		}, nil
	}

	return map[string]interface{}{"error": "Pergunta invalida"}, nil
}

// finalizeGAD7Assessment calcula score final e completa assessment
func (h *ToolsHandler) finalizeGAD7Assessment(idosoID int64, assessmentID int64, sessionID string) (map[string]interface{}, error) {
	log.Printf("[GAD-7] Finalizando assessment %d para paciente %d", assessmentID, idosoID)
	ctx := context.Background()

	// 1. Buscar todas as respostas
	responseRows, err := h.db.QueryByLabel(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid ORDER BY n.question_number",
		map[string]interface{}{"aid": assessmentID}, 0)
	if err != nil {
		return nil, err
	}

	var responses []scales.GAD7Response
	for i, row := range responseRows {
		value := int(database.GetInt64(row, "response_value"))
		responses = append(responses, scales.GAD7Response{
			Question: i + 1,
			Score:    value,
		})
	}

	// 2. Calcular score
	scalesManager := scales.NewClinicalScalesManager(h.db)
	result := scalesManager.CalculateGAD7Score(responses)

	// 3. Atualizar assessment com resultado
	interpretationGAD := fmt.Sprintf("GAD-7 Score: %d - %s", result.TotalScore, result.SeverityLevel)
	err = h.db.Update(ctx, "clinical_assessments",
		map[string]interface{}{"id": assessmentID},
		map[string]interface{}{
			"total_score":             result.TotalScore,
			"severity_level":          result.SeverityLevel,
			"clinical_interpretation": interpretationGAD,
			"status":                  "completed",
			"completed_at":            time.Now().Format(time.RFC3339),
		})
	if err != nil {
		log.Printf("[GAD-7] Erro ao atualizar assessment: %v", err)
		return nil, err
	}

	// 4. Verificar se precisa alertar
	shouldAlert := false
	alertMessage := ""

	if result.SeverityLevel == "severe" {
		shouldAlert = true
		alertMessage = fmt.Sprintf("ATENCAO: Ansiedade severa detectada. Score GAD-7: %d", result.TotalScore)

		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "high_anxiety_score", map[string]interface{}{
				"gad7_score": result.TotalScore,
				"severity":   result.SeverityLevel,
				"session_id": sessionID,
			})
		}
	}

	// 5. Retornar resultado
	response := map[string]interface{}{
		"status":          "completed",
		"session_id":      sessionID,
		"assessment_type": "GAD-7",
		"total_score":     result.TotalScore,
		"severity":        result.SeverityLevel,
		"interpretation":  interpretationGAD,
		"recommendations": result.Recommendations,
	}

	if shouldAlert {
		response["alert"] = true
		response["alert_message"] = alertMessage
	}

	log.Printf("[GAD-7] Assessment completado. Score: %d, Severidade: %s", result.TotalScore, result.SeverityLevel)

	return response, nil
}

// handleSubmitCSSRSResponse processa resposta de uma pergunta do C-SSRS
func (h *ToolsHandler) handleSubmitCSSRSResponse(idosoID int64, sessionID string, questionNumber int, responseValue int, responseText string) (map[string]interface{}, error) {
	log.Printf("[C-SSRS] Recebendo resposta Q%d do paciente %d (sessao: %s)", questionNumber, idosoID, sessionID)
	ctx := context.Background()

	// 1. Buscar assessment ID
	rows, err := h.db.QueryByLabel(ctx, "clinical_assessments",
		" AND n.patient_id = $pid AND n.session_id = $sid AND n.assessment_type = $atype ORDER BY n.created_at DESC",
		map[string]interface{}{"pid": idosoID, "sid": sessionID, "atype": "C-SSRS"}, 1)
	if err != nil || len(rows) == 0 {
		log.Printf("[C-SSRS] Assessment nao encontrado: %v", err)
		return map[string]interface{}{"error": "Sessao C-SSRS nao encontrada"}, nil
	}
	assessmentID := database.GetInt64(rows[0], "id")

	// 2. Salvar resposta (upsert via Update or Insert)
	questions := scales.GetCSSRSQuestions()
	questionText := ""
	if questionNumber > 0 && questionNumber <= len(questions) {
		questionText = questions[questionNumber-1].Text
	}

	existingRows, _ := h.db.QueryByLabel(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid AND n.question_number = $qn",
		map[string]interface{}{"aid": assessmentID, "qn": questionNumber}, 1)

	if len(existingRows) > 0 {
		err = h.db.Update(ctx, "clinical_assessment_responses",
			map[string]interface{}{"assessment_id": assessmentID, "question_number": questionNumber},
			map[string]interface{}{"response_value": responseValue, "response_text": responseText, "responded_at": time.Now().Format(time.RFC3339)})
	} else {
		_, err = h.db.Insert(ctx, "clinical_assessment_responses", map[string]interface{}{
			"assessment_id":   assessmentID,
			"question_number": questionNumber,
			"question_text":   questionText,
			"response_value":  responseValue,
			"response_text":   responseText,
			"responded_at":    time.Now().Format(time.RFC3339),
		})
	}
	if err != nil {
		log.Printf("[C-SSRS] Erro ao salvar resposta: %v", err)
		return map[string]interface{}{"error": "Erro ao salvar resposta"}, nil
	}

	// 3. CRITICO: Se resposta positiva em qualquer pergunta, alerta imediato
	if responseValue == 1 { // Sim
		log.Printf("[C-SSRS] RESPOSTA POSITIVA na Q%d - ALERTA CRITICO!", questionNumber)

		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "critical_suicide_risk", map[string]interface{}{
				"cssrs_question":            questionNumber,
				"question_text":             questionText,
				"response":                  "positive",
				"session_id":                sessionID,
				"requires_immediate_action": true,
			})
		}

		// Alerta tambem via push/email tradicional
		if h.db != nil {
			_ = actions.AlertFamilyWithSeverity(h.db, h.pushService, h.emailService, idosoID,
				fmt.Sprintf("EMERGENCIA: Risco suicida CRITICO detectado (C-SSRS Q%d positiva). ACAO IMEDIATA NECESSARIA.", questionNumber),
				"critica")
		}
	}

	// 4. Verificar se completou todas as perguntas
	if h.db == nil {
		return map[string]interface{}{"error": "Database indisponivel para C-SSRS"}, nil
	}
	responsesCount, _ := h.db.Count(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid", map[string]interface{}{"aid": assessmentID})

	// Se completou 6 perguntas, calcular score final
	if responsesCount >= 6 {
		return h.finalizeCSSRSAssessment(idosoID, assessmentID, sessionID)
	}

	// 5. Retornar proxima pergunta
	nextQuestion := questionNumber + 1
	if nextQuestion <= len(questions) {
		return map[string]interface{}{
			"status":             "in_progress",
			"session_id":         sessionID,
			"questions_answered": responsesCount,
			"total_questions":    6,
			"priority":           "CRITICAL",
			"next_question": map[string]interface{}{
				"number":  nextQuestion,
				"text":    questions[nextQuestion-1].Text,
				"options": []string{"Sim", "Nao"},
			},
		}, nil
	}

	return map[string]interface{}{"error": "Pergunta invalida"}, nil
}

// finalizeCSSRSAssessment calcula score final e completa assessment
func (h *ToolsHandler) finalizeCSSRSAssessment(idosoID int64, assessmentID int64, sessionID string) (map[string]interface{}, error) {
	log.Printf("[C-SSRS] Finalizando assessment CRITICO %d para paciente %d", assessmentID, idosoID)
	ctx := context.Background()

	// 1. Buscar todas as respostas
	responseRows, err := h.db.QueryByLabel(ctx, "clinical_assessment_responses",
		" AND n.assessment_id = $aid ORDER BY n.question_number",
		map[string]interface{}{"aid": assessmentID}, 0)
	if err != nil {
		return nil, err
	}

	var responses []scales.CSSRSResponse
	positiveCount := 0

	for i, row := range responseRows {
		value := int(database.GetInt64(row, "response_value"))

		if value == 1 {
			positiveCount++
		}

		responses = append(responses, scales.CSSRSResponse{
			Question: i + 1,
			Answer:   value == 1,
		})
	}

	// 2. Calcular score e risco
	scalesManager := scales.NewClinicalScalesManager(h.db)
	result := scalesManager.CalculateCSSRSScore(responses)

	// 3. Atualizar assessment com resultado
	interpretationCSSRS := fmt.Sprintf("C-SSRS Risk Level: %s - %d positive responses", result.RiskLevel, positiveCount)
	err = h.db.Update(ctx, "clinical_assessments",
		map[string]interface{}{"id": assessmentID},
		map[string]interface{}{
			"total_score":             positiveCount,
			"severity_level":          result.RiskLevel,
			"clinical_interpretation": interpretationCSSRS,
			"status":                  "completed",
			"completed_at":            time.Now().Format(time.RFC3339),
			"alert_sent":              true,
		})
	if err != nil {
		log.Printf("[C-SSRS] Erro ao atualizar assessment: %v", err)
		return nil, err
	}

	// 4. Alerta SEMPRE (qualquer resultado de C-SSRS e serio)
	alertSeverity := "alta"
	if result.RiskLevel == "high" || result.RiskLevel == "critical" {
		alertSeverity = "critica"
	}

	alertMessage := fmt.Sprintf("C-SSRS completado: %s. %d respostas positivas.", result.RiskLevel, positiveCount)

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "cssrs_completed", map[string]interface{}{
			"risk_level":                         result.RiskLevel,
			"positive_responses":                 positiveCount,
			"session_id":                         sessionID,
			"requires_professional_intervention": result.RiskLevel != "none" && result.RiskLevel != "low",
		})
	}

	// Alerta familia/equipe via sistema tradicional
	if h.db != nil {
		_ = actions.AlertFamilyWithSeverity(h.db, h.pushService, h.emailService, idosoID,
			alertMessage, alertSeverity)
	}

	// 5. Retornar resultado
	response := map[string]interface{}{
		"status":             "completed",
		"session_id":         sessionID,
		"assessment_type":    "C-SSRS",
		"risk_level":         result.RiskLevel,
		"positive_responses": positiveCount,
		"interpretation":     interpretationCSSRS,
		"interventions":      result.Interventions,
		"alert":              true,
		"alert_message":      alertMessage,
		"priority":           "CRITICAL",
	}

	// Se risco alto/critico, adicionar instrucoes de emergencia
	if result.RiskLevel == "high" || result.RiskLevel == "critical" {
		response["emergency_protocol"] = true
		response["emergency_actions"] = []string{
			"Contato imediato com profissional de saude mental",
			"Nao deixar paciente sozinho",
			"Remover meios letais do ambiente",
			"Ligar para CVV: 188 se necessario",
		}
	}

	log.Printf("[C-SSRS] Assessment CRITICO completado. Risco: %s, Positivas: %d", result.RiskLevel, positiveCount)

	return response, nil
}
