package retry

import (
	"context"
	"errors"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

// ============================================================================
// PERFORMANCE FIX: Retry com Exponential Backoff
// Issue: Falhas transientes causam erro imediato sem recuperacao
// Fix: Retry inteligente com backoff apenas para erros recuperaveis
// ============================================================================

// Config define configuracao do retry
type Config struct {
	MaxRetries     int           // Numero maximo de tentativas
	InitialBackoff time.Duration // Backoff inicial
	MaxBackoff     time.Duration // Backoff maximo
	Multiplier     float64       // Multiplicador do backoff (ex: 2.0)
	Jitter         float64       // Jitter (0.0-1.0) para evitar thundering herd
}

// DefaultConfig retorna configuracao padrao
func DefaultConfig() Config {
	return Config{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.2,
	}
}

// FastConfig para operacoes que precisam resposta rapida
func FastConfig() Config {
	return Config{
		MaxRetries:     2,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0.1,
	}
}

// SlowConfig para operacoes que podem esperar mais
func SlowConfig() Config {
	return Config{
		MaxRetries:     5,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.3,
	}
}

// Operation representa uma operacao que pode ser retried
type Operation func(ctx context.Context) error

// Do executa a operacao com retry
func Do(ctx context.Context, cfg Config, op Operation) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Executar operacao
		err := op(ctx)
		if err == nil {
			return nil // Sucesso!
		}

		lastErr = err

		// Verificar se erro e recuperavel
		if !IsRetryable(err) {
			log.Printf("⚠️ [RETRY] Erro nao-recuperavel: %v", err)
			return err
		}

		// Verificar contexto cancelado
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Se foi a ultima tentativa, retornar erro
		if attempt == cfg.MaxRetries {
			break
		}

		// Calcular backoff
		backoff := calculateBackoff(cfg, attempt)
		log.Printf("🔄 [RETRY] Tentativa %d/%d falhou, aguardando %v: %v",
			attempt+1, cfg.MaxRetries+1, backoff, err)

		// Aguardar com timeout do contexto
		select {
		case <-time.After(backoff):
			// Continua para proxima tentativa
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

// DoWithResult executa operacao que retorna resultado
func DoWithResult[T any](ctx context.Context, cfg Config, op func(ctx context.Context) (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		res, err := op(ctx)
		if err == nil {
			return res, nil
		}

		lastErr = err

		if !IsRetryable(err) {
			return result, err
		}

		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		if attempt == cfg.MaxRetries {
			break
		}

		backoff := calculateBackoff(cfg, attempt)
		log.Printf("🔄 [RETRY] Tentativa %d/%d falhou, aguardando %v",
			attempt+1, cfg.MaxRetries+1, backoff)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return result, ctx.Err()
		}
	}

	return result, lastErr
}

// calculateBackoff calcula tempo de espera com jitter
func calculateBackoff(cfg Config, attempt int) time.Duration {
	// Exponential backoff: initial * multiplier^attempt
	backoff := float64(cfg.InitialBackoff) * math.Pow(cfg.Multiplier, float64(attempt))

	// Aplicar max cap
	if backoff > float64(cfg.MaxBackoff) {
		backoff = float64(cfg.MaxBackoff)
	}

	// Adicionar jitter para evitar thundering herd
	if cfg.Jitter > 0 {
		jitter := backoff * cfg.Jitter * (rand.Float64()*2 - 1) // -jitter to +jitter
		backoff += jitter
	}

	return time.Duration(backoff)
}

// ============================================================================
// CLASSIFICACAO DE ERROS
// ============================================================================

// RetryableError marca um erro como retryable
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// PermanentError marca um erro como permanente (nao retry)
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

// MarkRetryable marca erro como retryable
func MarkRetryable(err error) error {
	return &RetryableError{Err: err}
}

// MarkPermanent marca erro como permanente
func MarkPermanent(err error) error {
	return &PermanentError{Err: err}
}

// IsRetryable verifica se erro e recuperavel
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Verificar se foi marcado explicitamente
	var permanent *PermanentError
	if errors.As(err, &permanent) {
		return false
	}

	var retryable *RetryableError
	if errors.As(err, &retryable) {
		return true
	}

	// Erros de rede sao geralmente retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}

	// Verificar erros especificos por string (fallback)
	errStr := strings.ToLower(err.Error())

	// Erros retryable
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no such host",
		"temporary failure",
		"service unavailable",
		"too many requests",
		"rate limit",
		"429", // Too Many Requests
		"503", // Service Unavailable
		"502", // Bad Gateway
		"504", // Gateway Timeout
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Erros permanentes (nao retry)
	permanentPatterns := []string{
		"invalid",
		"unauthorized",
		"forbidden",
		"not found",
		"bad request",
		"400",
		"401",
		"403",
		"404",
		"422",
	}

	for _, pattern := range permanentPatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// Default: nao retry para evitar loops infinitos
	return false
}

// IsHTTPRetryable verifica se status HTTP e retryable
func IsHTTPRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,        // 429
		http.StatusServiceUnavailable,      // 503
		http.StatusBadGateway,              // 502
		http.StatusGatewayTimeout,          // 504
		http.StatusRequestTimeout:          // 408
		return true
	default:
		return false
	}
}

// ============================================================================
// HELPERS
// ============================================================================

// WithRetry wrapper simples para funcoes
func WithRetry(ctx context.Context, maxRetries int, fn func() error) error {
	cfg := DefaultConfig()
	cfg.MaxRetries = maxRetries

	return Do(ctx, cfg, func(ctx context.Context) error {
		return fn()
	})
}

// init inicializa seed do random
func init() {
	rand.Seed(time.Now().UnixNano())
}
