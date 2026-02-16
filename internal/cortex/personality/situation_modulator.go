// DEPRECATED: Este módulo é duplicado de internal/cortex/situation/modulator.go
// que é a implementação ativa usada pelo FDPN pipeline.
// SelectPostureWithSituation no personality_router.go nunca é chamado externamente.
// TODO: Consolidar com cortex/situation e remover este arquivo.
package personality

import (
	"strings"
	"time"
)

// Situation represents the current situational context of a user
type Situation struct {
	Stressors     []string // ["luto", "dor_cronica", "solidao"]
	SocialContext string   // "sozinho", "com_familia", "hospital"
	TimeOfDay     string   // "madrugada", "tarde", "noite"
	PhysicalState string   // "dor", "cansado", "medicado"
	RecentEvents  []string // ["morte_cachorro", "visita_filho"]
	Timestamp     time.Time
}

// LifeEvent represents a significant event in user's life
type LifeEvent struct {
	Type      string // "morte_conjuge", "visita_filho", etc.
	Timestamp time.Time
	Impact    float64 // 0-1
}

// InferSituation infers the current situation from session data
func InferSituation(userID int64, sessionData SessionData, recentEvents []LifeEvent) Situation {
	sit := Situation{
		Stressors:     []string{},
		SocialContext: "desconhecido",
		TimeOfDay:     getTimeOfDay(sessionData.Timestamp),
		PhysicalState: "normal",
		RecentEvents:  []string{},
		Timestamp:     time.Now(),
	}

	// Infer stressors from behaviors and recent events
	for _, behavior := range sessionData.Behaviors {
		if strings.Contains(behavior, "dor") || strings.Contains(behavior, "dolorido") {
			sit.Stressors = append(sit.Stressors, "dor_cronica")
			sit.PhysicalState = "dor"
		}
		if strings.Contains(behavior, "cansado") || strings.Contains(behavior, "exausto") {
			sit.PhysicalState = "cansado"
		}
		if strings.Contains(behavior, "sozinho") || strings.Contains(behavior, "solidão") {
			sit.Stressors = append(sit.Stressors, "solidao")
			sit.SocialContext = "sozinho"
		}
	}

	// Add recent events as stressors
	for _, event := range recentEvents {
		if event.Impact > 0.7 {
			sit.RecentEvents = append(sit.RecentEvents, event.Type)
			if strings.Contains(event.Type, "morte") {
				sit.Stressors = append(sit.Stressors, "luto")
			}
		}
	}

	return sit
}

// ModulateWeights modulates personality weights based on situation
func ModulateWeights(baseType int, sit Situation) map[string]float64 {
	// Get base weights for this Enneagram type
	weights := GetBaseWeights(baseType)

	// Apply situational modulation
	for _, stressor := range sit.Stressors {
		switch stressor {
		case "luto":
			modulateForGrief(baseType, weights)
		case "solidao":
			modulateForLoneliness(baseType, weights)
		case "dor_cronica":
			modulateForPain(baseType, weights)
		}
	}

	// Time of day modulation
	if sit.TimeOfDay == "madrugada" {
		// Anxiety and negative thoughts increase at night
		if weights["ANSIEDADE"] > 0 {
			weights["ANSIEDADE"] *= 1.3
		}
	}

	// Social context modulation
	if sit.SocialContext == "sozinho" {
		modulateForSolitude(baseType, weights)
	}

	return weights
}

func modulateForGrief(baseType int, weights map[string]float64) {
	// All types experience increased sadness and anxiety during grief
	if weights["TRISTEZA"] > 0 {
		weights["TRISTEZA"] *= 1.5
	}
	if weights["ANSIEDADE"] > 0 {
		weights["ANSIEDADE"] *= 1.3
	}

	// Type-specific modulation
	switch baseType {
	case 2: // Helper - may feel abandoned
		weights["BUSCA_CONEXAO"] *= 1.8
	case 4: // Individualist - deeper emotional processing
		weights["PROFUNDIDADE_EMOCIONAL"] *= 1.6
	case 6: // Loyalist - increased anxiety
		weights["ANSIEDADE"] *= 1.5
		weights["BUSCA_SEGURANÇA"] *= 1.7
	}
}

func modulateForLoneliness(baseType int, weights map[string]float64) {
	switch baseType {
	case 2: // Helper - craves connection
		weights["BUSCA_CONEXAO"] *= 2.0
		weights["TRISTEZA"] *= 1.4
	case 5: // Investigator - may be comfortable alone
		weights["ISOLAMENTO"] *= 0.8 // Less affected
	case 6: // Loyalist - increased anxiety
		weights["ANSIEDADE"] *= 1.6
	case 9: // Peacemaker - may feel lost
		weights["APATIA"] *= 1.5
	}
}

func modulateForPain(baseType int, weights map[string]float64) {
	// Pain increases irritability and decreases patience
	if weights["IRRITABILIDADE"] > 0 {
		weights["IRRITABILIDADE"] *= 1.5
	}

	// Type-specific
	switch baseType {
	case 1: // Perfectionist - frustration with body
		weights["FRUSTRAÇÃO"] *= 1.6
	case 8: // Challenger - may push through
		weights["RESISTÊNCIA"] *= 1.3
	}
}

func modulateForSolitude(baseType int, weights map[string]float64) {
	// Being alone amplifies baseline tendencies
	switch baseType {
	case 4: // Individualist - deeper introspection
		weights["PROFUNDIDADE_EMOCIONAL"] *= 1.4
	case 5: // Investigator - comfortable
		weights["AUTONOMIA"] *= 1.2
	case 6: // Loyalist - increased anxiety
		weights["ANSIEDADE"] *= 1.5
	case 7: // Enthusiast - may feel restless
		weights["INQUIETAÇÃO"] *= 1.6
	}
}

// GenerateSituationalGuidance generates guidance adapted to the situation
func GenerateSituationalGuidance(baseType int, sit Situation) string {
	guidance := ""

	// Check for critical situations
	if containsString(sit.Stressors, "luto") && sit.SocialContext == "sozinho" && sit.TimeOfDay == "madrugada" {
		guidance = "ATENÇÃO: Usuário em luto, sozinho, de madrugada. Risco elevado de crise. Seja especialmente empática e considere acionar suporte."
		return guidance
	}

	// General situational guidance
	if len(sit.Stressors) > 2 {
		guidance = "Usuário sob múltiplos estressores. Abordagem gentil e validação emocional são prioritárias."
	} else if sit.SocialContext == "sozinho" {
		guidance = "Usuário está sozinho. EVA pode ser a única companhia no momento. Seja presente e acolhedora."
	}

	return guidance
}

// GetBaseWeights returns base cognitive weights for an Enneagram type
func GetBaseWeights(enneaType int) map[string]float64 {
	baseWeights := map[int]map[string]float64{
		1: {"PERFECCIONISMO": 0.9, "FRUSTRAÇÃO": 0.7, "RESPONSABILIDADE": 0.8},
		2: {"BUSCA_CONEXAO": 0.9, "EMPATIA": 0.8, "NECESSIDADE_APROVACAO": 0.7},
		3: {"AMBIÇÃO": 0.9, "IMAGEM": 0.8, "EFICIÊNCIA": 0.8},
		4: {"PROFUNDIDADE_EMOCIONAL": 0.9, "AUTENTICIDADE": 0.8, "TRISTEZA": 0.6},
		5: {"AUTONOMIA": 0.9, "ANÁLISE": 0.8, "ISOLAMENTO": 0.6},
		6: {"ANSIEDADE": 0.9, "BUSCA_SEGURANÇA": 0.8, "LEALDADE": 0.7},
		7: {"ENTUSIASMO": 0.9, "INQUIETAÇÃO": 0.7, "EVITAÇÃO_DOR": 0.6},
		8: {"CONTROLE": 0.9, "RESISTÊNCIA": 0.8, "PROTEÇÃO": 0.7},
		9: {"HARMONIA": 0.9, "APATIA": 0.6, "EVITAÇÃO_CONFLITO": 0.8},
	}

	if weights, exists := baseWeights[enneaType]; exists {
		return copyWeights(weights)
	}

	return map[string]float64{}
}

func getTimeOfDay(t time.Time) string {
	hour := t.Hour()
	if hour >= 0 && hour < 6 {
		return "madrugada"
	} else if hour >= 6 && hour < 12 {
		return "manhã"
	} else if hour >= 12 && hour < 18 {
		return "tarde"
	} else {
		return "noite"
	}
}

func copyWeights(original map[string]float64) map[string]float64 {
	copy := make(map[string]float64)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}
