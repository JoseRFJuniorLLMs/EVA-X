package mocks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ======================================================
// Testes do MockFirebaseService
// ======================================================

func TestMockFirebaseService_SendAlertNotification(t *testing.T) {
	mock := NewMockFirebaseService()
	ctx := context.Background()

	// Teste básico - sucesso
	err := mock.SendAlertNotification(ctx, "token123", "Alerta", "Corpo", "alta", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, mock.GetAlertCount())

	// Verificar registro
	lastAlert := mock.GetLastAlert()
	require.NotNil(t, lastAlert)
	assert.Equal(t, "token123", lastAlert.DeviceToken)
	assert.Equal(t, "alta", lastAlert.Severity)
	assert.Equal(t, int64(1), lastAlert.IdosoID)
}

func TestMockFirebaseService_FailureMode(t *testing.T) {
	mock := NewMockFirebaseService()
	mock.ShouldFail = true
	mock.FailureError = errors.New("firebase error")
	ctx := context.Background()

	err := mock.SendAlertNotification(ctx, "token", "Alerta", "Corpo", "alta", 1)
	assert.Error(t, err)
	assert.Equal(t, "firebase error", err.Error())
}

func TestMockFirebaseService_HasAlertWithSeverity(t *testing.T) {
	mock := NewMockFirebaseService()
	ctx := context.Background()

	mock.SendAlertNotification(ctx, "token", "Alerta", "Corpo", "critica", 1)
	mock.SendAlertNotification(ctx, "token", "Alerta", "Corpo", "alta", 2)

	assert.True(t, mock.HasAlertWithSeverity("critica"))
	assert.True(t, mock.HasAlertWithSeverity("alta"))
	assert.False(t, mock.HasAlertWithSeverity("baixa"))
}

// ======================================================
// Testes do MockTwilioService
// ======================================================

func TestMockTwilioService_SendSMS(t *testing.T) {
	mock := NewMockTwilioService()
	ctx := context.Background()

	err := mock.SendSMS(ctx, "+5511999999999", "+5511888888888", "Teste SMS", 1, "alta")
	require.NoError(t, err)
	assert.Equal(t, 1, mock.GetSMSCount())

	lastSMS := mock.GetLastSMS()
	require.NotNil(t, lastSMS)
	assert.Equal(t, "+5511999999999", lastSMS.To)
}

func TestMockTwilioService_HasSMSTo(t *testing.T) {
	mock := NewMockTwilioService()
	ctx := context.Background()

	mock.SendSMS(ctx, "+5511999999999", "+5511888888888", "Teste", 1, "alta")

	assert.True(t, mock.HasSMSTo("+5511999999999"))
	assert.False(t, mock.HasSMSTo("+5511777777777"))
}

// ======================================================
// Testes do MockEmailService
// ======================================================

func TestMockEmailService_SendEmail(t *testing.T) {
	mock := NewMockEmailService()
	ctx := context.Background()

	to := []string{"test@example.com", "test2@example.com"}
	err := mock.SendEmail(ctx, to, "Assunto", "Corpo", true)
	require.NoError(t, err)
	assert.Equal(t, 1, mock.GetEmailCount())

	lastEmail := mock.GetLastEmail()
	require.NotNil(t, lastEmail)
	assert.Equal(t, "Assunto", lastEmail.Subject)
	assert.True(t, lastEmail.IsHTML)
}

func TestMockEmailService_HasEmailTo(t *testing.T) {
	mock := NewMockEmailService()
	ctx := context.Background()

	mock.SendEmail(ctx, []string{"test@example.com"}, "Assunto", "Corpo", false)

	assert.True(t, mock.HasEmailTo("test@example.com"))
	assert.False(t, mock.HasEmailTo("other@example.com"))
}

// ======================================================
// Testes do MockAlertService
// ======================================================

func TestMockAlertService_CriticalAlert(t *testing.T) {
	mock := NewMockAlertService()
	ctx := context.Background()

	err := mock.SendCriticalAlert(ctx, 1, "Emergência!")
	require.NoError(t, err)
	assert.Equal(t, 1, mock.CriticalAlertCount)

	// Crítico deve tentar todos os canais
	successful := mock.GetSuccessfulAlerts()
	assert.GreaterOrEqual(t, len(successful), 1)
}

func TestMockAlertService_FallbackChain(t *testing.T) {
	mock := NewMockAlertService()
	mock.PushShouldFail = true
	mock.PushService.FailureError = errors.New("push failed")
	ctx := context.Background()

	// Alta prioridade: push falha, deve tentar SMS
	err := mock.SendHighAlert(ctx, 1, "Alerta importante")
	require.NoError(t, err) // Deve suceder via SMS

	// Verificar que tentou push e falhou, então tentou SMS
	alerts := mock.AlertsSent
	assert.GreaterOrEqual(t, len(alerts), 2)
}

func TestMockAlertService_AllChannelsFail(t *testing.T) {
	mock := NewMockAlertService()
	mock.PushShouldFail = true
	mock.SMSShouldFail = true
	mock.EmailShouldFail = true
	mock.PushService.FailureError = errors.New("all failed")
	ctx := context.Background()

	err := mock.SendCriticalAlert(ctx, 1, "Emergência!")
	assert.Error(t, err) // Deve falhar se todos os canais falharem
}

func TestMockAlertService_Reset(t *testing.T) {
	mock := NewMockAlertService()
	ctx := context.Background()

	mock.SendCriticalAlert(ctx, 1, "Teste")
	mock.SendHighAlert(ctx, 2, "Teste")
	assert.Greater(t, mock.GetTotalAlertCount(), 0)

	mock.Reset()
	assert.Equal(t, 0, mock.GetTotalAlertCount())
	assert.Empty(t, mock.AlertsSent)
}

// ======================================================
// Testes de Integração dos Mocks
// ======================================================

func TestMocksImplementInterfaces(t *testing.T) {
	// Verifica que os mocks implementam as interfaces corretamente
	var _ PushService = (*MockFirebaseService)(nil)
	var _ SMSService = (*MockTwilioService)(nil)
	var _ VoiceService = (*MockTwilioService)(nil)
	var _ EmailService = (*MockEmailService)(nil)
}
