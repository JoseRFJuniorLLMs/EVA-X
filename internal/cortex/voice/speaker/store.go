// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package speaker

import (
	"context"
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

// SpeakerStore handles persistence of speaker profiles (NietzscheDB) and embeddings (NietzscheDB).
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
	content := map[string]interface{}{
		"name":           profile.Name,
		"relationship":   profile.Relationship,
		"cpf":            profile.CPF,
		"last_seen_at":   time.Now().Format(time.RFC3339),
		"total_sessions": 0,
		"total_audio_seconds": 0,
	}
	if profile.PatientID != nil {
		content["patient_id"] = *profile.PatientID
	}

	id, err := s.db.Insert(ctx, "speaker_profiles", content)
	if err != nil {
		return 0, fmt.Errorf("failed to insert speaker profile: %w", err)
	}

	// Store embedding in NietzscheDB vector
	if s.vectorAdapter != nil {
		s.storeEmbeddingVector(ctx, int(id), embedding)
	}

	log.Info().Int("speaker_id", int(id)).Str("name", profile.Name).Msg("Speaker enrolled")
	return int(id), nil
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
	// Read current profile to compute running averages
	current, err := s.db.GetNodeByID(ctx, "speaker_profiles", speakerID)
	if err != nil {
		return fmt.Errorf("failed to get speaker profile %d: %w", speakerID, err)
	}
	if current == nil {
		return fmt.Errorf("speaker profile %d not found", speakerID)
	}

	sessions := database.GetInt64(current, "total_sessions")
	avgPitch := database.GetFloat64(current, "avg_pitch_hz")
	avgRate := database.GetFloat64(current, "avg_speech_rate")
	avgIntensity := database.GetFloat64(current, "avg_intensity")
	totalAudio := database.GetInt64(current, "total_audio_seconds")

	newSessions := sessions + 1
	if sessions == 0 {
		avgPitch = features.PitchHz
		avgRate = features.SpeechRate
		avgIntensity = features.Intensity
	} else {
		avgPitch = (avgPitch*float64(sessions) + features.PitchHz) / float64(newSessions)
		avgRate = (avgRate*float64(sessions) + features.SpeechRate) / float64(newSessions)
		avgIntensity = (avgIntensity*float64(sessions) + features.Intensity) / float64(newSessions)
	}

	return s.db.Update(ctx, "speaker_profiles",
		map[string]interface{}{"id": float64(speakerID)},
		map[string]interface{}{
			"avg_pitch_hz":        avgPitch,
			"avg_speech_rate":     avgRate,
			"avg_intensity":       avgIntensity,
			"total_sessions":      newSessions,
			"total_audio_seconds": totalAudio + int64(features.DurationMs/1000),
			"last_seen_at":        time.Now().Format(time.RFC3339),
		})
}

// GetProfileByID retrieves a speaker profile by ID.
func (s *SpeakerStore) GetProfileByID(ctx context.Context, id int) (*SpeakerProfile, error) {
	m, err := s.db.GetNodeByID(ctx, "speaker_profiles", id)
	if err != nil {
		return nil, fmt.Errorf("speaker profile not found: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("speaker profile %d not found", id)
	}

	return s.profileFromMap(m), nil
}

// FindByCPF finds a speaker profile by CPF.
func (s *SpeakerStore) FindByCPF(ctx context.Context, cpf string) (*SpeakerProfile, error) {
	rows, err := s.db.QueryByLabel(ctx, "speaker_profiles",
		" AND n.cpf = $cpf",
		map[string]interface{}{"cpf": cpf}, 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	return s.profileFromMap(rows[0]), nil
}

// SaveIdentification logs a speaker identification event.
func (s *SpeakerStore) SaveIdentification(ctx context.Context, sessionID string, speakerID int, confidence float64, emotion string, pitchHz, energy, stress float64) error {
	_, err := s.db.Insert(ctx, "speaker_identifications", map[string]interface{}{
		"session_id":   sessionID,
		"speaker_id":   speakerID,
		"confidence":   confidence,
		"timestamp_ms": time.Now().UnixMilli(),
		"emotion":      emotion,
		"pitch_hz":     pitchHz,
		"energy":       energy,
		"stress_level": stress,
	})
	return err
}

// profileFromMap converts a NietzscheDB content map to a SpeakerProfile.
func (s *SpeakerStore) profileFromMap(m map[string]interface{}) *SpeakerProfile {
	p := &SpeakerProfile{
		ID:             int(database.GetInt64(m, "id")),
		Name:           database.GetString(m, "name"),
		Relationship:   database.GetString(m, "relationship"),
		CPF:            database.GetString(m, "cpf"),
		AvgPitchHz:     database.GetFloat64(m, "avg_pitch_hz"),
		AvgSpeechRate:  database.GetFloat64(m, "avg_speech_rate"),
		AvgIntensity:   database.GetFloat64(m, "avg_intensity"),
		TotalSessions:  int(database.GetInt64(m, "total_sessions")),
		TotalAudioSecs: int(database.GetInt64(m, "total_audio_seconds")),
		LastSeenAt:     database.GetTime(m, "last_seen_at"),
		CreatedAt:      database.GetTime(m, "created_at"),
	}

	if pid := database.GetInt64(m, "patient_id"); pid != 0 {
		pidInt := int(pid)
		p.PatientID = &pidInt
	}

	return p
}
