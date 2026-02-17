// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration
// +build integration

// NOTE: These tests use mock structs that cannot satisfy the concrete types
// in RetrievalService (e.g., *sql.DB, *EmbeddingService, *vector.QdrantClient).
// They require refactoring to use interfaces before they can compile.
// Run with: go test -tags=integration ./internal/hippocampus/memory/
// TODO: Refactor RetrievalService to accept interfaces for testability.

package memory

import (
	"context"
	"testing"
)

func TestRetrievalService_RetrieveHybridWithFDPN(t *testing.T) {
	t.Skip("Requires interface refactor — mocks cannot satisfy concrete types")
}

func TestRetrievalService_RetrieveHybridWithFDPN_NoFDPN(t *testing.T) {
	t.Skip("Requires interface refactor — mocks cannot satisfy concrete types")
}

func TestFDPNEngine_GetActivatedNodes(t *testing.T) {
	t.Skip("Requires interface refactor — mocks cannot satisfy concrete types")
}

func TestCalculateKeywordActivation(t *testing.T) {
	tests := []struct {
		keyword  string
		expected float64
	}{
		{"a", 0.5},
		{"café", 0.5},
		{"Maria", 0.5},
		{"hospital", 0.7},
		{"aniversário", 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			activation := calculateKeywordActivation(tt.keyword)
			if activation != tt.expected {
				t.Errorf("Activation for '%s': expected %.1f, got %.1f", tt.keyword, tt.expected, activation)
			}
		})
	}
}

func TestFDPNBoost_Integration(t *testing.T) {
	t.Skip("Requires interface refactor — mocks cannot satisfy concrete types")
}

func TestSortByScore(t *testing.T) {
	results := []*SearchResult{
		{Memory: &Memory{ID: 1}, Score: 0.5},
		{Memory: &Memory{ID: 2}, Score: 0.9},
		{Memory: &Memory{ID: 3}, Score: 0.7},
	}

	sortByScore(results)

	if results[0].Score != 0.9 {
		t.Errorf("First result should have highest score (0.9), got %.1f", results[0].Score)
	}
	if results[1].Score != 0.7 {
		t.Errorf("Second result should have score 0.7, got %.1f", results[1].Score)
	}
	if results[2].Score != 0.5 {
		t.Errorf("Third result should have lowest score (0.5), got %.1f", results[2].Score)
	}
}

func TestGetFDPNBoostStats(t *testing.T) {
	retrieval := &RetrievalService{}

	results := []*SearchResult{
		{Memory: &Memory{ID: 1}, Score: 0.8},
		{Memory: &Memory{ID: 2}, Score: 0.6},
		{Memory: &Memory{ID: 3}, Score: 0.5},
	}

	activatedNodes := map[string]float64{
		"memory_1": 0.9,
		"memory_2": 0.5,
	}

	stats := retrieval.GetFDPNBoostStats(results, activatedNodes)

	if stats.TotalMemories != 3 {
		t.Errorf("Total memories: expected 3, got %d", stats.TotalMemories)
	}

	if stats.BoostedMemories != 2 {
		t.Errorf("Boosted memories: expected 2, got %d", stats.BoostedMemories)
	}

	expectedAvg := 0.105
	if stats.AvgBoostFactor < expectedAvg-0.01 || stats.AvgBoostFactor > expectedAvg+0.01 {
		t.Errorf("Avg boost factor: expected ~%.3f, got %.3f", expectedAvg, stats.AvgBoostFactor)
	}

	expectedMax := 0.135
	if stats.MaxBoostFactor < expectedMax-0.01 || stats.MaxBoostFactor > expectedMax+0.01 {
		t.Errorf("Max boost factor: expected ~%.3f, got %.3f", expectedMax, stats.MaxBoostFactor)
	}
}
