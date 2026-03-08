// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// LegacyService gerencia modo pos-morte, herdeiros e personality snapshots
type LegacyService struct {
	db          *database.DB
	graphClient *nietzscheInfra.GraphAdapter
}

// NewLegacyService cria novo servico de legacy
func NewLegacyService(db *database.DB, graphAdapter *nietzscheInfra.GraphAdapter) *LegacyService {
	return &LegacyService{db: db, graphClient: graphAdapter}
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
	err := ls.db.Update(ctx, "idosos",
		map[string]interface{}{"id": float64(idosoID)},
		map[string]interface{}{"legacy_mode": true})
	if err != nil {
		return fmt.Errorf("falha ao ativar legacy mode: %w", err)
	}
	log.Printf("[LEGACY] Legacy mode ativado para paciente %d", idosoID)
	return nil
}

// ActivatePosMorte ativa modo pos-morte (verificando permissao do herdeiro)
func (ls *LegacyService) ActivatePosMorte(ctx context.Context, idosoID int64, heirCPF string) error {
	// Verificar se herdeiro tem permissao
	rows, err := ls.db.QueryByLabel(ctx, "legacy_heirs",
		" AND n.idoso_id = $iid AND n.heir_cpf = $cpf AND n.is_active = $active",
		map[string]interface{}{
			"iid":    float64(idosoID),
			"cpf":    heirCPF,
			"active": true,
		}, 1)
	if err != nil {
		return fmt.Errorf("erro ao verificar herdeiro: %w", err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("herdeiro nao encontrado ou inativo")
	}
	canActivate := database.GetBool(rows[0], "can_activate_pos_morte")
	if !canActivate {
		return fmt.Errorf("herdeiro nao tem permissao para ativar pos-morte")
	}

	// Verificar se legacy_mode esta ativo
	idosoNode, err := ls.db.GetNodeByID(ctx, "idosos", idosoID)
	if err != nil || idosoNode == nil {
		return fmt.Errorf("paciente nao encontrado")
	}
	legacyMode := database.GetBool(idosoNode, "legacy_mode")
	if !legacyMode {
		return fmt.Errorf("legacy mode nao esta ativo para este paciente")
	}

	// Ativar pos-morte
	err = ls.db.Update(ctx, "idosos",
		map[string]interface{}{"id": float64(idosoID)},
		map[string]interface{}{
			"pos_morte":              true,
			"pos_morte_activated_at": time.Now().Format(time.RFC3339),
			"pos_morte_activated_by": heirCPF,
		})
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
	// Check if heir already exists (upsert logic)
	existing, err := ls.db.QueryByLabel(ctx, "legacy_heirs",
		" AND n.idoso_id = $iid AND n.heir_cpf = $cpf",
		map[string]interface{}{
			"iid": float64(heir.IdosoID),
			"cpf": heir.CPF,
		}, 1)
	if err != nil {
		return fmt.Errorf("erro ao verificar herdeiro existente: %w", err)
	}

	content := map[string]interface{}{
		"idoso_id":               float64(heir.IdosoID),
		"heir_name":              heir.Name,
		"heir_cpf":               heir.CPF,
		"heir_email":             heir.Email,
		"heir_phone":             heir.Phone,
		"relationship":           heir.Relationship,
		"can_read_memories":      heir.CanReadMemories,
		"can_read_emotions":      heir.CanReadEmotions,
		"can_read_signifiers":    heir.CanReadSignifiers,
		"can_read_personality":   heir.CanReadPersonality,
		"can_read_clinical":      heir.CanReadClinical,
		"can_activate_pos_morte": heir.CanActivatePosMorte,
		"can_export_snapshot":    heir.CanExportSnapshot,
		"is_active":              true,
	}

	if heir.ConsentGivenAt != nil {
		content["consent_given_at"] = heir.ConsentGivenAt.Format(time.RFC3339)
	}
	if heir.ConsentGivenMethod != "" {
		content["consent_given_method"] = heir.ConsentGivenMethod
	}

	if len(existing) > 0 {
		// Update existing heir
		content["updated_at"] = time.Now().Format(time.RFC3339)
		err = ls.db.Update(ctx, "legacy_heirs",
			map[string]interface{}{
				"idoso_id": float64(heir.IdosoID),
				"heir_cpf": heir.CPF,
			},
			content)
		if err != nil {
			return fmt.Errorf("erro ao atualizar herdeiro: %w", err)
		}
		heir.ID = database.GetInt64(existing[0], "id")
	} else {
		// Insert new heir
		id, err := ls.db.Insert(ctx, "legacy_heirs", content)
		if err != nil {
			return fmt.Errorf("erro ao registrar herdeiro: %w", err)
		}
		heir.ID = id
	}
	return nil
}

// GetHeirs retorna todos os herdeiros ativos de um paciente
func (ls *LegacyService) GetHeirs(ctx context.Context, idosoID int64) ([]Heir, error) {
	rows, err := ls.db.QueryByLabel(ctx, "legacy_heirs",
		" AND n.idoso_id = $iid AND n.is_active = $active",
		map[string]interface{}{
			"iid":    float64(idosoID),
			"active": true,
		}, 0)
	if err != nil {
		return nil, err
	}

	var heirs []Heir
	for _, m := range rows {
		h := Heir{
			ID:                  database.GetInt64(m, "id"),
			IdosoID:             database.GetInt64(m, "idoso_id"),
			Name:                database.GetString(m, "heir_name"),
			CPF:                 database.GetString(m, "heir_cpf"),
			Email:               database.GetString(m, "heir_email"),
			Phone:               database.GetString(m, "heir_phone"),
			Relationship:        database.GetString(m, "relationship"),
			CanReadMemories:     database.GetBool(m, "can_read_memories"),
			CanReadEmotions:     database.GetBool(m, "can_read_emotions"),
			CanReadSignifiers:   database.GetBool(m, "can_read_signifiers"),
			CanReadPersonality:  database.GetBool(m, "can_read_personality"),
			CanReadClinical:     database.GetBool(m, "can_read_clinical"),
			CanActivatePosMorte: database.GetBool(m, "can_activate_pos_morte"),
			CanExportSnapshot:   database.GetBool(m, "can_export_snapshot"),
			ConsentGivenAt:      database.GetTimePtr(m, "consent_given_at"),
			ConsentGivenMethod:  database.GetString(m, "consent_given_method"),
			IsActive:            database.GetBool(m, "is_active"),
		}
		heirs = append(heirs, h)
	}

	// Sort by name in Go (replaces ORDER BY heir_name)
	sort.Slice(heirs, func(i, j int) bool {
		return heirs[i].Name < heirs[j].Name
	})

	return heirs, nil
}

// CreatePersonalitySnapshot exporta Enneagram + significantes + top-K memorias como JSON
func (ls *LegacyService) CreatePersonalitySnapshot(ctx context.Context, idosoID int64, createdBy string) (*PersonalitySnapshot, error) {
	snapshot := &PersonalitySnapshot{
		IdosoID:   idosoID,
		CreatedAt: time.Now(),
	}

	// 1. Buscar Enneagram type
	assessments, err := ls.db.QueryByLabel(ctx, "personality_assessments",
		" AND n.idoso_id = $iid",
		map[string]interface{}{"iid": float64(idosoID)}, 0)
	if err == nil && len(assessments) > 0 {
		// Sort by created_at DESC in Go, pick latest
		sort.Slice(assessments, func(i, j int) bool {
			ti := database.GetTime(assessments[i], "created_at")
			tj := database.GetTime(assessments[j], "created_at")
			return ti.After(tj)
		})
		latest := assessments[0]
		snapshot.EnneagramType = int(database.GetInt64(latest, "enneagram_type"))
		snapshot.EnneagramWing = int(database.GetInt64(latest, "enneagram_wing"))
	}

	// 2. Buscar top significantes do grafo (NQL graph query - unchanged)
	if ls.graphClient != nil {
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

	// 5. Buscar relacoes significativas do grafo (NQL graph query - unchanged)
	if ls.graphClient != nil {
		relations, err := ls.getSignificantRelations(ctx, idosoID)
		if err == nil {
			snapshot.SignificantRelations, _ = json.Marshal(relations)
		}
	}

	// 6. Marcar snapshots anteriores como nao-latest
	existingSnapshots, _ := ls.db.QueryByLabel(ctx, "personality_snapshots",
		" AND n.idoso_id = $iid AND n.is_latest = $latest",
		map[string]interface{}{
			"iid":    float64(idosoID),
			"latest": true,
		}, 0)
	for _, s := range existingSnapshots {
		sID := database.GetInt64(s, "id")
		_ = ls.db.Update(ctx, "personality_snapshots",
			map[string]interface{}{"id": float64(sID)},
			map[string]interface{}{"is_latest": false})
	}

	// 7. Calcular tamanho e versao
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

	// Determine version from existing snapshots count
	allSnapshots, _ := ls.db.QueryByLabel(ctx, "personality_snapshots",
		" AND n.idoso_id = $iid",
		map[string]interface{}{"iid": float64(idosoID)}, 0)
	snapshot.Version = len(allSnapshots) + 1

	// 8. Salvar snapshot
	content := map[string]interface{}{
		"idoso_id":               float64(idosoID),
		"enneagram_type":         float64(snapshot.EnneagramType),
		"enneagram_wing":         float64(snapshot.EnneagramWing),
		"top_signifiers":         string(snapshot.TopSignifiers),
		"top_memories":           string(snapshot.TopMemories),
		"emotional_profile":      string(snapshot.EmotionalProfile),
		"significant_relations":  string(snapshot.SignificantRelations),
		"full_snapshot":          string(fullJSON),
		"created_by":             createdBy,
		"snapshot_size_bytes":    float64(snapshot.SizeBytes),
		"snapshot_version":       float64(snapshot.Version),
		"is_latest":              true,
		"created_at":             snapshot.CreatedAt.Format(time.RFC3339),
	}

	id, err := ls.db.Insert(ctx, "personality_snapshots", content)
	if err != nil {
		return nil, fmt.Errorf("falha ao salvar snapshot: %w", err)
	}
	snapshot.ID = id

	log.Printf("[LEGACY] Personality snapshot criado para paciente %d (v%d, %d bytes)",
		idosoID, snapshot.Version, snapshot.SizeBytes)
	return snapshot, nil
}

// ReadMemoriesAsHeir permite herdeiro ler memorias (read-only, pos-morte)
func (ls *LegacyService) ReadMemoriesAsHeir(ctx context.Context, idosoID int64, heirCPF string, limit int) ([]map[string]interface{}, error) {
	// 1. Verificar pos-morte ativo
	idosoNode, err := ls.db.GetNodeByID(ctx, "idosos", idosoID)
	if err != nil || idosoNode == nil {
		return nil, fmt.Errorf("paciente nao encontrado: %v", err)
	}
	posMorte := database.GetBool(idosoNode, "pos_morte")
	if !posMorte {
		return nil, fmt.Errorf("modo pos-morte nao esta ativo")
	}

	// 2. Verificar consent do herdeiro
	heirRows, err := ls.db.QueryByLabel(ctx, "legacy_heirs",
		" AND n.idoso_id = $iid AND n.heir_cpf = $cpf AND n.is_active = $active",
		map[string]interface{}{
			"iid":    float64(idosoID),
			"cpf":    heirCPF,
			"active": true,
		}, 1)
	if err != nil || len(heirRows) == 0 {
		return nil, fmt.Errorf("herdeiro nao tem permissao para ler memorias")
	}
	canRead := database.GetBool(heirRows[0], "can_read_memories")
	if !canRead {
		return nil, fmt.Errorf("herdeiro nao tem permissao para ler memorias")
	}

	// 3. Buscar memorias (read-only, sem embeddings)
	memRows, err := ls.db.QueryByLabel(ctx, "episodic_memories",
		" AND n.idoso_id = $iid",
		map[string]interface{}{"iid": float64(idosoID)}, 0)
	if err != nil {
		return nil, err
	}

	// Sort by importance DESC, timestamp DESC in Go
	sort.Slice(memRows, func(i, j int) bool {
		impI := database.GetFloat64(memRows[i], "importance")
		impJ := database.GetFloat64(memRows[j], "importance")
		if impI != impJ {
			return impI > impJ
		}
		tsI := database.GetTime(memRows[i], "timestamp")
		tsJ := database.GetTime(memRows[j], "timestamp")
		return tsI.After(tsJ)
	})

	if limit > 0 && len(memRows) > limit {
		memRows = memRows[:limit]
	}

	var memories []map[string]interface{}
	for _, m := range memRows {
		memories = append(memories, map[string]interface{}{
			"id":         database.GetInt64(m, "id"),
			"timestamp":  database.GetTime(m, "timestamp").Format(time.RFC3339),
			"speaker":    database.GetString(m, "speaker"),
			"content":    database.GetString(m, "content"),
			"emotion":    database.GetString(m, "emotion"),
			"importance": database.GetFloat64(m, "importance"),
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
	idosoNode, err := ls.db.GetNodeByID(ctx, "idosos", idosoID)
	if err != nil || idosoNode == nil {
		return nil, fmt.Errorf("paciente nao encontrado")
	}
	posMorte := database.GetBool(idosoNode, "pos_morte")
	if !posMorte {
		return nil, fmt.Errorf("modo pos-morte nao esta ativo")
	}

	// 2. Verificar consent
	heirRows, err := ls.db.QueryByLabel(ctx, "legacy_heirs",
		" AND n.idoso_id = $iid AND n.heir_cpf = $cpf AND n.is_active = $active",
		map[string]interface{}{
			"iid":    float64(idosoID),
			"cpf":    heirCPF,
			"active": true,
		}, 1)
	if err != nil || len(heirRows) == 0 {
		return nil, fmt.Errorf("herdeiro nao tem permissao para ler personalidade")
	}
	canRead := database.GetBool(heirRows[0], "can_read_personality")
	if !canRead {
		return nil, fmt.Errorf("herdeiro nao tem permissao para ler personalidade")
	}

	// 3. Buscar latest snapshot
	snapRows, err := ls.db.QueryByLabel(ctx, "personality_snapshots",
		" AND n.idoso_id = $iid AND n.is_latest = $latest",
		map[string]interface{}{
			"iid":    float64(idosoID),
			"latest": true,
		}, 1)
	if err != nil || len(snapRows) == 0 {
		return nil, fmt.Errorf("snapshot nao encontrado")
	}

	m := snapRows[0]
	snapshot := &PersonalitySnapshot{
		ID:                   database.GetInt64(m, "id"),
		IdosoID:              database.GetInt64(m, "idoso_id"),
		Version:              int(database.GetInt64(m, "snapshot_version")),
		EnneagramType:        int(database.GetInt64(m, "enneagram_type")),
		EnneagramWing:        int(database.GetInt64(m, "enneagram_wing")),
		TopSignifiers:        json.RawMessage(database.GetString(m, "top_signifiers")),
		TopMemories:          json.RawMessage(database.GetString(m, "top_memories")),
		EmotionalProfile:     json.RawMessage(database.GetString(m, "emotional_profile")),
		SignificantRelations: json.RawMessage(database.GetString(m, "significant_relations")),
		CreatedAt:            database.GetTime(m, "created_at"),
		SizeBytes:            int(database.GetInt64(m, "snapshot_size_bytes")),
	}

	// 4. Log
	ls.logAccess(ctx, idosoID, heirCPF, "read_personality", "Read personality snapshot")

	return snapshot, nil
}

// GetLegacyStatus retorna status do legacy mode de um paciente
func (ls *LegacyService) GetLegacyStatus(ctx context.Context, idosoID int64) (*LegacyStatus, error) {
	status := &LegacyStatus{IdosoID: idosoID}

	idosoNode, err := ls.db.GetNodeByID(ctx, "idosos", idosoID)
	if err != nil || idosoNode == nil {
		return nil, fmt.Errorf("paciente nao encontrado")
	}

	status.LegacyMode = database.GetBool(idosoNode, "legacy_mode")
	status.PosMorte = database.GetBool(idosoNode, "pos_morte")
	status.PosMorteAt = database.GetTimePtr(idosoNode, "pos_morte_activated_at")

	// Count active heirs
	heirCount, err := ls.db.Count(ctx, "legacy_heirs",
		" AND n.idoso_id = $iid AND n.is_active = $active",
		map[string]interface{}{
			"iid":    float64(idosoID),
			"active": true,
		})
	if err == nil {
		status.TotalHeirs = heirCount
	}

	// Check for latest snapshot
	snapshotCount, err := ls.db.Count(ctx, "personality_snapshots",
		" AND n.idoso_id = $iid AND n.is_latest = $latest",
		map[string]interface{}{
			"iid":    float64(idosoID),
			"latest": true,
		})
	if err == nil {
		status.HasSnapshot = snapshotCount > 0
	}

	if status.HasSnapshot {
		// Get latest snapshot date
		snapRows, err := ls.db.QueryByLabel(ctx, "personality_snapshots",
			" AND n.idoso_id = $iid",
			map[string]interface{}{"iid": float64(idosoID)}, 0)
		if err == nil && len(snapRows) > 0 {
			var latestTime time.Time
			for _, s := range snapRows {
				t := database.GetTime(s, "created_at")
				if t.After(latestTime) {
					latestTime = t
				}
			}
			if !latestTime.IsZero() {
				status.LastSnapshotDate = &latestTime
			}
		}
	}

	return status, nil
}

// IsPosMorte verifica se paciente esta em modo pos-morte (utility para outros servicos)
func (ls *LegacyService) IsPosMorte(ctx context.Context, idosoID int64) bool {
	idosoNode, err := ls.db.GetNodeByID(ctx, "idosos", idosoID)
	if err != nil || idosoNode == nil {
		return false
	}
	return database.GetBool(idosoNode, "pos_morte")
}

// --- Helpers internos ---

func (ls *LegacyService) getTopSignifiers(ctx context.Context, idosoID int64, topN int) ([]map[string]interface{}, error) {
	// NQL graph query - unchanged
	nql := `MATCH (s:Significante {idoso_id: $idosoId}) WHERE s.frequency >= 3 RETURN s LIMIT $limit`
	queryResult, err := ls.graphClient.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
		"limit":   topN,
	}, "")
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, node := range queryResult.Nodes {
		result = append(result, map[string]interface{}{
			"word":      fmt.Sprintf("%v", node.Content["word"]),
			"frequency": node.Content["frequency"],
		})
	}
	return result, nil
}

func (ls *LegacyService) getTopMemories(ctx context.Context, idosoID int64, topN int) ([]map[string]interface{}, error) {
	rows, err := ls.db.QueryByLabel(ctx, "episodic_memories",
		" AND n.idoso_id = $iid",
		map[string]interface{}{"iid": float64(idosoID)}, 0)
	if err != nil {
		return nil, err
	}

	// Sort by importance DESC in Go
	sort.Slice(rows, func(i, j int) bool {
		return database.GetFloat64(rows[i], "importance") > database.GetFloat64(rows[j], "importance")
	})

	if topN > 0 && len(rows) > topN {
		rows = rows[:topN]
	}

	var result []map[string]interface{}
	for _, m := range rows {
		result = append(result, map[string]interface{}{
			"content":    database.GetString(m, "content"),
			"emotion":    database.GetString(m, "emotion"),
			"importance": database.GetFloat64(m, "importance"),
			"timestamp":  database.GetTime(m, "timestamp").Format(time.RFC3339),
		})
	}
	return result, nil
}

func (ls *LegacyService) calculateEmotionalProfile(ctx context.Context, idosoID int64) (map[string]interface{}, error) {
	rows, err := ls.db.QueryByLabel(ctx, "episodic_memories",
		" AND n.idoso_id = $iid",
		map[string]interface{}{"iid": float64(idosoID)}, 0)
	if err != nil {
		return nil, err
	}

	// GROUP BY emotion, COUNT(*), AVG(importance) in Go
	type emotionStats struct {
		count          int
		totalImportance float64
	}
	emotionMap := make(map[string]*emotionStats)

	for _, m := range rows {
		emotion := database.GetString(m, "emotion")
		if emotion == "" {
			continue
		}
		importance := database.GetFloat64(m, "importance")
		if s, ok := emotionMap[emotion]; ok {
			s.count++
			s.totalImportance += importance
		} else {
			emotionMap[emotion] = &emotionStats{count: 1, totalImportance: importance}
		}
	}

	emotions := make([]map[string]interface{}, 0, len(emotionMap))
	for emotion, stats := range emotionMap {
		avgImportance := 0.0
		if stats.count > 0 {
			avgImportance = stats.totalImportance / float64(stats.count)
		}
		emotions = append(emotions, map[string]interface{}{
			"emotion":        emotion,
			"count":          stats.count,
			"avg_importance": avgImportance,
		})
	}

	// Sort by count DESC
	sort.Slice(emotions, func(i, j int) bool {
		return emotions[i]["count"].(int) > emotions[j]["count"].(int)
	})

	// Limit to top 10
	if len(emotions) > 10 {
		emotions = emotions[:10]
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
	// NQL graph query - unchanged
	nql := `MATCH (p:Person {id: $idosoId})-[:MENTIONED]->(person:Person) RETURN person LIMIT 20`
	queryResult, err := ls.graphClient.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, node := range queryResult.Nodes {
		result = append(result, map[string]interface{}{
			"name":     fmt.Sprintf("%v", node.Content["name"]),
			"relation": fmt.Sprintf("%v", node.Content["relation"]),
			"mentions": node.Content["mentions"],
		})
	}
	return result, nil
}

func (ls *LegacyService) logAccess(ctx context.Context, idosoID int64, heirCPF, actionType, detail string) {
	_, _ = ls.db.Insert(ctx, "legacy_access_log", map[string]interface{}{
		"idoso_id":      float64(idosoID),
		"heir_cpf":      heirCPF,
		"action_type":   actionType,
		"action_detail": detail,
		"created_at":    time.Now().Format(time.RFC3339),
	})
}
