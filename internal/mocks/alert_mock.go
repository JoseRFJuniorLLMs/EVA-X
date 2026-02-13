package mocks

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrPushFailed erro padrão para falha de push
	ErrPushFailed = errors.New("push notification failed")
	// ErrSMSFailed erro padrão para falha de SMS
	ErrSMSFailed = errors.New("SMS send failed")
	// ErrEmailFailed erro padrão para falha de email
	ErrEmailFailed = errors.New("email send failed")
)

// MockAlertService simula o serviço de alertas com fallback chain
type MockAlertService struct {
	mu sync.Mutex

	// Serviços de fallback
	PushService  *MockFirebaseService
	SMSService   *MockTwilioService
	EmailService *MockEmailService

	// Contadores
	CriticalAlertCount int
	HighAlertCount     int
	MediumAlertCount   int
	LowAlertCount      int

	// Registros
	AlertsSent []AlertRecord

	// Controle de comportamento
	PushShouldFail  bool
	SMSShouldFail   bool
	EmailShouldFail bool
}

// AlertRecord registro de um alerta enviado
type AlertRecord struct {
	IdosoID   int64
	Message   string
	Severity  string
	Channel   string // push, sms, email, voice
	Success   bool
	Error     string
}

// NewMockAlertService cria um novo mock de alertas
func NewMockAlertService() *MockAlertService {
	return &MockAlertService{
		PushService:  NewMockFirebaseService(),
		SMSService:   NewMockTwilioService(),
		EmailService: NewMockEmailService(),
		AlertsSent:   make([]AlertRecord, 0),
	}
}

// SendCriticalAlert envia alerta crítico (todos os canais)
func (m *MockAlertService) SendCriticalAlert(ctx context.Context, idosoID int64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CriticalAlertCount++

	// Tentar Push
	pushErr := m.sendPush(ctx, idosoID, message, "critica")

	// Sempre tentar SMS em crítico
	smsErr := m.sendSMS(ctx, idosoID, message, "critica")

	// Sempre tentar Email em crítico
	emailErr := m.sendEmail(ctx, idosoID, message, "critica")

	// Crítico: se todos falharem, retorna erro
	if pushErr != nil && smsErr != nil && emailErr != nil {
		return pushErr
	}

	return nil
}

// SendHighAlert envia alerta de alta prioridade (push + sms fallback)
func (m *MockAlertService) SendHighAlert(ctx context.Context, idosoID int64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.HighAlertCount++

	// Tentar Push primeiro
	pushErr := m.sendPush(ctx, idosoID, message, "alta")
	if pushErr == nil {
		return nil
	}

	// Fallback para SMS
	smsErr := m.sendSMS(ctx, idosoID, message, "alta")
	if smsErr == nil {
		return nil
	}

	return pushErr
}

// SendMediumAlert envia alerta de média prioridade (push + email fallback)
func (m *MockAlertService) SendMediumAlert(ctx context.Context, idosoID int64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MediumAlertCount++

	// Tentar Push primeiro
	pushErr := m.sendPush(ctx, idosoID, message, "media")
	if pushErr == nil {
		return nil
	}

	// Fallback para Email
	emailErr := m.sendEmail(ctx, idosoID, message, "media")
	if emailErr == nil {
		return nil
	}

	return pushErr
}

// SendLowAlert envia alerta de baixa prioridade (apenas email)
func (m *MockAlertService) SendLowAlert(ctx context.Context, idosoID int64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.LowAlertCount++

	return m.sendEmail(ctx, idosoID, message, "baixa")
}

// Helpers internos (não precisam de lock - chamados de métodos já com lock)

func (m *MockAlertService) sendPush(ctx context.Context, idosoID int64, message, severity string) error {
	if m.PushShouldFail {
		err := m.PushService.FailureError
		if err == nil {
			err = ErrPushFailed
		}
		m.AlertsSent = append(m.AlertsSent, AlertRecord{
			IdosoID:  idosoID,
			Message:  message,
			Severity: severity,
			Channel:  "push",
			Success:  false,
			Error:    err.Error(),
		})
		return err
	}

	m.AlertsSent = append(m.AlertsSent, AlertRecord{
		IdosoID:  idosoID,
		Message:  message,
		Severity: severity,
		Channel:  "push",
		Success:  true,
	})
	return nil
}

func (m *MockAlertService) sendSMS(ctx context.Context, idosoID int64, message, severity string) error {
	if m.SMSShouldFail {
		err := m.SMSService.FailureError
		if err == nil {
			err = ErrSMSFailed
		}
		m.AlertsSent = append(m.AlertsSent, AlertRecord{
			IdosoID:  idosoID,
			Message:  message,
			Severity: severity,
			Channel:  "sms",
			Success:  false,
			Error:    err.Error(),
		})
		return err
	}

	m.AlertsSent = append(m.AlertsSent, AlertRecord{
		IdosoID:  idosoID,
		Message:  message,
		Severity: severity,
		Channel:  "sms",
		Success:  true,
	})
	return nil
}

func (m *MockAlertService) sendEmail(ctx context.Context, idosoID int64, message, severity string) error {
	if m.EmailShouldFail {
		err := m.EmailService.FailureError
		if err == nil {
			err = ErrEmailFailed
		}
		m.AlertsSent = append(m.AlertsSent, AlertRecord{
			IdosoID:  idosoID,
			Message:  message,
			Severity: severity,
			Channel:  "email",
			Success:  false,
			Error:    err.Error(),
		})
		return err
	}

	m.AlertsSent = append(m.AlertsSent, AlertRecord{
		IdosoID:  idosoID,
		Message:  message,
		Severity: severity,
		Channel:  "email",
		Success:  true,
	})
	return nil
}

// Reset limpa o estado do mock
func (m *MockAlertService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CriticalAlertCount = 0
	m.HighAlertCount = 0
	m.MediumAlertCount = 0
	m.LowAlertCount = 0
	m.AlertsSent = make([]AlertRecord, 0)
	m.PushShouldFail = false
	m.SMSShouldFail = false
	m.EmailShouldFail = false

	m.PushService.Reset()
	m.SMSService.Reset()
	m.EmailService.Reset()
}

// GetTotalAlertCount retorna total de alertas
func (m *MockAlertService) GetTotalAlertCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.CriticalAlertCount + m.HighAlertCount + m.MediumAlertCount + m.LowAlertCount
}

// GetSuccessfulAlerts retorna apenas alertas bem-sucedidos
func (m *MockAlertService) GetSuccessfulAlerts() []AlertRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	var successful []AlertRecord
	for _, a := range m.AlertsSent {
		if a.Success {
			successful = append(successful, a)
		}
	}
	return successful
}

// GetFailedAlerts retorna apenas alertas que falharam
func (m *MockAlertService) GetFailedAlerts() []AlertRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	var failed []AlertRecord
	for _, a := range m.AlertsSent {
		if !a.Success {
			failed = append(failed, a)
		}
	}
	return failed
}

// HasAlertForIdoso verifica se existe alerta para um idoso
func (m *MockAlertService) HasAlertForIdoso(idosoID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.AlertsSent {
		if a.IdosoID == idosoID && a.Success {
			return true
		}
	}
	return false
}
