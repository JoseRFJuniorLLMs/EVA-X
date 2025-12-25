package voice

import (
	"context"
	"fmt"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/gemini"
	"eva-mind/internal/twilio"
	"eva-mind/pkg/models"

	"github.com/cbroglie/mustache"
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

	// 2. Prepara o prompt de emergência para a EVA falar com a família
	template, err := s.db.GetTemplate(ctx, "eva_alerta_familia_critico", "")
	if err != nil {
		s.logger.Warn().Err(err).Msg("Template de alerta família não encontrado, usando fallback")
		template = "Olá {{familiar_nome}}, aqui é a Eva que cuida do(a) {{idoso_nome}}. Liguei para avisar que detectei um problema: {{motivo}}. A urgência é {{urgencia}}."
	}

	prompt, err := mustache.Render(template, map[string]interface{}{
		"familiar_nome": callCtx.FamiliarNome,
		"idoso_nome":    callCtx.IdosoNome,
		"motivo":        motivo,
		"urgencia":      urgencia,
	})
	if err != nil {
		prompt = fmt.Sprintf("Olá, aqui é a Eva. Emergência com %s: %s.", callCtx.IdosoNome, motivo)
	}

	// 3. Cria sessão Gemini para FALAR com o familiar
	// Usamos um ID especial para a sessão da família: "alert_<idosoID>"
	sessionID := fmt.Sprintf("alert_%d", idosoID)

	geminiClient, err := gemini.NewLiveClient(ctx, s.cfg, prompt)
	if err != nil {
		return fmt.Errorf("falha ao criar sessão Gemini para alerta: %w", err)
	}

	StoreSession(sessionID, geminiClient)

	// 4. Faz a ligação via Twilio.
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

	// Prepara o prompt de escalonamento
	template, err := s.db.GetTemplate(ctx, "eva_alerta_familia_nao_atende", "")
	if err != nil {
		template = "Olá {{familiar_nome}}, aqui é a Eva. Tentei falar com o(a) {{idoso_nome}} para o lembrete de {{medicamento}}, mas não consegui contato após várias tentativas. Poderia verificar?"
	}

	prompt, _ := mustache.Render(template, map[string]interface{}{
		"familiar_nome": callCtx.FamiliarNome,
		"idoso_nome":    callCtx.IdosoNome,
		"medicamento":   callCtx.Medicamento,
	})

	sessionID := fmt.Sprintf("escalation_%d", ag.ID)
	geminiClient, err := gemini.NewLiveClient(ctx, s.cfg, prompt)
	if err != nil {
		return err
	}
	StoreSession(sessionID, geminiClient)

	// Faz a ligação para o familiar usando range negativo -2000000
	return twilio.MakeOutboundCall(s.cfg, callCtx.FamiliarTelefone, int64(-2000000-ag.ID))
}
