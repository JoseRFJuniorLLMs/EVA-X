package mocks

import (
	"context"
	"sync"
)

// MockTwilioService simula o serviço de SMS/Voice do Twilio
type MockTwilioService struct {
	mu sync.Mutex

	// Contadores
	SMSCallCount   int
	VoiceCallCount int

	// Registros de chamadas
	SMSSent     []SMSCall
	VoiceCalls  []VoiceCall

	// Controle de comportamento
	SMSShouldFail   bool
	VoiceShouldFail bool
	FailureError    error
}

// SMSCall registra uma chamada de SMS
type SMSCall struct {
	To       string
	From     string
	Body     string
	IdosoID  int64
	Severity string
}

// VoiceCall registra uma chamada de voz
type VoiceCall struct {
	To       string
	From     string
	Message  string
	IdosoID  int64
	Severity string
}

// NewMockTwilioService cria um novo mock
func NewMockTwilioService() *MockTwilioService {
	return &MockTwilioService{
		SMSSent:    make([]SMSCall, 0),
		VoiceCalls: make([]VoiceCall, 0),
	}
}

// SendSMS simula envio de SMS
func (m *MockTwilioService) SendSMS(ctx context.Context, to, from, body string, idosoID int64, severity string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SMSCallCount++

	m.SMSSent = append(m.SMSSent, SMSCall{
		To:       to,
		From:     from,
		Body:     body,
		IdosoID:  idosoID,
		Severity: severity,
	})

	if m.SMSShouldFail {
		return m.FailureError
	}

	return nil
}

// MakeVoiceCall simula uma chamada de voz
func (m *MockTwilioService) MakeVoiceCall(ctx context.Context, to, from, message string, idosoID int64, severity string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.VoiceCallCount++

	m.VoiceCalls = append(m.VoiceCalls, VoiceCall{
		To:       to,
		From:     from,
		Message:  message,
		IdosoID:  idosoID,
		Severity: severity,
	})

	if m.VoiceShouldFail {
		return m.FailureError
	}

	return nil
}

// Reset limpa o estado do mock
func (m *MockTwilioService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SMSCallCount = 0
	m.VoiceCallCount = 0
	m.SMSSent = make([]SMSCall, 0)
	m.VoiceCalls = make([]VoiceCall, 0)
	m.SMSShouldFail = false
	m.VoiceShouldFail = false
	m.FailureError = nil
}

// GetSMSCount retorna o número de SMS enviados
func (m *MockTwilioService) GetSMSCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.SMSSent)
}

// GetVoiceCallCount retorna o número de chamadas de voz
func (m *MockTwilioService) GetVoiceCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.VoiceCalls)
}

// GetLastSMS retorna o último SMS enviado
func (m *MockTwilioService) GetLastSMS() *SMSCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.SMSSent) == 0 {
		return nil
	}
	return &m.SMSSent[len(m.SMSSent)-1]
}

// HasSMSTo verifica se foi enviado SMS para um número
func (m *MockTwilioService) HasSMSTo(phoneNumber string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sms := range m.SMSSent {
		if sms.To == phoneNumber {
			return true
		}
	}
	return false
}
