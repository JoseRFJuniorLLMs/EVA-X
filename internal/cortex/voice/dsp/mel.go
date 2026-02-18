// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package dsp

import "math"

// hzToMel converts frequency in Hz to Mel scale.
func hzToMel(hz float64) float64 {
	return 2595.0 * math.Log10(1.0+hz/700.0)
}

// melToHz converts Mel scale value to frequency in Hz.
func melToHz(mel float64) float64 {
	return 700.0 * (math.Pow(10.0, mel/2595.0) - 1.0)
}

// MelFilterbank constructs a Mel-scale triangular filterbank.
//
// Parameters:
//   - nFilters: number of Mel filters (typically 26)
//   - fftSize: FFT size (e.g. 512)
//   - sampleRate: audio sample rate in Hz (e.g. 16000)
//   - lowFreq: lower frequency bound in Hz (default 0)
//   - highFreq: upper frequency bound in Hz (default sampleRate/2)
//
// Returns a matrix [nFilters][fftSize/2+1] of filter weights.
func MelFilterbank(nFilters, fftSize int, sampleRate, lowFreq, highFreq float64) [][]float64 {
	if highFreq <= 0 {
		highFreq = sampleRate / 2
	}

	nBins := fftSize/2 + 1
	lowMel := hzToMel(lowFreq)
	highMel := hzToMel(highFreq)

	// nFilters+2 equally spaced points in Mel scale
	melPoints := make([]float64, nFilters+2)
	step := (highMel - lowMel) / float64(nFilters+1)
	for i := range melPoints {
		melPoints[i] = lowMel + float64(i)*step
	}

	// Convert Mel points to FFT bin indices
	bins := make([]int, nFilters+2)
	for i, mel := range melPoints {
		hz := melToHz(mel)
		bins[i] = int(math.Floor((float64(fftSize) + 1) * hz / sampleRate))
	}

	// Build triangular filters
	filterbank := make([][]float64, nFilters)
	for m := 0; m < nFilters; m++ {
		filterbank[m] = make([]float64, nBins)
		left := bins[m]
		center := bins[m+1]
		right := bins[m+2]

		for k := left; k < center && k < nBins; k++ {
			if center > left {
				filterbank[m][k] = float64(k-left) / float64(center-left)
			}
		}
		for k := center; k <= right && k < nBins; k++ {
			if right > center {
				filterbank[m][k] = float64(right-k) / float64(right-center)
			}
		}
	}

	return filterbank
}
