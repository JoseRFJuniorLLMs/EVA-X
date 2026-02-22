// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package evolution

import (
	"context"
	"fmt"
	"log"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	nietzsche "nietzsche-sdk"
)

// SleepService gerencia os ciclos de reconsolidação geométrica (sono) da EVA
type SleepService struct {
	client *nietzscheInfra.GraphAdapter
}

// NewSleepService cria um novo serviço de reconsolidação
func NewSleepService(client *nietzscheInfra.GraphAdapter) *SleepService {
	return &SleepService{client: client}
}

// TriggerRiemannianSleep executa um ciclo de sono para reduzir o drift do manifold
func (s *SleepService) TriggerRiemannianSleep(ctx context.Context, collection string) (*nietzsche.SleepResult, error) {
	sdk := s.client.SDK()
	if sdk == nil {
		return nil, fmt.Errorf("nietzsche sdk not available")
	}

	log.Printf("💤 [SLEEP] Iniciando reconsolidação Riemanniana na coleção: %s", collection)

	// Parâmetros para otimização geométrica via Adam
	opts := nietzsche.SleepOpts{
		Collection:         collection,
		Noise:              0.02, // Pequena perturbação para evitar mínimos locais
		AdamSteps:          15,   // Passos de otimização
		AdamLr:             0.005,
		HausdorffThreshold: 0.1, // Tolerância máxima de drift
	}

	result, err := sdk.TriggerSleep(ctx, opts)
	if err != nil {
		log.Printf("❌ [SLEEP] Erro no ciclo de sono: %v", err)
		return nil, err
	}

	if result.Committed {
		log.Printf("✅ [SLEEP] Reconsolidação concluída e COMMITADA. ΔH: %.4f -> %.4f (Delta: %.4f)",
			result.HausdorffBefore, result.HausdorffAfter, result.HausdorffDelta)
	} else {
		log.Printf("⚠️ [SLEEP] Ciclo de sono REJEITADO (Drift acima do threshold). ΔH: %.4f", result.HausdorffDelta)
	}

	return &result, nil
}
