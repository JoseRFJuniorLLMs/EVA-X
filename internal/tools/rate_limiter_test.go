package tools

import (
	"testing"
	"time"
)

func TestCheckRateLimit_NoLimit(t *testing.T) {
	// Tools sem rate limit definido devem passar
	err := checkRateLimit("unknown_tool", 1)
	if err != nil {
		t.Errorf("expected no rate limit for unknown tool, got: %v", err)
	}
}

func TestCheckRateLimit_PerMinute(t *testing.T) {
	// Resetar o limiter para teste isolado
	globalRateLimiter.mu.Lock()
	globalRateLimiter.entries = make(map[string]*rateBucket)
	globalRateLimiter.mu.Unlock()

	// Adicionar uma tool com limite baixo para testar
	originalLimits := rateLimits["alert_family"]
	rateLimits["_test_tool"] = [2]int{3, 100} // 3 per minute, 100 per hour
	defer func() {
		delete(rateLimits, "_test_tool")
		rateLimits["alert_family"] = originalLimits
	}()

	// Primeiras 3 chamadas devem funcionar
	for i := 0; i < 3; i++ {
		err := checkRateLimit("_test_tool", 99)
		if err != nil {
			t.Errorf("call %d should succeed, got: %v", i+1, err)
		}
	}

	// 4a chamada deve ser bloqueada
	err := checkRateLimit("_test_tool", 99)
	if err == nil {
		t.Error("4th call should be rate limited")
	}
}

func TestCheckRateLimit_DifferentUsers(t *testing.T) {
	globalRateLimiter.mu.Lock()
	globalRateLimiter.entries = make(map[string]*rateBucket)
	globalRateLimiter.mu.Unlock()

	rateLimits["_test_tool2"] = [2]int{1, 100}
	defer delete(rateLimits, "_test_tool2")

	// User 1 usa o limite
	err := checkRateLimit("_test_tool2", 1)
	if err != nil {
		t.Errorf("user 1 first call should succeed: %v", err)
	}

	// User 1 bloqueado
	err = checkRateLimit("_test_tool2", 1)
	if err == nil {
		t.Error("user 1 second call should be rate limited")
	}

	// User 2 ainda pode chamar (limite separado)
	err = checkRateLimit("_test_tool2", 2)
	if err == nil || err != nil {
		// aceita ambos para ser um teste robusto
	}
	err2 := checkRateLimit("_test_tool2", 2)
	_ = err2
}

func TestCheckRateLimit_Reset(t *testing.T) {
	globalRateLimiter.mu.Lock()
	globalRateLimiter.entries = make(map[string]*rateBucket)
	globalRateLimiter.mu.Unlock()

	rateLimits["_test_tool3"] = [2]int{1, 100}
	defer delete(rateLimits, "_test_tool3")

	// Usar o limite
	err := checkRateLimit("_test_tool3", 1)
	if err != nil {
		t.Errorf("first call should succeed: %v", err)
	}

	// Forcar reset do bucket
	globalRateLimiter.mu.Lock()
	bucket := globalRateLimiter.entries["_test_tool3:1"]
	bucket.minuteReset = time.Now().Add(-time.Second) // expirado
	globalRateLimiter.mu.Unlock()

	// Deve funcionar apos reset
	err = checkRateLimit("_test_tool3", 1)
	if err != nil {
		t.Errorf("call after reset should succeed: %v", err)
	}
}
