// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	godotenv.Load() // Carrega .env do root

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Erro ao carregar config")
	}

	db, err := database.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Erro ao conectar no banco")
	}
	defer db.Close()

	// Lê o arquivo de migração
	migrationPath := filepath.Join("migrations", "003_system_upgrades.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Erro ao ler arquivo de migração")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Info().Msg("Executando migração 003...")

	_, err = db.Conn.ExecContext(ctx, string(content))
	if err != nil {
		log.Fatal().Err(err).Msg("Erro ao executar migração")
	}

	log.Info().Msg("✅ Migração concluída com sucesso!")
}
