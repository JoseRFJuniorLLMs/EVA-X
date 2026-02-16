package voice_test

import (
	"bytes"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/gemini"
	"eva-mind/internal/multimodal"
	"eva-mind/internal/voice"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestJPEG cria uma imagem JPEG de teste
func createTestJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

// setupTestHandler cria handler para testes
func setupTestHandler(t *testing.T) *voice.Handler {
	cfg := &config.Config{}
	logger := zerolog.Nop()
	return voice.NewHandler(nil, cfg, logger, nil, nil)
}

func TestHandleMediaUpload_MethodNotAllowed(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/media/upload?agendamento_id=test", nil)
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleMediaUpload_MissingAgendamentoID(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/media/upload", nil)
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "agendamento_id required")
}

func TestHandleMediaUpload_SessionNotFound(t *testing.T) {
	handler := setupTestHandler(t)

	imgData := createTestJPEG()
	req := httptest.NewRequest(http.MethodPost, "/media/upload?agendamento_id=nonexistent", bytes.NewReader(imgData))
	req.Header.Set("Content-Type", "image/jpeg")
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Session not found")
}

func TestHandleMediaUpload_MultimodalNotEnabled(t *testing.T) {
	handler := setupTestHandler(t)

	// Cria sessão SEM multimodal
	mockClient := &gemini.Client{}
	agID := "test-session-123"
	voice.StoreSession(agID, mockClient)
	defer voice.RemoveSession(agID)

	imgData := createTestJPEG()
	req := httptest.NewRequest(http.MethodPost, "/media/upload?agendamento_id="+agID, bytes.NewReader(imgData))
	req.Header.Set("Content-Type", "image/jpeg")
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	// Deve retornar 403 Forbidden pois multimodal não está habilitado
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Multimodal not enabled")
}

func TestHandleMediaUpload_MissingContentType(t *testing.T) {
	handler := setupTestHandler(t)

	// Cria sessão COM multimodal
	mockClient := &gemini.Client{}
	agID := "test-session-456"
	voice.StoreSession(agID, mockClient)
	defer voice.RemoveSession(agID)

	session := voice.GetSession(agID)
	config := multimodal.DefaultMultimodalConfig()
	config.EnableImageInput = true
	session.EnableMultimodal(config)

	imgData := createTestJPEG()
	req := httptest.NewRequest(http.MethodPost, "/media/upload?agendamento_id="+agID, bytes.NewReader(imgData))
	// NÃO define Content-Type
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Content-Type header required")
}

func TestHandleMediaUpload_EmptyBody(t *testing.T) {
	handler := setupTestHandler(t)

	mockClient := &gemini.Client{}
	agID := "test-session-789"
	voice.StoreSession(agID, mockClient)
	defer voice.RemoveSession(agID)

	session := voice.GetSession(agID)
	config := multimodal.DefaultMultimodalConfig()
	config.EnableImageInput = true
	session.EnableMultimodal(config)

	req := httptest.NewRequest(http.MethodPost, "/media/upload?agendamento_id="+agID, bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "image/jpeg")
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Empty body")
}

func TestHandleMediaUpload_UnsupportedMediaType(t *testing.T) {
	handler := setupTestHandler(t)

	mockClient := &gemini.Client{}
	agID := "test-session-abc"
	voice.StoreSession(agID, mockClient)
	defer voice.RemoveSession(agID)

	session := voice.GetSession(agID)
	config := multimodal.DefaultMultimodalConfig()
	config.EnableImageInput = true
	session.EnableMultimodal(config)

	// Envia texto como se fosse mídia
	req := httptest.NewRequest(http.MethodPost, "/media/upload?agendamento_id="+agID, bytes.NewReader([]byte("text data")))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	assert.Contains(t, w.Body.String(), "Unsupported media type")
}

func TestHandleMediaUpload_InvalidImageData(t *testing.T) {
	handler := setupTestHandler(t)

	mockClient := &gemini.Client{}
	agID := "test-session-def"
	voice.StoreSession(agID, mockClient)
	defer voice.RemoveSession(agID)

	session := voice.GetSession(agID)
	config := multimodal.DefaultMultimodalConfig()
	config.EnableImageInput = true
	session.EnableMultimodal(config)

	// Envia dados inválidos
	req := httptest.NewRequest(http.MethodPost, "/media/upload?agendamento_id="+agID, bytes.NewReader([]byte("not an image")))
	req.Header.Set("Content-Type", "image/jpeg")
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to process media")
}

// TestHandleMediaUpload_SuccessFlow seria o teste completo, mas requer
// conexão WebSocket ativa com Gemini, o que não é possível em teste unitário
// Este teste será feito na integração E2E (Fase 2 final)

// Teste de REGRESSÃO: Valida que sessões de áudio continuam funcionando
func TestMediaUpload_DoesNotAffectAudioSessions(t *testing.T) {
	// Cria sessão de áudio pura (sem multimodal)
	mockClient := &gemini.Client{}
	agID := "audio-only-session"
	voice.StoreSession(agID, mockClient)
	defer voice.RemoveSession(agID)

	session := voice.GetSession(agID)
	require.NotNil(t, session)

	// Estado de áudio deve funcionar normalmente
	session.SetState(voice.StateListening)
	assert.Equal(t, voice.StateListening, session.GetState())

	session.SetState(voice.StateSpeaking)
	assert.Equal(t, voice.StateSpeaking, session.GetState())

	// GetMultimodal deve retornar nil
	assert.Nil(t, session.GetMultimodal())

	// Tentar upload de mídia deve falhar gracefully (403)
	handler := setupTestHandler(t)
	imgData := createTestJPEG()
	req := httptest.NewRequest(http.MethodPost, "/media/upload?agendamento_id="+agID, bytes.NewReader(imgData))
	req.Header.Set("Content-Type", "image/jpeg")
	w := httptest.NewRecorder()

	handler.HandleMediaUpload(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	// Sessão de áudio ainda está ativa e funcionando
	assert.Equal(t, voice.StateSpeaking, session.GetState())
}
