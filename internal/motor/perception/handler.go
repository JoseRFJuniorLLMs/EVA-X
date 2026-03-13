// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package perception

import (
	"context"
	"encoding/base64"
	"sync"
	"sync/atomic"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	"github.com/rs/zerolog/log"
)

// PerceptionHandler manages the continuous perception pipeline:
// Camera frames (JPEG) -> Gemini Vision -> SceneAnalysis -> NietzscheDB graph.
//
// It is designed to be wired into the browser_voice_handler's video frame path.
// Frames are submitted via SubmitFrame(); analysis runs asynchronously.
type PerceptionHandler struct {
	engine *PerceptionEngine
	store  *PerceptionStore

	// Active perception state
	active    atomic.Bool
	userID    int64
	sessionID string

	// Frame channel for async processing
	frameChan chan []byte
	stopChan  chan struct{}
	wg        sync.WaitGroup

	// Stats
	framesReceived atomic.Int64
	framesAnalyzed atomic.Int64
	scenesStored   atomic.Int64
	lastScene      atomic.Pointer[SceneAnalysis]
}

// NewPerceptionHandler creates a handler that connects the camera stream
// to the perception engine and NietzscheDB store.
func NewPerceptionHandler(apiKey string, nzClient *nietzscheInfra.Client, minInterval time.Duration) (*PerceptionHandler, error) {
	engine, err := NewPerceptionEngine(apiKey, minInterval)
	if err != nil {
		return nil, err
	}

	store := NewPerceptionStore(nzClient)

	return &PerceptionHandler{
		engine:    engine,
		store:     store,
		frameChan: make(chan []byte, 5), // buffer 5 frames
		stopChan:  make(chan struct{}),
	}, nil
}

// Start begins async perception processing for a session.
// Call this when a voice/video session starts.
func (ph *PerceptionHandler) Start(ctx context.Context, sessionID string, userID int64) error {
	if ph.active.Load() {
		return nil // already running
	}

	// Ensure the collection exists
	if err := ph.store.EnsureCollection(ctx); err != nil {
		log.Error().Err(err).Msg("[PERCEPTION] Failed to ensure collection")
		return err
	}

	ph.sessionID = sessionID
	ph.userID = userID
	ph.active.Store(true)

	ph.wg.Add(1)
	go ph.processLoop(ctx)

	log.Info().
		Str("session", sessionID).
		Int64("user_id", userID).
		Msg("[PERCEPTION] Handler started — EVA has eyes")

	return nil
}

// Stop halts the perception pipeline. Call when session ends.
func (ph *PerceptionHandler) Stop() {
	if !ph.active.Load() {
		return
	}

	ph.active.Store(false)
	close(ph.stopChan)
	ph.wg.Wait()

	// Reset channels for reuse
	ph.stopChan = make(chan struct{})
	ph.frameChan = make(chan []byte, 5)

	log.Info().
		Str("session", ph.sessionID).
		Int64("frames_received", ph.framesReceived.Load()).
		Int64("frames_analyzed", ph.framesAnalyzed.Load()).
		Int64("scenes_stored", ph.scenesStored.Load()).
		Msg("[PERCEPTION] Handler stopped")
}

// SubmitFrame queues a JPEG frame for async perception analysis.
// Non-blocking: drops frames if the buffer is full.
func (ph *PerceptionHandler) SubmitFrame(jpegData []byte) {
	if !ph.active.Load() {
		return
	}

	ph.framesReceived.Add(1)

	// Non-blocking send — drop frame if buffer is full
	select {
	case ph.frameChan <- jpegData:
	default:
		// Frame dropped (processing too slow)
	}
}

// SubmitFrameBase64 queues a base64-encoded JPEG frame.
func (ph *PerceptionHandler) SubmitFrameBase64(b64Data string) {
	jpegData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return
	}
	ph.SubmitFrame(jpegData)
}

// IsActive returns whether perception is currently running.
func (ph *PerceptionHandler) IsActive() bool {
	return ph.active.Load()
}

// GetLastScene returns the most recent scene analysis, or nil.
func (ph *PerceptionHandler) GetLastScene() *SceneAnalysis {
	return ph.lastScene.Load()
}

// GetStats returns perception statistics for the current session.
func (ph *PerceptionHandler) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"active":          ph.active.Load(),
		"session_id":      ph.sessionID,
		"frames_received": ph.framesReceived.Load(),
		"frames_analyzed": ph.framesAnalyzed.Load(),
		"scenes_stored":   ph.scenesStored.Load(),
	}

	if scene := ph.lastScene.Load(); scene != nil {
		stats["last_scene"] = map[string]interface{}{
			"scene_type":   scene.SceneType,
			"objects":      len(scene.Objects),
			"people_count": scene.PeopleCount,
			"activity":     scene.Activity,
			"timestamp":    scene.Timestamp,
		}
	}

	return stats
}

// Close releases all resources.
func (ph *PerceptionHandler) Close() error {
	ph.Stop()
	return ph.engine.Close()
}

// processLoop is the main async loop that processes frames.
func (ph *PerceptionHandler) processLoop(ctx context.Context) {
	defer ph.wg.Done()

	for {
		select {
		case <-ph.stopChan:
			return
		case <-ctx.Done():
			return
		case frame := <-ph.frameChan:
			ph.processFrame(ctx, frame)
		}
	}
}

// processFrame analyzes a single frame and stores the result.
func (ph *PerceptionHandler) processFrame(ctx context.Context, jpegData []byte) {
	// Rate-limited by the engine
	scene, err := ph.engine.AnalyzeFrame(ctx, jpegData)
	if err != nil {
		log.Warn().Err(err).Msg("[PERCEPTION] Frame analysis failed")
		return
	}
	if scene == nil {
		return // rate limited
	}

	ph.framesAnalyzed.Add(1)

	// Deduplicate: skip if same frame hash as last stored scene
	if prev := ph.lastScene.Load(); prev != nil && prev.FrameHash == scene.FrameHash {
		return
	}

	ph.lastScene.Store(scene)

	// Store in NietzscheDB
	_, err = ph.store.StoreScene(ctx, scene, ph.userID)
	if err != nil {
		log.Warn().Err(err).Msg("[PERCEPTION] Failed to store scene")
		return
	}

	ph.scenesStored.Add(1)
}
