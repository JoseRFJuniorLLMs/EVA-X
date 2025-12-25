package voice

import (
	"eva-mind/internal/gemini"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// SafeSession envolve o cliente Gemini com um Mutex para evitar race conditions
type SafeSession struct {
	Client    *gemini.Client
	mu        sync.RWMutex
	closed    bool
	CreatedAt time.Time
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

func (s *SafeSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.Client.Close()
	}
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
		Client:    client,
		CreatedAt: time.Now(),
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
