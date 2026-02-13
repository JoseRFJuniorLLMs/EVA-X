package attention

import (
	"eva-mind/internal/cortex/attention/models"
	"strings"
)

// CenterRouter - Detecta e roteia baseado nos três centros
type CenterRouter struct {
	emotionalKeywords    []string
	intellectualKeywords []string
	motorKeywords        []string
}

func NewCenterRouter() *CenterRouter {
	return &CenterRouter{
		emotionalKeywords: []string{
			"sinto", "emoção", "angústia", "medo", "amor",
			"raiva", "tristeza", "ansiedade", "frustração",
			"feliz", "preocupado", "estressado",
		},
		intellectualKeywords: []string{
			"penso", "acho", "analisar", "entender", "explicar",
			"por que", "como", "razão", "lógica", "conceito",
			"teoria", "princípio",
		},
		motorKeywords: []string{
			"fazer", "ação", "executar", "implementar", "passo",
			"começar", "terminar", "construir", "criar",
			"hábito", "rotina", "prática",
		},
	}
}

// DetectCenter - Detecta qual centro está ativo
func (cr *CenterRouter) DetectCenter(
	input string,
	userModel *models.UserModel,
) models.Center {

	inputLower := strings.ToLower(input)

	scores := map[models.Center]int{
		models.CenterEmotional:    0,
		models.CenterIntellectual: 0,
		models.CenterMotor:        0,
	}

	// Score baseado em keywords
	for _, kw := range cr.emotionalKeywords {
		if strings.Contains(inputLower, kw) {
			scores[models.CenterEmotional]++
		}
	}

	for _, kw := range cr.intellectualKeywords {
		if strings.Contains(inputLower, kw) {
			scores[models.CenterIntellectual]++
		}
	}

	for _, kw := range cr.motorKeywords {
		if strings.Contains(inputLower, kw) {
			scores[models.CenterMotor]++
		}
	}

	// Adiciona contexto do userModel se disponível
	if userModel != nil && userModel.EmotionalTone.Intensity > 0.7 {
		scores[models.CenterEmotional] += 2
	}

	// Retorna centro com maior score
	maxScore := 0
	activeCenter := models.CenterUnknown

	for center, score := range scores {
		if score > maxScore {
			maxScore = score
			activeCenter = center
		}
	}

	if activeCenter == models.CenterUnknown {
		return models.CenterIntellectual // Default para Tipo 5/Arquiteto
	}

	return activeCenter
}

// RouteResponse - Define estratégia baseada no centro
func (cr *CenterRouter) RouteResponse(
	center models.Center,
) Strategy {

	switch center {
	case models.CenterEmotional:
		return StrategyEmotionalContainment
	case models.CenterIntellectual:
		return StrategyAnalytical
	case models.CenterMotor:
		return StrategyActionable
	default:
		return StrategyDirect
	}
}
