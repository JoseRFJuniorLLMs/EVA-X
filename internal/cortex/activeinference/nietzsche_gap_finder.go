// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package activeinference

import (
	"context"
	"fmt"
	"math"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// NietzscheGapFinder implements GapFinder using NietzscheDB NQL queries.
//
// It queries the graph for:
//  1. Patients with no medication report in > N hours
//  2. Patients with symptom follow-up gaps
//  3. Patients with large session gaps
//
// Free energy is computed as a decay function: Fe = 1 - exp(-λ * stale_hours)
// where λ controls how fast uncertainty grows with time.
type NietzscheGapFinder struct {
	graph      *nietzscheInfra.GraphAdapter
	collection string
}

// NewNietzscheGapFinder creates a GapFinder backed by NietzscheDB.
func NewNietzscheGapFinder(graph *nietzscheInfra.GraphAdapter, collection string) *NietzscheGapFinder {
	return &NietzscheGapFinder{graph: graph, collection: collection}
}

// FindUncertaintyGaps queries the graph for knowledge gaps.
func (f *NietzscheGapFinder) FindUncertaintyGaps(ctx context.Context) ([]UncertaintyGap, error) {
	var gaps []UncertaintyGap

	// Query 1: Patients with stale medication data (> 24h)
	medGaps, err := f.findMedicationGaps(ctx)
	if err == nil {
		gaps = append(gaps, medGaps...)
	}

	// Query 2: Patients with session gaps (> 48h)
	sessionGaps, err := f.findSessionGaps(ctx)
	if err == nil {
		gaps = append(gaps, sessionGaps...)
	}

	return gaps, nil
}

func (f *NietzscheGapFinder) findMedicationGaps(ctx context.Context) ([]UncertaintyGap, error) {
	// NQL: find patient nodes where last_medication_report > 24 hours ago
	nql := `
		MATCH (p:Patient)
		WHERE p.last_medication_ts < now() - duration(hours: 24)
		  AND p.active = true
		RETURN p
		ORDER BY p.last_medication_ts ASC
		LIMIT 50
	`
	result, err := f.graph.ExecuteNQL(ctx, nql, nil, f.collection)
	if err != nil {
		return nil, fmt.Errorf("medication gap query: %w", err)
	}

	var gaps []UncertaintyGap
	for _, node := range result.Nodes {
		staleHours := computeStaleHours(node.Content)
		freeEnergy := computeFreeEnergy(staleHours, 0.02) // λ=0.02 (decays over ~50h)

		gaps = append(gaps, UncertaintyGap{
			PatientID:      extractInt64(node.Content, "patient_id"),
			PatientNodeID:  node.ID,
			GapType:        "medication_report",
			FreeEnergy:     freeEnergy,
			StalenessHours: staleHours,
			LastDataPoint:  time.Now().Add(-time.Duration(staleHours) * time.Hour),
			Description:    "Paciente sem relatório de medicação",
		})
	}
	return gaps, nil
}

func (f *NietzscheGapFinder) findSessionGaps(ctx context.Context) ([]UncertaintyGap, error) {
	// NQL: find patient nodes with no session in > 48 hours
	nql := `
		MATCH (p:Patient)
		WHERE p.last_session_ts < now() - duration(hours: 48)
		  AND p.active = true
		RETURN p
		ORDER BY p.last_session_ts ASC
		LIMIT 50
	`
	result, err := f.graph.ExecuteNQL(ctx, nql, nil, f.collection)
	if err != nil {
		return nil, fmt.Errorf("session gap query: %w", err)
	}

	var gaps []UncertaintyGap
	for _, node := range result.Nodes {
		staleHours := computeStaleHours(node.Content)
		freeEnergy := computeFreeEnergy(staleHours, 0.015) // slower decay for sessions

		gaps = append(gaps, UncertaintyGap{
			PatientID:      extractInt64(node.Content, "patient_id"),
			PatientNodeID:  node.ID,
			GapType:        "session_gap",
			FreeEnergy:     freeEnergy,
			StalenessHours: staleHours,
			LastDataPoint:  time.Now().Add(-time.Duration(staleHours) * time.Hour),
			Description:    "Paciente sem sessão recente",
		})
	}
	return gaps, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Math helpers
// ──────────────────────────────────────────────────────────────────────────────

// computeFreeEnergy: Fe = 1 - exp(-λ * stale_hours)
// At λ=0.02, Fe reaches 0.65 after ~57 hours, 0.85 after ~95 hours.
func computeFreeEnergy(staleHours, lambda float64) float64 {
	fe := 1 - math.Exp(-lambda*staleHours)
	if fe < 0 {
		fe = 0
	}
	if fe > 1 {
		fe = 1
	}
	return fe
}

func computeStaleHours(props map[string]interface{}) float64 {
	if ts, ok := props["last_session_ts"].(float64); ok {
		return time.Since(time.Unix(int64(ts), 0)).Hours()
	}
	if ts, ok := props["last_medication_ts"].(float64); ok {
		return time.Since(time.Unix(int64(ts), 0)).Hours()
	}
	return 48 // default: 48h if unknown
}

func extractInt64(props map[string]interface{}, key string) int64 {
	if v, ok := props[key].(float64); ok {
		return int64(v)
	}
	if v, ok := props[key].(int64); ok {
		return v
	}
	return 0
}
