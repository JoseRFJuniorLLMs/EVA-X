package sms

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TwilioConfig holds the Twilio configuration
type TwilioConfig struct {
	AccountSID   string
	AuthToken    string
	FromNumber   string
	FromWhatsApp string // formato: whatsapp:+14155238886
}

// TwilioService handles SMS and WhatsApp messaging via Twilio
type TwilioService struct {
	config     TwilioConfig
	httpClient *http.Client
	baseURL    string
}

// MessageResult represents the result of a message send operation
type MessageResult struct {
	Success   bool      `json:"success"`
	MessageID string    `json:"message_id,omitempty"`
	Status    string    `json:"status,omitempty"`
	Error     string    `json:"error,omitempty"`
	SentAt    time.Time `json:"sent_at"`
	Channel   string    `json:"channel"` // "sms", "whatsapp", "call"
}

// TwilioResponse represents the API response from Twilio
type TwilioResponse struct {
	SID         string `json:"sid"`
	Status      string `json:"status"`
	ErrorCode   int    `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// NewTwilioService creates a new Twilio SMS/WhatsApp service
func NewTwilioService(config TwilioConfig) (*TwilioService, error) {
	if config.AccountSID == "" || config.AuthToken == "" {
		return nil, fmt.Errorf("twilio AccountSID and AuthToken are required")
	}

	if config.FromNumber == "" && config.FromWhatsApp == "" {
		return nil, fmt.Errorf("at least one of FromNumber or FromWhatsApp is required")
	}

	service := &TwilioService{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s", config.AccountSID),
	}

	log.Printf("✅ Twilio SMS/WhatsApp service initialized (Account: %s...)", config.AccountSID[:8])
	return service, nil
}

// SendSMS sends an SMS message to the specified phone number
func (s *TwilioService) SendSMS(toNumber, message string) (*MessageResult, error) {
	if s.config.FromNumber == "" {
		return nil, fmt.Errorf("SMS not configured: FromNumber is empty")
	}

	result := &MessageResult{
		SentAt:  time.Now(),
		Channel: "sms",
	}

	// Prepare the request
	endpoint := fmt.Sprintf("%s/Messages.json", s.baseURL)

	data := url.Values{}
	data.Set("To", toNumber)
	data.Set("From", s.config.FromNumber)
	data.Set("Body", message)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(s.config.AccountSID, s.config.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to send SMS: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to read response: %w", err)
	}

	var twilioResp TwilioResponse
	if err := json.Unmarshal(body, &twilioResp); err != nil {
		result.Error = fmt.Sprintf("failed to parse response: %s", string(body))
		return result, fmt.Errorf("failed to parse Twilio response: %w", err)
	}

	if resp.StatusCode >= 400 {
		result.Error = twilioResp.ErrorMessage
		return result, fmt.Errorf("twilio error: %s (code: %d)", twilioResp.ErrorMessage, twilioResp.ErrorCode)
	}

	result.Success = true
	result.MessageID = twilioResp.SID
	result.Status = twilioResp.Status

	log.Printf("📱 SMS enviado para %s - SID: %s", toNumber, twilioResp.SID)
	return result, nil
}

// SendWhatsApp sends a WhatsApp message to the specified phone number
func (s *TwilioService) SendWhatsApp(toNumber, message string) (*MessageResult, error) {
	if s.config.FromWhatsApp == "" {
		return nil, fmt.Errorf("WhatsApp not configured: FromWhatsApp is empty")
	}

	result := &MessageResult{
		SentAt:  time.Now(),
		Channel: "whatsapp",
	}

	// Prepare the request
	endpoint := fmt.Sprintf("%s/Messages.json", s.baseURL)

	// WhatsApp numbers need the whatsapp: prefix
	toWhatsApp := toNumber
	if !strings.HasPrefix(toNumber, "whatsapp:") {
		toWhatsApp = fmt.Sprintf("whatsapp:%s", toNumber)
	}

	data := url.Values{}
	data.Set("To", toWhatsApp)
	data.Set("From", s.config.FromWhatsApp)
	data.Set("Body", message)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(s.config.AccountSID, s.config.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to send WhatsApp: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to read response: %w", err)
	}

	var twilioResp TwilioResponse
	if err := json.Unmarshal(body, &twilioResp); err != nil {
		result.Error = fmt.Sprintf("failed to parse response: %s", string(body))
		return result, fmt.Errorf("failed to parse Twilio response: %w", err)
	}

	if resp.StatusCode >= 400 {
		result.Error = twilioResp.ErrorMessage
		return result, fmt.Errorf("twilio WhatsApp error: %s (code: %d)", twilioResp.ErrorMessage, twilioResp.ErrorCode)
	}

	result.Success = true
	result.MessageID = twilioResp.SID
	result.Status = twilioResp.Status

	log.Printf("📱 WhatsApp enviado para %s - SID: %s", toNumber, twilioResp.SID)
	return result, nil
}

// IsConfigured returns true if the service is properly configured
func (s *TwilioService) IsConfigured() bool {
	return s.config.AccountSID != "" && s.config.AuthToken != ""
}

// HasSMS returns true if SMS is configured
func (s *TwilioService) HasSMS() bool {
	return s.config.FromNumber != ""
}

// HasWhatsApp returns true if WhatsApp is configured
func (s *TwilioService) HasWhatsApp() bool {
	return s.config.FromWhatsApp != ""
}
