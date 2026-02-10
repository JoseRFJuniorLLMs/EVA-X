package ab

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
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
	db *sql.DB
}

// NewABTestService cria novo serviço de A/B testing
func NewABTestService(db *sql.DB) *ABTestService {
	return &ABTestService{db: db}
}

// AssignGroup atribui usuário a um grupo de teste
func (s *ABTestService) AssignGroup(testName string, idosoID int64) (string, error) {
	var group string

	query := `SELECT assign_ab_test_group($1, $2)`
	err := s.db.QueryRow(query, testName, idosoID).Scan(&group)

	if err != nil {
		return "", fmt.Errorf("erro ao atribuir grupo: %w", err)
	}

	log.Printf("🧪 [A/B TEST] Usuário %d atribuído ao grupo: %s (teste: %s)", idosoID, group, testName)
	return group, nil
}

// ShouldUseThinkingMode verifica se usuário deve usar Thinking Mode
func (s *ABTestService) ShouldUseThinkingMode(idosoID int64) bool {
	group, err := s.AssignGroup("health_triage_mode", idosoID)
	if err != nil {
		log.Printf("⚠️ Erro no A/B test, usando modo padrão: %v", err)
		return true // Fallback para Thinking Mode
	}

	return group == "thinking_mode"
}

// LogMetric registra métrica de teste A/B
func (s *ABTestService) LogMetric(testName string, idosoID int64, metricType string, value float64, metadata map[string]interface{}) error {
	query := `SELECT log_ab_test_metric($1, $2, $3, $4, $5)`

	var metadataJSON interface{}
	if metadata != nil {
		metadataJSON = metadata
	}

	_, err := s.db.Exec(query, testName, idosoID, metricType, value, metadataJSON)
	if err != nil {
		return fmt.Errorf("erro ao registrar métrica: %w", err)
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
	query := `
		SELECT 
			metrica,
			grupo_a,
			media_a,
			amostras_a,
			grupo_b,
			media_b,
			amostras_b,
			diferenca_percentual,
			vencedor
		FROM v_ab_test_comparison
		WHERE test_name = $1
		ORDER BY metrica
	`

	rows, err := s.db.Query(query, testName)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar resultados: %w", err)
	}
	defer rows.Close()

	var results []TestResult
	for rows.Next() {
		var r TestResult
		err := rows.Scan(
			&r.Metric,
			&r.GroupA,
			&r.MeanA,
			&r.SamplesA,
			&r.GroupB,
			&r.MeanB,
			&r.SamplesB,
			&r.DifferencePercent,
			&r.Winner,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear resultado: %w", err)
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
	query := `
		SELECT grupo, total_usuarios, percentual
		FROM v_ab_test_distribution
		WHERE test_name = $1
	`

	rows, err := s.db.Query(query, testName)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar distribuição: %w", err)
	}
	defer rows.Close()

	var distribution []GroupDistribution
	for rows.Next() {
		var d GroupDistribution
		err := rows.Scan(&d.Group, &d.TotalUsers, &d.Percentage)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear distribuição: %w", err)
		}
		distribution = append(distribution, d)
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
