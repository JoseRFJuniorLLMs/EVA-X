// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package dsp

import (
	"encoding/binary"
	"math"
	"testing"
)

// generateSineWavePCM generates a PCM 16-bit LE mono sine wave.
func generateSineWavePCM(freqHz, sampleRate float64, durationMs int) []byte {
	nSamples := int(sampleRate * float64(durationMs) / 1000.0)
	pcm := make([]byte, nSamples*2)
	for i := 0; i < nSamples; i++ {
		t := float64(i) / sampleRate
		val := math.Sin(2.0 * math.Pi * freqHz * t)
		sample := int16(val * 32000)
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(sample))
	}
	return pcm
}

func TestPCM16ToFloat64(t *testing.T) {
	pcm := make([]byte, 4)
	binary.LittleEndian.PutUint16(pcm[0:], uint16(int16(16384))) // 0.5
	neg := int16(-16384)
	binary.LittleEndian.PutUint16(pcm[2:], uint16(neg)) // -0.5

	samples := PCM16ToFloat64(pcm)
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}
	if math.Abs(samples[0]-0.5) > 0.001 {
		t.Errorf("sample[0] = %f, expected ~0.5", samples[0])
	}
	if math.Abs(samples[1]+0.5) > 0.001 {
		t.Errorf("sample[1] = %f, expected ~-0.5", samples[1])
	}
}

func TestPreEmphasis(t *testing.T) {
	samples := []float64{1.0, 0.5, 0.3, 0.1}
	out := PreEmphasis(samples, 0.97)
	if len(out) != 4 {
		t.Fatalf("expected 4 samples")
	}
	if out[0] != 1.0 {
		t.Errorf("first sample should be unchanged")
	}
	expected1 := 0.5 - 0.97*1.0
	if math.Abs(out[1]-expected1) > 1e-10 {
		t.Errorf("out[1] = %f, expected %f", out[1], expected1)
	}
}

func TestMelFilterbank(t *testing.T) {
	fb := MelFilterbank(26, 512, 16000, 0, 8000)
	if len(fb) != 26 {
		t.Fatalf("expected 26 filters, got %d", len(fb))
	}
	nBins := 512/2 + 1
	if len(fb[0]) != nBins {
		t.Fatalf("expected %d bins, got %d", nBins, len(fb[0]))
	}

	// Each filter should have non-negative weights
	for m, filter := range fb {
		for k, w := range filter {
			if w < 0 {
				t.Errorf("filter[%d][%d] = %f, expected non-negative", m, k, w)
			}
		}
	}
}

func TestDCT2(t *testing.T) {
	// DCT of a constant signal should have energy only in the first coefficient
	input := []float64{1, 1, 1, 1}
	out := DCT2(input, 4)
	if len(out) != 4 {
		t.Fatalf("expected 4 coefficients")
	}
	if math.Abs(out[0]-4.0) > 1e-10 {
		t.Errorf("DC coefficient = %f, expected 4.0", out[0])
	}
	for i := 1; i < 4; i++ {
		if math.Abs(out[i]) > 1e-10 {
			t.Errorf("out[%d] = %f, expected ~0 for constant signal", i, out[i])
		}
	}
}

func TestMFCCExtraction(t *testing.T) {
	// Generate 1 second of 440 Hz sine wave at 16kHz
	pcm := generateSineWavePCM(440, 16000, 1000)

	cfg := DefaultMFCCConfig()
	extractor := NewMFCCExtractor(cfg)
	mfccs := extractor.ExtractFromPCM(pcm)

	if len(mfccs) == 0 {
		t.Fatal("expected non-empty MFCC frames")
	}

	// 1s @ 16kHz = 16000 samples. Frames: 1 + (16000-400)/160 = 98 frames
	expectedFrames := 1 + (16000-cfg.FrameSize)/cfg.HopSize
	if len(mfccs) != expectedFrames {
		t.Errorf("expected %d frames, got %d", expectedFrames, len(mfccs))
	}

	// Each frame should have 13 MFCCs
	for i, frame := range mfccs {
		if len(frame) != cfg.NumMFCC {
			t.Errorf("frame %d: expected %d MFCCs, got %d", i, cfg.NumMFCC, len(frame))
		}
	}

	// MFCCs should not be all zero
	allZero := true
	for _, v := range mfccs[0] {
		if math.Abs(v) > 1e-10 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("first MFCC frame is all zeros — something is wrong")
	}
}

func TestMeanMFCC(t *testing.T) {
	mfccs := [][]float64{
		{1, 2, 3},
		{3, 4, 5},
	}
	mean := MeanMFCC(mfccs)
	expected := []float64{2, 3, 4}
	for i, v := range mean {
		if math.Abs(v-expected[i]) > 1e-10 {
			t.Errorf("mean[%d] = %f, expected %f", i, v, expected[i])
		}
	}
}

func TestEstimatePitch(t *testing.T) {
	// Generate 100ms of 200 Hz sine at 16kHz
	sampleRate := 16000.0
	freq := 200.0
	nSamples := int(sampleRate * 0.1) // 100ms
	samples := make([]float64, nSamples)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / sampleRate)
	}

	pitch := EstimatePitchAutocorrelation(samples, sampleRate, 50, 500)

	// Should be close to 200 Hz (within 5%)
	if math.Abs(pitch-freq)/freq > 0.05 {
		t.Errorf("estimated pitch = %.1f Hz, expected ~%.1f Hz", pitch, freq)
	}
}

func TestRMS(t *testing.T) {
	samples := []float64{1, -1, 1, -1}
	rms := RMS(samples)
	if math.Abs(rms-1.0) > 1e-10 {
		t.Errorf("RMS = %f, expected 1.0", rms)
	}
}

func BenchmarkMFCCExtraction(b *testing.B) {
	pcm := generateSineWavePCM(440, 16000, 3000) // 3s audio
	cfg := DefaultMFCCConfig()
	ext := NewMFCCExtractor(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ext.ExtractFromPCM(pcm)
	}
}
