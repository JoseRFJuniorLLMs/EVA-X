// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package multimodal

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	_ "image/gif" // Suporte para decode GIF (mesmo que não seja output)
	"time"

	_ "golang.org/x/image/webp" // Suporte para WEBP
)

// ImageProcessor processa imagens para envio ao Gemini
type ImageProcessor struct {
	config *MultimodalConfig
}

// NewImageProcessor cria novo processador de imagens
func NewImageProcessor(config *MultimodalConfig) *ImageProcessor {
	if config == nil {
		config = DefaultMultimodalConfig()
	}
	return &ImageProcessor{config: config}
}

// GetType retorna o tipo de mídia
func (p *ImageProcessor) GetType() MediaType {
	return MediaTypeImage
}

// Validate valida a imagem (tamanho e formato)
func (p *ImageProcessor) Validate(input []byte) error {
	// Valida tamanho
	sizeMB := float64(len(input)) / (1024 * 1024)
	if sizeMB > float64(p.config.MaxImageSizeMB) {
		return fmt.Errorf("image size %.2fMB exceeds limit %dMB",
			sizeMB, p.config.MaxImageSizeMB)
	}

	// Valida formato (tenta decodificar)
	_, format, err := image.DecodeConfig(bytes.NewReader(input))
	if err != nil {
		return fmt.Errorf("invalid image format: %w", err)
	}

	// Valida formatos suportados
	switch format {
	case "jpeg", "png", "webp", "gif":
		return nil
	default:
		return fmt.Errorf("unsupported image format: %s (supported: jpeg, png, webp, gif)", format)
	}
}

// Process processa a imagem e retorna MediaChunk pronto para envio
func (p *ImageProcessor) Process(ctx context.Context, input []byte) (*MediaChunk, error) {
	// Valida entrada
	if err := p.Validate(input); err != nil {
		return nil, err
	}

	// Decode imagem
	img, format, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Re-encode com qualidade controlada para reduzir tamanho
	var buf bytes.Buffer
	mimeType := ""

	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: p.config.ImageQuality})
		mimeType = "image/jpeg"
	case "png":
		// PNG não tem qualidade, mas podemos converter para JPEG se muito grande
		err = png.Encode(&buf, img)
		mimeType = "image/png"

		// Se PNG resultante é muito grande, converte para JPEG
		if buf.Len() > len(input) {
			buf.Reset()
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: p.config.ImageQuality})
			mimeType = "image/jpeg"
		}
	default:
		// Outros formatos: converte para JPEG
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: p.config.ImageQuality})
		mimeType = "image/jpeg"
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	return &MediaChunk{
		MimeType:  mimeType,
		Data:      encoded,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"original_format": format,
			"original_size":   len(input),
			"compressed_size": buf.Len(),
			"image_width":     img.Bounds().Dx(),
			"image_height":    img.Bounds().Dy(),
		},
	}, nil
}
