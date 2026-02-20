// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// DEPRECATED: Este módulo delega para internal/cortex/situation/modulator.go
// Mantido apenas para compatibilidade com personality_router.go
package personality

import (
	"time"

	"eva/internal/cortex/situation"
)

// Situation wraps situation.Situation for backward compatibility
type Situation = situation.Situation

// LifeEvent represents a significant event in user's life
type LifeEvent struct {
	Type      string
	Timestamp time.Time
	Impact    float64
}

// InferSituation delegates to situation.Modulator.Infer (simplified for backward compat)
func InferSituation(userID int64, sessionData SessionData, recentEvents []LifeEvent) Situation {
	mod := situation.NewModulator(nil, nil)

	// Build recent text from session behaviors
	recentText := ""
	for _, b := range sessionData.Behaviors {
		recentText += b + " "
	}

	// Convert LifeEvents to situation.Events
	var events []situation.Event
	for _, e := range recentEvents {
		events = append(events, situation.Event{
			Type:      e.Type,
			Content:   e.Type,
			Timestamp: e.Timestamp,
		})
	}

	sit, _ := mod.Infer(nil, "", recentText, events)
	return sit
}

// ModulateWeights delegates to situation.Modulator.ModulateWeights
func ModulateWeights(baseType int, sit Situation) map[string]float64 {
	mod := situation.NewModulator(nil, nil)
	baseWeights := situation.GetEnneagramBaseWeights(baseType)
	return mod.ModulateWeights(baseWeights, sit)
}

// GenerateSituationalGuidance generates guidance adapted to the situation
func GenerateSituationalGuidance(baseType int, sit Situation) string {
	if containsString(sit.Stressors, "luto") && sit.SocialContext == "sozinho" && sit.TimeOfDay == "madrugada" {
		return "ATENÇÃO: Usuário em luto, sozinho, de madrugada. Risco elevado de crise. Seja especialmente empática e considere acionar suporte."
	}
	if len(sit.Stressors) > 2 {
		return "Usuário sob múltiplos estressores. Abordagem gentil e validação emocional são prioritárias."
	}
	if sit.SocialContext == "sozinho" {
		return "Usuário está sozinho. EVA pode ser a única companhia no momento. Seja presente e acolhedora."
	}
	return ""
}

// GetBaseWeights delegates to situation.GetEnneagramBaseWeights
func GetBaseWeights(enneaType int) map[string]float64 {
	return situation.GetEnneagramBaseWeights(enneaType)
}
