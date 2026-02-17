// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"eva-mind/internal/cortex/scales"
	"eva-mind/internal/motor/actions"
	"fmt"
	"log"
)

// handleSubmitPHQ9Response processa resposta de uma pergunta do PHQ-9
func (h *ToolsHandler) handleSubmitPHQ9Response(idosoID int64, sessionID string, questionNumber int, responseValue int, responseText string) (map[string]interface{}, error) {
	log.Printf("📝 [PHQ-9] Recebendo resposta Q%d do paciente %d (sessão: %s)", questionNumber, idosoID, sessionID)

	// 1. Buscar assessment ID
	var assessmentID int64
	query := `
		SELECT id FROM clinical_assessments
		WHERE patient_id = $1 AND session_id = $2 AND assessment_type = 'PHQ-9'
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := h.db.Conn.QueryRow(query, idosoID, sessionID).Scan(&assessmentID)
	if err != nil {
		log.Printf("❌ [PHQ-9] Assessment não encontrado: %v", err)
		return map[string]interface{}{"error": "Sessão PHQ-9 não encontrada"}, nil
	}

	// 2. Salvar resposta
	queryInsert := `
		INSERT INTO clinical_assessment_responses (
			assessment_id, question_number, question_text, response_value, response_text
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (assessment_id, question_number) DO UPDATE
		SET response_value = $4, response_text = $5, responded_at = NOW()
	`

	questions := scales.GetPHQ9Questions()
	questionText := ""
	if questionNumber > 0 && questionNumber <= len(questions) {
		questionText = questions[questionNumber-1].Text
	}

	_, err = h.db.Conn.Exec(queryInsert, assessmentID, questionNumber, questionText, responseValue, responseText)
	if err != nil {
		log.Printf("❌ [PHQ-9] Erro ao salvar resposta: %v", err)
		return map[string]interface{}{"error": "Erro ao salvar resposta"}, nil
	}

	// 3. Verificar se completou todas as perguntas
	var responsesCount int
	queryCount := `SELECT COUNT(*) FROM clinical_assessment_responses WHERE assessment_id = $1`
	h.db.Conn.QueryRow(queryCount, assessmentID).Scan(&responsesCount)

	// Se completou 9 perguntas, calcular score final
	if responsesCount >= 9 {
		return h.finalizePHQ9Assessment(idosoID, assessmentID, sessionID)
	}

	// 4. Retornar próxima pergunta
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
				"options": []string{"Nenhuma vez", "Vários dias", "Mais da metade dos dias", "Quase todos os dias"},
			},
		}, nil
	}

	return map[string]interface{}{"error": "Pergunta inválida"}, nil
}

// finalizePHQ9Assessment calcula score final e completa assessment
func (h *ToolsHandler) finalizePHQ9Assessment(idosoID int64, assessmentID int64, sessionID string) (map[string]interface{}, error) {
	log.Printf("✅ [PHQ-9] Finalizando assessment %d para paciente %d", assessmentID, idosoID)

	// 1. Buscar todas as respostas
	queryResponses := `
		SELECT response_value
		FROM clinical_assessment_responses
		WHERE assessment_id = $1
		ORDER BY question_number
	`

	rows, err := h.db.Conn.Query(queryResponses, assessmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var responses []scales.PHQ9Response
	questionNum := 1
	for rows.Next() {
		var value int
		rows.Scan(&value)
		responses = append(responses, scales.PHQ9Response{
			Question: questionNum,
			Score:    value,
		})
		questionNum++
	}

	// 2. Calcular score
	scalesManager := scales.NewClinicalScalesManager(h.db)
	result := scalesManager.CalculatePHQ9Score(responses)

	// 3. Atualizar assessment com resultado
	queryUpdate := `
		UPDATE clinical_assessments
		SET total_score = $1,
		    severity_level = $2,
		    clinical_interpretation = $3,
		    status = 'completed',
		    completed_at = NOW()
		WHERE id = $4
	`

	interpretation := fmt.Sprintf("PHQ-9 Score: %d - %s", result.TotalScore, result.SeverityLevel)
	_, err = h.db.Conn.Exec(queryUpdate, result.TotalScore, result.SeverityLevel, interpretation, assessmentID)
	if err != nil {
		log.Printf("❌ [PHQ-9] Erro ao atualizar assessment: %v", err)
		return nil, err
	}

	// 4. Verificar se precisa alertar (risco de suicídio na Q9)
	shouldAlert := false
	alertMessage := ""

	if result.SuicideRisk {
		shouldAlert = true
		alertMessage = fmt.Sprintf("🚨 ALERTA: Paciente indicou pensamentos suicidas (Q9). Score PHQ-9: %d (%s)", result.TotalScore, result.SeverityLevel)

		// Alerta crítico para família/equipe
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
		log.Printf("🚨 [PHQ-9] Risco suicida detectado. Recomendando C-SSRS para paciente %d", idosoID)
	} else if result.SeverityLevel == "severe" || result.SeverityLevel == "moderately_severe" {
		shouldAlert = true
		alertMessage = fmt.Sprintf("⚠️ ATENÇÃO: Depressão %s detectada. Score PHQ-9: %d", result.SeverityLevel, result.TotalScore)

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

	log.Printf("✅ [PHQ-9] Assessment completado. Score: %d, Severidade: %s", result.TotalScore, result.SeverityLevel)

	return response, nil
}

// handleSubmitGAD7Response processa resposta de uma pergunta do GAD-7
func (h *ToolsHandler) handleSubmitGAD7Response(idosoID int64, sessionID string, questionNumber int, responseValue int, responseText string) (map[string]interface{}, error) {
	log.Printf("📝 [GAD-7] Recebendo resposta Q%d do paciente %d (sessão: %s)", questionNumber, idosoID, sessionID)

	// 1. Buscar assessment ID
	var assessmentID int64
	query := `
		SELECT id FROM clinical_assessments
		WHERE patient_id = $1 AND session_id = $2 AND assessment_type = 'GAD-7'
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := h.db.Conn.QueryRow(query, idosoID, sessionID).Scan(&assessmentID)
	if err != nil {
		log.Printf("❌ [GAD-7] Assessment não encontrado: %v", err)
		return map[string]interface{}{"error": "Sessão GAD-7 não encontrada"}, nil
	}

	// 2. Salvar resposta
	queryInsert := `
		INSERT INTO clinical_assessment_responses (
			assessment_id, question_number, question_text, response_value, response_text
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (assessment_id, question_number) DO UPDATE
		SET response_value = $4, response_text = $5, responded_at = NOW()
	`

	questions := scales.GetGAD7Questions()
	questionText := ""
	if questionNumber > 0 && questionNumber <= len(questions) {
		questionText = questions[questionNumber-1].Text
	}

	_, err = h.db.Conn.Exec(queryInsert, assessmentID, questionNumber, questionText, responseValue, responseText)
	if err != nil {
		log.Printf("❌ [GAD-7] Erro ao salvar resposta: %v", err)
		return map[string]interface{}{"error": "Erro ao salvar resposta"}, nil
	}

	// 3. Verificar se completou todas as perguntas
	var responsesCount int
	queryCount := `SELECT COUNT(*) FROM clinical_assessment_responses WHERE assessment_id = $1`
	h.db.Conn.QueryRow(queryCount, assessmentID).Scan(&responsesCount)

	// Se completou 7 perguntas, calcular score final
	if responsesCount >= 7 {
		return h.finalizeGAD7Assessment(idosoID, assessmentID, sessionID)
	}

	// 4. Retornar próxima pergunta
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
				"options": []string{"Nenhuma vez", "Vários dias", "Mais da metade dos dias", "Quase todos os dias"},
			},
		}, nil
	}

	return map[string]interface{}{"error": "Pergunta inválida"}, nil
}

// finalizeGAD7Assessment calcula score final e completa assessment
func (h *ToolsHandler) finalizeGAD7Assessment(idosoID int64, assessmentID int64, sessionID string) (map[string]interface{}, error) {
	log.Printf("✅ [GAD-7] Finalizando assessment %d para paciente %d", assessmentID, idosoID)

	// 1. Buscar todas as respostas
	queryResponses := `
		SELECT response_value
		FROM clinical_assessment_responses
		WHERE assessment_id = $1
		ORDER BY question_number
	`

	rows, err := h.db.Conn.Query(queryResponses, assessmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var responses []scales.GAD7Response
	questionNum := 1
	for rows.Next() {
		var value int
		rows.Scan(&value)
		responses = append(responses, scales.GAD7Response{
			Question: questionNum,
			Score:    value,
		})
		questionNum++
	}

	// 2. Calcular score
	scalesManager := scales.NewClinicalScalesManager(h.db)
	result := scalesManager.CalculateGAD7Score(responses)

	// 3. Atualizar assessment com resultado
	queryUpdate := `
		UPDATE clinical_assessments
		SET total_score = $1,
		    severity_level = $2,
		    clinical_interpretation = $3,
		    status = 'completed',
		    completed_at = NOW()
		WHERE id = $4
	`

	interpretationGAD := fmt.Sprintf("GAD-7 Score: %d - %s", result.TotalScore, result.SeverityLevel)
	_, err = h.db.Conn.Exec(queryUpdate, result.TotalScore, result.SeverityLevel, interpretationGAD, assessmentID)
	if err != nil {
		log.Printf("❌ [GAD-7] Erro ao atualizar assessment: %v", err)
		return nil, err
	}

	// 4. Verificar se precisa alertar
	shouldAlert := false
	alertMessage := ""

	if result.SeverityLevel == "severe" {
		shouldAlert = true
		alertMessage = fmt.Sprintf("⚠️ ATENÇÃO: Ansiedade severa detectada. Score GAD-7: %d", result.TotalScore)

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

	log.Printf("✅ [GAD-7] Assessment completado. Score: %d, Severidade: %s", result.TotalScore, result.SeverityLevel)

	return response, nil
}

// handleSubmitCSSRSResponse processa resposta de uma pergunta do C-SSRS
func (h *ToolsHandler) handleSubmitCSSRSResponse(idosoID int64, sessionID string, questionNumber int, responseValue int, responseText string) (map[string]interface{}, error) {
	log.Printf("🚨 [C-SSRS] Recebendo resposta Q%d do paciente %d (sessão: %s)", questionNumber, idosoID, sessionID)

	// 1. Buscar assessment ID
	var assessmentID int64
	query := `
		SELECT id FROM clinical_assessments
		WHERE patient_id = $1 AND session_id = $2 AND assessment_type = 'C-SSRS'
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := h.db.Conn.QueryRow(query, idosoID, sessionID).Scan(&assessmentID)
	if err != nil {
		log.Printf("❌ [C-SSRS] Assessment não encontrado: %v", err)
		return map[string]interface{}{"error": "Sessão C-SSRS não encontrada"}, nil
	}

	// 2. Salvar resposta
	queryInsert := `
		INSERT INTO clinical_assessment_responses (
			assessment_id, question_number, question_text, response_value, response_text
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (assessment_id, question_number) DO UPDATE
		SET response_value = $4, response_text = $5, responded_at = NOW()
	`

	questions := scales.GetCSSRSQuestions()
	questionText := ""
	if questionNumber > 0 && questionNumber <= len(questions) {
		questionText = questions[questionNumber-1].Text
	}

	_, err = h.db.Conn.Exec(queryInsert, assessmentID, questionNumber, questionText, responseValue, responseText)
	if err != nil {
		log.Printf("❌ [C-SSRS] Erro ao salvar resposta: %v", err)
		return map[string]interface{}{"error": "Erro ao salvar resposta"}, nil
	}

	// 3. CRÍTICO: Se resposta positiva em qualquer pergunta, alerta imediato
	if responseValue == 1 { // Sim
		log.Printf("🚨🚨🚨 [C-SSRS] RESPOSTA POSITIVA na Q%d - ALERTA CRÍTICO!", questionNumber)

		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "critical_suicide_risk", map[string]interface{}{
				"cssrs_question":  questionNumber,
				"question_text":   questionText,
				"response":        "positive",
				"session_id":      sessionID,
				"requires_immediate_action": true,
			})
		}

		// Alerta também via push/email tradicional
		_ = actions.AlertFamilyWithSeverity(h.db.Conn, h.pushService, h.emailService, idosoID,
			fmt.Sprintf("🚨🚨🚨 EMERGÊNCIA: Risco suicida CRÍTICO detectado (C-SSRS Q%d positiva). AÇÃO IMEDIATA NECESSÁRIA.", questionNumber),
			"critica")
	}

	// 4. Verificar se completou todas as perguntas
	var responsesCount int
	queryCount := `SELECT COUNT(*) FROM clinical_assessment_responses WHERE assessment_id = $1`
	h.db.Conn.QueryRow(queryCount, assessmentID).Scan(&responsesCount)

	// Se completou 6 perguntas, calcular score final
	if responsesCount >= 6 {
		return h.finalizeCSSRSAssessment(idosoID, assessmentID, sessionID)
	}

	// 5. Retornar próxima pergunta
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
				"options": []string{"Sim", "Não"},
			},
		}, nil
	}

	return map[string]interface{}{"error": "Pergunta inválida"}, nil
}

// finalizeCSSRSAssessment calcula score final e completa assessment
func (h *ToolsHandler) finalizeCSSRSAssessment(idosoID int64, assessmentID int64, sessionID string) (map[string]interface{}, error) {
	log.Printf("🚨 [C-SSRS] Finalizando assessment CRÍTICO %d para paciente %d", assessmentID, idosoID)

	// 1. Buscar todas as respostas
	queryResponses := `
		SELECT response_value
		FROM clinical_assessment_responses
		WHERE assessment_id = $1
		ORDER BY question_number
	`

	rows, err := h.db.Conn.Query(queryResponses, assessmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var responses []scales.CSSRSResponse
	questionNum := 1
	positiveCount := 0

	for rows.Next() {
		var value int
		rows.Scan(&value)

		if value == 1 {
			positiveCount++
		}

		responses = append(responses, scales.CSSRSResponse{
			Question: questionNum,
			Answer:   value == 1,
		})
		questionNum++
	}

	// 2. Calcular score e risco
	scalesManager := scales.NewClinicalScalesManager(h.db)
	result := scalesManager.CalculateCSSRSScore(responses)

	// 3. Atualizar assessment com resultado
	queryUpdate := `
		UPDATE clinical_assessments
		SET total_score = $1,
		    severity_level = $2,
		    clinical_interpretation = $3,
		    status = 'completed',
		    completed_at = NOW(),
		    alert_sent = TRUE
		WHERE id = $4
	`

	interpretationCSSRS := fmt.Sprintf("C-SSRS Risk Level: %s - %d positive responses", result.RiskLevel, positiveCount)
	_, err = h.db.Conn.Exec(queryUpdate, positiveCount, result.RiskLevel, interpretationCSSRS, assessmentID)
	if err != nil {
		log.Printf("❌ [C-SSRS] Erro ao atualizar assessment: %v", err)
		return nil, err
	}

	// 4. Alerta SEMPRE (qualquer resultado de C-SSRS é sério)
	alertSeverity := "alta"
	if result.RiskLevel == "high" || result.RiskLevel == "critical" {
		alertSeverity = "critica"
	}

	alertMessage := fmt.Sprintf("🚨 C-SSRS completado: %s. %d respostas positivas.", result.RiskLevel, positiveCount)

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "cssrs_completed", map[string]interface{}{
			"risk_level":       result.RiskLevel,
			"positive_responses": positiveCount,
			"session_id":       sessionID,
			"requires_professional_intervention": result.RiskLevel != "none" && result.RiskLevel != "low",
		})
	}

	// Alerta família/equipe via sistema tradicional
	_ = actions.AlertFamilyWithSeverity(h.db.Conn, h.pushService, h.emailService, idosoID,
		alertMessage, alertSeverity)

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

	// Se risco alto/crítico, adicionar instruções de emergência
	if result.RiskLevel == "high" || result.RiskLevel == "critical" {
		response["emergency_protocol"] = true
		response["emergency_actions"] = []string{
			"Contato imediato com profissional de saúde mental",
			"Não deixar paciente sozinho",
			"Remover meios letais do ambiente",
			"Ligar para CVV: 188 se necessário",
		}
	}

	log.Printf("🚨 [C-SSRS] Assessment CRÍTICO completado. Risco: %s, Positivas: %d", result.RiskLevel, positiveCount)

	return response, nil
}
