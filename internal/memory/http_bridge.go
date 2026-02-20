// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"eva/internal/memory/krylov"
)

// KrylovHTTPBridge expoe o KrylovMemoryManager via HTTP/JSON
// Bridge temporario ate protoc compilar os stubs gRPC
// Porta: 50052 (HTTP) - complementa a porta 50051 (gRPC)
type KrylovHTTPBridge struct {
	kmm  *krylov.KrylovMemoryManager
	port int
}

// NewKrylovHTTPBridge cria novo bridge HTTP
func NewKrylovHTTPBridge(kmm *krylov.KrylovMemoryManager, port int) *KrylovHTTPBridge {
	return &KrylovHTTPBridge{kmm: kmm, port: port}
}

// compressReq request body para compressao
type compressReq struct {
	Vector []float64 `json:"vector"`
}

// compressResp response body para compressao
type compressResp struct {
	Compressed      []float64 `json:"compressed"`
	CompressionTime int64     `json:"compression_time_us"`
}

// reconstructReq request body para reconstrucao
type reconstructReq struct {
	Compressed []float64 `json:"compressed"`
}

// reconstructResp response body para reconstrucao
type reconstructResp struct {
	Reconstructed []float64 `json:"reconstructed"`
}

// batchReq request body para compressao em lote
type batchReq struct {
	Vectors [][]float64 `json:"vectors"`
}

// batchResp response body para compressao em lote
type batchResp struct {
	Compressed [][]float64 `json:"compressed"`
	TotalTime  int64       `json:"total_time_us"`
}

// updateReq request body para update
type updateReq struct {
	Vector []float64 `json:"vector"`
}

// updateResp response body para update
type updateResp struct {
	Accepted         bool    `json:"accepted"`
	TotalUpdates     int64   `json:"total_updates"`
	OrthogonalityErr float64 `json:"orthogonality_error"`
}

// checkpointReq request body para checkpoint
type checkpointReq struct {
	Filepath string `json:"filepath"`
}

// Start inicia o bridge HTTP
func (b *KrylovHTTPBridge) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/krylov/compress", b.handleCompress)
	mux.HandleFunc("/krylov/reconstruct", b.handleReconstruct)
	mux.HandleFunc("/krylov/batch_compress", b.handleBatchCompress)
	mux.HandleFunc("/krylov/update", b.handleUpdate)
	mux.HandleFunc("/krylov/stats", b.handleStats)
	mux.HandleFunc("/krylov/health", b.handleHealth)
	mux.HandleFunc("/krylov/checkpoint/save", b.handleSaveCheckpoint)
	mux.HandleFunc("/krylov/checkpoint/load", b.handleLoadCheckpoint)

	addr := fmt.Sprintf(":%d", b.port)
	log.Printf("[HTTP] KrylovBridge escutando na porta %d", b.port)
	return http.ListenAndServe(addr, mux)
}

// StartAsync inicia em background
func (b *KrylovHTTPBridge) StartAsync() {
	go func() {
		if err := b.Start(); err != nil {
			log.Printf("[HTTP] Erro no KrylovBridge: %v", err)
		}
	}()
}

func (b *KrylovHTTPBridge) handleCompress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req compressReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	start := time.Now()
	compressed, err := b.kmm.CompressVector(req.Vector)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, compressResp{
		Compressed:      compressed,
		CompressionTime: time.Since(start).Microseconds(),
	})
}

func (b *KrylovHTTPBridge) handleReconstruct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req reconstructReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	reconstructed, err := b.kmm.ReconstructVector(req.Compressed)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, reconstructResp{Reconstructed: reconstructed})
}

func (b *KrylovHTTPBridge) handleBatchCompress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req batchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	start := time.Now()
	compressed := make([][]float64, len(req.Vectors))

	for i, vec := range req.Vectors {
		c, err := b.kmm.CompressVector(vec)
		if err != nil {
			writeError(w, fmt.Sprintf("falha no vetor %d: %v", i, err), http.StatusBadRequest)
			return
		}
		compressed[i] = c
	}

	writeJSON(w, batchResp{
		Compressed: compressed,
		TotalTime:  time.Since(start).Microseconds(),
	})
}

func (b *KrylovHTTPBridge) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req updateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err := b.kmm.UpdateSubspace(req.Vector)

	writeJSON(w, updateResp{
		Accepted:         err == nil,
		TotalUpdates:     b.kmm.TotalUpdates(),
		OrthogonalityErr: b.kmm.OrthogonalityError(),
	})
}

func (b *KrylovHTTPBridge) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}

	stats := b.kmm.GetStatistics()
	writeJSON(w, stats)
}

func (b *KrylovHTTPBridge) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, map[string]interface{}{
		"status":              b.kmm.GetStatistics()["status"],
		"total_updates":       b.kmm.TotalUpdates(),
		"orthogonality_error": b.kmm.OrthogonalityError(),
	})
}

func (b *KrylovHTTPBridge) handleSaveCheckpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req checkpointReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err := b.kmm.SaveCheckpoint(req.Filepath)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "message": "checkpoint saved"})
}

func (b *KrylovHTTPBridge) handleLoadCheckpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req checkpointReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err := b.kmm.LoadCheckpoint(req.Filepath)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "message": "checkpoint loaded"})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
