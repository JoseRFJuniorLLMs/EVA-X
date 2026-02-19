// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package smarthome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Device dispositivo IoT
type Device struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Domain     string `json:"domain"`     // light, switch, climate, sensor, etc
	State      string `json:"state"`      // on, off, 23.5, etc
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Service cliente para Home Assistant REST API
type Service struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewService cria smart home service via Home Assistant
func NewService(baseURL, token string) *Service {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &Service{
		baseURL: baseURL,
		token:   token,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// ListDevices lista todos os dispositivos
func (s *Service) ListDevices() ([]Device, error) {
	body, err := s.apiGet("/api/states")
	if err != nil {
		return nil, err
	}

	var states []struct {
		EntityID   string                 `json:"entity_id"`
		State      string                 `json:"state"`
		Attributes map[string]interface{} `json:"attributes"`
	}
	if err := json.Unmarshal(body, &states); err != nil {
		return nil, fmt.Errorf("erro ao parsear devices: %v", err)
	}

	var devices []Device
	for _, state := range states {
		parts := strings.SplitN(state.EntityID, ".", 2)
		domain := ""
		name := state.EntityID
		if len(parts) == 2 {
			domain = parts[0]
			name = parts[1]
		}

		// Usar friendly_name se disponível
		if fn, ok := state.Attributes["friendly_name"].(string); ok {
			name = fn
		}

		devices = append(devices, Device{
			ID:         state.EntityID,
			Name:       name,
			Domain:     domain,
			State:      state.State,
			Attributes: state.Attributes,
		})
	}

	return devices, nil
}

// GetDeviceState obtém estado de um dispositivo
func (s *Service) GetDeviceState(entityID string) (*Device, error) {
	body, err := s.apiGet("/api/states/" + entityID)
	if err != nil {
		return nil, err
	}

	var state struct {
		EntityID   string                 `json:"entity_id"`
		State      string                 `json:"state"`
		Attributes map[string]interface{} `json:"attributes"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("erro ao parsear estado: %v", err)
	}

	parts := strings.SplitN(state.EntityID, ".", 2)
	domain := ""
	name := state.EntityID
	if len(parts) == 2 {
		domain = parts[0]
		name = parts[1]
	}
	if fn, ok := state.Attributes["friendly_name"].(string); ok {
		name = fn
	}

	return &Device{
		ID:         state.EntityID,
		Name:       name,
		Domain:     domain,
		State:      state.State,
		Attributes: state.Attributes,
	}, nil
}

// ControlDevice controla um dispositivo (ligar, desligar, ajustar)
func (s *Service) ControlDevice(entityID, action string, data map[string]interface{}) error {
	parts := strings.SplitN(entityID, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("entity_id inválido: %s (formato: domain.name)", entityID)
	}
	domain := parts[0]

	// Mapear ações comuns
	service := action
	switch strings.ToLower(action) {
	case "on", "ligar", "turn_on":
		service = "turn_on"
	case "off", "desligar", "turn_off":
		service = "turn_off"
	case "toggle", "alternar":
		service = "toggle"
	}

	payload := map[string]interface{}{
		"entity_id": entityID,
	}
	// Merge data extra (brightness, temperature, etc)
	for k, v := range data {
		payload[k] = v
	}

	_, err := s.apiPost(fmt.Sprintf("/api/services/%s/%s", domain, service), payload)
	return err
}

// apiGet faz GET na API do Home Assistant
func (s *Service) apiGet(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", s.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("home assistant unreachable: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("home assistant error (%d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// apiPost faz POST na API do Home Assistant
func (s *Service) apiPost(path string, payload interface{}) ([]byte, error) {
	jsonBody, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", s.baseURL+path, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("home assistant unreachable: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("home assistant error (%d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}
