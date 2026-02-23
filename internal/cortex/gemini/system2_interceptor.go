// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gemini

import (
	"context"
	"log"
	"sync"
	"time"

	"eva/internal/cortex/llm/thinking"
	"eva/internal/cortex/orchestration"
)

// System2Interceptor wraps the ConversationOrchestrator + System2Engine.
// It is called synchronously for each patient text turn before Gemini replies.
//
// If the message qualifies for System 2 reasoning, the interceptor injects the
// synthesised response back into the Gemini session as a "[CONTEXT]" injection,
// so the native-audio model has the full clinical reasoning before generating audio.
//
// The interceptor is optional — if nil, the handler operates in System 1 mode.
type System2Interceptor struct {
	orchestrator *orchestration.ConversationOrchestrator
	engine       *thinking.System2Engine

	// maxLatency caps how long we wait for System 2 before falling back to System 1.
	maxLatency time.Duration

	mu      sync.Mutex
	enabled bool
}

// NewSystem2Interceptor creates a new interceptor.
func NewSystem2Interceptor(
	orch *orchestration.ConversationOrchestrator,
	engine *thinking.System2Engine,
	maxLatency time.Duration,
) *System2Interceptor {
	if maxLatency <= 0 {
		maxLatency = 8 * time.Second
	}
	s := &System2Interceptor{
		orchestrator: orch,
		engine:       engine,
		maxLatency:   maxLatency,
		enabled:      engine != nil,
	}
	if orch != nil && engine != nil {
		orch.SetSystem2Engine(engine)
	}
	return s
}

// Enable / Disable can toggle System 2 at runtime without restart.
func (s *System2Interceptor) Enable()  { s.mu.Lock(); s.enabled = true; s.mu.Unlock() }
func (s *System2Interceptor) Disable() { s.mu.Lock(); s.enabled = false; s.mu.Unlock() }

// InterceptResult holds what the interceptor decided.
type InterceptResult struct {
	System2Used bool
	Synthesis   string  // non-empty if System 2 was triggered
	Score       float64 // complexity score
	LatencyMs   int64
}

// Intercept receives a patient utterance and runs System 2 if appropriate.
//
// patientID       — patient identifier (int64 as string)
// seedNodeID      — patient's NietzscheDB node (for MCTS starting point)
// patientCtx      — clinical context summary from the prontuário
// userTranscript  — what the patient said (transcribed by STT or Gemini)
func (s *System2Interceptor) Intercept(
	ctx context.Context,
	patientID int64,
	seedNodeID string,
	patientCtx string,
	userTranscript string,
) *InterceptResult {
	s.mu.Lock()
	active := s.enabled && s.orchestrator != nil
	s.mu.Unlock()

	assessment := thinking.AssessComplexity(userTranscript)
	result := &InterceptResult{Score: assessment.Score}

	if !active || !assessment.NeedsSystem2 {
		return result
	}

	log.Printf("[SYSTEM2-INTERCEPTOR] Ativando Sistema 2 (score=%.2f) para paciente %d", assessment.Score, patientID)

	// Cap total latency so the patient doesn't wait forever
	tCtx, cancel := context.WithTimeout(ctx, s.maxLatency)
	defer cancel()

	t0 := time.Now()
	turn, err := s.orchestrator.ProcessTurn(tCtx, patientID, seedNodeID, patientCtx, userTranscript)
	result.LatencyMs = time.Since(t0).Milliseconds()

	if err != nil || turn == nil || turn.Response == "" {
		log.Printf("[SYSTEM2-INTERCEPTOR] Sistema 2 falhou ou sem resposta em %dms: %v", result.LatencyMs, err)
		return result
	}

	result.System2Used = true
	result.Synthesis = turn.Response
	log.Printf("[SYSTEM2-INTERCEPTOR] Sistema 2 concluído em %dms (dialética=%v)", result.LatencyMs, turn.Dialectic)
	return result
}
