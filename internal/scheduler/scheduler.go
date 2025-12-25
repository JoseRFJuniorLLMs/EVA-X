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

	query := `
		SELECT id, idoso_id, telefone, nome_idoso, remedios 
		FROM agendamentos 
		WHERE horario <= NOW() + INTERVAL '5 minutes' 
		  AND horario > NOW() - INTERVAL '5 minutes'
		  AND status = 'pendente'
		  AND tentativas_realizadas < $1`

	rows, err := db.Pool.Query(ctx, query, cfg.MaxRetries)
	if err != nil {
		logger.Error().Err(err).Msg("Erro ao buscar agendamentos")
		return
	}
	defer rows.Close()

	var ags []models.Agendamento
	for rows.Next() {
		var ag models.Agendamento
		if err := rows.Scan(&ag.ID, &ag.IdosoID, &ag.Telefone, &ag.NomeIdoso, &ag.Remedios); err != nil {
			logger.Error().Err(err).Msg("Erro ao escanear agendamento")
			continue
		}
		ags = append(ags, ag)
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

	// 1. Atualiza status
	_, err := db.Pool.Exec(ctx, `UPDATE agendamentos SET status = 'em_andamento' WHERE id = $1`, ag.ID)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao atualizar status para em_andamento")
		return
	}

	// 2. Monta prompt personalizado
	systemPrompt := fmt.Sprintf(`
Você é Eva, uma assistente carinhosa e companheira para idosos.
Hoje você vai conversar com %s.
Tópico da conversa: %s.

Instruções importantes:
- Fale devagar e com clareza
- Use frases curtas e simples
- Pergunte como a pessoa está se sentindo
- Escute com atenção e paciência
- Seja carinhosa e empática
- Se a pessoa parecer confusa, repita gentilmente
- Termine sempre desejando um ótimo dia

Lembre-se: você está falando por telefone, então seja clara e objetiva.
`, ag.NomeIdoso, ag.Remedios)

	l.Debug().Msg("Criando sessão Gemini Live")

	// 3. Cria sessão Gemini Live com o prompt
	geminiClient, err := gemini.NewLiveClient(ctx, cfg, systemPrompt)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao criar Gemini Live")
		db.Pool.Exec(ctx, `UPDATE agendamentos SET status = 'falhou' WHERE id = $1`, ag.ID)
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
		db.Pool.Exec(ctx, `UPDATE agendamentos SET status = 'falhou' WHERE id = $1`, ag.ID)
		return
	}

	l.Info().Msg("Ligação iniciada com sucesso")
}
