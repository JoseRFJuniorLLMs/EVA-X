// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Webhook definição de um webhook
type Webhook struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Secret    string    `json:"secret"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	LastFired time.Time `json:"last_fired,omitempty"`
	FireCount int       `json:"fire_count"`
}

// DeliveryResult resultado de entrega de webhook
type DeliveryResult struct {
	WebhookID  string `json:"webhook_id"`
	StatusCode int    `json:"status_code"`
	Duration   int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Service gerencia webhooks outgoing
type Service struct {
	webhooks map[string]*Webhook
	mu       sync.RWMutex
	client   *http.Client
}

// NewService cria webhook service
func NewService() *Service {
	return &Service{
		webhooks: make(map[string]*Webhook),
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Create cria novo webhook
func (s *Service) Create(name, url string, events []string) (*Webhook, error) {
	if name == "" || url == "" {
		return nil, fmt.Errorf("name e url são obrigatórios")
	}

	wh := &Webhook{
		ID:        uuid.New().String()[:8],
		Name:      name,
		URL:       url,
		Events:    events,
		Secret:    uuid.New().String(),
		Active:    true,
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.webhooks[wh.ID] = wh
	s.mu.Unlock()

	log.Printf("🔗 [WEBHOOK] Criado: %s → %s (events: %v)", wh.Name, wh.URL, events)
	return wh, nil
}

// List lista todos webhooks
func (s *Service) List() []*Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Webhook
	for _, wh := range s.webhooks {
		result = append(result, wh)
	}
	return result
}

// Delete remove um webhook
func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.webhooks[id]; !ok {
		return fmt.Errorf("webhook não encontrado: %s", id)
	}
	delete(s.webhooks, id)
	return nil
}

// Trigger dispara todos webhooks registrados para um evento
func (s *Service) Trigger(event string, payload map[string]interface{}) []DeliveryResult {
	s.mu.RLock()
	var targets []*Webhook
	for _, wh := range s.webhooks {
		if !wh.Active {
			continue
		}
		for _, e := range wh.Events {
			if e == event || e == "*" {
				targets = append(targets, wh)
				break
			}
		}
	}
	s.mu.RUnlock()

	var results []DeliveryResult
	for _, wh := range targets {
		result := s.deliver(wh, event, payload)
		results = append(results, result)
	}
	return results
}

// TriggerByName dispara webhook específico por nome
func (s *Service) TriggerByName(name string, payload map[string]interface{}) (*DeliveryResult, error) {
	s.mu.RLock()
	var target *Webhook
	for _, wh := range s.webhooks {
		if wh.Name == name && wh.Active {
			target = wh
			break
		}
	}
	s.mu.RUnlock()

	if target == nil {
		return nil, fmt.Errorf("webhook '%s' não encontrado", name)
	}

	result := s.deliver(target, "manual", payload)
	return &result, nil
}

// deliver entrega um webhook
func (s *Service) deliver(wh *Webhook, event string, payload map[string]interface{}) DeliveryResult {
	body := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      payload,
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", wh.URL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return DeliveryResult{WebhookID: wh.ID, Error: err.Error()}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-EVA-Event", event)
	req.Header.Set("X-EVA-Webhook-ID", wh.ID)

	// HMAC signature
	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(jsonBody)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-EVA-Signature", "sha256="+signature)
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ [WEBHOOK] Falha ao entregar %s → %s: %v", wh.Name, wh.URL, err)
		return DeliveryResult{WebhookID: wh.ID, Duration: duration.Milliseconds(), Error: err.Error()}
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // drain

	// Atualizar stats
	s.mu.Lock()
	wh.LastFired = time.Now()
	wh.FireCount++
	s.mu.Unlock()

	log.Printf("🔗 [WEBHOOK] Entregue: %s → %s (%d) em %dms", wh.Name, wh.URL, resp.StatusCode, duration.Milliseconds())

	return DeliveryResult{
		WebhookID:  wh.ID,
		StatusCode: resp.StatusCode,
		Duration:   duration.Milliseconds(),
	}
}

// HandleIncoming processa webhook incoming (para ser montado no router)
func (s *Service) HandleIncoming(handler func(event string, payload map[string]interface{})) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, _ := io.ReadAll(io.LimitReader(r.Body, 1*1024*1024))

		var payload struct {
			Event string                 `json:"event"`
			Data  map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		go handler(payload.Event, payload.Data)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
	}
}
