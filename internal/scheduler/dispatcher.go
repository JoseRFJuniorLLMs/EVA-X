package scheduler

import (
	"context"
	"time"

	"eva-mind/internal/config"
	"eva-mind/internal/database"

	"github.com/rs/zerolog"
)

func Start(ctx context.Context, db *database.DB, cfg *config.Config, logger zerolog.Logger) {
	ticker := time.NewTicker(time.Duration(cfg.SchedulerInterval) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dispatch(ctx, db, cfg, logger)
		}
	}
}

func dispatch(ctx context.Context, db *database.DB, cfg *config.Config, logger zerolog.Logger) {
	logger.Info().Msg("Checking for pending calls...")

	pending, err := db.GetPendingCalls(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get pending calls")
		return
	}

	if len(pending) == 0 {
		return
	}

	logger.Info().Int("count", len(pending)).Msg("Processing pending calls")

	for _, call := range pending {
		go ProcessJob(ctx, db, cfg, logger, call)
	}
}
