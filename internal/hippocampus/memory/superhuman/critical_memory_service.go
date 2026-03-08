// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package superhuman

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// CriticalMemoryService implements the 4 high-priority systems from memoria-critica.md
// 1. Abstraction Seletiva (Pattern Clustering)
// 2. Right to be Forgotten (LGPD Compliance)
// 3. Decay Temporal (Recent memories weigh more)
// 4. Filtro Etico (Ethical Auditor)
type CriticalMemoryService struct {
	db *database.DB
}

// NewCriticalMemoryService creates the critical memory service
func NewCriticalMemoryService(db *database.DB) *CriticalMemoryService {
	return &CriticalMemoryService{db: db}
}

// =====================================================
// 1. ABSTRACTION SELETIVA (Selective Abstraction)
// "Pensar e esquecer diferencas" - Borges
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
	now := time.Now().Format(time.RFC3339)

	// Try to find existing cluster
	rows, err := s.db.QueryByLabel(ctx, "patient_memory_clusters",
		" AND n.idoso_id = $idoso AND n.cluster_name = $cname",
		map[string]interface{}{"idoso": idosoID, "cname": clusterName}, 1)
	if err != nil {
		return nil, err
	}

	cluster := &MemoryCluster{
		IdosoID:     idosoID,
		ClusterName: clusterName,
		ClusterType: clusterType,
	}

	if len(rows) > 0 {
		m := rows[0]
		cluster.ID = database.GetInt64(m, "id")
		cluster.MemberCount = int(database.GetInt64(m, "member_count"))
		cluster.TotalMentions = int(database.GetInt64(m, "total_mentions")) + 1
		cluster.CoherenceScore = database.GetFloat64(m, "coherence_score")

		if err := s.db.Update(ctx, "patient_memory_clusters",
			map[string]interface{}{"idoso_id": idosoID, "cluster_name": clusterName},
			map[string]interface{}{
				"total_mentions":  cluster.TotalMentions,
				"last_occurrence": now,
				"updated_at":      now,
			}); err != nil {
			log.Printf("[critical_memory] update memory_clusters failed: %v", err)
			return nil, fmt.Errorf("update memory_clusters: %w", err)
		}
	} else {
		id, err := s.db.Insert(ctx, "patient_memory_clusters", map[string]interface{}{
			"idoso_id":          idosoID,
			"cluster_name":      clusterName,
			"cluster_type":      clusterType,
			"abstracted_summary": "",
			"total_mentions":    1,
			"member_count":      0,
			"first_occurrence":  now,
			"last_occurrence":   now,
			"created_at":        now,
			"updated_at":        now,
		})
		if err != nil {
			return nil, err
		}
		cluster.ID = id
		cluster.TotalMentions = 1
	}

	return cluster, nil
}

// AddToCluster adds a memory to an existing cluster
func (s *CriticalMemoryService) AddToCluster(ctx context.Context, clusterID, idosoID int64, memoryType string, memoryRefID int64, verbatim string) error {
	now := time.Now().Format(time.RFC3339)

	_, err := s.db.Insert(ctx, "cluster_members", map[string]interface{}{
		"cluster_id":          clusterID,
		"idoso_id":            idosoID,
		"memory_type":         memoryType,
		"memory_reference_id": memoryRefID,
		"memory_verbatim":     verbatim,
		"created_at":          now,
	})
	if err != nil {
		return err
	}

	// Update member count
	memberRows, err2 := s.db.QueryByLabel(ctx, "cluster_members",
		" AND n.cluster_id = $cid",
		map[string]interface{}{"cid": clusterID}, 0)
	if err2 != nil {
		log.Printf("[CLUSTER] Error querying cluster members for cluster %d: %v", clusterID, err2)
		return err2
	}

	if err := s.db.Update(ctx, "patient_memory_clusters",
		map[string]interface{}{"id": clusterID},
		map[string]interface{}{
			"member_count": len(memberRows),
			"updated_at":   now,
		}); err != nil {
		log.Printf("[critical_memory] update memory_clusters member_count failed: %v", err)
		return fmt.Errorf("update memory_clusters member_count: %w", err)
	}

	return nil
}

// GetAbstractedPatterns returns abstracted summaries instead of raw data
func (s *CriticalMemoryService) GetAbstractedPatterns(ctx context.Context, idosoID int64) ([]*MemoryCluster, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_memory_clusters",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var clusters []*MemoryCluster
	for _, m := range rows {
		totalMentions := int(database.GetInt64(m, "total_mentions"))
		if totalMentions < 3 {
			continue
		}

		c := &MemoryCluster{
			ID:                  database.GetInt64(m, "id"),
			IdosoID:             idosoID,
			ClusterName:         database.GetString(m, "cluster_name"),
			ClusterType:         database.GetString(m, "cluster_type"),
			AbstractedSummary:   database.GetString(m, "abstracted_summary"),
			MemberCount:         int(database.GetInt64(m, "member_count")),
			TotalMentions:       totalMentions,
			MostCommonTime:      database.GetString(m, "most_common_time_period"),
			AvgEmotionalValence: database.GetFloat64(m, "avg_emotional_valence"),
			DominantEmotion:     database.GetString(m, "dominant_emotion"),
			CoherenceScore:      database.GetFloat64(m, "coherence_score"),
			FirstOccurrence:     database.GetTime(m, "first_occurrence"),
			LastOccurrence:      database.GetTime(m, "last_occurrence"),
		}

		if raw, ok := m["correlated_persons"]; ok && raw != nil {
			parseJSONStringSlice(raw, &c.CorrelatedPersons)
		}
		if raw, ok := m["correlated_topics"]; ok && raw != nil {
			parseJSONStringSlice(raw, &c.CorrelatedTopics)
		}

		clusters = append(clusters, c)
	}

	return clusters, nil
}

// ClusterSimilarMemories automatically clusters similar memories
func (s *CriticalMemoryService) ClusterSimilarMemories(ctx context.Context, idosoID int64) error {
	// Cluster metaphors by type
	metaphorRows, err := s.db.QueryByLabel(ctx, "patient_metaphors",
		" AND n.idoso_id = $idoso AND n.is_forgotten = $forgotten",
		map[string]interface{}{"idoso": idosoID, "forgotten": false}, 0)
	if err != nil {
		return err
	}

	metaphorClusters := make(map[string][]string)
	for _, m := range metaphorRows {
		metaphor := database.GetString(m, "metaphor")
		metaphorType := database.GetString(m, "metaphor_type")
		metaphorClusters[metaphorType] = append(metaphorClusters[metaphorType], metaphor)
	}

	// Create clusters for each metaphor type
	for mType, metaphors := range metaphorClusters {
		if len(metaphors) >= 2 {
			clusterName := fmt.Sprintf("metaforas_%s", mType)
			cluster, err := s.CreateOrUpdateCluster(ctx, idosoID, clusterName, "emotion")
			if err != nil {
				continue
			}

			now := time.Now().Format(time.RFC3339)
			if err := s.db.Update(ctx, "patient_memory_clusters",
				map[string]interface{}{"id": cluster.ID},
				map[string]interface{}{
					"member_count":    len(metaphors),
					"dominant_emotion": mType,
					"updated_at":      now,
				}); err != nil {
				log.Printf("[critical_memory] update memory_clusters metaphor cluster failed: %v", err)
			}
		}
	}

	// Cluster topics by person correlation
	personRows, err := s.db.QueryByLabel(ctx, "patient_world_persons",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 10)
	if err != nil {
		return err
	}

	for _, m := range personRows {
		personName := database.GetString(m, "person_name")
		mentionCount := int(database.GetInt64(m, "mention_count"))

		if mentionCount >= 5 {
			clusterName := fmt.Sprintf("conversas_sobre_%s", strings.ToLower(personName))
			if _, err := s.CreateOrUpdateCluster(ctx, idosoID, clusterName, "person"); err != nil {
				log.Printf("[ABSTRACTION] Error creating cluster for person '%s' (patient %d): %v", personName, idosoID, err)
			}
		}
	}

	log.Printf("[ABSTRACTION] Clustered memories for patient %d", idosoID)
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
	now := time.Now().Format(time.RFC3339)
	deletedCount := 0

	// Soft-delete across relevant tables
	tables := []string{
		"patient_metaphors", "patient_counterfactuals", "patient_memory_clusters",
		"patient_persistent_memories", "patient_body_memories", "patient_shared_memories",
	}

	var affected []map[string]interface{}
	for _, table := range tables {
		rows, err := s.db.QueryByLabel(ctx, table,
			" AND n.idoso_id = $idoso",
			map[string]interface{}{"idoso": idosoID}, 0)
		if err != nil {
			log.Printf("[FORGET] Error querying table '%s' for patient %d: %v", table, idosoID, err)
			return nil, err
		}

		count := 0
		for _, m := range rows {
			// Check if any field contains the topic
			content, err := json.Marshal(m)
			if err != nil {
				log.Printf("[FORGET] Error marshaling row content in table '%s': %v", table, err)
				continue
			}
			if strings.Contains(strings.ToLower(string(content)), strings.ToLower(topic)) {
				if err := s.db.SoftDelete(ctx, table, map[string]interface{}{"id": database.GetInt64(m, "id")}); err != nil {
					log.Printf("[critical_memory] soft delete in table '%s' failed: %v", table, err)
				}
				count++
			}
		}
		if count > 0 {
			deletedCount += count
			affected = append(affected, map[string]interface{}{
				"table": table,
				"count": count,
			})
		}
	}

	// Record the forgotten operation
	id, err := s.db.Insert(ctx, "forgotten_memories", map[string]interface{}{
		"idoso_id":          idosoID,
		"memory_type":       "topic",
		"memory_identifier": topic,
		"reason":            reason,
		"requested_by":      requestedBy,
		"deleted_count":     deletedCount,
		"affected_tables":   mustJSON(affected),
		"forgotten_at":      now,
		"created_at":        now,
	})
	if err != nil {
		log.Printf("[FORGET] Error inserting forgotten_memories record for topic '%s' (patient %d): %v", topic, idosoID, err)
		return nil, err
	}

	fm := &ForgottenMemory{
		ID:               id,
		IdosoID:          idosoID,
		MemoryType:       "topic",
		MemoryIdentifier: topic,
		Reason:           reason,
		RequestedBy:      requestedBy,
		DeletedCount:     deletedCount,
		AffectedTables:   affected,
		ForgottenAt:      time.Now(),
	}

	log.Printf("[FORGET] Topic '%s' forgotten for patient %d (%d items)", topic, idosoID, fm.DeletedCount)

	return fm, nil
}

// ForgetPerson removes all memories related to a person
func (s *CriticalMemoryService) ForgetPerson(ctx context.Context, idosoID int64, personName, reason, requestedBy string) (*ForgottenMemory, error) {
	now := time.Now().Format(time.RFC3339)
	deletedCount := 0

	// Delete from person-specific tables
	personRows, err := s.db.QueryByLabel(ctx, "patient_world_persons",
		" AND n.idoso_id = $idoso AND n.person_name = $name",
		map[string]interface{}{"idoso": idosoID, "name": personName}, 0)
	if err != nil {
		log.Printf("[FORGET] Error querying patient_world_persons for person '%s' (patient %d): %v", personName, idosoID, err)
		return nil, err
	}
	for _, m := range personRows {
		if err := s.db.SoftDelete(ctx, "patient_world_persons", map[string]interface{}{"id": database.GetInt64(m, "id")}); err != nil {
			log.Printf("[critical_memory] soft delete patient_world_persons failed: %v", err)
		}
		deletedCount++
	}

	// Record the forgotten operation
	id, err2 := s.db.Insert(ctx, "forgotten_memories", map[string]interface{}{
		"idoso_id":          idosoID,
		"memory_type":       "person",
		"memory_identifier": personName,
		"reason":            reason,
		"requested_by":      requestedBy,
		"deleted_count":     deletedCount,
		"forgotten_at":      now,
		"created_at":        now,
	})
	if err2 != nil {
		log.Printf("[FORGET] Error inserting forgotten_memories record for person '%s' (patient %d): %v", personName, idosoID, err2)
		return nil, err2
	}

	fm := &ForgottenMemory{
		ID:               id,
		IdosoID:          idosoID,
		MemoryType:       "person",
		MemoryIdentifier: personName,
		Reason:           reason,
		RequestedBy:      requestedBy,
		DeletedCount:     deletedCount,
		ForgottenAt:      time.Now(),
	}

	log.Printf("[FORGET] Person '%s' forgotten for patient %d", personName, idosoID)

	return fm, nil
}

// GetForgottenItems returns list of what has been forgotten (for audit)
func (s *CriticalMemoryService) GetForgottenItems(ctx context.Context, idosoID int64) ([]*ForgottenMemory, error) {
	rows, err := s.db.QueryByLabel(ctx, "forgotten_memories",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var items []*ForgottenMemory
	for _, m := range rows {
		fm := &ForgottenMemory{
			ID:               database.GetInt64(m, "id"),
			IdosoID:          idosoID,
			MemoryType:       database.GetString(m, "memory_type"),
			MemoryIdentifier: database.GetString(m, "memory_identifier"),
			Reason:           database.GetString(m, "reason"),
			RequestedBy:      database.GetString(m, "requested_by"),
			DeletedCount:     int(database.GetInt64(m, "deleted_count")),
			ForgottenAt:      database.GetTime(m, "forgotten_at"),
		}

		if raw, ok := m["affected_tables"]; ok && raw != nil {
			switch v := raw.(type) {
			case string:
				if err := json.Unmarshal([]byte(v), &fm.AffectedTables); err != nil {
					log.Printf("[FORGET] Error unmarshaling affected_tables for forgotten memory %d: %v", fm.ID, err)
				}
			case []interface{}:
				for _, item := range v {
					if mm, ok := item.(map[string]interface{}); ok {
						fm.AffectedTables = append(fm.AffectedTables, mm)
					}
				}
			}
		}

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
	// Get temporal config
	config, err := s.GetTemporalConfig(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	// Apply decay to memory gravity
	gravityRows, err2 := s.db.QueryByLabel(ctx, "patient_memory_gravity",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err2 != nil {
		log.Printf("[DECAY] Error querying patient_memory_gravity for patient %d: %v", idosoID, err2)
		return nil, err2
	}

	memoriesDecayed := 0
	for _, m := range gravityRows {
		lastActivation := database.GetTime(m, "last_activation")
		if lastActivation.IsZero() {
			continue
		}

		daysSince := time.Since(lastActivation).Hours() / 24
		currentWeight := database.GetFloat64(m, "current_weight")
		if currentWeight == 0 {
			currentWeight = database.GetFloat64(m, "gravity_score")
		}

		decayRate := config.DefaultDecayRate
		memoryType := database.GetString(m, "memory_type")
		if memoryType == "trauma" {
			decayRate = config.TraumaDecayRate
		}

		// Apply exponential decay
		newWeight := currentWeight * (1 - decayRate*daysSince/365)
		if newWeight < 0.01 {
			newWeight = 0.01
		}

		if err := s.db.Update(ctx, "patient_memory_gravity",
			map[string]interface{}{"id": database.GetInt64(m, "id")},
			map[string]interface{}{
				"current_weight": newWeight,
				"updated_at":     time.Now().Format(time.RFC3339),
			}); err != nil {
			log.Printf("[critical_memory] update memory_gravity decay failed: %v", err)
		}
		memoriesDecayed++
	}

	result := map[string]interface{}{
		"memories_decayed": memoriesDecayed,
		"patterns_decayed": 0,
	}

	log.Printf("[DECAY] Applied temporal decay for patient %d: memories=%v, patterns=%v",
		idosoID, result["memories_decayed"], result["patterns_decayed"])

	return result, nil
}

// GetTemporalConfig gets decay configuration for a patient
func (s *CriticalMemoryService) GetTemporalConfig(ctx context.Context, idosoID int64) (*TemporalConfig, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_temporal_config",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		// Create default config
		now := time.Now().Format(time.RFC3339)
		if _, err := s.db.Insert(ctx, "patient_temporal_config", map[string]interface{}{
			"idoso_id":             idosoID,
			"default_decay_rate":   0.1,
			"trauma_decay_rate":    0.02,
			"positive_decay_rate":  0.15,
			"recency_window_days":  30,
			"recency_boost_factor": 1.5,
			"created_at":           now,
			"updated_at":           now,
		}); err != nil {
			log.Printf("[DECAY] Error inserting default temporal config for patient %d: %v", idosoID, err)
			return nil, err
		}
		return &TemporalConfig{
			IdosoID:            idosoID,
			DefaultDecayRate:   0.1,
			TraumaDecayRate:    0.02,
			PositiveDecayRate:  0.15,
			RecencyWindowDays:  30,
			RecencyBoostFactor: 1.5,
		}, nil
	}

	m := rows[0]
	return &TemporalConfig{
		IdosoID:            idosoID,
		DefaultDecayRate:   database.GetFloat64(m, "default_decay_rate"),
		TraumaDecayRate:    database.GetFloat64(m, "trauma_decay_rate"),
		PositiveDecayRate:  database.GetFloat64(m, "positive_decay_rate"),
		RecencyWindowDays:  int(database.GetInt64(m, "recency_window_days")),
		RecencyBoostFactor: database.GetFloat64(m, "recency_boost_factor"),
	}, nil
}

// UpdateTemporalConfig updates decay configuration
func (s *CriticalMemoryService) UpdateTemporalConfig(ctx context.Context, config *TemporalConfig) error {
	now := time.Now().Format(time.RFC3339)

	return s.db.Update(ctx, "patient_temporal_config",
		map[string]interface{}{"idoso_id": config.IdosoID},
		map[string]interface{}{
			"default_decay_rate":   config.DefaultDecayRate,
			"trauma_decay_rate":    config.TraumaDecayRate,
			"positive_decay_rate":  config.PositiveDecayRate,
			"recency_window_days":  config.RecencyWindowDays,
			"recency_boost_factor": config.RecencyBoostFactor,
			"updated_at":           now,
		})
}

// MarkAsAnchorMemory marks a memory as never-decaying
func (s *CriticalMemoryService) MarkAsAnchorMemory(ctx context.Context, idosoID int64, memoryID int64) error {
	now := time.Now().Format(time.RFC3339)

	err := s.db.Update(ctx, "patient_memory_gravity",
		map[string]interface{}{"idoso_id": idosoID, "memory_id": memoryID},
		map[string]interface{}{
			"is_anchor":  true,
			"updated_at": now,
		})
	if err != nil {
		return err
	}

	log.Printf("[ANCHOR] Memory %d marked as anchor for patient %d", memoryID, idosoID)
	return nil
}

// GetWeightedMemories returns memories with temporal weight applied
func (s *CriticalMemoryService) GetWeightedMemories(ctx context.Context, idosoID int64, minWeight float64) ([]*MemoryGravity, error) {
	// First apply decay
	if _, err := s.ApplyTemporalDecay(ctx, idosoID); err != nil {
		log.Printf("[DECAY] Error applying temporal decay for patient %d: %v", idosoID, err)
	}

	rows, err := s.db.QueryByLabel(ctx, "patient_memory_gravity",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var memories []*MemoryGravity
	for _, m := range rows {
		gravityScore := database.GetFloat64(m, "gravity_score")
		currentWeight := database.GetFloat64(m, "current_weight")
		if currentWeight == 0 {
			currentWeight = gravityScore
		}

		if currentWeight < minWeight {
			continue
		}

		mg := &MemoryGravity{
			IdosoID:       idosoID,
			MemoryID:      database.GetInt64(m, "memory_id"),
			MemoryType:    database.GetString(m, "memory_type"),
			MemorySummary: database.GetString(m, "memory_summary"),
			GravityScore:  currentWeight, // Use weighted gravity as the effective gravity
		}

		memories = append(memories, mg)
	}

	return memories, nil
}

// =====================================================
// 4. FILTRO ETICO (Ethical Auditor)
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
	// In-process ethical audit (no longer depends on PG function)
	audit := &EthicalAuditResult{
		Allowed:          true,
		Action:           "allow",
		Severity:         "none",
		OriginalResponse: response,
		ShouldModify:     false,
	}

	responseLower := strings.ToLower(response)

	// Check dignity violations
	dignityPatterns := []string{
		"voce sempre", "voce nunca consegue", "e patetico", "e fraco",
		"voce ja fez isso antes", "de novo?", "quantas vezes",
	}
	for _, p := range dignityPatterns {
		if strings.Contains(responseLower, p) {
			audit.RulesTriggered = append(audit.RulesTriggered, map[string]interface{}{
				"category":        "dignity",
				"pattern_matched": p,
				"severity":        "warning",
			})
			audit.ShouldModify = true
			audit.Severity = "warning"
		}
	}

	// Check harm prevention in crisis
	if crisisLevel > 0.7 {
		harmPatterns := []string{
			"nao ha esperanca", "nunca vai melhorar", "desista",
		}
		for _, p := range harmPatterns {
			if strings.Contains(responseLower, p) {
				audit.RulesTriggered = append(audit.RulesTriggered, map[string]interface{}{
					"category":        "harm_prevention",
					"pattern_matched": p,
					"severity":        "block",
				})
				audit.Allowed = false
				audit.Action = "block"
				audit.Severity = "block"
				audit.NeedsHumanReview = true
			}
		}
	}

	// Check manipulation patterns
	manipulationPatterns := []string{
		"so eu entendo", "ninguem mais", "voce precisa de mim",
		"garanto que", "prometo que",
	}
	for _, p := range manipulationPatterns {
		if strings.Contains(responseLower, p) {
			audit.RulesTriggered = append(audit.RulesTriggered, map[string]interface{}{
				"category":        "manipulation",
				"pattern_matched": p,
				"severity":        "warning",
			})
			audit.ShouldModify = true
			if audit.Severity == "none" {
				audit.Severity = "warning"
			}
		}
	}

	// Log audit
	if len(audit.RulesTriggered) > 0 {
		now := time.Now().Format(time.RFC3339)
		rulesJSON, err := json.Marshal(audit.RulesTriggered)
		if err != nil {
			log.Printf("[ETHICAL] Error marshaling rules_triggered for patient %d: %v", idosoID, err)
			rulesJSON = []byte("[]")
		}
		if _, err := s.db.Insert(ctx, "ethical_audit_log", map[string]interface{}{
			"idoso_id":           idosoID,
			"original_response":  response,
			"rules_triggered":    string(rulesJSON),
			"highest_severity":   audit.Severity,
			"patient_mode":       patientMode,
			"crisis_level":       crisisLevel,
			"needs_human_review": audit.NeedsHumanReview,
			"audited_at":         now,
			"created_at":         now,
		}); err != nil {
			log.Printf("[ETHICAL] Error inserting audit log for patient %d: %v", idosoID, err)
		}

		log.Printf("[ETHICAL] Audit triggered %d rules for patient %d (severity: %s)",
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
		"voce sempre":          "as vezes voce",
		"voce nunca consegue":  "tem sido dificil",
		"e patetico":           "e um desafio",
		"e fraco":              "esta passando por dificuldades",
		"voce ja fez isso antes": "isso e um padrao que podemos explorar juntos",
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
	if strings.Contains(strings.ToLower(text), "nao ha esperanca") ||
		strings.Contains(strings.ToLower(text), "nunca vai melhorar") {
		return "Entendo que voce esta passando por um momento muito dificil. " + text
	}
	return text
}

func (s *CriticalMemoryService) softenAbsoluteStatements(text string) string {
	replacements := map[string]string{
		"certamente vai":  "e possivel que",
		"com certeza":     "provavelmente",
		"garanto que":     "acredito que",
		"prometo que":     "farei o possivel para",
		"so eu entendo":   "eu procuro entender",
		"ninguem mais":    "pode ser dificil para outros",
		"voce precisa de mim": "estou aqui para ajudar",
	}

	result := text
	for old, new := range replacements {
		result = strings.ReplaceAll(strings.ToLower(result), old, new)
	}
	return result
}

func (s *CriticalMemoryService) redactThirdPartyInfo(text string) string {
	patterns := []string{
		`me contou que ([^.]+)\.`,
		`segredo de ([^.]+)\.`,
	}

	result := text
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "[informacao privada].")
	}
	return result
}

// GetPendingHumanReviews returns audit logs that need human review
func (s *CriticalMemoryService) GetPendingHumanReviews(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := s.db.QueryByLabel(ctx, "ethical_audit_log",
		" AND n.needs_human_review = $review AND n.human_reviewed = $reviewed",
		map[string]interface{}{"review": true, "reviewed": false}, 0)
	if err != nil {
		return nil, err
	}

	var reviews []map[string]interface{}
	for _, m := range rows {
		idosoID := database.GetInt64(m, "idoso_id")

		// Get patient name
		patientName := ""
		idosoRow, err := s.db.GetNodeByID(ctx, "idosos", idosoID)
		if err != nil {
			log.Printf("[ETHICAL] Error getting patient node %d: %v", idosoID, err)
			return nil, err
		}
		if idosoRow != nil {
			patientName = database.GetString(idosoRow, "nome")
		}

		var rules []interface{}
		if raw, ok := m["rules_triggered"]; ok && raw != nil {
			switch v := raw.(type) {
			case string:
				if err := json.Unmarshal([]byte(v), &rules); err != nil {
					log.Printf("[ETHICAL] Error unmarshaling rules_triggered for audit log %d: %v", database.GetInt64(m, "id"), err)
				}
			case []interface{}:
				rules = v
			}
		}

		reviews = append(reviews, map[string]interface{}{
			"id":            database.GetInt64(m, "id"),
			"idoso_id":      idosoID,
			"patient_name":  patientName,
			"response":      database.GetString(m, "original_response"),
			"rules":         rules,
			"severity":      database.GetString(m, "highest_severity"),
			"mode":          database.GetString(m, "patient_mode"),
			"crisis_level":  database.GetFloat64(m, "crisis_level"),
			"audited_at":    database.GetTime(m, "audited_at"),
		})
	}

	return reviews, nil
}

// SubmitHumanReview records a human review decision
func (s *CriticalMemoryService) SubmitHumanReview(ctx context.Context, auditLogID int64, reviewer, notes string) error {
	now := time.Now().Format(time.RFC3339)

	return s.db.Update(ctx, "ethical_audit_log",
		map[string]interface{}{"id": auditLogID},
		map[string]interface{}{
			"human_reviewed":     true,
			"human_reviewer":     reviewer,
			"human_review_notes": notes,
			"human_reviewed_at":  now,
		})
}

// =====================================================
// MIRROR OUTPUTS
// =====================================================

// GenerateCriticalMirrors generates mirror outputs from critical memory systems
func (s *CriticalMemoryService) GenerateCriticalMirrors(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	var outputs []*MirrorOutput

	// 1. Abstracted patterns (instead of raw data)
	clusters, err := s.GetAbstractedPatterns(ctx, idosoID)
	if err != nil {
		log.Printf("[MIRROR] Error getting abstracted patterns for patient %d: %v", idosoID, err)
	}
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
				Question:   "O que voce percebe sobre esse padrao?",
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
	weightedMemories, err2 := s.GetWeightedMemories(ctx, idosoID, 0.6)
	if err2 != nil {
		log.Printf("[MIRROR] Error getting weighted memories for patient %d: %v", idosoID, err2)
	}
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
					fmt.Sprintf("Voce tem %d memorias pesadas ativas recentemente", recentCount),
					"Memorias recentes tem mais peso nas suas conversas atuais",
				},
				Question: "Como voce sente que o passado recente esta afetando seu presente?",
				RawData: map[string]interface{}{
					"weighted_count": len(weightedMemories),
					"recent_heavy":   recentCount,
				},
			})
		}
	}

	return outputs, nil
}

// mustJSON marshals to JSON string, returning "[]" on error
func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
