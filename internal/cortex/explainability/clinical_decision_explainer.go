// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package explainability

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// ClinicalDecisionExplainer explica decisões clínicas usando feature importance
type ClinicalDecisionExplainer struct {
	db *sql.DB
}

// ClinicalPrediction representa uma predição clínica
type ClinicalPrediction struct {
	PatientID           int64
	DecisionType        string  // crisis_prediction, depression_alert, etc
	PredictionScore     float64 // 0-1
	PredictionTimeframe string  // '24-48h', '7-14 days'
	Severity            string  // low, medium, high, critical
	Features            map[string]Feature
	ModelVersion        string
}

// Feature representa uma feature usada na predição
type Feature struct {
	Name          string
	CurrentValue  float64
	BaselineValue float64
	Category      string // primary, secondary, tertiary
	Status        string // normal, warning, concerning, critical
	Details       map[string]interface{}
}

// Explanation representa explicação completa
type Explanation struct {
	ID                  string
	PatientID           int64
	DecisionType        string
	PredictionScore     float64
	Severity            string
	Timeframe           string
	FeatureContributions map[string]float64 // SHAP-like values
	PrimaryFactors      []ExplanationFactor
	SecondaryFactors    []ExplanationFactor
	Recommendations     []Recommendation
	SupportingEvidence  map[string]interface{}
	ExplanationText     string
	CreatedAt           time.Time
}

// ExplanationFactor fator que contribuiu para decisão
type ExplanationFactor struct {
	Factor              string
	Contribution        float64 // 0-1 (percentage)
	Status              string
	Details             string
	BaselineComparison  string
	HumanReadable       string
}

// Recommendation recomendação clínica
type Recommendation struct {
	Urgency     string // low, medium, high, critical
	Action      string
	Rationale   string
	Timeframe   string
}

// NewClinicalDecisionExplainer cria novo explainer
func NewClinicalDecisionExplainer(db *sql.DB) *ClinicalDecisionExplainer {
	return &ClinicalDecisionExplainer{db: db}
}

// ExplainDecision gera explicação completa para uma decisão clínica
func (cde *ClinicalDecisionExplainer) ExplainDecision(prediction ClinicalPrediction) (*Explanation, error) {
	log.Printf("🔍 [EXPLAINER] Gerando explicação para decisão: %s (paciente %d)", prediction.DecisionType, prediction.PatientID)

	// 1. Calcular contribuições (SHAP-like)
	contributions := cde.calculateContributions(prediction.Features, prediction.PredictionScore)

	// 2. Classificar features por importância
	primaryFactors, secondaryFactors := cde.classifyFactors(prediction.Features, contributions)

	// 3. Gerar recomendações
	recommendations := cde.generateRecommendations(prediction, primaryFactors)

	// 4. Coletar evidências de suporte
	evidence := cde.collectSupportingEvidence(prediction.PatientID, prediction.Features)

	// 5. Gerar explicação em linguagem natural
	explanationText := cde.generateNaturalLanguageExplanation(prediction, primaryFactors, secondaryFactors)

	// 6. Criar objeto de explicação
	explanation := &Explanation{
		PatientID:            prediction.PatientID,
		DecisionType:         prediction.DecisionType,
		PredictionScore:      prediction.PredictionScore,
		Severity:             prediction.Severity,
		Timeframe:            prediction.PredictionTimeframe,
		FeatureContributions: contributions,
		PrimaryFactors:       primaryFactors,
		SecondaryFactors:     secondaryFactors,
		Recommendations:      recommendations,
		SupportingEvidence:   evidence,
		ExplanationText:      explanationText,
		CreatedAt:            time.Now(),
	}

	// 7. Salvar no banco
	err := cde.saveExplanation(explanation, prediction)
	if err != nil {
		return nil, fmt.Errorf("erro ao salvar explicação: %w", err)
	}

	log.Printf("✅ [EXPLAINER] Explicação gerada com sucesso: ID=%s", explanation.ID)

	return explanation, nil
}

// calculateContributions calcula contribuição de cada feature (SHAP-like)
func (cde *ClinicalDecisionExplainer) calculateContributions(features map[string]Feature, predictionScore float64) map[string]float64 {
	contributions := make(map[string]float64)

	// Heurística simplificada: quanto a feature desvia da baseline, maior a contribuição
	totalDeviation := 0.0
	deviations := make(map[string]float64)

	for name, feature := range features {
		// Calcular desvio normalizado
		deviation := 0.0
		if feature.BaselineValue != 0 {
			deviation = (feature.CurrentValue - feature.BaselineValue) / feature.BaselineValue
		} else {
			deviation = feature.CurrentValue
		}

		// Converter para valor absoluto e aplicar pesos por tipo
		absDeviation := deviation
		if absDeviation < 0 {
			absDeviation = -absDeviation
		}

		// Pesos por categoria de feature
		weight := 1.0
		switch {
		case strings.Contains(strings.ToLower(name), "medication"):
			weight = 1.5 // Medicação é muito importante
		case strings.Contains(strings.ToLower(name), "voice"):
			weight = 1.3 // Voz é bom indicador
		case strings.Contains(strings.ToLower(name), "phq") || strings.Contains(strings.ToLower(name), "gad"):
			weight = 1.2 // Escalas clínicas
		case strings.Contains(strings.ToLower(name), "sleep"):
			weight = 1.1 // Sono importante
		}

		absDeviation *= weight
		deviations[name] = absDeviation
		totalDeviation += absDeviation
	}

	// Normalizar contribuições (soma = predictionScore)
	if totalDeviation > 0 {
		for name, deviation := range deviations {
			contributions[name] = (deviation / totalDeviation) * predictionScore
		}
	}

	return contributions
}

// classifyFactors classifica features em primários e secundários
func (cde *ClinicalDecisionExplainer) classifyFactors(features map[string]Feature, contributions map[string]float64) ([]ExplanationFactor, []ExplanationFactor) {
	// Criar slice de fatores com contribuições
	type factorWithContrib struct {
		name         string
		feature      Feature
		contribution float64
	}

	var factors []factorWithContrib
	for name, feature := range features {
		factors = append(factors, factorWithContrib{
			name:         name,
			feature:      feature,
			contribution: contributions[name],
		})
	}

	// Ordenar por contribuição
	sort.Slice(factors, func(i, j int) bool {
		return factors[i].contribution > factors[j].contribution
	})

	// Top 3 = primários, resto = secundários
	var primary, secondary []ExplanationFactor

	for i, f := range factors {
		factor := ExplanationFactor{
			Factor:       f.name,
			Contribution: f.contribution,
			Status:       f.feature.Status,
			Details:      cde.formatDetails(f.feature),
			BaselineComparison: cde.formatBaselineComparison(f.feature),
			HumanReadable:      cde.generateHumanReadableExplanation(f.name, f.feature),
		}

		if i < 3 {
			primary = append(primary, factor)
		} else {
			secondary = append(secondary, factor)
		}
	}

	return primary, secondary
}

// generateRecommendations gera recomendações baseadas nos fatores
func (cde *ClinicalDecisionExplainer) generateRecommendations(prediction ClinicalPrediction, primaryFactors []ExplanationFactor) []Recommendation {
	var recommendations []Recommendation

	// Recomendações baseadas na severidade
	if prediction.Severity == "critical" || prediction.Severity == "high" {
		recommendations = append(recommendations, Recommendation{
			Urgency:   "high",
			Action:    "Contato telefônico urgente nas próximas 24 horas",
			Rationale: fmt.Sprintf("Predição de %s com probabilidade %.0f%%", prediction.DecisionType, prediction.PredictionScore*100),
			Timeframe: "24h",
		})
	}

	// Recomendações específicas por fator principal
	for _, factor := range primaryFactors {
		if strings.Contains(strings.ToLower(factor.Factor), "medication") {
			if factor.Status == "critical" || factor.Status == "concerning" {
				recommendations = append(recommendations, Recommendation{
					Urgency:   "high",
					Action:    "Investigar barreiras à adesão medicamentosa",
					Rationale: "Adesão medicamentosa abaixo do esperado é principal fator de risco",
					Timeframe: "48h",
				})
			}
		}

		if strings.Contains(strings.ToLower(factor.Factor), "voice") {
			recommendations = append(recommendations, Recommendation{
				Urgency:   "medium",
				Action:    "Análise de áudio detalhada com especialista",
				Rationale: "Biomarcadores vocais indicam mudança significativa no estado mental",
				Timeframe: "3-5 dias",
			})
		}

		if strings.Contains(strings.ToLower(factor.Factor), "sleep") {
			recommendations = append(recommendations, Recommendation{
				Urgency:   "medium",
				Action:    "Protocolo de higiene do sono + avaliação de insônia",
				Rationale: "Qualidade de sono deteriorada contribui para risco",
				Timeframe: "1 semana",
			})
		}

		if strings.Contains(strings.ToLower(factor.Factor), "phq9") || strings.Contains(strings.ToLower(factor.Factor), "depression") {
			recommendations = append(recommendations, Recommendation{
				Urgency:   "high",
				Action:    "Considerar ajuste medicamentoso ou psicoterapia",
				Rationale: "Score de depressão elevado ou em piora",
				Timeframe: "1 semana",
			})
		}
	}

	// Limitar a 5 recomendações mais importantes
	if len(recommendations) > 5 {
		recommendations = recommendations[:5]
	}

	return recommendations
}

// collectSupportingEvidence coleta evidências de suporte
func (cde *ClinicalDecisionExplainer) collectSupportingEvidence(patientID int64, features map[string]Feature) map[string]interface{} {
	evidence := make(map[string]interface{})

	// Buscar trechos de conversa recentes
	conversations := cde.getRecentConversations(patientID, 3)
	if len(conversations) > 0 {
		evidence["conversation_excerpts"] = conversations
	}

	// Buscar samples de áudio (se houver voice features)
	for name := range features {
		if strings.Contains(strings.ToLower(name), "voice") {
			audioSamples := cde.getRecentAudioSamples(patientID, 2)
			if len(audioSamples) > 0 {
				evidence["audio_samples"] = audioSamples
			}
			break
		}
	}

	// Adicionar gráficos de tendências
	evidence["graph_data"] = map[string]interface{}{
		"mood_trend_7d":           cde.getMoodTrend(patientID, 7),
		"medication_adherence_30d": cde.getMedicationAdherenceTrend(patientID, 30),
	}

	return evidence
}

// generateNaturalLanguageExplanation gera explicação em português
func (cde *ClinicalDecisionExplainer) generateNaturalLanguageExplanation(
	prediction ClinicalPrediction,
	primaryFactors []ExplanationFactor,
	secondaryFactors []ExplanationFactor,
) string {
	var sb strings.Builder

	// Título
	sb.WriteString(fmt.Sprintf("🚨 ALERTA: %s\n\n", cde.translateDecisionType(prediction.DecisionType)))

	// Predição
	sb.WriteString(fmt.Sprintf("Probabilidade: %.0f%% (%s)\n", prediction.PredictionScore*100, cde.translateSeverity(prediction.Severity)))
	sb.WriteString(fmt.Sprintf("Janela temporal: %s\n\n", prediction.PredictionTimeframe))

	// Fatores principais
	sb.WriteString("📊 FATORES PRINCIPAIS (por ordem de importância):\n\n")
	for i, factor := range primaryFactors {
		sb.WriteString(fmt.Sprintf("%d. %s (contribuição: %.0f%%)\n", i+1, cde.formatFactorName(factor.Factor), factor.Contribution*100))
		sb.WriteString(fmt.Sprintf("   Status: %s\n", cde.translateStatus(factor.Status)))
		sb.WriteString(fmt.Sprintf("   %s\n", factor.HumanReadable))
		if factor.BaselineComparison != "" {
			sb.WriteString(fmt.Sprintf("   Comparação: %s\n", factor.BaselineComparison))
		}
		sb.WriteString("\n")
	}

	// Fatores secundários (se houver)
	if len(secondaryFactors) > 0 {
		sb.WriteString("📋 FATORES SECUNDÁRIOS:\n\n")
		for _, factor := range secondaryFactors {
			sb.WriteString(fmt.Sprintf("• %s: %s\n", cde.formatFactorName(factor.Factor), factor.HumanReadable))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// saveExplanation salva explicação no banco
func (cde *ClinicalDecisionExplainer) saveExplanation(explanation *Explanation, prediction ClinicalPrediction) error {
	// Converter maps para JSON
	contributionsJSON, _ := json.Marshal(explanation.FeatureContributions)
	featuresSnapshotJSON, _ := json.Marshal(prediction.Features)

	// Estruturar explicação
	primaryFactorsJSON, _ := json.Marshal(explanation.PrimaryFactors)
	secondaryFactorsJSON, _ := json.Marshal(explanation.SecondaryFactors)
	explanationStructured := map[string]interface{}{
		"primary_factors":   json.RawMessage(primaryFactorsJSON),
		"secondary_factors": json.RawMessage(secondaryFactorsJSON),
	}
	explanationStructuredJSON, _ := json.Marshal(explanationStructured)

	recommendationsJSON, _ := json.Marshal(explanation.Recommendations)
	evidenceJSON, _ := json.Marshal(explanation.SupportingEvidence)

	// Insert
	query := `
		INSERT INTO clinical_decision_explanations (
			patient_id, decision_type, prediction_score, prediction_timeframe, severity,
			feature_contributions, features_snapshot, explanation_text, explanation_structured,
			recommendations, supporting_evidence, model_version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`

	var explanationID string
	err := cde.db.QueryRow(
		query,
		explanation.PatientID,
		explanation.DecisionType,
		explanation.PredictionScore,
		explanation.Timeframe,
		explanation.Severity,
		contributionsJSON,
		featuresSnapshotJSON,
		explanation.ExplanationText,
		explanationStructuredJSON,
		recommendationsJSON,
		evidenceJSON,
		prediction.ModelVersion,
	).Scan(&explanationID)

	if err != nil {
		return err
	}

	explanation.ID = explanationID

	// Inserir fatores individuais
	for _, factor := range append(explanation.PrimaryFactors, explanation.SecondaryFactors...) {
		err = cde.saveDecisionFactor(explanationID, factor)
		if err != nil {
			log.Printf("⚠️ [EXPLAINER] Erro ao salvar fator: %v", err)
		}
	}

	return nil
}

// saveDecisionFactor salva fator individual
func (cde *ClinicalDecisionExplainer) saveDecisionFactor(explanationID string, factor ExplanationFactor) error {
	detailsJSON, _ := json.Marshal(map[string]string{
		"details":             factor.Details,
		"baseline_comparison": factor.BaselineComparison,
	})

	// Determinar categoria
	category := "secondary"
	if factor.Contribution > 0.25 {
		category = "primary"
	} else if factor.Contribution > 0.10 {
		category = "secondary"
	} else {
		category = "tertiary"
	}

	query := `
		INSERT INTO decision_factors (
			explanation_id, factor_name, factor_category, shap_value, contribution_percentage,
			status, details, human_readable_explanation
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := cde.db.Exec(
		query,
		explanationID,
		factor.Factor,
		category,
		factor.Contribution, // Simplificação: usando contribution como SHAP
		factor.Contribution*100,
		factor.Status,
		detailsJSON,
		factor.HumanReadable,
	)

	return err
}

// Helper functions

func (cde *ClinicalDecisionExplainer) formatDetails(feature Feature) string {
	if feature.Details != nil {
		detailsJSON, _ := json.Marshal(feature.Details)
		return string(detailsJSON)
	}
	return fmt.Sprintf("Valor atual: %.2f", feature.CurrentValue)
}

func (cde *ClinicalDecisionExplainer) formatBaselineComparison(feature Feature) string {
	if feature.BaselineValue == 0 {
		return ""
	}

	change := feature.CurrentValue - feature.BaselineValue
	changePercent := (change / feature.BaselineValue) * 100

	if change > 0 {
		return fmt.Sprintf("↑ %.1f%% acima da baseline (baseline: %.2f)", changePercent, feature.BaselineValue)
	} else if change < 0 {
		return fmt.Sprintf("↓ %.1f%% abaixo da baseline (baseline: %.2f)", -changePercent, feature.BaselineValue)
	}

	return "Sem mudança em relação à baseline"
}

func (cde *ClinicalDecisionExplainer) generateHumanReadableExplanation(name string, feature Feature) string {
	// Gerar explicação humanizada baseada no tipo de feature
	lowerName := strings.ToLower(name)

	if strings.Contains(lowerName, "medication") {
		adherence := feature.CurrentValue * 100
		if adherence < 50 {
			return fmt.Sprintf("Adesão medicamentosa crítica: apenas %.0f%% das doses tomadas", adherence)
		} else if adherence < 70 {
			return fmt.Sprintf("Adesão medicamentosa preocupante: %.0f%% das doses", adherence)
		}
		return fmt.Sprintf("Adesão medicamentosa: %.0f%%", adherence)
	}

	if strings.Contains(lowerName, "voice") || strings.Contains(lowerName, "pitch") {
		return fmt.Sprintf("Biomarcadores vocais alterados (valor: %.2f vs baseline: %.2f)", feature.CurrentValue, feature.BaselineValue)
	}

	if strings.Contains(lowerName, "sleep") {
		hours := feature.CurrentValue
		if hours < 5 {
			return fmt.Sprintf("Sono severamente comprometido: média de %.1f horas/noite", hours)
		} else if hours < 6 {
			return fmt.Sprintf("Qualidade de sono ruim: %.1f horas/noite", hours)
		}
		return fmt.Sprintf("Duração do sono: %.1f horas/noite", hours)
	}

	if strings.Contains(lowerName, "phq9") || strings.Contains(lowerName, "depression") {
		score := feature.CurrentValue
		if score >= 20 {
			return fmt.Sprintf("Depressão severa (PHQ-9: %.0f)", score)
		} else if score >= 15 {
			return fmt.Sprintf("Depressão moderadamente severa (PHQ-9: %.0f)", score)
		} else if score >= 10 {
			return fmt.Sprintf("Depressão moderada (PHQ-9: %.0f)", score)
		}
		return fmt.Sprintf("Score PHQ-9: %.0f", score)
	}

	return fmt.Sprintf("Valor: %.2f (baseline: %.2f)", feature.CurrentValue, feature.BaselineValue)
}

func (cde *ClinicalDecisionExplainer) translateDecisionType(decisionType string) string {
	translations := map[string]string{
		"crisis_prediction":     "Risco de Crise Mental",
		"depression_alert":      "Alerta de Depressão",
		"anxiety_alert":         "Alerta de Ansiedade",
		"medication_alert":      "Alerta de Adesão Medicamentosa",
		"suicide_risk":          "Risco de Suicídio",
		"hospitalization_risk":  "Risco de Hospitalização",
		"fall_risk":             "Risco de Queda",
	}

	if translated, ok := translations[decisionType]; ok {
		return translated
	}
	return decisionType
}

func (cde *ClinicalDecisionExplainer) translateSeverity(severity string) string {
	translations := map[string]string{
		"low":      "baixo",
		"medium":   "médio",
		"high":     "alto",
		"critical": "crítico",
	}

	if translated, ok := translations[severity]; ok {
		return translated
	}
	return severity
}

func (cde *ClinicalDecisionExplainer) translateStatus(status string) string {
	translations := map[string]string{
		"normal":      "Normal",
		"warning":     "⚠️ Atenção",
		"concerning":  "⚠️ Preocupante",
		"critical":    "🔴 Crítico",
	}

	if translated, ok := translations[status]; ok {
		return translated
	}
	return status
}

func (cde *ClinicalDecisionExplainer) formatFactorName(name string) string {
	// Formatar nomes de features para exibição
	formatted := strings.ReplaceAll(name, "_", " ")
	formatted = strings.Title(formatted)
	return formatted
}

// Helper: buscar conversas recentes
func (cde *ClinicalDecisionExplainer) getRecentConversations(patientID int64, limit int) []string {
	query := `
		SELECT CONCAT(DATE_PART('day', created_at), '/', DATE_PART('month', created_at), ' ',
		              DATE_PART('hour', created_at), ':', LPAD(DATE_PART('minute', created_at)::TEXT, 2, '0'),
		              ' - ', SUBSTRING(conversation_text, 1, 100))
		FROM interaction_cognitive_load
		WHERE patient_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := cde.db.Query(query, patientID, limit)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var conversations []string
	for rows.Next() {
		var conv string
		rows.Scan(&conv)
		conversations = append(conversations, conv)
	}

	return conversations
}

// Helper: buscar audio samples
func (cde *ClinicalDecisionExplainer) getRecentAudioSamples(patientID int64, limit int) []string {
	// Placeholder: retornar paths de áudio se existirem
	return []string{
		fmt.Sprintf("s3://eva-audio/patient-%d/recent-1.wav", patientID),
		fmt.Sprintf("s3://eva-audio/patient-%d/recent-2.wav", patientID),
	}
}

// Helper: tendência de humor
func (cde *ClinicalDecisionExplainer) getMoodTrend(patientID int64, days int) []int {
	// Placeholder: retornar array de scores de humor
	// TODO: implementar query real
	return []int{6, 5, 4, 4, 3, 3, 2}
}

// Helper: tendência de adesão medicamentosa
func (cde *ClinicalDecisionExplainer) getMedicationAdherenceTrend(patientID int64, days int) string {
	// Placeholder: retornar URL do gráfico
	return fmt.Sprintf("/api/graphs/medication-adherence/%d?days=%d", patientID, days)
}
