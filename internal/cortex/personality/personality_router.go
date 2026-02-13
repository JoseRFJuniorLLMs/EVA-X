package personality

import "fmt"

type EneagramType int

const (
	Type1 EneagramType = 1
	Type2 EneagramType = 2
	Type3 EneagramType = 3
	Type4 EneagramType = 4
	Type5 EneagramType = 5
	Type6 EneagramType = 6
	Type7 EneagramType = 7
	Type8 EneagramType = 8
	Type9 EneagramType = 9
)

type RouterState struct {
	BaseType    EneagramType
	CurrentType EneagramType
	Mode        string // "balanced", "stress", "growth"
}

// PersonalityRouter decide qual "máscara" a EVA deve usar
type PersonalityRouter struct {
	// Configurações fixas ou carregadas
}

func NewPersonalityRouter() *PersonalityRouter {
	return &PersonalityRouter{}
}

// RoutePersonality decide o Eneatipo baseado na emoção detectada
// Baseado na Lei do 3 (Gurdjieff) e movimentos de integração/desintegração
func (r *PersonalityRouter) RoutePersonality(baseType EneagramType, emotion string) (EneagramType, string) {
	// Mapeamento de emoções para estados (PT + EN)
	switch emotion {
	case "estresse", "raiva", "medo", "ansiedade", "confusão",
		"stress", "anger", "fear", "anxiety", "confusion":
		// Movimento de Desintegração (Stress)
		return getStressPoint(baseType), "stress"
	case "alegria", "gratidão", "paz", "esperança",
		"joy", "gratitude", "peace", "hope":
		// Movimento de Integração (Growth)
		return getGrowthPoint(baseType), "growth"
	default:
		// Estado base
		return baseType, "balanced"
	}
}

// GetSystemPromptFragment retorna a instrução de personalidade para o LLM
func (r *PersonalityRouter) GetSystemPromptFragment(currentType EneagramType) string {
	descriptions := map[EneagramType]string{
		Type2: "Você está no modo AJUDANTE (Tipo 2). Seja calorosa, empática e focada nas necessidades emocionais. Priorize a conexão e o cuidado.",
		Type8: "Você está no modo DESAFIADOR (Tipo 8). Seja direta, protetora e assertiva. Transmita força e segurança. Não hesite em assumir o controle se necessário.",
		Type9: "Você está no modo PACIFICADOR (Tipo 9). Seja calma, aceitadora e harmoniosa. Evite conflitos e busque trazer tranquilidade e estabilidade.",
		Type6: "Você está no modo LEALISTA (Tipo 6). Seja atenta, vigilante e transmita segurança. Mostre que você está lá para proteger e prevenir riscos.",
		Type3: "Você está no modo REALIZADOR (Tipo 3). Seja eficiente, motivadora e focada em resultados. Incentive a ação e a superação.",
		Type1: "Você está no modo PERFECCIONISTA (Tipo 1). Seja correta, precisa e estruturada. Mantenha a ordem e a clareza.",
		Type4: "Você está no modo INDIVIDUALISTA (Tipo 4). Seja profunda, sensível e autêntica. Valide a singularidade dos sentimentos.",
		Type5: "Você está no modo INVESTIGADOR (Tipo 5). Seja observadora, lógica e analítica. Forneça informações claras e objetivas.",
		Type7: "Você está no modo ENTUSIASTA (Tipo 7). Seja otimista, alegre e espontânea. Traga leveza e novas perspectivas.",
	}

	instruction, exists := descriptions[currentType]
	if !exists {
		return descriptions[Type9] // Fallback
	}

	// Add Attention Weights to the prompt
	weights := r.GetAttentionWeights(currentType)
	instruction += "\n\n[ATENÇÃO COGNITIVA - ZEROS DE ATENÇÃO]:"
	for concept, weight := range weights {
		if weight > 1.0 {
			instruction += fmt.Sprintf("\n- AMPLIFICAR foco em '%s' (fator %.1fx)", concept, weight)
		} else if weight < 1.0 {
			instruction += fmt.Sprintf("\n- REDUZIR foco em '%s' (fator %.1fx)", concept, weight)
		}
	}

	return instruction
}

// GetAttentionWeights retorna os 'Zeros de Atenção' (Gurdjieff/Riemann) para cada tipo
// Isso define o que a personalidade "vê" ou "ignora" no grafo.
func (r *PersonalityRouter) GetAttentionWeights(t EneagramType) map[string]float64 {
	switch t {
	case Type1: // O Reformador
		return map[string]float64{
			"DEVER": 1.8, "PROTOCOLO": 1.6, "ÉTICO": 1.7, "CORREÇÃO": 1.9, "EMOCIONAL": 0.6,
		}
	case Type2: // O Ajudante
		return map[string]float64{
			"AFETO": 2.0, "NECESSIDADE": 1.8, "CUIDADO": 1.9, "VÍNCULO": 1.85, "DADO_TÉCNICO": 0.7,
		}
	case Type3: // O Realizador
		return map[string]float64{
			"SUCESSO": 1.9, "META": 1.8, "EFICIÊNCIA": 1.7, "IMAGEM": 1.6, "SENTIMENTO": 0.5,
		}
	case Type4: // O Individualista
		return map[string]float64{
			"SENTIMENTO": 2.1, "SIGNIFICADO": 1.9, "AUTENTICIDADE": 2.0, "COMUM": 0.4,
		}
	case Type5: // O Investigador
		return map[string]float64{
			"EVIDÊNCIA": 2.0, "LÓGICA": 1.9, "ANÁLISE": 1.95, "DADOS": 1.85, "EMOCIONAL": 0.6,
		}
	case Type6: // O Lealista
		return map[string]float64{
			"RISCO": 2.2, "SEGURANÇA": 2.0, "PROTOCOLO": 1.8, "PERIGO": 2.1, "AMBIGUIDADE": 0.5,
		}
	case Type7: // O Entusiasta
		return map[string]float64{
			"NOVIDADE": 2.0, "PRAZER": 1.9, "FUTURO": 1.8, "DOR": 0.3, "ROTINA": 0.4,
		}
	case Type8: // O Desafiador
		return map[string]float64{
			"PODER": 1.9, "CONTROLE": 1.8, "JUSTIÇA": 1.8, "FRAQUEZA": 0.2, "AÇÃO": 1.7,
		}
	case Type9: // O Pacificador
		return map[string]float64{
			"HARMONIA": 1.9, "PAZ": 1.85, "UNIÃO": 1.8, "CONFLITO": 0.5,
		}
	default:
		return map[string]float64{}
	}
}

// getStressPoint retorna o ponto de estresse (desintegração)
func getStressPoint(t EneagramType) EneagramType {
	// Sequência externa: 1->4->2->8->5->7->1
	// Triângulo: 9->6->3->9
	switch t {
	case Type1:
		return Type4
	case Type4:
		return Type2
	case Type2:
		return Type8
	case Type8:
		return Type5
	case Type5:
		return Type7
	case Type7:
		return Type1
	case Type9:
		return Type6
	case Type6:
		return Type3
	case Type3:
		return Type9
	default:
		return t
	}
}

// getGrowthPoint retorna o ponto de crescimento (integração) - inverso do estresse
func getGrowthPoint(t EneagramType) EneagramType {
	// ... (rest of implementation)
	switch t {
	case Type1:
		return Type7
	case Type7:
		return Type5
	case Type5:
		return Type8
	case Type8:
		return Type2
	case Type2:
		return Type4
	case Type4:
		return Type1
	case Type9:
		return Type3
	case Type3:
		return Type6
	case Type6:
		return Type9
	default:
		return t
	}
}
