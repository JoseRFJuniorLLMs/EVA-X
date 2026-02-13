package workerpool

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// PERFORMANCE FIX: Worker Pool com Semaforo
// Issue: Goroutines ilimitadas podem causar OOM em producao
// Fix: Pool de workers com limite de concorrencia configuravel
// ============================================================================

// Pool gerencia um conjunto limitado de workers para processamento concorrente
type Pool struct {
	maxWorkers   int64
	activeCount  int64
	sem          chan struct{}
	wg           sync.WaitGroup
	metrics      *Metrics
	name         string
}

// Metrics armazena estatisticas do pool
type Metrics struct {
	TotalSubmitted  int64
	TotalCompleted  int64
	TotalRejected   int64
	TotalPanics     int64
	MaxConcurrent   int64
	mu              sync.Mutex
}

// NewPool cria um novo worker pool
// maxWorkers: numero maximo de goroutines simultaneas
// name: identificador para logs
func NewPool(maxWorkers int, name string) *Pool {
	if maxWorkers <= 0 {
		maxWorkers = 10 // default sensato
	}

	return &Pool{
		maxWorkers: int64(maxWorkers),
		sem:        make(chan struct{}, maxWorkers),
		metrics:    &Metrics{},
		name:       name,
	}
}

// Submit submete uma tarefa para execucao
// Bloqueia se o pool estiver cheio (backpressure)
// Retorna error se o contexto for cancelado enquanto espera
func (p *Pool) Submit(ctx context.Context, task func()) error {
	atomic.AddInt64(&p.metrics.TotalSubmitted, 1)

	// Tentar adquirir slot com timeout do contexto
	select {
	case p.sem <- struct{}{}:
		// Slot adquirido
	case <-ctx.Done():
		atomic.AddInt64(&p.metrics.TotalRejected, 1)
		return ctx.Err()
	}

	// Atualizar contador de ativos
	current := atomic.AddInt64(&p.activeCount, 1)
	p.updateMaxConcurrent(current)

	p.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&p.metrics.TotalPanics, 1)
				log.Printf("🚨 [WORKERPOOL:%s] Panic recuperado: %v", p.name, r)
			}

			atomic.AddInt64(&p.activeCount, -1)
			atomic.AddInt64(&p.metrics.TotalCompleted, 1)
			<-p.sem // Liberar slot
			p.wg.Done()
		}()

		task()
	}()

	return nil
}

// SubmitWithTimeout submete tarefa com timeout proprio
func (p *Pool) SubmitWithTimeout(timeout time.Duration, task func()) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.Submit(ctx, task)
}

// TrySubmit tenta submeter sem bloquear
// Retorna false se o pool estiver cheio
func (p *Pool) TrySubmit(task func()) bool {
	select {
	case p.sem <- struct{}{}:
		// Slot adquirido
	default:
		atomic.AddInt64(&p.metrics.TotalRejected, 1)
		return false
	}

	atomic.AddInt64(&p.metrics.TotalSubmitted, 1)
	current := atomic.AddInt64(&p.activeCount, 1)
	p.updateMaxConcurrent(current)

	p.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&p.metrics.TotalPanics, 1)
				log.Printf("🚨 [WORKERPOOL:%s] Panic recuperado: %v", p.name, r)
			}

			atomic.AddInt64(&p.activeCount, -1)
			atomic.AddInt64(&p.metrics.TotalCompleted, 1)
			<-p.sem
			p.wg.Done()
		}()

		task()
	}()

	return true
}

// Wait aguarda todas as tarefas terminarem
func (p *Pool) Wait() {
	p.wg.Wait()
}

// ActiveCount retorna numero de workers ativos
func (p *Pool) ActiveCount() int64 {
	return atomic.LoadInt64(&p.activeCount)
}

// GetMetrics retorna copia das metricas
func (p *Pool) GetMetrics() Metrics {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	return Metrics{
		TotalSubmitted: atomic.LoadInt64(&p.metrics.TotalSubmitted),
		TotalCompleted: atomic.LoadInt64(&p.metrics.TotalCompleted),
		TotalRejected:  atomic.LoadInt64(&p.metrics.TotalRejected),
		TotalPanics:    atomic.LoadInt64(&p.metrics.TotalPanics),
		MaxConcurrent:  atomic.LoadInt64(&p.metrics.MaxConcurrent),
	}
}

func (p *Pool) updateMaxConcurrent(current int64) {
	for {
		max := atomic.LoadInt64(&p.metrics.MaxConcurrent)
		if current <= max {
			return
		}
		if atomic.CompareAndSwapInt64(&p.metrics.MaxConcurrent, max, current) {
			return
		}
	}
}

// ============================================================================
// POOLS GLOBAIS PRE-CONFIGURADOS
// ============================================================================

var (
	// AnalysisPool para processamento de texto/audio (CPU-bound)
	AnalysisPool = NewPool(10, "analysis")

	// IOPool para operacoes de I/O (Redis, Postgres, APIs)
	IOPool = NewPool(20, "io")

	// BackgroundPool para tarefas de baixa prioridade
	BackgroundPool = NewPool(5, "background")
)

// GetPoolStats retorna estatisticas de todos os pools
func GetPoolStats() map[string]Metrics {
	return map[string]Metrics{
		"analysis":   AnalysisPool.GetMetrics(),
		"io":         IOPool.GetMetrics(),
		"background": BackgroundPool.GetMetrics(),
	}
}
