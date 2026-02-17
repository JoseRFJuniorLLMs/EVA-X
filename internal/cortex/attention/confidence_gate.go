// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package attention

// ConfidenceGate - Implementa inibição forte (não responde sem confiança)
type ConfidenceGate struct {
	threshold float64
}

func NewConfidenceGate(threshold float64) *ConfidenceGate {
	return &ConfidenceGate{
		threshold: threshold,
	}
}

// ShouldProceed - Verifica se deve prosseguir com resposta
func (cg *ConfidenceGate) ShouldProceed(confidence float64) bool {
	return confidence >= cg.threshold
}

// GenerateClarification - Gera pergunta clarificadora
func (cg *ConfidenceGate) GenerateClarification(
	uncertaintyAreas []string,
) string {

	if len(uncertaintyAreas) == 0 {
		return "Pode esclarecer o que você precisa?"
	}

	// Retorna a primeira área de incerteza
	return "Sobre " + uncertaintyAreas[0] + ", você pode detalhar?"
}

// AssessConfidence - Avalia confiança baseada em múltiplos fatores
func (cg *ConfidenceGate) AssessConfidence(
	intentClarity float64,
	contextDepth float64,
	taskComplexity float64,
) float64 {

	// Confiança = função de clareza + contexto - complexidade

	weights := struct {
		clarity    float64
		context    float64
		complexity float64
	}{
		clarity:    0.5,
		context:    0.3,
		complexity: 0.2,
	}

	confidence := (intentClarity * weights.clarity) +
		(contextDepth * weights.context) -
		(taskComplexity * weights.complexity)

	// Clamp entre 0 e 1
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	return confidence
}
