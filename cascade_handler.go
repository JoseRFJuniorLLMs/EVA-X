// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"firebase.google.com/go/v4/messaging"
)

// handleVideoCascade gerencia a cascata de notificações para chamada de vídeo
// Prioridades: 1=Família, 2=Cuidador, 3=Médico
// Tenta 5x cada nível antes de escalar
// ctx controls the goroutine lifetime — returns early if the context is cancelled.
func (s *SignalingServer) handleVideoCascade(ctx context.Context, idosoID int64, sessionID string) {
	log.Printf("🌊 Iniciando cascata de vídeo para idoso %d, sessão %s", idosoID, sessionID)

	// Buscar todos os cuidadores ativos ordenados por prioridade
	query := `
		SELECT device_token, prioridade, nome, telefone
		FROM cuidadores
		WHERE idoso_id = $1 AND ativo = true
		ORDER BY prioridade ASC
	`

	if s.db == nil {
		log.Printf("❌ Database não inicializada")
		s.escalateToEmergency(idosoID, sessionID, "Database não inicializada")
		return
	}
	conn := s.db.GetConnection()
	if conn == nil {
		log.Printf("❌ Conexão com banco indisponível")
		s.escalateToEmergency(idosoID, sessionID, "Conexão com banco indisponível")
		return
	}

	rows, err := conn.Query(query, idosoID)
	if err != nil {
		log.Printf("❌ Erro ao buscar cuidadores: %v", err)
		s.escalateToEmergency(idosoID, sessionID, "Erro ao buscar cuidadores")
		return
	}
	defer rows.Close()

	type Caregiver struct {
		Token    sql.NullString
		Priority int
		Name     string
		Phone    sql.NullString
	}

	var caregivers []Caregiver
	for rows.Next() {
		var cg Caregiver
		if err := rows.Scan(&cg.Token, &cg.Priority, &cg.Name, &cg.Phone); err != nil {
			log.Printf("❌ Erro ao ler cuidador: %v", err)
			continue
		}
		caregivers = append(caregivers, cg)
	}
	if err := rows.Err(); err != nil {
		log.Printf("❌ Erro na iteração de cuidadores: %v", err)
	}

	if len(caregivers) == 0 {
		log.Printf("⚠️ Nenhum cuidador encontrado, escalando para emergência")
		s.escalateToEmergency(idosoID, sessionID, "Sem cuidadores cadastrados")
		return
	}

	// Agrupar por prioridade
	priorityGroups := make(map[int][]Caregiver)
	for _, cg := range caregivers {
		priorityGroups[cg.Priority] = append(priorityGroups[cg.Priority], cg)
	}

	// Tentar cada nível de prioridade
	priorities := []int{1, 2, 3} // Família, Cuidador, Médico
	priorityNames := map[int]string{
		1: "Família",
		2: "Cuidador",
		3: "Médico",
	}

	for _, priority := range priorities {
		group, exists := priorityGroups[priority]
		if !exists || len(group) == 0 {
			log.Printf("⏭️ Prioridade %d (%s) não tem contatos, pulando", priority, priorityNames[priority])
			continue
		}

		log.Printf("📞 Tentando prioridade %d (%s) - %d contato(s)", priority, priorityNames[priority], len(group))

		// Tentar 5 vezes para este nível
		for attempt := 1; attempt <= 5; attempt++ {
			log.Printf("🔔 Tentativa %d/5 para %s", attempt, priorityNames[priority])

			// Enviar notificação para todos os contatos deste nível
			for _, cg := range group {
				if !cg.Token.Valid || cg.Token.String == "" {
					log.Printf("⚠️ %s (%s) sem token FCM", cg.Name, priorityNames[priority])
					continue
				}

				err := s.sendVideoCallNotification(cg.Token.String, sessionID, cg.Name, priorityNames[priority])
				if err != nil {
					log.Printf("❌ Erro ao enviar notificação para %s: %v", cg.Name, err)
				} else {
					log.Printf("✅ Notificação enviada para %s (%s)", cg.Name, priorityNames[priority])
				}
			}

			// Aguardar 30 segundos antes de verificar se alguém atendeu
			select {
			case <-ctx.Done():
				log.Printf("⚠️ Cascade cancelled for session %s: %v", sessionID, ctx.Err())
				return
			case <-time.After(30 * time.Second):
			}

			// Verificar se a sessão foi aceita (status mudou para 'active')
			session, err := s.db.GetVideoSession(sessionID)
			if err == nil && session.Status == "active" {
				log.Printf("✅ Chamada aceita por %s! Cascata finalizada.", priorityNames[priority])
				return
			}
		}

		log.Printf("⏭️ %s não atendeu após 5 tentativas, escalando...", priorityNames[priority])
	}

	// Se chegou aqui, ninguém atendeu
	log.Printf("🚨 NENHUM CONTATO ATENDEU - Escalando para EMERGÊNCIA")
	s.escalateToEmergency(idosoID, sessionID, "Nenhum contato atendeu após 5 tentativas em cada nível")
}

// sendVideoCallNotification envia notificação push para chamada de vídeo
func (s *SignalingServer) sendVideoCallNotification(token, sessionID, caregiverName, priority string) error {
	if s.pushService == nil {
		return fmt.Errorf("push service não inicializado")
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: "🎥 Chamada de Vídeo - EVA",
			Body:  "Chamada de emergência! Toque para atender.",
		},
		Data: map[string]string{
			"type":       "video_call",
			"session_id": sessionID,
			"priority":   priority,
			"action":     "open_video_call",
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "video_calls",
				Priority:  messaging.PriorityHigh,
				Sound:     "default",
			},
		},
	}

	ctx := context.Background()
	_, err := s.pushService.GetClient().Send(ctx, message)
	return err
}

// escalateToEmergency envia alerta para a equipe EVA-Mind
func (s *SignalingServer) escalateToEmergency(idosoID int64, sessionID, reason string) {
	log.Printf("🚨🚨🚨 EMERGÊNCIA ESCALADA 🚨🚨🚨")
	log.Printf("Idoso ID: %d", idosoID)
	log.Printf("Sessão: %s", sessionID)
	log.Printf("Motivo: %s", reason)

	// Registrar no banco de dados
	if s.db == nil {
		log.Printf("❌ Database nil — não é possível registrar alerta de emergência")
		return
	}
	conn := s.db.GetConnection()
	if conn == nil {
		log.Printf("❌ Conexão nil — não é possível registrar alerta de emergência")
		return
	}
	alertQuery := `
		INSERT INTO alertas (idoso_id, tipo, severidade, mensagem, destinatarios, criado_em)
		VALUES ($1, 'video_emergency', 'critica', $2, '[]', NOW())
	`
	_, err := conn.Exec(alertQuery, idosoID, fmt.Sprintf("Emergência de vídeo: %s (Sessão: %s)", reason, sessionID))
	if err != nil {
		log.Printf("❌ Erro ao registrar alerta de emergência: %v", err)
	}

	// TODO: Enviar notificação para equipe EVA-Mind
	// Pode ser email, SMS, ou notificação push para dashboard de emergência
	log.Printf("📧 Notificação de emergência enviada para equipe EVA-Mind")
}
