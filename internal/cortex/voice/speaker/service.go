// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package speaker

import (
	"context"
	"sync"
	"time"

	"eva/internal/brainstem/database"
	"eva/internal/brainstem/infrastructure/vector"

	"github.com/rs/zerolog/log"
)

const (
	// bufferThreshold is the minimum bytes before processing (3s @ 16kHz 16-bit mono).
	bufferThreshold = 3 * 16000 * 2 // 96000 bytes

	// maxBufferSize caps the buffer to 10 seconds to prevent memory growth.
	maxBufferSize = 10 * 16000 * 2
)

// SpeakerMessage is sent to the frontend via WebSocket when a speaker is identified.
type SpeakerMessage struct {
	Type       string  `json:"type"`        // always "speaker"
	SpeakerID  int     `json:"speaker_id"`
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	IsNew      bool    `json:"is_new"`
	Emotion    string  `json:"emotion"`
	PitchHz    float64 `json:"pitch_hz"`
	Energy     float64 `json:"energy"`
	StressLevel float64 `json:"stress_level"`
}

// SpeakerCallback is invoked when a speaker is identified or updated.
type SpeakerCallback func(sessionID string, msg SpeakerMessage)

// audioBuffer accumulates PCM data for a session.
type audioBuffer struct {
	data []byte
}

// SpeakerService orchestrates speaker identification and timbre analysis.
type SpeakerService struct {
	embedder *SpeakerEmbedder
	store    *SpeakerStore
	timbre   *TimbreAnalyzer

	mu        sync.Mutex
	buffers   map[string]*audioBuffer
	callbacks map[string]SpeakerCallback
}

// NewSpeakerService creates a new speaker service.
// embedder can be nil if ONNX is not available (timbre-only mode).
func NewSpeakerService(db *database.DB, qdrant *vector.QdrantClient, modelPath string) (*SpeakerService, error) {
	store := NewSpeakerStore(db, qdrant)
	timbre := NewTimbreAnalyzer()

	var embedder *SpeakerEmbedder
	if modelPath != "" {
		var err error
		embedder, err = NewSpeakerEmbedder(modelPath)
		if err != nil {
			log.Warn().Err(err).Msg("Speaker embedder unavailable, running in timbre-only mode")
		}
	}

	// Ensure Qdrant collection exists
	if qdrant != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := store.EnsureQdrantCollection(ctx); err != nil {
			log.Warn().Err(err).Msg("Failed to ensure Qdrant collection for speakers")
		}
	}

	return &SpeakerService{
		embedder:  embedder,
		store:     store,
		timbre:    timbre,
		buffers:   make(map[string]*audioBuffer),
		callbacks: make(map[string]SpeakerCallback),
	}, nil
}

// SetCallback registers a callback for speaker events on a session.
func (s *SpeakerService) SetCallback(sessionID string, cb SpeakerCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbacks[sessionID] = cb
}

// RemoveSession cleans up buffers and callbacks for a session.
func (s *SpeakerService) RemoveSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.buffers, sessionID)
	delete(s.callbacks, sessionID)
}

// ProcessAudioChunk accumulates PCM data and triggers analysis when the buffer is full.
// This method is designed to be called from a goroutine (non-blocking).
func (s *SpeakerService) ProcessAudioChunk(sessionID, cpf string, pcmData []byte) {
	s.mu.Lock()
	buf, ok := s.buffers[sessionID]
	if !ok {
		buf = &audioBuffer{}
		s.buffers[sessionID] = buf
	}

	// Append data (cap at maxBufferSize)
	remaining := maxBufferSize - len(buf.data)
	if remaining <= 0 {
		s.mu.Unlock()
		return
	}
	if len(pcmData) > remaining {
		pcmData = pcmData[:remaining]
	}
	buf.data = append(buf.data, pcmData...)

	if len(buf.data) < bufferThreshold {
		s.mu.Unlock()
		return
	}

	// Extract buffer and reset
	audioData := make([]byte, len(buf.data))
	copy(audioData, buf.data)
	buf.data = buf.data[:0]

	cb := s.callbacks[sessionID]
	s.mu.Unlock()

	// Process in current goroutine (already off the main path)
	s.processBuffer(sessionID, cpf, audioData, cb)
}

// processBuffer runs speaker identification + timbre analysis on accumulated audio.
func (s *SpeakerService) processBuffer(sessionID, cpf string, audioData []byte, cb SpeakerCallback) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Timbre analysis (always runs, fast)
	snapshot := s.timbre.AnalyzeQuick(audioData)

	// 2. Speaker identification (requires ONNX embedder)
	msg := SpeakerMessage{
		Type:        "speaker",
		Emotion:     snapshot.Emotion,
		PitchHz:     snapshot.PitchHz,
		Energy:      snapshot.Energy,
		StressLevel: snapshot.StressLevel,
	}

	if s.embedder != nil {
		embedding, err := s.embedder.ExtractEmbedding(audioData)
		if err != nil {
			log.Warn().Err(err).Str("session", sessionID).Msg("Failed to extract speaker embedding")
		} else {
			profile, similarity, err := s.store.FindSpeaker(ctx, embedding)
			if err != nil {
				log.Warn().Err(err).Str("session", sessionID).Msg("Speaker search failed")
			}

			if profile != nil {
				// Known speaker
				msg.SpeakerID = profile.ID
				msg.Name = profile.Name
				msg.Confidence = similarity
				msg.IsNew = false

				// Update running averages
				durationMs := len(audioData) / (16000 * 2 / 1000)
				go s.store.UpdateProfile(ctx, profile.ID, &VocalFeatures{
					PitchHz:    snapshot.PitchHz,
					Intensity:  snapshot.Energy,
					DurationMs: durationMs,
				})
			} else {
				// New speaker — enroll
				newProfile := &SpeakerProfile{
					Name:         "Desconhecido",
					Relationship: "unknown",
					CPF:          cpf,
				}

				// If CPF provided, try to associate with existing profile
				if cpf != "" {
					existing, _ := s.store.FindByCPF(ctx, cpf)
					if existing != nil {
						// CPF already enrolled, add new embedding to existing profile
						msg.SpeakerID = existing.ID
						msg.Name = existing.Name
						msg.Confidence = 1.0
						msg.IsNew = false

						go s.store.storeEmbeddingQdrant(ctx, existing.ID, embedding)
					} else {
						id, err := s.store.EnrollSpeaker(ctx, newProfile, embedding)
						if err != nil {
							log.Warn().Err(err).Msg("Failed to enroll new speaker")
						} else {
							msg.SpeakerID = id
							msg.Name = newProfile.Name
							msg.Confidence = 1.0
							msg.IsNew = true
						}
					}
				} else {
					id, err := s.store.EnrollSpeaker(ctx, newProfile, embedding)
					if err != nil {
						log.Warn().Err(err).Msg("Failed to enroll new speaker")
					} else {
						msg.SpeakerID = id
						msg.Name = newProfile.Name
						msg.Confidence = 1.0
						msg.IsNew = true
					}
				}
			}

			// Log identification
			go s.store.SaveIdentification(ctx, sessionID, msg.SpeakerID, msg.Confidence,
				msg.Emotion, msg.PitchHz, msg.Energy, msg.StressLevel)
		}
	}

	// 3. Notify frontend
	if cb != nil {
		cb(sessionID, msg)
	}

	log.Debug().
		Str("session", sessionID).
		Int("speaker_id", msg.SpeakerID).
		Str("name", msg.Name).
		Float64("confidence", msg.Confidence).
		Str("emotion", msg.Emotion).
		Float64("pitch", msg.PitchHz).
		Msg("Speaker analysis complete")
}

// Close releases resources.
func (s *SpeakerService) Close() {
	if s.embedder != nil {
		s.embedder.Close()
	}
}
