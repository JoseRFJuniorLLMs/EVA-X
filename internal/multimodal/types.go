package multimodal

import (
	"context"
	"time"
)

// MediaType representa o tipo de mídia
type MediaType string

const (
	MediaTypeAudio MediaType = "audio/pcm"
	MediaTypeImage MediaType = "image/jpeg"
	MediaTypeVideo MediaType = "video/mp4"
	MediaTypeWebP  MediaType = "image/webp"
	MediaTypePNG   MediaType = "image/png"
)

// MediaChunk representa um chunk de mídia processado
type MediaChunk struct {
	MimeType  string                 `json:"mime_type"`
	Data      string                 `json:"data"` // base64
	Timestamp time.Time              `json:"-"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MultimodalConfig configuração de sessão multimodal
type MultimodalConfig struct {
	EnableImageInput  bool
	EnableVideoInput  bool
	MaxImageSizeMB    int // default: 7MB
	MaxVideoSizeMB    int // default: 30MB
	VideoFrameRateFPS int // default: 1 FPS
	ImageQuality      int // JPEG quality 1-100
}

// DefaultMultimodalConfig retorna configuração padrão
func DefaultMultimodalConfig() *MultimodalConfig {
	return &MultimodalConfig{
		EnableImageInput:  false,
		EnableVideoInput:  false,
		MaxImageSizeMB:    7,
		MaxVideoSizeMB:    30,
		VideoFrameRateFPS: 1,
		ImageQuality:      85,
	}
}

// MediaProcessor interface para processar diferentes tipos de mídia
type MediaProcessor interface {
	Process(ctx context.Context, input []byte) (*MediaChunk, error)
	Validate(input []byte) error
	GetType() MediaType
}

// VisualMemoryEntry representa uma entrada de memória visual
type VisualMemoryEntry struct {
	ID             string
	SessionID      string
	Timestamp      time.Time
	MediaType      MediaType
	RawData        []byte
	Embedding3072D []float64 // Gemini full embedding
	Embedding64D   []float64 // Krylov compressed
	ContextText    string
	Description    string
	CreatedAt      time.Time
}
