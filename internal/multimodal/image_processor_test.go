package multimodal_test

import (
	"bytes"
	"context"
	"eva-mind/internal/multimodal"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestImage cria uma imagem de teste com padrão colorido
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Cria padrão gradiente
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	return img
}

// encodeJPEG encoda imagem como JPEG com qualidade específica
func encodeJPEG(img image.Image, quality int) []byte {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// encodePNG encoda imagem como PNG
func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestImageProcessor_Validate_ValidJPEG(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	processor := multimodal.NewImageProcessor(config)

	img := createTestImage(100, 100)
	data := encodeJPEG(img, 80)

	err := processor.Validate(data)
	assert.NoError(t, err)
}

func TestImageProcessor_Validate_ValidPNG(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	processor := multimodal.NewImageProcessor(config)

	img := createTestImage(100, 100)
	data := encodePNG(img)

	err := processor.Validate(data)
	assert.NoError(t, err)
}

func TestImageProcessor_Validate_ExceedsSizeLimit(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	config.MaxImageSizeMB = 1 // Limite de 1MB
	processor := multimodal.NewImageProcessor(config)

	// Cria imagem grande (>1MB)
	img := createTestImage(5000, 5000)
	data := encodeJPEG(img, 100)

	err := processor.Validate(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func TestImageProcessor_Validate_InvalidFormat(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	processor := multimodal.NewImageProcessor(config)

	// Dados inválidos (não é imagem)
	data := []byte("not an image")

	err := processor.Validate(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image format")
}

func TestImageProcessor_Process_JPEG(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	config.ImageQuality = 85
	processor := multimodal.NewImageProcessor(config)

	img := createTestImage(200, 150)
	data := encodeJPEG(img, 90)

	ctx := context.Background()
	chunk, err := processor.Process(ctx, data)

	require.NoError(t, err)
	assert.NotNil(t, chunk)
	assert.Equal(t, "image/jpeg", chunk.MimeType)
	assert.NotEmpty(t, chunk.Data) // Base64 encoded
	assert.NotZero(t, chunk.Timestamp)

	// Verifica metadata
	assert.NotNil(t, chunk.Metadata)
	assert.Equal(t, "jpeg", chunk.Metadata["original_format"])
	assert.Equal(t, len(data), chunk.Metadata["original_size"])
	assert.Equal(t, 200, chunk.Metadata["image_width"])
	assert.Equal(t, 150, chunk.Metadata["image_height"])
}

func TestImageProcessor_Process_PNG(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	processor := multimodal.NewImageProcessor(config)

	img := createTestImage(100, 100)
	data := encodePNG(img)

	ctx := context.Background()
	chunk, err := processor.Process(ctx, data)

	require.NoError(t, err)
	assert.NotNil(t, chunk)
	// PNG pode ser convertido para JPEG se muito grande, mas para 100x100 deve manter PNG
	assert.Contains(t, []string{"image/png", "image/jpeg"}, chunk.MimeType)
	assert.NotEmpty(t, chunk.Data)
}

func TestImageProcessor_Process_Compression(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	config.ImageQuality = 60 // Baixa qualidade para maior compressão
	processor := multimodal.NewImageProcessor(config)

	img := createTestImage(500, 500)
	data := encodeJPEG(img, 95) // Alta qualidade no input

	ctx := context.Background()
	chunk, err := processor.Process(ctx, data)

	require.NoError(t, err)

	// Verifica que houve compressão
	compressedSize := chunk.Metadata["compressed_size"].(int)
	originalSize := chunk.Metadata["original_size"].(int)

	assert.Less(t, compressedSize, originalSize, "Compressed size should be smaller than original")
}

func TestImageProcessor_GetType(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	processor := multimodal.NewImageProcessor(config)

	assert.Equal(t, multimodal.MediaTypeImage, processor.GetType())
}

func TestImageProcessor_Process_InvalidInput(t *testing.T) {
	config := multimodal.DefaultMultimodalConfig()
	processor := multimodal.NewImageProcessor(config)

	ctx := context.Background()
	chunk, err := processor.Process(ctx, []byte("invalid data"))

	assert.Error(t, err)
	assert.Nil(t, chunk)
}

func TestImageProcessor_NilConfig(t *testing.T) {
	// Deve usar config padrão se nil
	processor := multimodal.NewImageProcessor(nil)
	assert.NotNil(t, processor)

	img := createTestImage(100, 100)
	data := encodeJPEG(img, 80)

	err := processor.Validate(data)
	assert.NoError(t, err)
}
