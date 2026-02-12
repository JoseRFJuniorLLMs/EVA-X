package importance

import (
	"context"
	"database/sql"
	"math"
	"time"

	"github.com/rs/zerolog/log"
)

// Scorer calculates importance scores for memories
type Scorer struct {
	db *sql.DB
}

// NewScorer creates a new importance scorer
func NewScorer(db *sql.DB) *Scorer {
	return &Scorer{db: db}
}

// ImportanceFactors represents the components of importance
type ImportanceFactors struct {
	Frequency       float64 // How often accessed (0-1)
	Recency         float64 // How recent (0-1)
	GraphCentrality float64 // Neo4j connections (0-1)
	EmotionalIntensity float64 // Emotional weight (0-1)
	GoalRelevance   float64 // Relevance to patient goals (0-1)
}

// MemoryImportance represents a memory with its importance score
type MemoryImportance struct {
	MemoryID   int64
	Score      float64
	Factors    ImportanceFactors
	CalculatedAt time.Time
}

// CalculateImportance computes the importance score for a memory
func (s *Scorer) CalculateImportance(ctx context.Context, memoryID int64) (*MemoryImportance, error) {
	factors := ImportanceFactors{}
	
	// 1. Frequency: How often this memory was accessed
	var accessCount int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM memory_access_log 
		WHERE memory_id = $1 
		AND accessed_at > NOW() - INTERVAL '30 days'
	`, memoryID).Scan(&accessCount)
	
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	
	// Normalize: 0 accesses = 0, 10+ accesses = 1.0
	factors.Frequency = math.Min(float64(accessCount)/10.0, 1.0)
	
	// 2. Recency: How recent is this memory
	var createdAt time.Time
	err = s.db.QueryRowContext(ctx, `
		SELECT created_at FROM memories WHERE id = $1
	`, memoryID).Scan(&createdAt)
	
	if err != nil {
		return nil, err
	}
	
	daysSinceCreation := time.Since(createdAt).Hours() / 24.0
	// Exponential decay: recent = 1.0, 30 days ago = 0.5, 90 days = 0.1
	factors.Recency = math.Exp(-daysSinceCreation / 30.0)
	
	// 3. Graph Centrality: How connected in Neo4j
	// TODO: Query Neo4j for degree centrality
	// For now, use placeholder
	factors.GraphCentrality = 0.5
	
	// 4. Emotional Intensity: Extract from metadata
	var emotionalIntensity float64
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(metadata->>'emotional_intensity', '0')::float
		FROM memories WHERE id = $1
	`, memoryID).Scan(&emotionalIntensity)
	
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	
	factors.EmotionalIntensity = math.Min(emotionalIntensity, 1.0)
	
	// 5. Goal Relevance: How relevant to patient's current goals
	// TODO: Implement goal matching
	factors.GoalRelevance = 0.5
	
	// Weighted combination
	// Frequency: 20%, Recency: 25%, Centrality: 20%, Emotion: 20%, Goals: 15%
	totalScore := (
		factors.Frequency * 0.20 +
		factors.Recency * 0.25 +
		factors.GraphCentrality * 0.20 +
		factors.EmotionalIntensity * 0.20 +
		factors.GoalRelevance * 0.15,
	)
	
	return &MemoryImportance{
		MemoryID:     memoryID,
		Score:        totalScore,
		Factors:      factors,
		CalculatedAt: time.Now(),
	}, nil
}

// BatchCalculateImportance calculates importance for multiple memories
func (s *Scorer) BatchCalculateImportance(ctx context.Context, memoryIDs []int64) ([]*MemoryImportance, error) {
	results := make([]*MemoryImportance, 0, len(memoryIDs))
	
	for _, id := range memoryIDs {
		importance, err := s.CalculateImportance(ctx, id)
		if err != nil {
			log.Warn().Err(err).Int64("memory_id", id).Msg("Failed to calculate importance")
			continue
		}
		results = append(results, importance)
	}
	
	return results, nil
}

// UpdateImportanceScores updates importance scores in database
func (s *Scorer) UpdateImportanceScores(ctx context.Context, scores []*MemoryImportance) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE memories 
		SET importance_score = $1,
		    importance_updated_at = NOW()
		WHERE id = $2
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, score := range scores {
		_, err = stmt.ExecContext(ctx, score.Score, score.MemoryID)
		if err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

// GetLowImportanceMemories returns memories with low importance scores
func (s *Scorer) GetLowImportanceMemories(ctx context.Context, threshold float64, limit int) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id 
		FROM memories 
		WHERE importance_score < $1
		AND created_at < NOW() - INTERVAL '7 days'
		ORDER BY importance_score ASC
		LIMIT $2
	`, threshold, limit)
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var memoryIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		memoryIDs = append(memoryIDs, id)
	}
	
	return memoryIDs, rows.Err()
}
