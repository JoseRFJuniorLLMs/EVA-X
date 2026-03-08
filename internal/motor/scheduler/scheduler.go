// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sort"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	"eva/internal/cortex/gemini"
	"eva/internal/brainstem/push"
	"eva/internal/motor/email"
)

type Scheduler struct {
	cfg          *config.Config
	db           *database.DB
	pushService  *push.FirebaseService
	emailService *email.EmailService
	stopChan     chan struct{}
}

func NewScheduler(cfg *config.Config, db *database.DB) (*Scheduler, error) {
	pushService, err := push.NewFirebaseService(cfg.FirebaseCredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase: %w", err)
	}

	// Inicializar serviço de email
	var emailService *email.EmailService
	if cfg.EnableEmailFallback {
		emailService, err = email.NewEmailService(cfg)
		if err != nil {
			log.Printf("Email service not configured: %v", err)
			emailService = nil
		} else {
			log.Println("Email service initialized")
		}
	}

	return &Scheduler{
		cfg:          cfg,
		db:           db,
		pushService:  pushService,
		emailService: emailService,
		stopChan:     make(chan struct{}),
	}, nil
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Ticker para verificar alertas não visualizados (a cada 2 minutos)
	alertTicker := time.NewTicker(2 * time.Minute)
	defer alertTicker.Stop()

	log.Println("Scheduler iniciado (verifica chamadas a cada 30s, alertas a cada 2min)")

	// Recovery global
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC no scheduler: %v", r)
			log.Printf("Stack trace: %s", debug.Stack())

			// Tentar reiniciar após 10 segundos
			time.Sleep(10 * time.Second)
			log.Println("Reiniciando scheduler...")
			go s.Start(ctx)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler parado por contexto")
			return
		case <-s.stopChan:
			log.Println("Scheduler parado por stopChan")
			return
		case <-ticker.C:
			// Executar com recovery individual
			s.safeExecute("checkAndTriggerCalls", s.checkAndTriggerCalls)
			s.safeExecute("checkMissedCalls", s.checkMissedCalls)
		case <-alertTicker.C:
			s.safeExecute("checkUnacknowledgedAlerts", s.checkUnacknowledgedAlerts)
		}
	}
}

// safeExecute wraps function execution with panic recovery
func (s *Scheduler) safeExecute(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC em %s: %v", name, r)
			log.Printf("Stack trace: %s", debug.Stack())

			// Registrar erro no banco
			now := time.Now().UTC().Format(time.RFC3339)
			s.db.Insert(context.Background(), "system_errors", map[string]interface{}{
				"component":     name,
				"error_type":    "panic",
				"error_message": fmt.Sprintf("%v", r),
				"stack_trace":   string(debug.Stack()),
				"created_at":    now,
			})
		}
	}()

	fn()
}

func (s *Scheduler) Stop() {
	close(s.stopChan)
}

func (s *Scheduler) checkAndTriggerCalls() {
	ctx := context.Background()
	now := time.Now()

	log.Printf("Buscando agendamentos para executar... (Server Time: %s)", now.Format(time.RFC3339))

	// Buscar agendamentos pendentes
	rows, err := s.db.QueryByLabel(ctx, "agendamentos",
		" AND n.status = $status",
		map[string]interface{}{"status": "agendado"}, 0)
	if err != nil {
		log.Printf("Erro na query do scheduler: %v", err)
		return
	}

	// Filter by data_hora_agendada <= now and idoso ativo, then limit to 10
	type agendamentoInfo struct {
		agendamentoID int64
		idosoID       int64
		dataHora      time.Time
		deviceToken   string
		nome          string
	}

	var candidates []agendamentoInfo
	for _, m := range rows {
		dataHora := database.GetTime(m, "data_hora_agendada")
		if dataHora.IsZero() || dataHora.After(now) {
			continue
		}

		idosoID := database.GetInt64(m, "idoso_id")

		// Buscar dados do idoso
		idosoRow, err := s.db.GetNodeByID(ctx, "idosos", idosoID)
		if err != nil || idosoRow == nil {
			continue
		}
		if !database.GetBool(idosoRow, "ativo") {
			continue
		}

		candidates = append(candidates, agendamentoInfo{
			agendamentoID: database.GetInt64(m, "id"),
			idosoID:       idosoID,
			dataHora:      dataHora,
			deviceToken:   database.GetString(idosoRow, "device_token"),
			nome:          database.GetString(idosoRow, "nome"),
		})
	}

	// Limit to 10
	if len(candidates) > 10 {
		candidates = candidates[:10]
	}

	found := false

	for _, c := range candidates {
		if c.deviceToken == "" {
			log.Printf("Sem device_token: %s", c.nome)
			s.updateStatus(c.agendamentoID, "falha_sem_token")
			continue
		}

		sessionID := fmt.Sprintf("call-%d-%d", c.agendamentoID, time.Now().Unix())

		err := s.pushService.SendCallNotification(c.deviceToken, sessionID, c.nome)
		if err != nil {
			if push.IsInvalidTokenError(err) {
				log.Printf("Token invalido para: %s (%v)", c.nome, err)
				s.updateStatus(c.agendamentoID, "falha_token_invalido")

				// Marcar que o token precisa ser atualizado
				now := time.Now().UTC().Format(time.RFC3339)
				s.db.Update(ctx, "idosos",
					map[string]interface{}{"id": c.idosoID},
					map[string]interface{}{
						"device_token_valido":       false,
						"device_token_atualizado_em": now,
					})
			} else {
				log.Printf("Erro ao enviar push: %s - %v", c.nome, err)
				s.updateStatus(c.agendamentoID, "falha_envio")
			}
			continue
		}

		log.Printf("Push enviado: %s", c.nome)
		s.updateStatusWithTimestamp(c.agendamentoID, "em_andamento")
		found = true
	}

	if !found {
		log.Printf("Nenhum agendamento encontrado para agora.")
	}
}

// checkMissedCalls verifica chamadas que ficaram "penduradas" (tocaram mas ninguém atendeu)
func (s *Scheduler) checkMissedCalls() {
	ctx := context.Background()

	// Buscar agendamentos em_andamento com mais de 45 segundos
	cutoff := time.Now().Add(-45 * time.Second).UTC().Format(time.RFC3339)

	rows, err := s.db.QueryByLabel(ctx, "agendamentos",
		" AND n.status = $status",
		map[string]interface{}{"status": "em_andamento"}, 0)
	if err != nil {
		log.Printf("Erro ao verificar chamadas perdidas: %v", err)
		return
	}

	for _, m := range rows {
		dataHora := database.GetTime(m, "data_hora_agendada")
		cutoffTime, _ := time.Parse(time.RFC3339, cutoff)
		if dataHora.IsZero() || dataHora.After(cutoffTime) {
			continue
		}

		agendamentoID := database.GetInt64(m, "id")
		idosoID := database.GetInt64(m, "idoso_id")

		// Buscar dados do idoso
		idosoRow, err := s.db.GetNodeByID(ctx, "idosos", idosoID)
		if err != nil || idosoRow == nil {
			continue
		}
		nomeIdoso := database.GetString(idosoRow, "nome")

		// Buscar cuidador com prioridade mais alta
		cuidadores, _ := s.db.QueryByLabel(ctx, "cuidadores",
			" AND n.idoso_id = $idoso AND n.ativo = $ativo",
			map[string]interface{}{"idoso": idosoID, "ativo": true}, 0)

		var tokenCuidador, phoneCuidador, emailCuidador string
		if len(cuidadores) > 0 {
			// Sort by prioridade ASC and pick first with device_token
			sort.Slice(cuidadores, func(i, j int) bool {
				return database.GetInt64(cuidadores[i], "prioridade") < database.GetInt64(cuidadores[j], "prioridade")
			})
			for _, c := range cuidadores {
				tk := database.GetString(c, "device_token")
				if tk != "" {
					tokenCuidador = tk
					phoneCuidador = database.GetString(c, "telefone")
					emailCuidador = database.GetString(c, "email")
					break
				}
			}
		}

		log.Printf("CHAMADA PERDIDA detectada para Idoso: %s (ID: %d)", nomeIdoso, idosoID)

		// 1. Atualizar status do agendamento
		now := time.Now().UTC().Format(time.RFC3339)
		tentativas := database.GetInt64(m, "tentativas_realizadas") + 1
		err = s.db.Update(ctx, "agendamentos",
			map[string]interface{}{"id": agendamentoID},
			map[string]interface{}{
				"status":                "nao_atendido",
				"ultima_tentativa":      now,
				"tentativas_realizadas": tentativas,
			})
		if err != nil {
			log.Printf("Erro ao atualizar agendamento: %v", err)
			continue
		}

		// 2. Registrar no histórico de ligações
		historicoID, errHistorico := s.db.Insert(ctx, "historico_ligacoes", map[string]interface{}{
			"agendamento_id":      agendamentoID,
			"idoso_id":            idosoID,
			"inicio_chamada":      time.Now().Add(-45 * time.Second).UTC().Format(time.RFC3339),
			"fim_chamada":         now,
			"duracao_segundos":    45,
			"tarefa_concluida":    false,
			"motivo_falha":        "Chamada nao atendida pelo idoso apos 45 segundos",
			"transcricao_completa": fmt.Sprintf("Push notification enviado mas nao houve resposta do dispositivo. Idoso: %s", nomeIdoso),
			"criado_em":           now,
		})
		if errHistorico != nil {
			log.Printf("Erro ao registrar historico: %v", errHistorico)
		} else {
			log.Printf("Historico registrado: ID %d", historicoID)
		}

		// 3. Criar alerta no sistema
		alertID, errAlerta := s.db.Insert(ctx, "alertas", map[string]interface{}{
			"idoso_id":    idosoID,
			"ligacao_id":  historicoID,
			"tipo":        "nao_atende_telefone",
			"severidade":  "aviso",
			"mensagem":    fmt.Sprintf("%s nao atendeu a chamada programada da EVA as %s", nomeIdoso, time.Now().Format("15:04")),
			"destinatarios": `["cuidador"]`,
			"enviado":     false,
			"visualizado": false,
			"data_envio":  now,
			"criado_em":   now,
		})
		if errAlerta != nil {
			log.Printf("Erro ao criar alerta: %v", errAlerta)
		}

		// 4. Registrar na timeline
		_, errTimeline := s.db.Insert(ctx, "timeline", map[string]interface{}{
			"idoso_id":  idosoID,
			"tipo":      "ligacao",
			"subtipo":   "nao_atendida",
			"titulo":    "Chamada Nao Atendida",
			"descricao": fmt.Sprintf("EVA tentou contato com %s mas a chamada nao foi atendida.", nomeIdoso),
			"data":      now,
			"criado_em": now,
		})
		if errTimeline != nil {
			log.Printf("Erro ao registrar timeline: %v", errTimeline)
		}

		// 5. Notificar o cuidador via push notification
		if tokenCuidador != "" {
			errPush := s.pushService.SendMissedCallAlert(tokenCuidador, nomeIdoso)
			if errPush != nil {
				log.Printf("Erro ao enviar push para cuidador: %v", errPush)

				// Marcar alerta para envio por outros meios
				escalTime := time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339)
				s.db.Update(ctx, "alertas",
					map[string]interface{}{"id": alertID},
					map[string]interface{}{
						"necessita_escalamento": true,
						"tempo_escalamento":     escalTime,
					})
			} else {
				log.Printf("Cuidador notificado sobre chamada perdida de %s", nomeIdoso)

				// Marcar alerta como enviado
				s.db.Update(ctx, "alertas",
					map[string]interface{}{"id": alertID},
					map[string]interface{}{"enviado": true})
			}

			// Tentar outros meios (Email) e Escalamento
			log.Printf("Tentando meios alternativos para %s...", nomeIdoso)

			if emailCuidador != "" && s.emailService != nil {
				subject := fmt.Sprintf("Alerta de Chamada Perdida: %s", nomeIdoso)
				body := fmt.Sprintf(`
					<h2>Atencao! Chamada Nao Atendida</h2>
					<p>A EVA tentou entrar em contato com <b>%s</b> hoje as %s, mas nao houve resposta.</p>
					<p>Como nao conseguimos enviar a notificacao via aplicativo, estamos enviando este email de seguranca.</p>
					<p>Por favor, verifique o bem-estar do idoso assim que possivel.</p>
					<hr>
					<p><small>Este e um aviso automatico gerado pelo sistema EVA-Mind.</small></p>
				`, nomeIdoso, time.Now().Format("15:04"))

				if errEmail := s.emailService.SendEmail(emailCuidador, subject, body); errEmail != nil {
					log.Printf("Falha critica ao enviar email de seguranca para %s: %v", emailCuidador, errEmail)
				} else {
					log.Printf("Email de seguranca enviado com sucesso para %s", emailCuidador)
					// Marcar alerta como enviado (mesmo que por email)
					s.db.Update(ctx, "alertas",
						map[string]interface{}{"id": alertID},
						map[string]interface{}{"enviado": true})
				}
			}

			// Forçar escalamento para que o painel de monitoramento destaque o problema
			log.Printf("Escalando alerta ID %d para monitoramento administrativo", alertID)
			s.db.Update(ctx, "alertas",
				map[string]interface{}{"id": alertID},
				map[string]interface{}{
					"necessita_escalamento": true,
					"tempo_escalamento":     now,
				})
		}

		_ = phoneCuidador // kept for future SMS integration
		log.Printf("Chamada perdida processada completamente para %s", nomeIdoso)
	}
}

// checkUnacknowledgedAlerts verifica alertas críticos não visualizados
func (s *Scheduler) checkUnacknowledgedAlerts() {
	if err := gemini.CheckUnacknowledgedAlerts(s.db, s.pushService); err != nil {
		log.Printf("Erro ao verificar alertas nao visualizados: %v", err)
	}
}

func (s *Scheduler) updateStatus(id int64, status string) {
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.db.Update(ctx, "agendamentos",
		map[string]interface{}{"id": id},
		map[string]interface{}{
			"status":       status,
			"atualizado_em": now,
		})
	if err != nil {
		log.Printf("Erro ao atualizar status: %v", err)
	}
}

func (s *Scheduler) updateStatusWithTimestamp(id int64, status string) {
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.db.Update(ctx, "agendamentos",
		map[string]interface{}{"id": id},
		map[string]interface{}{
			"status":           status,
			"ultima_tentativa": now,
			"atualizado_em":    now,
		})
	if err != nil {
		log.Printf("Erro ao atualizar status: %v", err)
	}
}
