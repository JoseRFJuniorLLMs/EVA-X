// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package voice_test

import (
	"eva-mind/internal/gemini"
	"eva-mind/internal/multimodal"
	"eva-mind/internal/voice"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSafeSession_AudioOnlyStillWorks é um teste de REGRESSÃO CRÍTICO
// Valida que o comportamento de áudio não foi afetado pela adição de multimodal
func TestSafeSession_AudioOnlyStillWorks(t *testing.T) {
	// Setup: Criar sessão SEM multimodal (comportamento original)
	mockClient := &gemini.Client{} // Mock simplificado
	session := &voice.SafeSession{
		Client:    mockClient,
		CreatedAt: time.Now(),
		State:     voice.StateListening,
	}

	// Test 1: GetMultimodal deve retornar nil (não habilitado)
	mm := session.GetMultimodal()
	assert.Nil(t, mm, "Multimodal should be nil by default")

	// Test 2: Estado da sessão funciona normalmente
	assert.Equal(t, voice.StateListening, session.GetState())

	session.SetState(voice.StateSpeaking)
	assert.Equal(t, voice.StateSpeaking, session.GetState())

	// Test 3: Close funciona normalmente
	session.Close()
	// Após Close, operações devem falhar
	err := session.SendAudio([]byte("test"))
	assert.Error(t, err, "Should error after close")
}

// TestSafeSession_MultimodalOptional valida que multimodal é realmente opcional
func TestSafeSession_MultimodalOptional(t *testing.T) {
	mockClient := &gemini.Client{}
	session := &voice.SafeSession{
		Client:    mockClient,
		CreatedAt: time.Now(),
		State:     voice.StateListening,
	}

	// Antes de habilitar
	assert.Nil(t, session.GetMultimodal())

	// Habilitar multimodal
	config := multimodal.DefaultMultimodalConfig()
	config.EnableImageInput = true

	err := session.EnableMultimodal(config)
	require.NoError(t, err)

	// Após habilitar
	mm := session.GetMultimodal()
	assert.NotNil(t, mm, "Multimodal should be enabled")
	assert.NotNil(t, mm.GetImageProcessor())

	// Estado de áudio ainda funciona
	session.SetState(voice.StateProcessing)
	assert.Equal(t, voice.StateProcessing, session.GetState())
}

// TestSafeSession_EnableMultimodal_SessionClosed verifica erro se sessão fechada
func TestSafeSession_EnableMultimodal_SessionClosed(t *testing.T) {
	mockClient := &gemini.Client{}
	session := &voice.SafeSession{
		Client:    mockClient,
		CreatedAt: time.Now(),
	}

	// Fecha sessão
	session.Close()

	// Tentar habilitar multimodal deve falhar
	config := multimodal.DefaultMultimodalConfig()
	err := session.EnableMultimodal(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session closed")
}

// TestSafeSession_EnableMultimodal_AlreadyEnabled verifica erro se já habilitado
func TestSafeSession_EnableMultimodal_AlreadyEnabled(t *testing.T) {
	mockClient := &gemini.Client{}
	session := &voice.SafeSession{
		Client:    mockClient,
		CreatedAt: time.Now(),
	}

	config := multimodal.DefaultMultimodalConfig()

	// Primeira chamada: OK
	err := session.EnableMultimodal(config)
	require.NoError(t, err)

	// Segunda chamada: deve falhar
	err = session.EnableMultimodal(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already enabled")
}

// TestSafeSession_ThreadSafety_GetMultimodal valida thread-safety
func TestSafeSession_ThreadSafety_GetMultimodal(t *testing.T) {
	mockClient := &gemini.Client{}
	session := &voice.SafeSession{
		Client:    mockClient,
		CreatedAt: time.Now(),
	}

	// Chama GetMultimodal em múltiplas goroutines (não deve dar race condition)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			mm := session.GetMultimodal()
			assert.Nil(t, mm) // Deve ser nil consistentemente
			done <- true
		}()
	}

	// Aguarda todas goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestSafeSession_StateTransitions_WithMultimodal valida que estados funcionam com multimodal
func TestSafeSession_StateTransitions_WithMultimodal(t *testing.T) {
	mockClient := &gemini.Client{}
	session := &voice.SafeSession{
		Client:    mockClient,
		CreatedAt: time.Now(),
		State:     voice.StateListening,
	}

	// Habilita multimodal
	config := multimodal.DefaultMultimodalConfig()
	err := session.EnableMultimodal(config)
	require.NoError(t, err)

	// Transições de estado devem continuar funcionando
	session.SetState(voice.StateSpeaking)
	assert.Equal(t, voice.StateSpeaking, session.GetState())

	session.SetState(voice.StateProcessing)
	assert.Equal(t, voice.StateProcessing, session.GetState())

	session.SetState(voice.StateListening)
	assert.Equal(t, voice.StateListening, session.GetState())
}

// TestSafeSession_MultimodalConfig_Applied valida que config é aplicada
func TestSafeSession_MultimodalConfig_Applied(t *testing.T) {
	mockClient := &gemini.Client{}
	session := &voice.SafeSession{
		Client:    mockClient,
		CreatedAt: time.Now(),
	}

	// Config customizada
	config := &multimodal.MultimodalConfig{
		EnableImageInput:  true,
		EnableVideoInput:  false,
		MaxImageSizeMB:    10,
		MaxVideoSizeMB:    50,
		VideoFrameRateFPS: 2,
		ImageQuality:      90,
	}

	err := session.EnableMultimodal(config)
	require.NoError(t, err)

	mm := session.GetMultimodal()
	require.NotNil(t, mm)

	// Verifica que config foi aplicada
	mmConfig := mm.GetConfig()
	assert.Equal(t, true, mmConfig.EnableImageInput)
	assert.Equal(t, false, mmConfig.EnableVideoInput)
	assert.Equal(t, 10, mmConfig.MaxImageSizeMB)
	assert.Equal(t, 90, mmConfig.ImageQuality)
}

// TestSessionManager_StoreAndRetrieve verifica que sessions.Map ainda funciona
func TestSessionManager_StoreAndRetrieve(t *testing.T) {
	mockClient := &gemini.Client{}

	// Store session
	agID := "test-ag-123"
	voice.StoreSession(agID, mockClient)

	// Retrieve session
	session := voice.GetSession(agID)
	require.NotNil(t, session)
	assert.Equal(t, mockClient, session.Client)
	assert.Equal(t, voice.StateListening, session.GetState())

	// Multimodal deve ser nil por padrão
	assert.Nil(t, session.GetMultimodal())

	// Cleanup
	voice.RemoveSession(agID)
	assert.Nil(t, voice.GetSession(agID))
}

// TestSessionManager_RemoveSession_ClosesConnection valida que Close é chamado
func TestSessionManager_RemoveSession_ClosesConnection(t *testing.T) {
	mockClient := &gemini.Client{}

	agID := "test-ag-456"
	voice.StoreSession(agID, mockClient)

	session := voice.GetSession(agID)
	require.NotNil(t, session)

	// Remove deve fechar a sessão
	voice.RemoveSession(agID)

	// Tentar operações deve falhar
	err := session.SendAudio([]byte("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session closed")
}
