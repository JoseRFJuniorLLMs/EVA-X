package attention

import (
	"eva-mind/internal/cortex/attention/models"
	"fmt"
)

// AttentionOutput - Saída da tripla atenção
type AttentionOutput struct {
	UserFocus    string // O que o usuário precisa
	TaskFocus    string // Qual o objetivo
	ProcessFocus string // Como estou processando
	Synthesis    string // Síntese das três
}

// TripleAttention - Implementa atenção tripla simultânea
type TripleAttention struct{}

func NewTripleAttention() *TripleAttention {
	return &TripleAttention{}
}

// Observe - Observa simultaneamente: usuário, tarefa, self
func (ta *TripleAttention) Observe(
	input string,
	state *models.ExecutiveState,
) *AttentionOutput {

	return &AttentionOutput{
		UserFocus:    ta.observeUser(input, state),
		TaskFocus:    ta.observeTask(input, state),
		ProcessFocus: ta.observeSelf(state),
		Synthesis:    ta.synthesize(input, state),
	}
}

func (ta *TripleAttention) observeUser(
	input string,
	state *models.ExecutiveState,
) string {
	// Análise do estado do usuário
	if state.UserState == nil {
		return "Usuário iniciando interação"
	}

	// Exemplo de análise
	return fmt.Sprintf(
		"Usuário em centro %s, tom emocional valence=%.2f",
		state.UserState.ActiveCenter,
		state.UserState.EmotionalTone.Valence,
	)
}

func (ta *TripleAttention) observeTask(
	input string,
	state *models.ExecutiveState,
) string {
	// Análise do objetivo
	if state.TaskGoal == nil {
		return "Objetivo: responder query"
	}

	return fmt.Sprintf("Objetivo: %s", state.TaskGoal.Primary)
}

func (ta *TripleAttention) observeSelf(
	state *models.ExecutiveState,
) string {
	// Auto-observação do processamento
	if state.SelfProcess == nil {
		return "Processando inicialmente"
	}

	return fmt.Sprintf(
		"Processamento: %d etapas, %d pontos de decisão",
		len(state.SelfProcess.ProcessingSteps),
		len(state.SelfProcess.DecisionPoints),
	)
}

func (ta *TripleAttention) synthesize(
	input string,
	state *models.ExecutiveState,
) string {
	// Síntese das três atenções
	return "Presença metacognitiva ativa"
}
