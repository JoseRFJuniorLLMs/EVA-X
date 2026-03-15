// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"eva/internal/brainstem/database"
	"eva/internal/brainstem/push"
	"eva/internal/motor/email"
	"fmt"
	"log"
	"sync"
	"time"

	"firebase.google.com/go/v4/messaging"
)

// PendingSchedule armazena um agendamento aguardando confirmacao
type PendingSchedule struct {
	Timestamp   string
	Tipo        string
	Description string
	CreatedAt   time.Time
}

// pendingSchedules armazena agendamentos pendentes por idoso_id
var pendingSchedules = make(map[int64]*PendingSchedule)
var pendingMu sync.RWMutex

// AlertFamily envia notificacao push para cuidadores com sistema de fallback
func AlertFamily(db *database.DB, pushService *push.FirebaseService, emailService *email.EmailService, idosoID int64, reason string) error {
	return AlertFamilyWithSeverity(db, pushService, emailService, idosoID, reason, "alta")
}

// AlertFamilyWithSeverity envia alertas com niveis de severidade
func AlertFamilyWithSeverity(db *database.DB, pushService *push.FirebaseService, emailService *email.EmailService, idosoID int64, reason, severity string) error {
	ctx := context.Background()

	// 1. Buscar todos os cuidadores ativos (primarios e secundarios)
	cuidadorRows, err := db.QueryByLabel(ctx, "cuidadores",
		" AND n.idoso_id = $idoso_id AND n.ativo = $ativo",
		map[string]interface{}{
			"idoso_id": idosoID,
			"ativo":    true,
		}, 0)
	if err != nil {
		return fmt.Errorf("failed to query caregivers: %w", err)
	}

	// Fetch the elder's name from idosos table
	idosoNode, err := db.GetNodeByID(ctx, "idosos", idosoID)
	if err != nil {
		return fmt.Errorf("failed to get idoso: %w", err)
	}

	var elderName string
	if idosoNode != nil {
		elderName = database.GetString(idosoNode, "nome")
	}

	type Caregiver struct {
		Token    string
		Phone    string
		Email    string
		Priority int64
	}

	var caregivers []Caregiver

	for _, m := range cuidadorRows {
		cg := Caregiver{
			Token:    database.GetString(m, "device_token"),
			Phone:    database.GetString(m, "telefone"),
			Email:    database.GetString(m, "email"),
			Priority: database.GetInt64(m, "prioridade"),
		}
		caregivers = append(caregivers, cg)
	}

	if len(caregivers) == 0 {
		log.Printf("WARNING: No active caregivers found for idoso %d", idosoID)
		return fmt.Errorf("no caregivers registered")
	}

	// 2. Registrar alerta no banco ANTES de enviar
	now := time.Now().Format(time.RFC3339)
	alertID, err := db.Insert(ctx, "alertas", map[string]interface{}{
		"idoso_id":    idosoID,
		"tipo":        "familia",
		"severidade":  severity,
		"mensagem":    reason,
		"visualizado": false,
		"criado_em":   now,
	})

	if err != nil {
		log.Printf("WARNING: Failed to log alert in database: %v", err)
	} else {
		log.Printf("Alert registered in DB with ID: %d", alertID)
	}

	// 3. Tentar enviar push notifications para todos os cuidadores
	var successCount int
	var tokens []string

	for _, cg := range caregivers {
		if cg.Token != "" {
			tokens = append(tokens, cg.Token)
		}
	}

	if len(tokens) > 0 {
		log.Printf("Enviando push para %d cuidador(es)", len(tokens))

		for _, token := range tokens {
			result, err := pushService.SendAlertNotification(token, elderName, reason)

			if err == nil && result.Success {
				successCount++

				// Registrar envio no banco
				if updErr := db.Update(ctx, "alertas",
					map[string]interface{}{"id": alertID},
					map[string]interface{}{
						"enviado":    true,
						"data_envio": time.Now().Format(time.RFC3339),
					}); updErr != nil {
					log.Printf("⚠️ Failed to record alert delivery state (id=%v): %v", alertID, updErr)
				}

				log.Printf("Alert sent successfully to caregiver for %s", elderName)
			} else {
				log.Printf("Failed to send alert to caregiver: %v", err)
			}
		}
	}

	// 4. Se NENHUM push funcionou, tentar fallbacks
	if successCount == 0 {
		log.Printf("WARNING: Nenhum push notification enviado com sucesso. Tentando fallbacks...")

		// Registrar que o alerta precisa de escalamento
		if updErr := db.Update(ctx, "alertas",
			map[string]interface{}{"id": alertID},
			map[string]interface{}{
				"necessita_escalamento": true,
				"ultima_tentativa":      time.Now().Format(time.RFC3339),
			}); updErr != nil {
			log.Printf("⚠️ Failed to mark alert for escalation (id=%v): %v", alertID, updErr)
		}

		// Fallback para Email
		if emailService != nil {
			for _, cg := range caregivers {
				if cg.Email != "" {
					subject := fmt.Sprintf("ALERTA DE EMERGENCIA (%s): %s", severity, elderName)
					body := fmt.Sprintf(`
						<h2>Atencao! Alerta de Emergencia Detectado</h2>
						<p>O sistema EVA-Mind detectou uma situacao de urgencia para <b>%s</b>.</p>
						<p><b>Motivo do Alerta:</b> %s</p>
						<hr>
						<p>Como nao conseguimos confirmar a entrega via aplicativo movel, este email de seguranca foi enviado.</p>
						<p>Por favor, verifique a situacao imediatamente.</p>
					`, elderName, reason)

					if errEmail := emailService.SendEmail(cg.Email, subject, body); errEmail != nil {
						log.Printf("Falha ao enviar email de fallback para %s: %v", cg.Email, errEmail)
					} else {
						log.Printf("Email de fallback enviado com sucesso para %s", cg.Email)
						successCount++
						// Marcar como enviado
						if updErr := db.Update(ctx, "alertas",
							map[string]interface{}{"id": alertID},
							map[string]interface{}{
								"enviado":    true,
								"data_envio": time.Now().Format(time.RFC3339),
							}); updErr != nil {
							log.Printf("⚠️ Failed to mark alert as sent via email (id=%v): %v", alertID, updErr)
						}
					}
				}
			}
		}

		if successCount == 0 {
			return fmt.Errorf("all notification methods (Push/Email) failed, alert needs immediate escalation")
		}
	}

	log.Printf("Alert sent to %d of %d caregivers", successCount, len(tokens))

	// 5. Para alertas criticos, marcar para escalonamento automatico
	if severity == "critica" {
		escalTime := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
		if updErr := db.Update(ctx, "alertas",
			map[string]interface{}{"id": alertID},
			map[string]interface{}{
				"necessita_escalamento": true,
				"tempo_escalamento":     escalTime,
			}); updErr != nil {
			log.Printf("⚠️ Failed to set critical alert escalation timer (id=%v): %v", alertID, updErr)
		}

		log.Printf("Alert critico - configurado para escalonamento em 5 minutos se nao visualizado")
	}

	return nil
}

// ConfirmMedication registra que o idoso tomou o remedio
func ConfirmMedication(db *database.DB, pushService *push.FirebaseService, idosoID int64, medicationName string) error {
	ctx := context.Background()

	// 1. Registrar no historico
	now := time.Now().Format(time.RFC3339)
	_, err := db.Insert(ctx, "historico_medicamentos", map[string]interface{}{
		"idoso_id":    idosoID,
		"medicamento": medicationName,
		"tomado_em":   now,
	})

	if err != nil {
		return fmt.Errorf("failed to log medication: %w", err)
	}

	log.Printf("Medication logged: %d took %s", idosoID, medicationName)

	// 2. Atualizar status do agendamento de hoje
	// Query agendamentos for this idoso that are in progress today
	todayStr := time.Now().Format("2006-01-02")
	agRows, err := db.QueryByLabel(ctx, "agendamentos",
		" AND n.idoso_id = $idoso_id AND n.status = $status",
		map[string]interface{}{
			"idoso_id": idosoID,
			"status":   "em_andamento",
		}, 0)
	if err != nil {
		log.Printf("WARNING: Failed to query agendamentos: %v", err)
	} else {
		for _, ag := range agRows {
			dataHora := database.GetString(ag, "data_hora_agendada")
			// Check if the agendamento is for today
			if len(dataHora) >= 10 && dataHora[:10] == todayStr {
				agID := database.GetInt64(ag, "id")
				if updErr := db.Update(ctx, "agendamentos",
					map[string]interface{}{"id": agID},
					map[string]interface{}{
						"medicamento_tomado": true,
						"status":             "concluido",
					}); updErr != nil {
					log.Printf("⚠️ Failed to mark medication schedule as completed (agendamento_id=%d): %v", agID, updErr)
				}
			}
		}
	}

	// 3. Notificar TODOS os cuidadores ativos
	cuidadorRows, err := db.QueryByLabel(ctx, "cuidadores",
		" AND n.idoso_id = $idoso_id AND n.ativo = $ativo",
		map[string]interface{}{
			"idoso_id": idosoID,
			"ativo":    true,
		}, 0)
	if err != nil {
		log.Printf("WARNING: Failed to query caregivers: %v", err)
		return nil
	}

	// Fetch elder name
	idosoNode, _ := db.GetNodeByID(ctx, "idosos", idosoID)
	var elderName string
	if idosoNode != nil {
		elderName = database.GetString(idosoNode, "nome")
	}

	notificationsSent := 0

	for _, m := range cuidadorRows {
		token := database.GetString(m, "device_token")
		if token == "" {
			continue
		}

		message := &messaging.Message{
			Token: token,
			Notification: &messaging.Notification{
				Title: "Medicamento Confirmado",
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

		pushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = pushService.GetClient().Send(pushCtx, message)
		if err != nil {
			log.Printf("WARNING: Failed to notify caregiver: %v", err)
		} else {
			notificationsSent++
		}
	}

	if notificationsSent > 0 {
		log.Printf("%d caregiver(s) notified about medication", notificationsSent)
	}

	return nil
}

// ScheduleAppointment insere um novo agendamento no banco de dados
func ScheduleAppointment(db *database.DB, idosoID int64, timestampStr, tipo, descricao string) error {
	ctx := context.Background()

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
		return fmt.Errorf("formato de data invalido (%s): %w", timestampStr, err)
	}

	// 2. Preparar dados_tarefa como JSON
	dadosJSON, err := json.Marshal(map[string]string{
		"description":      descricao,
		"original_request": timestampStr,
	})
	if err != nil {
		// Fallback para JSON vazio valido se der erro no marshal
		dadosJSON = []byte("{}")
	}

	// 3. Inserir no banco
	now := time.Now().Format(time.RFC3339)
	id, err := db.Insert(ctx, "agendamentos", map[string]interface{}{
		"idoso_id":              idosoID,
		"tipo":                  tipo,
		"data_hora_agendada":    dataHora.Format(time.RFC3339),
		"status":                "agendado",
		"prioridade":            "media",
		"dados_tarefa":          string(dadosJSON),
		"criado_em":             now,
		"atualizado_em":         now,
		"max_retries":           int64(3),
		"tentativas_realizadas": int64(0),
	})
	if err != nil {
		return fmt.Errorf("failed to insert appointment: %w", err)
	}

	log.Printf("Appointment scheduled: ID %d for Idoso %d at %s", id, idosoID, dataHora)
	return nil
}

// StorePendingSchedule armazena um agendamento pendente aguardando confirmacao
// Retorna uma mensagem para EVA pedir confirmacao ao usuario
func StorePendingSchedule(idosoID int64, timestampStr, tipo, description string) string {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	pendingSchedules[idosoID] = &PendingSchedule{
		Timestamp:   timestampStr,
		Tipo:        tipo,
		Description: description,
		CreatedAt:   time.Now(),
	}

	// Parse para mostrar horario amigavel
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		timestamp, _ = time.Parse("2006-01-02 15:04:05", timestampStr)
	}

	horaFormatada := timestamp.Format("15:04")
	log.Printf("Agendamento pendente armazenado para idoso %d: %s as %s", idosoID, description, horaFormatada)

	// Retorna mensagem para EVA pedir confirmacao
	return fmt.Sprintf("[[CONFIRM_SCHEDULE:%s|%s|%s]]", horaFormatada, tipo, description)
}

// ConfirmPendingSchedule confirma ou cancela um agendamento pendente
func ConfirmPendingSchedule(db *database.DB, idosoID int64, confirmed bool) (bool, string, error) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	pending, exists := pendingSchedules[idosoID]
	if !exists {
		log.Printf("WARNING: Nenhum agendamento pendente para idoso %d", idosoID)
		return false, "", nil
	}

	// Remove o pendente
	delete(pendingSchedules, idosoID)

	if !confirmed {
		log.Printf("Agendamento cancelado pelo usuario: %s", pending.Description)
		return false, pending.Description, nil
	}

	// Confirma - executa o agendamento
	log.Printf("Agendamento confirmado: %s", pending.Description)

	err := ScheduleAppointment(db, idosoID, pending.Timestamp, pending.Tipo, pending.Description)
	if err != nil {
		return false, pending.Description, err
	}

	return true, pending.Description, nil
}

// HasPendingSchedule verifica se ha agendamento pendente para um idoso
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
			log.Printf("Limpando agendamento pendente expirado para idoso %d", id)
			delete(pendingSchedules, id)
		}
	}
}

// InteracaoRisco representa uma interacao perigosa entre medicamentos
type InteracaoRisco struct {
	MedicamentoA    string
	MedicamentoB    string
	NivelPerigo     string // MODERADO, GRAVE, FATAL
	MensagemAlerta  string
	AcaoRecomendada string
}

// CheckMedicationInteractions verifica se um medicamento tem interacoes perigosas
// com os medicamentos atuais do idoso
func CheckMedicationInteractions(db *database.DB, idosoID int64, novoMedicamento string) ([]InteracaoRisco, error) {
	ctx := context.Background()
	log.Printf("[SAFETY] Verificando interacoes para: %s (Idoso: %d)", novoMedicamento, idosoID)

	// 1. Get active medications for this idoso
	medRows, err := db.QueryByLabel(ctx, "medicamentos",
		" AND n.idoso_id = $idoso_id AND n.ativo = $ativo",
		map[string]interface{}{
			"idoso_id": idosoID,
			"ativo":    true,
		}, 0)
	if err != nil {
		log.Printf("WARNING: [SAFETY] Erro ao buscar medicamentos ativos: %v", err)
		return nil, err
	}

	if len(medRows) == 0 {
		log.Printf("[SAFETY] Nenhuma interacao perigosa detectada (sem medicamentos ativos)")
		return nil, nil
	}

	// 2. For each active medication, get its catalogo_ref_id
	// and look up interactions in interacoes_risco
	var interacoes []InteracaoRisco

	// Get the catalogo entry for the new medication
	novoCatRows, err := db.QueryByLabel(ctx, "catalogo_farmaceutico", "", nil, 0)
	if err != nil {
		log.Printf("WARNING: [SAFETY] Erro ao buscar catalogo farmaceutico: %v", err)
		return nil, err
	}

	// Find catalogo IDs matching the new medication name
	var novoCatalogoIDs []int64
	novoMedLower := fmt.Sprintf("%s", novoMedicamento)
	for _, cat := range novoCatRows {
		nomeComercial := database.GetString(cat, "nome_comercial")
		principioAtivo := database.GetString(cat, "principio_ativo")
		if containsIgnoreCase(nomeComercial, novoMedLower) || containsIgnoreCase(principioAtivo, novoMedLower) {
			novoCatalogoIDs = append(novoCatalogoIDs, database.GetInt64(cat, "id"))
		}
	}

	if len(novoCatalogoIDs) == 0 {
		log.Printf("[SAFETY] Nenhuma interacao perigosa detectada (medicamento nao encontrado no catalogo)")
		return nil, nil
	}

	// Get all interaction rules
	interacoesRows, err := db.QueryByLabel(ctx, "interacoes_risco", "", nil, 0)
	if err != nil {
		log.Printf("WARNING: [SAFETY] Erro ao buscar interacoes_risco: %v", err)
		return nil, err
	}

	// For each active medication, check interactions
	for _, medRow := range medRows {
		catRefID := database.GetInt64(medRow, "catalogo_ref_id")
		medNome := database.GetString(medRow, "nome")

		for _, ir := range interacoesRows {
			catIDA := database.GetInt64(ir, "catalogo_id_a")
			catIDB := database.GetInt64(ir, "catalogo_id_b")
			nivelPerigo := database.GetString(ir, "nivel_perigo")

			// Only check GRAVE and FATAL
			if nivelPerigo != "GRAVE" && nivelPerigo != "FATAL" {
				continue
			}

			for _, novoID := range novoCatalogoIDs {
				if (catIDA == catRefID && catIDB == novoID) || (catIDA == novoID && catIDB == catRefID) {
					acaoRecomendada := database.GetString(ir, "acao_recomendada")
					if acaoRecomendada == "" {
						acaoRecomendada = "Consultar medico imediatamente"
					}

					interacao := InteracaoRisco{
						MedicamentoA:    medNome,
						MedicamentoB:    novoMedicamento,
						NivelPerigo:     nivelPerigo,
						MensagemAlerta:  database.GetString(ir, "mensagem_alerta"),
						AcaoRecomendada: acaoRecomendada,
					}
					interacoes = append(interacoes, interacao)
					log.Printf("ALERT: [SAFETY] INTERACAO %s DETECTADA: %s + %s", nivelPerigo, medNome, novoMedicamento)
				}
			}
		}
	}

	// Sort: FATAL first, then GRAVE
	sortInteracoes(interacoes)

	if len(interacoes) > 0 {
		log.Printf("ALERT: [SAFETY] %d interacoes perigosas encontradas!", len(interacoes))
	} else {
		log.Printf("[SAFETY] Nenhuma interacao perigosa detectada")
	}

	return interacoes, nil
}

// sortInteracoes sorts interactions with FATAL first, then GRAVE
func sortInteracoes(interacoes []InteracaoRisco) {
	for i := 0; i < len(interacoes); i++ {
		for j := i + 1; j < len(interacoes); j++ {
			if nivelPriority(interacoes[j].NivelPerigo) < nivelPriority(interacoes[i].NivelPerigo) {
				interacoes[i], interacoes[j] = interacoes[j], interacoes[i]
			}
		}
	}
}

func nivelPriority(nivel string) int {
	switch nivel {
	case "FATAL":
		return 1
	case "GRAVE":
		return 2
	default:
		return 3
	}
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	sLower := make([]byte, len(s))
	subLower := make([]byte, len(substr))
	for i := range s {
		if s[i] >= 'A' && s[i] <= 'Z' {
			sLower[i] = s[i] + 32
		} else {
			sLower[i] = s[i]
		}
	}
	for i := range substr {
		if substr[i] >= 'A' && substr[i] <= 'Z' {
			subLower[i] = substr[i] + 32
		} else {
			subLower[i] = substr[i]
		}
	}
	return len(subLower) > 0 && bytesContains(sLower, subLower)
}

func bytesContains(s, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := range sub {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// FormatInteractionWarning formata alerta de interacao para EVA falar
func FormatInteractionWarning(interacoes []InteracaoRisco) string {
	if len(interacoes) == 0 {
		return ""
	}

	// Priorizar FATAL
	for _, ir := range interacoes {
		if ir.NivelPerigo == "FATAL" {
			return fmt.Sprintf("[[BLOCKED:FATAL]] ATENCAO! Nao posso agendar %s porque pode causar uma interacao FATAL com %s que voce ja toma. %s. %s",
				ir.MedicamentoB, ir.MedicamentoA, ir.MensagemAlerta, ir.AcaoRecomendada)
		}
	}

	// Se nao tem FATAL, pegar GRAVE
	ir := interacoes[0]
	return fmt.Sprintf("[[BLOCKED:GRAVE]] Cuidado! %s pode ter uma interacao GRAVE com %s. %s. Recomendo: %s",
		ir.MedicamentoB, ir.MedicamentoA, ir.MensagemAlerta, ir.AcaoRecomendada)
}
