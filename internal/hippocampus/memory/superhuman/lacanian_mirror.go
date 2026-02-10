package superhuman

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// LacanianMirror generates objective reflections for the patient
// PRINCIPLE: EVA does not interpret. EVA reflects.
// All outputs are data-driven questions that let the patient discover insights.
type LacanianMirror struct {
	db *sql.DB
}

// NewLacanianMirror creates a new mirror service
func NewLacanianMirror(db *sql.DB) *LacanianMirror {
	return &LacanianMirror{db: db}
}

// ReflectPattern reflects a behavioral pattern back to the patient
func (m *LacanianMirror) ReflectPattern(pattern *BehavioralPattern) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Padrao observado: %s", pattern.PatternName),
		fmt.Sprintf("Ocorrencias: %d vezes", pattern.OccurrenceCount),
		fmt.Sprintf("Probabilidade: %.0f%%", pattern.Probability*100),
	}

	if pattern.FirstObserved != (time.Time{}) {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Primeira vez: %s", pattern.FirstObserved.Format("02/01/2006")))
	}

	return &MirrorOutput{
		Type:       "pattern",
		DataPoints: dataPoints,
		Frequency:  &pattern.OccurrenceCount,
		Question:   "Voce havia percebido esse padrao? O que voce acha que o desencadeia?",
		RawData: map[string]interface{}{
			"pattern_type":     pattern.PatternType,
			"trigger":          pattern.TriggerCondition,
			"typical_response": pattern.TypicalResponse,
		},
	}
}

// ReflectIntention reflects unrealized intentions
func (m *LacanianMirror) ReflectIntention(intention *PatientIntention) *MirrorOutput {
	daysSinceFirst := int(time.Since(intention.FirstDeclared).Hours() / 24)

	dataPoints := []string{
		fmt.Sprintf("Voce disse: \"%s\"", intention.IntentionVerbatim),
		fmt.Sprintf("Vezes que mencionou: %d", intention.DeclarationCount),
		fmt.Sprintf("Primeira vez: %s (%d dias atras)",
			intention.FirstDeclared.Format("02/01/2006"), daysSinceFirst),
	}

	if intention.StatedBlocker != "" {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Voce mencionou como obstáculo: \"%s\"", intention.StatedBlocker))
	}

	question := "O que voce acha que acontece entre a intencao e a acao?"
	if intention.RelatedPerson != "" {
		question = fmt.Sprintf("O que voce acha que acontece entre voce e %s que dificulta isso?",
			intention.RelatedPerson)
	}

	return &MirrorOutput{
		Type:       "intention",
		DataPoints: dataPoints,
		Frequency:  &intention.DeclarationCount,
		Question:   question,
		RawData: map[string]interface{}{
			"category":       intention.Category,
			"related_person": intention.RelatedPerson,
			"status":         intention.Status,
			"days_pending":   daysSinceFirst,
		},
	}
}

// ReflectCounterfactual reflects "what if" ruminations
func (m *LacanianMirror) ReflectCounterfactual(cf *PatientCounterfactual) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce mencionou: \"%s\"", cf.Verbatim),
		fmt.Sprintf("Vezes que voltou a esse tema: %d", cf.MentionCount),
	}

	if cf.LifePeriod != "" {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Periodo da vida: %s", cf.LifePeriod))
	}

	if cf.VoiceTremorDetected {
		dataPoints = append(dataPoints,
			"Sua voz apresentou tremor ao falar disso")
	}

	if len(cf.CorrelatedPersons) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Aparece junto com mencoes a: %s", strings.Join(cf.CorrelatedPersons, ", ")))
	}

	return &MirrorOutput{
		Type:       "counterfactual",
		DataPoints: dataPoints,
		Frequency:  &cf.MentionCount,
		Question:   "O que esse 'e se' representa para voce hoje? O que voce sente quando pensa nisso?",
		RawData: map[string]interface{}{
			"theme":              cf.Theme,
			"emotional_valence":  cf.AvgEmotionalValence,
			"correlated_topics":  cf.CorrelatedTopics,
			"correlated_persons": cf.CorrelatedPersons,
		},
	}
}

// ReflectMetaphor reflects metaphorical language
func (m *LacanianMirror) ReflectMetaphor(metaphor *PatientMetaphor) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce usa a expressao: \"%s\"", metaphor.Metaphor),
		fmt.Sprintf("Vezes que usou: %d", metaphor.UsageCount),
	}

	if len(metaphor.CorrelatedTopics) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Aparece quando fala de: %s", strings.Join(metaphor.CorrelatedTopics, ", ")))
	}

	if len(metaphor.CorrelatedPersons) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Conectado a: %s", strings.Join(metaphor.CorrelatedPersons, ", ")))
	}

	return &MirrorOutput{
		Type:       "metaphor",
		DataPoints: dataPoints,
		Frequency:  &metaphor.UsageCount,
		Question:   "O que essa expressao significa para voce? O que voce esta querendo dizer quando usa?",
		RawData: map[string]interface{}{
			"metaphor_type":       metaphor.MetaphorType,
			"correlated_topics":   metaphor.CorrelatedTopics,
			"correlated_emotions": metaphor.CorrelatedEmotions,
		},
	}
}

// ReflectSignifier reflects recurring words
func (m *LacanianMirror) ReflectSignifier(sig *MasterSignifier) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce usou a palavra '%s' %d vezes", sig.Signifier, sig.TotalCount),
		fmt.Sprintf("Primeira vez: %s", sig.FirstSeen.Format("02/01/2006")),
		fmt.Sprintf("Ultima vez: %s", sig.LastSeen.Format("02/01/2006")),
	}

	if len(sig.CoOccurringSignifiers) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Aparece junto com: %s", strings.Join(sig.CoOccurringSignifiers, ", ")))
	}

	return &MirrorOutput{
		Type:       "signifier",
		DataPoints: dataPoints,
		Frequency:  &sig.TotalCount,
		Question:   "Voce havia notado que usa tanto essa palavra? O que ela significa para voce?",
		RawData: map[string]interface{}{
			"context_type":       sig.ContextType,
			"emotional_valence":  sig.AvgEmotionalValence,
			"frequency_by_month": sig.FrequencyByPeriod,
		},
	}
}

// ReflectSomaticCorrelation reflects body-speech correlations
func (m *LacanianMirror) ReflectSomaticCorrelation(corr *SomaticCorrelation) *MirrorOutput {
	somaticTypesPT := map[string]string{
		"blood_glucose":        "glicemia",
		"blood_pressure":       "pressao arterial",
		"heart_rate":           "frequencia cardiaca",
		"sleep_quality":        "qualidade do sono",
		"pain_level":           "nivel de dor",
		"medication_adherence": "adesao a medicacao",
	}

	somaticPT := somaticTypesPT[corr.SomaticType]
	if somaticPT == "" {
		somaticPT = corr.SomaticType
	}

	dataPoints := []string{
		fmt.Sprintf("Quando sua %s esta %s", somaticPT, corr.ConditionRange),
		fmt.Sprintf("Voce fala mais sobre: %s", corr.CorrelatedTopic),
		fmt.Sprintf("Correlacao: %.0f%% das vezes", corr.CorrelationStrength*100),
		fmt.Sprintf("Baseado em: %d observacoes", corr.ObservationCount),
	}

	return &MirrorOutput{
		Type:       "somatic_correlation",
		DataPoints: dataPoints,
		Frequency:  &corr.ObservationCount,
		Question:   "Voce percebe alguma conexao entre seu corpo e esse assunto?",
		RawData: map[string]interface{}{
			"somatic_type":         corr.SomaticType,
			"condition":            corr.ConditionRange,
			"topic":                corr.CorrelatedTopic,
			"correlation_strength": corr.CorrelationStrength,
		},
	}
}

// ReflectFamilyPattern reflects transgenerational patterns
func (m *LacanianMirror) ReflectFamilyPattern(fp *FamilyPattern) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce disse: \"%s\"", fp.PatternVerbatim),
		fmt.Sprintf("Vezes que mencionou: %d", fp.MentionCount),
	}

	if len(fp.GenerationsMentioned) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Geracoes envolvidas: %s", strings.Join(fp.GenerationsMentioned, ", ")))
	}

	return &MirrorOutput{
		Type:       "family_pattern",
		DataPoints: dataPoints,
		Frequency:  &fp.MentionCount,
		Question:   "Voce ve esse padrao se repetindo? O que voce acha que isso significa para sua familia?",
		RawData: map[string]interface{}{
			"pattern_type":         fp.PatternType,
			"generations":          fp.GenerationsMentioned,
			"emotional_valence":    fp.AvgEmotionalValence,
		},
	}
}

// ReflectWorldPerson reflects how patient talks about someone
func (m *LacanianMirror) ReflectWorldPerson(ctx context.Context, person *WorldPerson) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce mencionou %s %d vezes", person.PersonName, person.MentionCount),
	}

	if person.Role != "" {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Papel: %s", person.Role))
	}

	// Describe emotional trend
	if person.EmotionalValence > 0.3 {
		dataPoints = append(dataPoints, "Tom geral das mencoes: positivo")
	} else if person.EmotionalValence < -0.3 {
		dataPoints = append(dataPoints, "Tom geral das mencoes: negativo")
	} else {
		dataPoints = append(dataPoints, "Tom geral das mencoes: neutro/misto")
	}

	if len(person.AssociatedTopics) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Temas associados: %s", strings.Join(person.AssociatedTopics, ", ")))
	}

	if person.CurrentStatus != "" {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Status atual que voce descreve: %s", person.CurrentStatus))
	}

	return &MirrorOutput{
		Type:       "relationship",
		DataPoints: dataPoints,
		Frequency:  &person.MentionCount,
		Question:   fmt.Sprintf("O que %s representa para voce hoje?", person.PersonName),
		RawData: map[string]interface{}{
			"person_name":       person.PersonName,
			"role":              person.Role,
			"emotional_valence": person.EmotionalValence,
			"associated_topics": person.AssociatedTopics,
			"timeline":          person.RelationshipTimeline,
		},
	}
}

// GenerateRiskAlert generates alert for caregivers (not for patient)
func (m *LacanianMirror) GenerateRiskAlert(risk *RiskScore) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Nivel de alerta: %s", risk.AlertLevel),
		fmt.Sprintf("Score geral: %.2f", risk.OverallRiskScore),
	}

	if risk.RiskSuicidal30D > 0.3 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Risco suicida 30d: %.2f", risk.RiskSuicidal30D))
	}
	if risk.RiskDepressionSevere > 0.5 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Risco depressao grave: %.2f", risk.RiskDepressionSevere))
	}
	if risk.RiskSocialIsolation > 0.5 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Risco isolamento: %.2f", risk.RiskSocialIsolation))
	}

	if len(risk.ActiveMarkers) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Marcadores ativos: %s", strings.Join(risk.ActiveMarkers, ", ")))
	}

	return &MirrorOutput{
		Type:       "risk_alert",
		DataPoints: dataPoints,
		Question:   "", // No question for alerts
		RawData: map[string]interface{}{
			"alert_level":          risk.AlertLevel,
			"overall_score":        risk.OverallRiskScore,
			"risk_suicidal":        risk.RiskSuicidal30D,
			"risk_depression":      risk.RiskDepressionSevere,
			"risk_hospitalization": risk.RiskHospitalization90D,
			"risk_isolation":       risk.RiskSocialIsolation,
			"recommended_action":   risk.RecommendedAction,
		},
	}
}

// FormatForPatient formats a mirror output as text for the patient
func (m *LacanianMirror) FormatForPatient(mo *MirrorOutput) string {
	if mo == nil {
		return ""
	}

	var sb strings.Builder

	for _, dp := range mo.DataPoints {
		sb.WriteString("- ")
		sb.WriteString(dp)
		sb.WriteString("\n")
	}

	if mo.Question != "" {
		sb.WriteString("\n")
		sb.WriteString(mo.Question)
	}

	return sb.String()
}

// FormatForCaregiver formats risk alerts for caregivers
func (m *LacanianMirror) FormatForCaregiver(mo *MirrorOutput) string {
	if mo == nil || mo.Type != "risk_alert" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[ALERTA DE RISCO]\n\n")

	for _, dp := range mo.DataPoints {
		sb.WriteString("- ")
		sb.WriteString(dp)
		sb.WriteString("\n")
	}

	if action, ok := mo.RawData["recommended_action"].(string); ok && action != "" {
		sb.WriteString("\nAcao recomendada: ")
		sb.WriteString(action)
	}

	return sb.String()
}
