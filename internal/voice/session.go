// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package voice

import (
	"eva-mind/internal/gemini"
	"eva-mind/internal/multimodal"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ConversationState representa o estado atual da conversação
type ConversationState int

const (
	StateListening  ConversationState = iota // Aguardando fala do usuário
	StateSpeaking                            // EVA está falando
	StateProcessing                          // Processando resposta
)

// SafeSession envolve o cliente Gemini com um Mutex para evitar race conditions
type SafeSession struct {
	Client       *gemini.Client
	mu           sync.RWMutex
	closed       bool
	CreatedAt    time.Time
	State        ConversationState
	lastActivity time.Time

	// NOVO: Campo opcional para capacidades multimodais
	// Se nil, a sessão funciona apenas com áudio (comportamento original)
	// Se não-nil, permite processar imagens e vídeo
	multimodal *multimodal.MultimodalSession
}

func (s *SafeSession) SendAudio(data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	return s.Client.SendAudio(data)
}

func (s *SafeSession) ReadResponse() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("session closed")
	}
	return s.Client.ReadResponse()
}

func (s *SafeSession) SendToolResponse(resp map[string]interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	// Usamos o Client diretamente pois o WriteJSON já é um wrapper no websocket.Conn
	// mas mantemos o RLock para garantir que a conexão não feche durante o envio
	return s.Client.WriteJSON(resp)
}

func (s *SafeSession) SendText(text string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	return s.Client.SendText(text)
}

// EnableMultimodal habilita capacidades multimodais na sessão
// Este método NÃO afeta o funcionamento do áudio se não for chamado
func (s *SafeSession) EnableMultimodal(config *multimodal.MultimodalConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("session closed")
	}

	if s.multimodal != nil {
		return fmt.Errorf("multimodal already enabled for this session")
	}

	// Cria sessão multimodal
	s.multimodal = multimodal.NewMultimodalSession(fmt.Sprintf("session-%v", s.CreatedAt.Unix()), config)

	// Inicializa processadores
	s.multimodal.SetImageProcessor(multimodal.NewImageProcessor(config))
	s.multimodal.SetVideoProcessor(multimodal.NewVideoProcessor(config, nil)) // nil extractor por enquanto

	return nil
}

// GetMultimodal retorna a sessão multimodal (ou nil se não habilitado)
// Thread-safe read
func (s *SafeSession) GetMultimodal() *multimodal.MultimodalSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.multimodal
}

func (s *SafeSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.Client.Close()
	}
}

// SetState atualiza o estado da conversação de forma thread-safe
func (s *SafeSession) SetState(state ConversationState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
	s.lastActivity = time.Now()
}

// GetState retorna o estado atual da conversação
func (s *SafeSession) GetState() ConversationState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// sessions armazena os SafeSession por agendamento_id (thread-safe)
var sessions sync.Map
var logger zerolog.Logger

// InitSessionManager inicializa o logger para o gerenciador de sessões e inicia o worker de limpeza.
func InitSessionManager(l zerolog.Logger) {
	logger = l
	go cleanupWorker()
}

// StoreSession armazena um cliente Gemini associado ao agendamento_id.
func StoreSession(agID string, client *gemini.Client) {
	sessions.Store(agID, &SafeSession{
		Client:       client,
		CreatedAt:    time.Now(),
		State:        StateListening,
		lastActivity: time.Now(),
	})
}

// GetSession recupera a sessão segura pelo agendamento_id.
func GetSession(agID string) *SafeSession {
	val, ok := sessions.Load(agID)
	if !ok {
		return nil
	}
	return val.(*SafeSession)
}

// RemoveSession remove o cliente Gemini e fecha a conexão.
func RemoveSession(agID string) {
	val, ok := sessions.LoadAndDelete(agID)
	if ok {
		s := val.(*SafeSession)
		s.Close()
	}
}

func cleanupWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		logger.Debug().Msg("Iniciando limpeza de sessões órfãs")
		sessions.Range(func(key, value interface{}) bool {
			s := value.(*SafeSession)
			if time.Since(s.CreatedAt) > 10*time.Minute {
				logger.Warn().Str("ag_id", key.(string)).Msg("Removendo sessão expirada")
				RemoveSession(key.(string))
			}
			return true
		})
	}
}
