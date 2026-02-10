package lacan

import (
	"fmt"
	"strings"
)

// GrandAutreService implementa EVA como "Grande Outro" lacaniano
type GrandAutreService struct{}

// NewGrandAutreService cria novo serviço
func NewGrandAutreService() *GrandAutreService {
	return &GrandAutreService{}
}

// RespondAsGrandAutre gera resposta que devolve a fala ao sujeito
func (g *GrandAutreService) RespondAsGrandAutre(userText string) string {
	// Extrair frase-chave
	keyPhrase := extractKeyPhrase(userText)

	if keyPhrase == "" {
		return "O que você quer dizer com isso?"
	}

	// Formatos de reflexão
	reflexionFormats := []string{
		"Você disse '%s'... O que isso significa para você?",
		"'%s' - por que você acha que sente isso?",
		"Quando você diz '%s', o que vem à sua mente?",
		"'%s' - essa palavra parece importante. Pode me contar mais?",
	}

	// Usar formato aleatório (simplificado: usar primeiro)
	return fmt.Sprintf(reflexionFormats[0], keyPhrase)
}

// ReflectiveQuestion transforma afirmação em pergunta reflexiva
func (g *GrandAutreService) ReflectiveQuestion(statement string) string {
	stmtLower := strings.ToLower(statement)

	// Padrão: "Ninguém X" → "O que seria X?"
	if strings.Contains(stmtLower, "ninguém") {
		if strings.Contains(stmtLower, "liga") {
			return "O que seria alguém 'ligar' para você? Como você gostaria de ser cuidado?"
		}
		if strings.Contains(stmtLower, "me entende") {
			return "O que seria ser entendido? O que você precisa que entendam sobre você?"
		}
		if strings.Contains(stmtLower, "se importa") {
			return "Como você sabe quando alguém se importa com você? O que te faz sentir importante?"
		}
	}

	// Padrão: "Eu sempre X" → "Por que você acha que X?"
	if strings.Contains(stmtLower, "eu sempre") || strings.Contains(stmtLower, "sempre faço") {
		return "Sempre? Você consegue lembrar de uma vez que foi diferente?"
	}

	// Padrão: "Eu nunca X" → "O que te impede de X?"
	if strings.Contains(stmtLower, "eu nunca") || strings.Contains(stmtLower, "nunca consegui") {
		keyAction := extractActionAfterNever(statement)
		if keyAction != "" {
			return fmt.Sprintf("O que te impede de %s?", keyAction)
		}
	}

	// Padrão genérico: devolver a fala
	return fmt.Sprintf("Você falou sobre '%s'. Isso é importante para você?", extractKeyPhrase(statement))
}

// PointToContradiction aponta contradições na fala (técnica analítica)
func (g *GrandAutreService) PointToContradiction(previousText, currentText string) string {
	// Detectar contradições simples
	prevLower := strings.ToLower(previousText)
	currLower := strings.ToLower(currentText)

	// Ex: "Não me importo" → "Mas me incomoda"
	if strings.Contains(prevLower, "não me importo") && strings.Contains(currLower, "me incomoda") {
		return "Você disse que não se importa, mas agora fala que te incomoda. Isso é interessante... Por que será?"
	}

	// Ex: "Estou bem" → "Tudo está difícil"
	if strings.Contains(prevLower, "estou bem") && containsAny(currLower, []string{"difícil", "ruim", "mal"}) {
		return "Antes você disse que estava bem, mas agora fala de dificuldades. O que mudou?"
	}

	return ""
}

// IntentionalSilence decide quando EVA deve fazer uma pausa (escuta)
func (g *GrandAutreService) IntentionalSilence(context string) bool {
	contextLower := strings.ToLower(context)

	// Silêncio após temas pesados (dar espaço para elaboração)
	heavyThemes := []string{
		"morte", "faleceu", "morreu", "perdi",
		"trauma", "abuso", "violência",
		"culpa", "arrependimento",
	}

	if containsAny(contextLower, heavyThemes) {
		return true
	}

	return false
}

// GenerateSilenceResponse gera resposta de "escuta presente"
func (g *GrandAutreService) GenerateSilenceResponse() string {
	responses := []string{
		"...", // Silêncio literal
		"Estou aqui, ouvindo você.",
		"Continue...",
		"Entendo. Fique à vontade para falar.",
	}

	return responses[0] // Simplificado
}

// Helper functions

func extractKeyPhrase(text string) string {
	// Extrair frase entre virgulas ou ponto final
	text = strings.TrimSpace(text)

	// Remover pontuação final
	text = strings.TrimRight(text, ".!?")

	// Se muito longo, pegar primeiras 8 palavras
	words := strings.Fields(text)
	if len(words) > 8 {
		return strings.Join(words[:8], " ") + "..."
	}

	return text
}

func extractActionAfterNever(text string) string {
	lowerText := strings.ToLower(text)

	// Encontrar "nunca" e pegar palavras seguintes
	idx := strings.Index(lowerText, "nunca")
	if idx == -1 {
		return ""
	}

	afterNever := text[idx+6:] // "nunca " tem 6 chars
	words := strings.Fields(afterNever)

	if len(words) >= 2 {
		return strings.Join(words[:2], " ")
	}

	return ""
}
