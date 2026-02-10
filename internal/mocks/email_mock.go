package mocks

import (
	"context"
	"sync"
)

// MockEmailService simula o serviço de email
type MockEmailService struct {
	mu sync.Mutex

	// Contadores
	CallCount int

	// Registros de chamadas
	EmailsSent []EmailCall

	// Controle de comportamento
	ShouldFail   bool
	FailureError error
}

// EmailCall registra uma chamada de envio de email
type EmailCall struct {
	To       []string
	Subject  string
	Body     string
	IsHTML   bool
	IdosoID  int64
	Severity string
}

// NewMockEmailService cria um novo mock
func NewMockEmailService() *MockEmailService {
	return &MockEmailService{
		EmailsSent: make([]EmailCall, 0),
	}
}

// SendEmail simula envio de email
func (m *MockEmailService) SendEmail(ctx context.Context, to []string, subject, body string, isHTML bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++

	m.EmailsSent = append(m.EmailsSent, EmailCall{
		To:      to,
		Subject: subject,
		Body:    body,
		IsHTML:  isHTML,
	})

	if m.ShouldFail {
		return m.FailureError
	}

	return nil
}

// SendAlertEmail simula envio de email de alerta
func (m *MockEmailService) SendAlertEmail(ctx context.Context, to []string, subject, body string, idosoID int64, severity string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++

	m.EmailsSent = append(m.EmailsSent, EmailCall{
		To:       to,
		Subject:  subject,
		Body:     body,
		IsHTML:   true,
		IdosoID:  idosoID,
		Severity: severity,
	})

	if m.ShouldFail {
		return m.FailureError
	}

	return nil
}

// Reset limpa o estado do mock
func (m *MockEmailService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount = 0
	m.EmailsSent = make([]EmailCall, 0)
	m.ShouldFail = false
	m.FailureError = nil
}

// GetEmailCount retorna o número de emails enviados
func (m *MockEmailService) GetEmailCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.EmailsSent)
}

// GetLastEmail retorna o último email enviado
func (m *MockEmailService) GetLastEmail() *EmailCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.EmailsSent) == 0 {
		return nil
	}
	return &m.EmailsSent[len(m.EmailsSent)-1]
}

// HasEmailTo verifica se foi enviado email para um destinatário
func (m *MockEmailService) HasEmailTo(email string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.EmailsSent {
		for _, to := range e.To {
			if to == email {
				return true
			}
		}
	}
	return false
}

// HasEmailWithSubject verifica se foi enviado email com determinado assunto
func (m *MockEmailService) HasEmailWithSubject(subject string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.EmailsSent {
		if e.Subject == subject {
			return true
		}
	}
	return false
}
