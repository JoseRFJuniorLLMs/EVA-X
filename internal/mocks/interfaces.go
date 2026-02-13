package mocks

import "context"

// PushService interface para serviço de push notifications
type PushService interface {
	SendCallNotification(ctx context.Context, deviceToken, callerName string, callerID int64, callType string) error
	SendAlertNotification(ctx context.Context, deviceToken, title, body, severity string, idosoID int64) error
}

// SMSService interface para serviço de SMS
type SMSService interface {
	SendSMS(ctx context.Context, to, from, body string, idosoID int64, severity string) error
}

// VoiceService interface para serviço de chamadas de voz
type VoiceService interface {
	MakeVoiceCall(ctx context.Context, to, from, message string, idosoID int64, severity string) error
}

// EmailService interface para serviço de email
type EmailService interface {
	SendEmail(ctx context.Context, to []string, subject, body string, isHTML bool) error
	SendAlertEmail(ctx context.Context, to []string, subject, body string, idosoID int64, severity string) error
}

// AlertService interface para serviço de alertas
type AlertService interface {
	SendCriticalAlert(ctx context.Context, idosoID int64, message string) error
	SendHighAlert(ctx context.Context, idosoID int64, message string) error
	SendMediumAlert(ctx context.Context, idosoID int64, message string) error
	SendLowAlert(ctx context.Context, idosoID int64, message string) error
}

// CSSRSService interface para escala C-SSRS
type CSSRSService interface {
	StartAssessment(ctx context.Context, idosoID int64, triggerPhrase string) (string, error)
	SubmitResponse(ctx context.Context, sessionID string, questionNumber, responseValue int, responseText string) error
	GetResult(ctx context.Context, sessionID string) (*CSSRSResult, error)
}

// CSSRSResult resultado da escala C-SSRS
type CSSRSResult struct {
	SessionID    string
	IdosoID      int64
	TotalScore   int
	RiskLevel    string // none, low, moderate, high, imminent
	Responses    []CSSRSResponse
	CompletedAt  string
	AlertSent    bool
}

// CSSRSResponse resposta individual do C-SSRS
type CSSRSResponse struct {
	QuestionNumber int
	QuestionText   string
	ResponseValue  int
	ResponseText   string
}

// PHQ9Service interface para escala PHQ-9
type PHQ9Service interface {
	StartAssessment(ctx context.Context, idosoID int64) (string, error)
	SubmitResponse(ctx context.Context, sessionID string, questionNumber, responseValue int, responseText string) error
	GetResult(ctx context.Context, sessionID string) (*PHQ9Result, error)
}

// PHQ9Result resultado da escala PHQ-9
type PHQ9Result struct {
	SessionID       string
	IdosoID         int64
	TotalScore      int    // 0-27
	SeverityLevel   string // minimal, mild, moderate, moderately_severe, severe
	Responses       []PHQ9Response
	CompletedAt     string
	Question9Positive bool // Ideação suicida
}

// PHQ9Response resposta individual do PHQ-9
type PHQ9Response struct {
	QuestionNumber int
	QuestionText   string
	ResponseValue  int // 0-3
	ResponseText   string
}

// GAD7Service interface para escala GAD-7
type GAD7Service interface {
	StartAssessment(ctx context.Context, idosoID int64) (string, error)
	SubmitResponse(ctx context.Context, sessionID string, questionNumber, responseValue int, responseText string) error
	GetResult(ctx context.Context, sessionID string) (*GAD7Result, error)
}

// GAD7Result resultado da escala GAD-7
type GAD7Result struct {
	SessionID     string
	IdosoID       int64
	TotalScore    int    // 0-21
	SeverityLevel string // minimal, mild, moderate, severe
	Responses     []GAD7Response
	CompletedAt   string
}

// GAD7Response resposta individual do GAD-7
type GAD7Response struct {
	QuestionNumber int
	QuestionText   string
	ResponseValue  int // 0-3
	ResponseText   string
}

// Verificar que os mocks implementam as interfaces
var _ PushService = (*MockFirebaseService)(nil)
var _ SMSService = (*MockTwilioService)(nil)
var _ VoiceService = (*MockTwilioService)(nil)
var _ EmailService = (*MockEmailService)(nil)
