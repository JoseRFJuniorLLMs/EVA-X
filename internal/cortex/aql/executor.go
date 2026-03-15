// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AQL Executor — dispatches AQL Statements to NietzscheDB via gRPC.
// This is the central entry point for all AQL operations in EVA.
//
// Architecture:
//   AQL Statement → Executor.Execute() → lower to gRPC calls → CognitiveResult
//
// Side-effects (energy boost on access, temporal edges) are applied automatically.

package aql

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	nietzsche "nietzsche-sdk"
)

// EmbedFunc generates embeddings for text (text → []float32).
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

// Executor dispatches AQL statements to the NietzscheDB backend.
type Executor struct {
	client            *nietzsche.NietzscheClient
	embedFunc         EmbedFunc
	defaultCollection string
}

// NewExecutor creates a new AQL executor.
func NewExecutor(client *nietzsche.NietzscheClient) *Executor {
	return &Executor{
		client:            client,
		defaultCollection: "eva_mind",
	}
}

// SetEmbedFunc sets the embedding function for semantic search verbs.
func (e *Executor) SetEmbedFunc(f EmbedFunc) {
	e.embedFunc = f
}

// SetDefaultCollection sets the default collection for operations.
func (e *Executor) SetDefaultCollection(c string) {
	e.defaultCollection = c
}

// collection resolves the target collection from the statement or default.
func (e *Executor) collection(s *Statement) string {
	if s.Collection != "" {
		return s.Collection
	}
	return e.defaultCollection
}

// Execute dispatches a single AQL statement and returns a CognitiveResult.
func (e *Executor) Execute(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	if e.client == nil {
		return nil, fmt.Errorf("AQL executor: NietzscheDB client not initialized")
	}

	start := time.Now()

	var result *CognitiveResult
	var err error

	switch stmt.Verb {
	case VerbRecall:
		result, err = e.executeRecall(ctx, stmt)
	case VerbResonate:
		result, err = e.executeResonate(ctx, stmt)
	case VerbReflect:
		result, err = e.executeReflect(ctx, stmt)
	case VerbTrace:
		result, err = e.executeTrace(ctx, stmt)
	case VerbImprint:
		result, err = e.executeImprint(ctx, stmt)
	case VerbAssociate:
		result, err = e.executeAssociate(ctx, stmt)
	case VerbDistill:
		result, err = e.executeDistill(ctx, stmt)
	case VerbFade:
		result, err = e.executeFade(ctx, stmt)
	case VerbDescend:
		result, err = e.executeDescend(ctx, stmt)
	case VerbAscend:
		result, err = e.executeAscend(ctx, stmt)
	case VerbOrbit:
		result, err = e.executeOrbit(ctx, stmt)
	case VerbDream:
		result, err = e.executeDream(ctx, stmt)
	case VerbImagine:
		result, err = e.executeImagine(ctx, stmt)
	default:
		return nil, fmt.Errorf("AQL: unknown verb %q", stmt.Verb)
	}

	if err != nil {
		return nil, err
	}

	// Stamp metadata
	elapsed := time.Since(start).Milliseconds()
	result.Metadata.ExecutionMs = elapsed
	result.Metadata.Backend = "NietzscheDB"
	result.Metadata.Verb = string(stmt.Verb)
	result.Metadata.Count = len(result.Nodes)

	// Compute aggregate stats
	if len(result.Nodes) > 0 {
		var sum float32
		var maxE float32
		for _, n := range result.Nodes {
			sum += n.Energy
			if n.Energy > maxE {
				maxE = n.Energy
			}
		}
		result.Metadata.AvgEnergy = sum / float32(len(result.Nodes))
		result.Metadata.MaxEnergy = maxE
	}

	log.Debug().
		Str("verb", string(stmt.Verb)).
		Str("collection", e.collection(stmt)).
		Int("results", len(result.Nodes)).
		Int64("ms", elapsed).
		Msg("[AQL] executed")

	return result, nil
}

// ExecuteRaw is a convenience for executing raw AQL statement strings.
// Parses "VERB query QUALIFIERS" and dispatches.
func (e *Executor) ExecuteRaw(ctx context.Context, aqlText string) (*CognitiveResult, error) {
	stmt, err := ParseStatement(aqlText)
	if err != nil {
		return nil, fmt.Errorf("AQL parse error: %w", err)
	}
	return e.Execute(ctx, stmt)
}
