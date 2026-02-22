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

// ZaratustraService orquestra a evolução autônoma de energia e snapshots temporais
type ZaratustraService struct {
	client *nietzscheInfra.GraphAdapter
}

// NewZaratustraService cria um novo serviço de evolução
func NewZaratustraService(client *nietzscheInfra.GraphAdapter) *ZaratustraService {
	return &ZaratustraService{client: client}
}

// RunEvolutionCycle executa um ciclo completo de Zaratustra (Vontade de Poder + Retorno Eterno)
func (z *ZaratustraService) RunEvolutionCycle(ctx context.Context, collection string) (*nietzsche.ZaratustraResult, error) {
	sdk := z.client.SDK()
	if sdk == nil {
		return nil, fmt.Errorf("nietzsche sdk not available")
	}

	log.Printf("🦅 [ZARATUSTRA] Iniciando ciclo de evolução na coleção: %s", collection)

	// Parâmetros padrão para o Ubermensch
	opts := nietzsche.ZaratustraOpts{
		Collection: collection,
		Alpha:      0.15, // Propagação de energia
		Decay:      0.05, // Decaimento natural
		Cycles:     3,    // Número de iterações de propagação
	}

	result, err := sdk.InvokeZaratustra(ctx, opts)
	if err != nil {
		log.Printf("❌ [ZARATUSTRA] Erro no ciclo de evolução: %v", err)
		return nil, err
	}

	log.Printf("✅ [ZARATUSTRA] Ciclo concluído. Nós atualizados: %d | Elite identificada: %d",
		result.NodesUpdated, result.EliteCount)

	if result.EliteCount > 0 {
		log.Printf("⭐ [ZARATUSTRA] Conceitos Übermensch detectados: %v", result.EliteNodeIDs)
	}

	return result, nil
}

// GetWillToPowerStats retorna estatísticas da "Vontade de Poder" (distribuição de energia)
func (z *ZaratustraService) GetWillToPowerStats(ctx context.Context, collection string) (map[string]interface{}, error) {
	sdk := z.client.SDK()
	if sdk == nil {
		return nil, fmt.Errorf("nietzsche sdk not available")
	}

	stats, err := sdk.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"node_count":     stats.NodeCount,
		"sensory_count":  stats.SensoryCount,
		"evolution_meta": "NietzscheDB-v1 (Zaratustra Engine)",
	}, nil
}
