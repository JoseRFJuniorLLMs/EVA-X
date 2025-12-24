package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eva-mind/api"
	"eva-mind/internal/config"
	"eva-mind/internal/database"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

func main() {
	// Carrega .env
	godotenv.Load()

	// Carrega configuração
	cfg := config.Load()

	// Setup logger (simples para agora)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Info().Msg("🚀 Starting EVA-Mind...")

	// Conecta ao banco de dados
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	logger.Info().Msg("✓ Database connected")

	// Setup HTTP router
	router := api.NewRouter(db, cfg, logger)

	// HTTP Server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server em goroutine
	go func() {
		logger.Info().Msgf("🎙️  EVA-Mind listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("🛑 Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("✓ Server stopped")
}
