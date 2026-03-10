// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consciousness

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ============================================================================
// ThoughtBus — Barramento Cognitivo do Cognitive Operating System (COS)
// ============================================================================
//
// O ThoughtBus e o sistema nervoso central da EVA. Modulos cognitivos
// independentes (Lacan, Bayesian, SpeakerRecognition, Swarm Agents)
// publicam ThoughtEvents no bus, que distribui para subscribers interessados.
//
// O GlobalWorkspace subscreve ao bus como listener global ("*") e actua
// como Attention Scheduler, decidindo quais pensamentos ganham o "foco"
// e sao broadcast para o LLM (Gemini) para gerar resposta.
//
// Arquitectura:
//
//	Perception → ThoughtBus → GlobalWorkspace (attention) → Action
//	                ↑                    ↓
//	           CognitiveModules    Memory Update
//
// Ciencia: Baars (1988) Global Workspace Theory + Dehaene (2014) Neural Workspace
// Engineering: Go channels com buffer + goroutines isoladas com recover()

// ThoughtType define a categoria do pensamento para roteamento no barramento
type ThoughtType string

const (
	// Perception — entradas sensoriais (voz, imagem, texto, telemetria)
	Perception ThoughtType = "perception"

	// Inference — deducoes dos motores cognitivos (Lacan, Bayesian, FDPN)
	Inference ThoughtType = "inference"

	// Intent — intencoes de acao para o Motor Cortex (tool calls, responses)
	Intent ThoughtType = "intent"

	// Reflection — processos de autoconsciencia e memoria meta-cognitiva
	Reflection ThoughtType = "reflection"

	// Memory — eventos de activacao/consolidacao de memoria
	Memory ThoughtType = "memory"

	// Emotion — mudancas no estado emocional detectadas
	Emotion ThoughtType = "emotion"

	// Global — listener especial que recebe TODOS os pensamentos (ex: GlobalWorkspace)
	Global ThoughtType = "*"
)

// ThoughtEvent representa a unidade fundamental de processamento cognitivo.
// Cada evento carrega metadados geometricos (para homeostase hiperbolica)
// e causais (para rastreabilidade Lacaniana via Manifold/NietzscheDB).
type ThoughtEvent struct {
	// Identidade
	ID            string      `json:"id"`               // UUID unico do pensamento
	CausalChainID string      `json:"causal_chain_id"`  // ID para rastreamento no Manifold do NietzscheDB
	Source        string      `json:"source"`            // Modulo de origem (ex: "lacan_engine", "speaker_svc")
	Type          ThoughtType `json:"type"`              // Categoria para roteamento

	// Conteudo
	Payload interface{} `json:"payload"` // Dados brutos do pensamento (tipado por Source)

	// Metadados para o Attention Scheduler
	Salience   float64 `json:"salience"`    // Importancia intrinseca (0.0 a 1.0)
	EnergyCost float64 `json:"energy_cost"` // Custo computacional estimado para processar

	// Temporal
	Timestamp time.Time `json:"timestamp"`

	// Contexto opcional
	SessionID string `json:"session_id,omitempty"` // Sessao que gerou o pensamento
	UserID    string `json:"user_id,omitempty"`    // CPF/ID do usuario associado
}

// ThoughtListener e a assinatura para funcoes que consomem pensamentos
type ThoughtListener func(ThoughtEvent)

// ThoughtBus implementa o barramento de mensagens cognitivas com:
// - Channels com buffer para evitar bloqueios entre modulos
// - Goroutines isoladas com recover() para robustez
// - Metricas de throughput e drops para observabilidade
type ThoughtBus struct {
	subscribers map[ThoughtType][]ThoughtListener
	mu          sync.RWMutex
	inputChan   chan ThoughtEvent
	ctx         context.Context
	cancel      context.CancelFunc

	// Metricas
	published atomic.Int64
	delivered atomic.Int64
	dropped   atomic.Int64
}

// NewThoughtBus inicializa o barramento cognitivo.
// bufferSize controla quantos pensamentos podem ser enfileirados sem bloqueio.
// Recomendado: 256 para producao, 64 para testes.
func NewThoughtBus(parentCtx context.Context, bufferSize int) *ThoughtBus {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	ctx, cancel := context.WithCancel(parentCtx)
	tb := &ThoughtBus{
		subscribers: make(map[ThoughtType][]ThoughtListener),
		inputChan:   make(chan ThoughtEvent, bufferSize),
		ctx:         ctx,
		cancel:      cancel,
	}
	go tb.processLoop()
	log.Info().Int("buffer", bufferSize).Msg("[ThoughtBus] Barramento cognitivo iniciado")
	return tb
}

// Publish envia um pensamento para o barramento de forma nao-bloqueante.
// Se o buffer estiver cheio, o pensamento e descartado para preservar
// homeostase cognitiva (evita backpressure que pararia a percepcao).
func (tb *ThoughtBus) Publish(event ThoughtEvent) {
	// Garantir ID
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	tb.published.Add(1)

	select {
	case tb.inputChan <- event:
		// Publicado com sucesso
	case <-tb.ctx.Done():
		return
	default:
		// Buffer cheio — descarta pensamento de baixa saliencia para manter homeostase
		tb.dropped.Add(1)
		log.Warn().
			Str("source", event.Source).
			Str("type", string(event.Type)).
			Float64("salience", event.Salience).
			Msg("[ThoughtBus] Pensamento descartado (buffer cheio)")
	}
}

// Subscribe regista um listener para um tipo especifico de pensamento.
// Use Global ("*") para receber todos os pensamentos (ex: GlobalWorkspace).
func (tb *ThoughtBus) Subscribe(t ThoughtType, listener ThoughtListener) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.subscribers[t] = append(tb.subscribers[t], listener)
	log.Debug().Str("type", string(t)).Msg("[ThoughtBus] Subscriber registado")
}

// processLoop distribui os pensamentos para os subscritores em goroutines isoladas.
// Cada listener corre na sua propria goroutine com recover() para que um modulo
// cognitivo com panic nao derrube o kernel.
func (tb *ThoughtBus) processLoop() {
	for {
		select {
		case <-tb.ctx.Done():
			log.Info().
				Int64("published", tb.published.Load()).
				Int64("delivered", tb.delivered.Load()).
				Int64("dropped", tb.dropped.Load()).
				Msg("[ThoughtBus] Barramento cognitivo encerrado")
			return
		case event := <-tb.inputChan:
			tb.mu.RLock()

			// Subscritores do tipo especifico
			listeners := make([]ThoughtListener, 0)
			if typeListeners, ok := tb.subscribers[event.Type]; ok {
				listeners = append(listeners, typeListeners...)
			}
			// Subscritores globais (GlobalWorkspace = attention scheduler)
			if globalListeners, ok := tb.subscribers[Global]; ok {
				listeners = append(listeners, globalListeners...)
			}

			tb.mu.RUnlock()

			for _, listener := range listeners {
				tb.delivered.Add(1)
				l := listener // Captura local para goroutine
				go func(e ThoughtEvent) {
					defer func() {
						if r := recover(); r != nil {
							log.Error().
								Str("source", e.Source).
								Interface("panic", r).
								Msg("[ThoughtBus] Panic em listener cognitivo (recuperado)")
						}
					}()
					l(e)
				}(event)
			}
		}
	}
}

// Stop encerra o barramento cognitivo de forma limpa.
func (tb *ThoughtBus) Stop() {
	tb.cancel()
}

// Metrics retorna metricas do barramento para monitoramento.
func (tb *ThoughtBus) Metrics() map[string]interface{} {
	return map[string]interface{}{
		"published":    tb.published.Load(),
		"delivered":    tb.delivered.Load(),
		"dropped":      tb.dropped.Load(),
		"buffer_size":  cap(tb.inputChan),
		"buffer_usage": len(tb.inputChan),
		"subscribers":  tb.subscriberCount(),
	}
}

func (tb *ThoughtBus) subscriberCount() int {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	count := 0
	for _, listeners := range tb.subscribers {
		count += len(listeners)
	}
	return count
}

// --- Helper: criar pensamentos padronizados ---

// NewThought cria um ThoughtEvent com defaults sensatos.
func NewThought(source string, t ThoughtType, payload interface{}, salience float64) ThoughtEvent {
	return ThoughtEvent{
		ID:        uuid.New().String(),
		Source:    source,
		Type:      t,
		Payload:   payload,
		Salience:  salience,
		Timestamp: time.Now(),
	}
}

// NewPerception cria um pensamento de percepcao (entrada sensorial).
func NewPerception(source string, payload interface{}, salience float64) ThoughtEvent {
	return NewThought(source, Perception, payload, salience)
}

// NewInference cria um pensamento de inferencia (deducao de modulo cognitivo).
func NewInference(source string, payload interface{}, salience float64) ThoughtEvent {
	return NewThought(source, Inference, payload, salience)
}

// NewIntent cria um pensamento de intencao (acao a executar).
func NewIntent(source string, payload interface{}, salience float64) ThoughtEvent {
	return NewThought(source, Intent, payload, salience)
}

// NewReflection cria um pensamento de reflexao (meta-cognicao).
func NewReflection(source string, payload interface{}, salience float64) ThoughtEvent {
	return NewThought(source, Reflection, payload, salience)
}
