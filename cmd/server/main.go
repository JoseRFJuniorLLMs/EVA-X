package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/scheduler"
	"eva-mind/internal/telemetry"
	"eva-mind/internal/twilio"
	"eva-mind/internal/voice"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

func main() {
	// 1. Setup Logger
	// Inicializa Logger
	logger := telemetry.NewLogger("development")
	logger.Info().Msg("Starting EVA-Mind Server")

	// Inicializa Gerenciador de Sessões
	voice.InitSessionManager(logger)

	// 2. Load Config
	godotenv.Load()             // Tenta carregar da raiz (se rodar de EVA-Mind)
	godotenv.Load("../../.env") // Fallback se rodar de cmd/server
	cfg := config.Load()

	// 3. Connect to Database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// 4. Start Alert Service
	alertService := voice.NewAlertService(db, cfg, logger)

	// 5. Start Scheduler in background
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info().Msg("Starting scheduler")
	go scheduler.Start(ctx, db, cfg, logger, alertService)

	// 6. Setup Router
	r := SetupRouter(db, cfg, logger, alertService)

	// 6. Start Server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		logger.Info().Msgf("HTTP Server running on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Wait for termination
	<-ctx.Done()
	logger.Info().Msg("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Server exited")
}

func SetupRouter(db *database.DB, cfg *config.Config, logger zerolog.Logger, alertService *voice.AlertService) *gin.Engine {
	r := gin.Default()

	voiceHandler := voice.NewHandler(db, cfg, logger, alertService)
	twilioHandler := twilio.NewTwimlHandler(cfg)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		if err := db.Health(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "db": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Twilio TwiML endpoint for incoming/outgoing calls
	r.POST("/calls/twiml", func(c *gin.Context) {
		twilioHandler.TwimlHandler(c.Writer, c.Request)
	})

	// Media Stream WebSocket
	r.GET("/calls/stream/:agendamento_id", func(c *gin.Context) {
		voiceHandler.HandleMediaStream(c.Writer, c.Request)
	})

	return r
}
