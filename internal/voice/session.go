package voice

import (
	"eva-mind/internal/gemini"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type sessionEntry struct {
	client    *gemini.Client
	createdAt time.Time
}

// sessions armazena os clientes Gemini por agendamento_id (thread-safe)
var sessions sync.Map
var logger zerolog.Logger

// InitSessionManager inicializa o logger para o gerenciador de sessões e inicia o worker de limpeza.
func InitSessionManager(l zerolog.Logger) {
	logger = l
	go cleanupWorker()
}

// StoreSession armazena um cliente Gemini associado ao agendamento_id.
func StoreSession(agID string, client *gemini.Client) {
	sessions.Store(agID, &sessionEntry{
		client:    client,
		createdAt: time.Now(),
	})
}

// GetSession recupera o cliente Gemini pelo agendamento_id.
func GetSession(agID string) *gemini.Client {
	val, ok := sessions.Load(agID)
	if !ok {
		return nil
	}
	return val.(*sessionEntry).client
}

// RemoveSession remove o cliente Gemini e fecha a conexão.
func RemoveSession(agID string) {
	val, ok := sessions.LoadAndDelete(agID)
	if ok {
		entry := val.(*sessionEntry)
		entry.client.Close()
	}
}

func cleanupWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		logger.Debug().Msg("Iniciando limpeza de sessões órfãs")
		sessions.Range(func(key, value interface{}) bool {
			entry := value.(*sessionEntry)
			if time.Since(entry.createdAt) > 10*time.Minute {
				logger.Warn().Str("ag_id", key.(string)).Msg("Removendo sessão expirada")
				RemoveSession(key.(string))
			}
			return true
		})
	}
}
