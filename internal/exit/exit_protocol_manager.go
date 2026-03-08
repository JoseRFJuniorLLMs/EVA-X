// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package exit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"eva/internal/brainstem/database"
)

// ============================================================================
// EXIT PROTOCOL MANAGER
// ============================================================================
// Gerencia cuidados paliativos, qualidade de vida e preparação para despedida

type ExitProtocolManager struct {
	db *database.DB
}

func NewExitProtocolManager(db *database.DB) *ExitProtocolManager {
	return &ExitProtocolManager{db: db}
}

// ============================================================================
// LAST WISHES (TESTAMENTO VITAL)
// ============================================================================

type LastWishes struct {
	ID                       string
	PatientID                int64
	ResuscitationPreference  string
	MechanicalVentilation    *bool
	ArtificialNutrition      *bool
	ArtificialHydration      *bool
	PreferredDeathLocation   string
	PainManagementPreference string
	SedationAcceptable       *bool
	ReligiousPreferences     string
	SpiritualPractices       []string
	WhoWantsPresent          []string
	OrganDonationPreference  string
	BurialCremation          string
	PersonalStatement        string
	CompletionPercentage     int
	Completed                bool
}

func (epm *ExitProtocolManager) CreateLastWishes(patientID int64) (*LastWishes, error) {
	log.Printf("📝 [EXIT] Criando Last Wishes para paciente %d", patientID)

	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)

	id, err := epm.db.Insert(ctx, "last_wishes", map[string]interface{}{
		"patient_id":            patientID,
		"completion_percentage": 0,
		"completed":             false,
		"created_at":            now,
		"updated_at":            now,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao criar last wishes: %w", err)
	}

	lw := &LastWishes{
		ID:                   fmt.Sprintf("%d", id),
		PatientID:            patientID,
		CompletionPercentage: 0,
		Completed:            false,
	}

	log.Printf("✅ [EXIT] Last Wishes criado: ID=%s", lw.ID)
	return lw, nil
}

func (epm *ExitProtocolManager) UpdateLastWishes(lastWishesID string, updates map[string]interface{}) error {
	log.Printf("📝 [EXIT] Atualizando Last Wishes %s", lastWishesID)

	ctx := context.Background()

	updates["last_reviewed_at"] = time.Now().Format(time.RFC3339)

	err := epm.db.Update(ctx, "last_wishes",
		map[string]interface{}{"id": lastWishesID},
		updates,
	)
	if err != nil {
		return fmt.Errorf("erro ao atualizar last wishes: %w", err)
	}

	log.Printf("✅ [EXIT] Last Wishes atualizado")
	return nil
}

func (epm *ExitProtocolManager) GetLastWishes(patientID int64) (*LastWishes, error) {
	ctx := context.Background()

	rows, err := epm.db.QueryByLabel(ctx, "last_wishes",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		1,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("last wishes não encontrado para paciente %d", patientID)
	}

	m := rows[0]
	lw := &LastWishes{
		ID:                       fmt.Sprintf("%v", m["id"]),
		PatientID:                database.GetInt64(m, "patient_id"),
		ResuscitationPreference:  database.GetString(m, "resuscitation_preference"),
		PreferredDeathLocation:   database.GetString(m, "preferred_death_location"),
		PainManagementPreference: database.GetString(m, "pain_management_preference"),
		OrganDonationPreference:  database.GetString(m, "organ_donation_preference"),
		BurialCremation:          database.GetString(m, "burial_cremation"),
		PersonalStatement:        database.GetString(m, "personal_statement"),
		CompletionPercentage:     int(database.GetInt64(m, "completion_percentage")),
		Completed:                database.GetBool(m, "completed"),
	}

	return lw, nil
}

// ============================================================================
// QUALITY OF LIFE ASSESSMENTS (WHOQOL-BREF)
// ============================================================================

type QoLAssessment struct {
	ID                        string
	PatientID                 int64
	AssessmentDate            time.Time
	PhysicalDomainScore       float64
	PsychologicalDomainScore  float64
	SocialDomainScore         float64
	EnvironmentalDomainScore  float64
	OverallQoLScore           float64
	OverallQualityOfLife      int
	OverallHealthSatisfaction int
}

// computeDomainScores calculates WHOQOL-BREF domain scores from individual question values.
// Each domain score is the mean of its items, transformed to a 0-100 scale: ((mean - 1) / 4) * 100.
func computeDomainScores(physical [7]int, psychological [6]int, social [3]int, environmental [8]int) (physScore, psychScore, socialScore, envScore, overallScore float64) {
	mean := func(vals []int) float64 {
		if len(vals) == 0 {
			return 0
		}
		sum := 0
		for _, v := range vals {
			sum += v
		}
		return float64(sum) / float64(len(vals))
	}
	transform := func(m float64) float64 {
		return ((m - 1) / 4) * 100
	}

	physScore = transform(mean(physical[:]))
	psychScore = transform(mean(psychological[:]))
	socialScore = transform(mean(social[:]))
	envScore = transform(mean(environmental[:]))
	overallScore = (physScore + psychScore + socialScore + envScore) / 4
	return
}

func (epm *ExitProtocolManager) RecordQoLAssessment(assessment *QoLAssessment) error {
	log.Printf("📊 [EXIT] Registrando avaliação WHOQOL-BREF para paciente %d", assessment.PatientID)

	ctx := context.Background()
	now := time.Now()

	// Default values (3) for all questions as per original code
	physical := [7]int{3, 3, 3, 3, 3, 3, 3}
	psychological := [6]int{3, 3, 3, 3, 3, 3}
	social := [3]int{3, 3, 3}
	environmental := [8]int{3, 3, 3, 3, 3, 3, 3, 3}

	physScore, psychScore, socialScore, envScore, overallScore := computeDomainScores(
		physical, psychological, social, environmental,
	)

	assessment.PhysicalDomainScore = physScore
	assessment.PsychologicalDomainScore = psychScore
	assessment.SocialDomainScore = socialScore
	assessment.EnvironmentalDomainScore = envScore
	assessment.OverallQoLScore = overallScore
	assessment.AssessmentDate = now

	id, err := epm.db.Insert(ctx, "quality_of_life_assessments", map[string]interface{}{
		"patient_id":                 assessment.PatientID,
		"assessment_date":            now.Format(time.RFC3339),
		"physical_pain":              3,
		"energy_fatigue":             3,
		"sleep_quality":              3,
		"mobility":                   3,
		"daily_activities":           3,
		"medication_dependence":      3,
		"work_capacity":              3,
		"positive_feelings":          3,
		"thinking_concentration":     3,
		"self_esteem":                3,
		"body_image":                 3,
		"negative_feelings":          3,
		"meaning_in_life":            3,
		"personal_relationships":     3,
		"social_support":             3,
		"sexual_activity":            3,
		"physical_safety":            3,
		"home_environment":           3,
		"financial_resources":        3,
		"healthcare_access":          3,
		"information_access":         3,
		"leisure_opportunities":      3,
		"environment_quality":        3,
		"transportation":             3,
		"overall_quality_of_life":    assessment.OverallQualityOfLife,
		"overall_health_satisfaction": assessment.OverallHealthSatisfaction,
		"administered_by":            "eva",
		"assessment_method":          "eva_assisted",
		"physical_domain_score":      physScore,
		"psychological_domain_score": psychScore,
		"social_domain_score":        socialScore,
		"environmental_domain_score": envScore,
		"overall_qol_score":          overallScore,
	})
	if err != nil {
		return fmt.Errorf("erro ao registrar QoL: %w", err)
	}

	assessment.ID = fmt.Sprintf("%d", id)

	log.Printf("✅ [EXIT] QoL registrado: Overall=%.1f", assessment.OverallQoLScore)
	return nil
}

func (epm *ExitProtocolManager) GetLatestQoL(patientID int64) (*QoLAssessment, error) {
	ctx := context.Background()

	rows, err := epm.db.QueryByLabel(ctx, "quality_of_life_assessments",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		0,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("nenhuma avaliação QoL encontrada para paciente %d", patientID)
	}

	// Sort by assessment_date DESC and pick the latest
	sort.Slice(rows, func(i, j int) bool {
		ti := database.GetTime(rows[i], "assessment_date")
		tj := database.GetTime(rows[j], "assessment_date")
		return ti.After(tj)
	})

	m := rows[0]
	qol := &QoLAssessment{
		ID:                        fmt.Sprintf("%v", m["id"]),
		PatientID:                 database.GetInt64(m, "patient_id"),
		AssessmentDate:            database.GetTime(m, "assessment_date"),
		PhysicalDomainScore:       database.GetFloat64(m, "physical_domain_score"),
		PsychologicalDomainScore:  database.GetFloat64(m, "psychological_domain_score"),
		SocialDomainScore:         database.GetFloat64(m, "social_domain_score"),
		EnvironmentalDomainScore:  database.GetFloat64(m, "environmental_domain_score"),
		OverallQoLScore:           database.GetFloat64(m, "overall_qol_score"),
		OverallQualityOfLife:      int(database.GetInt64(m, "overall_quality_of_life")),
		OverallHealthSatisfaction: int(database.GetInt64(m, "overall_health_satisfaction")),
	}

	return qol, nil
}

func (epm *ExitProtocolManager) GetQoLTrend(patientID int64, days int) ([]QoLAssessment, error) {
	ctx := context.Background()

	rows, err := epm.db.QueryByLabel(ctx, "quality_of_life_assessments",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		0,
	)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	var assessments []QoLAssessment
	for _, m := range rows {
		assessmentDate := database.GetTime(m, "assessment_date")
		if assessmentDate.After(cutoff) {
			assessments = append(assessments, QoLAssessment{
				ID:                       fmt.Sprintf("%v", m["id"]),
				AssessmentDate:           assessmentDate,
				OverallQoLScore:          database.GetFloat64(m, "overall_qol_score"),
				PhysicalDomainScore:      database.GetFloat64(m, "physical_domain_score"),
				PsychologicalDomainScore: database.GetFloat64(m, "psychological_domain_score"),
			})
		}
	}

	// Sort by assessment_date ASC
	sort.Slice(assessments, func(i, j int) bool {
		return assessments[i].AssessmentDate.Before(assessments[j].AssessmentDate)
	})

	return assessments, nil
}

// ============================================================================
// PAIN & SYMPTOM MONITORING
// ============================================================================

type PainLog struct {
	ID                        string
	PatientID                 int64
	LogTimestamp               time.Time
	PainPresent               bool
	PainIntensity             int
	PainLocation              []string
	PainQuality               []string
	NauseaVomiting            int
	ShortnessOfBreath         int
	Fatigue                   int
	AnxietyLevel              int
	DepressionLevel           int
	OverallWellbeing          int
	MedicationsTaken          []string
	InterventionEffectiveness int
	ReportedBy                string
}

func (epm *ExitProtocolManager) LogPainSymptoms(pl *PainLog) error {
	pl.LogTimestamp = time.Now()

	ctx := context.Background()

	painLocationJSON, _ := json.Marshal(pl.PainLocation)
	painQualityJSON, _ := json.Marshal(pl.PainQuality)
	medicationsJSON, _ := json.Marshal(pl.MedicationsTaken)

	id, err := epm.db.Insert(ctx, "pain_symptom_logs", map[string]interface{}{
		"patient_id":                 pl.PatientID,
		"log_timestamp":              pl.LogTimestamp.Format(time.RFC3339),
		"pain_present":               pl.PainPresent,
		"pain_intensity":             pl.PainIntensity,
		"pain_location":              string(painLocationJSON),
		"pain_quality":               string(painQualityJSON),
		"nausea_vomiting":            pl.NauseaVomiting,
		"shortness_of_breath":        pl.ShortnessOfBreath,
		"fatigue":                    pl.Fatigue,
		"anxiety_level":              pl.AnxietyLevel,
		"depression_level":           pl.DepressionLevel,
		"overall_wellbeing":          pl.OverallWellbeing,
		"medications_taken":          string(medicationsJSON),
		"intervention_effectiveness": pl.InterventionEffectiveness,
		"reported_by":                pl.ReportedBy,
	})
	if err != nil {
		return fmt.Errorf("erro ao registrar dor: %w", err)
	}

	pl.ID = fmt.Sprintf("%d", id)

	// Alertar se dor severa
	if pl.PainIntensity >= 7 {
		epm.handleSeverePainAlert(pl)
	}

	return nil
}

func (epm *ExitProtocolManager) handleSeverePainAlert(painLog *PainLog) {
	log.Printf("🚨 [EXIT] ALERTA: Dor severa detectada (Paciente %d, Intensidade %d/10)",
		painLog.PatientID, painLog.PainIntensity)

	// Buscar comfort care plan
	plan, err := epm.GetComfortCarePlan(painLog.PatientID, "severe_pain")
	if err == nil && plan != nil {
		log.Printf("📋 [EXIT] Comfort Care Plan ativado: %s", plan.ID)
		// Aqui você notificaria cuidadores, sugeriria intervenções, etc.
	} else {
		log.Printf("⚠️ [EXIT] Nenhum Comfort Care Plan encontrado para dor severa")
	}
}

func (epm *ExitProtocolManager) GetRecentPainLogs(patientID int64, hours int) ([]PainLog, error) {
	ctx := context.Background()

	rows, err := epm.db.QueryByLabel(ctx, "pain_symptom_logs",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		0,
	)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	var logs []PainLog
	for _, m := range rows {
		logTime := database.GetTime(m, "log_timestamp")
		if logTime.After(cutoff) {
			logs = append(logs, PainLog{
				ID:                fmt.Sprintf("%v", m["id"]),
				LogTimestamp:       logTime,
				PainPresent:       database.GetBool(m, "pain_present"),
				PainIntensity:     int(database.GetInt64(m, "pain_intensity")),
				NauseaVomiting:    int(database.GetInt64(m, "nausea_vomiting")),
				ShortnessOfBreath: int(database.GetInt64(m, "shortness_of_breath")),
				Fatigue:           int(database.GetInt64(m, "fatigue")),
				AnxietyLevel:      int(database.GetInt64(m, "anxiety_level")),
				DepressionLevel:   int(database.GetInt64(m, "depression_level")),
				OverallWellbeing:  int(database.GetInt64(m, "overall_wellbeing")),
			})
		}
	}

	// Sort by log_timestamp DESC
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].LogTimestamp.After(logs[j].LogTimestamp)
	})

	return logs, nil
}

// ============================================================================
// LEGACY MESSAGES
// ============================================================================

type LegacyMessage struct {
	ID                    string
	PatientID             int64
	RecipientName         string
	RecipientRelationship string
	MessageType           string
	TextContent           string
	DeliveryTrigger       string
	DeliveryDate          *time.Time
	IsComplete            bool
	HasBeenDelivered      bool
	EmotionalTone         string
	Topics                []string
}

func (epm *ExitProtocolManager) CreateLegacyMessage(msg *LegacyMessage) error {
	log.Printf("💌 [EXIT] Criando mensagem de legado para %s (paciente %d)",
		msg.RecipientName, msg.PatientID)

	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)

	topicsJSON, _ := json.Marshal(msg.Topics)

	content := map[string]interface{}{
		"patient_id":              msg.PatientID,
		"recipient_name":         msg.RecipientName,
		"recipient_relationship": msg.RecipientRelationship,
		"message_type":           msg.MessageType,
		"text_content":           msg.TextContent,
		"delivery_trigger":       msg.DeliveryTrigger,
		"emotional_tone":         msg.EmotionalTone,
		"topics":                 string(topicsJSON),
		"is_complete":            false,
		"has_been_delivered":     false,
		"created_at":             now,
		"updated_at":             now,
	}

	if msg.DeliveryDate != nil {
		content["delivery_date"] = msg.DeliveryDate.Format(time.RFC3339)
	}

	id, err := epm.db.Insert(ctx, "legacy_messages", content)
	if err != nil {
		return fmt.Errorf("erro ao criar legacy message: %w", err)
	}

	msg.ID = fmt.Sprintf("%d", id)

	log.Printf("✅ [EXIT] Mensagem de legado criada: ID=%s", msg.ID)
	return nil
}

func (epm *ExitProtocolManager) MarkLegacyMessageComplete(messageID string) error {
	ctx := context.Background()
	return epm.db.Update(ctx, "legacy_messages",
		map[string]interface{}{"id": messageID},
		map[string]interface{}{
			"is_complete": true,
			"updated_at":  time.Now().Format(time.RFC3339),
		},
	)
}

func (epm *ExitProtocolManager) GetLegacyMessages(patientID int64) ([]LegacyMessage, error) {
	ctx := context.Background()

	rows, err := epm.db.QueryByLabel(ctx, "legacy_messages",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		0,
	)
	if err != nil {
		return nil, err
	}

	// Sort by created_at ASC
	sort.Slice(rows, func(i, j int) bool {
		ti := database.GetTime(rows[i], "created_at")
		tj := database.GetTime(rows[j], "created_at")
		return ti.Before(tj)
	})

	var messages []LegacyMessage
	for _, m := range rows {
		messages = append(messages, LegacyMessage{
			ID:                    fmt.Sprintf("%v", m["id"]),
			RecipientName:         database.GetString(m, "recipient_name"),
			RecipientRelationship: database.GetString(m, "recipient_relationship"),
			MessageType:           database.GetString(m, "message_type"),
			DeliveryTrigger:       database.GetString(m, "delivery_trigger"),
			IsComplete:            database.GetBool(m, "is_complete"),
			HasBeenDelivered:      database.GetBool(m, "has_been_delivered"),
		})
	}

	return messages, nil
}

// ============================================================================
// FAREWELL PREPARATION
// ============================================================================

type FarewellPreparation struct {
	ID                          string
	PatientID                   int64
	LegalAffairsComplete        bool
	FinancialAffairsComplete    bool
	FuneralArrangementsComplete bool
	ReconciliationsNeeded       []string
	ReconciliationsCompleted    []string
	GoodbyesNeeded              []string
	GoodbyesCompleted           []string
	FiveStagesGriefPosition     string
	EmotionalReadiness          int
	SpiritualReadiness          int
	BucketListItems             []string
	BucketListCompleted         []string
	OverallPreparationScore     int
	PeaceWithLife               bool
	PeaceWithDeath              bool
}

func (epm *ExitProtocolManager) CreateFarewellPreparation(patientID int64) (*FarewellPreparation, error) {
	log.Printf("🕊️ [EXIT] Iniciando preparação para despedida (paciente %d)", patientID)

	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)

	id, err := epm.db.Insert(ctx, "farewell_preparation", map[string]interface{}{
		"patient_id":                  patientID,
		"five_stages_grief_position":  "denial",
		"legal_affairs_complete":      false,
		"financial_affairs_complete":  false,
		"funeral_arrangements_complete": false,
		"peace_with_life":             false,
		"peace_with_death":            false,
		"created_at":                  now,
		"last_updated":                now,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao criar farewell preparation: %w", err)
	}

	fp := &FarewellPreparation{
		ID:                      fmt.Sprintf("%d", id),
		PatientID:               patientID,
		FiveStagesGriefPosition: "denial",
	}

	log.Printf("✅ [EXIT] Farewell Preparation criado: ID=%s", fp.ID)
	return fp, nil
}

func (epm *ExitProtocolManager) UpdateFarewellPreparation(patientID int64, updates map[string]interface{}) error {
	ctx := context.Background()

	updates["last_updated"] = time.Now().Format(time.RFC3339)

	return epm.db.Update(ctx, "farewell_preparation",
		map[string]interface{}{"patient_id": patientID},
		updates,
	)
}

func (epm *ExitProtocolManager) GetFarewellPreparation(patientID int64) (*FarewellPreparation, error) {
	ctx := context.Background()

	rows, err := epm.db.QueryByLabel(ctx, "farewell_preparation",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		1,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("farewell preparation não encontrado")
	}

	m := rows[0]
	fp := &FarewellPreparation{
		ID:                          fmt.Sprintf("%v", m["id"]),
		PatientID:                   database.GetInt64(m, "patient_id"),
		LegalAffairsComplete:        database.GetBool(m, "legal_affairs_complete"),
		FinancialAffairsComplete:    database.GetBool(m, "financial_affairs_complete"),
		FuneralArrangementsComplete: database.GetBool(m, "funeral_arrangements_complete"),
		FiveStagesGriefPosition:     database.GetString(m, "five_stages_grief_position"),
		EmotionalReadiness:          int(database.GetInt64(m, "emotional_readiness")),
		SpiritualReadiness:          int(database.GetInt64(m, "spiritual_readiness")),
		OverallPreparationScore:     int(database.GetInt64(m, "overall_preparation_score")),
		PeaceWithLife:               database.GetBool(m, "peace_with_life"),
		PeaceWithDeath:              database.GetBool(m, "peace_with_death"),
	}

	return fp, nil
}

// ============================================================================
// COMFORT CARE PLANS
// ============================================================================

type ComfortCarePlan struct {
	ID               string
	PatientID        int64
	TriggerSymptom   string
	TriggerThreshold int
	Interventions    []Intervention
	IsActive         bool
	TimesUsed        int
}

type Intervention struct {
	Order              int    `json:"order"`
	Type               string `json:"type"`
	Action             string `json:"action"`
	RepeatAfterMinutes int    `json:"repeat_after_minutes,omitempty"`
}

func (epm *ExitProtocolManager) CreateComfortCarePlan(plan *ComfortCarePlan) error {
	log.Printf("📋 [EXIT] Criando Comfort Care Plan para %s (paciente %d)",
		plan.TriggerSymptom, plan.PatientID)

	ctx := context.Background()

	interventionsJSON, _ := json.Marshal(plan.Interventions)

	id, err := epm.db.Insert(ctx, "comfort_care_plans", map[string]interface{}{
		"patient_id":        plan.PatientID,
		"trigger_symptom":   plan.TriggerSymptom,
		"trigger_threshold": plan.TriggerThreshold,
		"interventions":     string(interventionsJSON),
		"is_active":         plan.IsActive,
		"times_used":        0,
		"created_at":        time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("erro ao criar comfort care plan: %w", err)
	}

	plan.ID = fmt.Sprintf("%d", id)

	log.Printf("✅ [EXIT] Comfort Care Plan criado: ID=%s", plan.ID)
	return nil
}

func (epm *ExitProtocolManager) GetComfortCarePlan(patientID int64, symptom string) (*ComfortCarePlan, error) {
	ctx := context.Background()

	rows, err := epm.db.QueryByLabel(ctx, "comfort_care_plans",
		" AND n.patient_id = $pid AND n.trigger_symptom = $symptom",
		map[string]interface{}{
			"pid":     patientID,
			"symptom": symptom,
		},
		0,
	)
	if err != nil {
		return nil, err
	}

	// Filter for is_active = true in Go and pick first match
	for _, m := range rows {
		if !database.GetBool(m, "is_active") {
			continue
		}

		plan := &ComfortCarePlan{
			ID:               fmt.Sprintf("%v", m["id"]),
			PatientID:        database.GetInt64(m, "patient_id"),
			TriggerSymptom:   database.GetString(m, "trigger_symptom"),
			TriggerThreshold: int(database.GetInt64(m, "trigger_threshold")),
			TimesUsed:        int(database.GetInt64(m, "times_used")),
			IsActive:         true,
		}

		// Parse interventions JSON
		interventionsStr := database.GetString(m, "interventions")
		if interventionsStr != "" {
			json.Unmarshal([]byte(interventionsStr), &plan.Interventions)
		}

		return plan, nil
	}

	return nil, nil // Nenhum plano encontrado (não é erro)
}

func (epm *ExitProtocolManager) IncrementComfortCarePlanUsage(planID string, effectiveness int) error {
	ctx := context.Background()

	// First, read the current node to compute new average
	rows, err := epm.db.QueryByLabel(ctx, "comfort_care_plans",
		" AND n.id = $planid",
		map[string]interface{}{"planid": planID},
		1,
	)
	if err != nil {
		return err
	}

	var newTimesUsed int
	var newAvgEffectiveness float64

	if len(rows) > 0 {
		m := rows[0]
		timesUsed := int(database.GetInt64(m, "times_used"))
		avgEff := database.GetFloat64(m, "average_effectiveness")

		newTimesUsed = timesUsed + 1
		if timesUsed == 0 {
			newAvgEffectiveness = float64(effectiveness)
		} else {
			newAvgEffectiveness = (avgEff*float64(timesUsed) + float64(effectiveness)) / float64(newTimesUsed)
		}
	} else {
		newTimesUsed = 1
		newAvgEffectiveness = float64(effectiveness)
	}

	return epm.db.Update(ctx, "comfort_care_plans",
		map[string]interface{}{"id": planID},
		map[string]interface{}{
			"times_used":              newTimesUsed,
			"last_used":               time.Now().Format(time.RFC3339),
			"average_effectiveness":   newAvgEffectiveness,
		},
	)
}

// ============================================================================
// SPIRITUAL CARE SESSIONS
// ============================================================================

type SpiritualCareSession struct {
	ID                       string
	PatientID                int64
	SessionDate              time.Time
	ConductedBy              string
	ConductorName            string
	TopicsDiscussed          []string
	PracticesPerformed       []string
	PreSessionPeaceLevel     int
	PostSessionPeaceLevel    int
	SpiritualNeedsIdentified []string
	FollowUpNeeded           bool
	DurationMinutes          int
}

func (epm *ExitProtocolManager) RecordSpiritualCareSession(session *SpiritualCareSession) error {
	log.Printf("🕊️ [EXIT] Registrando sessão de cuidado espiritual (paciente %d)", session.PatientID)

	ctx := context.Background()
	session.SessionDate = time.Now()

	topicsJSON, _ := json.Marshal(session.TopicsDiscussed)
	practicesJSON, _ := json.Marshal(session.PracticesPerformed)
	needsJSON, _ := json.Marshal(session.SpiritualNeedsIdentified)

	id, err := epm.db.Insert(ctx, "spiritual_care_sessions", map[string]interface{}{
		"patient_id":                session.PatientID,
		"session_date":              session.SessionDate.Format(time.RFC3339),
		"conducted_by":              session.ConductedBy,
		"conductor_name":            session.ConductorName,
		"topics_discussed":          string(topicsJSON),
		"practices_performed":       string(practicesJSON),
		"pre_session_peace_level":   session.PreSessionPeaceLevel,
		"post_session_peace_level":  session.PostSessionPeaceLevel,
		"spiritual_needs_identified": string(needsJSON),
		"follow_up_needed":          session.FollowUpNeeded,
		"duration_minutes":          session.DurationMinutes,
	})
	if err != nil {
		return fmt.Errorf("erro ao registrar sessão espiritual: %w", err)
	}

	session.ID = fmt.Sprintf("%d", id)

	peaceDelta := session.PostSessionPeaceLevel - session.PreSessionPeaceLevel
	log.Printf("✅ [EXIT] Sessão espiritual registrada: Peace Δ=%+d", peaceDelta)

	return nil
}

// ============================================================================
// PALLIATIVE CARE SUMMARY
// ============================================================================

type PalliativeSummary struct {
	PatientID               int64
	PatientName             string
	Age                     int
	LastWishesCompletion    int
	ResuscitationPreference string
	OverallQoLScore         float64
	AvgPain7Days            float64
	MaxPain7Days            int
	EmotionalReadiness      int
	SpiritualReadiness      int
	LegacyMessagesCompleted int
	LegacyMessagesPending   int
}

func (epm *ExitProtocolManager) GetPalliativeCareSummary(patientID int64) (*PalliativeSummary, error) {
	ctx := context.Background()
	summary := &PalliativeSummary{PatientID: patientID}

	// 1. Get Last Wishes
	lw, err := epm.GetLastWishes(patientID)
	if err == nil && lw != nil {
		summary.LastWishesCompletion = lw.CompletionPercentage
		summary.ResuscitationPreference = lw.ResuscitationPreference
	}

	// 2. Get latest QoL
	qol, err := epm.GetLatestQoL(patientID)
	if err == nil && qol != nil {
		summary.OverallQoLScore = qol.OverallQoLScore
	}

	// 3. Get pain stats for last 7 days
	painLogs, err := epm.GetRecentPainLogs(patientID, 7*24)
	if err == nil && len(painLogs) > 0 {
		totalPain := 0
		maxPain := 0
		for _, pl := range painLogs {
			totalPain += pl.PainIntensity
			if pl.PainIntensity > maxPain {
				maxPain = pl.PainIntensity
			}
		}
		summary.AvgPain7Days = float64(totalPain) / float64(len(painLogs))
		summary.MaxPain7Days = maxPain
	}

	// 4. Get farewell preparation readiness
	fp, err := epm.GetFarewellPreparation(patientID)
	if err == nil && fp != nil {
		summary.EmotionalReadiness = fp.EmotionalReadiness
		summary.SpiritualReadiness = fp.SpiritualReadiness
	}

	// 5. Get legacy messages stats
	messages, err := epm.GetLegacyMessages(patientID)
	if err == nil {
		for _, msg := range messages {
			if msg.IsComplete {
				summary.LegacyMessagesCompleted++
			} else {
				summary.LegacyMessagesPending++
			}
		}
	}

	// 6. Try to get patient name/age from idosos table
	idosoNode, err := epm.db.GetNodeByID(ctx, "idosos", patientID)
	if err == nil && idosoNode != nil {
		summary.PatientName = database.GetString(idosoNode, "nome")
		dob := database.GetTime(idosoNode, "data_nascimento")
		if !dob.IsZero() {
			summary.Age = int(time.Since(dob).Hours() / 24 / 365.25)
		}
	}

	return summary, nil
}

// ============================================================================
// UNCONTROLLED PAIN ALERTS
// ============================================================================

type PainAlert struct {
	PatientID                 int64
	PatientName               string
	PainIntensity             int
	PainLocation              []string
	HoursSinceReport          float64
	InterventionEffectiveness int
}

func (epm *ExitProtocolManager) GetUncontrolledPainAlerts() ([]PainAlert, error) {
	ctx := context.Background()

	// Query all recent pain logs with high intensity (>= 7) and low/no intervention effectiveness
	rows, err := epm.db.QueryByLabel(ctx, "pain_symptom_logs", "", nil, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var alerts []PainAlert

	for _, m := range rows {
		intensity := int(database.GetInt64(m, "pain_intensity"))
		if intensity < 7 {
			continue
		}

		interventionEff := int(database.GetInt64(m, "intervention_effectiveness"))
		if interventionEff >= 5 {
			continue // Intervention is working, not uncontrolled
		}

		logTime := database.GetTime(m, "log_timestamp")
		hoursSince := now.Sub(logTime).Hours()

		// Only include recent alerts (within 48 hours)
		if hoursSince > 48 {
			continue
		}

		patientID := database.GetInt64(m, "patient_id")

		// Try to get patient name
		patientName := ""
		idosoNode, err := epm.db.GetNodeByID(ctx, "idosos", patientID)
		if err == nil && idosoNode != nil {
			patientName = database.GetString(idosoNode, "nome")
		}

		alerts = append(alerts, PainAlert{
			PatientID:                 patientID,
			PatientName:               patientName,
			PainIntensity:             intensity,
			HoursSinceReport:          hoursSince,
			InterventionEffectiveness: interventionEff,
		})
	}

	// Sort by pain_intensity DESC, hours_since_report DESC
	sort.Slice(alerts, func(i, j int) bool {
		if alerts[i].PainIntensity != alerts[j].PainIntensity {
			return alerts[i].PainIntensity > alerts[j].PainIntensity
		}
		return alerts[i].HoursSinceReport > alerts[j].HoursSinceReport
	})

	return alerts, nil
}
