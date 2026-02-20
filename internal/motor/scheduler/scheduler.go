// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/motor/email"
	"eva/internal/cortex/gemini"
	"eva/internal/brainstem/push"
)

type Scheduler struct {
	cfg          *config.Config
	db           *sql.DB
	pushService  *push.FirebaseService
	emailService *email.EmailService
	stopChan     chan struct{}
}

func NewScheduler(cfg *config.Config, db *sql.DB) (*Scheduler, error) {
	pushService, err := push.NewFirebaseService(cfg.FirebaseCredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase: %w", err)
	}

	// Inicializar serviço de email
	var emailService *email.EmailService
	if cfg.EnableEmailFallback {
		emailService, err = email.NewEmailService(cfg)
		if err != nil {
			log.Printf("⚠️ Email service not configured: %v", err)
			emailService = nil
		} else {
			log.Println("✅ Email service initialized")
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

	log.Println("⏰ Scheduler iniciado (verifica chamadas a cada 30s, alertas a cada 2min)")

	// ✅ NOVO: Recovery global
	defer func() {
		if r := recover(); r != nil {
			log.Printf("🚨 PANIC no scheduler: %v", r)
			log.Printf("Stack trace: %s", debug.Stack())

			// Tentar reiniciar após 10 segundos
			time.Sleep(10 * time.Second)
			log.Println("🔄 Reiniciando scheduler...")
			go s.Start(ctx)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Scheduler parado por contexto")
			return
		case <-s.stopChan:
			log.Println("🛑 Scheduler parado por stopChan")
			return
		case <-ticker.C:
			// ✅ Executar com recovery individual
			s.safeExecute("checkAndTriggerCalls", s.checkAndTriggerCalls)
			s.safeExecute("checkMissedCalls", s.checkMissedCalls)
		case <-alertTicker.C:
			s.safeExecute("checkUnacknowledgedAlerts", s.checkUnacknowledgedAlerts)
		}
	}
}

// ✅ NOVO: Wrapper com recovery
func (s *Scheduler) safeExecute(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("🚨 PANIC em %s: %v", name, r)
			log.Printf("Stack trace: %s", debug.Stack())

			// Registrar erro no banco
			_, _ = s.db.Exec(`
				INSERT INTO system_errors (
					component,
					error_type,
					error_message,
					stack_trace,
					created_at
				) VALUES ($1, 'panic', $2, $3, NOW())
				ON CONFLICT DO NOTHING
			`, name, fmt.Sprintf("%v", r), string(debug.Stack()))
		}
	}()

	fn()
}

func (s *Scheduler) Stop() {
	close(s.stopChan)
}

func (s *Scheduler) checkAndTriggerCalls() {
	now := time.Now()
	query := `
		SELECT a.id, a.idoso_id, a.data_hora_agendada, i.device_token, i.nome
		FROM agendamentos a
		JOIN idosos i ON i.id = a.idoso_id
		WHERE a.status = 'agendado'
		  AND a.data_hora_agendada <= $1
		  AND i.ativo = true
		LIMIT 10
	`

	log.Printf("🔍 Buscando agendamentos para executar... (Server Time: %s)", now.Format(time.RFC3339))

	// DEBUG: Verificar se o DB responde e qual o horário dele
	var dbTime time.Time
	errDbTime := s.db.QueryRow("SELECT NOW()").Scan(&dbTime)
	if errDbTime != nil {
		log.Printf("❌ ERRO CRÍTICO: Falha ao verificar hora do banco: %v", errDbTime)
	} else {
		log.Printf("🕒 Horário do Banco (DB Time): %s", dbTime.Format(time.RFC3339))
	}

	rows, err := s.db.Query(query, now)
	if err != nil {
		log.Printf("❌ Erro na query do scheduler: %v", err)
		return
	}
	defer rows.Close()

	found := false

	for rows.Next() {
		var agendamentoID, idosoID int64
		var dataHora time.Time
		var deviceToken sql.NullString
		var nome string

		rows.Scan(&agendamentoID, &idosoID, &dataHora, &deviceToken, &nome)

		if !deviceToken.Valid || deviceToken.String == "" {
			log.Printf("⚠️  Sem device_token: %s", nome)
			s.updateStatus(agendamentoID, "falha_sem_token")
			continue
		}

		sessionID := fmt.Sprintf("call-%d-%d", agendamentoID, time.Now().Unix())

		err := s.pushService.SendCallNotification(deviceToken.String, sessionID, nome)
		if err != nil {
			if push.IsInvalidTokenError(err) {
				log.Printf("⚠️  Token inválido para: %s (%v)", nome, err)
				s.updateStatus(agendamentoID, "falha_token_invalido")

				// Marcar que o token precisa ser atualizado
				_, _ = s.db.Exec(`
					UPDATE idosos 
					SET device_token_valido = false, 
					    device_token_atualizado_em = NOW()
					WHERE id = $1
				`, idosoID)
			} else {
				log.Printf("❌ Erro ao enviar push: %s - %v", nome, err)
				s.updateStatus(agendamentoID, "falha_envio")
			}
			continue
		}

		log.Printf("📲 Push enviado: %s", nome)
		s.updateStatusWithTimestamp(agendamentoID, "em_andamento")
		found = true
	}

	if !found {
		log.Printf("📭 Nenhum agendamento encontrado para agora.")
	}
}

// checkMissedCalls verifica chamadas que ficaram "penduradas" (tocaram mas ninguém atendeu)
func (s *Scheduler) checkMissedCalls() {
	query := `
		SELECT a.id, a.idoso_id, i.nome, c.device_token, c.telefone, c.email
		FROM agendamentos a
		JOIN idosos i ON i.id = a.idoso_id
		LEFT JOIN (
			SELECT DISTINCT ON (idoso_id) idoso_id, device_token, telefone, email
			FROM cuidadores
			WHERE ativo = true AND device_token IS NOT NULL AND device_token != ''
			ORDER BY idoso_id, prioridade ASC
		) c ON c.idoso_id = i.id
		WHERE a.status = 'em_andamento' 
		  AND a.data_hora_agendada < (NOW() - INTERVAL '45 seconds')
	`

	rows, err := s.db.Query(query)
	if err != nil {
		log.Printf("❌ Erro ao verificar chamadas perdidas: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var agendamentoID, idosoID int64
		var nomeIdoso string
		var tokenCuidador, phoneCuidador, emailCuidador sql.NullString

		if err := rows.Scan(&agendamentoID, &idosoID, &nomeIdoso, &tokenCuidador, &phoneCuidador, &emailCuidador); err != nil {
			log.Printf("❌ Erro ao fazer scan de chamada perdida: %v", err)
			continue
		}

		log.Printf("⚠️ CHAMADA PERDIDA detectada para Idoso: %s (ID: %d)", nomeIdoso, idosoID)

		// 1. Atualizar status do agendamento
		_, errUpdate := s.db.Exec(`
			UPDATE agendamentos 
			SET status = 'nao_atendido', 
			    ultima_tentativa = NOW(),
			    tentativas_realizadas = tentativas_realizadas + 1
			WHERE id = $1
		`, agendamentoID)

		if errUpdate != nil {
			log.Printf("❌ Erro ao atualizar agendamento: %v", errUpdate)
			continue
		}

		// 2. Registrar no histórico de ligações
		var historicoID int64
		errHistorico := s.db.QueryRow(`
			INSERT INTO historico_ligacoes (
				agendamento_id,
				idoso_id,
				inicio_chamada,
				fim_chamada,
				duracao_segundos,
				tarefa_concluida,
				motivo_falha,
				transcricao_completa,
				criado_em
			) VALUES ($1, $2, NOW() - INTERVAL '45 seconds', NOW(), 45, false, $3, $4, NOW())
			RETURNING id
		`, agendamentoID, idosoID,
			"Chamada não atendida pelo idoso após 45 segundos",
			fmt.Sprintf("Push notification enviado mas não houve resposta do dispositivo. Idoso: %s", nomeIdoso),
		).Scan(&historicoID)

		if errHistorico != nil {
			log.Printf("⚠️ Erro ao registrar histórico: %v", errHistorico)
		} else {
			log.Printf("📝 Histórico registrado: ID %d", historicoID)
		}

		// 3. Criar alerta no sistema
		var alertID int64
		errAlerta := s.db.QueryRow(`
			INSERT INTO alertas (
				idoso_id,
				ligacao_id,
				tipo,
				severidade,
				mensagem,
				destinatarios,
				enviado,
				visualizado,
				data_envio,
				criado_em
			) VALUES ($1, $2, 'nao_atende_telefone', 'aviso', $3, $4, false, false, NOW(), NOW())
			RETURNING id
		`, idosoID, historicoID,
			fmt.Sprintf("%s não atendeu a chamada programada da EVA às %s",
				nomeIdoso, time.Now().Format("15:04")),
			`["cuidador"]`).Scan(&alertID)

		if errAlerta != nil {
			log.Printf("⚠️ Erro ao criar alerta: %v", errAlerta)
		}

		// 4. Registrar na timeline
		_, errTimeline := s.db.Exec(`
			INSERT INTO timeline (
				idoso_id,
				tipo,
				subtipo,
				titulo,
				descricao,
				data,
				criado_em
			) VALUES ($1, 'ligacao', 'nao_atendida', 'Chamada Não Atendida', $2, NOW(), NOW())
		`, idosoID,
			fmt.Sprintf("EVA tentou contato com %s mas a chamada não foi atendida.", nomeIdoso))

		if errTimeline != nil {
			log.Printf("⚠️ Erro ao registrar timeline: %v", errTimeline)
		}

		// 5. Notificar o cuidador via push notification
		if tokenCuidador.Valid && tokenCuidador.String != "" {
			errPush := s.pushService.SendMissedCallAlert(tokenCuidador.String, nomeIdoso)
			if errPush != nil {
				log.Printf("❌ Erro ao enviar push para cuidador: %v", errPush)

				// Marcar alerta para envio por outros meios
				_, _ = s.db.Exec(`
					UPDATE alertas 
					SET necessita_escalamento = true,
					    tempo_escalamento = NOW() + INTERVAL '5 minutes'
					WHERE id = $1
				`, alertID)
			} else {
				log.Printf("📵 Cuidador notificado sobre chamada perdida de %s", nomeIdoso)

				// Marcar alerta como enviado
				_, _ = s.db.Exec(`
					UPDATE alertas SET enviado = true WHERE id = $1
				`, alertID)
			}
			// 📩 ESCUDO DE SEGURANÇA: Tentar outros meios (Email) e Escalamento
			log.Printf("⚠️ Tentando meios alternativos para %s...", nomeIdoso)

			if emailCuidador.Valid && emailCuidador.String != "" && s.emailService != nil {
				subject := fmt.Sprintf("⚠️ Alerta de Chamada Perdida: %s", nomeIdoso)
				body := fmt.Sprintf(`
					<h2>Atenção! Chamada Não Atendida</h2>
					<p>A EVA tentou entrar em contato com <b>%s</b> hoje às %s, mas não houve resposta.</p>
					<p>Como não conseguimos enviar a notificação via aplicativo, estamos enviando este email de segurança.</p>
					<p>Por favor, verifique o bem-estar do idoso assim que possível.</p>
					<hr>
					<p><small>Este é um aviso automático gerado pelo sistema EVA-Mind.</small></p>
				`, nomeIdoso, time.Now().Format("15:04"))

				if errEmail := s.emailService.SendEmail(emailCuidador.String, subject, body); errEmail != nil {
					log.Printf("❌ Falha crítica ao enviar email de segurança para %s: %v", emailCuidador.String, errEmail)
				} else {
					log.Printf("📧 Email de segurança enviado com sucesso para %s", emailCuidador.String)
					// Marcar alerta como enviado (mesmo que por email)
					_, _ = s.db.Exec(`UPDATE alertas SET enviado = true WHERE id = $1`, alertID)
				}
			}

			// Forçar escalamento para que o painel de monitoramento destaque o problema
			log.Printf("🚨 Escalando alerta ID %d para monitoramento administrativo", alertID)
			_, _ = s.db.Exec(`
				UPDATE alertas 
				SET necessita_escalamento = true,
					tempo_escalamento = NOW()
				WHERE id = $1
			`, alertID)
		}

		log.Printf("✅ Chamada perdida processada completamente para %s", nomeIdoso)
	}
}

// checkUnacknowledgedAlerts verifica alertas críticos não visualizados
func (s *Scheduler) checkUnacknowledgedAlerts() {
	if err := gemini.CheckUnacknowledgedAlerts(s.db, s.pushService); err != nil {
		log.Printf("❌ Erro ao verificar alertas não visualizados: %v", err)
	}
}

func (s *Scheduler) updateStatus(id int64, status string) {
	_, err := s.db.Exec(`
		UPDATE agendamentos 
		SET status = $1, atualizado_em = NOW() 
		WHERE id = $2
	`, status, id)

	if err != nil {
		log.Printf("❌ Erro ao atualizar status: %v", err)
	}
}

func (s *Scheduler) updateStatusWithTimestamp(id int64, status string) {
	_, err := s.db.Exec(`
		UPDATE agendamentos 
		SET status = $1, 
		    ultima_tentativa = NOW(),
		    atualizado_em = NOW() 
		WHERE id = $2
	`, status, id)

	if err != nil {
		log.Printf("❌ Erro ao atualizar status: %v", err)
	}
}
