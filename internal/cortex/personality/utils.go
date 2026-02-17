// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package personality

// Shared utility functions for personality modules

// containsString checks if a slice contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// containsKeyword checks if text contains any of the keywords
func containsKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if len(text) >= len(keyword) && text[:len(keyword)] == keyword {
			return true
		}
	}
	return false
}

// minFloat returns the minimum of two float64 values
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// maxFloat returns the maximum of two float64 values
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// absFloat returns the absolute value of a float64
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// minInt returns the minimum of two int values
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateVariance calculates variance of a float64 slice
func calculateVariance(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	mean := calculateAverage(values)
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	return variance / float64(len(values))
}

// calculateAverage calculates average of a float64 slice
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
