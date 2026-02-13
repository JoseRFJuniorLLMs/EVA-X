package attention

import (
	"context"
	"eva-mind/internal/cortex/attention/models"
	"time"
)

// Executive - Layer executivo Gurdjieffiano
type Executive struct {
	tripleAttention  *TripleAttention
	affectStabilizer *AffectStabilizer
	patternInterrupt *PatternInterrupt
	centerRouter     *CenterRouter
	minimalOptimizer *MinimalOptimizer
	confidenceGate   *ConfidenceGate

	config *Config
}

// Config - Configuração da personalidade
type Config struct {
	// Thresholds
	ConfidenceThreshold     float64 `yaml:"confidence_threshold"`
	LoopSimilarityThreshold float64 `yaml:"loop_similarity_threshold"`
	MaxResponseTokens       int     `yaml:"max_response_tokens"`

	// Behavioral
	Temperature             float64 `yaml:"temperature"`
	EmotionalMirroring      bool    `yaml:"emotional_mirroring"`
	CenterMatching          bool    `yaml:"center_matching"`
	PatternInterruptEnabled bool    `yaml:"pattern_interrupt_enabled"`

	// Memory
	WorkingMemorySize int `yaml:"working_memory_size"`
	PatternBufferSize int `yaml:"pattern_buffer_size"`
}

// NewExecutive - Construtor
func NewExecutive(config *Config) *Executive {
	return &Executive{
		tripleAttention:  NewTripleAttention(),
		affectStabilizer: NewAffectStabilizer(),
		patternInterrupt: NewPatternInterrupt(config.LoopSimilarityThreshold),
		centerRouter:     NewCenterRouter(),
		minimalOptimizer: NewMinimalOptimizer(config.MaxResponseTokens),
		confidenceGate:   NewConfidenceGate(config.ConfidenceThreshold),
		config:           config,
	}
}

// Process - Pipeline principal
func (e *Executive) Process(
	ctx context.Context,
	userInput string,
	state *models.ExecutiveState,
) (*ExecutiveDecision, error) {

	// 1. TRIPLA ATENÇÃO
	// Observa: usuário + tarefa + self
	attention := e.tripleAttention.Observe(userInput, state)

	// 2. ATUALIZA MODELO DO USUÁRIO
	userModel := e.buildUserModel(userInput, state)
	state.UserState = userModel

	// 3. DETECTA CENTRO ATIVO
	center := e.centerRouter.DetectCenter(userInput, userModel)

	// 4. VERIFICA PADRÕES (LOOP DETECTION)
	loopDetected := false
	if e.config.PatternInterruptEnabled {
		loopDetected = e.patternInterrupt.DetectLoop(userInput, state)
		state.LoopDetected = loopDetected
	}

	// 5. AVALIA CONFIANÇA
	confidence := e.assessConfidence(userInput, state)
	state.ConfidenceScore = confidence

	// 6. DECISÃO EXECUTIVA
	decision := &ExecutiveDecision{
		ShouldRespond:     true,
		ResponseStrategy:  e.determineStrategy(center, loopDetected, confidence),
		ActiveCenter:      center,
		LoopDetected:      loopDetected,
		Confidence:        confidence,
		AffectiveBaseline: models.AffectNeutralClear,
		Attention:         attention,
		MaxTokens:         e.minimalOptimizer.OptimizeLength(state.UserState.CognitiveLoad, state.UserState.IntentClarity),
		Temperature:       e.config.Temperature,
	}

	// 7. GATE DE CONFIANÇA
	if !e.confidenceGate.ShouldProceed(confidence) {
		decision.ResponseStrategy = StrategyClarifyingQuestion
		decision.ClarificationNeeded = true
	}

	// 8. INTERRUPÇÃO DE PADRÃO
	if loopDetected {
		decision.ResponseStrategy = StrategyPatternInterrupt
		decision.InterruptionQuestion = e.patternInterrupt.GenerateInterruption(state)
	}

	// 9. ATUALIZA ESTADO
	e.updateState(state, userInput, decision)

	return decision, nil
}

// ExecutiveDecision - Decisão do executive layer
type ExecutiveDecision struct {
	ShouldRespond        bool
	ResponseStrategy     Strategy
	ActiveCenter         models.Center
	LoopDetected         bool
	Confidence           float64
	AffectiveBaseline    models.AffectState
	ClarificationNeeded  bool
	InterruptionQuestion string
	Attention            *AttentionOutput
	MaxTokens            int
	Temperature          float64
}

// Strategy - Estratégia de resposta
type Strategy string

const (
	StrategyDirect               Strategy = "direct"
	StrategyClarifyingQuestion   Strategy = "clarifying_question"
	StrategyPatternInterrupt     Strategy = "pattern_interrupt"
	StrategyEmotionalContainment Strategy = "emotional_containment"
	StrategyAnalytical           Strategy = "analytical"
	StrategyActionable           Strategy = "actionable"
)

// buildUserModel - Constrói modelo do usuário
func (e *Executive) buildUserModel(
	input string,
	state *models.ExecutiveState,
) *models.UserModel {

	return &models.UserModel{
		ActiveCenter:   e.centerRouter.DetectCenter(input, state.UserState),
		EmotionalTone:  e.detectEmotionalTone(input),
		CognitiveLoad:  e.estimateCognitiveLoad(input),
		IntentClarity:  e.assessIntentClarity(input),
		SemanticVector: generateEmbedding(input),
	}
}

// determineStrategy - Determina estratégia baseada em centro + contexto
func (e *Executive) determineStrategy(
	center models.Center,
	loopDetected bool,
	confidence float64,
) Strategy {

	if loopDetected {
		return StrategyPatternInterrupt
	}

	if confidence < e.config.ConfidenceThreshold {
		return StrategyClarifyingQuestion
	}

	if e.config.CenterMatching {
		switch center {
		case models.CenterEmotional:
			return StrategyEmotionalContainment
		case models.CenterIntellectual:
			return StrategyAnalytical
		case models.CenterMotor:
			return StrategyActionable
		}
	}

	return StrategyDirect
}

// updateState - Atualiza estado metacognitivo
func (e *Executive) updateState(
	state *models.ExecutiveState,
	userInput string,
	decision *ExecutiveDecision,
) {
	state.TurnNumber++
	state.Timestamp = time.Now()

	// Adiciona à working memory
	frame := models.ContextFrame{
		Turn:      state.TurnNumber,
		UserInput: userInput,
		State:     state.UserState,
	}

	state.WorkingMemory = append(state.WorkingMemory, frame)

	// Limita tamanho da working memory
	if len(state.WorkingMemory) > e.config.WorkingMemorySize {
		state.WorkingMemory = state.WorkingMemory[1:]
	}

	// Adiciona ao pattern buffer
	hash := models.SemanticHash{
		Vector:    state.UserState.SemanticVector,
		Timestamp: time.Now(),
		Content:   userInput,
	}

	state.PatternBuffer = append(state.PatternBuffer, hash)

	if len(state.PatternBuffer) > e.config.PatternBufferSize {
		state.PatternBuffer = state.PatternBuffer[1:]
	}
}

// Funções auxiliares (placeholders - você plugaria seus modelos reais)

func (e *Executive) detectEmotionalTone(input string) models.EmotionalState {
	return models.EmotionalState{
		Valence:   0.0,
		Arousal:   0.5,
		Intensity: 0.3,
	}
}

func (e *Executive) estimateCognitiveLoad(input string) float64 {
	return 0.5
}

func (e *Executive) assessIntentClarity(input string) float64 {
	return 0.7
}

func (e *Executive) assessConfidence(
	input string,
	state *models.ExecutiveState,
) float64 {
	clarity := e.assessIntentClarity(input)
	contextDepth := 0.5
	if e.config.WorkingMemorySize > 0 {
		contextDepth = float64(len(state.WorkingMemory)) / float64(e.config.WorkingMemorySize)
	}

	return e.confidenceGate.AssessConfidence(clarity, contextDepth, 0.3)
}
