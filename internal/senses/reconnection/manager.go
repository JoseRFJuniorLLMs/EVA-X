// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package reconnection

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ConversationState armazena o estado da conversa durante desconexão
type ConversationState struct {
	CPF                 string                   `json:"cpf"`
	IdosoID             int64                    `json:"idoso_id"`
	LastMessageID       int64                    `json:"last_message_id"`
	Mode                string                   `json:"mode"` // "audio" ou "video"
	AudioBufferPending  [][]byte                 `json:"-"`    // Não serializar binário
	PendingToolCalls    []PendingToolCall        `json:"pending_tool_calls"`
	LastTranscriptID    int64                    `json:"last_transcript_id"`
	SessionID           string                   `json:"session_id"`
	GeminiVoice         string                   `json:"gemini_voice"`
	DisconnectedAt      time.Time                `json:"disconnected_at"`
	ConversationContext []ConversationMessage    `json:"conversation_context"`
}

// PendingToolCall representa uma tool call que não foi completada
type PendingToolCall struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	AttemptAt time.Time             `json:"attempt_at"`
}

// ConversationMessage representa uma mensagem da conversa
type ConversationMessage struct {
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

// ReconnectionManager gerencia reconexões automáticas
type ReconnectionManager struct {
	mu             sync.RWMutex
	savedStates    map[string]*ConversationState // key = CPF
	maxRetries     int
	baseBackoff    time.Duration
	maxBackoff     time.Duration
	stateExpiry    time.Duration
	onStateRestored func(cpf string, state *ConversationState) error
}

// NewReconnectionManager cria um novo gerenciador de reconexões
func NewReconnectionManager() *ReconnectionManager {
	return &ReconnectionManager{
		savedStates:  make(map[string]*ConversationState),
		maxRetries:   5,
		baseBackoff:  2 * time.Second,
		maxBackoff:   30 * time.Second,
		stateExpiry:  5 * time.Minute, // Estado expira após 5 minutos
	}
}

// SaveState salva o estado da conversa antes da desconexão
func (rm *ReconnectionManager) SaveState(state *ConversationState) error {
	if state.CPF == "" {
		return fmt.Errorf("CPF is required")
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	state.DisconnectedAt = time.Now()
	rm.savedStates[state.CPF] = state

	log.Printf("💾 Estado salvo para CPF: %s (Session: %s, Mode: %s, Tools pendentes: %d)",
		state.CPF, state.SessionID, state.Mode, len(state.PendingToolCalls))

	return nil
}

// LoadState recupera o estado salvo de uma conversa
func (rm *ReconnectionManager) LoadState(cpf string) (*ConversationState, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	state, exists := rm.savedStates[cpf]
	if !exists {
		return nil, fmt.Errorf("no saved state for CPF: %s", cpf)
	}

	// Verificar se o estado expirou
	if time.Since(state.DisconnectedAt) > rm.stateExpiry {
		log.Printf("⚠️ Estado expirado para CPF: %s (desconectado há %v)",
			cpf, time.Since(state.DisconnectedAt))
		return nil, fmt.Errorf("state expired")
	}

	log.Printf("📂 Estado recuperado para CPF: %s (Session: %s, Tools pendentes: %d)",
		cpf, state.SessionID, len(state.PendingToolCalls))

	return state, nil
}

// DeleteState remove o estado salvo após reconexão bem-sucedida
func (rm *ReconnectionManager) DeleteState(cpf string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.savedStates, cpf)
	log.Printf("🗑️ Estado removido para CPF: %s", cpf)
}

// CleanExpiredStates remove estados expirados periodicamente
func (rm *ReconnectionManager) CleanExpiredStates() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	for cpf, state := range rm.savedStates {
		if now.Sub(state.DisconnectedAt) > rm.stateExpiry {
			delete(rm.savedStates, cpf)
			log.Printf("🧹 Estado expirado removido: %s", cpf)
		}
	}
}

// StartCleanupScheduler inicia limpeza periódica de estados expirados
func (rm *ReconnectionManager) StartCleanupScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("🛑 Cleanup scheduler stopped")
			return
		case <-ticker.C:
			rm.CleanExpiredStates()
		}
	}
}

// AttemptReconnection tenta reconectar com exponential backoff
func (rm *ReconnectionManager) AttemptReconnection(
	ctx context.Context,
	wsURL string,
	cpf string,
	onConnected func(*websocket.Conn) error,
) error {
	for attempt := 1; attempt <= rm.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		default:
		}

		// Calcular backoff exponencial
		backoff := time.Duration(math.Pow(2, float64(attempt-1))) * rm.baseBackoff
		if backoff > rm.maxBackoff {
			backoff = rm.maxBackoff
		}

		log.Printf("🔄 Tentativa de reconexão %d/%d para CPF: %s (aguardando %v)",
			attempt, rm.maxRetries, cpf, backoff)

		// Aguardar backoff (exceto na primeira tentativa)
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during backoff")
			case <-time.After(backoff):
			}
		}

		// Tentar conectar
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			log.Printf("❌ Tentativa %d falhou: %v", attempt, err)
			continue
		}

		maskedCPF := "***"
		if len(cpf) >= 3 {
			maskedCPF = "***" + cpf[len(cpf)-3:]
		}
		log.Printf("✅ Reconexão bem-sucedida na tentativa %d para CPF: %s", attempt, maskedCPF)

		// Executar callback de conexão
		if err := onConnected(conn); err != nil {
			log.Printf("❌ Erro no callback de conexão: %v", err)
			conn.Close()
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to reconnect after %d attempts", rm.maxRetries)
}

// RestoreConversation restaura o contexto da conversa após reconexão
func (rm *ReconnectionManager) RestoreConversation(
	cpf string,
	sendMessage func(interface{}) error,
) error {
	state, err := rm.LoadState(cpf)
	if err != nil {
		return err
	}

	maskedCPF := "***"
	if len(cpf) >= 3 {
		maskedCPF = "***" + cpf[len(cpf)-3:]
	}
	log.Printf("🔄 Restaurando conversa para CPF: %s", maskedCPF)

	// 1. Notificar cliente sobre restauração
	if err := sendMessage(map[string]interface{}{
		"type":    "reconnection_restored",
		"message": "Conversa restaurada com sucesso",
		"session_id": state.SessionID,
		"mode": state.Mode,
	}); err != nil {
		return fmt.Errorf("failed to send restoration notification: %w", err)
	}

	// 2. Re-executar tool calls pendentes
	for _, toolCall := range state.PendingToolCalls {
		log.Printf("🔄 Re-executando tool call pendente: %s", toolCall.ToolName)

		if err := sendMessage(map[string]interface{}{
			"type": "retry_tool_call",
			"tool_name": toolCall.ToolName,
			"args": toolCall.Args,
		}); err != nil {
			log.Printf("⚠️ Erro ao re-executar tool call: %v", err)
		}
	}

	// 3. Enviar buffer de áudio pendente (se houver)
	if len(state.AudioBufferPending) > 0 {
		log.Printf("📦 Reenviando %d chunks de áudio pendentes", len(state.AudioBufferPending))

		for _, audioChunk := range state.AudioBufferPending {
			if err := sendMessage(map[string]interface{}{
				"type": "audio_replay",
				"data": audioChunk,
			}); err != nil {
				log.Printf("⚠️ Erro ao reenviar áudio: %v", err)
			}
		}
	}

	// 4. Enviar contexto de conversa para Gemini
	if len(state.ConversationContext) > 0 {
		contextJSON, _ := json.Marshal(state.ConversationContext)
		log.Printf("💬 Restaurando %d mensagens de contexto", len(state.ConversationContext))

		if err := sendMessage(map[string]interface{}{
			"type": "restore_context",
			"context": string(contextJSON),
		}); err != nil {
			log.Printf("⚠️ Erro ao restaurar contexto: %v", err)
		}
	}

	log.Printf("✅ Conversa restaurada com sucesso para CPF: %s", maskedCPF)

	// Limpar estado após restauração
	rm.DeleteState(cpf)

	return nil
}

// AddPendingToolCall adiciona uma tool call ao estado pendente
func (rm *ReconnectionManager) AddPendingToolCall(cpf string, toolName string, args map[string]interface{}) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.savedStates[cpf]
	if !exists {
		return
	}

	state.PendingToolCalls = append(state.PendingToolCalls, PendingToolCall{
		ToolName:  toolName,
		Args:      args,
		AttemptAt: time.Now(),
	})

	log.Printf("➕ Tool call pendente adicionada: %s para CPF: %s", toolName, cpf)
}

// AddAudioBuffer adiciona áudio ao buffer pendente
func (rm *ReconnectionManager) AddAudioBuffer(cpf string, audioData []byte) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.savedStates[cpf]
	if !exists {
		return
	}

	// Limitar buffer a últimos 10 chunks (evitar memory overflow)
	if len(state.AudioBufferPending) >= 10 {
		state.AudioBufferPending = state.AudioBufferPending[1:]
	}

	state.AudioBufferPending = append(state.AudioBufferPending, audioData)
}

// AddConversationMessage adiciona uma mensagem ao contexto
func (rm *ReconnectionManager) AddConversationMessage(cpf string, role string, text string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.savedStates[cpf]
	if !exists {
		return
	}

	// Limitar contexto a últimas 20 mensagens
	if len(state.ConversationContext) >= 20 {
		state.ConversationContext = state.ConversationContext[1:]
	}

	state.ConversationContext = append(state.ConversationContext, ConversationMessage{
		Role:      role,
		Text:      text,
		Timestamp: time.Now(),
	})
}
