package personality

import (
	"fmt"
	"sort"
	"time"

	"github.com/JoseRFJuniorLLMs/EVA-Mind/internal/cortex/pattern"
)

// InterpretationHypothesis represents a possible interpretation of a behavioral cue
type InterpretationHypothesis struct {
	Explanation string
	Confidence  float64
	Evidence    []string
}

// Interpretation represents the analysis of a behavioral cue with multiple hypotheses
type Interpretation struct {
	Cue           pattern.BehavioralCue
	Hypotheses    []InterpretationHypothesis
	SelectedIndex int
	Justification string
	Timestamp     time.Time
}

// UnifiedContext represents the full context available for interpretation
type UnifiedContext struct {
	RecentMemories []string
	UserProfile    map[string]interface{}
	CurrentMood    string
	RecentEvents   []string
	SessionCount   int
}

// UserFeedback represents feedback from the user about an interpretation
type UserFeedback struct {
	InterpretationID string
	WasCorrect       bool
	ActualState      string
	Comments         string
}

// GenerateHypotheses creates multiple possible interpretations for a behavioral cue
func GenerateHypotheses(cue pattern.BehavioralCue, context UnifiedContext) []InterpretationHypothesis {
	hypotheses := []InterpretationHypothesis{}

	switch cue.Type {
	case pattern.CueTypePause:
		hypotheses = generatePauseHypotheses(cue, context)
	case pattern.CueTypeIncongruence:
		hypotheses = generateIncongruenceHypotheses(cue, context)
	case pattern.CueTypeToneShift:
		hypotheses = generateToneShiftHypotheses(cue, context)
	case pattern.CueTypeRecurrence:
		hypotheses = generateRecurrenceHypotheses(cue, context)
	}

	// Sort by confidence (highest first)
	sort.Slice(hypotheses, func(i, j int) bool {
		return hypotheses[i].Confidence > hypotheses[j].Confidence
	})

	return hypotheses
}

func generatePauseHypotheses(cue pattern.BehavioralCue, context UnifiedContext) []InterpretationHypothesis {
	duration := cue.Metadata["duration_seconds"].(float64)

	hypotheses := []InterpretationHypothesis{
		{
			Explanation: "Usuário está pensando/refletindo",
			Confidence:  0.4,
			Evidence:    []string{"Pausa natural em conversação"},
		},
		{
			Explanation: "Usuário está emocionalmente sobrecarregado (chorando ou muito emocionado)",
			Confidence:  0.5,
			Evidence:    []string{fmt.Sprintf("Pausa longa (%.1fs)", duration)},
		},
		{
			Explanation: "Usuário desconectou ou teve problema técnico",
			Confidence:  0.1,
			Evidence:    []string{"Silêncio prolongado sem contexto emocional"},
		},
	}

	// Adjust based on context
	if duration > 10 {
		hypotheses[1].Confidence += 0.2 // More likely emotional
		hypotheses[2].Confidence += 0.1 // Also more likely technical issue
	}

	// Check for recent emotional events
	for _, event := range context.RecentEvents {
		if contains(event, []string{"morte", "luto", "perda"}) {
			hypotheses[1].Confidence += 0.3
			hypotheses[1].Evidence = append(hypotheses[1].Evidence, "Evento emocional recente: "+event)
		}
	}

	// Normalize confidences
	return normalizeConfidences(hypotheses)
}

func generateIncongruenceHypotheses(cue pattern.BehavioralCue, context UnifiedContext) []InterpretationHypothesis {
	hypotheses := []InterpretationHypothesis{
		{
			Explanation: "Usuário está tentando esconder emoções negativas (máscara social)",
			Confidence:  0.7,
			Evidence:    []string{"Palavras positivas com tom negativo"},
		},
		{
			Explanation: "Usuário está em negação sobre seu estado emocional",
			Confidence:  0.5,
			Evidence:    []string{"Incongruência entre fala e emoção"},
		},
		{
			Explanation: "Usuário está cansado mas tentando parecer bem",
			Confidence:  0.4,
			Evidence:    []string{"Baixa energia apesar de palavras positivas"},
		},
	}

	// Adjust based on context
	if context.CurrentMood == "deprimido" || context.CurrentMood == "ansioso" {
		hypotheses[0].Confidence += 0.15
		hypotheses[0].Evidence = append(hypotheses[0].Evidence, "Humor atual: "+context.CurrentMood)
	}

	return normalizeConfidences(hypotheses)
}

func generateToneShiftHypotheses(cue pattern.BehavioralCue, context UnifiedContext) []InterpretationHypothesis {
	toneChange, hasToneChange := cue.Metadata["tone_change"].(float64)

	hypotheses := []InterpretationHypothesis{}

	if hasToneChange && toneChange < 0 {
		// Shift to negative tone
		hypotheses = []InterpretationHypothesis{
			{
				Explanation: "Usuário tocou em tópico emocionalmente difícil",
				Confidence:  0.7,
				Evidence:    []string{"Mudança súbita para tom negativo"},
			},
			{
				Explanation: "Usuário lembrou de algo triste",
				Confidence:  0.6,
				Evidence:    []string{"Transição emocional abrupta"},
			},
			{
				Explanation: "Usuário está ficando cansado/frustrado",
				Confidence:  0.3,
				Evidence:    []string{"Declínio no tom"},
			},
		}
	} else {
		// Shift to positive tone
		hypotheses = []InterpretationHypothesis{
			{
				Explanation: "Usuário está tentando mudar de assunto (evitação)",
				Confidence:  0.6,
				Evidence:    []string{"Mudança súbita para tom positivo"},
			},
			{
				Explanation: "Usuário lembrou de algo bom",
				Confidence:  0.5,
				Evidence:    []string{"Transição emocional positiva"},
			},
			{
				Explanation: "Usuário está se recuperando emocionalmente",
				Confidence:  0.4,
				Evidence:    []string{"Melhora no tom"},
			},
		}
	}

	return normalizeConfidences(hypotheses)
}

func generateRecurrenceHypotheses(cue pattern.BehavioralCue, context UnifiedContext) []InterpretationHypothesis {
	signifier := cue.Metadata["signifier"].(string)
	count := cue.Metadata["count"].(int)

	hypotheses := []InterpretationHypothesis{
		{
			Explanation: fmt.Sprintf("Preocupação central/obsessiva com '%s'", signifier),
			Confidence:  0.8,
			Evidence:    []string{fmt.Sprintf("Mencionou '%s' %d vezes", signifier, count)},
		},
		{
			Explanation: fmt.Sprintf("Trauma não resolvido relacionado a '%s'", signifier),
			Confidence:  0.6,
			Evidence:    []string{"Recorrência excessiva de significante"},
		},
		{
			Explanation: "Usuário está tentando processar emoção difícil",
			Confidence:  0.5,
			Evidence:    []string{"Repetição como mecanismo de processamento"},
		},
	}

	// Critical signifiers get higher confidence for trauma hypothesis
	criticalSignifiers := []string{"morte", "morrer", "abandono", "não aguento"}
	for _, critical := range criticalSignifiers {
		if signifier == critical {
			hypotheses[1].Confidence += 0.2
			hypotheses[1].Evidence = append(hypotheses[1].Evidence, "Significante crítico detectado")
			break
		}
	}

	return normalizeConfidences(hypotheses)
}

// SelectBestInterpretation selects the most likely interpretation and provides justification
func SelectBestInterpretation(hypotheses []InterpretationHypothesis) (int, string) {
	if len(hypotheses) == 0 {
		return -1, "Nenhuma hipótese disponível"
	}

	// Already sorted by confidence, so select first
	selectedIndex := 0
	best := hypotheses[selectedIndex]

	// Build justification
	justification := fmt.Sprintf(
		"Selecionei a interpretação '%s' (confiança: %.0f%%) porque: %s",
		best.Explanation,
		best.Confidence*100,
		joinEvidence(best.Evidence),
	)

	// Mention alternative if confidence is not very high
	if best.Confidence < 0.7 && len(hypotheses) > 1 {
		second := hypotheses[1]
		justification += fmt.Sprintf(
			". No entanto, também considero possível que %s (confiança: %.0f%%).",
			second.Explanation,
			second.Confidence*100,
		)
	}

	return selectedIndex, justification
}

// ValidateInterpretation updates accuracy based on user feedback
func ValidateInterpretation(interp Interpretation, feedback UserFeedback) float64 {
	selected := interp.Hypotheses[interp.SelectedIndex]

	if feedback.WasCorrect {
		// Interpretation was correct
		return selected.Confidence
	}

	// Interpretation was incorrect
	// Find if any other hypothesis matches the actual state
	for i, hyp := range interp.Hypotheses {
		if i != interp.SelectedIndex && contains(hyp.Explanation, []string{feedback.ActualState}) {
			// We had the right hypothesis but didn't select it
			return hyp.Confidence
		}
	}

	// We didn't even consider the correct interpretation
	return 0.0
}

// Helper functions

func normalizeConfidences(hypotheses []InterpretationHypothesis) []InterpretationHypothesis {
	total := 0.0
	for _, h := range hypotheses {
		total += h.Confidence
	}

	if total == 0 {
		return hypotheses
	}

	for i := range hypotheses {
		hypotheses[i].Confidence = hypotheses[i].Confidence / total
	}

	return hypotheses
}

func contains(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if len(text) >= len(keyword) && text[:len(keyword)] == keyword {
			return true
		}
	}
	return false
}

func joinEvidence(evidence []string) string {
	if len(evidence) == 0 {
		return "sem evidências específicas"
	}
	if len(evidence) == 1 {
		return evidence[0]
	}
	result := evidence[0]
	for i := 1; i < len(evidence); i++ {
		result += ", " + evidence[i]
	}
	return result
}
