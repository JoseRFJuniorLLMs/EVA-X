// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package workers

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"
)

// Worker interface que todos os workers devem implementar
type Worker interface {
	Name() string
	Interval() time.Duration
	Run(ctx context.Context) error
}

// WorkerManager gerencia múltiplos workers
type WorkerManager struct {
	workers  []Worker
	db       *sql.DB
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewWorkerManager cria um novo gerenciador de workers
func NewWorkerManager(db *sql.DB) *WorkerManager {
	return &WorkerManager{
		workers:  []Worker{},
		db:       db,
		stopChan: make(chan struct{}),
	}
}

// RegisterWorker registra um novo worker
func (wm *WorkerManager) RegisterWorker(w Worker) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.workers = append(wm.workers, w)
	log.Printf("✅ Worker '%s' registrado (intervalo: %v)", w.Name(), w.Interval())
}

// Start inicia todos os workers registrados
func (wm *WorkerManager) Start() {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	log.Printf("🚀 Iniciando %d worker(s)...", len(wm.workers))

	for _, worker := range wm.workers {
		wm.wg.Add(1)
		go wm.runWorker(worker)
	}

	log.Println("✅ Todos os workers iniciados")
}

// runWorker executa um worker específico
func (wm *WorkerManager) runWorker(w Worker) {
	defer wm.wg.Done()

	ticker := time.NewTicker(w.Interval())
	defer ticker.Stop()

	log.Printf("🤖 Worker '%s' iniciado (intervalo: %v)", w.Name(), w.Interval())

	// Executar imediatamente na primeira vez
	wm.executeWorker(w)

	for {
		select {
		case <-ticker.C:
			wm.executeWorker(w)

		case <-wm.stopChan:
			log.Printf("🛑 Worker '%s' parado", w.Name())
			return
		}
	}
}

// executeWorker executa um worker com timeout e tratamento de erros
func (wm *WorkerManager) executeWorker(w Worker) {
	// Timeout de 10 minutos para cada execução
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	startTime := time.Now()

	if err := w.Run(ctx); err != nil {
		log.Printf("❌ Erro no worker '%s': %v", w.Name(), err)
	} else {
		duration := time.Since(startTime)
		log.Printf("✅ Worker '%s' executado com sucesso (duração: %v)", w.Name(), duration)
	}
}

// Stop para todos os workers
func (wm *WorkerManager) Stop() {
	log.Println("🛑 Parando todos os workers...")

	close(wm.stopChan)
	wm.wg.Wait()

	log.Println("✅ Todos os workers parados")
}

// GetDB retorna a conexão com o banco de dados
func (wm *WorkerManager) GetDB() *sql.DB {
	return wm.db
}
