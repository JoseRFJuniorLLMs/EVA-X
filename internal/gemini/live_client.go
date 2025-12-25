package gemini

import (
	"context"
	"eva-mind/internal/config"
)

// NewLiveClient cria um cliente Gemini configurado com o prompt do sistema.
func NewLiveClient(ctx context.Context, cfg *config.Config, systemPrompt string) (*Client, error) {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	err = client.SendSetup(systemPrompt, GetDefaultTools())
	if err != nil {
		client.Close() // Cleanup em caso de erro
		return nil, err
	}
	return client, nil
}
