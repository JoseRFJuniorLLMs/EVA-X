// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package transnar

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// ABTestVariant representa uma variante de teste A/B
type ABTestVariant string

const (
	VariantControl    ABTestVariant = "control"    // Estratégia padrão
	VariantAggressive ABTestVariant = "aggressive" // Interpelação mais frequente
	VariantEmpathetic ABTestVariant = "empathetic" // Foco em reflexão
	VariantDirective  ABTestVariant = "directive"  // Mais diretivo/pontuação
)

// ABTestConfig configuração do teste A/B
type ABTestConfig struct {
	Enabled     bool
	Variants    map[ABTestVariant]float64 // Distribuição de tráfego
	MinSessions int                       // Mínimo de sessões para significância
}

// ABTestMetrics métricas coletadas por variante
type ABTestMetrics struct {
	Variant              ABTestVariant
	TotalSessions        int
	TotalInterventions   int
	AvgConfidence        float64
	AvgConversationTurns int
	CriticalCasesHandled int     // Casos de pulsão de morte
	UserEngagement       float64 // 0-1 (baseado em respostas)
	mu                   sync.RWMutex
}

// ABTestManager gerencia testes A/B
type ABTestManager struct {
	config  ABTestConfig
	metrics map[ABTestVariant]*ABTestMetrics
	mu      sync.RWMutex
}

// NewABTestManager cria um novo gerenciador de testes A/B
func NewABTestManager() *ABTestManager {
	return &ABTestManager{
		config: ABTestConfig{
			Enabled: true,
			Variants: map[ABTestVariant]float64{
				VariantControl:    0.40, // 40% controle
				VariantAggressive: 0.30, // 30% agressivo
				VariantEmpathetic: 0.20, // 20% empático
				VariantDirective:  0.10, // 10% diretivo
			},
			MinSessions: 100, // Mínimo para significância estatística
		},
		metrics: make(map[ABTestVariant]*ABTestMetrics),
	}
}

// AssignVariant atribui uma variante baseada no user ID (consistente)
func (m *ABTestManager) AssignVariant(userID int64) ABTestVariant {
	if !m.config.Enabled {
		return VariantControl
	}

	// Hash do userID para distribuição consistente
	hash := md5.Sum([]byte(fmt.Sprintf("%d", userID)))
	hashStr := hex.EncodeToString(hash[:])

	// Converter hash para número 0-1
	var hashValue float64
	fmt.Sscanf(hashStr[:8], "%x", &hashValue)
	hashValue = hashValue / float64(0xFFFFFFFF)

	// Distribuir baseado nos pesos
	cumulative := 0.0
	for variant, weight := range m.config.Variants {
		cumulative += weight
		if hashValue <= cumulative {
			return variant
		}
	}

	return VariantControl
}

// GetStrategy retorna a estratégia baseada na variante
func (m *ABTestManager) GetStrategy(
	variant ABTestVariant,
	desire *DesireInference,
	chain *SignifierChain,
) ResponseStrategy {

	switch variant {
	case VariantAggressive:
		// Sempre interpelar se confiança > 0.5
		if desire.Confidence > 0.5 {
			return Interpellation
		}
		return Reflection

	case VariantEmpathetic:
		// Priorizar reflexão e validação
		if chain.Intensity > 0.6 {
			return Reflection
		}
		return Punctuation

	case VariantDirective:
		// Mais diretivo, usar pontuação
		if desire.Confidence > 0.6 {
			return Punctuation
		}
		return Reflection

	case VariantControl:
		fallthrough
	default:
		// Estratégia padrão (original)
		generator := NewResponseGenerator()
		return generator.SelectStrategy(desire, chain)
	}
}

// RecordIntervention registra uma intervenção
func (m *ABTestManager) RecordIntervention(
	variant ABTestVariant,
	desire *DesireInference,
	conversationTurns int,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.metrics[variant] == nil {
		m.metrics[variant] = &ABTestMetrics{
			Variant: variant,
		}
	}

	metric := m.metrics[variant]
	metric.mu.Lock()
	defer metric.mu.Unlock()

	metric.TotalInterventions++

	// Atualizar média de confiança
	metric.AvgConfidence = (metric.AvgConfidence*float64(metric.TotalInterventions-1) + desire.Confidence) / float64(metric.TotalInterventions)

	// Atualizar média de turnos
	metric.AvgConversationTurns = (metric.AvgConversationTurns*(metric.TotalInterventions-1) + conversationTurns) / metric.TotalInterventions

	// Casos críticos
	if desire.Desire == DesireRelief && desire.Confidence > 0.8 {
		metric.CriticalCasesHandled++
	}
}

// RecordSession registra uma sessão completa
func (m *ABTestManager) RecordSession(variant ABTestVariant, engagement float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.metrics[variant] == nil {
		m.metrics[variant] = &ABTestMetrics{
			Variant: variant,
		}
	}

	metric := m.metrics[variant]
	metric.mu.Lock()
	defer metric.mu.Unlock()

	metric.TotalSessions++

	// Atualizar engajamento médio
	metric.UserEngagement = (metric.UserEngagement*float64(metric.TotalSessions-1) + engagement) / float64(metric.TotalSessions)
}

// GetReport gera relatório de A/B testing
func (m *ABTestManager) GetReport() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	report += "📊 A/B TESTING REPORT - TransNAR\n"
	report += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"

	for variant, metric := range m.metrics {
		metric.mu.RLock()

		report += fmt.Sprintf("Variant: %s\n", variant)
		report += fmt.Sprintf("  Sessions: %d\n", metric.TotalSessions)
		report += fmt.Sprintf("  Interventions: %d\n", metric.TotalInterventions)
		report += fmt.Sprintf("  Avg Confidence: %.2f\n", metric.AvgConfidence)
		report += fmt.Sprintf("  Avg Conv Turns: %d\n", metric.AvgConversationTurns)
		report += fmt.Sprintf("  Critical Cases: %d\n", metric.CriticalCasesHandled)
		report += fmt.Sprintf("  User Engagement: %.2f\n", metric.UserEngagement)

		// Significância estatística
		if metric.TotalSessions >= m.config.MinSessions {
			report += "  ✅ Statistically significant\n"
		} else {
			report += fmt.Sprintf("  ⚠️ Need %d more sessions\n", m.config.MinSessions-metric.TotalSessions)
		}

		report += "\n"
		metric.mu.RUnlock()
	}

	// Recomendação
	report += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	report += "RECOMMENDATION:\n"

	bestVariant := m.getBestVariant()
	if bestVariant != "" {
		report += fmt.Sprintf("  🏆 Best Performer: %s\n", bestVariant)
		report += "  Consider rolling out to 100% of users.\n"
	} else {
		report += "  ⏳ Insufficient data for recommendation.\n"
	}

	return report
}

// getBestVariant retorna a melhor variante baseada em métricas
func (m *ABTestManager) getBestVariant() ABTestVariant {
	var bestVariant ABTestVariant
	bestScore := 0.0

	for variant, metric := range m.metrics {
		if metric.TotalSessions < m.config.MinSessions {
			continue // Não tem dados suficientes
		}

		metric.mu.RLock()
		// Score composto: engagement (60%) + critical cases (40%)
		score := metric.UserEngagement*0.6 + (float64(metric.CriticalCasesHandled)/float64(metric.TotalSessions))*0.4
		metric.mu.RUnlock()

		if score > bestScore {
			bestScore = score
			bestVariant = variant
		}
	}

	return bestVariant
}

// LogMetrics registra métricas periodicamente
func (m *ABTestManager) LogMetrics(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println(m.GetReport())
		}
	}
}
