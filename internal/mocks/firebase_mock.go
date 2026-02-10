package mocks

import (
	"context"
	"sync"
)

// MockFirebaseService simula o serviço de push notifications
type MockFirebaseService struct {
	mu sync.Mutex

	// Contadores de chamadas
	CallCount              int
	SendCallNotifications  []SendCallNotificationCall
	SendAlertNotifications []SendAlertNotificationCall

	// Controle de comportamento
	ShouldFail    bool
	FailureError  error
	FailAfterN    int // Falha após N chamadas bem-sucedidas
}

// SendCallNotificationCall registra uma chamada a SendCallNotification
type SendCallNotificationCall struct {
	DeviceToken string
	CallerName  string
	CallerID    int64
	CallType    string
}

// SendAlertNotificationCall registra uma chamada a SendAlertNotification
type SendAlertNotificationCall struct {
	DeviceToken string
	Title       string
	Body        string
	Severity    string
	IdosoID     int64
}

// NewMockFirebaseService cria um novo mock
func NewMockFirebaseService() *MockFirebaseService {
	return &MockFirebaseService{
		SendCallNotifications:  make([]SendCallNotificationCall, 0),
		SendAlertNotifications: make([]SendAlertNotificationCall, 0),
	}
}

// SendCallNotification simula envio de notificação de chamada
func (m *MockFirebaseService) SendCallNotification(ctx context.Context, deviceToken, callerName string, callerID int64, callType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++

	// Registrar chamada
	m.SendCallNotifications = append(m.SendCallNotifications, SendCallNotificationCall{
		DeviceToken: deviceToken,
		CallerName:  callerName,
		CallerID:    callerID,
		CallType:    callType,
	})

	// Simular falha se configurado
	if m.ShouldFail || (m.FailAfterN > 0 && m.CallCount > m.FailAfterN) {
		return m.FailureError
	}

	return nil
}

// SendAlertNotification simula envio de notificação de alerta
func (m *MockFirebaseService) SendAlertNotification(ctx context.Context, deviceToken, title, body, severity string, idosoID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++

	// Registrar chamada
	m.SendAlertNotifications = append(m.SendAlertNotifications, SendAlertNotificationCall{
		DeviceToken: deviceToken,
		Title:       title,
		Body:        body,
		Severity:    severity,
		IdosoID:     idosoID,
	})

	// Simular falha se configurado
	if m.ShouldFail || (m.FailAfterN > 0 && m.CallCount > m.FailAfterN) {
		return m.FailureError
	}

	return nil
}

// Reset limpa o estado do mock
func (m *MockFirebaseService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount = 0
	m.SendCallNotifications = make([]SendCallNotificationCall, 0)
	m.SendAlertNotifications = make([]SendAlertNotificationCall, 0)
	m.ShouldFail = false
	m.FailureError = nil
	m.FailAfterN = 0
}

// GetAlertCount retorna o número de alertas enviados
func (m *MockFirebaseService) GetAlertCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.SendAlertNotifications)
}

// GetLastAlert retorna o último alerta enviado
func (m *MockFirebaseService) GetLastAlert() *SendAlertNotificationCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.SendAlertNotifications) == 0 {
		return nil
	}
	return &m.SendAlertNotifications[len(m.SendAlertNotifications)-1]
}

// HasAlertWithSeverity verifica se existe alerta com determinada severidade
func (m *MockFirebaseService) HasAlertWithSeverity(severity string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.SendAlertNotifications {
		if alert.Severity == severity {
			return true
		}
	}
	return false
}
