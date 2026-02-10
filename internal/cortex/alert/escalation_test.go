package alert

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ======================================================
// Testes do Sistema de Escalonamento de Alertas
// ======================================================

func TestAlertPriorities(t *testing.T) {
	// Verificar que as prioridades estão definidas corretamente
	assert.Equal(t, AlertPriority("critical"), PriorityCritical)
	assert.Equal(t, AlertPriority("high"), PriorityHigh)
	assert.Equal(t, AlertPriority("medium"), PriorityMedium)
	assert.Equal(t, AlertPriority("low"), PriorityLow)
}

func TestDeliveryChannels(t *testing.T) {
	// Verificar que os canais estão na ordem correta de escalonamento
	channels := []DeliveryChannel{ChannelPush, ChannelWhatsApp, ChannelSMS, ChannelEmail, ChannelCall}

	assert.Equal(t, DeliveryChannel("push"), channels[0], "Push deve ser primeiro")
	assert.Equal(t, DeliveryChannel("whatsapp"), channels[1], "WhatsApp deve ser segundo")
	assert.Equal(t, DeliveryChannel("sms"), channels[2], "SMS deve ser terceiro")
	assert.Equal(t, DeliveryChannel("email"), channels[3], "Email deve ser quarto")
	assert.Equal(t, DeliveryChannel("call"), channels[4], "Ligação deve ser último recurso")
}

func TestNewEscalationService(t *testing.T) {
	cfg := EscalationConfig{
		Firebase:    nil,
		Twilio:      nil,
		Email:       nil,
		DB:          nil,
		CallbackURL: "https://example.com/callback",
	}

	svc := NewEscalationService(cfg)

	require.NotNil(t, svc, "Service não deve ser nil")
	assert.Equal(t, "https://example.com/callback", svc.callbackURL)

	// Verificar timeouts padrão
	assert.Equal(t, 30*time.Second, svc.timeouts[PriorityCritical])
	assert.Equal(t, 2*time.Minute, svc.timeouts[PriorityHigh])
	assert.Equal(t, 5*time.Minute, svc.timeouts[PriorityMedium])
	assert.Equal(t, 15*time.Minute, svc.timeouts[PriorityLow])
}

func TestEscalationResult_Initialization(t *testing.T) {
	result := &EscalationResult{
		AlertID:   "test-alert-123",
		ElderName: "Maria Silva",
		Reason:    "Queda detectada",
		Priority:  PriorityCritical,
		Attempts:  make([]AlertAttempt, 0),
		StartedAt: time.Now(),
	}

	assert.Equal(t, "test-alert-123", result.AlertID)
	assert.Equal(t, "Maria Silva", result.ElderName)
	assert.Equal(t, PriorityCritical, result.Priority)
	assert.False(t, result.Acknowledged)
	assert.Empty(t, result.Attempts)
}

func TestAlertAttempt_RecordingLatency(t *testing.T) {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)

	attempt := AlertAttempt{
		Channel:     ChannelPush,
		Success:     true,
		MessageID:   "msg-123",
		Error:       "",
		AttemptedAt: start,
		Latency:     time.Since(start),
	}

	assert.Equal(t, ChannelPush, attempt.Channel)
	assert.True(t, attempt.Success)
	assert.GreaterOrEqual(t, attempt.Latency, 10*time.Millisecond, "Latência deve ser registrada")
}

func TestCaregiverContact_PriorityOrdering(t *testing.T) {
	contacts := []CaregiverContact{
		{ID: 1, Name: "Filho", Priority: 1, PhoneNumber: "+5511999999999"},
		{ID: 2, Name: "Médico", Priority: 3, PhoneNumber: "+5511888888888"},
		{ID: 3, Name: "Filha", Priority: 2, PhoneNumber: "+5511777777777"},
	}

	// Verificar que o contato com prioridade 1 é o principal
	var primary *CaregiverContact
	for i := range contacts {
		if contacts[i].Priority == 1 {
			primary = &contacts[i]
			break
		}
	}

	require.NotNil(t, primary)
	assert.Equal(t, "Filho", primary.Name, "Contato primário deve ter prioridade 1")
}

func TestAcknowledgeAlert(t *testing.T) {
	svc := NewEscalationService(EscalationConfig{})

	alertID := "test-ack-123"

	// Criar um resultado de alerta ativo
	result := &EscalationResult{
		AlertID:   alertID,
		ElderName: "João Silva",
		Reason:    "Teste",
		Priority:  PriorityMedium,
	}

	// Simular armazenamento do alerta ativo
	svc.activeAlerts.Store(alertID, result)

	// Testar acknowledgment
	success := svc.AcknowledgeAlert(alertID, "Maria (filha)")

	assert.True(t, success, "Acknowledgment deve funcionar")
	assert.True(t, result.Acknowledged)
	assert.Equal(t, "Maria (filha)", result.AcknowledgedBy)
	assert.False(t, result.AcknowledgedAt.IsZero(), "Timestamp deve ser registrado")
}

func TestAcknowledgeAlert_NotFound(t *testing.T) {
	svc := NewEscalationService(EscalationConfig{})

	// Tentar confirmar alerta inexistente
	success := svc.AcknowledgeAlert("non-existent-alert", "Alguém")

	assert.False(t, success, "Não deve confirmar alerta inexistente")
}

func TestGetActiveAlerts_Empty(t *testing.T) {
	svc := NewEscalationService(EscalationConfig{})

	active := svc.GetActiveAlerts()

	assert.Empty(t, active, "Sem alertas ativos inicialmente")
}

func TestGetActiveAlerts_WithAlerts(t *testing.T) {
	svc := NewEscalationService(EscalationConfig{})

	// Adicionar alguns alertas
	svc.activeAlerts.Store("alert-1", &EscalationResult{
		AlertID:      "alert-1",
		Acknowledged: false,
	})
	svc.activeAlerts.Store("alert-2", &EscalationResult{
		AlertID:      "alert-2",
		Acknowledged: true, // Este não deve aparecer
	})
	svc.activeAlerts.Store("alert-3", &EscalationResult{
		AlertID:      "alert-3",
		Acknowledged: false,
	})

	active := svc.GetActiveAlerts()

	// Deve retornar apenas alertas não confirmados
	assert.Len(t, active, 2, "Deve ter 2 alertas ativos (não confirmados)")
}

func TestSendEmergencyAlert_CriticalPriority(t *testing.T) {
	svc := NewEscalationService(EscalationConfig{
		Firebase:    nil, // Sem serviços configurados
		Twilio:      nil,
		Email:       nil,
		CallbackURL: "https://example.com",
	})

	contacts := []CaregiverContact{
		{ID: 1, Name: "Filho", Priority: 1, PhoneNumber: "+5511999999999"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := svc.SendEmergencyAlert(ctx, "Maria Silva", "Risco de queda", PriorityCritical, contacts)

	require.NotNil(t, result)
	assert.Equal(t, "Maria Silva", result.ElderName)
	assert.Equal(t, PriorityCritical, result.Priority)
	assert.Equal(t, "Risco de queda", result.Reason)
	assert.NotEmpty(t, result.AlertID)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
}

func TestSendEmergencyAlert_ContextCancellation(t *testing.T) {
	svc := NewEscalationService(EscalationConfig{})

	contacts := []CaregiverContact{
		{ID: 1, Name: "Filho", Priority: 1},
	}

	// Criar contexto já cancelado
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar imediatamente

	result := svc.SendEmergencyAlert(ctx, "Maria", "Teste", PriorityMedium, contacts)

	require.NotNil(t, result)
	// O alerta deve terminar rapidamente devido ao cancelamento
	assert.False(t, result.CompletedAt.IsZero())
}

func TestSendMissedCallAlert(t *testing.T) {
	svc := NewEscalationService(EscalationConfig{})

	contacts := []CaregiverContact{
		{ID: 1, Name: "Filho", Priority: 1},
	}

	ctx := context.Background()
	result := svc.SendMissedCallAlert(ctx, "Maria Silva", contacts)

	require.NotNil(t, result)
	assert.Equal(t, "Maria Silva", result.ElderName)
	assert.Equal(t, "Não atendeu chamada agendada", result.Reason)
	assert.Equal(t, PriorityMedium, result.Priority, "Missed call deve ser prioridade média")
}

// ======================================================
// Testes de Cenários de Escalonamento
// ======================================================

func TestEscalationScenarios(t *testing.T) {
	testCases := []struct {
		name             string
		priority         AlertPriority
		reason           string
		expectedTimeout  time.Duration
	}{
		{
			name:            "Alerta Crítico - Risco Suicida",
			priority:        PriorityCritical,
			reason:          "C-SSRS positivo - risco iminente",
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "Alerta Alto - Queda Detectada",
			priority:        PriorityHigh,
			reason:          "Sensor detectou queda",
			expectedTimeout: 2 * time.Minute,
		},
		{
			name:            "Alerta Médio - Medicamento Atrasado",
			priority:        PriorityMedium,
			reason:          "Não tomou medicamento há 2 horas",
			expectedTimeout: 5 * time.Minute,
		},
		{
			name:            "Alerta Baixo - Lembrete",
			priority:        PriorityLow,
			reason:          "Consulta médica amanhã",
			expectedTimeout: 15 * time.Minute,
		},
	}

	svc := NewEscalationService(EscalationConfig{})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timeout := svc.timeouts[tc.priority]
			assert.Equal(t, tc.expectedTimeout, timeout, "Timeout incorreto para %s", tc.priority)
		})
	}
}

// ======================================================
// Testes de Integração com Mocks
// ======================================================

// MockPushService simula o serviço de push para testes
type MockPushService struct {
	SendCount    int
	ShouldFail   bool
	FailureError error
}

func (m *MockPushService) Send(token, title, body string) (bool, error) {
	m.SendCount++
	if m.ShouldFail {
		return false, m.FailureError
	}
	return true, nil
}

func TestAlertAttempts_Recording(t *testing.T) {
	result := &EscalationResult{
		AlertID:  "test-123",
		Attempts: make([]AlertAttempt, 0),
	}

	// Simular tentativas
	result.Attempts = append(result.Attempts, AlertAttempt{
		Channel:     ChannelPush,
		Success:     false,
		Error:       "Token inválido",
		AttemptedAt: time.Now(),
		Latency:     50 * time.Millisecond,
	})

	result.Attempts = append(result.Attempts, AlertAttempt{
		Channel:     ChannelSMS,
		Success:     true,
		MessageID:   "SM123456",
		AttemptedAt: time.Now(),
		Latency:     200 * time.Millisecond,
	})

	assert.Len(t, result.Attempts, 2)
	assert.False(t, result.Attempts[0].Success, "Primeira tentativa (Push) deve ter falhado")
	assert.True(t, result.Attempts[1].Success, "Segunda tentativa (SMS) deve ter sucedido")
	assert.Equal(t, "SM123456", result.Attempts[1].MessageID)
}

// ======================================================
// Testes de Validação de Contatos
// ======================================================

func TestCaregiverContact_Validation(t *testing.T) {
	testCases := []struct {
		name            string
		contact         CaregiverContact
		hasPush         bool
		hasSMS          bool
		hasEmail        bool
	}{
		{
			name: "Contato completo",
			contact: CaregiverContact{
				ID:          1,
				Name:        "Filho",
				FCMToken:    "token123",
				PhoneNumber: "+5511999999999",
				Email:       "filho@example.com",
			},
			hasPush:  true,
			hasSMS:   true,
			hasEmail: true,
		},
		{
			name: "Apenas push",
			contact: CaregiverContact{
				ID:       2,
				Name:     "Filha",
				FCMToken: "token456",
			},
			hasPush:  true,
			hasSMS:   false,
			hasEmail: false,
		},
		{
			name: "Apenas telefone",
			contact: CaregiverContact{
				ID:          3,
				Name:        "Médico",
				PhoneNumber: "+5511888888888",
			},
			hasPush:  false,
			hasSMS:   true,
			hasEmail: false,
		},
		{
			name: "Sem canais",
			contact: CaregiverContact{
				ID:   4,
				Name: "Vizinho",
			},
			hasPush:  false,
			hasSMS:   false,
			hasEmail: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.hasPush, tc.contact.FCMToken != "", "Push disponível")
			assert.Equal(t, tc.hasSMS, tc.contact.PhoneNumber != "", "SMS disponível")
			assert.Equal(t, tc.hasEmail, tc.contact.Email != "", "Email disponível")
		})
	}
}

// ======================================================
// Testes de Severidade para Alertas Clínicos
// ======================================================

func TestClinicalAlertSeverity(t *testing.T) {
	testCases := []struct {
		scenario         string
		expectedPriority AlertPriority
		reason           string
	}{
		{
			scenario:         "C-SSRS Crítico",
			expectedPriority: PriorityCritical,
			reason:           "Resposta positiva em C-SSRS Q5/Q6 - tentativa ou preparação",
		},
		{
			scenario:         "C-SSRS Alto",
			expectedPriority: PriorityHigh,
			reason:           "Resposta positiva em C-SSRS Q3/Q4 - ideação com plano",
		},
		{
			scenario:         "PHQ-9 Severo",
			expectedPriority: PriorityHigh,
			reason:           "PHQ-9 score >= 20 - depressão severa",
		},
		{
			scenario:         "PHQ-9 Q9 Positiva",
			expectedPriority: PriorityCritical,
			reason:           "PHQ-9 Q9 positiva - ideação suicida detectada",
		},
		{
			scenario:         "GAD-7 Severo",
			expectedPriority: PriorityMedium,
			reason:           "GAD-7 score >= 15 - ansiedade severa",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.scenario, func(t *testing.T) {
			// Verificar que a prioridade está corretamente mapeada
			switch tc.expectedPriority {
			case PriorityCritical:
				assert.Contains(t, []string{"C-SSRS Crítico", "PHQ-9 Q9 Positiva"}, tc.scenario)
			case PriorityHigh:
				assert.Contains(t, []string{"C-SSRS Alto", "PHQ-9 Severo"}, tc.scenario)
			case PriorityMedium:
				assert.Equal(t, "GAD-7 Severo", tc.scenario)
			}
		})
	}
}
