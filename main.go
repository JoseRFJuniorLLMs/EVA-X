// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eva-mind/internal/brainstem/auth"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/brainstem/database"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"eva-mind/internal/brainstem/push"
	"eva-mind/internal/cortex/eva_memory"
	"eva-mind/internal/cortex/gemini"
	"eva-mind/internal/cortex/lacan"
	"eva-mind/internal/cortex/personality"
	"eva-mind/internal/hippocampus/knowledge"
	"eva-mind/internal/hippocampus/memory"
	"eva-mind/internal/scheduler"
	"eva-mind/internal/security"
	"eva-mind/internal/telemetry"
	"eva-mind/internal/voice"

	// Importações necessárias para rotas

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type PCMClient struct {
	CPF     string
	IdosoID int64
}

type SignalingServer struct {
	db                 *database.DB
	cfg                *config.Config
	alertService       *voice.AlertService
	geminiHandler      *gemini.Handler
	voiceHandler       *voice.Handler
	pushService        *push.FirebaseService
	memoryStore        *memory.MemoryStore
	personalityService *personality.PersonalityService
	narrativeShift     *lacan.NarrativeShiftDetector
	vsm                *VideoSessionManager
	evaMemory          *eva_memory.EvaMemory
	wisdomService      *knowledge.WisdomService
}

func main() {
	// 1. Configuração e Logger
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Falha ao carregar configurações")
	}
	logger := telemetry.NewLogger(cfg.Environment)

	// 2. Infraestrutura (DBs)
	db, err := database.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Falha ao conectar PostgreSQL")
	}
	defer db.Close()

	neo4jClient, err := graph.NewNeo4jClient(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("Neo4j indisponível - Funcionalidades FZPN limitadas")
	}

	qdrantClient, err := vector.NewQdrantClient(cfg.QdrantHost, cfg.QdrantPort)
	if err != nil {
		log.Warn().Err(err).Msg("Qdrant indisponível - Memória semântica limitada")
	}

	// 3. Serviços Base
	pushService, err := push.NewFirebaseService(cfg.FirebaseCredentialsPath)
	if err != nil {
		log.Error().Err(err).Msg("Firebase indisponível - Push notifications desabilitadas. Alertas de emergência podem falhar!")
	}
	alertService := voice.NewAlertService(db, cfg, logger)

	// 4. Cortex (Lógica de Negócio e IA)
	geminiHandler := gemini.NewHandler(cfg, db, neo4jClient, qdrantClient)

	// 5. Voice Handler (WebSocket & DSP)
	voiceHandler := voice.NewHandler(db, cfg, logger, alertService, geminiHandler)
	voice.InitSessionManager(logger)

	// 6. Memory & Personality Stores
	var graphStore *memory.GraphStore
	if neo4jClient != nil {
		graphStore = memory.NewGraphStore(neo4jClient, cfg)
		log.Info().Msg("🧠 GraphStore conectado ao Neo4j")
	}
	memoryStore := memory.NewMemoryStore(db.Conn, graphStore, qdrantClient)
	personalitySvc := personality.NewPersonalityService(db.Conn)

	// 6.2 Wisdom Service (busca semantica em colecoes de sabedoria)
	var wisdomSvc *knowledge.WisdomService
	if qdrantClient != nil {
		embedSvc, err := knowledge.NewEmbeddingService(cfg, qdrantClient)
		if err != nil {
			log.Warn().Err(err).Msg("EmbeddingService indisponivel - Wisdom desabilitada")
		} else {
			wisdomSvc = knowledge.NewWisdomService(qdrantClient, embedSvc)
			log.Info().Msg("📖 Wisdom Service inicializado")
		}
	}

	// 6.1 EVA Meta-Cognitive Memory (Neo4j)
	var evaMemSvc *eva_memory.EvaMemory
	if neo4jClient != nil {
		evaMemSvc = eva_memory.New(neo4jClient)
		if err := evaMemSvc.InitSchema(context.Background()); err != nil {
			log.Warn().Err(err).Msg("EVA Memory schema init warning")
		}
		log.Info().Msg("🧠 EVA Meta-Cognitive Memory inicializada")
	} else {
		log.Warn().Msg("⚠️ EVA Memory desabilitada (Neo4j indisponivel)")
	}

	// 7. Cognitive Services
	signifierService := lacan.NewSignifierService(neo4jClient)
	narrativeShiftDetector := lacan.NewNarrativeShiftDetector(neo4jClient, signifierService)
	log.Info().Msg("📊 Narrative Shift Detector initialized")

	// 8. SignalingServer
	server := &SignalingServer{
		db:                 db,
		cfg:                cfg,
		alertService:       alertService,
		geminiHandler:      geminiHandler,
		voiceHandler:       voiceHandler,
		pushService:        pushService,
		memoryStore:        memoryStore,
		personalityService: personalitySvc,
		narrativeShift:     narrativeShiftDetector,
		vsm:                NewVideoSessionManager(),
		evaMemory:          evaMemSvc,
		wisdomService:      wisdomSvc,
	}

	// 9. Router & Servidor HTTP
	router := mux.NewRouter()

	// Middleware
	router.Use(security.CORSMiddleware(security.DefaultCORSConfig()))

	// Rotas WebSocket
	router.HandleFunc("/ws/pcm", server.voiceHandler.HandleMediaStream)
	router.HandleFunc("/ws/browser", server.handleBrowserVoice)
	router.HandleFunc("/ws/eva", server.handleEvaChat)
	router.HandleFunc("/ws/logs", server.handleLogStream)
	// Rota legado para Twilio Media Stream
	router.HandleFunc("/calls/stream/{agendamento_id}", server.voiceHandler.HandleMediaStream)

	// Rotas de Vídeo (Helpers)
	router.HandleFunc("/video/ws", HandleVideoWebSocket(server.vsm))
	router.HandleFunc("/video/create", server.handleCreateVideoSession).Methods("POST")
	router.HandleFunc("/video/candidate", server.handleCreateVideoCandidate).Methods("POST")
	router.HandleFunc("/video/session/{id}", server.handleGetVideoSession).Methods("GET")
	router.HandleFunc("/video/session/{id}/answer", server.handleSaveVideoAnswer).Methods("POST")
	router.HandleFunc("/video/session/{id}/answer/poll", server.handleGetVideoAnswer).Methods("GET")
	router.HandleFunc("/video/candidates/{id}", server.handleGetMobileCandidates).Methods("GET")
	router.HandleFunc("/video/pending", server.handleGetPendingSessions).Methods("GET")

	// API Routes
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Chat (Malaria-Angolar integration)
	api.HandleFunc("/chat", server.handleChat).Methods("POST")

	// Auth
	authHandler := auth.NewHandler(db, cfg)
	api.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	// Mobile API (EVA-Mobile)
	v1 := api.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/idosos/by-cpf/{cpf}", server.handleGetIdosoByCpf).Methods("GET")
	v1.HandleFunc("/idosos/{id}", server.handleGetIdoso).Methods("GET")
	v1.HandleFunc("/idosos/sync-token-by-cpf", server.handleSyncTokenByCpf).Methods("PATCH")

	// 7. Scheduler (Background Jobs)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("CRITICO: Scheduler panic - background jobs parados")
			}
		}()
		scheduler.Start(ctx, db, cfg, log.Logger, alertService, pushService)
	}()

	// 8. Start Server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Info().Msgf("🚀 EVA-Mind V3 rodando na porta %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Erro no servidor HTTP")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("Desligando graciosamente...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("Erro ao desligar servidor")
	}
}
