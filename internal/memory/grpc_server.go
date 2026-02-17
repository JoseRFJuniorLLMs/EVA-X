// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"eva-mind/internal/memory/krylov"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// KrylovGRPCServer expoe o KrylovMemoryManager via gRPC na porta 50051
// Arquitetura: FastAPI (porta 8000) --gRPC--> Go KrylovService (porta 50051)
type KrylovGRPCServer struct {
	kmm        *krylov.KrylovMemoryManager
	grpcServer *grpc.Server
	port       int
}

// CompressRequest pedido de compressao
type CompressRequest struct {
	Vector []float64
}

// CompressResponse resposta de compressao
type CompressResponse struct {
	Compressed      []float64
	CompressionTime int64 // microsegundos
}

// ReconstructRequest pedido de reconstrucao
type ReconstructRequest struct {
	Compressed []float64
}

// ReconstructResponse resposta de reconstrucao
type ReconstructResponse struct {
	Reconstructed     []float64
	ReconstructionErr float64
}

// UpdateSubspaceRequest pedido de update
type UpdateSubspaceRequest struct {
	Vector []float64
}

// UpdateSubspaceResponse resposta de update
type UpdateSubspaceResponse struct {
	Accepted         bool
	TotalUpdates     int64
	OrthogonalityErr float64
}

// BatchCompressRequest pedido de compressao em lote
type BatchCompressRequest struct {
	Vectors [][]float64
}

// BatchCompressResponse resposta de compressao em lote
type BatchCompressResponse struct {
	Compressed [][]float64
	TotalTime  int64 // microsegundos
}

// KrylovStats estatisticas do sistema
type KrylovStats struct {
	Dimension          int
	SubspaceSize       int
	WindowSize         int
	QueueFill          int
	TotalUpdates       int64
	OrthogonalityErr   float64
	ReconstructionErr  float64
	CompressionRatio   float64
	MemoryReductionPct float64
	Status             string
	LastUpdate         string
	AvgUpdateTimeUs    int64
}

// NewKrylovGRPCServer cria novo servidor gRPC
func NewKrylovGRPCServer(kmm *krylov.KrylovMemoryManager, port int) *KrylovGRPCServer {
	return &KrylovGRPCServer{
		kmm:  kmm,
		port: port,
	}
}

// Compress comprime um vetor via gRPC
func (s *KrylovGRPCServer) Compress(ctx context.Context, req *CompressRequest) (*CompressResponse, error) {
	if req == nil || len(req.Vector) == 0 {
		return nil, status.Error(codes.InvalidArgument, "vetor vazio")
	}

	start := time.Now()
	compressed, err := s.kmm.CompressVector(req.Vector)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "falha ao comprimir: %v", err)
	}

	return &CompressResponse{
		Compressed:      compressed,
		CompressionTime: time.Since(start).Microseconds(),
	}, nil
}

// Reconstruct reconstroi um vetor comprimido
func (s *KrylovGRPCServer) Reconstruct(ctx context.Context, req *ReconstructRequest) (*ReconstructResponse, error) {
	if req == nil || len(req.Compressed) == 0 {
		return nil, status.Error(codes.InvalidArgument, "vetor comprimido vazio")
	}

	reconstructed, err := s.kmm.ReconstructVector(req.Compressed)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "falha ao reconstruir: %v", err)
	}

	return &ReconstructResponse{
		Reconstructed: reconstructed,
	}, nil
}

// Update adiciona um novo vetor ao subespaco
func (s *KrylovGRPCServer) Update(ctx context.Context, req *UpdateSubspaceRequest) (*UpdateSubspaceResponse, error) {
	if req == nil || len(req.Vector) == 0 {
		return nil, status.Error(codes.InvalidArgument, "vetor vazio")
	}

	err := s.kmm.UpdateSubspace(req.Vector)
	accepted := err == nil

	return &UpdateSubspaceResponse{
		Accepted:         accepted,
		TotalUpdates:     s.kmm.TotalUpdates(),
		OrthogonalityErr: s.kmm.OrthogonalityError(),
	}, nil
}

// BatchCompress comprime multiplos vetores
func (s *KrylovGRPCServer) BatchCompress(ctx context.Context, req *BatchCompressRequest) (*BatchCompressResponse, error) {
	if req == nil || len(req.Vectors) == 0 {
		return nil, status.Error(codes.InvalidArgument, "nenhum vetor fornecido")
	}

	start := time.Now()
	compressed := make([][]float64, len(req.Vectors))

	for i, vec := range req.Vectors {
		c, err := s.kmm.CompressVector(vec)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "falha ao comprimir vetor %d: %v", i, err)
		}
		compressed[i] = c
	}

	return &BatchCompressResponse{
		Compressed: compressed,
		TotalTime:  time.Since(start).Microseconds(),
	}, nil
}

// Statistics retorna estatisticas do sistema
func (s *KrylovGRPCServer) Statistics(ctx context.Context) (*KrylovStats, error) {
	stats := s.kmm.GetStatistics()

	return &KrylovStats{
		Dimension:          stats["dimension"].(int),
		SubspaceSize:       stats["subspace_size"].(int),
		WindowSize:         stats["window_size"].(int),
		QueueFill:          stats["queue_fill"].(int),
		TotalUpdates:       stats["total_updates"].(int64),
		OrthogonalityErr:   stats["orthogonality_error"].(float64),
		ReconstructionErr:  stats["reconstruction_error"].(float64),
		CompressionRatio:   stats["compression_ratio"].(float64),
		MemoryReductionPct: stats["memory_reduction_%"].(float64),
		Status:             stats["status"].(string),
		LastUpdate:         stats["last_update"].(string),
		AvgUpdateTimeUs:    stats["avg_update_time_us"].(int64),
	}, nil
}

// SaveCheckpoint salva estado em disco
func (s *KrylovGRPCServer) SaveCheckpoint(ctx context.Context, filepath string) error {
	return s.kmm.SaveCheckpoint(filepath)
}

// LoadCheckpoint carrega estado de disco
func (s *KrylovGRPCServer) LoadCheckpoint(ctx context.Context, filepath string) error {
	return s.kmm.LoadCheckpoint(filepath)
}

// Start inicia o servidor gRPC
func (s *KrylovGRPCServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("falha ao escutar na porta %d: %w", s.port, err)
	}

	s.grpcServer = grpc.NewServer()

	// Health check padrao do gRPC
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(s.grpcServer, healthServer)
	healthServer.SetServingStatus("krylov.v1.KrylovService", healthpb.HealthCheckResponse_SERVING)

	// Reflection para ferramentas como grpcurl
	reflection.Register(s.grpcServer)

	log.Printf("[gRPC] KrylovService escutando na porta %d", s.port)
	log.Printf("[gRPC] Dimension=%d, SubspaceSize=%d, WindowSize=%d",
		s.kmm.Dimension, s.kmm.K, s.kmm.WindowSize)

	return s.grpcServer.Serve(lis)
}

// Stop para o servidor gRPC graciosamente
func (s *KrylovGRPCServer) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
		log.Printf("[gRPC] KrylovService parado")
	}
}

// StartAsync inicia o servidor gRPC em background
func (s *KrylovGRPCServer) StartAsync() {
	go func() {
		if err := s.Start(); err != nil {
			log.Printf("[gRPC] Erro ao iniciar KrylovService: %v", err)
		}
	}()
}
