package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/gemini"
	"eva-mind/internal/twilio"
	"eva-mind/internal/voice"
	"eva-mind/pkg/models"

	"github.com/rs/zerolog"
)

func Start(ctx context.Context, db *database.DB, cfg *config.Config, logger zerolog.Logger) {
	ticker := time.NewTicker(time.Minute * time.Duration(cfg.SchedulerInterval))
	defer ticker.Stop()

	logger.Info().
		Int("interval_minutes", cfg.SchedulerInterval).
		Msg("Scheduler iniciado")

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Scheduler encerrado")
			return
		case <-ticker.C:
			processAgendamentos(ctx, db, cfg, logger)
		}
	}
}

func processAgendamentos(ctx context.Context, db *database.DB, cfg *config.Config, logger zerolog.Logger) {
	logger.Info().Msg("Verificando agendamentos pendentes...")

	ags, err := db.GetPendingCalls(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Erro ao buscar agendamentos do banco")
		return
	}

	logger.Info().Int("count", len(ags)).Msg("Agendamentos encontrados")

	for _, ag := range ags {
		go handleAgendamento(ctx, ag, db, cfg, logger)
	}
}

func handleAgendamento(ctx context.Context, ag models.Agendamento, db *database.DB, cfg *config.Config, logger zerolog.Logger) {
	l := logger.With().
		Int("ag_id", ag.ID).
		Str("idoso", ag.NomeIdoso).
		Str("telefone", ag.Telefone).
		Logger()

	l.Info().Msg("Processando agendamento")

	// ✅ Incrementa tentativas
	if err := db.IncrementAttempts(ctx, ag.ID); err != nil {
		l.Error().Err(err).Msg("Erro ao incrementar tentativas")
	}

	// 1. Atualiza status para em_andamento
	err := db.UpdateCallStatus(ctx, ag.ID, "em_andamento", 0)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao atualizar status para em_andamento")
		return
	}

	// 2. Monta prompt personalizado
	systemPrompt := fmt.Sprintf(`
Você é Eva, uma assistente carinhosa e companheira para idosos.
Hoje você vai conversar com %s.
Tópico da conversa: %s.
`, ag.NomeIdoso, ag.Remedios)

	l.Debug().Msg("Criando sessão Gemini Live")

	// 3. Cria sessão Gemini Live com o prompt
	geminiClient, err := gemini.NewLiveClient(ctx, cfg, systemPrompt)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao criar Gemini Live")
		db.UpdateCallStatus(ctx, ag.ID, "aguardando_retry", ag.RetryIntervalMinutes)
		return
	}

	// 4. Salva o client na sessão global
	agIDStr := strconv.Itoa(ag.ID)
	voice.StoreSession(agIDStr, geminiClient)
	l.Info().Msg("Sessão Gemini armazenada")

	// 5. Faz a ligação outbound Twilio
	l.Info().Msg("Iniciando chamada Twilio")
	err = twilio.MakeOutboundCall(cfg, ag.Telefone, int64(ag.ID))
	if err != nil {
		l.Error().Err(err).Msg("Falha na ligação Twilio")
		voice.RemoveSession(agIDStr)
		db.UpdateCallStatus(ctx, ag.ID, "aguardando_retry", ag.RetryIntervalMinutes)
		return
	}

	l.Info().Msg("Ligação iniciada com sucesso")
}
