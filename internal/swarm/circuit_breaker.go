package swarm

import (
	"sync"
	"time"
)

// CircuitState representa o estado do circuit breaker
type CircuitState int

const (
	CircuitClosed   CircuitState = 0 // Normal - requests passam
	CircuitOpen     CircuitState = 1 // Aberto - requests bloqueados
	CircuitHalfOpen CircuitState = 2 // Semi-aberto - testando recovery
)

// CircuitBreaker protege swarms contra falhas em cascata
type CircuitBreaker struct {
	mu       sync.RWMutex
	circuits map[string]*circuit

	// Configuração
	failureThreshold int           // Falhas consecutivas para abrir
	resetTimeout     time.Duration // Tempo até tentar half-open
	successThreshold int           // Sucessos em half-open para fechar
}

type circuit struct {
	state            CircuitState
	failures         int
	successes        int
	lastFailure      time.Time
	lastStateChange  time.Time
}

// NewCircuitBreaker cria um novo circuit breaker
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		circuits:         make(map[string]*circuit),
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		successThreshold: 2,
	}
}

// IsOpen verifica se o circuit está aberto (bloqueando requests)
func (cb *CircuitBreaker) IsOpen(swarmName string) bool {
	cb.mu.RLock()
	c, exists := cb.circuits[swarmName]
	cb.mu.RUnlock()

	if !exists {
		return false
	}

	switch c.state {
	case CircuitClosed:
		return false
	case CircuitOpen:
		// Verificar se já passou o resetTimeout
		if time.Since(c.lastStateChange) > cb.resetTimeout {
			cb.mu.Lock()
			c.state = CircuitHalfOpen
			c.successes = 0
			c.lastStateChange = time.Now()
			cb.mu.Unlock()
			return false // Permitir tentativa
		}
		return true
	case CircuitHalfOpen:
		return false // Permitir tentativa
	}

	return false
}

// RecordSuccess registra uma execução bem-sucedida
func (cb *CircuitBreaker) RecordSuccess(swarmName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	c := cb.getOrCreate(swarmName)

	switch c.state {
	case CircuitHalfOpen:
		c.successes++
		if c.successes >= cb.successThreshold {
			c.state = CircuitClosed
			c.failures = 0
			c.successes = 0
			c.lastStateChange = time.Now()
		}
	case CircuitClosed:
		c.failures = 0 // Reset consecutive failures
	}
}

// RecordFailure registra uma falha de execução
func (cb *CircuitBreaker) RecordFailure(swarmName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	c := cb.getOrCreate(swarmName)
	c.failures++
	c.lastFailure = time.Now()

	switch c.state {
	case CircuitClosed:
		if c.failures >= cb.failureThreshold {
			c.state = CircuitOpen
			c.lastStateChange = time.Now()
		}
	case CircuitHalfOpen:
		// Uma falha em half-open reabre o circuit
		c.state = CircuitOpen
		c.lastStateChange = time.Now()
	}
}

// State retorna o estado atual do circuit de um swarm
func (cb *CircuitBreaker) State(swarmName string) CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	c, exists := cb.circuits[swarmName]
	if !exists {
		return CircuitClosed
	}
	return c.state
}

// Reset força o reset de um circuit (manual override)
func (cb *CircuitBreaker) Reset(swarmName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if c, exists := cb.circuits[swarmName]; exists {
		c.state = CircuitClosed
		c.failures = 0
		c.successes = 0
		c.lastStateChange = time.Now()
	}
}

func (cb *CircuitBreaker) getOrCreate(swarmName string) *circuit {
	c, exists := cb.circuits[swarmName]
	if !exists {
		c = &circuit{
			state:           CircuitClosed,
			lastStateChange: time.Now(),
		}
		cb.circuits[swarmName] = c
	}
	return c
}
