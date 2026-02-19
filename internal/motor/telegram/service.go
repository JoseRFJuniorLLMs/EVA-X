// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Service cliente para Telegram Bot API
type Service struct {
	botToken string
	client   *http.Client
}

// NewService cria um serviço Telegram com o bot token
func NewService(botToken string) *Service {
	return &Service{
		botToken: botToken,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SendMessage envia mensagem de texto via Telegram Bot API
func (s *Service) SendMessage(chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("telegram send failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errResp struct {
			Description string `json:"description"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("telegram error (%d): %s", resp.StatusCode, errResp.Description)
	}

	return nil
}

// SendPhoto envia foto com legenda via Telegram Bot API
func (s *Service) SendPhoto(chatID, photoURL, caption string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", s.botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"photo":   photoURL,
		"caption": caption,
	}

	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("telegram send photo failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram error: status %d", resp.StatusCode)
	}

	return nil
}
