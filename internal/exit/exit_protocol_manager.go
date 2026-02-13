package exit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ============================================================================
// EXIT PROTOCOL MANAGER
// ============================================================================
// Gerencia cuidados paliativos, qualidade de vida e preparação para despedida

type ExitProtocolManager struct {
	db *sql.DB
}

func NewExitProtocolManager(db *sql.DB) *ExitProtocolManager {
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

	query := `
		INSERT INTO last_wishes (patient_id)
		VALUES ($1)
		RETURNING id, patient_id, completion_percentage, completed
	`

	lw := &LastWishes{}
	err := epm.db.QueryRow(query, patientID).Scan(
		&lw.ID,
		&lw.PatientID,
		&lw.CompletionPercentage,
		&lw.Completed,
	)

	if err != nil {
		return nil, fmt.Errorf("erro ao criar last wishes: %w", err)
	}

	log.Printf("✅ [EXIT] Last Wishes criado: ID=%s", lw.ID)
	return lw, nil
}

func (epm *ExitProtocolManager) UpdateLastWishes(lastWishesID string, updates map[string]interface{}) error {
	log.Printf("📝 [EXIT] Atualizando Last Wishes %s", lastWishesID)

	// Construir query dinâmica
	setClause := ""
	args := []interface{}{}
	argIndex := 1

	for key, value := range updates {
		if setClause != "" {
			setClause += ", "
		}
		setClause += fmt.Sprintf("%s = $%d", key, argIndex)
		args = append(args, value)
		argIndex++
	}

	args = append(args, lastWishesID)

	query := fmt.Sprintf(`
		UPDATE last_wishes
		SET %s, last_reviewed_at = NOW()
		WHERE id = $%d
	`, setClause, argIndex)

	_, err := epm.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("erro ao atualizar last wishes: %w", err)
	}

	log.Printf("✅ [EXIT] Last Wishes atualizado")
	return nil
}

func (epm *ExitProtocolManager) GetLastWishes(patientID int64) (*LastWishes, error) {
	query := `
		SELECT
			id, patient_id, resuscitation_preference, preferred_death_location,
			pain_management_preference, organ_donation_preference, burial_cremation,
			personal_statement, completion_percentage, completed
		FROM last_wishes
		WHERE patient_id = $1
	`

	lw := &LastWishes{}
	var resuscitation, deathLocation, painMgmt, organDonation, burialCremation sql.NullString
	var personalStatement sql.NullString

	err := epm.db.QueryRow(query, patientID).Scan(
		&lw.ID,
		&lw.PatientID,
		&resuscitation,
		&deathLocation,
		&painMgmt,
		&organDonation,
		&burialCremation,
		&personalStatement,
		&lw.CompletionPercentage,
		&lw.Completed,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("last wishes não encontrado para paciente %d", patientID)
		}
		return nil, err
	}

	if resuscitation.Valid {
		lw.ResuscitationPreference = resuscitation.String
	}
	if deathLocation.Valid {
		lw.PreferredDeathLocation = deathLocation.String
	}
	if painMgmt.Valid {
		lw.PainManagementPreference = painMgmt.String
	}
	if organDonation.Valid {
		lw.OrganDonationPreference = organDonation.String
	}
	if burialCremation.Valid {
		lw.BurialCremation = burialCremation.String
	}
	if personalStatement.Valid {
		lw.PersonalStatement = personalStatement.String
	}

	return lw, nil
}

// ============================================================================
// QUALITY OF LIFE ASSESSMENTS (WHOQOL-BREF)
// ============================================================================

type QoLAssessment struct {
	ID                         string
	PatientID                  int64
	AssessmentDate             time.Time
	PhysicalDomainScore        float64
	PsychologicalDomainScore   float64
	SocialDomainScore          float64
	EnvironmentalDomainScore   float64
	OverallQoLScore            float64
	OverallQualityOfLife       int
	OverallHealthSatisfaction  int
}

func (epm *ExitProtocolManager) RecordQoLAssessment(assessment *QoLAssessment) error {
	log.Printf("📊 [EXIT] Registrando avaliação WHOQOL-BREF para paciente %d", assessment.PatientID)

	query := `
		INSERT INTO quality_of_life_assessments (
			patient_id,
			physical_pain, energy_fatigue, sleep_quality, mobility, daily_activities,
			medication_dependence, work_capacity,
			positive_feelings, thinking_concentration, self_esteem, body_image,
			negative_feelings, meaning_in_life,
			personal_relationships, social_support, sexual_activity,
			physical_safety, home_environment, financial_resources, healthcare_access,
			information_access, leisure_opportunities, environment_quality, transportation,
			overall_quality_of_life, overall_health_satisfaction,
			administered_by, assessment_method
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29
		)
		RETURNING id, physical_domain_score, psychological_domain_score,
		          social_domain_score, environmental_domain_score, overall_qol_score
	`

	// Por simplicidade, vou usar valores padrão médios (3) para todas as questões
	// Em produção, você coletaria todas as respostas do usuário
	err := epm.db.QueryRow(query,
		assessment.PatientID,
		3, 3, 3, 3, 3, 3, 3, // Físico
		3, 3, 3, 3, 3, 3, // Psicológico
		3, 3, 3, // Social
		3, 3, 3, 3, 3, 3, 3, 3, // Ambiental
		assessment.OverallQualityOfLife,
		assessment.OverallHealthSatisfaction,
		"eva", "eva_assisted",
	).Scan(
		&assessment.ID,
		&assessment.PhysicalDomainScore,
		&assessment.PsychologicalDomainScore,
		&assessment.SocialDomainScore,
		&assessment.EnvironmentalDomainScore,
		&assessment.OverallQoLScore,
	)

	if err != nil {
		return fmt.Errorf("erro ao registrar QoL: %w", err)
	}

	log.Printf("✅ [EXIT] QoL registrado: Overall=%.1f", assessment.OverallQoLScore)
	return nil
}

func (epm *ExitProtocolManager) GetLatestQoL(patientID int64) (*QoLAssessment, error) {
	query := `
		SELECT
			id, patient_id, assessment_date,
			physical_domain_score, psychological_domain_score,
			social_domain_score, environmental_domain_score,
			overall_qol_score,
			overall_quality_of_life, overall_health_satisfaction
		FROM quality_of_life_assessments
		WHERE patient_id = $1
		ORDER BY assessment_date DESC
		LIMIT 1
	`

	qol := &QoLAssessment{}
	err := epm.db.QueryRow(query, patientID).Scan(
		&qol.ID,
		&qol.PatientID,
		&qol.AssessmentDate,
		&qol.PhysicalDomainScore,
		&qol.PsychologicalDomainScore,
		&qol.SocialDomainScore,
		&qol.EnvironmentalDomainScore,
		&qol.OverallQoLScore,
		&qol.OverallQualityOfLife,
		&qol.OverallHealthSatisfaction,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("nenhuma avaliação QoL encontrada para paciente %d", patientID)
		}
		return nil, err
	}

	return qol, nil
}

func (epm *ExitProtocolManager) GetQoLTrend(patientID int64, days int) ([]QoLAssessment, error) {
	query := `
		SELECT
			id, assessment_date, overall_qol_score,
			physical_domain_score, psychological_domain_score
		FROM quality_of_life_assessments
		WHERE patient_id = $1
		  AND assessment_date > NOW() - INTERVAL '%d days'
		ORDER BY assessment_date ASC
	`

	rows, err := epm.db.Query(fmt.Sprintf(query, days), patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assessments := []QoLAssessment{}
	for rows.Next() {
		var qol QoLAssessment
		err := rows.Scan(
			&qol.ID,
			&qol.AssessmentDate,
			&qol.OverallQoLScore,
			&qol.PhysicalDomainScore,
			&qol.PsychologicalDomainScore,
		)
		if err != nil {
			continue
		}
		assessments = append(assessments, qol)
	}

	return assessments, nil
}

// ============================================================================
// PAIN & SYMPTOM MONITORING
// ============================================================================

type PainLog struct {
	ID                   string
	PatientID            int64
	LogTimestamp         time.Time
	PainPresent          bool
	PainIntensity        int
	PainLocation         []string
	PainQuality          []string
	NauseaVomiting       int
	ShortnessOfBreath    int
	Fatigue              int
	AnxietyLevel         int
	DepressionLevel      int
	OverallWellbeing     int
	MedicationsTaken     []string
	InterventionEffectiveness int
	ReportedBy           string
}

func (epm *ExitProtocolManager) LogPainSymptoms(log *PainLog) error {
	log.LogTimestamp = time.Now()

	query := `
		INSERT INTO pain_symptom_logs (
			patient_id, pain_present, pain_intensity,
			pain_location, pain_quality,
			nausea_vomiting, shortness_of_breath, fatigue,
			anxiety_level, depression_level, overall_wellbeing,
			medications_taken, intervention_effectiveness,
			reported_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`

	painLocationJSON, _ := json.Marshal(log.PainLocation)
	painQualityJSON, _ := json.Marshal(log.PainQuality)
	medicationsJSON, _ := json.Marshal(log.MedicationsTaken)

	err := epm.db.QueryRow(query,
		log.PatientID,
		log.PainPresent,
		log.PainIntensity,
		painLocationJSON,
		painQualityJSON,
		log.NauseaVomiting,
		log.ShortnessOfBreath,
		log.Fatigue,
		log.AnxietyLevel,
		log.DepressionLevel,
		log.OverallWellbeing,
		medicationsJSON,
		log.InterventionEffectiveness,
		log.ReportedBy,
	).Scan(&log.ID)

	if err != nil {
		return fmt.Errorf("erro ao registrar dor: %w", err)
	}

	// Alertar se dor severa
	if log.PainIntensity >= 7 {
		epm.handleSeverePainAlert(log)
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
	query := `
		SELECT
			id, log_timestamp, pain_present, pain_intensity,
			nausea_vomiting, shortness_of_breath, fatigue,
			anxiety_level, depression_level, overall_wellbeing
		FROM pain_symptom_logs
		WHERE patient_id = $1
		  AND log_timestamp > NOW() - INTERVAL '%d hours'
		ORDER BY log_timestamp DESC
	`

	rows, err := epm.db.Query(fmt.Sprintf(query, hours), patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []PainLog{}
	for rows.Next() {
		var pl PainLog
		err := rows.Scan(
			&pl.ID, &pl.LogTimestamp, &pl.PainPresent, &pl.PainIntensity,
			&pl.NauseaVomiting, &pl.ShortnessOfBreath, &pl.Fatigue,
			&pl.AnxietyLevel, &pl.DepressionLevel, &pl.OverallWellbeing,
		)
		if err != nil {
			continue
		}
		logs = append(logs, pl)
	}

	return logs, nil
}

// ============================================================================
// LEGACY MESSAGES
// ============================================================================

type LegacyMessage struct {
	ID                   string
	PatientID            int64
	RecipientName        string
	RecipientRelationship string
	MessageType          string
	TextContent          string
	DeliveryTrigger      string
	DeliveryDate         *time.Time
	IsComplete           bool
	HasBeenDelivered     bool
	EmotionalTone        string
	Topics               []string
}

func (epm *ExitProtocolManager) CreateLegacyMessage(msg *LegacyMessage) error {
	log.Printf("💌 [EXIT] Criando mensagem de legado para %s (paciente %d)",
		msg.RecipientName, msg.PatientID)

	query := `
		INSERT INTO legacy_messages (
			patient_id, recipient_name, recipient_relationship,
			message_type, text_content, delivery_trigger,
			delivery_date, emotional_tone, topics
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	topicsJSON, _ := json.Marshal(msg.Topics)

	err := epm.db.QueryRow(query,
		msg.PatientID,
		msg.RecipientName,
		msg.RecipientRelationship,
		msg.MessageType,
		msg.TextContent,
		msg.DeliveryTrigger,
		msg.DeliveryDate,
		msg.EmotionalTone,
		topicsJSON,
	).Scan(&msg.ID)

	if err != nil {
		return fmt.Errorf("erro ao criar legacy message: %w", err)
	}

	log.Printf("✅ [EXIT] Mensagem de legado criada: ID=%s", msg.ID)
	return nil
}

func (epm *ExitProtocolManager) MarkLegacyMessageComplete(messageID string) error {
	query := `UPDATE legacy_messages SET is_complete = TRUE, updated_at = NOW() WHERE id = $1`
	_, err := epm.db.Exec(query, messageID)
	return err
}

func (epm *ExitProtocolManager) GetLegacyMessages(patientID int64) ([]LegacyMessage, error) {
	query := `
		SELECT
			id, recipient_name, recipient_relationship, message_type,
			delivery_trigger, is_complete, has_been_delivered
		FROM legacy_messages
		WHERE patient_id = $1
		ORDER BY created_at ASC
	`

	rows, err := epm.db.Query(query, patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []LegacyMessage{}
	for rows.Next() {
		var msg LegacyMessage
		err := rows.Scan(
			&msg.ID, &msg.RecipientName, &msg.RecipientRelationship,
			&msg.MessageType, &msg.DeliveryTrigger,
			&msg.IsComplete, &msg.HasBeenDelivered,
		)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
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

	query := `
		INSERT INTO farewell_preparation (patient_id, five_stages_grief_position)
		VALUES ($1, 'denial')
		RETURNING id
	`

	fp := &FarewellPreparation{PatientID: patientID}
	err := epm.db.QueryRow(query, patientID).Scan(&fp.ID)

	if err != nil {
		return nil, fmt.Errorf("erro ao criar farewell preparation: %w", err)
	}

	log.Printf("✅ [EXIT] Farewell Preparation criado: ID=%s", fp.ID)
	return fp, nil
}

func (epm *ExitProtocolManager) UpdateFarewellPreparation(patientID int64, updates map[string]interface{}) error {
	// Similar à UpdateLastWishes, construir query dinâmica
	setClause := ""
	args := []interface{}{}
	argIndex := 1

	for key, value := range updates {
		if setClause != "" {
			setClause += ", "
		}
		setClause += fmt.Sprintf("%s = $%d", key, argIndex)
		args = append(args, value)
		argIndex++
	}

	args = append(args, patientID)

	query := fmt.Sprintf(`
		UPDATE farewell_preparation
		SET %s, last_updated = NOW()
		WHERE patient_id = $%d
	`, setClause, argIndex)

	_, err := epm.db.Exec(query, args...)
	return err
}

func (epm *ExitProtocolManager) GetFarewellPreparation(patientID int64) (*FarewellPreparation, error) {
	query := `
		SELECT
			id, patient_id, legal_affairs_complete, financial_affairs_complete,
			funeral_arrangements_complete, five_stages_grief_position,
			emotional_readiness, spiritual_readiness, overall_preparation_score,
			peace_with_life, peace_with_death
		FROM farewell_preparation
		WHERE patient_id = $1
	`

	fp := &FarewellPreparation{}
	var emotionalReadiness, spiritualReadiness, overallScore sql.NullInt64

	err := epm.db.QueryRow(query, patientID).Scan(
		&fp.ID,
		&fp.PatientID,
		&fp.LegalAffairsComplete,
		&fp.FinancialAffairsComplete,
		&fp.FuneralArrangementsComplete,
		&fp.FiveStagesGriefPosition,
		&emotionalReadiness,
		&spiritualReadiness,
		&overallScore,
		&fp.PeaceWithLife,
		&fp.PeaceWithDeath,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("farewell preparation não encontrado")
		}
		return nil, err
	}

	if emotionalReadiness.Valid {
		fp.EmotionalReadiness = int(emotionalReadiness.Int64)
	}
	if spiritualReadiness.Valid {
		fp.SpiritualReadiness = int(spiritualReadiness.Int64)
	}
	if overallScore.Valid {
		fp.OverallPreparationScore = int(overallScore.Int64)
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

	interventionsJSON, _ := json.Marshal(plan.Interventions)

	query := `
		INSERT INTO comfort_care_plans (
			patient_id, trigger_symptom, trigger_threshold,
			interventions, is_active
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	err := epm.db.QueryRow(query,
		plan.PatientID,
		plan.TriggerSymptom,
		plan.TriggerThreshold,
		interventionsJSON,
		plan.IsActive,
	).Scan(&plan.ID)

	if err != nil {
		return fmt.Errorf("erro ao criar comfort care plan: %w", err)
	}

	log.Printf("✅ [EXIT] Comfort Care Plan criado: ID=%s", plan.ID)
	return nil
}

func (epm *ExitProtocolManager) GetComfortCarePlan(patientID int64, symptom string) (*ComfortCarePlan, error) {
	query := `
		SELECT id, patient_id, trigger_symptom, trigger_threshold, interventions, times_used
		FROM comfort_care_plans
		WHERE patient_id = $1 AND trigger_symptom = $2 AND is_active = TRUE
		LIMIT 1
	`

	plan := &ComfortCarePlan{}
	var interventionsJSON []byte

	err := epm.db.QueryRow(query, patientID, symptom).Scan(
		&plan.ID,
		&plan.PatientID,
		&plan.TriggerSymptom,
		&plan.TriggerThreshold,
		&interventionsJSON,
		&plan.TimesUsed,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Nenhum plano encontrado (não é erro)
		}
		return nil, err
	}

	// Parse interventions
	json.Unmarshal(interventionsJSON, &plan.Interventions)

	return plan, nil
}

func (epm *ExitProtocolManager) IncrementComfortCarePlanUsage(planID string, effectiveness int) error {
	query := `
		UPDATE comfort_care_plans
		SET times_used = times_used + 1,
		    last_used = NOW(),
		    average_effectiveness = COALESCE(
		        (average_effectiveness * times_used + $2) / (times_used + 1),
		        $2
		    )
		WHERE id = $1
	`

	_, err := epm.db.Exec(query, planID, effectiveness)
	return err
}

// ============================================================================
// SPIRITUAL CARE SESSIONS
// ============================================================================

type SpiritualCareSession struct {
	ID                      string
	PatientID               int64
	SessionDate             time.Time
	ConductedBy             string
	ConductorName           string
	TopicsDiscussed         []string
	PracticesPerformed      []string
	PreSessionPeaceLevel    int
	PostSessionPeaceLevel   int
	SpiritualNeedsIdentified []string
	FollowUpNeeded          bool
	DurationMinutes         int
}

func (epm *ExitProtocolManager) RecordSpiritualCareSession(session *SpiritualCareSession) error {
	log.Printf("🕊️ [EXIT] Registrando sessão de cuidado espiritual (paciente %d)", session.PatientID)

	session.SessionDate = time.Now()

	topicsJSON, _ := json.Marshal(session.TopicsDiscussed)
	practicesJSON, _ := json.Marshal(session.PracticesPerformed)
	needsJSON, _ := json.Marshal(session.SpiritualNeedsIdentified)

	query := `
		INSERT INTO spiritual_care_sessions (
			patient_id, conducted_by, conductor_name,
			topics_discussed, practices_performed,
			pre_session_peace_level, post_session_peace_level,
			spiritual_needs_identified, follow_up_needed,
			duration_minutes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	err := epm.db.QueryRow(query,
		session.PatientID,
		session.ConductedBy,
		session.ConductorName,
		topicsJSON,
		practicesJSON,
		session.PreSessionPeaceLevel,
		session.PostSessionPeaceLevel,
		needsJSON,
		session.FollowUpNeeded,
		session.DurationMinutes,
	).Scan(&session.ID)

	if err != nil {
		return fmt.Errorf("erro ao registrar sessão espiritual: %w", err)
	}

	peaceDelta := session.PostSessionPeaceLevel - session.PreSessionPeaceLevel
	log.Printf("✅ [EXIT] Sessão espiritual registrada: Peace Δ=%+d", peaceDelta)

	return nil
}

// ============================================================================
// PALLIATIVE CARE SUMMARY
// ============================================================================

type PalliativeSummary struct {
	PatientID                 int64
	PatientName               string
	Age                       int
	LastWishesCompletion      int
	ResuscitationPreference   string
	OverallQoLScore           float64
	AvgPain7Days              float64
	MaxPain7Days              int
	EmotionalReadiness        int
	SpiritualReadiness        int
	LegacyMessagesCompleted   int
	LegacyMessagesPending     int
}

func (epm *ExitProtocolManager) GetPalliativeCareSummary(patientID int64) (*PalliativeSummary, error) {
	query := `
		SELECT
			patient_id, nome, age,
			last_wishes_completion, resuscitation_preference,
			overall_qol_score, avg_pain_7days, max_pain_7days,
			emotional_readiness, spiritual_readiness,
			legacy_messages_completed, legacy_messages_pending
		FROM v_palliative_care_summary
		WHERE patient_id = $1
	`

	summary := &PalliativeSummary{}
	var resuscitation sql.NullString
	var qolScore, avgPain sql.NullFloat64
	var maxPain, lastWishesCompletion, emotionalReadiness, spiritualReadiness sql.NullInt64
	var legacyCompleted, legacyPending sql.NullInt64

	err := epm.db.QueryRow(query, patientID).Scan(
		&summary.PatientID,
		&summary.PatientName,
		&summary.Age,
		&lastWishesCompletion,
		&resuscitation,
		&qolScore,
		&avgPain,
		&maxPain,
		&emotionalReadiness,
		&spiritualReadiness,
		&legacyCompleted,
		&legacyPending,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("paciente %d não está em cuidados paliativos", patientID)
		}
		return nil, err
	}

	if resuscitation.Valid {
		summary.ResuscitationPreference = resuscitation.String
	}
	if qolScore.Valid {
		summary.OverallQoLScore = qolScore.Float64
	}
	if avgPain.Valid {
		summary.AvgPain7Days = avgPain.Float64
	}
	if maxPain.Valid {
		summary.MaxPain7Days = int(maxPain.Int64)
	}
	if lastWishesCompletion.Valid {
		summary.LastWishesCompletion = int(lastWishesCompletion.Int64)
	}
	if emotionalReadiness.Valid {
		summary.EmotionalReadiness = int(emotionalReadiness.Int64)
	}
	if spiritualReadiness.Valid {
		summary.SpiritualReadiness = int(spiritualReadiness.Int64)
	}
	if legacyCompleted.Valid {
		summary.LegacyMessagesCompleted = int(legacyCompleted.Int64)
	}
	if legacyPending.Valid {
		summary.LegacyMessagesPending = int(legacyPending.Int64)
	}

	return summary, nil
}

// ============================================================================
// UNCONTROLLED PAIN ALERTS
// ============================================================================

type PainAlert struct {
	PatientID         int64
	PatientName       string
	PainIntensity     int
	PainLocation      []string
	HoursSinceReport  float64
	InterventionEffectiveness int
}

func (epm *ExitProtocolManager) GetUncontrolledPainAlerts() ([]PainAlert, error) {
	query := `
		SELECT
			patient_id, patient_name, pain_intensity,
			hours_since_report, intervention_effectiveness
		FROM v_uncontrolled_pain_alerts
		ORDER BY pain_intensity DESC, hours_since_report DESC
	`

	rows, err := epm.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts := []PainAlert{}
	for rows.Next() {
		var alert PainAlert
		var interventionEff sql.NullInt64

		err := rows.Scan(
			&alert.PatientID,
			&alert.PatientName,
			&alert.PainIntensity,
			&alert.HoursSinceReport,
			&interventionEff,
		)
		if err != nil {
			continue
		}

		if interventionEff.Valid {
			alert.InterventionEffectiveness = int(interventionEff.Int64)
		}

		alerts = append(alerts, alert)
	}

	return alerts, nil
}
