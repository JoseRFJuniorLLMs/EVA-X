// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package activeinference implements the Active Inference engine for EVA.
//
// Rooted in Karl Friston's Free Energy Principle: instead of waiting for data,
// the system actively reduces uncertainty by taking proactive actions.
//
// When the FreeEnergyMonitor detects high epistemic uncertainty about a patient
// (e.g. missing medication report, long gap in session data), it fires "motor
// outputs" — proactive outreach via Twilio/SMS/Telegram — to collect the
// missing information and stabilise its internal model.
package activeinference

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Motor interfaces (implementations live in their respective packages)
// ──────────────────────────────────────────────────────────────────────────────

// MotorOutput represents an action the system can take to reduce uncertainty.
type MotorOutput interface {
	// Execute performs the proactive action.
	Execute(ctx context.Context, action *ActiveAction) error
	// Name returns the output channel name (e.g. "twilio_sms", "telegram").
	Name() string
}

// ──────────────────────────────────────────────────────────────────────────────
// Uncertainty Gap
// ──────────────────────────────────────────────────────────────────────────────

// UncertaintyGap describes a knowledge gap detected in the graph.
type UncertaintyGap struct {
	PatientID      int64
	PatientNodeID  string  // NietzscheDB node for this patient
	GapType        string  // "medication_report", "symptom_followup", "session_gap"
	FreeEnergy     float64 // 0.0=certain, 1.0=maximum uncertainty
	LastDataPoint  time.Time
	StalenessHours float64
	Description    string
}

// ActiveAction is what the system decides to do to close the gap.
type ActiveAction struct {
	PatientID   int64
	ChannelID   string // Twilio phone, Telegram chat ID, etc.
	Message     string
	GapType     string
	Priority    int // 1=low, 2=medium, 3=high, 4=critical
	ScheduledAt time.Time
}

// ──────────────────────────────────────────────────────────────────────────────
// FreeEnergyMonitor
// ──────────────────────────────────────────────────────────────────────────────

// FreeEnergyMonitor scans for knowledge gaps and fires motor outputs to close them.
// It runs as a background ticker, implementing Friston's Active Inference loop:
//
//	Observe → Infer uncertainty (free energy) → Act to minimise it → Repeat
type FreeEnergyMonitor struct {
	motors    []MotorOutput
	gapFinder GapFinder
	ticker    *time.Ticker
	stopCh    chan struct{}

	// Thresholds
	highEnergy float64 // free energy above this triggers action (default: 0.65)
	critEnergy float64 // free energy above this triggers critical alert (default: 0.85)
}

// GapFinder is the interface to NietzscheDB's GapDaemon output.
type GapFinder interface {
	FindUncertaintyGaps(ctx context.Context) ([]UncertaintyGap, error)
}

// MonitorConfig configures the FreeEnergyMonitor.
type MonitorConfig struct {
	PollInterval time.Duration // how often to scan (default: 15m)
	HighEnergy   float64       // action threshold (default: 0.65)
	CritEnergy   float64       // critical threshold (default: 0.85)
}

// NewFreeEnergyMonitor creates a new monitor (does not start it).
func NewFreeEnergyMonitor(finder GapFinder, motors []MotorOutput, cfg MonitorConfig) *FreeEnergyMonitor {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 15 * time.Minute
	}
	if cfg.HighEnergy <= 0 {
		cfg.HighEnergy = 0.65
	}
	if cfg.CritEnergy <= 0 {
		cfg.CritEnergy = 0.85
	}
	return &FreeEnergyMonitor{
		motors:     motors,
		gapFinder:  finder,
		ticker:     time.NewTicker(cfg.PollInterval),
		stopCh:     make(chan struct{}),
		highEnergy: cfg.HighEnergy,
		critEnergy: cfg.CritEnergy,
	}
}

// Start begins the active inference loop in the background.
func (m *FreeEnergyMonitor) Start(ctx context.Context) {
	log.Printf("[ACTIVE-INFERENCE] Monitor iniciado (intervalo=%v, threshold=%.2f)", m.ticker, m.highEnergy)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-m.ticker.C:
				m.scan(ctx)
			}
		}
	}()
}

// Stop gracefully shuts down the monitor.
func (m *FreeEnergyMonitor) Stop() {
	m.ticker.Stop()
	close(m.stopCh)
}

// ScanNow triggers an immediate scan (useful for testing/manual trigger).
func (m *FreeEnergyMonitor) ScanNow(ctx context.Context) {
	m.scan(ctx)
}

// scan is the core Active Inference loop.
func (m *FreeEnergyMonitor) scan(ctx context.Context) {
	gaps, err := m.gapFinder.FindUncertaintyGaps(ctx)
	if err != nil {
		log.Printf("[ACTIVE-INFERENCE] Erro ao buscar gaps: %v", err)
		return
	}

	if len(gaps) == 0 {
		return
	}

	log.Printf("[ACTIVE-INFERENCE] %d gaps de incerteza detectados", len(gaps))

	for _, gap := range gaps {
		if gap.FreeEnergy < m.highEnergy {
			continue // incerteza aceitável, não agir
		}

		action := m.planAction(gap)
		if action == nil {
			continue
		}

		log.Printf("[ACTIVE-INFERENCE] Agindo sobre gap '%s' (paciente %d, energy=%.2f): %s",
			gap.GapType, gap.PatientID, gap.FreeEnergy, action.Message)

		for _, motor := range m.motors {
			if err := motor.Execute(ctx, action); err != nil {
				log.Printf("[ACTIVE-INFERENCE] Motor '%s' falhou: %v", motor.Name(), err)
			} else {
				log.Printf("[ACTIVE-INFERENCE] Motor '%s' executado com sucesso", motor.Name())
			}
		}
	}
}

// planAction decides what proactive message to send based on the gap type and free energy.
func (m *FreeEnergyMonitor) planAction(gap UncertaintyGap) *ActiveAction {
	priority := 2
	if gap.FreeEnergy >= m.critEnergy {
		priority = 4
	} else if gap.FreeEnergy >= 0.75 {
		priority = 3
	}

	var message string
	switch gap.GapType {
	case "medication_report":
		message = fmt.Sprintf(
			"Olá! Sou a EVA, assistente de saúde. Você tomou seus medicamentos hoje? "+
				"Estamos sem informação há %.0f horas e queremos garantir que você está bem. Responda com SIM ou NÃO.",
			gap.StalenessHours)
	case "symptom_followup":
		message = fmt.Sprintf(
			"Olá! A EVA está acompanhando você. Como você está se sentindo hoje? "+
				"Faz %.0f horas desde o nosso último contato.",
			gap.StalenessHours)
	case "session_gap":
		message = fmt.Sprintf(
			"Olá! A EVA sentiu sua falta. Faz %.0f horas desde a nossa última conversa. "+
				"Gostaria de saber como você está! Pode me ligar ou mandar uma mensagem.",
			gap.StalenessHours)
	default:
		message = fmt.Sprintf("Olá! A EVA está verificando como você está. %s", gap.Description)
	}

	return &ActiveAction{
		PatientID:   gap.PatientID,
		Message:     message,
		GapType:     gap.GapType,
		Priority:    priority,
		ScheduledAt: time.Now(),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// LogMotor (Default implementation)
// ──────────────────────────────────────────────────────────────────────────────

// LogMotor implements MotorOutput by simply logging the proactive action.
// Use this for debugging or as a base for real channel implementations.
type LogMotor struct{}

func (m *LogMotor) Execute(ctx context.Context, action *ActiveAction) error {
	log.Printf("🚀 Proactive Action [%s]: %s (Patient %d, Priority %d)",
		action.GapType, action.Message, action.PatientID, action.Priority)
	return nil
}

func (m *LogMotor) Name() string { return "log_motor" }
