package attention

// MinimalOptimizer - Otimiza para resposta mínima suficiente
type MinimalOptimizer struct {
	maxTokens int
}

func NewMinimalOptimizer(maxTokens int) *MinimalOptimizer {
	return &MinimalOptimizer{
		maxTokens: maxTokens,
	}
}

// OptimizeLength - Calcula comprimento ótimo
func (mo *MinimalOptimizer) OptimizeLength(
	complexity float64,
	clarity float64,
) int {

	// Fórmula: minimize(tokens) sujeito a clarity ≥ threshold

	// Complexidade alta = mais tokens necessários
	// Clareza alta = menos tokens necessários

	baseTokens := 50
	complexityFactor := complexity * 100
	clarityFactor := (1.0 - clarity) * 50

	optimal := int(float64(baseTokens) + complexityFactor + clarityFactor)

	// Cap no máximo
	if optimal > mo.maxTokens {
		optimal = mo.maxTokens
	}

	// Mínimo de 20 tokens
	if optimal < 20 {
		optimal = 20
	}

	return optimal
}

// ShouldExpandResponse - Decide se deve expandir resposta
func (mo *MinimalOptimizer) ShouldExpandResponse(
	currentLength int,
	targetClarity float64,
	achievedClarity float64,
) bool {

	// Só expande se:
	// 1. Ainda não atingiu clareza suficiente
	// 2. Ainda tem espaço no budget de tokens

	return achievedClarity < targetClarity && currentLength < mo.maxTokens
}
