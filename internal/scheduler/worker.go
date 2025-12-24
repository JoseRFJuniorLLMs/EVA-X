package scheduler

import (
	"context"
	"fmt"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/twilio"
	"eva-mind/pkg/models"

	"github.com/rs/zerolog"
)

// Background worker for processing jobs
func ProcessJob(ctx context.Context, db *database.DB, cfg *config.Config, logger zerolog.Logger, ag models.Agendamento) {
	l := logger.With().Int("agendamento_id", ag.ID).Logger()
	l.Info().Msg("Starting call process")

	// 1. Update status to in_progress and increment attempts
	err := db.UpdateCallStatus(ctx, ag.ID, "in_progress", nil)
	if err != nil {
		l.Error().Err(err).Msg("Failed to update status to in_progress")
		return
	}
	db.IncrementAttempts(ctx, ag.ID)

	// 2. Prepare Twilio client
	tClient := twilio.NewClient(cfg)

	// 3. Generate TwiML URL
	// The URL should point to our TwiML handler: /calls/twiml?agendamento_id=...
	twimlURL := fmt.Sprintf("https://%s/calls/twiml?agendamento_id=%d", cfg.ServiceDomain, ag.ID)

	// 4. Create Call
	sid, err := tClient.CreateCall(ag.Telefone, twimlURL)
	if err != nil {
		l.Error().Err(err).Msg("Failed to create Twilio call")
		db.UpdateCallStatus(ctx, ag.ID, "falhou", nil)
		return
	}

	l.Info().Str("call_sid", sid).Msg("Twilio call created successfully")

	// 5. Update DB with SID and status
	status := "realizada" // or "calling"? Let's use "realizada" for now as it was triggered
	err = db.UpdateCallStatus(ctx, ag.ID, status, &sid)
	if err != nil {
		l.Error().Err(err).Msg("Failed to update status with SID")
		return
	}

	// 6. Create initial history record
	hist := &models.Historico{
		AgendamentoID: ag.ID,
		IdosoID:       ag.IdosoID,
		CallSID:       sid,
		Status:        "iniciado",
	}
	_, err = db.CreateHistorico(ctx, hist)
	if err != nil {
		l.Error().Err(err).Msg("Failed to create history record")
	}
}
