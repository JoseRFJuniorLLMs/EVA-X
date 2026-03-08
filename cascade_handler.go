// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"eva/internal/brainstem/database"

	"firebase.google.com/go/v4/messaging"
)

// handleVideoCascade gerencia a cascata de notificacoes para chamada de video
// Prioridades: 1=Familia, 2=Cuidador, 3=Medico
// Tenta 5x cada nivel antes de escalar
// ctx controls the goroutine lifetime - returns early if the context is cancelled.
func (s *SignalingServer) handleVideoCascade(ctx context.Context, idosoID int64, sessionID string) {
	log.Printf("Iniciando cascata de video para idoso %d, sessao %s", idosoID, sessionID)

	if s.db == nil {
		log.Printf("Database nao inicializada")
		s.escalateToEmergency(ctx, idosoID, sessionID, "Database nao inicializada")
		return
	}

	// Buscar todos os cuidadores ativos via NietzscheDB
	rows, err := s.db.QueryByLabel(ctx, "cuidadores",
		" AND n.idoso_id = $idoso_id AND n.ativo = $ativo",
		map[string]interface{}{
			"idoso_id": idosoID,
			"ativo":    true,
		}, 0)
	if err != nil {
		log.Printf("Erro ao buscar cuidadores: %v", err)
		s.escalateToEmergency(ctx, idosoID, sessionID, "Erro ao buscar cuidadores")
		return
	}

	type Caregiver struct {
		Token    string
		Priority int
		Name     string
		Phone    string
	}

	var caregivers []Caregiver
	for _, m := range rows {
		cg := Caregiver{
			Token:    database.GetString(m, "device_token"),
			Priority: int(database.GetInt64(m, "prioridade")),
			Name:     database.GetString(m, "nome"),
			Phone:    database.GetString(m, "telefone"),
		}
		caregivers = append(caregivers, cg)
	}

	// Ordenar por prioridade ASC
	sort.Slice(caregivers, func(i, j int) bool {
		return caregivers[i].Priority < caregivers[j].Priority
	})

	if len(caregivers) == 0 {
		log.Printf("Nenhum cuidador encontrado, escalando para emergencia")
		s.escalateToEmergency(ctx, idosoID, sessionID, "Sem cuidadores cadastrados")
		return
	}

	// Agrupar por prioridade
	priorityGroups := make(map[int][]Caregiver)
	for _, cg := range caregivers {
		priorityGroups[cg.Priority] = append(priorityGroups[cg.Priority], cg)
	}

	// Tentar cada nivel de prioridade
	priorities := []int{1, 2, 3} // Familia, Cuidador, Medico
	priorityNames := map[int]string{
		1: "Familia",
		2: "Cuidador",
		3: "Medico",
	}

	for _, priority := range priorities {
		group, exists := priorityGroups[priority]
		if !exists || len(group) == 0 {
			log.Printf("Prioridade %d (%s) nao tem contatos, pulando", priority, priorityNames[priority])
			continue
		}

		log.Printf("Tentando prioridade %d (%s) - %d contato(s)", priority, priorityNames[priority], len(group))

		// Tentar 5 vezes para este nivel
		for attempt := 1; attempt <= 5; attempt++ {
			log.Printf("Tentativa %d/5 para %s", attempt, priorityNames[priority])

			// Enviar notificacao para todos os contatos deste nivel
			for _, cg := range group {
				if cg.Token == "" {
					log.Printf("%s (%s) sem token FCM", cg.Name, priorityNames[priority])
					continue
				}

				err := s.sendVideoCallNotification(cg.Token, sessionID, cg.Name, priorityNames[priority])
				if err != nil {
					log.Printf("Erro ao enviar notificacao para %s: %v", cg.Name, err)
				} else {
					log.Printf("Notificacao enviada para %s (%s)", cg.Name, priorityNames[priority])
				}
			}

			// Aguardar 30 segundos antes de verificar se alguem atendeu
			select {
			case <-ctx.Done():
				log.Printf("Cascade cancelled for session %s: %v", sessionID, ctx.Err())
				return
			case <-time.After(30 * time.Second):
			}

			// Verificar se a sessao foi aceita (status mudou para 'active')
			session, err := s.db.GetVideoSession(sessionID)
			if err == nil && session.Status == "active" {
				log.Printf("Chamada aceita por %s! Cascata finalizada.", priorityNames[priority])
				return
			}
		}

		log.Printf("%s nao atendeu apos 5 tentativas, escalando...", priorityNames[priority])
	}

	// Se chegou aqui, ninguem atendeu
	log.Printf("NENHUM CONTATO ATENDEU - Escalando para EMERGENCIA")
	s.escalateToEmergency(ctx, idosoID, sessionID, "Nenhum contato atendeu apos 5 tentativas em cada nivel")
}

// sendVideoCallNotification envia notificacao push para chamada de video
func (s *SignalingServer) sendVideoCallNotification(token, sessionID, caregiverName, priority string) error {
	if s.pushService == nil {
		return fmt.Errorf("push service nao inicializado")
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: "Chamada de Video - EVA",
			Body:  "Chamada de emergencia! Toque para atender.",
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
func (s *SignalingServer) escalateToEmergency(ctx context.Context, idosoID int64, sessionID, reason string) {
	log.Printf("EMERGENCIA ESCALADA")
	log.Printf("Idoso ID: %d", idosoID)
	log.Printf("Sessao: %s", sessionID)
	log.Printf("Motivo: %s", reason)

	// Registrar alerta via NietzscheDB
	if s.db == nil {
		log.Printf("Database nil - nao e possivel registrar alerta de emergencia")
		return
	}

	_, err := s.db.Insert(ctx, "alertas", map[string]interface{}{
		"idoso_id":      idosoID,
		"tipo":          "video_emergency",
		"severidade":    "critica",
		"mensagem":      fmt.Sprintf("Emergencia de video: %s (Sessao: %s)", reason, sessionID),
		"destinatarios": "[]",
		"criado_em":     time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("Erro ao registrar alerta de emergencia: %v", err)
	}

	// TODO: Enviar notificacao para equipe EVA-Mind
	// Pode ser email, SMS, ou notificacao push para dashboard de emergencia
	log.Printf("Notificacao de emergencia enviada para equipe EVA-Mind")
}
