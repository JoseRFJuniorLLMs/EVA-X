package lacan

import (
	"strings"
)

// DemandDesireService distingue entre demanda superficial e desejo latente
type DemandDesireService struct{}

// NewDemandDesireService cria novo serviço
func NewDemandDesireService() *DemandDesireService {
	return &DemandDesireService{}
}

// LatentDesire representa o desejo latente por trás de uma demanda
type LatentDesire string

const (
	DESEJO_RECONHECIMENTO LatentDesire = "reconhecimento_afetivo" // Quer ser visto/validado
	DESEJO_ESCUTA         LatentDesire = "escuta_validacao"       // Quer ser ouvido
	DESEJO_COMPANHIA      LatentDesire = "companhia_presenca"     // Sozinho, quer presença
	DESEJO_CONTROLE       LatentDesire = "controle_autonomia"     // Sente perda de autonomia
	DESEJO_SIGNIFICADO    LatentDesire = "significado_proposito"  // Busca sentido de vida
	DESEJO_AMOR           LatentDesire = "amor_afeto"             // Quer sentir-se amado
	DESEJO_PERDAO         LatentDesire = "perdao_reconciliacao"   // Culpa, arrependimento
	DESEJO_MORTE          LatentDesire = "morte_finitude"         // Elaboração da finitude
	DESEJO_INDEFINIDO     LatentDesire = "indefinido"
)

// Analysis representa a análise demanda/desejo
type Analysis struct {
	SurfaceDemand  string       // O que foi dito literalmente
	LatentDesire   LatentDesire // O que está implícito
	Confidence     float64      // Confiança na análise (0.0-1.0)
	Interpretation string       // Interpretação textual
}

// AnalyzeUtterance analisa uma fala para extrair desejo latente
func (d *DemandDesireService) AnalyzeUtterance(text string) *Analysis {
	textLower := strings.ToLower(text)

	analysis := &Analysis{
		SurfaceDemand: text,
		Confidence:    0.5, // Padrão
	}

	// Padrão: Demanda de visita → Desejo de reconhecimento
	if strings.Contains(textLower, "quero que") && containsAny(textLower, []string{"me visite", "venha me ver", "passe aqui"}) {
		analysis.LatentDesire = DESEJO_RECONHECIMENTO
		analysis.Confidence = 0.8
		analysis.Interpretation = "Por trás da demanda de visita, há um desejo de ser reconhecido e valorizado pela pessoa"
		return analysis
	}

	// Padrão: Expressão de solidão → Desejo de companhia
	if containsAny(textLower, []string{"sozinho", "solidão", "ninguém", "me sinto só"}) {
		analysis.LatentDesire = DESEJO_COMPANHIA
		analysis.Confidence = 0.9
		analysis.Interpretation = "A solidão expressa um desejo profundo de presença e companhia"
		return analysis
	}

	// Padrão: Queixa de dor/sofrimento → Desejo de escuta
	if containsAny(textLower, []string{"não aguento", "não suporto", "tá difícil", "sofro muito"}) {
		analysis.LatentDesire = DESEJO_ESCUTA
		analysis.Confidence = 0.85
		analysis.Interpretation = "A queixa indica necessidade de ser ouvido e validado em seu sofrimento"
		return analysis
	}

	// Padrão: Perda de autonomia → Desejo de controle
	if containsAny(textLower, []string{"não consigo mais", "dependo de", "preciso de ajuda para"}) {
		analysis.LatentDesire = DESEJO_CONTROLE
		analysis.Confidence = 0.75
		analysis.Interpretation = "A frustração com dependência revela desejo de manter autonomia e controle"
		return analysis
	}

	// Padrão: Questões existenciais → Desejo de significado
	if containsAny(textLower, []string{"qual o sentido", "pra que", "por que estou aqui", "não serve pra nada"}) {
		analysis.LatentDesire = DESEJO_SIGNIFICADO
		analysis.Confidence = 0.9
		analysis.Interpretation = "A questão existencial revela busca por sentido e propósito de vida"
		return analysis
	}

	// Padrão: Culpa/arrependimento → Desejo de perdão
	if containsAny(textLower, []string{"deveria ter", "me arrependo", "foi minha culpa", "não fiz o suficiente"}) {
		analysis.LatentDesire = DESEJO_PERDAO
		analysis.Confidence = 0.8
		analysis.Interpretation = "A culpa expressa desejo de perdão e reconciliação (consigo ou com outros)"
		return analysis
	}

	// Padrão: Fala sobre morte → Elaboração da finitude
	if containsAny(textLower, []string{"quando eu morrer", "não tenho muito tempo", "já vivi demais", "quero morrer"}) {
		analysis.LatentDesire = DESEJO_MORTE
		analysis.Confidence = 0.95
		analysis.Interpretation = "A menção à morte indica necessidade de elaborar a própria finitude"
		return analysis
	}

	// Padrão: Busca de afeto → Desejo de amor
	if containsAny(textLower, []string{"ninguém me ama", "sou amado", "me importam", "se importa comigo"}) {
		analysis.LatentDesire = DESEJO_AMOR
		analysis.Confidence = 0.85
		analysis.Interpretation = "A questão revela desejo fundamental de sentir-se amado e importante"
		return analysis
	}

	analysis.LatentDesire = DESEJO_INDEFINIDO
	analysis.Confidence = 0.3
	analysis.Interpretation = "Desejo não identificado nesta análise"
	return analysis
}

// GenerateResponse gera resposta baseada no desejo latente
func (d *DemandDesireService) GenerateResponse(analysis *Analysis) string {
	responses := map[LatentDesire]string{
		DESEJO_RECONHECIMENTO: "Você é importante e suas histórias têm valor. O que você mais gostaria que essa pessoa soubesse sobre você?",
		DESEJO_ESCUTA:         "Estou aqui para ouvir você. O que você mais precisa expressar agora?",
		DESEJO_COMPANHIA:      "A solidão é difícil. Como você tem lidado com esses momentos sozinho? Conte-me.",
		DESEJO_CONTROLE:       "É frustrante perder autonomia. O que você ainda consegue fazer que te dá satisfação?",
		DESEJO_SIGNIFICADO:    "É uma pergunta profunda. O que já deu sentido à sua vida até aqui?",
		DESEJO_AMOR:           "Sentir-se amado é fundamental. Quando você se sentiu mais amado na vida?",
		DESEJO_PERDAO:         "Todos carregamos arrependimentos. Você gostaria de falar sobre isso?",
		DESEJO_MORTE:          "Pensar na própria finitude é natural. O que você gostaria de deixar como legado?",
	}

	if response, ok := responses[analysis.LatentDesire]; ok {
		return response
	}

	return "Conte-me mais sobre isso. O que isso significa para você?"
}

// GetClinicalGuidance retorna orientação clínica para o desejo
func GetClinicalGuidance(desire LatentDesire) string {
	guidance := map[LatentDesire]string{
		DESEJO_RECONHECIMENTO: "Valide a pessoa. Demonstre que ela é vista e reconhecida.",
		DESEJO_ESCUTA:         "Pratique escuta ativa. Não ofereça soluções imediatas.",
		DESEJO_COMPANHIA:      "Seja presença. Não minimize a solidão ('você não está sozinho').",
		DESEJO_CONTROLE:       "Empodere. Pergunte preferências, dê escolhas.",
		DESEJO_SIGNIFICADO:    "Ajude a narrar a vida como história com sentido.",
		DESEJO_AMOR:           "Demonstre cuidado genuíno. Use linguagem afetiva.",
		DESEJO_PERDAO:         "Não julgue. Ajude a elaborar através da fala.",
		DESEJO_MORTE:          "Não evite o tema. O Real não se resolve, mas pode ser simbolizado.",
	}

	if g, ok := guidance[desire]; ok {
		return g
	}
	return "Escute atentamente e reflita a fala de volta."
}
