// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !cgo

package speaker

import "fmt"

// SpeakerEmbedder stub for non-CGO builds.
type SpeakerEmbedder struct{}

func NewSpeakerEmbedder(modelPath string) (*SpeakerEmbedder, error) {
	return nil, fmt.Errorf("speaker embedder requires CGO (ONNX runtime)")
}

func (e *SpeakerEmbedder) ExtractEmbedding(pcmData []byte) ([]float32, error) {
	return nil, fmt.Errorf("speaker embedder requires CGO")
}

func (e *SpeakerEmbedder) Close() {}

func CosineSimilarity(a, b []float32) float64 { return 0 }
