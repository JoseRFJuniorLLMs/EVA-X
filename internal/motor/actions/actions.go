package actions

import (
	"context"
	"database/sql"
	"encoding/json"
	"eva-mind/internal/brainstem/push"
	"eva-mind/internal/motor/email"
	"fmt"
	"log"
	"sync"
	"time"

	"firebase.google.com/go/v4/messaging"
)

// PendingSchedule armazena um agendamento aguardando confirmação
type PendingSchedule struct {
	Timestamp   string
	Tipo        string
	Description string
	CreatedAt   time.Time
}

// pendingSchedules armazena agendamentos pendentes por idoso_id
var pendingSchedules = make(map[int64]*PendingSchedule)
var pendingMu sync.RWMutex

// AlertFamily envia notificação push para cuidadores com sistema de fallback
func AlertFamily(db *sql.DB, pushService *push.FirebaseService, emailService *email.EmailService, idosoID int64, reason string) error {
	return AlertFamilyWithSeverity(db, pushService, emailService, idosoID, reason, "alta")
}

// AlertFamilyWithSeverity envia alertas com níveis de severidade
func AlertFamilyWithSeverity(db *sql.DB, pushService *push.FirebaseService, emailService *email.EmailService, idosoID int64, reason, severity string) error {
	// 1. Buscar todos os cuidadores ativos (primários e secundários)
	query := `
		SELECT 
			c.device_token, 
			c.telefone,
			c.email,
			c.prioridade,
			i.nome 
		FROM cuidadores c
		JOIN idosos i ON i.id = c.idoso_id
		WHERE c.idoso_id = $1 AND c.ativo = true
		ORDER BY c.prioridade ASC
	`

	rows, err := db.Query(query, idosoID)
	if err != nil {
		return fmt.Errorf("failed to query caregivers: %w", err)
	}
	defer rows.Close()

	type Caregiver struct {
		Token     sql.NullString
		Phone     sql.NullString
		Email     sql.NullString
		Priority  int
		ElderName string
	}

	var caregivers []Caregiver

	for rows.Next() {
		var cg Caregiver
		err := rows.Scan(&cg.Token, &cg.Phone, &cg.Email, &cg.Priority, &cg.ElderName)
		if err != nil {
			log.Printf("Error scanning caregiver: %v", err)
			continue
		}
		caregivers = append(caregivers, cg)
	}

	if len(caregivers) == 0 {
		log.Printf("⚠️ No active caregivers found for idoso %d", idosoID)
		return fmt.Errorf("no caregivers registered")
	}

	elderName := caregivers[0].ElderName

	// 2. Registrar alerta no banco ANTES de enviar
	var alertID int64
	insertQuery := `
		INSERT INTO alertas (
			idoso_id, 
			tipo, 
			severidade,
			mensagem, 
			visualizado,
			criado_em
		) 
		VALUES ($1, 'familia', $2, $3, false, NOW())
		RETURNING id
	`

	err = db.QueryRow(insertQuery, idosoID, severity, reason).Scan(&alertID)
	if err != nil {
		log.Printf("⚠️ Failed to log alert in database: %v", err)
	} else {
		log.Printf("📝 Alert registered in DB with ID: %d", alertID)
	}

	// 3. Tentar enviar push notifications para todos os cuidadores
	var successCount int
	var tokens []string

	for _, cg := range caregivers {
		if cg.Token.Valid && cg.Token.String != "" {
			tokens = append(tokens, cg.Token.String)
		}
	}

	if len(tokens) > 0 {
		log.Printf("📱 Enviando push para %d cuidador(es)", len(tokens))

		for _, token := range tokens {
			result, err := pushService.SendAlertNotification(token, elderName, reason)

			if err == nil && result.Success {
				successCount++

				// Registrar envio no banco
				_, _ = db.Exec(`
					UPDATE alertas 
					SET enviado = true, data_envio = NOW()
					WHERE id = $1
				`, alertID)

				log.Printf("✅ Alert sent successfully to caregiver for %s", elderName)
			} else {
				log.Printf("❌ Failed to send alert to caregiver: %v", err)
			}
		}
	}

	// 4. Se NENHUM push funcionou, tentar fallbacks
	if successCount == 0 {
		log.Printf("⚠️ Nenhum push notification enviado com sucesso. Tentando fallbacks...")

		// Registrar que o alerta precisa de escalamento
		_, _ = db.Exec(`
			UPDATE alertas 
			SET 
				necessita_escalamento = true,
				tentativas_envio = tentativas_envio + 1,
				ultima_tentativa = NOW()
			WHERE id = $1
		`, alertID)

		// 📧 ESCUDO DE SEGURANÇA: Fallback para Email
		if emailService != nil {
			for _, cg := range caregivers {
				if cg.Email.Valid && cg.Email.String != "" {
					subject := fmt.Sprintf("🚨 ALERTA DE EMERGÊNCIA (%s): %s", severity, elderName)
					body := fmt.Sprintf(`
						<h2>Atenção! Alerta de Emergência Detectado</h2>
						<p>O sistema EVA-Mind detectou uma situação de urgência para <b>%s</b>.</p>
						<p><b>Motivo do Alerta:</b> %s</p>
						<hr>
						<p>Como não conseguimos confirmar a entrega via aplicativo móvel, este email de segurança foi enviado.</p>
						<p>Por favor, verifique a situação imediatamente.</p>
					`, elderName, reason)

					if errEmail := emailService.SendEmail(cg.Email.String, subject, body); errEmail != nil {
						log.Printf("❌ Falha ao enviar email de fallback para %s: %v", cg.Email.String, errEmail)
					} else {
						log.Printf("📧 Email de fallback enviado com sucesso para %s", cg.Email.String)
						successCount++
						// Marcar como enviado
						_, _ = db.Exec(`UPDATE alertas SET enviado = true, data_envio = NOW() WHERE id = $1`, alertID)
					}
				}
			}
		}

		if successCount == 0 {
			return fmt.Errorf("all notification methods (Push/Email) failed, alert needs immediate escalation")
		}
	}

	log.Printf("✅ Alert sent to %d of %d caregivers", successCount, len(tokens))

	// 5. Para alertas críticos, marcar para escalonamento automático
	if severity == "critica" {
		_, _ = db.Exec(`
			UPDATE alertas 
			SET 
				necessita_escalamento = true,
				tempo_escalamento = NOW() + INTERVAL '5 minutes'
			WHERE id = $1
		`, alertID)

		log.Printf("🚨 Alert crítico - configurado para escalonamento em 5 minutos se não visualizado")
	}

	return nil
}

// ConfirmMedication registra que o idoso tomou o remédio
func ConfirmMedication(db *sql.DB, pushService *push.FirebaseService, idosoID int64, medicationName string) error {
	// 1. Registrar no histórico
	_, err := db.Exec(`
		INSERT INTO historico_medicamentos (idoso_id, medicamento, tomado_em) 
		VALUES ($1, $2, NOW())
	`, idosoID, medicationName)

	if err != nil {
		return fmt.Errorf("failed to log medication: %w", err)
	}

	log.Printf("💊 Medication logged: %d took %s", idosoID, medicationName)

	// 2. Atualizar status do agendamento de hoje
	_, err = db.Exec(`
		UPDATE agendamentos 
		SET medicamento_confirmado = true, 
		    status = 'concluido'
		WHERE idoso_id = $1 
		  AND DATE(data_hora_agendada) = CURRENT_DATE
		  AND status = 'em_andamento'
	`, idosoID)

	if err != nil {
		log.Printf("⚠️ Failed to update schedule: %v", err)
	}

	// 3. Notificar TODOS os cuidadores ativos
	query := `
		SELECT c.device_token, i.nome 
		FROM cuidadores c
		JOIN idosos i ON i.id = c.idoso_id
		WHERE c.idoso_id = $1 AND c.ativo = true
	`

	rows, err := db.Query(query, idosoID)
	if err != nil {
		log.Printf("⚠️ Failed to query caregivers: %v", err)
		return nil
	}
	defer rows.Close()

	var elderName string
	notificationsSent := 0

	for rows.Next() {
		var token sql.NullString
		err := rows.Scan(&token, &elderName)

		if err != nil || !token.Valid || token.String == "" {
			continue
		}

		message := &messaging.Message{
			Token: token.String,
			Notification: &messaging.Notification{
				Title: "✅ Medicamento Confirmado",
				Body:  fmt.Sprintf("%s tomou %s", elderName, medicationName),
			},
			Data: map[string]string{
				"type":       "medication_confirmed",
				"medication": medicationName,
				"idosoId":    fmt.Sprintf("%d", idosoID),
				"timestamp":  fmt.Sprintf("%d", time.Now().Unix()),
			},
			Android: &messaging.AndroidConfig{
				Priority: "normal",
				Notification: &messaging.AndroidNotification{
					Sound:        "default",
					ChannelID:    "eva_medications",
					DefaultSound: true,
					Color:        "#00FF00",
				},
			},
		}

		// ✅ Criar contexto local com timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = pushService.GetClient().Send(ctx, message)
		if err != nil {
			log.Printf("⚠️ Failed to notify caregiver: %v", err)
		} else {
			notificationsSent++
		}
	}

	if notificationsSent > 0 {
		log.Printf("✅ %d caregiver(s) notified about medication", notificationsSent)
	}

	return nil
}

// ScheduleAppointment insere um novo agendamento no banco de dados
func ScheduleAppointment(db *sql.DB, idosoID int64, timestampStr, tipo, descricao string) error {
	// 1. Parse convertendo string ISO para time.Time
	// Suporta formatos ISO parciais ou completos
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	var dataHora time.Time
	var err error

	for _, layout := range layouts {
		dataHora, err = time.Parse(layout, timestampStr)
		if err == nil {
			break
		}
	}

	if err != nil {
		return fmt.Errorf("formato de data inválido (%s): %w", timestampStr, err)
	}

	// 2. Preparar dados_tarefa como JSON
	dadosJSON, err := json.Marshal(map[string]string{
		"description":      descricao,
		"original_request": timestampStr,
	})
	if err != nil {
		// Fallback para JSON vazio válido se der erro no marshal
		dadosJSON = []byte("{}")
	}

	// 3. Inserir no banco
	query := `
		INSERT INTO agendamentos (
			idoso_id, 
			tipo, 
			data_hora_agendada, 
			status, 
			prioridade, 
			dados_tarefa, 
			criado_em, 
			atualizado_em,
			max_retries,
			tentativas_realizadas
		) 
		VALUES ($1, $2, $3, 'agendado', 'media', $4, NOW(), NOW(), 3, 0)
		RETURNING id
	`

	var id int64
	err = db.QueryRow(query, idosoID, tipo, dataHora, dadosJSON).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to insert appointment: %w", err)
	}

	log.Printf("📅 Appointment scheduled: ID %d for Idoso %d at %s", id, idosoID, dataHora)
	return nil
}

// StorePendingSchedule armazena um agendamento pendente aguardando confirmação
// Retorna uma mensagem para EVA pedir confirmação ao usuário
func StorePendingSchedule(idosoID int64, timestampStr, tipo, description string) string {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	pendingSchedules[idosoID] = &PendingSchedule{
		Timestamp:   timestampStr,
		Tipo:        tipo,
		Description: description,
		CreatedAt:   time.Now(),
	}

	// Parse para mostrar horário amigável
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		timestamp, _ = time.Parse("2006-01-02 15:04:05", timestampStr)
	}

	horaFormatada := timestamp.Format("15:04")
	log.Printf("⏳ Agendamento pendente armazenado para idoso %d: %s às %s", idosoID, description, horaFormatada)

	// Retorna mensagem para EVA pedir confirmação
	return fmt.Sprintf("[[CONFIRM_SCHEDULE:%s|%s|%s]]", horaFormatada, tipo, description)
}

// ConfirmPendingSchedule confirma ou cancela um agendamento pendente
func ConfirmPendingSchedule(db *sql.DB, idosoID int64, confirmed bool) (bool, string, error) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	pending, exists := pendingSchedules[idosoID]
	if !exists {
		log.Printf("⚠️ Nenhum agendamento pendente para idoso %d", idosoID)
		return false, "", nil
	}

	// Remove o pendente
	delete(pendingSchedules, idosoID)

	if !confirmed {
		log.Printf("❌ Agendamento cancelado pelo usuário: %s", pending.Description)
		return false, pending.Description, nil
	}

	// Confirma - executa o agendamento
	log.Printf("✅ Agendamento confirmado: %s", pending.Description)

	err := ScheduleAppointment(db, idosoID, pending.Timestamp, pending.Tipo, pending.Description)
	if err != nil {
		return false, pending.Description, err
	}

	return true, pending.Description, nil
}

// HasPendingSchedule verifica se há agendamento pendente para um idoso
func HasPendingSchedule(idosoID int64) bool {
	pendingMu.RLock()
	defer pendingMu.RUnlock()
	_, exists := pendingSchedules[idosoID]
	return exists
}

// GetPendingSchedule retorna o agendamento pendente de um idoso
func GetPendingSchedule(idosoID int64) *PendingSchedule {
	pendingMu.RLock()
	defer pendingMu.RUnlock()
	return pendingSchedules[idosoID]
}

// CleanExpiredPendingSchedules limpa agendamentos pendentes expirados (mais de 5 minutos)
func CleanExpiredPendingSchedules() {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	now := time.Now()
	for id, pending := range pendingSchedules {
		if now.Sub(pending.CreatedAt) > 5*time.Minute {
			log.Printf("🧹 Limpando agendamento pendente expirado para idoso %d", id)
			delete(pendingSchedules, id)
		}
	}
}

// InteracaoRisco representa uma interação perigosa entre medicamentos
type InteracaoRisco struct {
	MedicamentoA     string
	MedicamentoB     string
	NivelPerigo      string // MODERADO, GRAVE, FATAL
	MensagemAlerta   string
	AcaoRecomendada  string
}

// CheckMedicationInteractions verifica se um medicamento tem interações perigosas
// com os medicamentos atuais do idoso
func CheckMedicationInteractions(db *sql.DB, idosoID int64, novoMedicamento string) ([]InteracaoRisco, error) {
	log.Printf("🔍 [SAFETY] Verificando interações para: %s (Idoso: %d)", novoMedicamento, idosoID)

	query := `
		SELECT
			m_atual.nome AS medicamento_atual,
			ir.nivel_perigo,
			ir.mensagem_alerta,
			COALESCE(ir.acao_recomendada, 'Consultar médico imediatamente') AS acao_recomendada
		FROM medicamentos m_atual
		JOIN catalogo_farmaceutico cf_atual ON m_atual.catalogo_ref_id = cf_atual.id
		JOIN catalogo_farmaceutico cf_novo ON (
			LOWER(cf_novo.nome_comercial) LIKE LOWER('%' || $2 || '%')
			OR LOWER(cf_novo.principio_ativo) LIKE LOWER('%' || $2 || '%')
		)
		JOIN interacoes_risco ir ON (
			(ir.catalogo_id_a = cf_atual.id AND ir.catalogo_id_b = cf_novo.id)
			OR (ir.catalogo_id_a = cf_novo.id AND ir.catalogo_id_b = cf_atual.id)
		)
		WHERE m_atual.idoso_id = $1
		  AND m_atual.ativo = true
		  AND ir.nivel_perigo IN ('GRAVE', 'FATAL')
		ORDER BY
			CASE ir.nivel_perigo
				WHEN 'FATAL' THEN 1
				WHEN 'GRAVE' THEN 2
				ELSE 3
			END
	`

	rows, err := db.Query(query, idosoID, novoMedicamento)
	if err != nil {
		log.Printf("⚠️ [SAFETY] Erro ao verificar interações: %v", err)
		return nil, err
	}
	defer rows.Close()

	var interacoes []InteracaoRisco
	for rows.Next() {
		var ir InteracaoRisco
		ir.MedicamentoB = novoMedicamento
		if err := rows.Scan(&ir.MedicamentoA, &ir.NivelPerigo, &ir.MensagemAlerta, &ir.AcaoRecomendada); err != nil {
			log.Printf("⚠️ [SAFETY] Erro ao ler interação: %v", err)
			continue
		}
		interacoes = append(interacoes, ir)
		log.Printf("🚨 [SAFETY] INTERAÇÃO %s DETECTADA: %s + %s", ir.NivelPerigo, ir.MedicamentoA, ir.MedicamentoB)
	}

	if len(interacoes) > 0 {
		log.Printf("⛔ [SAFETY] %d interações perigosas encontradas!", len(interacoes))
	} else {
		log.Printf("✅ [SAFETY] Nenhuma interação perigosa detectada")
	}

	return interacoes, nil
}

// FormatInteractionWarning formata alerta de interação para EVA falar
func FormatInteractionWarning(interacoes []InteracaoRisco) string {
	if len(interacoes) == 0 {
		return ""
	}

	// Priorizar FATAL
	for _, ir := range interacoes {
		if ir.NivelPerigo == "FATAL" {
			return fmt.Sprintf("[[BLOCKED:FATAL]] ATENÇÃO! Não posso agendar %s porque pode causar uma interação FATAL com %s que você já toma. %s. %s",
				ir.MedicamentoB, ir.MedicamentoA, ir.MensagemAlerta, ir.AcaoRecomendada)
		}
	}

	// Se não tem FATAL, pegar GRAVE
	ir := interacoes[0]
	return fmt.Sprintf("[[BLOCKED:GRAVE]] Cuidado! %s pode ter uma interação GRAVE com %s. %s. Recomendo: %s",
		ir.MedicamentoB, ir.MedicamentoA, ir.MensagemAlerta, ir.AcaoRecomendada)
}
