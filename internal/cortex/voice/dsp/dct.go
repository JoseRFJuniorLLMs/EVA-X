// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package dsp

import "math"

// DCT2 computes the type-II Discrete Cosine Transform of the input signal.
// Returns the first nCoeffs coefficients.
// If nCoeffs <= 0 or > len(input), returns all coefficients.
func DCT2(input []float64, nCoeffs int) []float64 {
	N := len(input)
	if N == 0 {
		return nil
	}
	if nCoeffs <= 0 || nCoeffs > N {
		nCoeffs = N
	}

	out := make([]float64, nCoeffs)
	factor := math.Pi / float64(N)

	for k := 0; k < nCoeffs; k++ {
		var sum float64
		for n := 0; n < N; n++ {
			sum += input[n] * math.Cos(factor*float64(k)*(float64(n)+0.5))
		}
		out[k] = sum
	}

	return out
}
