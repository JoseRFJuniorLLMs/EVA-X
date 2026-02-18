// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package dsp

import (
	"math"
	"math/cmplx"
)

// MFCCConfig holds configuration for MFCC extraction.
type MFCCConfig struct {
	SampleRate   float64 // Audio sample rate (default 16000)
	FrameSize    int     // Frame length in samples (default 400 = 25ms @ 16kHz)
	HopSize      int     // Hop length in samples (default 160 = 10ms @ 16kHz)
	FFTSize      int     // FFT size (default 512)
	NumMelFilter int     // Number of Mel filters (default 26)
	NumMFCC      int     // Number of MFCC coefficients to keep (default 13)
	PreEmphCoeff float64 // Pre-emphasis coefficient (default 0.97)
	LowFreq      float64 // Lower frequency for Mel filters (default 0)
	HighFreq     float64 // Upper frequency for Mel filters (default sampleRate/2)
}

// DefaultMFCCConfig returns a default configuration for 16kHz audio.
func DefaultMFCCConfig() MFCCConfig {
	return MFCCConfig{
		SampleRate:   16000,
		FrameSize:    400,  // 25ms
		HopSize:      160,  // 10ms
		FFTSize:      512,
		NumMelFilter: 26,
		NumMFCC:      13,
		PreEmphCoeff: 0.97,
		LowFreq:      0,
		HighFreq:     8000,
	}
}

// MFCCExtractor extracts MFCC features from audio.
type MFCCExtractor struct {
	cfg       MFCCConfig
	filterbank [][]float64
	window     []float64
}

// NewMFCCExtractor creates a new extractor with the given config.
func NewMFCCExtractor(cfg MFCCConfig) *MFCCExtractor {
	fb := MelFilterbank(cfg.NumMelFilter, cfg.FFTSize, cfg.SampleRate, cfg.LowFreq, cfg.HighFreq)
	win := hammingWindow(cfg.FrameSize)
	return &MFCCExtractor{cfg: cfg, filterbank: fb, window: win}
}

// hammingWindow generates a Hamming window of length n.
func hammingWindow(n int) []float64 {
	w := make([]float64, n)
	for i := 0; i < n; i++ {
		w[i] = 0.54 - 0.46*math.Cos(2.0*math.Pi*float64(i)/float64(n-1))
	}
	return w
}

// ExtractFromPCM extracts MFCCs from raw PCM 16-bit LE mono bytes.
// Returns a matrix [numFrames][numMFCC].
func (e *MFCCExtractor) ExtractFromPCM(pcm []byte) [][]float64 {
	samples := PCM16ToFloat64(pcm)
	samples = PreEmphasis(samples, e.cfg.PreEmphCoeff)
	return e.Extract(samples)
}

// Extract computes MFCCs from float64 audio samples.
// Returns a matrix [numFrames][numMFCC].
func (e *MFCCExtractor) Extract(samples []float64) [][]float64 {
	frames := e.frame(samples)
	if len(frames) == 0 {
		return nil
	}

	result := make([][]float64, len(frames))
	for i, frame := range frames {
		// Apply window
		windowed := make([]float64, len(frame))
		for j := range frame {
			windowed[j] = frame[j] * e.window[j]
		}

		// FFT
		spectrum := e.powerSpectrum(windowed)

		// Mel filterbank
		melEnergies := e.applyFilterbank(spectrum)

		// Log
		for j := range melEnergies {
			if melEnergies[j] < 1e-10 {
				melEnergies[j] = 1e-10
			}
			melEnergies[j] = math.Log(melEnergies[j])
		}

		// DCT
		result[i] = DCT2(melEnergies, e.cfg.NumMFCC)
	}

	return result
}

// frame splits the signal into overlapping frames.
func (e *MFCCExtractor) frame(samples []float64) [][]float64 {
	if len(samples) < e.cfg.FrameSize {
		return nil
	}

	nFrames := 1 + (len(samples)-e.cfg.FrameSize)/e.cfg.HopSize
	frames := make([][]float64, nFrames)

	for i := 0; i < nFrames; i++ {
		start := i * e.cfg.HopSize
		end := start + e.cfg.FrameSize
		if end > len(samples) {
			break
		}
		f := make([]float64, e.cfg.FrameSize)
		copy(f, samples[start:end])
		frames[i] = f
	}

	return frames
}

// powerSpectrum computes the power spectrum using FFT.
// Uses a simple radix-2 DIT FFT (no external dependency).
func (e *MFCCExtractor) powerSpectrum(frame []float64) []float64 {
	n := e.cfg.FFTSize

	// Zero-pad frame to FFT size
	padded := make([]complex128, n)
	for i := 0; i < len(frame) && i < n; i++ {
		padded[i] = complex(frame[i], 0)
	}

	// FFT in-place
	fft(padded)

	// Power spectrum: |X(k)|^2 / N, only first N/2+1 bins
	nBins := n/2 + 1
	power := make([]float64, nBins)
	scale := 1.0 / float64(n)
	for k := 0; k < nBins; k++ {
		mag := cmplx.Abs(padded[k])
		power[k] = (mag * mag) * scale
	}

	return power
}

// fft performs an in-place radix-2 decimation-in-time FFT.
func fft(x []complex128) {
	n := len(x)
	if n <= 1 {
		return
	}

	// Bit-reverse permutation
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}

	// Cooley-Tukey butterfly
	for size := 2; size <= n; size <<= 1 {
		halfSize := size / 2
		wn := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := 0; k < halfSize; k++ {
				u := x[start+k]
				v := w * x[start+k+halfSize]
				x[start+k] = u + v
				x[start+k+halfSize] = u - v
				w *= wn
			}
		}
	}
}

// applyFilterbank applies the Mel filterbank to a power spectrum.
func (e *MFCCExtractor) applyFilterbank(spectrum []float64) []float64 {
	result := make([]float64, len(e.filterbank))
	for m, filter := range e.filterbank {
		var energy float64
		for k := 0; k < len(filter) && k < len(spectrum); k++ {
			energy += filter[k] * spectrum[k]
		}
		result[m] = energy
	}
	return result
}

// MeanMFCC computes the mean of each MFCC coefficient across all frames.
// Useful for creating a fixed-size "voice hash" from variable-length audio.
func MeanMFCC(mfccs [][]float64) []float64 {
	if len(mfccs) == 0 {
		return nil
	}
	nCoeffs := len(mfccs[0])
	mean := make([]float64, nCoeffs)
	for _, frame := range mfccs {
		for j := 0; j < nCoeffs && j < len(frame); j++ {
			mean[j] += frame[j]
		}
	}
	n := float64(len(mfccs))
	for j := range mean {
		mean[j] /= n
	}
	return mean
}

// StdMFCC computes the standard deviation of each MFCC coefficient across all frames.
func StdMFCC(mfccs [][]float64, mean []float64) []float64 {
	if len(mfccs) == 0 || len(mean) == 0 {
		return nil
	}
	nCoeffs := len(mean)
	variance := make([]float64, nCoeffs)
	for _, frame := range mfccs {
		for j := 0; j < nCoeffs && j < len(frame); j++ {
			diff := frame[j] - mean[j]
			variance[j] += diff * diff
		}
	}
	n := float64(len(mfccs))
	std := make([]float64, nCoeffs)
	for j := range variance {
		std[j] = math.Sqrt(variance[j] / n)
	}
	return std
}
