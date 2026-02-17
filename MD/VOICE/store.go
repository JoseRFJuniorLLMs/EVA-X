// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package voice

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// VoiceProfile é o perfil biométrico de um falante armazenado no Neo4j.
type VoiceProfile struct {
	SpeakerID     string    // ID único ex: "person_junior"
	Name          string    // Nome de exibição ex: "Junior"
	Centroid      []float64 // D-Vector centróide (512 dims, L2-normalizado)
	IntraVariance float64   // Variância média entre as amostras de enroll
	SampleCount   int       // Quantas amostras foram usadas no enroll
	EnrolledAt    time.Time
	Active        bool
}

// VoiceEvent registra cada tentativa de identificação (para Hebbian Update).
type VoiceEvent struct {
	SpeakerID   string
	CosineSim   float64
	ResidualErr float64
	Confidence  float64
	Confirmed   bool      // true = identificação aceita; false = desconhecido
	AudioQuality float64  // RMS dB do áudio (qualidade)
	Timestamp   time.Time
}

// HebbianDelta é o resultado de um update na conexão Person→VoiceProfile.
type HebbianDelta struct {
	SpeakerID    string
	OldConfidence float64
	NewConfidence float64
	Direction    string // "LTP" ou "LTD"
}

// ─── Neo4j Schema (rode uma única vez) ────────────────────────────────────
//
// CREATE CONSTRAINT person_id IF NOT EXISTS
//   FOR (p:Person) REQUIRE p.id IS UNIQUE;
//
// CREATE CONSTRAINT voice_profile_id IF NOT EXISTS
//   FOR (v:VoiceProfile) REQUIRE v.id IS UNIQUE;
//
// CREATE INDEX voice_profile_speaker IF NOT EXISTS
//   FOR (v:VoiceProfile) ON (v.speaker_id, v.active);
//
// // Estrutura do grafo:
// (Person {id, name}) -[:HAS_VOICE_PROFILE {confidence, created_at}]->
//     (VoiceProfile {id, speaker_id, centroid, intra_variance, sample_count, enrolled_at, active})
//
// (Person) -[:VOICE_EVENT {cosine_sim, residual_err, confidence, confirmed, audio_quality, ts}]->
//     (VoiceEvent {id, timestamp})
// ─────────────────────────────────────────────────────────────────────────

// Neo4jStore gerencia os perfis de voz no Neo4j.
type Neo4jStore struct {
	driver neo4j.DriverWithContext
	log    *zap.Logger
}

func NewNeo4jStore(driver neo4j.DriverWithContext, log *zap.Logger) *Neo4jStore {
	return &Neo4jStore{driver: driver, log: log}
}

// ─── Enroll ───────────────────────────────────────────────────────────────

// UpsertProfile cria ou atualiza o VoiceProfile de um falante.
// Chamado pelo endpoint de enroll após o cálculo do centróide.
func (s *Neo4jStore) UpsertProfile(ctx context.Context, profile VoiceProfile) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	profileID := "vp_" + profile.SpeakerID

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Cria/atualiza o nó VoiceProfile
		_, err := tx.Run(ctx, `
			MERGE (v:VoiceProfile {id: $profile_id})
			SET
				v.speaker_id      = $speaker_id,
				v.centroid        = $centroid,
				v.intra_variance  = $variance,
				v.sample_count    = $samples,
				v.enrolled_at     = datetime($enrolled_at),
				v.active          = true
			WITH v
			MATCH (p:Person {id: $speaker_id})
			MERGE (p)-[r:HAS_VOICE_PROFILE]->(v)
			ON CREATE SET r.confidence = 1.0, r.created_at = datetime()
			RETURN v.id
		`, map[string]any{
			"profile_id":  profileID,
			"speaker_id":  profile.SpeakerID,
			"centroid":    profile.Centroid,
			"variance":    profile.IntraVariance,
			"samples":     profile.SampleCount,
			"enrolled_at": profile.EnrolledAt.Format(time.RFC3339),
		})
		return nil, err
	})

	if err != nil {
		return fmt.Errorf("UpsertProfile %s: %w", profile.SpeakerID, err)
	}

	s.log.Info("voice profile upserted",
		zap.String("speaker_id", profile.SpeakerID),
		zap.Int("samples", profile.SampleCount),
		zap.Float64("variance", profile.IntraVariance),
	)
	return nil
}

// ─── Leitura de Perfis ─────────────────────────────────────────────────────

// LoadActiveProfiles carrega todos os perfis ativos. Usado pelo cache.
func (s *Neo4jStore) LoadActiveProfiles(ctx context.Context) ([]VoiceProfile, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rows, err := tx.Run(ctx, `
			MATCH (p:Person)-[:HAS_VOICE_PROFILE]->(v:VoiceProfile {active: true})
			RETURN
				p.id                  AS speaker_id,
				p.name                AS name,
				v.centroid            AS centroid,
				v.intra_variance      AS variance,
				v.sample_count        AS samples,
				toString(v.enrolled_at) AS enrolled_at
			ORDER BY p.name
		`, nil)
		if err != nil {
			return nil, err
		}

		var profiles []VoiceProfile
		for rows.Next(ctx) {
			rec := rows.Record()

			centroidRaw, _ := rec.Get("centroid")
			centroid := toFloat64Slice(centroidRaw)

			enrolledStr, _ := rec.Get("enrolled_at")
			enrolledAt, _ := time.Parse(time.RFC3339, fmt.Sprint(enrolledStr))

			profiles = append(profiles, VoiceProfile{
				SpeakerID:     fmt.Sprint(mustGet(rec, "speaker_id")),
				Name:          fmt.Sprint(mustGet(rec, "name")),
				Centroid:      centroid,
				IntraVariance: toFloat64(mustGet(rec, "variance")),
				SampleCount:   int(toInt64(mustGet(rec, "samples"))),
				EnrolledAt:    enrolledAt,
				Active:        true,
			})
		}
		return profiles, rows.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("LoadActiveProfiles: %w", err)
	}

	profiles := result.([]VoiceProfile)
	s.log.Debug("profiles loaded from neo4j", zap.Int("count", len(profiles)))
	return profiles, nil
}

// ─── Hebbian Update ────────────────────────────────────────────────────────
//
// Implementa a Regra de Hebb adaptada para biometria de voz:
//
//   LTP (Long-Term Potentiation):
//     Identificação confirmada → aumenta confiança da conexão Person→VoiceProfile
//     Δ = +ltpRate × cosineSim   (reconhecimentos fortes reforçam mais)
//
//   LTD (Long-Term Depression):
//     Voz divergente (alto resíduo, não confirmado) → reduz confiança
//     Δ = -ltdRate × residualErr  (quanto maior o desvio, mais forte a depressão)
//
//   Limites: confiança ∈ [0.10, 1.00]
//   Se confiança < 0.50 → EVA inicia re-enroll automático (ver pipeline.go)

const (
	hebbLTPRate = 0.04 // Taxa de potencialização
	hebbLTDRate = 0.08 // Taxa de depressão (maior para ser conservador)
	hebbMinConf = 0.10
	hebbMaxConf = 1.00
)

// HebbianUpdate atualiza o peso da relação Person→VoiceProfile.
// É chamado de forma assíncrona após cada identificação (fire and forget).
func (s *Neo4jStore) HebbianUpdate(ctx context.Context, event VoiceEvent) (HebbianDelta, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	var delta float64
	var direction string
	if event.Confirmed {
		delta = hebbLTPRate * event.CosineSim
		direction = "LTP"
	} else {
		delta = -hebbLTDRate * event.ResidualErr
		direction = "LTD"
	}

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		row, err := tx.Run(ctx, `
			MATCH (p:Person {id: $speaker_id})-[r:HAS_VOICE_PROFILE]->(v:VoiceProfile {active: true})
			WITH p, r, v,
				r.confidence AS old_conf,
				CASE
					WHEN r.confidence + $delta > $max THEN $max
					WHEN r.confidence + $delta < $min THEN $min
					ELSE r.confidence + $delta
				END AS new_conf
			SET
				r.confidence        = new_conf,
				r.last_updated      = datetime(),
				v.last_seen         = datetime(),
				v.recognition_count = coalesce(v.recognition_count, 0) + 1
			// Registra evento no grafo para auditoria
			CREATE (e:VoiceEvent {
				id:            randomUUID(),
				cosine_sim:    $cosine,
				residual_err:  $residual,
				confidence:    $confidence,
				confirmed:     $confirmed,
				audio_quality: $quality,
				timestamp:     datetime()
			})
			MERGE (p)-[:VOICE_EVENT]->(e)
			RETURN old_conf, new_conf
		`, map[string]any{
			"speaker_id": event.SpeakerID,
			"delta":      delta,
			"min":        hebbMinConf,
			"max":        hebbMaxConf,
			"cosine":     event.CosineSim,
			"residual":   event.ResidualErr,
			"confidence": event.Confidence,
			"confirmed":  event.Confirmed,
			"quality":    event.AudioQuality,
		})
		if err != nil {
			return nil, err
		}
		if row.Next(ctx) {
			old, _ := row.Record().Get("old_conf")
			new_, _ := row.Record().Get("new_conf")
			return [2]float64{toFloat64(old), toFloat64(new_)}, nil
		}
		return [2]float64{0, 0}, nil
	})

	if err != nil {
		return HebbianDelta{}, fmt.Errorf("HebbianUpdate %s: %w", event.SpeakerID, err)
	}

	confs := result.([2]float64)
	d := HebbianDelta{
		SpeakerID:     event.SpeakerID,
		OldConfidence: confs[0],
		NewConfidence: confs[1],
		Direction:     direction,
	}

	s.log.Info("hebbian update",
		zap.String("speaker", event.SpeakerID),
		zap.String("direction", direction),
		zap.Float64("old", confs[0]),
		zap.Float64("new", confs[1]),
		zap.Float64("delta", delta),
	)

	return d, nil
}

// ─── Helpers de conversão Neo4j → Go ─────────────────────────────────────

func mustGet(r *neo4j.Record, key string) any {
	v, _ := r.Get(key)
	return v
}

func toFloat64Slice(v any) []float64 {
	if v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]float64, len(raw))
	for i, x := range raw {
		out[i] = toFloat64(x)
	}
	return out
}

func toFloat64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	default:
		return 0
	}
}
