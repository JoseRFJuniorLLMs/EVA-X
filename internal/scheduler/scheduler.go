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

	"github.com/cbroglie/mustache"
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
		if ag.TentativasRealizadas < ag.MaxRetries {
			go handleAgendamento(ctx, ag, db, cfg, logger)
		} else {
			l := logger.With().Int("ag_id", ag.ID).Str("idoso", ag.NomeIdoso).Logger()
			l.Warn().Msg("Limite de tentativas atingido. Iniciando escalonamento.")
			// TODO: Implementar lógica real de escalonamento (e-mail, SMS, etc)
			db.UpdateCallStatus(ctx, ag.ID, "falhou", 0)
		}
	}
}

func handleAgendamento(ctx context.Context, ag models.Agendamento, db *database.DB, cfg *config.Config, logger zerolog.Logger) {
	l := logger.With().
		Int("ag_id", ag.ID).
		Str("idoso", ag.NomeIdoso).
		Str("telefone", ag.Telefone).
		Logger()

	// ✅ Tenta adquirir lock para evitar processamento duplicado
	acquired, err := db.AcquireLock(ctx, ag.ID)
	if err != nil {
		l.Error().Err(err).Msg("Erro ao tentar adquirir lock do banco")
		return
	}
	if !acquired {
		l.Warn().Msg("Agendamento já está sendo processado por outra instância")
		return
	}
	defer func() {
		if _, err := db.ReleaseLock(ctx, ag.ID); err != nil {
			l.Error().Err(err).Msg("Erro ao liberar lock")
		}
	}()

	l.Info().Msg("Processando agendamento")

	// ✅ Incrementa tentativas
	if err := db.IncrementAttempts(ctx, ag.ID); err != nil {
		l.Error().Err(err).Msg("Erro ao incrementar tentativas")
	}

	// 1. Atualiza status para em_andamento
	err = db.UpdateCallStatus(ctx, ag.ID, "em_andamento", 0)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao atualizar status para em_andamento")
		return
	}

	// 1. Busca contexto completo do agendamento e idoso
	callCtx, err := db.GetCallContext(ctx, ag.ID)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao buscar contexto detalhado")
		// Continua com os dados básicos do ag se falhar
	}

	// 2. Busca e monta prompt personalizado via Mustache
	template, err := db.GetTemplate(ctx, "eva_base_v2", "")
	if err != nil {
		l.Warn().Err(err).Msg("Falha ao buscar template, usando fallback")
		template = "Você é Eva, uma assistente carinhosa. Conversando com {{nome_idoso}} sobre {{medicamento}}."
	}

	// Prepara mapa de dados para o template
	templateData := map[string]interface{}{
		"nome_idoso":           ag.NomeIdoso,
		"medicamento":          ag.Remedios,
		"idade":                0,
		"nivel_cognitivo":      "normal",
		"limitacoes_auditivas": false,
		"tom_voz":              "amigável",
	}

	if callCtx != nil {
		templateData["idade"] = callCtx.Idade
		templateData["nivel_cognitivo"] = callCtx.NivelCognitivo
		templateData["limitacoes_auditivas"] = callCtx.LimitacoesAuditivas
		templateData["tom_voz"] = callCtx.TomVoz
		if callCtx.Medicamento != "" {
			templateData["medicamento"] = callCtx.Medicamento
		}
	}

	systemPrompt, err := mustache.Render(template, templateData)
	if err != nil {
		l.Error().Err(err).Msg("Erro ao renderizar template Mustache")
		systemPrompt = fmt.Sprintf("Você é Eva. Hoje você vai conversar com %s sobre %s.", ag.NomeIdoso, ag.Remedios)
	}

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
