package models

import "time"

// NewExecutiveState - Creates a new executive state
func NewExecutiveState(conversationID string, turnNumber int) *ExecutiveState {
	return &ExecutiveState{
		ConversationID: conversationID,
		TurnNumber:     turnNumber,
		Timestamp:      time.Now(),
		UserState: &UserModel{
			ActiveCenter: CenterUnknown,
		},
		WorkingMemory:   make([]ContextFrame, 0),
		PatternBuffer:   make([]SemanticHash, 0),
		AffectiveState:  AffectNeutralClear,
		ConfidenceScore: 1.0, // Start confident
	}
}

// ExecutiveState - Estado metacognitivo completo
type ExecutiveState struct {
	// Tripla Atenção
	UserState   *UserModel `json:"user_state"`
	TaskGoal    *Goal      `json:"task_goal"`
	SelfProcess *MetaTrace `json:"self_process"`

	// Histórico
	ConversationID string         `json:"conversation_id"`
	TurnNumber     int            `json:"turn_number"`
	WorkingMemory  []ContextFrame `json:"working_memory"`

	// Estado Afetivo (sempre baseline)
	AffectiveState AffectState `json:"affective_state"`

	// Detecção de Padrões
	PatternBuffer []SemanticHash `json:"pattern_buffer"`
	LoopDetected  bool           `json:"loop_detected"`

	// Confiança
	ConfidenceScore float64 `json:"confidence_score"`

	Timestamp time.Time `json:"timestamp"`
}

// UserModel - Modelo do estado do usuário
type UserModel struct {
	ActiveCenter   Center         `json:"active_center"`
	EmotionalTone  EmotionalState `json:"emotional_tone"`
	CognitiveLoad  float64        `json:"cognitive_load"`
	IntentClarity  float64        `json:"intent_clarity"`
	SemanticVector []float64      `json:"semantic_vector"`
}

// Goal - Objetivo da interação
type Goal struct {
	Primary         string   `json:"primary"`
	Constraints     []string `json:"constraints"`
	SuccessCriteria string   `json:"success_criteria"`
}

// MetaTrace - Traço do próprio processamento
type MetaTrace struct {
	ProcessingSteps  []string       `json:"processing_steps"`
	DecisionPoints   []DecisionNode `json:"decision_points"`
	UncertaintyAreas []string       `json:"uncertainty_areas"`
}

// Center - Os três centros de Gurdjieff
type Center string

const (
	CenterIntellectual Center = "intellectual"
	CenterEmotional    Center = "emotional"
	CenterMotor        Center = "motor"
	CenterUnknown      Center = "unknown"
)

// AffectState - Estado afetivo (sempre neutro)
type AffectState string

const (
	AffectNeutralClear AffectState = "neutral_clear"
	AffectObservant    AffectState = "observant"
	AffectPresent      AffectState = "present"
)

// EmotionalState - Estado emocional do usuário
type EmotionalState struct {
	Valence   float64 `json:"valence"`   // -1 a +1
	Arousal   float64 `json:"arousal"`   // 0 a 1
	Intensity float64 `json:"intensity"` // 0 a 1
}

// SemanticHash - Hash semântico para detecção de loops
type SemanticHash struct {
	Vector    []float64 `json:"vector"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
}

// ContextFrame - Frame de contexto na working memory
type ContextFrame struct {
	Turn      int        `json:"turn"`
	UserInput string     `json:"user_input"`
	Response  string     `json:"response"`
	State     *UserModel `json:"state"`
}

// DecisionNode - Nó de decisão no processamento
type DecisionNode struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Chosen   string   `json:"chosen"`
	Reason   string   `json:"reason"`
}
