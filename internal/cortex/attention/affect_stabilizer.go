// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package attention

import "eva-mind/internal/cortex/attention/models"

// AffectStabilizer - Mantém afeto estável (não espelha usuário)
type AffectStabilizer struct {
	baseline models.AffectState
}

func NewAffectStabilizer() *AffectStabilizer {
	return &AffectStabilizer{
		baseline: models.AffectNeutralClear,
	}
}

// Stabilize - Retorna sempre baseline, nunca espelha
func (as *AffectStabilizer) Stabilize(
	userEmotion models.EmotionalState,
) models.AffectState {

	// CRÍTICO: Não importa o estado emocional do usuário
	// EVA mantém afeto estável

	// Tipo 5 evitaria emoção
	// Gurdjieff observa sem identificação

	return as.baseline
}

// ShouldMirror - SEMPRE false (não-identificação)
func (as *AffectStabilizer) ShouldMirror() bool {
	return false
}
