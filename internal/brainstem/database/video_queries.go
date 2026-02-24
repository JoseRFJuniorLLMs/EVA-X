// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type VideoSession struct {
	ID        string
	SessionID string
	IdosoID   int64
	Status    string
	SdpOffer  string
	SdpAnswer sql.NullString
	CreatedAt time.Time
}

type SignalingMessage struct {
	ID        int64
	SessionID string
	Sender    string
	Type      string
	Payload   string // JSON
	CreatedAt time.Time
}

func contentToVideoSession(m map[string]interface{}) *VideoSession {
	return &VideoSession{
		ID:        getString(m, "id"),
		SessionID: getString(m, "session_id"),
		IdosoID:   getInt64(m, "idoso_id"),
		Status:    getString(m, "status"),
		SdpOffer:  getString(m, "sdp_offer"),
		SdpAnswer: getNullString(m, "sdp_answer"),
		CreatedAt: getTime(m, "created_em"),
	}
}

func contentToSignalingMessage(m map[string]interface{}) SignalingMessage {
	return SignalingMessage{
		ID:        getInt64(m, "id"),
		SessionID: getString(m, "session_id"),
		Sender:    getString(m, "sender"),
		Type:      getString(m, "type"),
		Payload:   getString(m, "payload"),
		CreatedAt: getTime(m, "created_at"),
	}
}

func (db *DB) CreateVideoSession(sessionID string, idosoID int64, sdpOffer string) error {
	ctx := context.Background()

	err := db.insertRowWithID(ctx, "video_sessions", sessionID, map[string]interface{}{
		"session_id": sessionID,
		"idoso_id":   idosoID,
		"status":     "waiting_operator",
		"sdp_offer":  sdpOffer,
		"created_em": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("failed to create video session: %w", err)
	}
	return nil
}

func (db *DB) CreateSignalingMessage(sessionID string, sender string, msgType string, payload string) error {
	ctx := context.Background()

	_, err := db.insertRow(ctx, "signaling_messages", map[string]interface{}{
		"session_id": sessionID,
		"sender":     sender,
		"type":       msgType,
		"payload":    payload,
		"created_at": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("failed to insert signaling message: %w", err)
	}
	return nil
}

func (db *DB) GetVideoSessionAnswer(sessionID string) (string, error) {
	ctx := context.Background()

	rows, err := db.queryNodesByLabel(ctx, "video_sessions",
		` AND n.session_id = $sid`, map[string]interface{}{
			"sid": sessionID,
		}, 1)
	if err != nil {
		return "", fmt.Errorf("failed to get session answer: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}

	answer := getString(rows[0], "sdp_answer")
	return answer, nil
}

func (db *DB) GetOperatorCandidates(sessionID string, sinceID int64) ([]SignalingMessage, error) {
	ctx := context.Background()

	rows, err := db.queryNodesByLabel(ctx, "signaling_messages",
		` AND n.session_id = $sid AND n.sender = $sender`, map[string]interface{}{
			"sid":    sessionID,
			"sender": "operator",
		}, 0)
	if err != nil {
		return nil, err
	}

	var msgs []SignalingMessage
	for _, m := range rows {
		msg := contentToSignalingMessage(m)
		if msg.ID > sinceID {
			msgs = append(msgs, msg)
		}
	}

	// Sort by ID ASC
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].ID < msgs[j].ID
	})

	return msgs, nil
}

func (db *DB) GetVideoSession(sessionID string) (*VideoSession, error) {
	ctx := context.Background()

	rows, err := db.queryNodesByLabel(ctx, "video_sessions",
		` AND n.session_id = $sid`, map[string]interface{}{
			"sid": sessionID,
		}, 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("video session not found")
	}

	return contentToVideoSession(rows[0]), nil
}

func (db *DB) UpdateVideoSessionAnswer(sessionID string, sdpAnswer string) error {
	ctx := context.Background()

	err := db.updateFields(ctx, "video_sessions",
		map[string]interface{}{"session_id": sessionID},
		map[string]interface{}{
			"sdp_answer": sdpAnswer,
			"status":     "active",
		})
	if err != nil {
		return fmt.Errorf("failed to update video session answer: %w", err)
	}
	return nil
}

func (db *DB) GetMobileCandidates(sessionID string, sinceID int64) ([]SignalingMessage, error) {
	ctx := context.Background()

	rows, err := db.queryNodesByLabel(ctx, "signaling_messages",
		` AND n.session_id = $sid AND n.sender = $sender`, map[string]interface{}{
			"sid":    sessionID,
			"sender": "mobile",
		}, 0)
	if err != nil {
		return nil, err
	}

	var msgs []SignalingMessage
	for _, m := range rows {
		msg := contentToSignalingMessage(m)
		if msg.ID > sinceID {
			msgs = append(msgs, msg)
		}
	}

	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].ID < msgs[j].ID
	})

	return msgs, nil
}

type VideoSessionDetail struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	IdosoID   int64     `json:"idoso_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	// Enriched fields from Idoso
	Nome           string         `json:"nome"`
	Idade          int            `json:"idade"`
	Telefone       string         `json:"telefone"`
	NivelCognitivo string         `json:"nivel_cognitivo"`
	FotoUrl        string         `json:"foto_url"`
	Limitacoes     sql.NullString `json:"limitacoes"`
}

// GetPendingVideoSessions retorna todas as sessoes aguardando atendimento COM DADOS DO IDOSO
func (db *DB) GetPendingVideoSessions() ([]VideoSessionDetail, error) {
	ctx := context.Background()

	// Step 1: Get waiting sessions
	rows, err := db.queryNodesByLabel(ctx, "video_sessions",
		` AND n.status = $status`, map[string]interface{}{
			"status": "waiting_operator",
		}, 0)
	if err != nil {
		return nil, err
	}

	// Sort by created_em DESC
	sort.Slice(rows, func(i, j int) bool {
		return getTime(rows[i], "created_em").After(getTime(rows[j], "created_em"))
	})

	// Step 2: Enrich with Idoso data (replaces the SQL JOIN)
	var sessions []VideoSessionDetail
	for _, m := range rows {
		s := VideoSessionDetail{
			ID:        fmt.Sprintf("%v", m["id"]),
			SessionID: getString(m, "session_id"),
			IdosoID:   getInt64(m, "idoso_id"),
			Status:    getString(m, "status"),
			CreatedAt: getTime(m, "created_em"),
		}

		// Fetch idoso data for enrichment
		idoso, err := db.GetIdoso(s.IdosoID)
		if err == nil && idoso != nil {
			s.Nome = idoso.Nome
			s.Idade = time.Now().Year() - idoso.DataNascimento.Year()
			s.Telefone = idoso.Telefone
			s.NivelCognitivo = idoso.NivelCognitivo
			if idoso.LimitacoesAuditivas.Valid && idoso.LimitacoesAuditivas.Bool {
				s.Limitacoes = sql.NullString{String: "Deficiencia Auditiva", Valid: true}
			}
		}

		sessions = append(sessions, s)
	}
	return sessions, nil
}
