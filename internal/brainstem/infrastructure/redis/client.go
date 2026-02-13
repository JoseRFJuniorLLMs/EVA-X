package redis

import (
	"context"
	"eva-mind/internal/brainstem/config"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	client *redis.Client
}

func NewClient(cfg *config.Config) (*Client, error) {
	addr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.RedisPassword,
		DB:       0, // DB default
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("falha ao conectar no Redis: %w", err)
	}

	log.Printf("✅ Conectado ao Redis em %s", addr)
	return &Client{client: rdb}, nil
}

// AppendAudioChunk adiciona um chunk de áudio à lista da sessão
func (c *Client) AppendAudioChunk(ctx context.Context, sessionID string, data []byte) error {
	key := fmt.Sprintf("audio:%s", sessionID)
	// Expira em 1 hora para não lotar memória se a sessão cair
	pipe := c.client.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.Expire(ctx, key, 1*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// GetFullAudio recupera todo o áudio da sessão e limpa a lista (opcionalmente)
func (c *Client) GetFullAudio(ctx context.Context, sessionID string, clear bool) ([]byte, error) {
	key := fmt.Sprintf("audio:%s", sessionID)

	// Pegar todos os chunks
	cmd := c.client.LRange(ctx, key, 0, -1)
	if err := cmd.Err(); err != nil {
		return nil, err
	}

	chunks, err := cmd.Result()
	if err != nil {
		return nil, err
	}

	// PERFORMANCE FIX: Pre-alocar buffer para evitar O(n²)
	// Antes: append em loop realocava memoria a cada iteracao
	// Depois: calcular tamanho total primeiro, alocar uma vez
	totalSize := 0
	for _, chunk := range chunks {
		totalSize += len(chunk)
	}

	fullAudio := make([]byte, 0, totalSize)
	for _, chunk := range chunks {
		fullAudio = append(fullAudio, []byte(chunk)...)
	}

	if clear {
		c.client.Del(ctx, key)
	}

	return fullAudio, nil
}

// GetUnderlyingClient retorna o cliente Redis subjacente para uso externo
func (c *Client) GetUnderlyingClient() *redis.Client {
	return c.client
}

func (c *Client) Close() error {
	return c.client.Close()
}
