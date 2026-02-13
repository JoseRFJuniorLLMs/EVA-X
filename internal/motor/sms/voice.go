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

// VoiceCallResult represents the result of a voice call
type VoiceCallResult struct {
	Success   bool      `json:"success"`
	CallSID   string    `json:"call_sid,omitempty"`
	Status    string    `json:"status,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Channel   string    `json:"channel"` // always "call"
}

// TwilioCallResponse represents the API response for calls
type TwilioCallResponse struct {
	SID          string `json:"sid"`
	Status       string `json:"status"`
	ErrorCode    int    `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// MakeEmergencyCall initiates an automated voice call with TTS message
func (s *TwilioService) MakeEmergencyCall(toNumber, elderName, reason, callbackURL string) (*VoiceCallResult, error) {
	result := &VoiceCallResult{
		StartedAt: time.Now(),
		Channel:   "call",
	}

	if s.config.FromNumber == "" {
		result.Error = "voice calls not configured: FromNumber is empty"
		return result, fmt.Errorf(result.Error)
	}

	// Build TwiML for the call
	twimlMessage := fmt.Sprintf(`
		<Response>
			<Say language="pt-BR" voice="Polly.Camila">
				Atenção! Esta é uma mensagem urgente do sistema EVA.
				%s precisa de ajuda imediata.
				Motivo: %s.
				Por favor, entre em contato o mais rápido possível.
				Pressione 1 para confirmar que recebeu esta mensagem.
			</Say>
			<Gather numDigits="1" action="%s" method="POST">
				<Say language="pt-BR" voice="Polly.Camila">
					Pressione 1 para confirmar.
				</Say>
			</Gather>
			<Say language="pt-BR" voice="Polly.Camila">
				Não recebemos confirmação. Tentaremos novamente.
			</Say>
		</Response>
	`, elderName, reason, callbackURL)

	// Prepare the request
	endpoint := fmt.Sprintf("%s/Calls.json", s.baseURL)

	data := url.Values{}
	data.Set("To", toNumber)
	data.Set("From", s.config.FromNumber)
	data.Set("Twiml", twimlMessage)

	// Optional: Set status callback for tracking
	if callbackURL != "" {
		data.Set("StatusCallback", callbackURL)
		data.Set("StatusCallbackEvent", "initiated ringing answered completed")
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to create call request: %w", err)
	}

	req.SetBasicAuth(s.config.AccountSID, s.config.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to initiate call: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to read response: %w", err)
	}

	var twilioResp TwilioCallResponse
	if err := json.Unmarshal(body, &twilioResp); err != nil {
		result.Error = fmt.Sprintf("failed to parse response: %s", string(body))
		return result, fmt.Errorf("failed to parse Twilio response: %w", err)
	}

	if resp.StatusCode >= 400 {
		result.Error = twilioResp.ErrorMessage
		return result, fmt.Errorf("twilio call error: %s (code: %d)", twilioResp.ErrorMessage, twilioResp.ErrorCode)
	}

	result.Success = true
	result.CallSID = twilioResp.SID
	result.Status = twilioResp.Status

	log.Printf("📞 Ligação de emergência iniciada para %s - SID: %s", toNumber, twilioResp.SID)
	return result, nil
}

// MakeMissedCallAlert initiates a voice call for missed call alerts
func (s *TwilioService) MakeMissedCallAlert(toNumber, elderName, callbackURL string) (*VoiceCallResult, error) {
	result := &VoiceCallResult{
		StartedAt: time.Now(),
		Channel:   "call",
	}

	if s.config.FromNumber == "" {
		result.Error = "voice calls not configured: FromNumber is empty"
		return result, fmt.Errorf(result.Error)
	}

	// Build TwiML for the call
	twimlMessage := fmt.Sprintf(`
		<Response>
			<Say language="pt-BR" voice="Polly.Camila">
				Olá! Esta é uma mensagem do sistema EVA.
				%s não atendeu a chamada programada.
				Por favor, verifique se está tudo bem.
				Pressione 1 para confirmar que recebeu esta mensagem.
			</Say>
			<Gather numDigits="1" action="%s" method="POST">
				<Say language="pt-BR" voice="Polly.Camila">
					Pressione 1 para confirmar.
				</Say>
			</Gather>
		</Response>
	`, elderName, callbackURL)

	// Prepare the request
	endpoint := fmt.Sprintf("%s/Calls.json", s.baseURL)

	data := url.Values{}
	data.Set("To", toNumber)
	data.Set("From", s.config.FromNumber)
	data.Set("Twiml", twimlMessage)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to create call request: %w", err)
	}

	req.SetBasicAuth(s.config.AccountSID, s.config.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to initiate call: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to read response: %w", err)
	}

	var twilioResp TwilioCallResponse
	if err := json.Unmarshal(body, &twilioResp); err != nil {
		result.Error = fmt.Sprintf("failed to parse response: %s", string(body))
		return result, fmt.Errorf("failed to parse Twilio response: %w", err)
	}

	if resp.StatusCode >= 400 {
		result.Error = twilioResp.ErrorMessage
		return result, fmt.Errorf("twilio call error: %s (code: %d)", twilioResp.ErrorMessage, twilioResp.ErrorCode)
	}

	result.Success = true
	result.CallSID = twilioResp.SID
	result.Status = twilioResp.Status

	log.Printf("📞 Ligação de alerta iniciada para %s - SID: %s", toNumber, twilioResp.SID)
	return result, nil
}

// GetCallStatus retrieves the current status of a call
func (s *TwilioService) GetCallStatus(callSID string) (string, error) {
	endpoint := fmt.Sprintf("%s/Calls/%s.json", s.baseURL, callSID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(s.config.AccountSID, s.config.AuthToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get call status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var twilioResp TwilioCallResponse
	if err := json.Unmarshal(body, &twilioResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return twilioResp.Status, nil
}

// CancelCall cancels an in-progress call
func (s *TwilioService) CancelCall(callSID string) error {
	endpoint := fmt.Sprintf("%s/Calls/%s.json", s.baseURL, callSID)

	data := url.Values{}
	data.Set("Status", "canceled")

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(s.config.AccountSID, s.config.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to cancel call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cancel call: %s", string(body))
	}

	log.Printf("📞 Ligação cancelada: %s", callSID)
	return nil
}
