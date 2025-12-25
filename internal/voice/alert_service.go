package voice

import (
	"context"
	"fmt"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/gemini"
	"eva-mind/internal/twilio"
	"eva-mind/pkg/models"

	"github.com/rs/zerolog"
)

type AlertService struct {
	db     *database.DB
	cfg    *config.Config
	logger zerolog.Logger
}

func NewAlertService(db *database.DB, cfg *config.Config, logger zerolog.Logger) *AlertService {
	return &AlertService{
		db:     db,
		cfg:    cfg,
		logger: logger,
	}
}

// TriggerFamilyAlertCall inicia uma ligação para o familiar avisando sobre uma intercorrência
func (s *AlertService) TriggerFamilyAlertCall(ctx context.Context, idosoID int, motivo, urgencia string) error {
	s.logger.Warn().Int("idoso_id", idosoID).Str("motivo", motivo).Msg("Iniciando alerta real para família")

	// 1. Busca dados do idoso e familiar
	callCtx, err := s.db.GetIdosoByID(ctx, idosoID)
	if err != nil {
		return fmt.Errorf("erro ao buscar dados do familiar: %w", err)
	}

	if callCtx.FamiliarTelefone == "" {
		return fmt.Errorf("familiar principal não possui telefone cadastrado")
	}

	// ✅ SIMPLIFICADO: Cria sessão com prompt minimalista
	// A Eva vai conversar naturalmente com o familiar sobre a situação
	sessionID := fmt.Sprintf("alert_%d", idosoID)

	geminiClient, err := gemini.NewLiveClient(ctx, s.cfg) // ✅ SEM prompt customizado
	if err != nil {
		return fmt.Errorf("falha ao criar sessão Gemini para alerta: %w", err)
	}

	StoreSession(sessionID, geminiClient)

	s.logger.Info().
		Str("familiar", callCtx.FamiliarNome).
		Str("telefone", callCtx.FamiliarTelefone).
		Str("motivo", motivo).
		Msg("Ligando para familiar sobre intercorrência")

	// 4. Faz a ligação via Twilio
	// Para o Twilio reconhecer que é um alerta, passamos uma flag ou usamos um range de ID negativo especial
	// Vamos usar -1000000 - idosoID para evitar colisão com agendamentos reais
	return twilio.MakeOutboundCall(s.cfg, callCtx.FamiliarTelefone, int64(-1000000-idosoID))
}

// TriggerEscalationCall avisa a família que o idoso não atendeu
func (s *AlertService) TriggerEscalationCall(ctx context.Context, ag models.Agendamento) error {
	s.logger.Info().Int("ag_id", ag.ID).Msg("Disparando escalonamento: Ligando para família")

	callCtx, err := s.db.GetCallContext(ctx, ag.ID)
	if err != nil {
		return err
	}

	if callCtx.FamiliarTelefone == "" {
		return fmt.Errorf("familiar não encontrado para escalonamento")
	}

	// ✅ SIMPLIFICADO: Cria sessão com prompt minimalista
	sessionID := fmt.Sprintf("escalation_%d", ag.ID)
	geminiClient, err := gemini.NewLiveClient(ctx, s.cfg) // ✅ SEM prompt customizado
	if err != nil {
		return err
	}
	StoreSession(sessionID, geminiClient)

	s.logger.Info().
		Str("familiar", callCtx.FamiliarNome).
		Str("telefone", callCtx.FamiliarTelefone).
		Str("idoso", callCtx.IdosoNome).
		Msg("Ligando para familiar sobre não atendimento")

	// Faz a ligação para o familiar usando range negativo -2000000
	return twilio.MakeOutboundCall(s.cfg, callCtx.FamiliarTelefone, int64(-2000000-ag.ID))
}
