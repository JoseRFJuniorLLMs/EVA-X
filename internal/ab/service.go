// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ab

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
)

// TestConfig representa configuração de teste A/B
type TestConfig struct {
	TestName         string
	PercentageGroupA int
	GroupAName       string
	GroupBName       string
}

// ABTestService gerencia testes A/B
type ABTestService struct {
	db *database.DB
}

// NewABTestService cria novo serviço de A/B testing
func NewABTestService(db *database.DB) *ABTestService {
	return &ABTestService{db: db}
}

// AssignGroup atribui usuário a um grupo de teste
func (s *ABTestService) AssignGroup(testName string, idosoID int64) (string, error) {
	ctx := context.Background()

	// Check if assignment already exists
	rows, err := s.db.QueryByLabel(ctx, "ab_test_assignments",
		" AND n.test_name = $test AND n.idoso_id = $idoso",
		map[string]interface{}{"test": testName, "idoso": idosoID}, 1)
	if err == nil && len(rows) > 0 {
		return database.GetString(rows[0], "group_name"), nil
	}

	// Assign using hash-based method
	group := HashBasedAssignment(idosoID, testName, 50)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Insert(ctx, "ab_test_assignments", map[string]interface{}{
		"test_name":  testName,
		"idoso_id":   idosoID,
		"group_name": group,
		"created_at": now,
	})
	if err != nil {
		return "", fmt.Errorf("erro ao atribuir grupo: %w", err)
	}

	log.Printf("[A/B TEST] Usuario %d atribuido ao grupo: %s (teste: %s)", idosoID, group, testName)
	return group, nil
}

// ShouldUseThinkingMode verifica se usuário deve usar Thinking Mode
func (s *ABTestService) ShouldUseThinkingMode(idosoID int64) bool {
	group, err := s.AssignGroup("health_triage_mode", idosoID)
	if err != nil {
		log.Printf("Erro no A/B test, usando modo padrao: %v", err)
		return true // Fallback para Thinking Mode
	}

	return group == "thinking_mode"
}

// LogMetric registra métrica de teste A/B
func (s *ABTestService) LogMetric(testName string, idosoID int64, metricType string, value float64, metadata map[string]interface{}) error {
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)

	content := map[string]interface{}{
		"test_name":   testName,
		"idoso_id":    idosoID,
		"metric_type": metricType,
		"value":       value,
		"created_at":  now,
	}

	if metadata != nil {
		metadataJSON, _ := json.Marshal(metadata)
		content["metadata"] = string(metadataJSON)
	}

	_, err := s.db.Insert(ctx, "ab_test_metrics", content)
	if err != nil {
		return fmt.Errorf("erro ao registrar metrica: %w", err)
	}

	return nil
}

// LogResponseTime registra tempo de resposta
func (s *ABTestService) LogResponseTime(idosoID int64, responseTimeMs float64) error {
	return s.LogMetric("health_triage_mode", idosoID, "response_time_ms", responseTimeMs, nil)
}

// LogDetectionAccuracy registra precisão de detecção
func (s *ABTestService) LogDetectionAccuracy(idosoID int64, wasCorrect bool) error {
	accuracy := 0.0
	if wasCorrect {
		accuracy = 1.0
	}
	return s.LogMetric("health_triage_mode", idosoID, "detection_accuracy", accuracy, nil)
}

// LogUserSatisfaction registra satisfação do usuário (1-5)
func (s *ABTestService) LogUserSatisfaction(idosoID int64, rating float64) error {
	return s.LogMetric("health_triage_mode", idosoID, "user_satisfaction", rating, nil)
}

// LogCostPerInteraction registra custo por interação
func (s *ABTestService) LogCostPerInteraction(idosoID int64, costUSD float64) error {
	return s.LogMetric("health_triage_mode", idosoID, "cost_per_interaction_usd", costUSD, nil)
}

// LogFalsePositiveRate registra taxa de falsos positivos
func (s *ABTestService) LogFalsePositiveRate(idosoID int64, wasFalsePositive bool) error {
	rate := 0.0
	if wasFalsePositive {
		rate = 1.0
	}
	return s.LogMetric("health_triage_mode", idosoID, "false_positive", rate, nil)
}

// GetTestResults retorna resultados do teste A/B
func (s *ABTestService) GetTestResults(testName string) ([]TestResult, error) {
	ctx := context.Background()

	// Get all metrics for this test
	metrics, err := s.db.QueryByLabel(ctx, "ab_test_metrics",
		" AND n.test_name = $test",
		map[string]interface{}{"test": testName}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar resultados: %w", err)
	}

	// Get all assignments for this test
	assignments, err := s.db.QueryByLabel(ctx, "ab_test_assignments",
		" AND n.test_name = $test",
		map[string]interface{}{"test": testName}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar atribuicoes: %w", err)
	}

	// Map idoso_id -> group
	idosoGroup := make(map[int64]string)
	for _, a := range assignments {
		idosoGroup[database.GetInt64(a, "idoso_id")] = database.GetString(a, "group_name")
	}

	// Aggregate metrics by metric_type and group
	type groupStats struct {
		sum     float64
		count   int
	}
	// metricType -> groupName -> stats
	aggregated := make(map[string]map[string]*groupStats)

	for _, m := range metrics {
		metricType := database.GetString(m, "metric_type")
		idosoID := database.GetInt64(m, "idoso_id")
		value := database.GetFloat64(m, "value")
		group := idosoGroup[idosoID]
		if group == "" {
			continue
		}

		if aggregated[metricType] == nil {
			aggregated[metricType] = make(map[string]*groupStats)
		}
		if aggregated[metricType][group] == nil {
			aggregated[metricType][group] = &groupStats{}
		}
		aggregated[metricType][group].sum += value
		aggregated[metricType][group].count++
	}

	var results []TestResult
	for metricType, groups := range aggregated {
		r := TestResult{Metric: metricType}
		for groupName, stats := range groups {
			mean := 0.0
			if stats.count > 0 {
				mean = stats.sum / float64(stats.count)
			}
			if r.GroupA == "" {
				r.GroupA = groupName
				r.MeanA = mean
				r.SamplesA = stats.count
			} else {
				r.GroupB = groupName
				r.MeanB = mean
				r.SamplesB = stats.count
			}
		}
		if r.MeanA != 0 {
			r.DifferencePercent = ((r.MeanB - r.MeanA) / r.MeanA) * 100
		}
		if r.MeanA > r.MeanB {
			r.Winner = r.GroupA
		} else if r.MeanB > r.MeanA {
			r.Winner = r.GroupB
		} else {
			r.Winner = "tie"
		}
		results = append(results, r)
	}

	return results, nil
}

// TestResult representa resultado de comparação A/B
type TestResult struct {
	Metric            string
	GroupA            string
	MeanA             float64
	SamplesA          int
	GroupB            string
	MeanB             float64
	SamplesB          int
	DifferencePercent float64
	Winner            string
}

// GetUserDistribution retorna distribuição de usuários por grupo
func (s *ABTestService) GetUserDistribution(testName string) ([]GroupDistribution, error) {
	ctx := context.Background()

	assignments, err := s.db.QueryByLabel(ctx, "ab_test_assignments",
		" AND n.test_name = $test",
		map[string]interface{}{"test": testName}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar distribuicao: %w", err)
	}

	groupCounts := make(map[string]int)
	total := 0
	for _, a := range assignments {
		group := database.GetString(a, "group_name")
		groupCounts[group]++
		total++
	}

	var distribution []GroupDistribution
	for group, count := range groupCounts {
		pct := 0.0
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		distribution = append(distribution, GroupDistribution{
			Group:      group,
			TotalUsers: count,
			Percentage: pct,
		})
	}

	return distribution, nil
}

// GroupDistribution representa distribuição de usuários
type GroupDistribution struct {
	Group      string
	TotalUsers int
	Percentage float64
}

// HashBasedAssignment atribui grupo baseado em hash (cliente-side fallback)
func HashBasedAssignment(idosoID int64, testName string, percentageA int) string {
	hash := md5.Sum([]byte(fmt.Sprintf("%s_%d", testName, idosoID)))
	hashStr := hex.EncodeToString(hash[:])

	// Converter primeiro byte para 0-100
	firstByte := int(hashStr[0])
	bucket := firstByte % 100

	if bucket < percentageA {
		return "thinking_mode"
	}
	return "normal_mode"
}
