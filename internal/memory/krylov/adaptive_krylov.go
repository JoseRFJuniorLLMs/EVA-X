// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package krylov

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// AdaptiveKrylov implementa neuroplasticidade: arquitetura que expande e contrai
// Dimensao do subespaco Krylov adapta entre minDim e maxDim baseado em metricas
// Ciencia: Pascual-Leone et al. (2005) - "The plastic human brain cortex"
type AdaptiveKrylov struct {
	krylov       *KrylovMemoryManager
	minDimension int // 32D
	maxDimension int // 256D

	// Metricas para decisao
	recallHistory   []float64 // Historico de recall@10
	latencyHistory  []time.Duration
	memoryPressure  float64
	lastAdaptation  time.Time
	adaptationCount int

	// Configuracao
	expandThreshold   float64       // Pressao > este valor = expandir
	contractThreshold float64       // Pressao < este valor = contrair
	adaptInterval     time.Duration // Intervalo minimo entre adaptacoes
	mu                sync.Mutex
}

// AdaptationEvent evento de adaptacao registrado
type AdaptationEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	OldDimension int       `json:"old_dimension"`
	NewDimension int       `json:"new_dimension"`
	Direction    string    `json:"direction"` // "expand", "contract", "stable"
	Pressure     float64   `json:"pressure"`
	Trigger      string    `json:"trigger"`
}

// NewAdaptiveKrylov cria um Krylov adaptativo
func NewAdaptiveKrylov(dimension int) *AdaptiveKrylov {
	initialK := 64
	return &AdaptiveKrylov{
		krylov:            NewKrylovMemoryManager(dimension, initialK, 1000),
		minDimension:      32,
		maxDimension:      256,
		expandThreshold:   0.8,
		contractThreshold: 0.3,
		adaptInterval:     5 * time.Minute,
		lastAdaptation:    time.Now(),
	}
}

// UpdateSubspace proxy que atualiza e verifica se adaptacao e necessaria
func (ak *AdaptiveKrylov) UpdateSubspace(vector []float64) error {
	err := ak.krylov.UpdateSubspace(vector)
	if err != nil {
		return err
	}

	// Verificar se e hora de adaptar (nao a cada update)
	if time.Since(ak.lastAdaptation) > ak.adaptInterval {
		go ak.AdaptArchitecture()
	}

	return nil
}

// CompressVector proxy para compressao
func (ak *AdaptiveKrylov) CompressVector(vector []float64) ([]float64, error) {
	return ak.krylov.CompressVector(vector)
}

// ReconstructVector proxy para reconstrucao
func (ak *AdaptiveKrylov) ReconstructVector(compressed []float64) ([]float64, error) {
	return ak.krylov.ReconstructVector(compressed)
}

// RecordRecall registra uma metrica de recall para monitoramento
func (ak *AdaptiveKrylov) RecordRecall(recall float64) {
	ak.mu.Lock()
	defer ak.mu.Unlock()

	ak.recallHistory = append(ak.recallHistory, recall)
	if len(ak.recallHistory) > 100 {
		ak.recallHistory = ak.recallHistory[1:]
	}
}

// RecordLatency registra uma metrica de latencia
func (ak *AdaptiveKrylov) RecordLatency(latency time.Duration) {
	ak.mu.Lock()
	defer ak.mu.Unlock()

	ak.latencyHistory = append(ak.latencyHistory, latency)
	if len(ak.latencyHistory) > 100 {
		ak.latencyHistory = ak.latencyHistory[1:]
	}
}

// AdaptArchitecture decide se deve expandir ou contrair o subespaco
func (ak *AdaptiveKrylov) AdaptArchitecture() *AdaptationEvent {
	ak.mu.Lock()
	defer ak.mu.Unlock()

	pressure := ak.measurePressure()
	ak.memoryPressure = pressure

	currentK := ak.krylov.K
	event := &AdaptationEvent{
		Timestamp:    time.Now(),
		OldDimension: currentK,
		Pressure:     pressure,
	}

	if pressure > ak.expandThreshold && currentK < ak.maxDimension {
		// Sistema sobrecarregado -> EXPANDIR
		newK := currentK * 2
		if newK > ak.maxDimension {
			newK = ak.maxDimension
		}
		ak.expandTo(newK)
		event.NewDimension = newK
		event.Direction = "expand"
		event.Trigger = fmt.Sprintf("pressure=%.2f > threshold=%.2f", pressure, ak.expandThreshold)

		log.Printf("[NEUROPLASTICITY] Expansao: %dD -> %dD (pressao=%.2f)",
			currentK, newK, pressure)

	} else if pressure < ak.contractThreshold && currentK > ak.minDimension {
		// Sistema subutilizado -> CONTRAIR (economizar RAM)
		newK := currentK / 2
		if newK < ak.minDimension {
			newK = ak.minDimension
		}
		ak.contractTo(newK)
		event.NewDimension = newK
		event.Direction = "contract"
		event.Trigger = fmt.Sprintf("pressure=%.2f < threshold=%.2f", pressure, ak.contractThreshold)

		log.Printf("[NEUROPLASTICITY] Contracao: %dD -> %dD (pressao=%.2f)",
			currentK, newK, pressure)

	} else {
		event.NewDimension = currentK
		event.Direction = "stable"
		event.Trigger = fmt.Sprintf("pressure=%.2f (dentro dos limites)", pressure)
	}

	ak.lastAdaptation = time.Now()
	ak.adaptationCount++

	return event
}

// measurePressure calcula a "pressao" no sistema (0-1)
// Alta pressao = precisa mais dimensoes; Baixa = pode economizar
func (ak *AdaptiveKrylov) measurePressure() float64 {
	recallPressure := ak.computeRecallPressure()
	latencyPressure := ak.computeLatencyPressure()
	orthPressure := ak.computeOrthogonalityPressure()

	// Peso: recall (40%) + latencia (30%) + ortogonalidade (30%)
	return 0.4*recallPressure + 0.3*latencyPressure + 0.3*orthPressure
}

// computeRecallPressure quanto pior o recall, maior a pressao
func (ak *AdaptiveKrylov) computeRecallPressure() float64 {
	if len(ak.recallHistory) == 0 {
		return 0.5 // Neutro
	}

	avgRecall := 0.0
	for _, r := range ak.recallHistory {
		avgRecall += r
	}
	avgRecall /= float64(len(ak.recallHistory))

	// Recall baixo (< 0.9) = alta pressao
	return math.Max(0, 1.0-avgRecall)
}

// computeLatencyPressure quanto maior a latencia, maior a pressao
func (ak *AdaptiveKrylov) computeLatencyPressure() float64 {
	if len(ak.latencyHistory) == 0 {
		return 0.5
	}

	var totalMs float64
	for _, l := range ak.latencyHistory {
		totalMs += float64(l.Milliseconds())
	}
	avgMs := totalMs / float64(len(ak.latencyHistory))

	// Normalizar: 0ms = 0 pressao, 100ms+ = alta pressao
	return math.Min(avgMs/100.0, 1.0)
}

// computeOrthogonalityPressure degradacao da base = pressao
func (ak *AdaptiveKrylov) computeOrthogonalityPressure() float64 {
	orthError := ak.krylov.OrthogonalityError()
	// Normalizar: 0 = perfeito, 0.1+ = degradado
	return math.Min(orthError*10.0, 1.0)
}

// expandTo expande o subespaco Krylov para uma dimensao maior
func (ak *AdaptiveKrylov) expandTo(newK int) {
	if newK <= ak.krylov.K {
		return
	}

	// Criar novo manager com dimensao expandida
	newManager := NewKrylovMemoryManager(ak.krylov.Dimension, newK, ak.krylov.WindowSize)

	// Copiar dados existentes da base
	ak.krylov.mu.RLock()
	for j := 0; j < ak.krylov.K; j++ {
		col := make([]float64, ak.krylov.Dimension)
		for i := 0; i < ak.krylov.Dimension; i++ {
			col[i] = ak.krylov.Basis.At(i, j)
		}
		newManager.Basis.SetCol(j, col)
	}

	// Copiar queue
	for i := 0; i < ak.krylov.queueSize; i++ {
		idx := (ak.krylov.queueHead + i) % ak.krylov.WindowSize
		if ak.krylov.memoryQueue[idx] != nil {
			copiedVec := make([]float64, len(ak.krylov.memoryQueue[idx]))
			copy(copiedVec, ak.krylov.memoryQueue[idx])
			newManager.memoryQueue[i] = copiedVec
		}
	}
	newManager.queueSize = ak.krylov.queueSize
	newManager.totalUpdates = ak.krylov.totalUpdates
	ak.krylov.mu.RUnlock()

	ak.krylov = newManager
}

// contractTo contrai o subespaco Krylov para uma dimensao menor
func (ak *AdaptiveKrylov) contractTo(newK int) {
	if newK >= ak.krylov.K {
		return
	}

	newManager := NewKrylovMemoryManager(ak.krylov.Dimension, newK, ak.krylov.WindowSize)

	// Copiar apenas as primeiras newK colunas (mais importantes)
	ak.krylov.mu.RLock()
	for j := 0; j < newK; j++ {
		col := make([]float64, ak.krylov.Dimension)
		for i := 0; i < ak.krylov.Dimension; i++ {
			col[i] = ak.krylov.Basis.At(i, j)
		}
		newManager.Basis.SetCol(j, col)
	}

	for i := 0; i < ak.krylov.queueSize; i++ {
		idx := (ak.krylov.queueHead + i) % ak.krylov.WindowSize
		if ak.krylov.memoryQueue[idx] != nil {
			copiedVec := make([]float64, len(ak.krylov.memoryQueue[idx]))
			copy(copiedVec, ak.krylov.memoryQueue[idx])
			newManager.memoryQueue[i] = copiedVec
		}
	}
	newManager.queueSize = ak.krylov.queueSize
	newManager.totalUpdates = ak.krylov.totalUpdates
	ak.krylov.mu.RUnlock()

	ak.krylov = newManager
}

// GetKrylov retorna o KrylovMemoryManager interno
func (ak *AdaptiveKrylov) GetKrylov() *KrylovMemoryManager {
	return ak.krylov
}

// GetStatistics retorna estatisticas de neuroplasticidade
func (ak *AdaptiveKrylov) GetStatistics() map[string]interface{} {
	ak.mu.Lock()
	defer ak.mu.Unlock()

	avgRecall := 0.0
	if len(ak.recallHistory) > 0 {
		for _, r := range ak.recallHistory {
			avgRecall += r
		}
		avgRecall /= float64(len(ak.recallHistory))
	}

	return map[string]interface{}{
		"engine":            "adaptive_neuroplasticity",
		"current_dimension": ak.krylov.K,
		"min_dimension":     ak.minDimension,
		"max_dimension":     ak.maxDimension,
		"memory_pressure":   fmt.Sprintf("%.2f", ak.memoryPressure),
		"avg_recall":        fmt.Sprintf("%.2f%%", avgRecall*100),
		"adaptation_count":  ak.adaptationCount,
		"last_adaptation":   ak.lastAdaptation.Format(time.RFC3339),
		"range":             fmt.Sprintf("%dD ↔ %dD", ak.minDimension, ak.maxDimension),
		"status":            "active",
	}
}
