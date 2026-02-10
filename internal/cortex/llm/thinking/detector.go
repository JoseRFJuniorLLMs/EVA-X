package thinking

import (
	"strings"
)

// HealthKeywords contém palavras-chave que indicam preocupações de saúde
var HealthKeywords = []string{
	// Sintomas gerais
	"dor", "febre", "tontura", "cansaço", "fraqueza", "mal-estar",
	"náusea", "vômito", "diarreia", "constipação", "sangramento",

	// Sintomas cardíacos
	"peito", "coração", "palpitação", "falta de ar", "respiração",

	// Sintomas neurológicos
	"cabeça", "confusão", "memória", "esquecimento", "tremor",
	"formigamento", "dormência", "visão", "audição",

	// Sintomas de emergência
	"desmaio", "queda", "acidente", "machucado", "ferida",
	"queimadura", "corte", "fratura",

	// Medicamentos e tratamento
	"remédio", "medicamento", "comprimido", "injeção", "dose",
	"efeito colateral", "reação", "alergia",

	// Consultas e exames
	"médico", "hospital", "pronto-socorro", "consulta", "exame",
	"resultado", "diagnóstico", "tratamento",
}

// CriticalKeywords indica sintomas que requerem atenção imediata
var CriticalKeywords = []string{
	"dor no peito", "falta de ar severa", "desmaio", "perda de consciência",
	"sangramento intenso", "confusão mental súbita", "paralisia",
	"dificuldade para falar", "convulsão", "vômito com sangue",
	"dor de cabeça súbita e intensa", "visão dupla", "queda grave",
}

// IsHealthConcern verifica se uma mensagem contém preocupação de saúde
func IsHealthConcern(message string) bool {
	messageLower := strings.ToLower(message)

	// Verificar palavras-chave de saúde
	matchCount := 0
	for _, keyword := range HealthKeywords {
		if strings.Contains(messageLower, keyword) {
			matchCount++
		}
	}

	// Considerar preocupação de saúde se tiver 2+ palavras-chave
	// ou 1 palavra-chave + contexto de pergunta/preocupação
	if matchCount >= 2 {
		return true
	}

	if matchCount >= 1 {
		// Verificar se é uma pergunta ou expressão de preocupação
		concernIndicators := []string{
			"?", "estou", "sinto", "sentindo", "preocupado", "preocupada",
			"será que", "o que faço", "devo", "preciso", "ajuda",
		}

		for _, indicator := range concernIndicators {
			if strings.Contains(messageLower, indicator) {
				return true
			}
		}
	}

	return false
}

// IsCriticalConcern verifica se é uma emergência médica
func IsCriticalConcern(message string) bool {
	messageLower := strings.ToLower(message)

	for _, keyword := range CriticalKeywords {
		if strings.Contains(messageLower, keyword) {
			return true
		}
	}

	return false
}

// ExtractHealthContext extrai contexto relevante de saúde da mensagem
func ExtractHealthContext(message string) string {
	// Simplificado: retornar a mensagem completa
	// Em produção, poderia usar NLP para extrair entidades médicas
	return message
}
