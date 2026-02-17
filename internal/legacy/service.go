// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"eva-mind/internal/brainstem/infrastructure/graph"
)

// LegacyService gerencia modo pos-morte, herdeiros e personality snapshots
type LegacyService struct {
	db          *sql.DB
	neo4jClient *graph.Neo4jClient
}

// NewLegacyService cria novo servico de legacy
func NewLegacyService(db *sql.DB, neo4j *graph.Neo4jClient) *LegacyService {
	return &LegacyService{db: db, neo4jClient: neo4j}
}

// Heir representa um herdeiro com consent granular
type Heir struct {
	ID           int64  `json:"id"`
	IdosoID      int64  `json:"idoso_id"`
	Name         string `json:"name"`
	CPF          string `json:"cpf"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Relationship string `json:"relationship"`

	// Consent granular
	CanReadMemories    bool `json:"can_read_memories"`
	CanReadEmotions    bool `json:"can_read_emotions"`
	CanReadSignifiers  bool `json:"can_read_signifiers"`
	CanReadPersonality bool `json:"can_read_personality"`
	CanReadClinical    bool `json:"can_read_clinical"`
	CanActivatePosMorte bool `json:"can_activate_pos_morte"`
	CanExportSnapshot  bool `json:"can_export_snapshot"`

	ConsentGivenAt     *time.Time `json:"consent_given_at,omitempty"`
	ConsentGivenMethod string     `json:"consent_given_method,omitempty"`
	IsActive           bool       `json:"is_active"`
}

// PersonalitySnapshot exportacao completa da personalidade
type PersonalitySnapshot struct {
	ID                   int64           `json:"id"`
	IdosoID              int64           `json:"idoso_id"`
	Version              int             `json:"version"`
	EnneagramType        int             `json:"enneagram_type"`
	EnneagramWing        int             `json:"enneagram_wing"`
	TopSignifiers        json.RawMessage `json:"top_signifiers"`
	TopMemories          json.RawMessage `json:"top_memories"`
	EmotionalProfile     json.RawMessage `json:"emotional_profile"`
	LacanState           json.RawMessage `json:"lacan_state"`
	SignificantRelations json.RawMessage `json:"significant_relations"`
	CreatedAt            time.Time       `json:"created_at"`
	SizeBytes            int             `json:"size_bytes"`
}

// LegacyStatus status do modo legacy de um paciente
type LegacyStatus struct {
	IdosoID          int64      `json:"idoso_id"`
	LegacyMode       bool       `json:"legacy_mode"`
	PosMorte         bool       `json:"pos_morte"`
	PosMorteAt       *time.Time `json:"pos_morte_activated_at,omitempty"`
	TotalHeirs       int        `json:"total_heirs"`
	HasSnapshot      bool       `json:"has_snapshot"`
	LastSnapshotDate *time.Time `json:"last_snapshot_date,omitempty"`
}

// EnableLegacyMode ativa o modo legacy para um paciente
func (ls *LegacyService) EnableLegacyMode(ctx context.Context, idosoID int64) error {
	_, err := ls.db.ExecContext(ctx,
		"UPDATE idosos SET legacy_mode = TRUE WHERE id = $1", idosoID)
	if err != nil {
		return fmt.Errorf("falha ao ativar legacy mode: %w", err)
	}
	log.Printf("[LEGACY] Legacy mode ativado para paciente %d", idosoID)
	return nil
}

// ActivatePosMorte ativa modo pos-morte (verificando permissao do herdeiro)
func (ls *LegacyService) ActivatePosMorte(ctx context.Context, idosoID int64, heirCPF string) error {
	// Verificar se herdeiro tem permissao
	var canActivate bool
	err := ls.db.QueryRowContext(ctx,
		`SELECT can_activate_pos_morte FROM legacy_heirs
		 WHERE idoso_id = $1 AND heir_cpf = $2 AND is_active = TRUE`,
		idosoID, heirCPF).Scan(&canActivate)

	if err == sql.ErrNoRows {
		return fmt.Errorf("herdeiro nao encontrado ou inativo")
	}
	if err != nil {
		return fmt.Errorf("erro ao verificar herdeiro: %w", err)
	}
	if !canActivate {
		return fmt.Errorf("herdeiro nao tem permissao para ativar pos-morte")
	}

	// Verificar se legacy_mode esta ativo
	var legacyMode bool
	ls.db.QueryRowContext(ctx,
		"SELECT legacy_mode FROM idosos WHERE id = $1", idosoID).Scan(&legacyMode)
	if !legacyMode {
		return fmt.Errorf("legacy mode nao esta ativo para este paciente")
	}

	// Ativar pos-morte
	_, err = ls.db.ExecContext(ctx,
		`UPDATE idosos SET pos_morte = TRUE, pos_morte_activated_at = NOW(),
		 pos_morte_activated_by = $2 WHERE id = $1`,
		idosoID, heirCPF)
	if err != nil {
		return fmt.Errorf("falha ao ativar pos-morte: %w", err)
	}

	// Log de auditoria
	ls.logAccess(ctx, idosoID, heirCPF, "activate_pos_morte", "Post-mortem mode activated")

	log.Printf("[LEGACY] Pos-morte ativado para paciente %d por herdeiro %s", idosoID, heirCPF)
	return nil
}

// RegisterHeir registra um novo herdeiro com consent granular
func (ls *LegacyService) RegisterHeir(ctx context.Context, heir *Heir) error {
	query := `
		INSERT INTO legacy_heirs
		(idoso_id, heir_name, heir_cpf, heir_email, heir_phone, relationship,
		 can_read_memories, can_read_emotions, can_read_signifiers, can_read_personality,
		 can_read_clinical, can_activate_pos_morte, can_export_snapshot,
		 consent_given_at, consent_given_method, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, TRUE)
		ON CONFLICT (idoso_id, heir_cpf) DO UPDATE SET
			heir_name = EXCLUDED.heir_name,
			can_read_memories = EXCLUDED.can_read_memories,
			can_read_emotions = EXCLUDED.can_read_emotions,
			can_read_signifiers = EXCLUDED.can_read_signifiers,
			can_read_personality = EXCLUDED.can_read_personality,
			can_read_clinical = EXCLUDED.can_read_clinical,
			can_activate_pos_morte = EXCLUDED.can_activate_pos_morte,
			can_export_snapshot = EXCLUDED.can_export_snapshot,
			updated_at = NOW()
		RETURNING id`

	return ls.db.QueryRowContext(ctx, query,
		heir.IdosoID, heir.Name, heir.CPF, heir.Email, heir.Phone, heir.Relationship,
		heir.CanReadMemories, heir.CanReadEmotions, heir.CanReadSignifiers,
		heir.CanReadPersonality, heir.CanReadClinical, heir.CanActivatePosMorte,
		heir.CanExportSnapshot, heir.ConsentGivenAt, heir.ConsentGivenMethod,
	).Scan(&heir.ID)
}

// GetHeirs retorna todos os herdeiros ativos de um paciente
func (ls *LegacyService) GetHeirs(ctx context.Context, idosoID int64) ([]Heir, error) {
	query := `
		SELECT id, idoso_id, heir_name, heir_cpf, heir_email, heir_phone, relationship,
		       can_read_memories, can_read_emotions, can_read_signifiers, can_read_personality,
		       can_read_clinical, can_activate_pos_morte, can_export_snapshot,
		       consent_given_at, consent_given_method, is_active
		FROM legacy_heirs WHERE idoso_id = $1 AND is_active = TRUE
		ORDER BY heir_name`

	rows, err := ls.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var heirs []Heir
	for rows.Next() {
		var h Heir
		err := rows.Scan(
			&h.ID, &h.IdosoID, &h.Name, &h.CPF, &h.Email, &h.Phone, &h.Relationship,
			&h.CanReadMemories, &h.CanReadEmotions, &h.CanReadSignifiers,
			&h.CanReadPersonality, &h.CanReadClinical, &h.CanActivatePosMorte,
			&h.CanExportSnapshot, &h.ConsentGivenAt, &h.ConsentGivenMethod, &h.IsActive,
		)
		if err != nil {
			return nil, err
		}
		heirs = append(heirs, h)
	}
	return heirs, rows.Err()
}

// CreatePersonalitySnapshot exporta Enneagram + significantes + top-K memorias como JSON
func (ls *LegacyService) CreatePersonalitySnapshot(ctx context.Context, idosoID int64, createdBy string) (*PersonalitySnapshot, error) {
	snapshot := &PersonalitySnapshot{
		IdosoID:   idosoID,
		CreatedAt: time.Now(),
	}

	// 1. Buscar Enneagram type
	ls.db.QueryRowContext(ctx,
		`SELECT enneagram_type, enneagram_wing FROM personality_assessments
		 WHERE idoso_id = $1 ORDER BY created_at DESC LIMIT 1`,
		idosoID).Scan(&snapshot.EnneagramType, &snapshot.EnneagramWing)

	// 2. Buscar top significantes do Neo4j
	if ls.neo4jClient != nil {
		signifiers, err := ls.getTopSignifiers(ctx, idosoID, 20)
		if err == nil {
			snapshot.TopSignifiers, _ = json.Marshal(signifiers)
		}
	}

	// 3. Buscar top-K memorias mais importantes
	topMemories, err := ls.getTopMemories(ctx, idosoID, 50)
	if err == nil {
		snapshot.TopMemories, _ = json.Marshal(topMemories)
	}

	// 4. Calcular perfil emocional
	emotionalProfile, err := ls.calculateEmotionalProfile(ctx, idosoID)
	if err == nil {
		snapshot.EmotionalProfile, _ = json.Marshal(emotionalProfile)
	}

	// 5. Buscar relacoes significativas do grafo
	if ls.neo4jClient != nil {
		relations, err := ls.getSignificantRelations(ctx, idosoID)
		if err == nil {
			snapshot.SignificantRelations, _ = json.Marshal(relations)
		}
	}

	// 6. Marcar snapshots anteriores como nao-latest
	ls.db.ExecContext(ctx,
		"UPDATE personality_snapshots SET is_latest = FALSE WHERE idoso_id = $1", idosoID)

	// 7. Calcular tamanho
	fullData := map[string]interface{}{
		"enneagram_type":        snapshot.EnneagramType,
		"enneagram_wing":        snapshot.EnneagramWing,
		"top_signifiers":        snapshot.TopSignifiers,
		"top_memories":          snapshot.TopMemories,
		"emotional_profile":     snapshot.EmotionalProfile,
		"significant_relations": snapshot.SignificantRelations,
	}
	fullJSON, _ := json.Marshal(fullData)
	snapshot.SizeBytes = len(fullJSON)

	// 8. Salvar snapshot
	query := `
		INSERT INTO personality_snapshots
		(idoso_id, enneagram_type, enneagram_wing, top_signifiers, top_memories,
		 emotional_profile, significant_relations, full_snapshot,
		 created_by, snapshot_size_bytes, is_latest)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, TRUE)
		RETURNING id, snapshot_version`

	err = ls.db.QueryRowContext(ctx, query,
		idosoID, snapshot.EnneagramType, snapshot.EnneagramWing,
		snapshot.TopSignifiers, snapshot.TopMemories,
		snapshot.EmotionalProfile, snapshot.SignificantRelations, fullJSON,
		createdBy, snapshot.SizeBytes,
	).Scan(&snapshot.ID, &snapshot.Version)

	if err != nil {
		return nil, fmt.Errorf("falha ao salvar snapshot: %w", err)
	}

	log.Printf("[LEGACY] Personality snapshot criado para paciente %d (v%d, %d bytes)",
		idosoID, snapshot.Version, snapshot.SizeBytes)
	return snapshot, nil
}

// ReadMemoriesAsHeir permite herdeiro ler memorias (read-only, pos-morte)
func (ls *LegacyService) ReadMemoriesAsHeir(ctx context.Context, idosoID int64, heirCPF string, limit int) ([]map[string]interface{}, error) {
	// 1. Verificar pos-morte ativo
	var posMorte bool
	err := ls.db.QueryRowContext(ctx,
		"SELECT pos_morte FROM idosos WHERE id = $1", idosoID).Scan(&posMorte)
	if err != nil {
		return nil, fmt.Errorf("paciente nao encontrado: %w", err)
	}
	if !posMorte {
		return nil, fmt.Errorf("modo pos-morte nao esta ativo")
	}

	// 2. Verificar consent do herdeiro
	var canRead bool
	err = ls.db.QueryRowContext(ctx,
		`SELECT can_read_memories FROM legacy_heirs
		 WHERE idoso_id = $1 AND heir_cpf = $2 AND is_active = TRUE`,
		idosoID, heirCPF).Scan(&canRead)
	if err != nil || !canRead {
		return nil, fmt.Errorf("herdeiro nao tem permissao para ler memorias")
	}

	// 3. Buscar memorias (read-only, sem embeddings)
	query := `
		SELECT id, timestamp, speaker, content, emotion, importance, topics
		FROM episodic_memories
		WHERE idoso_id = $1
		ORDER BY importance DESC, timestamp DESC
		LIMIT $2`

	rows, err := ls.db.QueryContext(ctx, query, idosoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []map[string]interface{}
	for rows.Next() {
		var id int64
		var ts time.Time
		var speaker, content, emotion, topics string
		var importance float64

		err := rows.Scan(&id, &ts, &speaker, &content, &emotion, &importance, &topics)
		if err != nil {
			continue
		}

		memories = append(memories, map[string]interface{}{
			"id":         id,
			"timestamp":  ts.Format(time.RFC3339),
			"speaker":    speaker,
			"content":    content,
			"emotion":    emotion,
			"importance": importance,
		})
	}

	// 4. Log de auditoria
	ls.logAccess(ctx, idosoID, heirCPF, "read_memories",
		fmt.Sprintf("Read %d memories", len(memories)))

	return memories, nil
}

// ReadPersonalityAsHeir permite herdeiro ler personalidade (read-only)
func (ls *LegacyService) ReadPersonalityAsHeir(ctx context.Context, idosoID int64, heirCPF string) (*PersonalitySnapshot, error) {
	// 1. Verificar pos-morte
	var posMorte bool
	ls.db.QueryRowContext(ctx, "SELECT pos_morte FROM idosos WHERE id = $1", idosoID).Scan(&posMorte)
	if !posMorte {
		return nil, fmt.Errorf("modo pos-morte nao esta ativo")
	}

	// 2. Verificar consent
	var canRead bool
	ls.db.QueryRowContext(ctx,
		`SELECT can_read_personality FROM legacy_heirs
		 WHERE idoso_id = $1 AND heir_cpf = $2 AND is_active = TRUE`,
		idosoID, heirCPF).Scan(&canRead)
	if !canRead {
		return nil, fmt.Errorf("herdeiro nao tem permissao para ler personalidade")
	}

	// 3. Buscar latest snapshot
	snapshot := &PersonalitySnapshot{}
	err := ls.db.QueryRowContext(ctx,
		`SELECT id, idoso_id, snapshot_version, enneagram_type, enneagram_wing,
		        top_signifiers, top_memories, emotional_profile, significant_relations,
		        created_at, snapshot_size_bytes
		 FROM personality_snapshots
		 WHERE idoso_id = $1 AND is_latest = TRUE`,
		idosoID).Scan(
		&snapshot.ID, &snapshot.IdosoID, &snapshot.Version,
		&snapshot.EnneagramType, &snapshot.EnneagramWing,
		&snapshot.TopSignifiers, &snapshot.TopMemories,
		&snapshot.EmotionalProfile, &snapshot.SignificantRelations,
		&snapshot.CreatedAt, &snapshot.SizeBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("snapshot nao encontrado: %w", err)
	}

	// 4. Log
	ls.logAccess(ctx, idosoID, heirCPF, "read_personality", "Read personality snapshot")

	return snapshot, nil
}

// GetLegacyStatus retorna status do legacy mode de um paciente
func (ls *LegacyService) GetLegacyStatus(ctx context.Context, idosoID int64) (*LegacyStatus, error) {
	status := &LegacyStatus{IdosoID: idosoID}

	err := ls.db.QueryRowContext(ctx,
		"SELECT legacy_mode, pos_morte, pos_morte_activated_at FROM idosos WHERE id = $1",
		idosoID).Scan(&status.LegacyMode, &status.PosMorte, &status.PosMorteAt)
	if err != nil {
		return nil, err
	}

	ls.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM legacy_heirs WHERE idoso_id = $1 AND is_active = TRUE",
		idosoID).Scan(&status.TotalHeirs)

	var snapshotCount int
	ls.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM personality_snapshots WHERE idoso_id = $1 AND is_latest = TRUE",
		idosoID).Scan(&snapshotCount)
	status.HasSnapshot = snapshotCount > 0

	if status.HasSnapshot {
		ls.db.QueryRowContext(ctx,
			"SELECT MAX(created_at) FROM personality_snapshots WHERE idoso_id = $1",
			idosoID).Scan(&status.LastSnapshotDate)
	}

	return status, nil
}

// IsPosMorte verifica se paciente esta em modo pos-morte (utility para outros servicos)
func (ls *LegacyService) IsPosMorte(ctx context.Context, idosoID int64) bool {
	var posMorte bool
	ls.db.QueryRowContext(ctx,
		"SELECT pos_morte FROM idosos WHERE id = $1", idosoID).Scan(&posMorte)
	return posMorte
}

// --- Helpers internos ---

func (ls *LegacyService) getTopSignifiers(ctx context.Context, idosoID int64, topN int) ([]map[string]interface{}, error) {
	query := `
		MATCH (s:Significante {idoso_id: $idosoId})
		WHERE s.frequency >= 3
		RETURN s.word AS word, s.frequency AS frequency
		ORDER BY s.frequency DESC
		LIMIT $limit`

	records, err := ls.neo4jClient.ExecuteRead(ctx, query, map[string]interface{}{
		"idosoId": idosoID,
		"limit":   topN,
	})
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, record := range records {
		word, _ := record.Get("word")
		freq, _ := record.Get("frequency")
		result = append(result, map[string]interface{}{
			"word":      word,
			"frequency": freq,
		})
	}
	return result, nil
}

func (ls *LegacyService) getTopMemories(ctx context.Context, idosoID int64, topN int) ([]map[string]interface{}, error) {
	query := `
		SELECT content, emotion, importance, timestamp
		FROM episodic_memories
		WHERE idoso_id = $1
		ORDER BY importance DESC
		LIMIT $2`

	rows, err := ls.db.QueryContext(ctx, query, idosoID, topN)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var content, emotion string
		var importance float64
		var ts time.Time
		rows.Scan(&content, &emotion, &importance, &ts)
		result = append(result, map[string]interface{}{
			"content":    content,
			"emotion":    emotion,
			"importance": importance,
			"timestamp":  ts.Format(time.RFC3339),
		})
	}
	return result, nil
}

func (ls *LegacyService) calculateEmotionalProfile(ctx context.Context, idosoID int64) (map[string]interface{}, error) {
	query := `
		SELECT emotion, COUNT(*) as count, AVG(importance) as avg_importance
		FROM episodic_memories
		WHERE idoso_id = $1 AND emotion IS NOT NULL AND emotion != ''
		GROUP BY emotion
		ORDER BY count DESC
		LIMIT 10`

	rows, err := ls.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	emotions := make([]map[string]interface{}, 0)
	for rows.Next() {
		var emotion string
		var count int
		var avgImportance float64
		rows.Scan(&emotion, &count, &avgImportance)
		emotions = append(emotions, map[string]interface{}{
			"emotion":        emotion,
			"count":          count,
			"avg_importance": avgImportance,
		})
	}

	profile := map[string]interface{}{
		"emotions": emotions,
	}
	if len(emotions) > 0 {
		profile["dominant"] = emotions[0]["emotion"]
	}
	return profile, nil
}

func (ls *LegacyService) getSignificantRelations(ctx context.Context, idosoID int64) ([]map[string]interface{}, error) {
	query := `
		MATCH (p:Person {id: $idosoId})-[:MENTIONED]->(person:Person)
		RETURN person.name AS name, person.relation AS relation, COUNT(*) AS mentions
		ORDER BY mentions DESC
		LIMIT 20`

	records, err := ls.neo4jClient.ExecuteRead(ctx, query, map[string]interface{}{
		"idosoId": idosoID,
	})
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, record := range records {
		name, _ := record.Get("name")
		relation, _ := record.Get("relation")
		mentions, _ := record.Get("mentions")
		result = append(result, map[string]interface{}{
			"name":     name,
			"relation": relation,
			"mentions": mentions,
		})
	}
	return result, nil
}

func (ls *LegacyService) logAccess(ctx context.Context, idosoID int64, heirCPF, actionType, detail string) {
	ls.db.ExecContext(ctx,
		`INSERT INTO legacy_access_log (idoso_id, heir_cpf, action_type, action_detail)
		 VALUES ($1, $2, $3, $4)`,
		idosoID, heirCPF, actionType, detail)
}
