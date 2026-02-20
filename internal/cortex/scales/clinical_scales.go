// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package scales

import (
	"eva/internal/brainstem/database"
	"fmt"
	"log"
	"time"
)

// ClinicalScalesManager manages clinical assessment scales
type ClinicalScalesManager struct {
	db *database.DB
}

// NewClinicalScalesManager creates a new clinical scales manager
func NewClinicalScalesManager(db *database.DB) *ClinicalScalesManager {
	return &ClinicalScalesManager{
		db: db,
	}
}

// ====================================================================
// PHQ-9 (Patient Health Questionnaire-9) - DEPRESSÃO
// ====================================================================

// PHQ9Question represents a PHQ-9 question
type PHQ9Question struct {
	Number int
	Text   string
}

// GetPHQ9Questions returns all PHQ-9 questions
func GetPHQ9Questions() []PHQ9Question {
	return []PHQ9Question{
		{1, "Pouco interesse ou prazer em fazer as coisas"},
		{2, "Sentindo-se para baixo, deprimido(a) ou sem esperança"},
		{3, "Dificuldade para pegar no sono, continuar dormindo ou dormir demais"},
		{4, "Sentindo-se cansado(a) ou com pouca energia"},
		{5, "Falta de apetite ou comendo demais"},
		{6, "Sentindo-se mal consigo mesmo(a), ou achando que você é um fracasso ou que decepcionou sua família ou a si mesmo(a)"},
		{7, "Dificuldade para se concentrar nas coisas, como ler o jornal ou ver televisão"},
		{8, "Lentidão para se movimentar ou falar, a ponto das outras pessoas perceberem. Ou o oposto: estar tão agitado(a) ou inquieto(a) que você fica andando de um lado para o outro muito mais do que de costume"},
		{9, "Pensar em se ferir de alguma maneira ou que seria melhor estar morto(a)"},
	}
}

// PHQ9Response represents a patient's response to PHQ-9
type PHQ9Response struct {
	Question int
	Score    int // 0 = Nenhuma vez, 1 = Vários dias, 2 = Mais da metade dos dias, 3 = Quase todos os dias
}

// PHQ9Result represents the assessment result
type PHQ9Result struct {
	TotalScore        int
	SeverityLevel     string // "minimal", "mild", "moderate", "moderately_severe", "severe"
	SuicideRisk       bool
	Responses         []PHQ9Response
	Recommendations   []string
	AssessedAt        time.Time
}

// CalculatePHQ9Score calculates PHQ-9 score and severity
func (m *ClinicalScalesManager) CalculatePHQ9Score(responses []PHQ9Response) *PHQ9Result {
	totalScore := 0
	suicideRisk := false

	for _, resp := range responses {
		totalScore += resp.Score
		// Question 9 is about suicidal ideation
		if resp.Question == 9 && resp.Score > 0 {
			suicideRisk = true
		}
	}

	severity := m.getPHQ9Severity(totalScore)
	recommendations := m.getPHQ9Recommendations(totalScore, suicideRisk)

	return &PHQ9Result{
		TotalScore:      totalScore,
		SeverityLevel:   severity,
		SuicideRisk:     suicideRisk,
		Responses:       responses,
		Recommendations: recommendations,
		AssessedAt:      time.Now(),
	}
}

// getPHQ9Severity returns severity level based on score
func (m *ClinicalScalesManager) getPHQ9Severity(score int) string {
	if score >= 20 {
		return "severe"
	} else if score >= 15 {
		return "moderately_severe"
	} else if score >= 10 {
		return "moderate"
	} else if score >= 5 {
		return "mild"
	}
	return "minimal"
}

// getPHQ9Recommendations generates recommendations
func (m *ClinicalScalesManager) getPHQ9Recommendations(score int, suicideRisk bool) []string {
	recommendations := []string{}

	if suicideRisk {
		recommendations = append(recommendations, "⚠️ URGENTE: Risco de suicídio detectado - contato imediato com profissional de saúde mental")
		recommendations = append(recommendations, "CVV: 188 (24 horas)")
	}

	if score >= 15 {
		recommendations = append(recommendations, "Depressão severa detectada - consulta psiquiátrica urgente recomendada")
		recommendations = append(recommendations, "Considere iniciar tratamento medicamentoso e psicoterapia")
	} else if score >= 10 {
		recommendations = append(recommendations, "Depressão moderada - consulta com psiquiatra ou psicólogo recomendada")
		recommendations = append(recommendations, "Psicoterapia pode ser benéfica")
	} else if score >= 5 {
		recommendations = append(recommendations, "Sintomas leves de depressão - monitoramento recomendado")
		recommendations = append(recommendations, "Atividade física e rotina de sono podem ajudar")
	}

	return recommendations
}

// SavePHQ9Assessment saves assessment to database
func (m *ClinicalScalesManager) SavePHQ9Assessment(patientID int64, result *PHQ9Result) error {
	// Insert into clinical_assessments table
	query := `
		INSERT INTO clinical_assessments (
			patient_id, assessment_type, score, severity_level,
			assessed_at, created_at
		) VALUES ($1, 'PHQ-9', $2, $3, $4, NOW())
		RETURNING id
	`

	var assessmentID int64
	err := m.db.Conn.QueryRow(
		query,
		patientID,
		result.TotalScore,
		result.SeverityLevel,
		result.AssessedAt,
	).Scan(&assessmentID)

	if err != nil {
		return err
	}

	// Save individual responses
	for _, resp := range result.Responses {
		queryResp := `
			INSERT INTO assessment_responses (
				assessment_id, question_number, response_value, created_at
			) VALUES ($1, $2, $3, NOW())
		`

		_, err = m.db.Conn.Exec(queryResp, assessmentID, resp.Question, resp.Score)
		if err != nil {
			log.Printf("⚠️ Erro ao salvar resposta da questão %d: %v", resp.Question, err)
		}
	}

	// Create alert if critical
	if result.SuicideRisk || result.TotalScore >= 15 {
		m.createCriticalAlert(patientID, "PHQ-9", result.TotalScore, result.SuicideRisk)
	}

	log.Printf("✅ [SCALES] PHQ-9 assessment saved for patient %d (score: %d, severity: %s)", patientID, result.TotalScore, result.SeverityLevel)
	return nil
}

// ====================================================================
// GAD-7 (Generalized Anxiety Disorder-7) - ANSIEDADE
// ====================================================================

type GAD7Question struct {
	Number int
	Text   string
}

func GetGAD7Questions() []GAD7Question {
	return []GAD7Question{
		{1, "Sentir-se nervoso(a), ansioso(a) ou muito tenso(a)"},
		{2, "Não ser capaz de impedir ou de controlar as preocupações"},
		{3, "Preocupar-se muito com diversas coisas"},
		{4, "Dificuldade para relaxar"},
		{5, "Ficar tão agitado(a) que se torna difícil permanecer sentado(a)"},
		{6, "Ficar facilmente aborrecido(a) ou irritado(a)"},
		{7, "Sentir medo como se algo horrível fosse acontecer"},
	}
}

type GAD7Response struct {
	Question int
	Score    int // 0-3 (same scale as PHQ-9)
}

type GAD7Result struct {
	TotalScore      int
	SeverityLevel   string // "minimal", "mild", "moderate", "severe"
	Responses       []GAD7Response
	Recommendations []string
	AssessedAt      time.Time
}

func (m *ClinicalScalesManager) CalculateGAD7Score(responses []GAD7Response) *GAD7Result {
	totalScore := 0
	for _, resp := range responses {
		totalScore += resp.Score
	}

	severity := m.getGAD7Severity(totalScore)
	recommendations := m.getGAD7Recommendations(totalScore)

	return &GAD7Result{
		TotalScore:      totalScore,
		SeverityLevel:   severity,
		Responses:       responses,
		Recommendations: recommendations,
		AssessedAt:      time.Now(),
	}
}

func (m *ClinicalScalesManager) getGAD7Severity(score int) string {
	if score >= 15 {
		return "severe"
	} else if score >= 10 {
		return "moderate"
	} else if score >= 5 {
		return "mild"
	}
	return "minimal"
}

func (m *ClinicalScalesManager) getGAD7Recommendations(score int) []string {
	recommendations := []string{}

	if score >= 15 {
		recommendations = append(recommendations, "Ansiedade severa - consulta psiquiátrica urgente recomendada")
		recommendations = append(recommendations, "Tratamento medicamentoso e psicoterapia podem ser necessários")
	} else if score >= 10 {
		recommendations = append(recommendations, "Ansiedade moderada - consulta com profissional de saúde mental recomendada")
		recommendations = append(recommendations, "Técnicas de relaxamento e terapia cognitivo-comportamental podem ajudar")
	} else if score >= 5 {
		recommendations = append(recommendations, "Ansiedade leve - exercícios de respiração e mindfulness podem ajudar")
	}

	return recommendations
}

func (m *ClinicalScalesManager) SaveGAD7Assessment(patientID int64, result *GAD7Result) error {
	query := `
		INSERT INTO clinical_assessments (
			patient_id, assessment_type, score, severity_level,
			assessed_at, created_at
		) VALUES ($1, 'GAD-7', $2, $3, $4, NOW())
		RETURNING id
	`

	var assessmentID int64
	err := m.db.Conn.QueryRow(
		query,
		patientID,
		result.TotalScore,
		result.SeverityLevel,
		result.AssessedAt,
	).Scan(&assessmentID)

	if err != nil {
		return err
	}

	// Save responses
	for _, resp := range result.Responses {
		queryResp := `
			INSERT INTO assessment_responses (
				assessment_id, question_number, response_value, created_at
			) VALUES ($1, $2, $3, NOW())
		`
		_, err = m.db.Conn.Exec(queryResp, assessmentID, resp.Question, resp.Score)
		if err != nil {
			log.Printf("⚠️ Erro ao salvar resposta: %v", err)
		}
	}

	if result.TotalScore >= 15 {
		m.createCriticalAlert(patientID, "GAD-7", result.TotalScore, false)
	}

	log.Printf("✅ [SCALES] GAD-7 assessment saved for patient %d (score: %d)", patientID, result.TotalScore)
	return nil
}

// ====================================================================
// C-SSRS (Columbia Suicide Severity Rating Scale) - RISCO SUICIDA
// ====================================================================

type CSSRSQuestion struct {
	Number   int
	Category string
	Text     string
}

func GetCSSRSQuestions() []CSSRSQuestion {
	return []CSSRSQuestion{
		{1, "ideation", "Você desejou estar morto(a) ou poder dormir e não acordar?"},
		{2, "ideation", "Você realmente teve pensamentos sobre se matar?"},
		{3, "ideation", "Você pensou em como poderia fazer isso?"},
		{4, "ideation", "Você teve intenção de seguir adiante com esses pensamentos?"},
		{5, "behavior", "Você já fez alguma coisa, começou a fazer alguma coisa ou se preparou para fazer algo para terminar com sua vida?"},
		{6, "behavior", "Alguma vez você já tentou se matar?"},
	}
}

type CSSRSResponse struct {
	Question int
	Answer   bool // Yes/No
}

type CSSRSResult struct {
	IdeationLevel   int    // 0-5
	BehaviorPresent bool
	RiskLevel       string // "none", "low", "moderate", "high", "critical"
	Responses       []CSSRSResponse
	Interventions   []string
	AssessedAt      time.Time
}

func (m *ClinicalScalesManager) CalculateCSSRSScore(responses []CSSRSResponse) *CSSRSResult {
	ideationLevel := 0
	behaviorPresent := false

	for _, resp := range responses {
		if resp.Answer {
			if resp.Question <= 5 {
				// Questions 1-5 are about ideation
				if resp.Question > ideationLevel {
					ideationLevel = resp.Question
				}
			} else {
				// Question 6 is about behavior
				behaviorPresent = true
			}
		}
	}

	riskLevel := m.getCSSRSRiskLevel(ideationLevel, behaviorPresent)
	interventions := m.getCSSRSInterventions(riskLevel)

	return &CSSRSResult{
		IdeationLevel:   ideationLevel,
		BehaviorPresent: behaviorPresent,
		RiskLevel:       riskLevel,
		Responses:       responses,
		Interventions:   interventions,
		AssessedAt:      time.Now(),
	}
}

func (m *ClinicalScalesManager) getCSSRSRiskLevel(ideation int, behavior bool) string {
	if behavior {
		return "critical" // Any suicidal behavior is critical
	}

	if ideation >= 4 {
		return "high" // Intent or plan
	} else if ideation >= 2 {
		return "moderate" // Active thoughts
	} else if ideation == 1 {
		return "low" // Passive ideation
	}

	return "none"
}

func (m *ClinicalScalesManager) getCSSRSInterventions(riskLevel string) []string {
	interventions := []string{}

	switch riskLevel {
	case "critical":
		interventions = append(interventions, "🚨 CRISE SUICIDA - INTERVENÇÃO IMEDIATA")
		interventions = append(interventions, "1. NÃO deixe o paciente sozinho")
		interventions = append(interventions, "2. Ligue SAMU 192 ou vá à emergência psiquiátrica")
		interventions = append(interventions, "3. Remova meios letais (medicamentos, armas)")
		interventions = append(interventions, "4. CVV: 188 (24 horas)")

	case "high":
		interventions = append(interventions, "⚠️ RISCO ALTO - Avaliação psiquiátrica urgente")
		interventions = append(interventions, "Contato com psiquiatra nas próximas 24h")
		interventions = append(interventions, "Ativar rede de apoio (família, amigos)")
		interventions = append(interventions, "Considerar hospitalização")

	case "moderate":
		interventions = append(interventions, "Consulta psiquiátrica em 48-72h")
		interventions = append(interventions, "Aumentar frequência de monitoramento")
		interventions = append(interventions, "Ativar plano de segurança")

	case "low":
		interventions = append(interventions, "Monitoramento contínuo")
		interventions = append(interventions, "Conversar sobre fatores de proteção")
		interventions = append(interventions, "Considerar psicoterapia")
	}

	return interventions
}

func (m *ClinicalScalesManager) SaveCSSRSAssessment(patientID int64, result *CSSRSResult) error {
	query := `
		INSERT INTO clinical_assessments (
			patient_id, assessment_type, score, severity_level,
			assessed_at, created_at
		) VALUES ($1, 'C-SSRS', $2, $3, $4, NOW())
		RETURNING id
	`

	var assessmentID int64
	err := m.db.Conn.QueryRow(
		query,
		patientID,
		result.IdeationLevel,
		result.RiskLevel,
		result.AssessedAt,
	).Scan(&assessmentID)

	if err != nil {
		return err
	}

	// Save responses
	for _, resp := range result.Responses {
		queryResp := `
			INSERT INTO assessment_responses (
				assessment_id, question_number, response_value, created_at
			) VALUES ($1, $2, $3, NOW())
		`
		value := 0
		if resp.Answer {
			value = 1
		}
		_, err = m.db.Conn.Exec(queryResp, assessmentID, resp.Question, value)
		if err != nil {
			log.Printf("⚠️ Erro ao salvar resposta: %v", err)
		}
	}

	// ALWAYS create alert for any C-SSRS result except "none"
	if result.RiskLevel != "none" {
		m.createCriticalAlert(patientID, "C-SSRS", result.IdeationLevel, true)
	}

	log.Printf("🚨 [SCALES] C-SSRS assessment saved for patient %d (risk: %s)", patientID, result.RiskLevel)
	return nil
}

// createCriticalAlert creates a critical alert in the database
func (m *ClinicalScalesManager) createCriticalAlert(patientID int64, scaleType string, score int, isSuicide bool) {
	message := fmt.Sprintf("%s score: %d", scaleType, score)
	if isSuicide {
		message = fmt.Sprintf("RISCO SUICIDA - %s", message)
	}

	query := `
		INSERT INTO clinical_alerts (
			patient_id, alert_type, severity, message, score, created_at
		) VALUES ($1, $2, 'critical', $3, $4, NOW())
	`

	_, err := m.db.Conn.Exec(query, patientID, scaleType, message, score)
	if err != nil {
		log.Printf("❌ Erro ao criar alerta crítico: %v", err)
	}
}
