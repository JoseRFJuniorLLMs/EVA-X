package lacan

import (
	"context"
	"fmt"
	"strings"
)

// ZetaRouter implementa a "Ética da Psicanálise" de Lacan
// Princípio: "Ne pas céder sur son désir" - Não ceder no desejo
//
// Determina o TOM e TIPO DE INTERVENÇÃO baseado na estrutura do discurso
type ZetaRouter struct {
	interpretation *InterpretationService
}

// InterventionType define o tipo de intervenção clínica
type InterventionType string

const (
	// Intervenções baseadas na Ética Lacaniana
	INTERVENTION_SILENCE        InterventionType = "silencio"      // Silêncio intencional (corte)
	INTERVENTION_PUNCTUATION    InterventionType = "pontuacao"     // Pontuar significante
	INTERVENTION_INTERPRETATION InterventionType = "interpretacao" // Interprelar
	INTERVENTION_REFLECTION     InterventionType = "reflexao"      // Devolver a fala
	INTERVENTION_SUPPORT        InterventionType = "suporte"       // Acolhimento empático
	INTERVENTION_CONFRONTATION  InterventionType = "confrontacao"  // Apontar contradição
	INTERVENTION_SCANSION       InterventionType = "escansao"      // Cortar sessão/frase
)

// EthicalStance representa a postura ética recomendada
type EthicalStance struct {
	Intervention   InterventionType
	ToneGuidance   string  // Como falar
	ContentExample string  // Exemplo de resposta
	Rationale      string  // Por que esta intervenção
	Urgency        float64 // 0.0-1.0 (urgência clínica)
}

// NewZetaRouter cria router ético
func NewZetaRouter(interpretation *InterpretationService) *ZetaRouter {
	return &ZetaRouter{
		interpretation: interpretation,
	}
}

// DetermineEthicalStance determina a postura ética apropriada
func (z *ZetaRouter) DetermineEthicalStance(ctx context.Context, idosoID int64, text string, result *InterpretationResult) (*EthicalStance, error) {
	stance := &EthicalStance{
		Urgency: 0.5, // Padrão
	}

	// 1. REGRA MÁXIMA: Nunca ceder no desejo do sujeito
	// Se o paciente está elaborando algo importante, NÃO interrompa com soluções
	if z.isElaborating(text, result) {
		stance.Intervention = INTERVENTION_SILENCE
		stance.ToneGuidance = "Presença silenciosa. Deixe o sujeito continuar."
		stance.ContentExample = "..." // Literal silence ou "Continue..."
		stance.Rationale = "O sujeito está elaborando. Silêncio = respeito ao desejo."
		stance.Urgency = 0.3
		return stance, nil
	}

	// 2. Se há contradição detectada, confronte (função do analista)
	if result.Contradiction != "" {
		stance.Intervention = INTERVENTION_CONFRONTATION
		stance.ToneGuidance = "Firme mas respeitosa. Aponte o impossível da fala."
		stance.ContentExample = result.Contradiction
		stance.Rationale = "Contradição indica ponto de resistência. Deve ser pontuado."
		stance.Urgency = 0.7
		return stance, nil
	}

	// 3. Se deve interpelar significante (momento de interpretação)
	if result.ShouldInterpel {
		stance.Intervention = INTERVENTION_INTERPRETATION
		stance.ToneGuidance = "Pontue o significante sem explicar demais."
		stance.ContentExample = result.InterpelPhrase
		stance.Rationale = "Significante recorrente = formação do inconsciente."
		stance.Urgency = 0.8
		return stance, nil
	}

	// 4. Se transferência forte, use reflexão (devolver a fala)
	if result.Transference != TRANSFERENCIA_NENHUMA {
		stance.Intervention = INTERVENTION_REFLECTION
		stance.ToneGuidance = "Acolha a transferência sem negar, mas devolva a questão."
		stance.ContentExample = result.ReflexiveQuestion
		stance.Rationale = "Transferência = material clínico. Use para fazer sujeito pensar."
		stance.Urgency = 0.6
		return stance, nil
	}

	// 5. Se desejo latente é claro, reflita sobre ele
	if result.DemandDesire != nil && result.DemandDesire.LatentDesire != DESEJO_INDEFINIDO {
		stance.Intervention = INTERVENTION_REFLECTION
		stance.ToneGuidance = "Distingue demanda de desejo. Não satisfaça demanda superficial."
		stance.ContentExample = result.SuggestedResponse
		stance.Rationale = fmt.Sprintf("Desejo latente: %s. Não ceder = fazer sujeito elaborar.", result.DemandDesire.LatentDesire)
		stance.Urgency = 0.5
		return stance, nil
	}

	// 6. Padrão: Suporte empático (mas sempre devolvendo a fala)
	stance.Intervention = INTERVENTION_SUPPORT
	stance.ToneGuidance = "Empática mas não consoladora. Valide sem resolver."
	stance.ContentExample = "Entendo. Conte-me mais sobre isso."
	stance.Rationale = "Postura analítica básica: escuta + devolução."
	stance.Urgency = 0.4
	return stance, nil
}

// isElaborating detecta se o sujeito está em processo de elaboração
func (z *ZetaRouter) isElaborating(text string, result *InterpretationResult) bool {
	textLower := strings.ToLower(text)

	// Marcadores de elaboração:
	// - Pausas, hesitações ("é que...", "tipo assim...", "sabe...")
	// - Reformulações ("ou melhor...", "na verdade...")
	// - Conexões temporais ("quando era criança...", "lembro que...")
	// - Emoção intensa (detectada via emotional_charge)

	elaborationMarkers := []string{
		"é que", "tipo assim", "sabe", "quer dizer",
		"ou melhor", "na verdade", "pensando bem",
		"quando era", "lembro que", "me faz lembrar",
		"às vezes penso", "não sei se", "talvez",
	}

	if containsAny(textLower, elaborationMarkers) {
		return true
	}

	// Se há transferência materna/filial forte, pode estar elaborando vínculo
	if result.Transference == TRANSFERENCIA_MATERNA || result.Transference == TRANSFERENCIA_FILIAL {
		return true
	}

	return false
}

// BuildEthicalPrompt constrói instrução ética para o prompt
func (z *ZetaRouter) BuildEthicalPrompt(stance *EthicalStance) string {
	prompt := "\n⚖️ POSTURA ÉTICA (LACAN - NE PAS CÉDER):\n\n"

	prompt += fmt.Sprintf("INTERVENÇÃO RECOMENDADA: %s\n", strings.ToUpper(string(stance.Intervention)))
	prompt += fmt.Sprintf("URGÊNCIA CLÍNICA: %.1f/1.0\n\n", stance.Urgency)

	prompt += fmt.Sprintf("TOM: %s\n", stance.ToneGuidance)
	prompt += fmt.Sprintf("EXEMPLO: \"%s\"\n\n", stance.ContentExample)

	prompt += fmt.Sprintf("FUNDAMENTAÇÃO: %s\n\n", stance.Rationale)

	// Adicionar princípios éticos gerais
	prompt += "PRINCÍPIOS ÉTICOS FUNDAMENTAIS:\n"
	prompt += "1. NÃO CEDER NO DESEJO: Não satisfaça a demanda superficial. Aponte para o desejo latente.\n"
	prompt += "2. NÃO CONSOLAR: Consolação = tampar a falta. A falta é estrutural.\n"
	prompt += "3. NÃO RESOLVER O IMPOSSÍVEL: Trauma, morte, perda não têm solução. Ajude a simbolizar.\n"
	prompt += "4. DEVOLVER A FALA: Faça o sujeito trabalhar. Você não tem as respostas, ele tem.\n"
	prompt += "5. RESPEITAR O TEMPO DO SUJEITO: Não force elaboração. Espere o momento.\n"
	prompt += "6. SER LUGAR DO OUTRO: EVA = Grande Outro. Não negue transferência, use-a.\n\n"

	// Advertências baseadas no tipo de intervenção
	switch stance.Intervention {
	case INTERVENTION_SILENCE:
		prompt += "⚠️ SILÊNCIO ANALÍTICO: Não preencha o vazio com palavras. O silêncio faz pensar.\n"
	case INTERVENTION_CONFRONTATION:
		prompt += "⚠️ CONFRONTAÇÃO: Seja firme mas não agressiva. Aponte o impossível da fala sem julgamento.\n"
	case INTERVENTION_INTERPRETATION:
		prompt += "⚠️ INTERPRETAÇÃO: Seja breve e enigmática. Não explique. Deixe o sujeito associar.\n"
	case INTERVENTION_SUPPORT:
		prompt += "⚠️ SUPORTE: Valide sem resolver. 'Entendo' ≠ 'Vou resolver'.\n"
	}

	prompt += "\n────────────────────────────────────────\n"

	return prompt
}

// DetermineGurdjieffType atualiza tipo de atenção baseado no contexto
// (Integração com o router de personalidade já existente)
func (z *ZetaRouter) DetermineGurdjieffType(ctx context.Context, idosoID int64, result *InterpretationResult) int {
	// Tipo 2 (Ajudante): Se há necessidade de cuidado maternal
	if result.DemandDesire != nil && (result.DemandDesire.LatentDesire == DESEJO_AMOR ||
		result.DemandDesire.LatentDesire == DESEJO_COMPANHIA) ||
		result.Transference == TRANSFERENCIA_MATERNA {
		return 2
	}

	// Tipo 6 (Leal/Segurança): Se há ansiedade ou busca de estrutura
	if result.DemandDesire != nil && result.DemandDesire.LatentDesire == DESEJO_CONTROLE ||
		result.Transference == TRANSFERENCIA_PATERNA {
		return 6
	}

	// Tipo 9 (Pacificador): Padrão harmonioso
	return 9
}
