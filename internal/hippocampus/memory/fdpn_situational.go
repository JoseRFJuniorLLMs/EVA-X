// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"fmt"
	"log"

	"eva/internal/cortex/situation"
)

// StreamingPrimeWithSituation ativa subgrafos com modulação situacional
// Esta é a versão NOVA que integra o Situational Modulator
func (e *FDPNEngine) StreamingPrimeWithSituation(ctx context.Context, userID string, text string, recentEvents []situation.Event, modulator *situation.SituationalModulator) (map[string]float64, error) {
	if modulator == nil {
		// Fallback: priming sem modulação
		log.Printf("[FDPN] No situational modulator provided, falling back to standard priming")
		return e.streamingPrimeStandard(ctx, userID, text)
	}

	// 1. Inferir situação (<10ms)
	sit, err := modulator.Infer(ctx, userID, text, recentEvents)
	if err != nil {
		log.Printf("⚠️ [FDPN] Situation inference failed: %v, falling back", err)
		return e.streamingPrimeStandard(ctx, userID, text)
	}

	log.Printf("🧠 [FDPN] Situation detected: stressors=%v, context=%s, time=%s, emotion=%.2f, intensity=%.2f",
		sit.Stressors, sit.SocialContext, sit.TimeOfDay, sit.EmotionScore, sit.Intensity)

	// 2. Obter pesos base de personality
	baseWeights := e.getBasePersonalityWeights(userID)

	// 3. Modular pesos (<1ms)
	modulatedWeights := modulator.ModulateWeights(baseWeights, sit)

	// Log modulação
	for trait, weight := range modulatedWeights {
		if baseWeight, exists := baseWeights[trait]; exists && weight != baseWeight {
			log.Printf("   🔄 [FDPN] Modulated %s: %.2f → %.2f (%.0f%%)",
				trait, baseWeight, weight, (weight/baseWeight-1)*100)
		}
	}

	// 4. Usar pesos modulados no priming
	activatedNodes, err := e.primeWithModulatedWeights(ctx, userID, text, modulatedWeights)
	if err != nil {
		return nil, fmt.Errorf("priming with modulated weights failed: %w", err)
	}

	// 5. Alertas críticos (se necessário)
	if sit.Intensity > 0.8 && containsStressor(sit.Stressors, "crise") {
		log.Printf("🚨 [FDPN] CRITICAL ALERT: Crisis detected for user %s - intensity %.2f", userID, sit.Intensity)
		// TODO: Integrar com alert service quando disponível
		// e.alertService.SendCritical(userID, "Possível crise detectada", sit)
	}

	return activatedNodes, nil
}

// streamingPrimeStandard é o fallback quando não há modulator
func (e *FDPNEngine) streamingPrimeStandard(ctx context.Context, userID string, text string) (map[string]float64, error) {
	// Chamar o StreamingPrime original (que não retorna map)
	err := e.StreamingPrime(ctx, userID, text)
	if err != nil {
		return nil, err
	}

	// Retornar mapa vazio (compatibilidade)
	return make(map[string]float64), nil
}

// getBasePersonalityWeights obtém pesos base de personality para um usuário
// TODO: Integrar com o personality service quando disponível
func (e *FDPNEngine) getBasePersonalityWeights(userID string) map[string]float64 {
	// Por enquanto, retornar pesos default
	// Na implementação final, isso viria do banco de dados / personality service

	return map[string]float64{
		"ANSIEDADE":        0.5,
		"BUSCA_SEGURANÇA":  0.4,
		"EXTROVERSÃO":      0.6,
		"ALERTA":           0.5,
		"SOLIDÃO":          0.3,
		"TRISTEZA":         0.3,
		"ALEGRIA":          0.5,
		"DEPRESSÃO":        0.3,
		"PREOCUPAÇÃO":      0.5,
		"DESESPERO":        0.2,
	}
}

// primeWithModulatedWeights executa priming com pesos modulados
func (e *FDPNEngine) primeWithModulatedWeights(ctx context.Context, userID string, text string, weights map[string]float64) (map[string]float64, error) {
	keywords := e.extractKeywords(text)

	activatedNodes := make(map[string]float64)

	for _, kw := range keywords {
		// Calcular ativação base
		activationScore := e.calculateBaseActivation(kw)

		// Aplicar modulação se o keyword corresponde a um trait
		kwUpper := toUpperSnakeCase(kw)
		if weight, exists := weights[kwUpper]; exists {
			originalScore := activationScore
			activationScore *= weight
			log.Printf("   🎯 [FDPN] Keyword '%s' modulated: %.2f → %.2f (weight=%.2f)",
				kw, originalScore, activationScore, weight)
		}

		// Prime keyword com score modulado
		nodeID, err := e.primeKeywordWithScore(ctx, userID, kw, activationScore)
		if err != nil {
			log.Printf("⚠️ [FDPN] Failed to prime keyword '%s': %v", kw, err)
			continue
		}

		if nodeID != "" {
			activatedNodes[nodeID] = activationScore
		}
	}

	return activatedNodes, nil
}

// calculateBaseActivation calcula ativação base para um keyword
func (e *FDPNEngine) calculateBaseActivation(keyword string) float64 {
	// Ativação base proporcional ao comprimento do keyword
	// Keywords mais específicos = maior ativação
	baseActivation := 0.5
	if len(keyword) > 5 {
		baseActivation = 0.7
	}
	if len(keyword) > 10 {
		baseActivation = 0.9
	}
	return baseActivation
}

// primeKeywordWithScore prima um keyword com score específico
func (e *FDPNEngine) primeKeywordWithScore(ctx context.Context, userID string, keyword string, score float64) (string, error) {
	// Buscar nó no grafo que corresponde ao keyword via NQL
	nql := `MATCH (n) WHERE n.name = $keyword OR n.content CONTAINS $keyword RETURN n LIMIT 1`

	result, err := e.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"keyword": keyword,
	}, "")
	if err != nil {
		return "", fmt.Errorf("graph query failed: %w", err)
	}

	if result == nil || len(result.Nodes) == 0 {
		// Nó não encontrado, retornar vazio (não é erro)
		return "", nil
	}

	// Extrair nodeID do primeiro resultado
	nodeID := result.Nodes[0].ID
	if nodeID == "" {
		nodeID = fmt.Sprintf("node_%s", keyword)
	}

	// Ativar nó no cache com score
	cacheKey := fmt.Sprintf("activated:%s:%s", userID, nodeID)
	e.localCache.Store(cacheKey, score)

	log.Printf("   ✅ [FDPN] Activated node '%s' with score %.2f", nodeID, score)

	return nodeID, nil
}

// Helper functions

func containsStressor(stressors []string, item string) bool {
	for _, s := range stressors {
		if s == item {
			return true
		}
	}
	return false
}

func toUpperSnakeCase(s string) string {
	// Converte "ansiedade" → "ANSIEDADE"
	// Converte "busca_seguranca" → "BUSCA_SEGURANÇA" (simplificado)
	return s // Por enquanto, return as-is (implementar normalização depois)
}