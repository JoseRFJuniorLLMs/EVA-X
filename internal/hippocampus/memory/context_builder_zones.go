// Package memory - Context Builder with Edge Zones Integration
// Injeta associações consolidated automaticamente no contexto Gemini
// Fase C do plano de implementação
package memory

import (
	"context"
	"fmt"
	"strings"
)

// ContextBuilderWithZones constrói contexto com associações consolidated
type ContextBuilderWithZones struct {
	edgeClassifier *EdgeClassifier
}

// NewContextBuilderWithZones cria um novo builder
func NewContextBuilderWithZones(edgeClassifier *EdgeClassifier) *ContextBuilderWithZones {
	return &ContextBuilderWithZones{
		edgeClassifier: edgeClassifier,
	}
}

// BuildUnifiedContextWithZones constrói contexto unificado incluindo associações consolidated
// Essas associações são injetadas ANTES do contexto de memórias
func (b *ContextBuilderWithZones) BuildUnifiedContextWithZones(
	ctx context.Context,
	patientID int64,
	memories []*SearchResult,
	personalityTraits map[string]float64,
) (string, error) {

	var contextParts []string

	// 1. Header
	contextParts = append(contextParts, "# Contexto Unificado - EVA-Mind")
	contextParts = append(contextParts, "")

	// 2. ✅ NOVO: Associações Consolidated (preload automático)
	consolidated, err := b.edgeClassifier.GetConsolidatedEdges(ctx, patientID)
	if err == nil && len(consolidated) > 0 {
		contextParts = append(contextParts, "## Associações Consolidadas (Alta Confiança)")
		contextParts = append(contextParts, "")
		contextParts = append(contextParts, "*Estas associações são fortes e bem estabelecidas no histórico do paciente:*")
		contextParts = append(contextParts, "")

		for _, edge := range consolidated {
			// Formatar associação com força e metadata
			line := fmt.Sprintf("- **%s** ↔ **%s**", edge.NodeAName, edge.NodeBName)
			line += fmt.Sprintf(" (força: %.2f", edge.Weight)

			if edge.CoActivations > 0 {
				line += fmt.Sprintf(", co-ativações: %d", edge.CoActivations)
			}

			// Indicar se é principalmente semântico (slow) ou comportamental (fast)
			if edge.SlowWeight > 0 && edge.FastWeight > 0 {
				if edge.SlowWeight > edge.FastWeight {
					line += ", base: **semântica**"
				} else {
					line += ", base: **uso/comportamento**"
				}
			}

			line += ")"
			contextParts = append(contextParts, line)
		}

		contextParts = append(contextParts, "")
		contextParts = append(contextParts, fmt.Sprintf("*Total: %d associações consolidadas pré-carregadas*", len(consolidated)))
		contextParts = append(contextParts, "")
	}

	// 3. Personality Traits (se fornecidos)
	if len(personalityTraits) > 0 {
		contextParts = append(contextParts, "## Traços de Personalidade")
		contextParts = append(contextParts, "")

		for trait, value := range personalityTraits {
			intensity := getIntensityLabel(value)
			contextParts = append(contextParts, fmt.Sprintf("- **%s**: %.2f (%s)", trait, value, intensity))
		}

		contextParts = append(contextParts, "")
	}

	// 4. Memórias Relevantes
	if len(memories) > 0 {
		contextParts = append(contextParts, "## Memórias Relevantes")
		contextParts = append(contextParts, "")

		for i, result := range memories {
			mem := result.Memory
			contextParts = append(contextParts, fmt.Sprintf("### Memória %d (similaridade: %.2f)", i+1, result.Similarity))
			contextParts = append(contextParts, fmt.Sprintf("**Data:** %s", mem.Timestamp.Format("2006-01-02 15:04")))
			contextParts = append(contextParts, fmt.Sprintf("**Conteúdo:** %s", mem.Content))

			if mem.Emotion != "" {
				contextParts = append(contextParts, fmt.Sprintf("**Emoção:** %s", mem.Emotion))
			}

			if len(mem.Topics) > 0 {
				contextParts = append(contextParts, fmt.Sprintf("**Tópicos:** %s", strings.Join(mem.Topics, ", ")))
			}

			contextParts = append(contextParts, "")
		}
	}

	// 5. Instruções para Gemini
	contextParts = append(contextParts, "## Instruções")
	contextParts = append(contextParts, "")
	contextParts = append(contextParts, "Use as **Associações Consolidadas** como contexto primário para entender relações fortes no histórico do paciente.")
	contextParts = append(contextParts, "Combine estas associações com as memórias relevantes para gerar uma resposta contextualizada.")
	contextParts = append(contextParts, "")

	return strings.Join(contextParts, "\n"), nil
}

// BuildConsolidatedAssociationsContext constrói apenas o contexto de associações
// Útil para incluir em outros contextos
func (b *ContextBuilderWithZones) BuildConsolidatedAssociationsContext(
	ctx context.Context,
	patientID int64,
) (string, error) {

	consolidated, err := b.edgeClassifier.GetConsolidatedEdges(ctx, patientID)
	if err != nil {
		return "", err
	}

	if len(consolidated) == 0 {
		return "", nil
	}

	var lines []string
	lines = append(lines, "**Associações Consolidadas:**")

	for _, edge := range consolidated {
		line := fmt.Sprintf("- %s ↔ %s (%.2f)", edge.NodeAName, edge.NodeBName, edge.Weight)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

// GetConsolidatedForContext retorna associações formatadas para injeção em contexto
func (b *ContextBuilderWithZones) GetConsolidatedForContext(
	ctx context.Context,
	patientID int64,
	limit int,
) ([]string, error) {

	consolidated, err := b.edgeClassifier.GetConsolidatedEdges(ctx, patientID)
	if err != nil {
		return nil, err
	}

	// Limitar quantidade se necessário
	if limit > 0 && len(consolidated) > limit {
		consolidated = consolidated[:limit]
	}

	associations := make([]string, 0, len(consolidated))
	for _, edge := range consolidated {
		assoc := fmt.Sprintf("%s ↔ %s", edge.NodeAName, edge.NodeBName)
		associations = append(associations, assoc)
	}

	return associations, nil
}

// Helper functions

func getIntensityLabel(value float64) string {
	switch {
	case value >= 0.8:
		return "muito alto"
	case value >= 0.6:
		return "alto"
	case value >= 0.4:
		return "moderado"
	case value >= 0.2:
		return "baixo"
	default:
		return "muito baixo"
	}
}

// PreloadConsolidatedInContext injeta associações no início do contexto
// Wrapper simples para uso rápido
func PreloadConsolidatedInContext(
	ctx context.Context,
	edgeClassifier *EdgeClassifier,
	patientID int64,
	existingContext string,
) (string, error) {

	builder := NewContextBuilderWithZones(edgeClassifier)
	consolidatedContext, err := builder.BuildConsolidatedAssociationsContext(ctx, patientID)
	if err != nil || consolidatedContext == "" {
		return existingContext, err
	}

	// Injetar no início do contexto existente
	return consolidatedContext + "\n\n" + existingContext, nil
}
