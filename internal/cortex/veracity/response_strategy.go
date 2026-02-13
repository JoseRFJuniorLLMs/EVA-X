package veracity

import (
	"eva-mind/internal/cortex/transnar"
	"fmt"
	"strings"
)

// ResponseStrategy estratégia de resposta a inconsistências
type ResponseStrategy string

const (
	SoftConfrontation ResponseStrategy = "soft_confrontation" // Confrontar suavemente
	Exploration       ResponseStrategy = "exploration"        // Explorar mais
	Validation        ResponseStrategy = "validation"         // Validar com terceiros
	Ignore            ResponseStrategy = "ignore"             // Ignorar (baixa confiança)
)

// ResponseGenerator gera respostas apropriadas para inconsistências
type ResponseGenerator struct{}

// NewResponseGenerator cria um novo gerador
func NewResponseGenerator() *ResponseGenerator {
	return &ResponseGenerator{}
}

// SelectStrategy seleciona estratégia baseada na inconsistência
func (g *ResponseGenerator) SelectStrategy(inc *Inconsistency) ResponseStrategy {
	// Ignorar se confiança muito baixa
	if inc.Confidence < 0.6 {
		return Ignore
	}

	// Validar casos críticos
	if inc.Severity == SeverityCritical {
		return Validation
	}

	// Confrontar suavemente se alta confiança
	if inc.ShouldConfront() {
		return SoftConfrontation
	}

	// Explorar demais casos
	return Exploration
}

// GenerateResponse gera resposta apropriada
func (g *ResponseGenerator) GenerateResponse(
	inc *Inconsistency,
	strategy ResponseStrategy,
) string {

	switch strategy {
	case SoftConfrontation:
		return g.generateSoftConfrontation(inc)

	case Exploration:
		return g.generateExploration(inc)

	case Validation:
		return g.generateValidation(inc)

	case Ignore:
		return "" // Não responder
	}

	return ""
}

// generateSoftConfrontation gera confrontação suave
func (g *ResponseGenerator) generateSoftConfrontation(inc *Inconsistency) string {
	switch inc.Type {
	case DirectContradiction:
		if len(inc.GraphEvidence) > 0 {
			return fmt.Sprintf(
				"Interessante... Lembro que você mencionou '%s' antes. "+
					"Algo mudou desde então?",
				inc.GraphEvidence[0].Fact,
			)
		}
		return "Deixa eu confirmar... Você tem certeza disso?"

	case TemporalInconsistency:
		return "Deixa eu confirmar... Quando exatamente isso aconteceu?"

	case EmotionalInconsistency:
		if len(inc.GraphEvidence) > 0 {
			emotion := extractEmotion(inc.GraphEvidence[0].Fact)
			return fmt.Sprintf(
				"Você mencionou '%s' algumas vezes recentemente. "+
					"Tem certeza que não está sentindo isso agora?",
				emotion,
			)
		}
		return "Percebo que você pode estar sentindo algo diferente do que está dizendo..."

	case NarrativeGap:
		return "Pode me contar mais sobre isso? Sinto que falta algum detalhe importante."

	case BehavioralChange:
		return "Isso é diferente do seu padrão usual. O que mudou?"
	}

	return "Pode me explicar melhor?"
}

// generateExploration gera pergunta exploratória
func (g *ResponseGenerator) generateExploration(inc *Inconsistency) string {
	templates := []string{
		"Pode me contar mais sobre isso?",
		"O que você quer dizer exatamente?",
		"Me ajuda a entender melhor...",
		"Tem algo mais que você gostaria de compartilhar?",
	}

	// Selecionar template baseado no tipo
	index := int(inc.Type[0]) % len(templates)
	return templates[index]
}

// generateValidation gera pedido de validação
func (g *ResponseGenerator) generateValidation(inc *Inconsistency) string {
	return "Vou verificar isso com seu cuidador para ter certeza, ok? " +
		"É importante que eu tenha as informações corretas para te ajudar melhor."
}

// InferDesireFromLie infere desejo latente por trás da mentira
func (g *ResponseGenerator) InferDesireFromLie(inc *Inconsistency) transnar.DesireType {
	switch inc.Type {
	case DirectContradiction:
		// Negar fato geralmente indica desejo de autonomia ou controle
		if strings.Contains(strings.ToLower(inc.Statement), "não tomei") ||
			strings.Contains(strings.ToLower(inc.Statement), "não fiz") {
			return transnar.DesireAutonomy
		}

	case EmotionalInconsistency:
		// Negar emoção indica desejo de reconhecimento (parecer forte)
		return transnar.DesireRecognition

	case NarrativeGap:
		// Omitir informação grave indica desejo de alívio (negar realidade)
		if inc.Severity == SeverityCritical {
			return transnar.DesireRelief
		}

	case BehavioralChange:
		// Mudança de comportamento pode indicar busca de autonomia
		return transnar.DesireAutonomy
	}

	return transnar.DesireUnknown
}

// GeneratePromptAddendum gera adendo para prompt do LLM
func (g *ResponseGenerator) GeneratePromptAddendum(
	inconsistencies []Inconsistency,
) string {

	if len(inconsistencies) == 0 {
		return ""
	}

	prompt := "\n[ALERTA DE INCONSISTÊNCIA DETECTADA]\n\n"

	for i, inc := range inconsistencies {
		prompt += fmt.Sprintf(
			"%d. %s (%.0f%% confiança)\n"+
				"   Afirmação: \"%s\"\n"+
				"   Evidência: %s\n"+
				"   Gravidade: %s\n\n",
			i+1,
			inc.Type.GetDescription(),
			inc.Confidence*100,
			inc.Statement,
			inc.GraphEvidence[0].Fact,
			inc.Severity,
		)
	}

	prompt += "INSTRUÇÕES:\n"
	prompt += "- NÃO acuse o usuário de mentir\n"
	prompt += "- Use confrontação suave ou exploração\n"
	prompt += "- Considere que pode ser memória imprecisa\n"
	prompt += "- Se crítico, valide com cuidador\n"

	return prompt
}

// extractEmotion extrai nome da emoção de um fato
func extractEmotion(fact string) string {
	emotions := []string{
		"medo", "tristeza", "solidão", "ansiedade",
		"preocupação", "raiva", "culpa", "vergonha",
	}

	for _, emotion := range emotions {
		if strings.Contains(strings.ToLower(fact), emotion) {
			return emotion
		}
	}

	return "emoção"
}
