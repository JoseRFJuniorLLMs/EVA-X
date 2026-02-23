// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package speaker

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	speakerCollection = "speaker_embeddings"
	speakerVectorSize = 192
	matchThreshold   = 0.75
)

// SpeakerProfile represents a known speaker.
type SpeakerProfile struct {
	ID             int       `json:"id"`
	PatientID      *int      `json:"patient_id,omitempty"`
	Name           string    `json:"name"`
	Relationship   string    `json:"relationship"`
	CPF            string    `json:"cpf,omitempty"`
	AvgPitchHz     float64   `json:"avg_pitch_hz"`
	AvgSpeechRate  float64   `json:"avg_speech_rate"`
	AvgIntensity   float64   `json:"avg_intensity"`
	TotalSessions  int       `json:"total_sessions"`
	TotalAudioSecs int       `json:"total_audio_seconds"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// VocalFeatures represents acoustic features for profile update.
type VocalFeatures struct {
	PitchHz    float64
	SpeechRate float64
	Intensity  float64
	Jitter     float64
	Shimmer    float64
	DurationMs int
}

// SpeakerStore handles persistence of speaker profiles (PostgreSQL) and embeddings (NietzscheDB).
type SpeakerStore struct {
	db            *database.DB
	vectorAdapter *nietzscheInfra.VectorAdapter
}

// NewSpeakerStore creates a new store.
func NewSpeakerStore(db *database.DB, vectorAdapter *nietzscheInfra.VectorAdapter) *SpeakerStore {
	return &SpeakerStore{db: db, vectorAdapter: vectorAdapter}
}

// EnsureVectorCollection is a no-op - NietzscheDB handles collection management.
func (s *SpeakerStore) EnsureVectorCollection(ctx context.Context) error {
	return nil
}

// FindSpeaker searches for a matching speaker by embedding similarity via NietzscheDB vector.
// Returns the best matching profile, similarity score, and error.
// Returns nil profile if no match above threshold.
func (s *SpeakerStore) FindSpeaker(ctx context.Context, embedding []float32) (*SpeakerProfile, float64, error) {
	if s.vectorAdapter == nil {
		return nil, 0, fmt.Errorf("vector store not available")
	}

	results, err := s.vectorAdapter.Search(ctx, speakerCollection, embedding, 1, 0)
	if err != nil {
		return nil, 0, fmt.Errorf("vector search failed: %w", err)
	}

	if len(results) == 0 {
		return nil, 0, nil
	}

	score := results[0].Score
	if score < matchThreshold {
		return nil, score, nil
	}

	// Extract speaker_id from payload
	payload := results[0].Payload
	if spkVal, ok := payload["speaker_id"]; ok {
		var intVal int64
		switch v := spkVal.(type) {
		case int64:
			intVal = v
		case float64:
			intVal = int64(v)
		case int:
			intVal = int64(v)
		}
		if intVal > 0 {
			profile, err := s.GetProfileByID(ctx, int(intVal))
			if err == nil {
				return profile, score, nil
			}
		}
	}

	return nil, score, nil
}

// EnrollSpeaker creates a new speaker profile and stores the embedding in NietzscheDB vector.
func (s *SpeakerStore) EnrollSpeaker(ctx context.Context, profile *SpeakerProfile, embedding []float32) (int, error) {
	// Insert profile in PostgreSQL
	query := `
		INSERT INTO speaker_profiles (patient_id, name, relationship, cpf, last_seen_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id
	`
	var patientID interface{}
	if profile.PatientID != nil {
		patientID = *profile.PatientID
	}

	var id int
	err := s.db.Conn.QueryRowContext(ctx, query,
		patientID, profile.Name, profile.Relationship, profile.CPF,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert speaker profile: %w", err)
	}

	// Store embedding in NietzscheDB vector
	if s.vectorAdapter != nil {
		s.storeEmbeddingVector(ctx, id, embedding)
	}

	log.Info().Int("speaker_id", id).Str("name", profile.Name).Msg("Speaker enrolled")
	return id, nil
}

// storeEmbeddingVector stores an embedding in NietzscheDB.
func (s *SpeakerStore) storeEmbeddingVector(ctx context.Context, speakerID int, embedding []float32) {
	pointID := uuid.New().String()

	payload := map[string]interface{}{
		"speaker_id": int64(speakerID),
	}

	err := s.vectorAdapter.Upsert(ctx, speakerCollection, pointID, embedding, payload)
	if err != nil {
		log.Warn().Err(err).Int("speaker_id", speakerID).Msg("Failed to store embedding")
	}
}

// UpdateProfile updates the running average of vocal features for a speaker.
func (s *SpeakerStore) UpdateProfile(ctx context.Context, speakerID int, features *VocalFeatures) error {
	query := `
		UPDATE speaker_profiles SET
			avg_pitch_hz = CASE
				WHEN total_sessions = 0 THEN $2
				ELSE (avg_pitch_hz * total_sessions + $2) / (total_sessions + 1)
			END,
			avg_speech_rate = CASE
				WHEN total_sessions = 0 THEN $3
				ELSE (avg_speech_rate * total_sessions + $3) / (total_sessions + 1)
			END,
			avg_intensity = CASE
				WHEN total_sessions = 0 THEN $4
				ELSE (avg_intensity * total_sessions + $4) / (total_sessions + 1)
			END,
			total_sessions = total_sessions + 1,
			total_audio_seconds = total_audio_seconds + $5,
			last_seen_at = NOW()
		WHERE id = $1
	`
	_, err := s.db.Conn.ExecContext(ctx, query,
		speakerID,
		features.PitchHz,
		features.SpeechRate,
		features.Intensity,
		features.DurationMs/1000,
	)
	return err
}

// GetProfileByID retrieves a speaker profile by ID.
func (s *SpeakerStore) GetProfileByID(ctx context.Context, id int) (*SpeakerProfile, error) {
	query := `
		SELECT id, patient_id, name, relationship, cpf,
			COALESCE(avg_pitch_hz, 0), COALESCE(avg_speech_rate, 0), COALESCE(avg_intensity, 0),
			total_sessions, total_audio_seconds, COALESCE(last_seen_at, created_at), created_at
		FROM speaker_profiles WHERE id = $1
	`

	p := &SpeakerProfile{}
	var patientID sql.NullInt64
	var cpf sql.NullString

	err := s.db.Conn.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &patientID, &p.Name, &p.Relationship, &cpf,
		&p.AvgPitchHz, &p.AvgSpeechRate, &p.AvgIntensity,
		&p.TotalSessions, &p.TotalAudioSecs, &p.LastSeenAt, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("speaker profile not found: %w", err)
	}

	if patientID.Valid {
		pid := int(patientID.Int64)
		p.PatientID = &pid
	}
	if cpf.Valid {
		p.CPF = cpf.String
	}

	return p, nil
}

// FindByCPF finds a speaker profile by CPF.
func (s *SpeakerStore) FindByCPF(ctx context.Context, cpf string) (*SpeakerProfile, error) {
	query := `
		SELECT id, patient_id, name, relationship, cpf,
			COALESCE(avg_pitch_hz, 0), COALESCE(avg_speech_rate, 0), COALESCE(avg_intensity, 0),
			total_sessions, total_audio_seconds, COALESCE(last_seen_at, created_at), created_at
		FROM speaker_profiles WHERE cpf = $1
		ORDER BY total_sessions DESC LIMIT 1
	`

	p := &SpeakerProfile{}
	var patientID sql.NullInt64
	var cpfVal sql.NullString

	err := s.db.Conn.QueryRowContext(ctx, query, cpf).Scan(
		&p.ID, &patientID, &p.Name, &p.Relationship, &cpfVal,
		&p.AvgPitchHz, &p.AvgSpeechRate, &p.AvgIntensity,
		&p.TotalSessions, &p.TotalAudioSecs, &p.LastSeenAt, &p.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if patientID.Valid {
		pid := int(patientID.Int64)
		p.PatientID = &pid
	}
	if cpfVal.Valid {
		p.CPF = cpfVal.String
	}

	return p, nil
}

// SaveIdentification logs a speaker identification event.
func (s *SpeakerStore) SaveIdentification(ctx context.Context, sessionID string, speakerID int, confidence float64, emotion string, pitchHz, energy, stress float64) error {
	query := `
		INSERT INTO speaker_identifications
			(session_id, speaker_id, confidence, timestamp_ms, emotion, pitch_hz, energy, stress_level)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := s.db.Conn.ExecContext(ctx, query,
		sessionID, speakerID, confidence,
		time.Now().UnixMilli(),
		emotion, pitchHz, energy, stress,
	)
	return err
}
