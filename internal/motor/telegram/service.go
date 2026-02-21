// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// Service cliente para Telegram Bot API
type Service struct {
	botToken   string
	client     *http.Client
	longClient *http.Client // client com timeout longo para polling
	botID      int64
}

// Update representa uma atualização do Telegram
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

// Message representa uma mensagem do Telegram
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      *Chat  `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text,omitempty"`
	ReplyTo   *Message `json:"reply_to_message,omitempty"`
}

// User representa um usuario do Telegram
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Chat representa um chat do Telegram
type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"` // "private", "group", "supergroup", "channel"
	Title string `json:"title,omitempty"`
}

// MessageHandler funcao callback para processar mensagens recebidas
type MessageHandler func(ctx context.Context, msg *Message) (string, error)

// NewService cria um serviço Telegram com o bot token
func NewService(botToken string) *Service {
	return &Service{
		botToken: botToken,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		longClient: &http.Client{
			Timeout: 35 * time.Second, // long polling timeout + margem
		},
	}
}

// GetBotID retorna o ID do bot (cached)
func (s *Service) GetBotID() int64 {
	return s.botID
}

// GetToken retorna o token do bot
func (s *Service) GetToken() string {
	return s.botToken
}

// SendMessage envia mensagem de texto via Telegram Bot API
func (s *Service) SendMessage(chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	// Telegram limita mensagens a 4096 caracteres
	if len(text) > 4096 {
		text = text[:4093] + "..."
	}

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

// SendReply envia resposta a uma mensagem especifica
func (s *Service) SendReply(chatID string, replyToID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	if len(text) > 4096 {
		text = text[:4093] + "..."
	}

	payload := map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"reply_to_message_id":      replyToID,
		"allow_sending_without_reply": true,
	}

	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("telegram reply failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram reply error (%d): %s", resp.StatusCode, string(respBody))
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

// SendChatAction envia indicador de "digitando..." no chat
func (s *Service) SendChatAction(chatID, action string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendChatAction", s.botToken)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"action":  action,
	}
	body, _ := json.Marshal(payload)
	s.client.Post(url, "application/json", bytes.NewBuffer(body))
}

// getUpdates busca atualizacoes do Telegram via long polling
func (s *Service) getUpdates(offset int64) ([]Update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30&allowed_updates=[\"message\"]",
		s.botToken, offset)

	resp, err := s.longClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("getUpdates failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode updates failed: %v", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram getUpdates returned ok=false")
	}

	return result.Result, nil
}

// getMe busca info do bot
func (s *Service) getMe() error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", s.botToken)
	resp, err := s.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			ID int64 `json:"id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	s.botID = result.Result.ID
	return nil
}

// StartPolling inicia long polling para receber mensagens e chamar o handler
func (s *Service) StartPolling(ctx context.Context, handler MessageHandler) {
	// Buscar ID do bot
	if err := s.getMe(); err != nil {
		log.Error().Err(err).Msg("[Telegram] Falha ao obter info do bot")
		return
	}
	log.Info().Int64("bot_id", s.botID).Msg("[Telegram] Bot identificado, iniciando polling")

	var offset int64
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("[Telegram] Polling encerrado (context cancelled)")
			return
		default:
		}

		updates, err := s.getUpdates(offset)
		if err != nil {
			log.Error().Err(err).Dur("backoff", backoff).Msg("[Telegram] Erro no polling, retry com backoff")
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second // reset backoff on success

		for _, update := range updates {
			offset = update.UpdateID + 1

			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			// Ignorar mensagens do proprio bot
			if update.Message.From != nil && update.Message.From.ID == s.botID {
				continue
			}

			// Ignorar mensagens antigas (mais de 2 minutos)
			msgAge := time.Since(time.Unix(update.Message.Date, 0))
			if msgAge > 2*time.Minute {
				log.Debug().
					Int64("msg_id", update.Message.MessageID).
					Dur("age", msgAge).
					Msg("[Telegram] Mensagem antiga ignorada")
				continue
			}

			chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
			userName := ""
			if update.Message.From != nil {
				userName = update.Message.From.FirstName
			}

			log.Info().
				Str("chat_id", chatID).
				Str("from", userName).
				Str("text", truncate(update.Message.Text, 80)).
				Msg("[Telegram] Mensagem recebida")

			// Enviar "digitando..." enquanto processa
			s.SendChatAction(chatID, "typing")

			// Processar em goroutine para nao bloquear polling
			go func(msg *Message) {
				response, err := handler(ctx, msg)
				if err != nil {
					log.Error().Err(err).Int64("msg_id", msg.MessageID).Msg("[Telegram] Erro ao processar mensagem")
					return
				}

				if response == "" {
					return
				}

				cid := strconv.FormatInt(msg.Chat.ID, 10)
				if err := s.SendReply(cid, msg.MessageID, response); err != nil {
					// Fallback: tenta sem reply
					log.Warn().Err(err).Msg("[Telegram] Reply falhou, tentando mensagem simples")
					s.SendMessage(cid, response)
				}
			}(update.Message)
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
