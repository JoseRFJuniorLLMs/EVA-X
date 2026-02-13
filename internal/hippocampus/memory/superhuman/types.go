// Package superhuman implements the 12 memory systems for EVA
// PRINCIPLE: EVA has no ego. EVA is a mirror.
// All memory is about the PATIENT, not about EVA.
package superhuman

import (
	"time"
)

// =====================================================
// ENNEAGRAM TYPES (Gurdjieff/Naranjo)
// =====================================================

// EnneagramType represents one of the 9 Enneagram types
type EnneagramType struct {
	ID                     int      `json:"id"`
	Name                   string   `json:"name"`
	NamePT                 string   `json:"name_pt"`
	Center                 string   `json:"center"`    // instinctive, emotional, mental
	CenterPT               string   `json:"center_pt"` // instintivo, emocional, mental
	RootEmotion            string   `json:"root_emotion"`
	RootEmotionPT          string   `json:"root_emotion_pt"`
	ChiefFeature           string   `json:"chief_feature"`
	ChiefFeaturePT         string   `json:"chief_feature_pt"`
	DefenseMechanism       string   `json:"defense_mechanism"`
	DefenseMechanismPT     string   `json:"defense_mechanism_pt"`
	Fixation               string   `json:"fixation"`
	FixationPT             string   `json:"fixation_pt"`
	Passion                string   `json:"passion"`
	PassionPT              string   `json:"passion_pt"`
	Virtue                 string   `json:"virtue"`
	VirtuePT               string   `json:"virtue_pt"`
	IntegrationDirection   int      `json:"integration_direction"`
	DisintegrationDirection int     `json:"disintegration_direction"`
	Keywords               []string `json:"keywords"`
	KeywordsPT             []string `json:"keywords_pt"`
}

// PatientEnneagram holds the patient's Enneagram assessment
type PatientEnneagram struct {
	IdosoID               int64              `json:"idoso_id"`
	PrimaryType           int                `json:"primary_type"`
	PrimaryTypeConfidence float64            `json:"primary_type_confidence"`
	DominantWing          int                `json:"dominant_wing"`
	WingInfluence         float64            `json:"wing_influence"`
	HealthLevel           int                `json:"health_level"` // 1-9 (1 = healthiest)
	InstinctualVariant    string             `json:"instinctual_variant"`
	TypeScores            map[string]float64 `json:"type_scores"`
	EvidenceCount         int                `json:"evidence_count"`
	LastEvidenceAt        time.Time          `json:"last_evidence_at"`
	IdentificationMethod  string             `json:"identification_method"`
	IdentifiedAt          time.Time          `json:"identified_at"`
}

// EnneagramEvidence represents a piece of evidence for Enneagram typing
type EnneagramEvidence struct {
	ID            int64     `json:"id"`
	IdosoID       int64     `json:"idoso_id"`
	MemoryID      int64     `json:"memory_id,omitempty"`
	Verbatim      string    `json:"verbatim"`
	SuggestedType int       `json:"suggested_type"`
	Weight        float64   `json:"weight"`
	Category      string    `json:"category"`
	Context       string    `json:"context,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// =====================================================
// SELF-CORE (Identity Memory)
// =====================================================

// PatientSelfCore holds the patient's identity as THEY describe it
type PatientSelfCore struct {
	IdosoID              int64             `json:"idoso_id"`
	SelfDescriptions     []SelfDescription `json:"self_descriptions"`
	SelfAttributedRoles  []string          `json:"self_attributed_roles"`
	NarrativeSummary     string            `json:"narrative_summary"`
	NarrativeLastUpdated time.Time         `json:"narrative_last_updated"`
	SelfConceptTimeline  []SelfConceptPeriod `json:"self_concept_timeline"`
}

// SelfDescription captures a self-description with context
type SelfDescription struct {
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Context   string    `json:"context"`
}

// SelfConceptPeriod represents how patient described themselves in a period
type SelfConceptPeriod struct {
	Period        string   `json:"period"`
	Descriptions  []string `json:"descriptions"`
	DominantTheme string   `json:"dominant_theme"`
}

// MasterSignifier represents a word/phrase the patient uses repeatedly
type MasterSignifier struct {
	IdosoID               int64              `json:"idoso_id"`
	Signifier             string             `json:"signifier"`
	ContextType           string             `json:"context_type"` // self, other, world
	TotalCount            int                `json:"total_count"`
	FirstSeen             time.Time          `json:"first_seen"`
	LastSeen              time.Time          `json:"last_seen"`
	FrequencyByPeriod     map[string]int     `json:"frequency_by_period"`
	AvgEmotionalValence   float64            `json:"avg_emotional_valence"`
	CoOccurringSignifiers []string           `json:"co_occurring_signifiers"`
}

// =====================================================
// BEHAVIORAL PATTERNS (Procedural Memory)
// =====================================================

// BehavioralPattern represents an automatic pattern of the patient
type BehavioralPattern struct {
	ID                    int64              `json:"id"`
	IdosoID               int64              `json:"idoso_id"`
	PatternType           string             `json:"pattern_type"`
	PatternName           string             `json:"pattern_name"`
	TriggerCondition      TriggerCondition   `json:"trigger_condition"`
	TypicalResponse       PatternResponse    `json:"typical_response"`
	OccurrenceCount       int                `json:"occurrence_count"`
	Probability           float64            `json:"probability"`
	LearnedFromCount      int                `json:"learned_from_count"`
	FirstObserved         time.Time          `json:"first_observed"`
	LastObserved          time.Time          `json:"last_observed"`
	EffectiveInterventions []string          `json:"effective_interventions"`
}

// TriggerCondition defines when a pattern is activated
type TriggerCondition struct {
	Type    string `json:"type"`    // topic, person, time, emotion
	Value   string `json:"value"`
	Context string `json:"context,omitempty"`
}

// PatternResponse defines typical patient response
type PatternResponse struct {
	Behavior  string `json:"behavior"`
	Verbal    string `json:"verbal,omitempty"`
	Nonverbal string `json:"nonverbal,omitempty"`
}

// CircadianPattern represents time-based psychological patterns
type CircadianPattern struct {
	IdosoID          int64    `json:"idoso_id"`
	TimePeriod       string   `json:"time_period"`
	DayOfWeek        *int     `json:"day_of_week,omitempty"`
	RecurringThemes  []string `json:"recurring_themes"`
	AvgEmotionalTone float64  `json:"avg_emotional_tone"`
	TypicalState     string   `json:"typical_state"`
	ObservationCount int      `json:"observation_count"`
}

// =====================================================
// PROSPECTIVE MEMORY (Intentions)
// =====================================================

// PatientIntention tracks declared intentions vs actions
type PatientIntention struct {
	ID                 int64     `json:"id"`
	IdosoID            int64     `json:"idoso_id"`
	IntentionVerbatim  string    `json:"intention_verbatim"`
	Category           string    `json:"category"`
	RelatedPerson      string    `json:"related_person,omitempty"`
	Status             string    `json:"status"`
	DeclarationCount   int       `json:"declaration_count"`
	FirstDeclared      time.Time `json:"first_declared"`
	LastDeclared       time.Time `json:"last_declared"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	StatedBlocker      string    `json:"stated_blocker,omitempty"`
}

// =====================================================
// COUNTERFACTUAL MEMORY ("What if?")
// =====================================================

// PatientCounterfactual represents a "what if" the patient ruminates about
type PatientCounterfactual struct {
	ID                  int64     `json:"id"`
	IdosoID             int64     `json:"idoso_id"`
	Verbatim            string    `json:"verbatim"`
	LifePeriod          string    `json:"life_period,omitempty"`
	ApproximateAge      *int      `json:"approximate_age,omitempty"`
	Theme               string    `json:"theme,omitempty"`
	MentionCount        int       `json:"mention_count"`
	FirstMentioned      time.Time `json:"first_mentioned"`
	LastMentioned       time.Time `json:"last_mentioned"`
	AvgPitchVariance    float64   `json:"avg_pitch_variance,omitempty"`
	AvgPauseDuration    float64   `json:"avg_pause_duration,omitempty"`
	VoiceTremorDetected bool      `json:"voice_tremor_detected"`
	AvgEmotionalValence float64   `json:"avg_emotional_valence"`
	CorrelatedTopics    []string  `json:"correlated_topics"`
	CorrelatedPersons   []string  `json:"correlated_persons"`
}

// =====================================================
// METAPHORICAL MEMORY
// =====================================================

// PatientMetaphor represents a metaphor in the patient's personal dictionary
type PatientMetaphor struct {
	ID                 int64                  `json:"id"`
	IdosoID            int64                  `json:"idoso_id"`
	Metaphor           string                 `json:"metaphor"`
	MetaphorType       string                 `json:"metaphor_type"`
	UsageCount         int                    `json:"usage_count"`
	FirstUsed          time.Time              `json:"first_used"`
	LastUsed           time.Time              `json:"last_used"`
	Contexts           []MetaphorContext      `json:"contexts"`
	CorrelatedTopics   []string               `json:"correlated_topics"`
	CorrelatedEmotions []string               `json:"correlated_emotions"`
	CorrelatedPersons  []string               `json:"correlated_persons"`
	AvgProsodicData    map[string]interface{} `json:"avg_prosodic_data"`
}

// MetaphorContext captures context where metaphor was used
type MetaphorContext struct {
	Topic     string `json:"topic,omitempty"`
	Person    string `json:"person,omitempty"`
	TimeOfDay string `json:"time_of_day,omitempty"`
}

// =====================================================
// TRANSGENERATIONAL MEMORY
// =====================================================

// FamilyPattern represents patterns the patient describes about their family
type FamilyPattern struct {
	ID                   int64     `json:"id"`
	IdosoID              int64     `json:"idoso_id"`
	PatternVerbatim      string    `json:"pattern_verbatim"`
	PatternType          string    `json:"pattern_type"`
	GenerationsMentioned []string  `json:"generations_mentioned"`
	MentionCount         int       `json:"mention_count"`
	FirstMentioned       time.Time `json:"first_mentioned"`
	LastMentioned        time.Time `json:"last_mentioned"`
	AvgEmotionalValence  float64   `json:"avg_emotional_valence"`
}

// =====================================================
// SOMATIC MEMORY
// =====================================================

// SomaticCorrelation represents body-speech correlations
type SomaticCorrelation struct {
	ID                  int64     `json:"id"`
	IdosoID             int64     `json:"idoso_id"`
	SomaticType         string    `json:"somatic_type"`
	ConditionRange      string    `json:"condition_range"`
	CorrelatedTopic     string    `json:"correlated_topic"`
	CorrelationStrength float64   `json:"correlation_strength"`
	ObservationCount    int       `json:"observation_count"`
	Direction           string    `json:"direction"`
	FirstObserved       time.Time `json:"first_observed"`
	LastObserved        time.Time `json:"last_observed"`
}

// =====================================================
// CULTURAL CONTEXT MEMORY
// =====================================================

// CulturalContext holds patient's historical/cultural context
type CulturalContext struct {
	IdosoID                    int64    `json:"idoso_id"`
	BirthYear                  int      `json:"birth_year,omitempty"`
	BirthRegion                string   `json:"birth_region,omitempty"`
	MentionedHistoricalEvents  []string `json:"mentioned_historical_events"`
	ExpressedGenerationalValues []string `json:"expressed_generational_values"`
	GenerationalExpressions    []string `json:"generational_expressions"`
	ExpressedValueConflicts    []string `json:"expressed_value_conflicts"`
}

// =====================================================
// LEARNING MEMORY (What works with this patient)
// =====================================================

// EffectiveApproach tracks what works with this specific patient
type EffectiveApproach struct {
	ID                   int64              `json:"id"`
	IdosoID              int64              `json:"idoso_id"`
	ApproachType         string             `json:"approach_type"`
	ApproachDescription  string             `json:"approach_description"`
	EffectivenessMetric  string             `json:"effectiveness_metric"`
	EffectivenessScore   float64            `json:"effectiveness_score"`
	ObservationCount     int                `json:"observation_count"`
	Conditions           map[string]string  `json:"conditions"`
}

// OptimalSilence tracks optimal silence duration for this patient
type OptimalSilence struct {
	IdosoID               int64   `json:"idoso_id"`
	ContextType           string  `json:"context_type"`
	OptimalDurationSecs   float64 `json:"optimal_duration_seconds"`
	MinEffectiveSecs      float64 `json:"min_effective_seconds"`
	MaxEffectiveSecs      float64 `json:"max_effective_seconds"`
	EffectivenessScore    float64 `json:"effectiveness_score"`
	ObservationCount      int     `json:"observation_count"`
}

// =====================================================
// CRISIS PREDICTION
// =====================================================

// CrisisPredictor represents a marker that predicts crisis
type CrisisPredictor struct {
	ID                   int64   `json:"id"`
	IdosoID              int64   `json:"idoso_id"`
	CrisisType           string  `json:"crisis_type"`
	PredictorType        string  `json:"predictor_type"`
	PredictorDescription string  `json:"predictor_description"`
	PredictiveWeight     float64 `json:"predictive_weight"`
	LeadTimeDays         int     `json:"lead_time_days"`
	BasedOnCrisisCount   int     `json:"based_on_crisis_count"`
	HistoricalAccuracy   float64 `json:"historical_accuracy"`
}

// RiskScore holds calculated risk scores
type RiskScore struct {
	ID                    int64     `json:"id"`
	IdosoID               int64     `json:"idoso_id"`
	CalculatedAt          time.Time `json:"calculated_at"`
	RiskDepressionSevere  float64   `json:"risk_depression_severe"`
	RiskSuicidal30D       float64   `json:"risk_suicidal_30d"`
	RiskHospitalization90D float64  `json:"risk_hospitalization_90d"`
	RiskSocialIsolation   float64   `json:"risk_social_isolation"`
	OverallRiskScore      float64   `json:"overall_risk_score"`
	AlertLevel            string    `json:"alert_level"`
	ActiveMarkers         []string  `json:"active_markers"`
	RecommendedAction     string    `json:"recommended_action,omitempty"`
}

// =====================================================
// SEMANTIC MEMORY (Patient's World)
// =====================================================

// WorldPerson represents a person in the patient's world
type WorldPerson struct {
	ID                  int64               `json:"id"`
	IdosoID             int64               `json:"idoso_id"`
	PersonName          string              `json:"person_name"`
	Role                string              `json:"role,omitempty"`
	EmotionalValence    float64             `json:"emotional_valence"`
	MentionCount        int                 `json:"mention_count"`
	FirstMentioned      time.Time           `json:"first_mentioned"`
	LastMentioned       time.Time           `json:"last_mentioned"`
	AssociatedTopics    []string            `json:"associated_topics"`
	RelationshipTimeline []RelationshipEntry `json:"relationship_timeline"`
	CurrentStatus       string              `json:"current_status,omitempty"`
}

// RelationshipEntry tracks relationship evolution
type RelationshipEntry struct {
	Period      string  `json:"period"`
	Description string  `json:"description"`
	Valence     float64 `json:"valence"`
}

// WorldPlace represents a place in the patient's world
type WorldPlace struct {
	ID                 int64     `json:"id"`
	IdosoID            int64     `json:"idoso_id"`
	PlaceName          string    `json:"place_name"`
	PlaceType          string    `json:"place_type,omitempty"`
	EmotionalValence   float64   `json:"emotional_valence"`
	TemporalPeriod     string    `json:"temporal_period,omitempty"`
	MentionCount       int       `json:"mention_count"`
	AssociatedMemoryIDs []int64  `json:"associated_memory_ids"`
}

// WorldObject represents a significant object in patient's world
type WorldObject struct {
	ID                   int64   `json:"id"`
	IdosoID              int64   `json:"idoso_id"`
	ObjectName           string  `json:"object_name"`
	DescribedSignificance string `json:"described_significance,omitempty"`
	MentionCount         int     `json:"mention_count"`
	AssociatedPerson     string  `json:"associated_person,omitempty"`
	AssociatedEvent      string  `json:"associated_event,omitempty"`
	EmotionalValence     float64 `json:"emotional_valence"`
}

// =====================================================
// MIRROR OUTPUT (Lacanian)
// =====================================================

// MirrorOutput represents objective data to present to patient
type MirrorOutput struct {
	Type        string                 `json:"type"` // pattern, statistic, correlation, reflection
	DataPoints  []string               `json:"data_points"`
	Frequency   *int                   `json:"frequency,omitempty"`
	TimeRange   *TimeRange             `json:"time_range,omitempty"`
	Question    string                 `json:"question"` // Always ends with a question for patient
	RawData     map[string]interface{} `json:"raw_data,omitempty"`
}

// TimeRange for mirror outputs
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
