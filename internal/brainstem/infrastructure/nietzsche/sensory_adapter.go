// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	nietzsche "nietzsche-sdk"

	"eva/internal/brainstem/logger"
)

// ── Sensory Adapter — multi-modal sensory compression via NietzscheDB ────────
//
// SensoryAdapter handles ALL modalities (text, audio, image, fused) through the
// NietzscheDB InsertSensory / GetSensory / Reconstruct / DegradeSensory RPCs.
// This replaces the old audio-only sensory path with a unified interface.

// Modality constants matching NietzscheDB proto values.
const (
	ModalityText  = "text"
	ModalityAudio = "audio"
	ModalityImage = "image"
	ModalityFused = "fused"
)

// SensoryData holds the result of a GetSensory call, providing a unified
// view of sensory metadata regardless of modality.
type SensoryData struct {
	Found                 bool
	NodeID                string
	Modality              string
	Dim                   uint32
	QuantLevel            string  // "f32"|"f16"|"int8"|"pq"|"gone"
	ReconstructionQuality float32
	CompressionRatio      float32
	EncoderVersion        uint32
	ByteSize              uint32
}

// SensoryAdapter provides a unified interface for storing and retrieving
// multi-modal sensory data (text, audio, image, fused) via NietzscheDB.
type SensoryAdapter struct {
	client *Client
}

// NewSensoryAdapter creates a SensoryAdapter backed by the given NietzscheDB client.
func NewSensoryAdapter(client *Client) *SensoryAdapter {
	return &SensoryAdapter{client: client}
}

// StoreTextSensory stores text-modality sensory data for a node.
// textEmbedding is a JSON-encoded []float32 latent vector from the text encoder.
// encoderVersion identifies the encoder version (e.g. "1", "2") for decoder compat.
func (sa *SensoryAdapter) StoreTextSensory(ctx context.Context, collection, nodeID string,
	textEmbedding []byte, encoderVersion string) error {

	log := logger.Nietzsche()

	latent, err := decodeLatentVector(textEmbedding)
	if err != nil {
		log.Error().Err(err).Str("node_id", nodeID).Msg("[SensoryAdapter] failed to decode text latent vector")
		return fmt.Errorf("sensory adapter decode text latent: %w", err)
	}

	encVer, err := parseEncoderVersion(encoderVersion)
	if err != nil {
		return fmt.Errorf("sensory adapter parse encoder version: %w", err)
	}

	err = sa.client.InsertSensory(ctx, nietzsche.InsertSensoryOpts{
		NodeID:         nodeID,
		Modality:       ModalityText,
		Latent:         latent,
		OriginalBytes:  uint32(len(textEmbedding)),
		EncoderVersion: encVer,
		Collection:     collection,
	})
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] StoreTextSensory failed")
		return fmt.Errorf("sensory adapter store text: %w", err)
	}

	log.Debug().
		Str("collection", collection).
		Str("node_id", nodeID).
		Int("latent_dim", len(latent)).
		Msg("[SensoryAdapter] text sensory stored")
	return nil
}

// StoreAudioSensory stores audio-modality sensory data for a node.
// audioFeatures is a JSON-encoded []float32 latent vector from the audio encoder.
// encoderVersion identifies the encoder version for decoder compatibility.
func (sa *SensoryAdapter) StoreAudioSensory(ctx context.Context, collection, nodeID string,
	audioFeatures []byte, encoderVersion string) error {

	log := logger.Nietzsche()

	latent, err := decodeLatentVector(audioFeatures)
	if err != nil {
		log.Error().Err(err).Str("node_id", nodeID).Msg("[SensoryAdapter] failed to decode audio latent vector")
		return fmt.Errorf("sensory adapter decode audio latent: %w", err)
	}

	encVer, err := parseEncoderVersion(encoderVersion)
	if err != nil {
		return fmt.Errorf("sensory adapter parse encoder version: %w", err)
	}

	err = sa.client.InsertSensory(ctx, nietzsche.InsertSensoryOpts{
		NodeID:         nodeID,
		Modality:       ModalityAudio,
		Latent:         latent,
		OriginalBytes:  uint32(len(audioFeatures)),
		EncoderVersion: encVer,
		Collection:     collection,
	})
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] StoreAudioSensory failed")
		return fmt.Errorf("sensory adapter store audio: %w", err)
	}

	log.Debug().
		Str("collection", collection).
		Str("node_id", nodeID).
		Int("latent_dim", len(latent)).
		Msg("[SensoryAdapter] audio sensory stored")
	return nil
}

// StoreImageSensory stores image-modality sensory data for a node.
// imageFeatures is a JSON-encoded []float32 latent vector from the image encoder.
// encoderVersion identifies the encoder version for decoder compatibility.
// originalShape is a JSON string describing the original image dimensions
// (e.g. `{"height":224,"width":224,"channels":3}`).
func (sa *SensoryAdapter) StoreImageSensory(ctx context.Context, collection, nodeID string,
	imageFeatures []byte, encoderVersion, originalShape string) error {

	log := logger.Nietzsche()

	latent, err := decodeLatentVector(imageFeatures)
	if err != nil {
		log.Error().Err(err).Str("node_id", nodeID).Msg("[SensoryAdapter] failed to decode image latent vector")
		return fmt.Errorf("sensory adapter decode image latent: %w", err)
	}

	encVer, err := parseEncoderVersion(encoderVersion)
	if err != nil {
		return fmt.Errorf("sensory adapter parse encoder version: %w", err)
	}

	err = sa.client.InsertSensory(ctx, nietzsche.InsertSensoryOpts{
		NodeID:         nodeID,
		Modality:       ModalityImage,
		Latent:         latent,
		OriginalShape:  []byte(originalShape),
		OriginalBytes:  uint32(len(imageFeatures)),
		EncoderVersion: encVer,
		Collection:     collection,
	})
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] StoreImageSensory failed")
		return fmt.Errorf("sensory adapter store image: %w", err)
	}

	log.Debug().
		Str("collection", collection).
		Str("node_id", nodeID).
		Int("latent_dim", len(latent)).
		Str("original_shape", originalShape).
		Msg("[SensoryAdapter] image sensory stored")
	return nil
}

// StoreFusedSensory stores fused multi-modal sensory data for a node.
// fusedVector is a JSON-encoded []float32 latent vector produced by fusing multiple
// modalities (e.g. text+audio, text+image, or all three).
// encoderVersion identifies the fusion encoder version for decoder compatibility.
func (sa *SensoryAdapter) StoreFusedSensory(ctx context.Context, collection, nodeID string,
	fusedVector []byte, encoderVersion string) error {

	log := logger.Nietzsche()

	latent, err := decodeLatentVector(fusedVector)
	if err != nil {
		log.Error().Err(err).Str("node_id", nodeID).Msg("[SensoryAdapter] failed to decode fused latent vector")
		return fmt.Errorf("sensory adapter decode fused latent: %w", err)
	}

	encVer, err := parseEncoderVersion(encoderVersion)
	if err != nil {
		return fmt.Errorf("sensory adapter parse encoder version: %w", err)
	}

	err = sa.client.InsertSensory(ctx, nietzsche.InsertSensoryOpts{
		NodeID:         nodeID,
		Modality:       ModalityFused,
		Latent:         latent,
		OriginalBytes:  uint32(len(fusedVector)),
		EncoderVersion: encVer,
		Collection:     collection,
	})
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] StoreFusedSensory failed")
		return fmt.Errorf("sensory adapter store fused: %w", err)
	}

	log.Debug().
		Str("collection", collection).
		Str("node_id", nodeID).
		Int("latent_dim", len(latent)).
		Msg("[SensoryAdapter] fused sensory stored")
	return nil
}

// RetrieveSensory returns sensory metadata for a node (without the full latent vector).
// Use ReconstructFull to get the actual latent data for decoder input.
func (sa *SensoryAdapter) RetrieveSensory(ctx context.Context, collection, nodeID string) (*SensoryData, error) {
	log := logger.Nietzsche()

	result, err := sa.client.GetSensory(ctx, nodeID, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] RetrieveSensory failed")
		return nil, fmt.Errorf("sensory adapter retrieve: %w", err)
	}

	if result == nil || !result.Found {
		log.Debug().
			Str("collection", collection).
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] no sensory data found")
		return nil, nil
	}

	return &SensoryData{
		Found:                 result.Found,
		NodeID:                result.NodeID,
		Modality:              result.Modality,
		Dim:                   result.Dim,
		QuantLevel:            result.QuantLevel,
		ReconstructionQuality: result.ReconstructionQuality,
		CompressionRatio:      result.CompressionRatio,
		EncoderVersion:        result.EncoderVersion,
		ByteSize:              result.ByteSize,
	}, nil
}

// ReconstructFull retrieves the full-quality sensory latent vector for a node,
// suitable for feeding into the appropriate decoder. Returns the raw latent
// bytes as a JSON-encoded []float32.
func (sa *SensoryAdapter) ReconstructFull(ctx context.Context, nodeID string) ([]byte, error) {
	log := logger.Nietzsche()

	result, err := sa.client.Reconstruct(ctx, nodeID, "full")
	if err != nil {
		log.Error().Err(err).
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] ReconstructFull failed")
		return nil, fmt.Errorf("sensory adapter reconstruct full: %w", err)
	}

	if result == nil || !result.Found {
		log.Debug().
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] no sensory data to reconstruct")
		return nil, nil
	}

	// Encode the latent vector back to JSON bytes for the decoder
	encoded, err := json.Marshal(result.Latent)
	if err != nil {
		return nil, fmt.Errorf("sensory adapter marshal reconstructed latent: %w", err)
	}

	log.Debug().
		Str("node_id", nodeID).
		Str("modality", result.Modality).
		Float32("quality", result.Quality).
		Int("latent_dim", len(result.Latent)).
		Msg("[SensoryAdapter] full reconstruction completed")
	return encoded, nil
}

// DegradeToNextLevel triggers progressive quantisation degradation on a node's
// sensory data. Each call moves the latent one step down the compression ladder:
// f32 -> f16 -> int8 -> pq -> gone.
func (sa *SensoryAdapter) DegradeToNextLevel(ctx context.Context, collection, nodeID string) error {
	log := logger.Nietzsche()

	err := sa.client.DegradeSensory(ctx, nodeID, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("node_id", nodeID).
			Msg("[SensoryAdapter] DegradeToNextLevel failed")
		return fmt.Errorf("sensory adapter degrade: %w", err)
	}

	log.Debug().
		Str("collection", collection).
		Str("node_id", nodeID).
		Msg("[SensoryAdapter] sensory degraded to next level")
	return nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// decodeLatentVector parses a JSON-encoded []float32 from raw bytes.
func decodeLatentVector(data []byte) ([]float32, error) {
	var latent []float32
	if err := json.Unmarshal(data, &latent); err != nil {
		return nil, fmt.Errorf("decode latent vector: %w", err)
	}
	if len(latent) == 0 {
		return nil, fmt.Errorf("decode latent vector: empty vector")
	}
	return latent, nil
}

// parseEncoderVersion converts a string encoder version to uint32.
// Accepts numeric strings (e.g. "1", "2", "10").
func parseEncoderVersion(version string) (uint32, error) {
	if version == "" {
		return 1, nil // default to version 1
	}
	v, err := strconv.ParseUint(version, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid encoder version %q: %w", version, err)
	}
	return uint32(v), nil
}
