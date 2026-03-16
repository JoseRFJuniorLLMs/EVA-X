// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consciousness

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// Cognitive Daemons — Phase 4 do Cognitive Operating System (COS)
// ============================================================================
//
// Daemons cognitivos sao processos autonomos que correm em background
// e mantem a homeostase do sistema cognitivo. Inspirados em:
//
//   - Daemons UNIX  — processos background que mantem o sistema
//   - Neuromodulacao — sistemas biologicos que regulam actividade neural
//   - Homeostase     — manter variaveis em ranges optimos
//
// Cada daemon subscreve ao ThoughtBus e reage a eventos relevantes,
// publicando ajustes e alertas quando necessario.
//
// Ciencia: Damasio (1999) Somatic Markers, Sterling (2012) Allostasis
// Engineering: Goroutines com ticker + ThoughtBus pub/sub

// DaemonState estado de um daemon cognitivo
type DaemonState string

const (
	DaemonRunning DaemonState = "running"
	DaemonPaused  DaemonState = "paused"
	DaemonStopped DaemonState = "stopped"
)

// CognitiveDaemon interface que todo daemon deve implementar
type CognitiveDaemon interface {
	Name() string
	Start(ctx context.Context)
	Stop()
	State() DaemonState
	Statistics() map[string]interface{}
}

// DaemonManager gere todos os daemons cognitivos do COS
type DaemonManager struct {
	bus     *ThoughtBus
	daemons []CognitiveDaemon
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewDaemonManager cria o gestor de daemons cognitivos
func NewDaemonManager(bus *ThoughtBus) *DaemonManager {
	return &DaemonManager{
		bus:     bus,
		daemons: make([]CognitiveDaemon, 0),
	}
}

// RegisterDaemon regista um daemon para ser gerido
func (dm *DaemonManager) RegisterDaemon(d CognitiveDaemon) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.daemons = append(dm.daemons, d)
	log.Info().Str("daemon", d.Name()).Msg("[DaemonManager] Daemon registado")
}

// StartAll inicia todos os daemons registados
func (dm *DaemonManager) StartAll(parentCtx context.Context) {
	dm.ctx, dm.cancel = context.WithCancel(parentCtx)

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, d := range dm.daemons {
		go func(daemon CognitiveDaemon) {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Str("daemon", daemon.Name()).
						Interface("panic", r).
						Msg("[DaemonManager] Daemon panic (recuperado)")
				}
			}()
			daemon.Start(dm.ctx)
		}(d)
	}

	log.Info().Int("count", len(dm.daemons)).Msg("[DaemonManager] Todos os daemons iniciados")
}

// StopAll para todos os daemons
func (dm *DaemonManager) StopAll() {
	if dm.cancel != nil {
		dm.cancel()
	}

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, d := range dm.daemons {
		d.Stop()
	}
	log.Info().Msg("[DaemonManager] Todos os daemons parados")
}

// GetStatistics retorna metricas de todos os daemons
func (dm *DaemonManager) GetStatistics() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	stats := make(map[string]interface{})
	for _, d := range dm.daemons {
		stats[d.Name()] = d.Statistics()
	}
	stats["daemon_count"] = len(dm.daemons)
	return stats
}

// ============================================================================
// EnergyGuard — Daemon de Proteccao Energetica
// ============================================================================
// Monitora a energia cognitiva global e ajusta thresholds quando
// o sistema esta sobrecarregado ou subcarregado.
// Inspirado em: Kahneman (2011) "Thinking, Fast and Slow" — effort as resource

type EnergyGuardDaemon struct {
	bus           *ThoughtBus
	workspace     *GlobalWorkspace
	state         DaemonState
	interval      time.Duration
	mu            sync.Mutex

	// Metricas internas
	energyLevel    float64 // 0.0-1.0, nivel de energia cognitiva global
	alertCount     atomic.Int64
	adjustCount    atomic.Int64
	lastCheck      time.Time

	// Limites de homeostase
	lowThreshold   float64 // Abaixo disto = overload
	highThreshold  float64 // Acima disto = idle
}

// NewEnergyGuardDaemon cria o daemon de guarda energetica
func NewEnergyGuardDaemon(bus *ThoughtBus, workspace *GlobalWorkspace) *EnergyGuardDaemon {
	return &EnergyGuardDaemon{
		bus:           bus,
		workspace:     workspace,
		state:         DaemonStopped,
		interval:      10 * time.Second,
		energyLevel:   0.7, // Nivel inicial saudavel
		lowThreshold:  0.2,
		highThreshold: 0.9,
	}
}

func (d *EnergyGuardDaemon) Name() string    { return "energy_guard" }
func (d *EnergyGuardDaemon) State() DaemonState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

func (d *EnergyGuardDaemon) Start(ctx context.Context) {
	d.mu.Lock()
	d.state = DaemonRunning
	d.mu.Unlock()

	// Subscrever ao ThoughtBus para monitorar custos energeticos
	if d.bus != nil {
		d.bus.Subscribe(Global, func(event ThoughtEvent) {
			d.mu.Lock()
			// Cada pensamento consome energia
			d.energyLevel -= event.EnergyCost * 0.01
			if d.energyLevel < 0 {
				d.energyLevel = 0
			}
			d.mu.Unlock()
		})
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.mu.Lock()
			d.state = DaemonStopped
			d.mu.Unlock()
			return
		case <-ticker.C:
			d.checkEnergy()
		}
	}
}

func (d *EnergyGuardDaemon) Stop() {
	d.mu.Lock()
	d.state = DaemonStopped
	d.mu.Unlock()
}

func (d *EnergyGuardDaemon) checkEnergy() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastCheck = time.Now()

	// Recuperacao natural de energia (simulacao de descanso)
	d.energyLevel = math.Min(1.0, d.energyLevel+0.02)

	if d.energyLevel < d.lowThreshold {
		// ALERTA: Energia baixa — aumentar threshold do attention scheduler
		// para ser mais selectivo (processar menos pensamentos)
		d.alertCount.Add(1)
		if d.workspace != nil {
			d.workspace.SetAttentionThreshold(0.7) // Muito selectivo
		}
		if d.bus != nil {
			d.bus.Publish(ThoughtEvent{
				Source:   "energy_guard",
				Type:     Reflection,
				Payload:  map[string]interface{}{"alert": "low_energy", "level": d.energyLevel},
				Salience: 0.9,
			})
		}
		log.Warn().Float64("energy", d.energyLevel).Msg("[EnergyGuard] Energia cognitiva baixa")

	} else if d.energyLevel > d.highThreshold {
		// Energia alta — relaxar threshold para aceitar mais pensamentos
		d.adjustCount.Add(1)
		if d.workspace != nil {
			d.workspace.SetAttentionThreshold(0.2) // Permissivo
		}
	}
}

func (d *EnergyGuardDaemon) Statistics() map[string]interface{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[string]interface{}{
		"state":        string(d.state),
		"energy_level": d.energyLevel,
		"alerts":       d.alertCount.Load(),
		"adjustments":  d.adjustCount.Load(),
		"last_check":   d.lastCheck,
	}
}

// ============================================================================
// SynaptogenesisDaemon — Daemon de Reforco Sinaptico
// ============================================================================
// Fortalece conexoes entre memorias que sao co-activadas frequentemente.
// Ciencia: Hebb (1949) "The Organization of Behavior"

type SynaptogenesisDaemon struct {
	bus           *ThoughtBus
	memoryKernel  *MemoryKernel
	state         DaemonState
	interval      time.Duration
	mu            sync.Mutex

	// Tracking de co-activacao
	coActivations map[string]map[string]int // traceID -> traceID -> count
	coMu          sync.Mutex

	// Metricas
	strengthened atomic.Int64
}

// NewSynaptogenesisDaemon cria o daemon de sinaptogenese
func NewSynaptogenesisDaemon(bus *ThoughtBus, mk *MemoryKernel) *SynaptogenesisDaemon {
	return &SynaptogenesisDaemon{
		bus:           bus,
		memoryKernel:  mk,
		state:         DaemonStopped,
		interval:      60 * time.Second,
		coActivations: make(map[string]map[string]int),
	}
}

func (d *SynaptogenesisDaemon) Name() string    { return "synaptogenesis" }
func (d *SynaptogenesisDaemon) State() DaemonState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

func (d *SynaptogenesisDaemon) Start(ctx context.Context) {
	d.mu.Lock()
	d.state = DaemonRunning
	d.mu.Unlock()

	// Subscrever a eventos de memoria para detectar co-activacao
	if d.bus != nil {
		recentActivations := make([]string, 0, 10)
		var recentMu sync.Mutex

		d.bus.Subscribe(Memory, func(event ThoughtEvent) {
			payload, ok := event.Payload.(map[string]interface{})
			if !ok {
				return
			}
			traceID, _ := payload["trace_id"].(string)
			if traceID == "" {
				return
			}

			recentMu.Lock()
			// Registar co-activacao com memorias recentes
			for _, recentID := range recentActivations {
				if recentID != traceID {
					d.recordCoActivation(recentID, traceID)
				}
			}
			// Manter window de 10 memorias recentes
			recentActivations = append(recentActivations, traceID)
			if len(recentActivations) > 10 {
				recentActivations = recentActivations[1:]
			}
			recentMu.Unlock()
		})
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.mu.Lock()
			d.state = DaemonStopped
			d.mu.Unlock()
			return
		case <-ticker.C:
			d.strengthenConnections()
		}
	}
}

func (d *SynaptogenesisDaemon) Stop() {
	d.mu.Lock()
	d.state = DaemonStopped
	d.mu.Unlock()
}

func (d *SynaptogenesisDaemon) recordCoActivation(id1, id2 string) {
	d.coMu.Lock()
	defer d.coMu.Unlock()

	if d.coActivations[id1] == nil {
		d.coActivations[id1] = make(map[string]int)
	}
	d.coActivations[id1][id2]++
}

func (d *SynaptogenesisDaemon) strengthenConnections() {
	d.coMu.Lock()
	pairs := make([][2]string, 0)
	for id1, coMap := range d.coActivations {
		for id2, count := range coMap {
			if count >= 3 { // Threshold: 3+ co-activacoes
				pairs = append(pairs, [2]string{id1, id2})
			}
		}
	}
	// Reset co-activations para proximo ciclo
	d.coActivations = make(map[string]map[string]int)
	d.coMu.Unlock()

	// Reforcar conexoes no MemoryKernel
	for _, pair := range pairs {
		if d.memoryKernel != nil {
			d.memoryKernel.Associate(pair[0], pair[1], 0.1)
			d.strengthened.Add(1)
		}
	}

	if len(pairs) > 0 {
		log.Debug().Int("pairs", len(pairs)).Msg("[Synaptogenesis] Conexoes reforcadas")
	}
}

func (d *SynaptogenesisDaemon) Statistics() map[string]interface{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[string]interface{}{
		"state":        string(d.state),
		"strengthened": d.strengthened.Load(),
	}
}

// ============================================================================
// EntropyMonitor — Daemon de Deteccao de Caos Cognitivo
// ============================================================================
// Monitora a entropia do stream de pensamentos. Alta entropia = confusao.
// Quando detecta caos, aumenta selectividade e publica alerta.
// Ciencia: Shannon (1948) Information Theory applied to cognition

type EntropyMonitorDaemon struct {
	bus       *ThoughtBus
	state     DaemonState
	interval  time.Duration
	mu        sync.Mutex

	// Tracking de entropia
	recentTypes   []ThoughtType
	maxWindow     int
	entropy       float64
	alertCount    atomic.Int64
}

// NewEntropyMonitorDaemon cria o daemon de monitorizacao de entropia
func NewEntropyMonitorDaemon(bus *ThoughtBus) *EntropyMonitorDaemon {
	return &EntropyMonitorDaemon{
		bus:       bus,
		state:     DaemonStopped,
		interval:  15 * time.Second,
		maxWindow: 100,
	}
}

func (d *EntropyMonitorDaemon) Name() string    { return "entropy_monitor" }
func (d *EntropyMonitorDaemon) State() DaemonState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

func (d *EntropyMonitorDaemon) Start(ctx context.Context) {
	d.mu.Lock()
	d.state = DaemonRunning
	d.mu.Unlock()

	// Subscrever a todos os pensamentos para contar distribuicao de tipos
	if d.bus != nil {
		d.bus.Subscribe(Global, func(event ThoughtEvent) {
			d.mu.Lock()
			d.recentTypes = append(d.recentTypes, event.Type)
			if len(d.recentTypes) > d.maxWindow {
				d.recentTypes = d.recentTypes[1:]
			}
			d.mu.Unlock()
		})
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.mu.Lock()
			d.state = DaemonStopped
			d.mu.Unlock()
			return
		case <-ticker.C:
			d.calculateEntropy()
		}
	}
}

func (d *EntropyMonitorDaemon) Stop() {
	d.mu.Lock()
	d.state = DaemonStopped
	d.mu.Unlock()
}

func (d *EntropyMonitorDaemon) calculateEntropy() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.recentTypes) < 10 {
		return // Nao ha dados suficientes
	}

	// Calcular distribuicao de frequencia
	counts := make(map[ThoughtType]int)
	for _, t := range d.recentTypes {
		counts[t]++
	}

	// Shannon entropy: H = -sum(p * log2(p))
	total := float64(len(d.recentTypes))
	entropy := 0.0
	for _, count := range counts {
		p := float64(count) / total
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	d.entropy = entropy

	// Alta entropia (> 2.3) indica caos cognitivo
	// (max para 6 tipos = log2(6) ~= 2.58)
	if entropy > 2.3 {
		d.alertCount.Add(1)
		if d.bus != nil {
			d.bus.Publish(ThoughtEvent{
				Source:   "entropy_monitor",
				Type:     Reflection,
				Payload:  map[string]interface{}{"alert": "high_entropy", "entropy": entropy},
				Salience: 0.7,
			})
		}
		log.Warn().Float64("entropy", entropy).Msg("[EntropyMonitor] Alta entropia cognitiva detectada")
	}
}

func (d *EntropyMonitorDaemon) Statistics() map[string]interface{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[string]interface{}{
		"state":       string(d.state),
		"entropy":     d.entropy,
		"window_size": len(d.recentTypes),
		"alerts":      d.alertCount.Load(),
	}
}

// ============================================================================
// ConsolidationDaemon — Daemon de Consolidacao de Memoria
// ============================================================================
// Periodicamente trigger consolidacao do MemoryKernel (WM -> EM/SM).
// Funciona como o sistema de consolidacao nocturno do cerebro.

type ConsolidationDaemon struct {
	bus          *ThoughtBus
	memoryKernel *MemoryKernel
	state        DaemonState
	interval     time.Duration
	mu           sync.Mutex

	totalConsolidated atomic.Int64
	cycles            atomic.Int64
}

// NewConsolidationDaemon cria o daemon de consolidacao
func NewConsolidationDaemon(bus *ThoughtBus, mk *MemoryKernel) *ConsolidationDaemon {
	return &ConsolidationDaemon{
		bus:          bus,
		memoryKernel: mk,
		state:        DaemonStopped,
		interval:     5 * time.Minute, // Consolidar a cada 5 minutos
	}
}

func (d *ConsolidationDaemon) Name() string    { return "consolidation" }
func (d *ConsolidationDaemon) State() DaemonState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

func (d *ConsolidationDaemon) Start(ctx context.Context) {
	d.mu.Lock()
	d.state = DaemonRunning
	d.mu.Unlock()

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.mu.Lock()
			d.state = DaemonStopped
			d.mu.Unlock()
			return
		case <-ticker.C:
			d.runConsolidation()
		}
	}
}

func (d *ConsolidationDaemon) Stop() {
	d.mu.Lock()
	d.state = DaemonStopped
	d.mu.Unlock()
}

func (d *ConsolidationDaemon) runConsolidation() {
	if d.memoryKernel == nil {
		return
	}

	d.cycles.Add(1)
	consolidated := d.memoryKernel.Consolidate()
	d.totalConsolidated.Add(int64(consolidated))

	if consolidated > 0 {
		log.Info().
			Int("consolidated", consolidated).
			Int64("total", d.totalConsolidated.Load()).
			Msg("[ConsolidationDaemon] Ciclo de consolidacao completo")
	}
}

func (d *ConsolidationDaemon) Statistics() map[string]interface{} {
	return map[string]interface{}{
		"state":              string(d.state),
		"cycles":             d.cycles.Load(),
		"total_consolidated": d.totalConsolidated.Load(),
	}
}

// ============================================================================
// HubDetectorDaemon — Daemon de Deteccao de Hubs Cognitivos
// ============================================================================
// Identifica memorias/conceitos que funcionam como hubs (muitas conexoes).
// Hubs cognitivos sao importantes para: retrieval, consolidation, evolution.
// Ciencia: Barabasi (1999) Scale-Free Networks

type HubDetectorDaemon struct {
	bus          *ThoughtBus
	memoryKernel *MemoryKernel
	state        DaemonState
	interval     time.Duration
	mu           sync.Mutex

	// Hubs detectados
	currentHubs []string
	hubCount    atomic.Int64
}

// NewHubDetectorDaemon cria o daemon de deteccao de hubs
func NewHubDetectorDaemon(bus *ThoughtBus, mk *MemoryKernel) *HubDetectorDaemon {
	return &HubDetectorDaemon{
		bus:          bus,
		memoryKernel: mk,
		state:        DaemonStopped,
		interval:     2 * time.Minute,
	}
}

func (d *HubDetectorDaemon) Name() string    { return "hub_detector" }
func (d *HubDetectorDaemon) State() DaemonState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

func (d *HubDetectorDaemon) Start(ctx context.Context) {
	d.mu.Lock()
	d.state = DaemonRunning
	d.mu.Unlock()

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.mu.Lock()
			d.state = DaemonStopped
			d.mu.Unlock()
			return
		case <-ticker.C:
			d.detectHubs()
		}
	}
}

func (d *HubDetectorDaemon) Stop() {
	d.mu.Lock()
	d.state = DaemonStopped
	d.mu.Unlock()
}

func (d *HubDetectorDaemon) detectHubs() {
	if d.memoryKernel == nil {
		return
	}

	hubThreshold := 3 // Minimo de associacoes para ser hub
	hubs := d.memoryKernel.FindHubs(hubThreshold)

	d.mu.Lock()
	d.currentHubs = hubs
	d.mu.Unlock()

	if len(hubs) > 0 {
		d.hubCount.Store(int64(len(hubs)))

		// Publicar hubs no ThoughtBus para que outros modulos usem
		if d.bus != nil {
			d.bus.Publish(ThoughtEvent{
				Source:   "hub_detector",
				Type:     Reflection,
				Payload:  map[string]interface{}{"hubs": hubs, "count": len(hubs)},
				Salience: 0.3,
			})
		}
	}
}

func (d *HubDetectorDaemon) Statistics() map[string]interface{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[string]interface{}{
		"state":    string(d.state),
		"hub_count": d.hubCount.Load(),
		"hubs":     d.currentHubs,
	}
}

// NewDefaultDaemons cria todos os daemons com configuracao padrao e regista-os
// Convenience function para main.go
func NewDefaultDaemons(bus *ThoughtBus, workspace *GlobalWorkspace, mk *MemoryKernel) *DaemonManager {
	dm := NewDaemonManager(bus)

	dm.RegisterDaemon(NewEnergyGuardDaemon(bus, workspace))
	dm.RegisterDaemon(NewSynaptogenesisDaemon(bus, mk))
	dm.RegisterDaemon(NewEntropyMonitorDaemon(bus))
	dm.RegisterDaemon(NewConsolidationDaemon(bus, mk))
	dm.RegisterDaemon(NewHubDetectorDaemon(bus, mk))

	return dm
}
