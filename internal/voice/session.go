package voice

import (
	"eva-mind/internal/gemini"
	"sync"
)

// sessions armazena os clientes Gemini por agendamento_id (thread-safe)
var sessions sync.Map

// StoreSession armazena um cliente Gemini associado ao agendamento_id.
func StoreSession(agID string, client *gemini.Client) {
	sessions.Store(agID, client)
}

// GetSession recupera o cliente Gemini pelo agendamento_id.
func GetSession(agID string) *gemini.Client {
	val, ok := sessions.Load(agID)
	if !ok {
		return nil
	}
	return val.(*gemini.Client)
}

// RemoveSession remove o cliente Gemini e fecha a conexão.
func RemoveSession(agID string) {
	val, ok := sessions.LoadAndDelete(agID)
	if ok {
		client := val.(*gemini.Client)
		client.Close()
	}
}
