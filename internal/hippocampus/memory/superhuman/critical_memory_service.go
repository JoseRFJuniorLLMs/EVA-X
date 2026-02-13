package superhuman

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// CriticalMemoryService implements the 4 high-priority systems from memoria-critica.md
// 1. Abstração Seletiva (Pattern Clustering)
// 2. Right to be Forgotten (LGPD Compliance)
// 3. Decay Temporal (Recent memories weigh more)
// 4. Filtro Ético (Ethical Auditor)
type CriticalMemoryService struct {
	db *sql.DB
}

// NewCriticalMemoryService creates the critical memory service
func NewCriticalMemoryService(db *sql.DB) *CriticalMemoryService {
	return &CriticalMemoryService{db: db}
}

// =====================================================
// 1. ABSTRAÇÃO SELETIVA (Selective Abstraction)
// "Pensar é esquecer diferenças" - Borges
// =====================================================

// MemoryCluster represents a group of similar memories
type MemoryCluster struct {
	ID                 int64     `json:"id"`
	IdosoID            int64     `json:"idoso_id"`
	ClusterName        string    `json:"cluster_name"`
	ClusterType        string    `json:"cluster_type"`
	AbstractedSummary  string    `json:"abstracted_summary"`
	MemberCount        int       `json:"member_count"`
	TotalMentions      int       `json:"total_mentions"`
	MostCommonTime     string    `json:"most_common_time_period"`
	AvgEmotionalValence float64  `json:"avg_emotional_valence"`
	DominantEmotion    string    `json:"dominant_emotion"`
	CorrelatedPersons  []string  `json:"correlated_persons"`
	CorrelatedTopics   []string  `json:"correlated_topics"`
	CoherenceScore     float64   `json:"coherence_score"`
	FirstOccurrence    time.Time `json:"first_occurrence"`
	LastOccurrence     time.Time `json:"last_occurrence"`
}

// CreateOrUpdateCluster creates or updates a memory cluster
func (s *CriticalMemoryService) CreateOrUpdateCluster(ctx context.Context, idosoID int64, clusterName, clusterType string) (*MemoryCluster, error) {
	query := `
		INSERT INTO patient_memory_clusters
		(idoso_id, cluster_name, cluster_type, abstracted_summary, first_occurrence, last_occurrence)
		VALUES ($1, $2, $3, '', NOW(), NOW())
		ON CONFLICT (idoso_id, cluster_name) DO UPDATE SET
			total_mentions = patient_memory_clusters.total_mentions + 1,
			last_occurrence = NOW(),
			updated_at = NOW()
		RETURNING id, member_count, total_mentions, coherence_score
	`

	cluster := &MemoryCluster{
		IdosoID:     idosoID,
		ClusterName: clusterName,
		ClusterType: clusterType,
	}

	err := s.db.QueryRowContext(ctx, query, idosoID, clusterName, clusterType).Scan(
		&cluster.ID, &cluster.MemberCount, &cluster.TotalMentions, &cluster.CoherenceScore,
	)
	if err != nil {
		return nil, err
	}

	// Generate abstraction
	s.db.ExecContext(ctx, "SELECT generate_cluster_abstraction($1)", cluster.ID)

	return cluster, nil
}

// AddToCluster adds a memory to an existing cluster
func (s *CriticalMemoryService) AddToCluster(ctx context.Context, clusterID, idosoID int64, memoryType string, memoryRefID int64, verbatim string) error {
	query := `
		INSERT INTO cluster_members
		(cluster_id, idoso_id, memory_type, memory_reference_id, memory_verbatim)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := s.db.ExecContext(ctx, query, clusterID, idosoID, memoryType, memoryRefID, verbatim)
	if err != nil {
		return err
	}

	// Update member count
	updateQuery := `
		UPDATE patient_memory_clusters
		SET member_count = (SELECT COUNT(*) FROM cluster_members WHERE cluster_id = $1),
		    updated_at = NOW()
		WHERE id = $1
	`
	s.db.ExecContext(ctx, updateQuery, clusterID)

	// Regenerate abstraction
	s.db.ExecContext(ctx, "SELECT generate_cluster_abstraction($1)", clusterID)

	return nil
}

// GetAbstractedPatterns returns abstracted summaries instead of raw data
func (s *CriticalMemoryService) GetAbstractedPatterns(ctx context.Context, idosoID int64) ([]*MemoryCluster, error) {
	query := `
		SELECT id, cluster_name, cluster_type, abstracted_summary,
		       member_count, total_mentions, most_common_time_period,
		       avg_emotional_valence, dominant_emotion,
		       correlated_persons, correlated_topics, coherence_score,
		       first_occurrence, last_occurrence
		FROM patient_memory_clusters
		WHERE idoso_id = $1 AND total_mentions >= 3
		ORDER BY total_mentions DESC
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []*MemoryCluster
	for rows.Next() {
		c := &MemoryCluster{IdosoID: idosoID}
		var personsJSON, topicsJSON []byte
		var mostCommonTime, dominantEmotion sql.NullString
		var avgValence, coherence sql.NullFloat64
		var firstOcc, lastOcc sql.NullTime

		err := rows.Scan(
			&c.ID, &c.ClusterName, &c.ClusterType, &c.AbstractedSummary,
			&c.MemberCount, &c.TotalMentions, &mostCommonTime,
			&avgValence, &dominantEmotion,
			&personsJSON, &topicsJSON, &coherence,
			&firstOcc, &lastOcc,
		)
		if err != nil {
			continue
		}

		if mostCommonTime.Valid {
			c.MostCommonTime = mostCommonTime.String
		}
		if dominantEmotion.Valid {
			c.DominantEmotion = dominantEmotion.String
		}
		if avgValence.Valid {
			c.AvgEmotionalValence = avgValence.Float64
		}
		if coherence.Valid {
			c.CoherenceScore = coherence.Float64
		}
		if firstOcc.Valid {
			c.FirstOccurrence = firstOcc.Time
		}
		if lastOcc.Valid {
			c.LastOccurrence = lastOcc.Time
		}

		json.Unmarshal(personsJSON, &c.CorrelatedPersons)
		json.Unmarshal(topicsJSON, &c.CorrelatedTopics)

		clusters = append(clusters, c)
	}

	return clusters, nil
}

// ClusterSimilarMemories automatically clusters similar memories
func (s *CriticalMemoryService) ClusterSimilarMemories(ctx context.Context, idosoID int64) error {
	// Cluster metaphors by type
	metaphorQuery := `
		SELECT metaphor, metaphor_type, usage_count
		FROM patient_metaphors
		WHERE idoso_id = $1 AND is_forgotten = FALSE
		ORDER BY usage_count DESC
	`
	rows, err := s.db.QueryContext(ctx, metaphorQuery, idosoID)
	if err != nil {
		return err
	}

	metaphorClusters := make(map[string][]string)
	for rows.Next() {
		var metaphor, metaphorType string
		var count int
		if err := rows.Scan(&metaphor, &metaphorType, &count); err != nil {
			continue
		}
		metaphorClusters[metaphorType] = append(metaphorClusters[metaphorType], metaphor)
	}
	rows.Close()

	// Create clusters for each metaphor type
	for mType, metaphors := range metaphorClusters {
		if len(metaphors) >= 2 {
			clusterName := fmt.Sprintf("metaforas_%s", mType)
			cluster, err := s.CreateOrUpdateCluster(ctx, idosoID, clusterName, "emotion")
			if err != nil {
				continue
			}

			// Update with aggregated info
			s.db.ExecContext(ctx, `
				UPDATE patient_memory_clusters
				SET member_count = $2,
				    dominant_emotion = $3,
				    updated_at = NOW()
				WHERE id = $1
			`, cluster.ID, len(metaphors), mType)
		}
	}

	// Cluster topics by person correlation
	personQuery := `
		SELECT person_name, mention_count
		FROM patient_world_persons
		WHERE idoso_id = $1
		ORDER BY mention_count DESC
		LIMIT 10
	`
	personRows, err := s.db.QueryContext(ctx, personQuery, idosoID)
	if err != nil {
		return err
	}

	for personRows.Next() {
		var personName string
		var mentionCount int
		if err := personRows.Scan(&personName, &mentionCount); err != nil {
			continue
		}

		if mentionCount >= 5 {
			clusterName := fmt.Sprintf("conversas_sobre_%s", strings.ToLower(personName))
			s.CreateOrUpdateCluster(ctx, idosoID, clusterName, "person")
		}
	}
	personRows.Close()

	log.Printf("🔮 [ABSTRACTION] Clustered memories for patient %d", idosoID)
	return nil
}

// =====================================================
// 2. RIGHT TO BE FORGOTTEN (LGPD Compliance)
// =====================================================

// ForgottenMemory represents a record of what was forgotten
type ForgottenMemory struct {
	ID              int64     `json:"id"`
	IdosoID         int64     `json:"idoso_id"`
	MemoryType      string    `json:"memory_type"`
	MemoryIdentifier string   `json:"memory_identifier"`
	Reason          string    `json:"reason"`
	RequestedBy     string    `json:"requested_by"`
	DeletedCount    int       `json:"deleted_count"`
	AffectedTables  []map[string]interface{} `json:"affected_tables"`
	ForgottenAt     time.Time `json:"forgotten_at"`
}

// ForgetTopic removes all memories related to a topic
func (s *CriticalMemoryService) ForgetTopic(ctx context.Context, idosoID int64, topic, reason, requestedBy string) (*ForgottenMemory, error) {
	var resultJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT forget_topic($1, $2, $3, $4)",
		idosoID, topic, reason, requestedBy).Scan(&resultJSON)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	json.Unmarshal(resultJSON, &result)

	fm := &ForgottenMemory{
		IdosoID:          idosoID,
		MemoryType:       "topic",
		MemoryIdentifier: topic,
		Reason:           reason,
		RequestedBy:      requestedBy,
		DeletedCount:     int(result["deleted_count"].(float64)),
		ForgottenAt:      time.Now(),
	}

	if affected, ok := result["affected_tables"].([]interface{}); ok {
		for _, a := range affected {
			if m, ok := a.(map[string]interface{}); ok {
				fm.AffectedTables = append(fm.AffectedTables, m)
			}
		}
	}

	log.Printf("🗑️ [FORGET] Topic '%s' forgotten for patient %d (%d items)",
		topic, idosoID, fm.DeletedCount)

	return fm, nil
}

// ForgetPerson removes all memories related to a person
func (s *CriticalMemoryService) ForgetPerson(ctx context.Context, idosoID int64, personName, reason, requestedBy string) (*ForgottenMemory, error) {
	var resultJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT forget_person($1, $2, $3, $4)",
		idosoID, personName, reason, requestedBy).Scan(&resultJSON)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	json.Unmarshal(resultJSON, &result)

	fm := &ForgottenMemory{
		IdosoID:          idosoID,
		MemoryType:       "person",
		MemoryIdentifier: personName,
		Reason:           reason,
		RequestedBy:      requestedBy,
		DeletedCount:     int(result["deleted_count"].(float64)),
		ForgottenAt:      time.Now(),
	}

	log.Printf("🗑️ [FORGET] Person '%s' forgotten for patient %d", personName, idosoID)

	return fm, nil
}

// GetForgottenItems returns list of what has been forgotten (for audit)
func (s *CriticalMemoryService) GetForgottenItems(ctx context.Context, idosoID int64) ([]*ForgottenMemory, error) {
	query := `
		SELECT id, memory_type, memory_identifier, reason, requested_by,
		       deleted_count, affected_tables, forgotten_at
		FROM forgotten_memories
		WHERE idoso_id = $1
		ORDER BY forgotten_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*ForgottenMemory
	for rows.Next() {
		fm := &ForgottenMemory{IdosoID: idosoID}
		var affectedJSON []byte
		var reason, requestedBy sql.NullString

		err := rows.Scan(
			&fm.ID, &fm.MemoryType, &fm.MemoryIdentifier, &reason, &requestedBy,
			&fm.DeletedCount, &affectedJSON, &fm.ForgottenAt,
		)
		if err != nil {
			continue
		}

		if reason.Valid {
			fm.Reason = reason.String
		}
		if requestedBy.Valid {
			fm.RequestedBy = requestedBy.String
		}

		var affected []map[string]interface{}
		json.Unmarshal(affectedJSON, &affected)
		fm.AffectedTables = affected

		items = append(items, fm)
	}

	return items, nil
}

// =====================================================
// 3. DECAY TEMPORAL (Temporal Decay)
// Recent memories weigh more
// =====================================================

// TemporalConfig represents decay configuration for a patient
type TemporalConfig struct {
	IdosoID            int64   `json:"idoso_id"`
	DefaultDecayRate   float64 `json:"default_decay_rate"`
	TraumaDecayRate    float64 `json:"trauma_decay_rate"`
	PositiveDecayRate  float64 `json:"positive_decay_rate"`
	RecencyWindowDays  int     `json:"recency_window_days"`
	RecencyBoostFactor float64 `json:"recency_boost_factor"`
}

// ApplyTemporalDecay applies time-based decay to all memories
func (s *CriticalMemoryService) ApplyTemporalDecay(ctx context.Context, idosoID int64) (map[string]interface{}, error) {
	var resultJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT apply_temporal_decay($1)", idosoID).Scan(&resultJSON)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	json.Unmarshal(resultJSON, &result)

	log.Printf("⏱️ [DECAY] Applied temporal decay for patient %d: memories=%v, patterns=%v",
		idosoID,
		result["memories_decayed"],
		result["patterns_decayed"])

	return result, nil
}

// GetTemporalConfig gets decay configuration for a patient
func (s *CriticalMemoryService) GetTemporalConfig(ctx context.Context, idosoID int64) (*TemporalConfig, error) {
	// Ensure config exists
	s.db.ExecContext(ctx, `
		INSERT INTO patient_temporal_config (idoso_id)
		VALUES ($1) ON CONFLICT (idoso_id) DO NOTHING
	`, idosoID)

	query := `
		SELECT default_decay_rate, trauma_decay_rate, positive_decay_rate,
		       recency_window_days, recency_boost_factor
		FROM patient_temporal_config
		WHERE idoso_id = $1
	`

	config := &TemporalConfig{IdosoID: idosoID}
	err := s.db.QueryRowContext(ctx, query, idosoID).Scan(
		&config.DefaultDecayRate, &config.TraumaDecayRate, &config.PositiveDecayRate,
		&config.RecencyWindowDays, &config.RecencyBoostFactor,
	)

	return config, err
}

// UpdateTemporalConfig updates decay configuration
func (s *CriticalMemoryService) UpdateTemporalConfig(ctx context.Context, config *TemporalConfig) error {
	query := `
		UPDATE patient_temporal_config
		SET default_decay_rate = $2,
		    trauma_decay_rate = $3,
		    positive_decay_rate = $4,
		    recency_window_days = $5,
		    recency_boost_factor = $6,
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	_, err := s.db.ExecContext(ctx, query,
		config.IdosoID, config.DefaultDecayRate, config.TraumaDecayRate,
		config.PositiveDecayRate, config.RecencyWindowDays, config.RecencyBoostFactor)
	return err
}

// MarkAsAnchorMemory marks a memory as never-decaying
func (s *CriticalMemoryService) MarkAsAnchorMemory(ctx context.Context, idosoID int64, memoryID int64) error {
	_, err := s.db.ExecContext(ctx, "SELECT mark_as_anchor_memory($1, $2)", idosoID, memoryID)
	if err != nil {
		return err
	}

	log.Printf("⚓ [ANCHOR] Memory %d marked as anchor for patient %d", memoryID, idosoID)
	return nil
}

// GetWeightedMemories returns memories with temporal weight applied
func (s *CriticalMemoryService) GetWeightedMemories(ctx context.Context, idosoID int64, minWeight float64) ([]*MemoryGravity, error) {
	// First apply decay
	s.ApplyTemporalDecay(ctx, idosoID)

	query := `
		SELECT memory_id, memory_type, memory_summary, gravity_score,
		       weighted_gravity, days_since_activation
		FROM v_weighted_memories
		WHERE idoso_id = $1 AND weighted_gravity >= $2
		ORDER BY weighted_gravity DESC
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID, minWeight)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*MemoryGravity
	for rows.Next() {
		m := &MemoryGravity{IdosoID: idosoID}
		var weightedGravity float64
		var daysSince sql.NullFloat64

		err := rows.Scan(
			&m.MemoryID, &m.MemoryType, &m.MemorySummary, &m.GravityScore,
			&weightedGravity, &daysSince,
		)
		if err != nil {
			continue
		}

		// Use weighted gravity as the effective gravity
		m.GravityScore = weightedGravity

		memories = append(memories, m)
	}

	return memories, nil
}

// =====================================================
// 4. FILTRO ÉTICO (Ethical Auditor)
// =====================================================

// EthicalAuditResult represents the result of an ethical audit
type EthicalAuditResult struct {
	Allowed          bool                     `json:"allowed"`
	Action           string                   `json:"action"`
	Severity         string                   `json:"severity"`
	RulesTriggered   []map[string]interface{} `json:"rules_triggered"`
	NeedsHumanReview bool                     `json:"needs_human_review"`
	OriginalResponse string                   `json:"original_response"`
	ModifiedResponse string                   `json:"modified_response,omitempty"`
	ShouldModify     bool                     `json:"should_modify"`
}

// AuditResponse checks a response for ethical violations
func (s *CriticalMemoryService) AuditResponse(ctx context.Context, idosoID int64, response, patientMode string, crisisLevel float64) (*EthicalAuditResult, error) {
	var resultJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT audit_response($1, $2, $3, $4)",
		idosoID, response, patientMode, crisisLevel).Scan(&resultJSON)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	json.Unmarshal(resultJSON, &result)

	audit := &EthicalAuditResult{
		Allowed:          result["allowed"].(bool),
		Action:           result["action"].(string),
		Severity:         result["severity"].(string),
		NeedsHumanReview: result["needs_human_review"].(bool),
		OriginalResponse: result["original_response"].(string),
		ShouldModify:     result["should_modify"].(bool),
	}

	if rules, ok := result["rules_triggered"].([]interface{}); ok {
		for _, r := range rules {
			if m, ok := r.(map[string]interface{}); ok {
				audit.RulesTriggered = append(audit.RulesTriggered, m)
			}
		}
	}

	if len(audit.RulesTriggered) > 0 {
		log.Printf("⚠️ [ETHICAL] Audit triggered %d rules for patient %d (severity: %s)",
			len(audit.RulesTriggered), idosoID, audit.Severity)
	}

	return audit, nil
}

// ModifyResponse applies ethical modifications to a response
func (s *CriticalMemoryService) ModifyResponse(ctx context.Context, response string, audit *EthicalAuditResult) string {
	if !audit.ShouldModify || len(audit.RulesTriggered) == 0 {
		return response
	}

	modifiedResponse := response

	for _, rule := range audit.RulesTriggered {
		category := rule["category"].(string)
		patternMatched := rule["pattern_matched"].(string)

		switch category {
		case "dignity":
			// Remove accusatory language
			modifiedResponse = s.removeAccusatoryLanguage(modifiedResponse, patternMatched)

		case "harm_prevention":
			// Add supportive framing
			modifiedResponse = s.addSupportiveFraming(modifiedResponse)

		case "manipulation":
			// Soften absolute statements
			modifiedResponse = s.softenAbsoluteStatements(modifiedResponse)

		case "privacy":
			// Redact third-party information
			modifiedResponse = s.redactThirdPartyInfo(modifiedResponse)
		}
	}

	return modifiedResponse
}

// Helper functions for ethical modifications
func (s *CriticalMemoryService) removeAccusatoryLanguage(text, pattern string) string {
	replacements := map[string]string{
		"você sempre":          "às vezes você",
		"você nunca consegue":  "tem sido difícil",
		"é patético":           "é um desafio",
		"é fraco":              "está passando por dificuldades",
		"você já fez isso antes": "isso é um padrão que podemos explorar juntos",
		"de novo?":             "percebo que isso acontece",
		"quantas vezes":        "isso tem acontecido",
	}

	result := text
	for old, new := range replacements {
		result = strings.ReplaceAll(strings.ToLower(result), old, new)
	}
	return result
}

func (s *CriticalMemoryService) addSupportiveFraming(text string) string {
	// Add supportive prefix if dealing with hopelessness
	if strings.Contains(strings.ToLower(text), "não há esperança") ||
		strings.Contains(strings.ToLower(text), "nunca vai melhorar") {
		return "Entendo que você está passando por um momento muito difícil. " + text
	}
	return text
}

func (s *CriticalMemoryService) softenAbsoluteStatements(text string) string {
	replacements := map[string]string{
		"certamente vai":  "é possível que",
		"com certeza":     "provavelmente",
		"garanto que":     "acredito que",
		"prometo que":     "farei o possível para",
		"só eu entendo":   "eu procuro entender",
		"ninguém mais":    "pode ser difícil para outros",
		"você precisa de mim": "estou aqui para ajudar",
	}

	result := text
	for old, new := range replacements {
		result = strings.ReplaceAll(strings.ToLower(result), old, new)
	}
	return result
}

func (s *CriticalMemoryService) redactThirdPartyInfo(text string) string {
	// Simple redaction pattern - in production would be more sophisticated
	patterns := []string{
		`me contou que ([^.]+)\.`,
		`segredo de ([^.]+)\.`,
	}

	result := text
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "[informação privada].")
	}
	return result
}

// GetPendingHumanReviews returns audit logs that need human review
func (s *CriticalMemoryService) GetPendingHumanReviews(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT eal.id, eal.idoso_id, eal.original_response, eal.rules_triggered,
		       eal.highest_severity, eal.patient_mode, eal.crisis_level, eal.audited_at,
		       i.nome as patient_name
		FROM ethical_audit_log eal
		JOIN idosos i ON eal.idoso_id = i.id
		WHERE eal.needs_human_review = TRUE AND eal.human_reviewed = FALSE
		ORDER BY
			CASE eal.highest_severity
				WHEN 'emergency' THEN 1
				WHEN 'block' THEN 2
				ELSE 3
			END,
			eal.audited_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []map[string]interface{}
	for rows.Next() {
		var id, idosoID int64
		var response string
		var rulesJSON []byte
		var severity, mode, patientName string
		var crisisLevel float64
		var auditedAt time.Time

		err := rows.Scan(&id, &idosoID, &response, &rulesJSON, &severity, &mode, &crisisLevel, &auditedAt, &patientName)
		if err != nil {
			continue
		}

		var rules []interface{}
		json.Unmarshal(rulesJSON, &rules)

		reviews = append(reviews, map[string]interface{}{
			"id":            id,
			"idoso_id":      idosoID,
			"patient_name":  patientName,
			"response":      response,
			"rules":         rules,
			"severity":      severity,
			"mode":          mode,
			"crisis_level":  crisisLevel,
			"audited_at":    auditedAt,
		})
	}

	return reviews, nil
}

// SubmitHumanReview records a human review decision
func (s *CriticalMemoryService) SubmitHumanReview(ctx context.Context, auditLogID int64, reviewer, notes string) error {
	query := `
		UPDATE ethical_audit_log
		SET human_reviewed = TRUE,
		    human_reviewer = $2,
		    human_review_notes = $3,
		    human_reviewed_at = NOW()
		WHERE id = $1
	`
	_, err := s.db.ExecContext(ctx, query, auditLogID, reviewer, notes)
	return err
}

// =====================================================
// MIRROR OUTPUTS
// =====================================================

// GenerateCriticalMirrors generates mirror outputs from critical memory systems
func (s *CriticalMemoryService) GenerateCriticalMirrors(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	var outputs []*MirrorOutput

	// 1. Abstracted patterns (instead of raw data)
	clusters, _ := s.GetAbstractedPatterns(ctx, idosoID)
	for _, c := range clusters[:minInt(3, len(clusters))] {
		if c.TotalMentions >= 5 {
			dataPoints := []string{c.AbstractedSummary}

			if len(c.CorrelatedPersons) > 0 {
				dataPoints = append(dataPoints,
					fmt.Sprintf("Pessoas relacionadas: %s", strings.Join(c.CorrelatedPersons, ", ")))
			}

			if c.DominantEmotion != "" {
				dataPoints = append(dataPoints,
					fmt.Sprintf("Tom emocional predominante: %s", c.DominantEmotion))
			}

			outputs = append(outputs, &MirrorOutput{
				Type:       "abstracted_pattern",
				DataPoints: dataPoints,
				Frequency:  &c.TotalMentions,
				Question:   "O que você percebe sobre esse padrão?",
				RawData: map[string]interface{}{
					"cluster_type":    c.ClusterType,
					"member_count":    c.MemberCount,
					"coherence":       c.CoherenceScore,
					"first_occurrence": c.FirstOccurrence,
				},
			})
		}
	}

	// 2. Weighted memories (with recency applied)
	weightedMemories, _ := s.GetWeightedMemories(ctx, idosoID, 0.6)
	if len(weightedMemories) > 0 {
		recentCount := 0
		for _, m := range weightedMemories {
			if m.GravityScore > 0.7 {
				recentCount++
			}
		}

		if recentCount > 0 {
			outputs = append(outputs, &MirrorOutput{
				Type: "temporal_weight",
				DataPoints: []string{
					fmt.Sprintf("Você tem %d memórias pesadas ativas recentemente", recentCount),
					"Memórias recentes têm mais peso nas suas conversas atuais",
				},
				Question: "Como você sente que o passado recente está afetando seu presente?",
				RawData: map[string]interface{}{
					"weighted_count": len(weightedMemories),
					"recent_heavy":   recentCount,
				},
			})
		}
	}

	return outputs, nil
}
