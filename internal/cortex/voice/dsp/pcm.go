// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package dsp

import (
	"encoding/binary"
	"math"
)

// PCM16ToFloat64 converts PCM 16-bit little-endian mono bytes to float64 samples normalized to [-1, 1].
func PCM16ToFloat64(pcm []byte) []float64 {
	n := len(pcm) / 2
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		sample := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		out[i] = float64(sample) / 32768.0
	}
	return out
}

// PreEmphasis applies a first-order pre-emphasis filter: y[n] = x[n] - coeff * x[n-1].
// Typical coefficient is 0.97. Boosts high frequencies for better MFCC extraction.
func PreEmphasis(samples []float64, coeff float64) []float64 {
	if len(samples) == 0 {
		return nil
	}
	out := make([]float64, len(samples))
	out[0] = samples[0]
	for i := 1; i < len(samples); i++ {
		out[i] = samples[i] - coeff*samples[i-1]
	}
	return out
}

// RMS computes the root mean square energy of a signal segment.
func RMS(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += s * s
	}
	return math.Sqrt(sum / float64(len(samples)))
}

// EstimatePitchAutocorrelation estimates the fundamental frequency (F0) of a signal using autocorrelation.
// sampleRate is in Hz. Returns 0 if no clear pitch is found.
// Searches for pitch between minHz and maxHz.
func EstimatePitchAutocorrelation(samples []float64, sampleRate float64, minHz, maxHz float64) float64 {
	if len(samples) < 2 || sampleRate <= 0 {
		return 0
	}

	minLag := int(sampleRate / maxHz)
	maxLag := int(sampleRate / minHz)
	if maxLag >= len(samples) {
		maxLag = len(samples) - 1
	}
	if minLag < 1 {
		minLag = 1
	}
	if minLag >= maxLag {
		return 0
	}

	// Normalized autocorrelation
	var r0 float64
	for _, s := range samples {
		r0 += s * s
	}
	if r0 < 1e-10 {
		return 0 // silence
	}

	bestLag := 0
	bestCorr := 0.0

	for lag := minLag; lag <= maxLag; lag++ {
		var num, denom float64
		for i := 0; i < len(samples)-lag; i++ {
			num += samples[i] * samples[i+lag]
			denom += samples[i+lag] * samples[i+lag]
		}
		if denom < 1e-10 {
			continue
		}
		corr := num / math.Sqrt(r0*denom)
		if corr > bestCorr {
			bestCorr = corr
			bestLag = lag
		}
	}

	if bestCorr < 0.3 || bestLag == 0 {
		return 0 // no clear pitch
	}

	return sampleRate / float64(bestLag)
}
