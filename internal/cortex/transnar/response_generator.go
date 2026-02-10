package transnar

import (
	"fmt"
	"strings"
)

// ResponseStrategy define estratégias de resposta baseadas em Lacan
type ResponseStrategy string

const (
	// Interpellation - Questionar o significante
	Interpellation ResponseStrategy = "interpellation"

	// Cut - Interromper cadeia de repetição
	Cut ResponseStrategy = "cut"

	// Punctuation - Dar sentido/fechar significado
	Punctuation ResponseStrategy = "punctuation"

	// Reflection - Espelhar emoção (Rogers)
	Reflection ResponseStrategy = "reflection"

	// Neutral - Resposta neutra (fallback)
	Neutral ResponseStrategy = "neutral"
)

// ResponseGenerator gera respostas interpretativas
type ResponseGenerator struct{}

// NewResponseGenerator cria um novo gerador de respostas
func NewResponseGenerator() *ResponseGenerator {
	return &ResponseGenerator{}
}

// SelectStrategy seleciona a melhor estratégia baseada no desejo e confiança
func (g *ResponseGenerator) SelectStrategy(
	desire *DesireInference,
	chain *SignifierChain,
) ResponseStrategy {

	// Se confiança baixa, usar resposta neutra
	if desire.Confidence < 0.5 {
		return Neutral
	}

	// Se alta intensidade emocional, usar reflexão
	if chain.Intensity > 0.8 {
		return Reflection
	}

	// Se desejo de conexão, usar interpelação
	if desire.Desire == DesireConnection {
		return Interpellation
	}

	// Se desejo de segurança, usar pontuação
	if desire.Desire == DesireSecurity {
		return Punctuation
	}

	// Default: reflexão
	return Reflection
}

// GenerateSystemPrompt gera o prompt do sistema para o LLM
func (g *ResponseGenerator) GenerateSystemPrompt(
	desire *DesireInference,
	chain *SignifierChain,
	strategy ResponseStrategy,
) string {

	var prompt strings.Builder

	// Header
	prompt.WriteString("[MODO TRANSNAR ATIVO]\n\n")

	// Contexto da inferência
	prompt.WriteString(fmt.Sprintf("DESEJO LATENTE DETECTADO: %s (%.0f%% confiança)\n",
		GetDesireDescription(desire.Desire),
		desire.Confidence*100))

	prompt.WriteString(fmt.Sprintf("RACIOCÍNIO: %s\n\n", desire.Reasoning))

	// Estratégia selecionada
	prompt.WriteString(fmt.Sprintf("ESTRATÉGIA DE RESPOSTA: %s\n\n", strategy))

	// Instruções específicas por estratégia
	switch strategy {
	case Interpellation:
		prompt.WriteString(g.getInterpellationInstructions(chain))

	case Cut:
		prompt.WriteString(g.getCutInstructions())

	case Punctuation:
		prompt.WriteString(g.getPunctuationInstructions(desire))

	case Reflection:
		prompt.WriteString(g.getReflectionInstructions(chain))

	case Neutral:
		prompt.WriteString(g.getNeutralInstructions())
	}

	// Footer: regras gerais
	prompt.WriteString("\n\nREGRAS GERAIS:\n")
	prompt.WriteString("- NÃO responda apenas à demanda superficial\n")
	prompt.WriteString("- Endereça o DESEJO LATENTE identificado\n")
	prompt.WriteString("- Seja empática mas não bajuladora\n")
	prompt.WriteString("- Mantenha limites de segurança (anti-sycophancy)\n")

	return prompt.String()
}

func (g *ResponseGenerator) getInterpellationInstructions(chain *SignifierChain) string {
	var instructions strings.Builder

	instructions.WriteString("INTERPELAÇÃO (Questionar o Significante):\n")
	instructions.WriteString("Objetivo: Fazer o usuário refletir sobre o significante repetido.\n\n")

	if len(chain.Words) > 0 {
		instructions.WriteString(fmt.Sprintf("Palavras-chave detectadas: %s\n",
			strings.Join(chain.Words[:min(3, len(chain.Words))], ", ")))
	}

	instructions.WriteString("\nExemplo de interpelação:\n")
	instructions.WriteString("'Você mencionou [palavra] algumas vezes. O que essa palavra significa para você?'\n")

	return instructions.String()
}

func (g *ResponseGenerator) getCutInstructions() string {
	return `CORTE (Interromper Repetição):
Objetivo: Interromper loop de queixa ou repetição improdutiva.

Técnica:
- Redirecionar para perspectiva diferente
- Introduzir elemento novo na conversa
- Fazer pergunta que quebre o padrão

Exemplo:
"E se olharmos isso de outro ângulo? O que você gostaria que fosse diferente?"
`
}

func (g *ResponseGenerator) getPunctuationInstructions(desire *DesireInference) string {
	return fmt.Sprintf(`PONTUAÇÃO (Dar Sentido):
Objetivo: Fechar/dar sentido ao que foi dito, validando o desejo latente.

Desejo detectado: %s

Técnica:
- Reformular o que foi dito em termos do desejo
- Validar a emoção subjacente
- Oferecer segurança/clareza

Exemplo:
"Parece que você está dizendo que precisa de [desejo]. É isso?"
`, GetDesireDescription(desire.Desire))
}

func (g *ResponseGenerator) getReflectionInstructions(chain *SignifierChain) string {
	var instructions strings.Builder

	instructions.WriteString("REFLEXÃO (Espelhar Emoção):\n")
	instructions.WriteString("Objetivo: Validar a emoção do usuário sem julgamento.\n\n")

	if len(chain.Emotions) > 0 {
		instructions.WriteString(fmt.Sprintf("Emoções detectadas: %s\n",
			strings.Join(chain.Emotions, ", ")))
	}

	instructions.WriteString(fmt.Sprintf("Intensidade emocional: %.0f%%\n\n", chain.Intensity*100))

	instructions.WriteString("Técnica:\n")
	instructions.WriteString("- Nomear a emoção percebida\n")
	instructions.WriteString("- Validar sem minimizar\n")
	instructions.WriteString("- Oferecer presença\n\n")

	instructions.WriteString("Exemplo:\n")
	instructions.WriteString("'Percebo que isso te deixa [emoção]. É compreensível sentir assim.'\n")

	return instructions.String()
}

func (g *ResponseGenerator) getNeutralInstructions() string {
	return `RESPOSTA NEUTRA (Fallback):
Objetivo: Responder de forma empática sem fazer inferências arriscadas.

Técnica:
- Escuta ativa
- Perguntas abertas
- Validação básica

Exemplo:
"Entendo. Pode me contar mais sobre isso?"
`
}
