// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Rate Limiter in-memory para ferramentas.
// Limita invocacoes por minuto e por hora por (tool + idosoID).

package tools

import (
	"fmt"
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateBucket
}

type rateBucket struct {
	minuteCount int
	hourCount   int
	minuteReset time.Time
	hourReset   time.Time
}

var globalRateLimiter = &rateLimiter{
	entries: make(map[string]*rateBucket),
}

// rateLimits define limites padrao por tool (pode ser sobrescrito pelo DB)
// Tools sem entrada aqui nao tem rate limit.
var rateLimits = map[string][2]int{
	// [per_minute, per_hour]
	"send_email":     {5, 30},
	"send_whatsapp":  {10, 60},
	"send_telegram":  {10, 60},
	"send_slack":     {10, 60},
	"send_discord":   {10, 60},
	"send_teams":     {10, 60},
	"send_signal":    {10, 60},
	"execute_code":   {10, 100},
	"query_postgresql": {30, 300},
	"query_nietzsche_graph":  {30, 300},
	"query_nietzsche_vector": {30, 300},
	"mcp_query_nietzsche_core": {30, 300},
	"mcp_teach_eva":  {10, 50},
	"mcp_learn_topic": {3, 20},
	"mcp_edit_source": {10, 60},
	"ask_llm":        {10, 60},
	"alert_family":   {5, 20},
	"create_webhook": {5, 30},
	"trigger_webhook": {10, 60},
}

// checkRateLimit retorna erro se o limite foi excedido
func checkRateLimit(toolName string, idosoID int64) error {
	limits, ok := rateLimits[toolName]
	if !ok {
		return nil // sem rate limit para esta tool
	}

	perMinute := limits[0]
	perHour := limits[1]

	key := fmt.Sprintf("%s:%d", toolName, idosoID)
	now := time.Now()

	globalRateLimiter.mu.Lock()
	defer globalRateLimiter.mu.Unlock()

	bucket, exists := globalRateLimiter.entries[key]
	if !exists {
		bucket = &rateBucket{
			minuteReset: now.Add(time.Minute),
			hourReset:   now.Add(time.Hour),
		}
		globalRateLimiter.entries[key] = bucket
	}

	// Resetar contadores expirados
	if now.After(bucket.minuteReset) {
		bucket.minuteCount = 0
		bucket.minuteReset = now.Add(time.Minute)
	}
	if now.After(bucket.hourReset) {
		bucket.hourCount = 0
		bucket.hourReset = now.Add(time.Hour)
	}

	// Verificar limites
	if perMinute > 0 && bucket.minuteCount >= perMinute {
		return fmt.Errorf("rate limit excedido: %s limitado a %d/min", toolName, perMinute)
	}
	if perHour > 0 && bucket.hourCount >= perHour {
		return fmt.Errorf("rate limit excedido: %s limitado a %d/hora", toolName, perHour)
	}

	// Incrementar contadores
	bucket.minuteCount++
	bucket.hourCount++

	return nil
}
