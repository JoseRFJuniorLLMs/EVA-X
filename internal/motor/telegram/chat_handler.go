// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TextGeneratorFunc funcao que recebe um prompt e retorna resposta de texto (injetada do main)
type TextGeneratorFunc func(prompt string) (string, error)

// ChatHandler processa mensagens do Telegram usando um gerador de texto
type ChatHandler struct {
	generate TextGeneratorFunc
	botID    int64
	botName  string

	// Historico de conversa por chat (simples ring buffer)
	historyMu sync.RWMutex
	history   map[int64]*chatHistory
}

type chatHistory struct {
	messages []chatMsg
	lastUsed time.Time
}

type chatMsg struct {
	role string // "user" ou "assistant"
	name string // nome do usuario
	text string
	time time.Time
}

// NewChatHandler cria handler de chat Telegram -> Gemini
func NewChatHandler(generate TextGeneratorFunc, botName string) *ChatHandler {
	return &ChatHandler{
		generate: generate,
		botName:  botName,
		history:  make(map[int64]*chatHistory),
	}
}

// SetBotID define o ID do bot para filtragem de mencoes
func (h *ChatHandler) SetBotID(botID int64) {
	h.botID = botID
}

// HandleMessage processa uma mensagem e retorna a resposta da EVA
func (h *ChatHandler) HandleMessage(ctx context.Context, msg *Message) (string, error) {
	if msg == nil || msg.Text == "" {
		return "", nil
	}

	text := msg.Text
	chatID := msg.Chat.ID
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"

	// Em grupos: so responde se mencionada ou em reply ao bot
	if isGroup {
		mentioned := containsBotMention(text, h.botName)
		isReply := msg.ReplyTo != nil && msg.ReplyTo.From != nil && msg.ReplyTo.From.ID == h.botID

		if !mentioned && !isReply {
			return "", nil // ignora mensagens que nao sao para o bot
		}

		// Remove mencao do texto
		text = removeBotMention(text, h.botName)
		text = strings.TrimSpace(text)
		if text == "" {
			return "Oi! Como posso te ajudar?", nil
		}
	}

	// Comandos basicos
	if strings.HasPrefix(text, "/start") {
		return "Ola! Eu sou a <b>EVA</b>, sua assistente virtual de saude.\n\nPode me perguntar sobre:\n- Saude e bem-estar\n- Medicamentos e horarios\n- Dicas de qualidade de vida\n\nE so me chamar pelo nome!", nil
	}
	if strings.HasPrefix(text, "/help") {
		return "<b>Como usar a EVA:</b>\n\n- Me mencione com @" + h.botName + " seguido da pergunta\n- Responda a uma mensagem minha para continuar a conversa\n- Use /start para ver a apresentacao", nil
	}

	userName := "Usuario"
	if msg.From != nil && msg.From.FirstName != "" {
		userName = msg.From.FirstName
	}

	// Adicionar mensagem ao historico
	h.addToHistory(chatID, "user", userName, text)

	// Construir prompt com contexto
	prompt := h.buildPrompt(chatID, userName, text)

	// Chamar gerador de texto (Gemini REST)
	response, err := h.generate(prompt)
	if err != nil {
		log.Error().Err(err).Str("user", userName).Msg("[Telegram] Erro ao gerar resposta")
		return "Desculpe, estou com dificuldade para processar sua mensagem. Tente novamente em alguns segundos.", nil
	}

	// Limpar resposta
	response = cleanResponse(response)
	if response == "" {
		return "Desculpe, nao consegui gerar uma resposta. Pode reformular?", nil
	}

	// Salvar resposta no historico
	h.addToHistory(chatID, "assistant", "EVA", response)

	return response, nil
}

// buildPrompt constroi o prompt completo com contexto e historico
func (h *ChatHandler) buildPrompt(chatID int64, userName, currentMsg string) string {
	var sb strings.Builder

	// System prompt da EVA
	sb.WriteString(`Voce e a EVA (Entidade Virtual de Apoio), uma assistente virtual de saude inteligente e empatica.

Regras:
- Responda em portugues de forma clara, acolhedora e profissional
- Seja concisa (maximo 3 paragrafos, a menos que o usuario peca mais detalhes)
- Use formatacao HTML simples quando apropriado (<b>negrito</b>, <i>italico</i>)
- NAO use markdown (*, **, #), use HTML pois o Telegram usa HTML
- Se a pergunta for sobre saude, de informacoes gerais e sugira consultar um profissional
- Voce pode conversar sobre qualquer assunto, nao apenas saude
- Trate o usuario pelo nome quando possivel
- Seja calorosa e humana, nao robotica
`)

	// Historico recente
	h.historyMu.RLock()
	hist := h.history[chatID]
	h.historyMu.RUnlock()

	if hist != nil && len(hist.messages) > 1 {
		sb.WriteString("\n--- Historico recente ---\n")
		msgs := hist.messages
		start := 0
		if len(msgs) > 10 {
			start = len(msgs) - 10
		}
		for i := start; i < len(msgs)-1; i++ {
			m := msgs[i]
			if m.role == "user" {
				sb.WriteString(fmt.Sprintf("%s: %s\n", m.name, m.text))
			} else {
				sb.WriteString(fmt.Sprintf("EVA: %s\n", m.text))
			}
		}
		sb.WriteString("--- Fim do historico ---\n\n")
	}

	sb.WriteString(fmt.Sprintf("%s diz: %s", userName, currentMsg))

	return sb.String()
}

// addToHistory adiciona mensagem ao historico do chat
func (h *ChatHandler) addToHistory(chatID int64, role, name, text string) {
	h.historyMu.Lock()
	defer h.historyMu.Unlock()

	hist, exists := h.history[chatID]
	if !exists {
		hist = &chatHistory{
			messages: make([]chatMsg, 0, 20),
		}
		h.history[chatID] = hist
	}

	hist.messages = append(hist.messages, chatMsg{
		role: role,
		name: name,
		text: text,
		time: time.Now(),
	})
	hist.lastUsed = time.Now()

	// Manter maximo de 20 mensagens por chat
	if len(hist.messages) > 20 {
		hist.messages = hist.messages[len(hist.messages)-20:]
	}
}

// CleanupOldHistory remove historicos inativos (chamar periodicamente)
func (h *ChatHandler) CleanupOldHistory() {
	h.historyMu.Lock()
	defer h.historyMu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for chatID, hist := range h.history {
		if hist.lastUsed.Before(cutoff) {
			delete(h.history, chatID)
		}
	}
}

// containsBotMention verifica se a mensagem menciona o bot
func containsBotMention(text, botName string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "@"+strings.ToLower(botName)) ||
		strings.Contains(lower, "eva")
}

// removeBotMention remove a mencao do bot do texto
func removeBotMention(text, botName string) string {
	text = strings.ReplaceAll(text, "@"+botName, "")
	text = strings.ReplaceAll(text, "@"+strings.ToLower(botName), "")
	return text
}

// cleanResponse limpa a resposta do Gemini para Telegram
func cleanResponse(text string) string {
	text = strings.TrimPrefix(text, "EVA: ")
	text = strings.TrimPrefix(text, "EVA diz: ")
	text = strings.TrimSpace(text)
	return text
}
