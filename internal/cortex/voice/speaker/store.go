// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package speaker

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"eva/internal/brainstem/database"
	"eva/internal/brainstem/infrastructure/vector"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/rs/zerolog/log"
)

const (
	qdrantCollection = "speaker_embeddings"
	qdrantVectorSize = 192
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

// SpeakerStore handles persistence of speaker profiles (PostgreSQL) and embeddings (Qdrant).
type SpeakerStore struct {
	db     *database.DB
	qdrant *vector.QdrantClient
}

// NewSpeakerStore creates a new store.
func NewSpeakerStore(db *database.DB, qdrant *vector.QdrantClient) *SpeakerStore {
	return &SpeakerStore{db: db, qdrant: qdrant}
}

// EnsureQdrantCollection creates the Qdrant collection if it doesn't exist.
func (s *SpeakerStore) EnsureQdrantCollection(ctx context.Context) error {
	if s.qdrant == nil {
		return nil
	}
	return s.qdrant.CreateCollection(ctx, qdrantCollection, qdrantVectorSize)
}

// FindSpeaker searches for a matching speaker by embedding similarity via Qdrant.
// Returns the best matching profile, similarity score, and error.
// Returns nil profile if no match above threshold.
func (s *SpeakerStore) FindSpeaker(ctx context.Context, embedding []float32) (*SpeakerProfile, float64, error) {
	if s.qdrant == nil {
		return nil, 0, fmt.Errorf("qdrant not available")
	}

	results, err := s.qdrant.Search(ctx, qdrantCollection, embedding, 1, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("qdrant search failed: %w", err)
	}

	if len(results) == 0 {
		return nil, 0, nil
	}

	score := float64(results[0].Score)
	if score < matchThreshold {
		return nil, score, nil
	}

	// Extract speaker_id from payload
	payload := results[0].Payload
	if spkVal, ok := payload["speaker_id"]; ok {
		if intVal := spkVal.GetIntegerValue(); intVal > 0 {
			profile, err := s.GetProfileByID(ctx, int(intVal))
			if err == nil {
				return profile, score, nil
			}
		}
	}

	return nil, score, nil
}

// EnrollSpeaker creates a new speaker profile and stores the embedding in Qdrant.
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

	// Store embedding in Qdrant
	if s.qdrant != nil {
		s.storeEmbeddingQdrant(ctx, id, embedding)
	}

	log.Info().Int("speaker_id", id).Str("name", profile.Name).Msg("Speaker enrolled")
	return id, nil
}

// storeEmbeddingQdrant stores an embedding in Qdrant.
func (s *SpeakerStore) storeEmbeddingQdrant(ctx context.Context, speakerID int, embedding []float32) {
	pointID := uuid.New().String()
	points := []*qdrant.PointStruct{
		{
			Id: &qdrant.PointId{
				PointIdOptions: &qdrant.PointId_Uuid{Uuid: pointID},
			},
			Vectors: &qdrant.Vectors{
				VectorsOptions: &qdrant.Vectors_Vector{
					Vector: &qdrant.Vector{Data: embedding},
				},
			},
			Payload: map[string]*qdrant.Value{
				"speaker_id": {Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(speakerID)}},
			},
		},
	}

	err := s.qdrant.Upsert(ctx, qdrantCollection, points)
	if err != nil {
		log.Warn().Err(err).Int("speaker_id", speakerID).Msg("Failed to store embedding in Qdrant")
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
