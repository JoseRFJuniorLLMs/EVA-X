// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	"eva/internal/brainstem/logger"
	"eva/internal/brainstem/push"
	"eva/internal/cortex/gemini"
	"eva/internal/motor/actions"

	"github.com/rs/zerolog"
)

// AnalysisService encapsula logica de analise de conversas
type AnalysisService struct {
	cfg         *config.Config
	db          *database.DB
	pushService *push.FirebaseService
	log         zerolog.Logger
}

// NewAnalysisService cria nova instancia do servico
func NewAnalysisService(cfg *config.Config, db *database.DB, pushService *push.FirebaseService) *AnalysisService {
	return &AnalysisService{
		cfg:         cfg,
		db:          db,
		pushService: pushService,
		log:         logger.Logger.With().Str("service", "analysis").Logger(),
	}
}

// AnalyzeAndSaveConversation analisa conversa e salva no banco
func (s *AnalysisService) AnalyzeAndSaveConversation(idosoID int64) error {
	s.log.Info().Int64("idoso_id", idosoID).Msg("Iniciando analise de conversa")

	// 1. Buscar ultima transcricao
	transcript, historyID, err := s.getLastTranscript(idosoID)
	if err != nil {
		s.log.Warn().Err(err).Msg("Nenhuma transcricao encontrada")
		return err
	}

	s.log.Debug().
		Int64("history_id", historyID).
		Int("transcript_length", len(transcript)).
		Msg("Transcricao encontrada")

	// 2. Analisar com Gemini
	analysis, err := gemini.AnalyzeConversation(s.cfg, transcript)
	if err != nil {
		s.log.Error().Err(err).Msg("Erro ao analisar conversa")
		return err
	}

	s.log.Info().
		Str("urgency", analysis.UrgencyLevel).
		Str("mood", analysis.MoodState).
		Bool("emergency", analysis.EmergencySymptoms).
		Msg("Analise concluida")

	// 3. Salvar no banco
	if err := s.saveAnalysis(historyID, analysis); err != nil {
		s.log.Error().Err(err).Msg("Erro ao salvar analise")
		return err
	}

	// 4. Processar alertas se necessario
	if analysis.UrgencyLevel == "CRITICO" || analysis.UrgencyLevel == "ALTO" {
		s.handleUrgentAlert(idosoID, analysis)
	}

	return nil
}

// getLastTranscript busca ultima transcricao nao analisada
func (s *AnalysisService) getLastTranscript(idosoID int64) (string, int64, error) {
	ctx := context.Background()

	rows, err := s.db.QueryByLabel(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso_id", map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return "", 0, fmt.Errorf("erro ao buscar transcricao: %w", err)
	}

	// Filter: fim_chamada must be empty and transcricao_completa length > 50
	type candidate struct {
		id             int64
		transcript     string
		inicioChamada  time.Time
	}
	var candidates []candidate
	for _, m := range rows {
		fimChamada := database.GetString(m, "fim_chamada")
		if fimChamada != "" {
			continue
		}
		transcricao := database.GetString(m, "transcricao_completa")
		if len(transcricao) <= 50 {
			continue
		}
		candidates = append(candidates, candidate{
			id:            database.GetInt64(m, "id"),
			transcript:    transcricao,
			inicioChamada: database.GetTime(m, "inicio_chamada"),
		})
	}

	if len(candidates) == 0 {
		return "", 0, fmt.Errorf("nenhuma transcricao encontrada")
	}

	// Sort by inicio_chamada DESC (most recent first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].inicioChamada.After(candidates[j].inicioChamada)
	})

	return candidates[0].transcript, candidates[0].id, nil
}

// saveAnalysis salva analise no banco
func (s *AnalysisService) saveAnalysis(historyID int64, analysis *gemini.ConversationAnalysis) error {
	ctx := context.Background()

	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return fmt.Errorf("erro ao serializar analise: %w", err)
	}

	matchKeys := map[string]interface{}{
		"id": historyID,
	}

	updates := map[string]interface{}{
		"fim_chamada":       time.Now().Format(time.RFC3339),
		"analise_gemini":    string(analysisJSON),
		"urgencia":          analysis.UrgencyLevel,
		"sentimento":        analysis.MoodState,
		"transcricao_resumo": analysis.Summary,
	}

	if err := s.db.Update(ctx, "historico_ligacoes", matchKeys, updates); err != nil {
		return fmt.Errorf("erro ao salvar analise: %w", err)
	}

	s.log.Info().Int64("history_id", historyID).Msg("Analise salva com sucesso")
	return nil
}

// handleUrgentAlert processa alertas urgentes
func (s *AnalysisService) handleUrgentAlert(idosoID int64, analysis *gemini.ConversationAnalysis) {
	s.log.Warn().
		Int64("idoso_id", idosoID).
		Str("urgency", analysis.UrgencyLevel).
		Msg("Alerta de urgencia detectado")

	alertMsg := fmt.Sprintf(
		"URGENCIA %s: %s. %s",
		analysis.UrgencyLevel,
		strings.Join(analysis.KeyConcerns, ", "),
		analysis.RecommendedAction,
	)

	err := actions.AlertFamily(s.db, s.pushService, nil, idosoID, alertMsg)
	if err != nil {
		s.log.Error().Err(err).Msg("Erro ao alertar familia")
	} else {
		s.log.Info().Msg("Familia alertada com sucesso")
	}

	// Registrar alerta no sistema
	s.createSystemAlert(idosoID, analysis)
}

// createSystemAlert cria registro de alerta no banco
func (s *AnalysisService) createSystemAlert(idosoID int64, analysis *gemini.ConversationAnalysis) {
	ctx := context.Background()

	severity := "aviso"
	if analysis.UrgencyLevel == "CRITICO" {
		severity = "critica"
	} else if analysis.UrgencyLevel == "ALTO" {
		severity = "alta"
	}

	concernsJSON, _ := json.Marshal(analysis.KeyConcerns)

	content := map[string]interface{}{
		"idoso_id":         idosoID,
		"tipo":             "analise_urgente",
		"severidade":       severity,
		"mensagem":         analysis.RecommendedAction,
		"dados_adicionais": string(concernsJSON),
		"criado_em":        time.Now().Format(time.RFC3339),
	}

	_, err := s.db.Insert(ctx, "alertas", content)
	if err != nil {
		s.log.Error().Err(err).Msg("Erro ao criar alerta no sistema")
	}
}

// GetAnalysisHistory retorna historico de analises
func (s *AnalysisService) GetAnalysisHistory(idosoID int64, limit int) ([]gemini.ConversationAnalysis, error) {
	ctx := context.Background()

	rows, err := s.db.QueryByLabel(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso_id", map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	// Filter rows that have analise_gemini and collect with inicio_chamada for sorting
	type entry struct {
		analysis      gemini.ConversationAnalysis
		inicioChamada time.Time
	}
	var entries []entry
	for _, m := range rows {
		analysisJSON := database.GetString(m, "analise_gemini")
		if analysisJSON == "" {
			continue
		}

		var analysis gemini.ConversationAnalysis
		if err := json.Unmarshal([]byte(analysisJSON), &analysis); err != nil {
			continue
		}

		entries = append(entries, entry{
			analysis:      analysis,
			inicioChamada: database.GetTime(m, "inicio_chamada"),
		})
	}

	// Sort by inicio_chamada DESC
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].inicioChamada.After(entries[j].inicioChamada)
	})

	// Apply limit
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	analyses := make([]gemini.ConversationAnalysis, len(entries))
	for i, e := range entries {
		analyses[i] = e.analysis
	}

	return analyses, nil
}

// GetAnalysisStats retorna estatisticas de analises
func (s *AnalysisService) GetAnalysisStats(idosoID int64, days int) (*AnalysisStats, error) {
	ctx := context.Background()

	rows, err := s.db.QueryByLabel(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso_id", map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var stats AnalysisStats

	for _, m := range rows {
		// Only count rows with analise_gemini
		if database.GetString(m, "analise_gemini") == "" {
			continue
		}

		// Only count rows within the time window
		inicioChamada := database.GetTime(m, "inicio_chamada")
		if inicioChamada.Before(cutoff) {
			continue
		}

		stats.Total++

		urgencia := database.GetString(m, "urgencia")
		sentimento := database.GetString(m, "sentimento")

		if urgencia == "CRITICO" {
			stats.Criticos++
		}
		if urgencia == "ALTO" {
			stats.Altos++
		}
		if sentimento == "triste" {
			stats.Tristes++
		}
		if sentimento == "feliz" {
			stats.Felizes++
		}
	}

	return &stats, nil
}

type AnalysisStats struct {
	Total    int
	Criticos int
	Altos    int
	Tristes  int
	Felizes  int
}
