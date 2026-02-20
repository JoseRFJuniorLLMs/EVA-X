// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"eva-mind/internal/brainstem/logger"
)

const (
	defaultTimeout = 10 * time.Second
)

// Client provides HTTP access to NietzscheDB's dashboard API.
// NietzscheDB is a database with biological memory concepts (sleep/consolidation)
// that replaces Neo4j + Qdrant + Redis in the EVA infrastructure.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new NietzscheDB HTTP client.
// The baseURL should point to the HTTP dashboard (e.g., "http://localhost:8082").
func NewClient(baseURL string) *Client {
	log := logger.Nietzsche()
	log.Info().Str("base_url", baseURL).Msg("NietzscheDB client initialized")

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// NewClientWithTimeout creates a new NietzscheDB HTTP client with a custom timeout.
func NewClientWithTimeout(baseURL string, timeout time.Duration) *Client {
	log := logger.Nietzsche()
	log.Info().
		Str("base_url", baseURL).
		Dur("timeout", timeout).
		Msg("NietzscheDB client initialized with custom timeout")

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Close releases any resources held by the client.
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// Health checks if NietzscheDB is reachable and healthy.
func (c *Client) Health(ctx context.Context) error {
	log := logger.Nietzsche()

	url := fmt.Sprintf("%s/api/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to create health check request")
		return fmt.Errorf("nietzsche health check: failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("NietzscheDB health check failed")
		return fmt.Errorf("nietzsche health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("body", string(body)).
			Msg("NietzscheDB health check returned non-200")
		return fmt.Errorf("nietzsche health check: unexpected status %d", resp.StatusCode)
	}

	log.Debug().Msg("NietzscheDB health check OK")
	return nil
}

// Store persists a memory/fact into a NietzscheDB collection.
func (c *Client) Store(ctx context.Context, collection string, key string, value interface{}) error {
	log := logger.Nietzsche()

	url := fmt.Sprintf("%s/api/collections/%s/records", c.baseURL, collection)

	payload := map[string]interface{}{
		"key":   key,
		"value": value,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Str("key", key).Msg("failed to marshal store payload")
		return fmt.Errorf("nietzsche store: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Error().Err(err).Msg("failed to create store request")
		return fmt.Errorf("nietzsche store: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB store request failed")
		return fmt.Errorf("nietzsche store: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("body", string(respBody)).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB store returned error")
		return fmt.Errorf("nietzsche store: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Info().
		Str("collection", collection).
		Str("key", key).
		Msg("NietzscheDB store completed")
	return nil
}

// Get retrieves a memory/fact from a NietzscheDB collection by key.
func (c *Client) Get(ctx context.Context, collection string, key string) (map[string]interface{}, error) {
	log := logger.Nietzsche()

	url := fmt.Sprintf("%s/api/collections/%s/records/%s", c.baseURL, collection, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to create get request")
		return nil, fmt.Errorf("nietzsche get: failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB get request failed")
		return nil, fmt.Errorf("nietzsche get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Debug().
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB record not found")
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("body", string(respBody)).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB get returned error")
		return nil, fmt.Errorf("nietzsche get: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("key", key).
			Msg("failed to decode NietzscheDB get response")
		return nil, fmt.Errorf("nietzsche get: failed to decode response: %w", err)
	}

	log.Debug().
		Str("collection", collection).
		Str("key", key).
		Msg("NietzscheDB get completed")
	return result, nil
}

// Search queries memories in a NietzscheDB collection using a text query.
func (c *Client) Search(ctx context.Context, collection string, query string, limit int) ([]map[string]interface{}, error) {
	log := logger.Nietzsche()

	url := fmt.Sprintf("%s/api/collections/%s/search", c.baseURL, collection)

	payload := map[string]interface{}{
		"query": query,
		"limit": limit,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Msg("failed to marshal search payload")
		return nil, fmt.Errorf("nietzsche search: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Error().Err(err).Msg("failed to create search request")
		return nil, fmt.Errorf("nietzsche search: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("query", query).
			Msg("NietzscheDB search request failed")
		return nil, fmt.Errorf("nietzsche search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("body", string(respBody)).
			Str("collection", collection).
			Str("query", query).
			Msg("NietzscheDB search returned error")
		return nil, fmt.Errorf("nietzsche search: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("query", query).
			Msg("failed to decode NietzscheDB search response")
		return nil, fmt.Errorf("nietzsche search: failed to decode response: %w", err)
	}

	log.Info().
		Str("collection", collection).
		Str("query", query).
		Int("results", len(results)).
		Msg("NietzscheDB search completed")
	return results, nil
}

// Delete removes a memory/fact from a NietzscheDB collection by key.
func (c *Client) Delete(ctx context.Context, collection string, key string) error {
	log := logger.Nietzsche()

	url := fmt.Sprintf("%s/api/collections/%s/records/%s", c.baseURL, collection, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to create delete request")
		return fmt.Errorf("nietzsche delete: failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB delete request failed")
		return fmt.Errorf("nietzsche delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("body", string(respBody)).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB delete returned error")
		return fmt.Errorf("nietzsche delete: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Info().
		Str("collection", collection).
		Str("key", key).
		Msg("NietzscheDB delete completed")
	return nil
}

// GetStats returns database statistics from NietzscheDB.
func (c *Client) GetStats(ctx context.Context) (map[string]interface{}, error) {
	log := logger.Nietzsche()

	url := fmt.Sprintf("%s/api/stats", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to create stats request")
		return nil, fmt.Errorf("nietzsche stats: failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("NietzscheDB stats request failed")
		return nil, fmt.Errorf("nietzsche stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("body", string(respBody)).
			Msg("NietzscheDB stats returned error")
		return nil, fmt.Errorf("nietzsche stats: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		log.Error().Err(err).Msg("failed to decode NietzscheDB stats response")
		return nil, fmt.Errorf("nietzsche stats: failed to decode response: %w", err)
	}

	log.Debug().Msg("NietzscheDB stats retrieved")
	return stats, nil
}
