package transnar

import (
	"context"
	"eva-mind/internal/cortex/lacan"
	"eva-mind/internal/cortex/personality"
	"strings"
)

// DesireType representa tipos de desejos latentes
type DesireType string

const (
	DesireConnection  DesireType = "connection"  // Desejo de conexão/vínculo
	DesireSecurity    DesireType = "security"    // Desejo de segurança
	DesireAutonomy    DesireType = "autonomy"    // Desejo de autonomia/controle
	DesireRecognition DesireType = "recognition" // Desejo de reconhecimento
	DesireRelief      DesireType = "relief"      // Desejo de alívio/paz
	DesireUnknown     DesireType = "unknown"     // Desejo não identificado
)

// DesireInference representa o resultado da inferência
type DesireInference struct {
	Desire       DesireType
	Confidence   float64
	Alternatives map[DesireType]float64
	Reasoning    string // Explicação da inferência
}

// DesireRule representa uma regra de inferência lacaniana
type DesireRule struct {
	Name        string
	Pattern     func(*SignifierChain, []lacan.Signifier, personality.EneagramType) bool
	Inference   DesireType
	Confidence  float64
	Description string
}

// DesireDetector aplica lógica lacaniana para detectar desejos latentes
type DesireDetector struct {
	rules []DesireRule
}

// NewDesireDetector cria um novo detector de desejos
func NewDesireDetector() *DesireDetector {
	detector := &DesireDetector{
		rules: []DesireRule{},
	}

	// Regra 1: Negação + Modal = Desejo Oposto (Lacan: denegação)
	detector.rules = append(detector.rules, DesireRule{
		Name: "negation_modal_reversal",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			return chain.HasPattern("negation_modal")
		},
		Inference:   DesireSecurity, // "Não quero X" geralmente = medo/insegurança
		Confidence:  0.75,           // Aumentado de 0.7 para produção
		Description: "Negação com modal indica desejo oposto (denegação freudiana)",
	})

	// Regra 2: Repetição de Significante = Fixação Inconsciente
	detector.rules = append(detector.rules, DesireRule{
		Name: "signifier_repetition",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			// Verificar se alguma palavra da cadeia é significante-mestre
			for _, word := range chain.Words {
				for _, sig := range history {
					if strings.Contains(strings.ToLower(sig.Word), strings.ToLower(word)) && sig.Frequency >= 3 {
						return true
					}
				}
			}
			return false
		},
		Inference:   DesireConnection, // Repetição geralmente indica falta/desejo
		Confidence:  0.85,             // Aumentado de 0.8
		Description: "Significante repetido indica fixação no objeto de desejo",
	})

	// Regra 3: Emoção Negativa + Tipo 6 = Desejo de Segurança
	detector.rules = append(detector.rules, DesireRule{
		Name: "fear_type6",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			hasNegativeEmotion := false
			for _, emotion := range chain.Emotions {
				if isNegativeEmotion(emotion) {
					hasNegativeEmotion = true
					break
				}
			}
			return hasNegativeEmotion && ptype == personality.Type6
		},
		Inference:   DesireSecurity,
		Confidence:  0.88, // Aumentado de 0.85 - muito confiável
		Description: "Tipo 6 com emoção negativa busca segurança",
	})

	// Regra 4: Modal "quero" + Negação = Ambivalência (desejo autonomia)
	detector.rules = append(detector.rules, DesireRule{
		Name: "want_negation",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			hasQuero := false
			for _, modal := range chain.Modals {
				if strings.Contains(strings.ToLower(modal), "quer") {
					hasQuero = true
					break
				}
			}
			return hasQuero && len(chain.Negations) > 0
		},
		Inference:   DesireAutonomy,
		Confidence:  0.70, // Aumentado de 0.65
		Description: "Quero + Negação indica desejo de controle/autonomia",
	})

	// Regra 5: Solidão/Abandono = Desejo de Conexão
	detector.rules = append(detector.rules, DesireRule{
		Name: "loneliness_signifier",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			lonelinessWords := []string{"solidão", "sozinho", "abandono", "ninguém", "isolado", "esquecido"}
			for _, word := range chain.Words {
				for _, lonely := range lonelinessWords {
					if strings.Contains(strings.ToLower(word), lonely) {
						return true
					}
				}
			}
			return false
		},
		Inference:   DesireConnection,
		Confidence:  0.92, // Aumentado de 0.9 - muito específico
		Description: "Menção de solidão indica desejo de conexão",
	})

	// Regra 6: Tipo 2 + Negação = Desejo de Reconhecimento
	detector.rules = append(detector.rules, DesireRule{
		Name: "type2_negation",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			return ptype == personality.Type2 && len(chain.Negations) > 0
		},
		Inference:   DesireRecognition,
		Confidence:  0.78, // Aumentado de 0.75
		Description: "Tipo 2 com negação busca reconhecimento",
	})

	// NOVA Regra 7: Resistência Verbal (múltiplas negações)
	detector.rules = append(detector.rules, DesireRule{
		Name: "verbal_resistance",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			return len(chain.Negations) >= 2 // "Não, não quero"
		},
		Inference:   DesireAutonomy,
		Confidence:  0.82,
		Description: "Múltiplas negações indicam resistência e desejo de controle",
	})

	// NOVA Regra 8: Transferência (menção de figuras de autoridade)
	detector.rules = append(detector.rules, DesireRule{
		Name: "authority_transference",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			authorityWords := []string{"médico", "doutor", "enfermeira", "filho", "filha", "família"}
			for _, word := range chain.Words {
				for _, auth := range authorityWords {
					if strings.Contains(strings.ToLower(word), auth) {
						return true
					}
				}
			}
			return false
		},
		Inference:   DesireSecurity,
		Confidence:  0.68,
		Description: "Menção de figuras de autoridade indica transferência e busca por segurança",
	})

	// NOVA Regra 9: Pulsão de Morte (menção de fim/morte/desistência)
	detector.rules = append(detector.rules, DesireRule{
		Name: "death_drive",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			deathWords := []string{"morrer", "morte", "acabar", "desistir", "fim", "parar"}
			for _, word := range chain.Words {
				for _, death := range deathWords {
					if strings.Contains(strings.ToLower(word), death) {
						return true
					}
				}
			}
			return false
		},
		Inference:   DesireRelief, // Desejo de alívio do sofrimento
		Confidence:  0.95,         // CRÍTICO - alta prioridade
		Description: "Menção de morte/fim indica desejo urgente de alívio",
	})

	// NOVA Regra 10: Impotência Aprendida (palavras de incapacidade)
	detector.rules = append(detector.rules, DesireRule{
		Name: "learned_helplessness",
		Pattern: func(chain *SignifierChain, history []lacan.Signifier, ptype personality.EneagramType) bool {
			helplessWords := []string{"não consigo", "não posso", "impossível", "difícil demais", "não aguento"}
			text := strings.ToLower(chain.RawText)
			for _, helpless := range helplessWords {
				if strings.Contains(text, helpless) {
					return true
				}
			}
			return false
		},
		Inference:   DesireRecognition, // Desejo de ser visto/validado
		Confidence:  0.80,
		Description: "Expressões de impotência indicam desejo de validação e suporte",
	})

	return detector
}

// Detect aplica as regras de inferência para detectar o desejo latente
func (d *DesireDetector) Detect(
	ctx context.Context,
	chain *SignifierChain,
	history []lacan.Signifier,
	personalityType personality.EneagramType,
) *DesireInference {

	// Acumulador de probabilidades
	scores := make(map[DesireType]float64)
	reasons := []string{}

	// Aplicar cada regra
	for _, rule := range d.rules {
		if rule.Pattern(chain, history, personalityType) {
			scores[rule.Inference] += rule.Confidence
			reasons = append(reasons, rule.Description)
		}
	}

	// Se nenhuma regra ativou, retornar desconhecido
	if len(scores) == 0 {
		return &DesireInference{
			Desire:       DesireUnknown,
			Confidence:   0.0,
			Alternatives: scores,
			Reasoning:    "Nenhum padrão detectado",
		}
	}

	// Normalizar scores (soma = 1.0)
	total := 0.0
	for _, score := range scores {
		total += score
	}
	for desire := range scores {
		scores[desire] /= total
	}

	// Encontrar desejo com maior probabilidade
	maxDesire := DesireUnknown
	maxScore := 0.0
	for desire, score := range scores {
		if score > maxScore {
			maxScore = score
			maxDesire = desire
		}
	}

	return &DesireInference{
		Desire:       maxDesire,
		Confidence:   maxScore,
		Alternatives: scores,
		Reasoning:    strings.Join(reasons, "; "),
	}
}

// GetDesireDescription retorna descrição humana do desejo
func GetDesireDescription(desire DesireType) string {
	descriptions := map[DesireType]string{
		DesireConnection:  "Desejo de conexão e vínculo afetivo",
		DesireSecurity:    "Desejo de segurança e proteção",
		DesireAutonomy:    "Desejo de autonomia e controle",
		DesireRecognition: "Desejo de reconhecimento e validação",
		DesireRelief:      "Desejo de alívio e paz interior",
		DesireUnknown:     "Desejo não identificado",
	}
	return descriptions[desire]
}
