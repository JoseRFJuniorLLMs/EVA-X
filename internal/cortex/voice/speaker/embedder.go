// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build cgo

package speaker

import (
	"fmt"
	"math"
	"sync"

	"eva-mind/internal/cortex/voice/dsp"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	// EmbeddingDim is the output dimension of the ECAPA-TDNN model.
	EmbeddingDim = 192

	// NumMelFeatures is the number of Mel filterbank features expected by the model.
	NumMelFeatures = 80

	// MinAudioDuration is the minimum PCM duration in bytes for a reliable embedding.
	// 3 seconds @ 16kHz 16-bit mono = 96000 bytes.
	MinAudioBytes = 96000
)

// SpeakerEmbedder loads an ECAPA-TDNN ONNX model and extracts speaker embeddings.
type SpeakerEmbedder struct {
	session     *ort.DynamicAdvancedSession
	modelPath   string
	mu          sync.RWMutex
	initialized bool
	mfcc        *dsp.MFCCExtractor
}

// NewSpeakerEmbedder creates a new embedder and loads the ONNX model.
// modelPath should point to an ECAPA-TDNN .onnx file.
func NewSpeakerEmbedder(modelPath string) (*SpeakerEmbedder, error) {
	e := &SpeakerEmbedder{modelPath: modelPath}

	// MFCC extractor configured for 80 Mel features (model input)
	cfg := dsp.MFCCConfig{
		SampleRate:   16000,
		FrameSize:    400, // 25ms
		HopSize:      160, // 10ms
		FFTSize:      512,
		NumMelFilter: NumMelFeatures,
		NumMFCC:      NumMelFeatures, // Keep all 80 for the model
		PreEmphCoeff: 0.97,
		LowFreq:      0,
		HighFreq:     8000,
	}
	e.mfcc = dsp.NewMFCCExtractor(cfg)

	if err := e.loadModel(); err != nil {
		return nil, fmt.Errorf("speaker embedder: %w", err)
	}

	return e, nil
}

// loadModel initializes ONNX Runtime and loads the model.
func (e *SpeakerEmbedder) loadModel() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Explicitly set the shared library path to avoid loading stale cached versions.
	ort.SetSharedLibraryPath("/usr/local/lib/libonnxruntime.so.1.24.1")

	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("failed to init ONNX runtime: %w", err)
	}

	options, err := ort.NewSessionOptions()
	if err != nil {
		return fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	if err := options.SetIntraOpNumThreads(2); err != nil {
		return fmt.Errorf("failed to set threads: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(
		e.modelPath,
		[]string{"feats"},  // Wespeaker ECAPA-TDNN input node
		[]string{"embs"},   // Wespeaker ECAPA-TDNN output node
		options,
	)
	if err != nil {
		return fmt.Errorf("failed to create ONNX session: %w", err)
	}

	e.session = session
	e.initialized = true
	return nil
}

// ExtractEmbedding extracts a 192-dim speaker embedding from raw PCM 16-bit LE mono audio.
// The PCM should be at least 3 seconds for reliable results.
func (e *SpeakerEmbedder) ExtractEmbedding(pcmData []byte) ([]float32, error) {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return nil, fmt.Errorf("embedder not initialized")
	}
	e.mu.RUnlock()

	if len(pcmData) < MinAudioBytes {
		return nil, fmt.Errorf("audio too short: %d bytes (min %d)", len(pcmData), MinAudioBytes)
	}

	// Extract MFCCs: [nFrames][80]
	mfccs := e.mfcc.ExtractFromPCM(pcmData)
	if len(mfccs) == 0 {
		return nil, fmt.Errorf("no MFCC frames extracted")
	}

	nFrames := len(mfccs)

	// Flatten to [1, nFrames, 80] for ONNX
	inputData := make([]float32, nFrames*NumMelFeatures)
	for i, frame := range mfccs {
		for j := 0; j < NumMelFeatures && j < len(frame); j++ {
			inputData[i*NumMelFeatures+j] = float32(frame[j])
		}
	}

	inputShape := ort.NewShape(1, int64(nFrames), NumMelFeatures)
	inputTensor, err := ort.NewTensor(inputShape, inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputShape := ort.NewShape(1, EmbeddingDim)
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	err = e.session.Run(
		[]ort.ArbitraryTensor{inputTensor},
		[]ort.ArbitraryTensor{outputTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("ONNX inference failed: %w", err)
	}

	embedding := outputTensor.GetData()

	// L2 normalize
	result := make([]float32, EmbeddingDim)
	copy(result, embedding[:EmbeddingDim])
	l2Normalize(result)

	return result, nil
}

// Close releases ONNX resources.
func (e *SpeakerEmbedder) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.session != nil {
		e.session.Destroy()
		e.session = nil
	}
	e.initialized = false
}

// l2Normalize normalizes a vector to unit length.
func l2Normalize(v []float32) {
	var sumSq float64
	for _, x := range v {
		sumSq += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sumSq))
	if norm < 1e-10 {
		return
	}
	for i := range v {
		v[i] /= norm
	}
}

// CosineSimilarity computes cosine similarity between two L2-normalized vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot
}
