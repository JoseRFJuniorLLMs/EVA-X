// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	"eva/internal/brainstem/push"
	"eva/internal/gemini"
	"eva/internal/voice"
	"eva/pkg/models"

	"github.com/rs/zerolog"
)

func Start(ctx context.Context, db *database.DB, cfg *config.Config, logger zerolog.Logger, alertService *voice.AlertService, pushService *push.FirebaseService) {
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
			processAgendamentos(ctx, db, cfg, logger, alertService, pushService)
		}
	}
}

func processAgendamentos(ctx context.Context, db *database.DB, cfg *config.Config, logger zerolog.Logger, alertService *voice.AlertService, pushService *push.FirebaseService) {
	logger.Info().Msg("Verificando agendamentos pendentes...")

	ags, err := db.GetPendingCalls(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Erro ao buscar agendamentos do banco")
		return
	}

	logger.Info().Int("count", len(ags)).Msg("Agendamentos encontrados")

	for _, ag := range ags {
		if ag.TentativasRealizadas < ag.MaxRetries {
			go handleAgendamento(ctx, ag, db, cfg, logger, alertService, pushService)
		} else {
			l := logger.With().Int("ag_id", ag.ID).Str("idoso", ag.NomeIdoso).Logger()
			l.Warn().Msg("Limite de tentativas atingido. Iniciando escalonamento.")
			db.UpdateCallStatus(ctx, ag.ID, "falhou_definitivamente", 0)
			alertService.TriggerEscalationCall(ctx, ag)
		}
	}
}

func handleAgendamento(ctx context.Context, ag models.Agendamento, db *database.DB, cfg *config.Config, logger zerolog.Logger, alertService *voice.AlertService, pushService *push.FirebaseService) {
	l := logger.With().
		Int("ag_id", ag.ID).
		Str("idoso", ag.NomeIdoso).
		Str("telefone", ag.Telefone).
		Logger()

	// Tenta adquirir lock para evitar processamento duplicado
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

	// Incrementa tentativas
	if err := db.IncrementAttempts(ctx, ag.ID); err != nil {
		l.Error().Err(err).Msg("Erro ao incrementar tentativas")
	}

	// Atualiza status para em_andamento
	err = db.UpdateCallStatus(ctx, ag.ID, "em_andamento", 0)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao atualizar status para em_andamento")
		return
	}

	// Busca configurações dinâmicas do sistema (modelo Gemini)
	modelID, err := db.GetSystemSetting(ctx, "gemini.model_id")
	if err == nil && modelID != "" {
		l.Info().Str("model", modelID).Msg("Usando modelo configurado no DB")
		cfg.ModelID = modelID
	}

	// Cria sessão Gemini Live
	l.Debug().Msg("Criando sessão Gemini Live")
	geminiClient, err := gemini.NewLiveClient(ctx, cfg)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao criar Gemini Live")
		db.UpdateCallStatus(ctx, ag.ID, "aguardando_retry", ag.RetryIntervalMinutes)
		return
	}

	// Salva o client na sessão global
	agIDStr := strconv.Itoa(ag.ID)
	voice.StoreSession(agIDStr, geminiClient)
	l.Info().Msg("Sessão Gemini armazenada")

	// Verifica se tem device_token (app mobile instalado)
	if ag.DeviceToken == "" {
		l.Warn().Msg("Idoso sem device_token - app não instalado ou token não sincronizado")
		voice.RemoveSession(agIDStr)
		db.UpdateCallStatus(ctx, ag.ID, "aguardando_retry", ag.RetryIntervalMinutes)
		return
	}

	if pushService == nil {
		l.Error().Msg("Firebase push service não disponível")
		voice.RemoveSession(agIDStr)
		db.UpdateCallStatus(ctx, ag.ID, "aguardando_retry", ag.RetryIntervalMinutes)
		return
	}

	// Envia push notification FCM para o app mobile
	sessionID := fmt.Sprintf("call-%d-%d", ag.ID, time.Now().Unix())
	tokenPreview := ag.DeviceToken
	if len(tokenPreview) > 20 {
		tokenPreview = tokenPreview[:20] + "..."
	}
	l.Info().Str("session_id", sessionID).Str("device_token", tokenPreview).Msg("Enviando FCM push para app mobile")

	err = pushService.SendCallNotification(ag.DeviceToken, sessionID, ag.NomeIdoso)
	if err != nil {
		l.Error().Err(err).Msg("Falha ao enviar FCM push")
		voice.RemoveSession(agIDStr)

		// Se token inválido, limpar
		if push.IsInvalidTokenError(err) {
			l.Warn().Msg("Device token inválido - limpando")
			updateErr := db.Update(ctx, "idosos",
				map[string]interface{}{"pg_id": ag.IdosoID},
				map[string]interface{}{"device_token": "", "device_token_valido": false},
			)
			if updateErr != nil {
				l.Warn().Err(updateErr).Msg("Falha ao limpar device_token via NietzscheDB")
			}
		}

		// Verifica se atingiu limite para escalonar
		if ag.TentativasRealizadas+1 >= ag.MaxRetries {
			l.Warn().Msg("Limite de tentativas atingido. Escalonando para família.")
			db.UpdateCallStatus(ctx, ag.ID, "falhou_definitivamente", 0)
			alertService.TriggerEscalationCall(ctx, ag)
		} else {
			db.UpdateCallStatus(ctx, ag.ID, "aguardando_retry", ag.RetryIntervalMinutes)
		}
		return
	}

	l.Info().Str("session_id", sessionID).Msg("Push FCM enviado - aguardando conexão WebSocket do app")
}
