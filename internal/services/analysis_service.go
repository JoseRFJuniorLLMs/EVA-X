package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"eva-mind/internal/motor/actions" // ✅ NEW IMPORT
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/cortex/gemini"
	"eva-mind/internal/brainstem/logger"
	"eva-mind/internal/brainstem/push"

	"github.com/rs/zerolog"
)

// AnalysisService encapsula lógica de análise de conversas
type AnalysisService struct {
	cfg         *config.Config
	db          *sql.DB
	pushService *push.FirebaseService
	log         zerolog.Logger
}

// NewAnalysisService cria nova instância do serviço
func NewAnalysisService(cfg *config.Config, db *sql.DB, pushService *push.FirebaseService) *AnalysisService {
	return &AnalysisService{
		cfg:         cfg,
		db:          db,
		pushService: pushService,
		log:         logger.Logger.With().Str("service", "analysis").Logger(),
	}
}

// AnalyzeAndSaveConversation analisa conversa e salva no banco
func (s *AnalysisService) AnalyzeAndSaveConversation(idosoID int64) error {
	s.log.Info().Int64("idoso_id", idosoID).Msg("Iniciando análise de conversa")

	// 1. Buscar última transcrição
	transcript, historyID, err := s.getLastTranscript(idosoID)
	if err != nil {
		s.log.Warn().Err(err).Msg("Nenhuma transcrição encontrada")
		return err
	}

	s.log.Debug().
		Int64("history_id", historyID).
		Int("transcript_length", len(transcript)).
		Msg("Transcrição encontrada")

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
		Msg("Análise concluída")

	// 3. Salvar no banco
	if err := s.saveAnalysis(historyID, analysis); err != nil {
		s.log.Error().Err(err).Msg("Erro ao salvar análise")
		return err
	}

	// 4. Processar alertas se necessário
	if analysis.UrgencyLevel == "CRITICO" || analysis.UrgencyLevel == "ALTO" {
		s.handleUrgentAlert(idosoID, analysis)
	}

	return nil
}

// getLastTranscript busca última transcrição não analisada
func (s *AnalysisService) getLastTranscript(idosoID int64) (string, int64, error) {
	query := `
		SELECT id, transcricao_completa
		FROM historico_ligacoes
		WHERE idoso_id = $1 
		  AND fim_chamada IS NULL
		  AND transcricao_completa IS NOT NULL
		  AND LENGTH(transcricao_completa) > 50
		ORDER BY inicio_chamada DESC
		LIMIT 1
	`

	var historyID int64
	var transcript string

	err := s.db.QueryRow(query, idosoID).Scan(&historyID, &transcript)
	if err == sql.ErrNoRows {
		return "", 0, fmt.Errorf("nenhuma transcrição encontrada")
	}
	if err != nil {
		return "", 0, fmt.Errorf("erro ao buscar transcrição: %w", err)
	}

	return transcript, historyID, nil
}

// saveAnalysis salva análise no banco
func (s *AnalysisService) saveAnalysis(historyID int64, analysis *gemini.ConversationAnalysis) error {
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return fmt.Errorf("erro ao serializar análise: %w", err)
	}

	query := `
		UPDATE historico_ligacoes 
		SET 
			fim_chamada = CURRENT_TIMESTAMP,
			analise_gemini = $2::jsonb,
			urgencia = $3,
			sentimento = $4,
			transcricao_resumo = $5
		WHERE id = $1
	`

	result, err := s.db.Exec(
		query,
		historyID,
		string(analysisJSON),
		analysis.UrgencyLevel,
		analysis.MoodState,
		analysis.Summary,
	)

	if err != nil {
		return fmt.Errorf("erro ao salvar análise: %w", err)
	}

	rows, _ := result.RowsAffected()
	s.log.Info().Int64("rows", rows).Msg("Análise salva com sucesso")

	return nil
}

// handleUrgentAlert processa alertas urgentes
func (s *AnalysisService) handleUrgentAlert(idosoID int64, analysis *gemini.ConversationAnalysis) {
	s.log.Warn().
		Int64("idoso_id", idosoID).
		Str("urgency", analysis.UrgencyLevel).
		Msg("Alerta de urgência detectado")

	alertMsg := fmt.Sprintf(
		"URGÊNCIA %s: %s. %s",
		analysis.UrgencyLevel,
		strings.Join(analysis.KeyConcerns, ", "),
		analysis.RecommendedAction,
	)

	err := actions.AlertFamily(s.db, s.pushService, nil, idosoID, alertMsg)
	if err != nil {
		s.log.Error().Err(err).Msg("Erro ao alertar família")
	} else {
		s.log.Info().Msg("Família alertada com sucesso")
	}

	// Registrar alerta no sistema
	s.createSystemAlert(idosoID, analysis)
}

// createSystemAlert cria registro de alerta no banco
func (s *AnalysisService) createSystemAlert(idosoID int64, analysis *gemini.ConversationAnalysis) {
	severity := "aviso"
	if analysis.UrgencyLevel == "CRITICO" {
		severity = "critica"
	} else if analysis.UrgencyLevel == "ALTO" {
		severity = "alta"
	}

	concernsJSON, _ := json.Marshal(analysis.KeyConcerns)

	query := `
		INSERT INTO alertas (
			idoso_id,
			tipo,
			severidade,
			mensagem,
			dados_adicionais,
			criado_em
		) VALUES ($1, 'analise_urgente', $2, $3, $4, NOW())
	`

	_, err := s.db.Exec(
		query,
		idosoID,
		severity,
		analysis.RecommendedAction,
		string(concernsJSON),
	)

	if err != nil {
		s.log.Error().Err(err).Msg("Erro ao criar alerta no sistema")
	}
}

// GetAnalysisHistory retorna histórico de análises
func (s *AnalysisService) GetAnalysisHistory(idosoID int64, limit int) ([]gemini.ConversationAnalysis, error) {
	query := `
		SELECT analise_gemini
		FROM historico_ligacoes
		WHERE idoso_id = $1 
		  AND analise_gemini IS NOT NULL
		ORDER BY inicio_chamada DESC
		LIMIT $2
	`

	rows, err := s.db.Query(query, idosoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var analyses []gemini.ConversationAnalysis

	for rows.Next() {
		var analysisJSON string
		if err := rows.Scan(&analysisJSON); err != nil {
			continue
		}

		var analysis gemini.ConversationAnalysis
		if err := json.Unmarshal([]byte(analysisJSON), &analysis); err != nil {
			continue
		}

		analyses = append(analyses, analysis)
	}

	return analyses, nil
}

// GetAnalysisStats retorna estatísticas de análises
func (s *AnalysisService) GetAnalysisStats(idosoID int64, days int) (*AnalysisStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN urgencia = 'CRITICO' THEN 1 END) as criticos,
			COUNT(CASE WHEN urgencia = 'ALTO' THEN 1 END) as altos,
			COUNT(CASE WHEN sentimento = 'triste' THEN 1 END) as tristes,
			COUNT(CASE WHEN sentimento = 'feliz' THEN 1 END) as felizes
		FROM historico_ligacoes
		WHERE idoso_id = $1
		  AND inicio_chamada > NOW() - INTERVAL '$2 days'
		  AND analise_gemini IS NOT NULL
	`

	var stats AnalysisStats
	err := s.db.QueryRow(query, idosoID, days).Scan(
		&stats.Total,
		&stats.Criticos,
		&stats.Altos,
		&stats.Tristes,
		&stats.Felizes,
	)

	if err != nil {
		return nil, err
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
