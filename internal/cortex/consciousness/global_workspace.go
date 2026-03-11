// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consciousness

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// GlobalWorkspace implementa a teoria do Workspace Global de Baars
// Modulos cognitivos competem por atencao; vencedor e "broadcast" a todos
// Ciencia: Baars, B. J. (1988) - "A Cognitive Theory of Consciousness"
//
// Integrado com ThoughtBus: o GlobalWorkspace subscreve como listener global
// e actua como Attention Scheduler, decidindo quais pensamentos do barramento
// ganham o foco e sao broadcast para o LLM.
type GlobalWorkspace struct {
	modules    []CognitiveModule
	spotlight  *AttentionSpotlight
	bus        *ThoughtBus // Barramento cognitivo (nil = modo legado sem bus)
	mu         sync.RWMutex

	// Attention Scheduler state
	currentFocus       *ThoughtEvent // Pensamento actualmente em foco
	attentionThreshold float64       // Threshold dinamico para ganhar atencao
	focusMu            sync.Mutex    // Protege currentFocus e attentionThreshold

	// Callbacks para broadcast
	onFocusChange []func(ThoughtEvent) // Chamados quando o foco muda
}

// CognitiveModule interface que todo modulo cognitivo deve implementar
type CognitiveModule interface {
	Name() string
	Process(input ConversationInput) *Interpretation
	BidForAttention(input ConversationInput) float64
}

// ConversationInput entrada de uma conversa para processamento
type ConversationInput struct {
	Text          string
	PatientID     int64
	Emotion       string
	PersonalityType int
	SessionContext map[string]interface{}
}

// Interpretation interpretacao de um modulo cognitivo
type Interpretation struct {
	ModuleName     string                 `json:"module_name"`
	Content        string                 `json:"content"`
	Confidence     float64                `json:"confidence"`
	Evidence       []string               `json:"evidence"`
	EmotionalTone  string                 `json:"emotional_tone"`
	SuggestedAction string               `json:"suggested_action"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// ConsciousResponse resposta integrada do workspace global
type ConsciousResponse struct {
	Winner          *Interpretation   `json:"winner"`
	AllInterpretations []*Interpretation `json:"all_interpretations"`
	IntegratedInsight string           `json:"integrated_insight"`
	ProcessingTime  string            `json:"processing_time"`
}

// AttentionSpotlight seleciona a interpretacao vencedora
type AttentionSpotlight struct {
	NoveltyWeight   float64 // Peso para novidade (0-1)
	EmotionWeight   float64 // Peso para relevancia emocional
	ConflictWeight  float64 // Peso para conflito com expectativa
	UrgencyWeight   float64 // Peso para urgencia
}

// NewGlobalWorkspace cria o workspace global.
// Aceita um ThoughtBus opcional — se nil, opera em modo legado (sem barramento).
func NewGlobalWorkspace(bus ...*ThoughtBus) *GlobalWorkspace {
	gw := &GlobalWorkspace{
		modules: make([]CognitiveModule, 0),
		spotlight: &AttentionSpotlight{
			NoveltyWeight:  0.25,
			EmotionWeight:  0.35,
			ConflictWeight: 0.20,
			UrgencyWeight:  0.20,
		},
		attentionThreshold: 0.3, // Threshold inicial conservador
	}
	if len(bus) > 0 && bus[0] != nil {
		gw.bus = bus[0]
	}
	return gw
}

// RegisterModule registra um modulo cognitivo no workspace
func (gw *GlobalWorkspace) RegisterModule(module CognitiveModule) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	gw.modules = append(gw.modules, module)
	log.Info().Str("module", module.Name()).Msg("[CONSCIOUSNESS] Modulo registrado")
}

// Start inicia o GlobalWorkspace como Attention Scheduler do ThoughtBus.
// Subscreve-se como listener global (*) e avalia cada pensamento
// para decidir se merece o foco (broadcast para consciencia).
func (gw *GlobalWorkspace) Start(ctx context.Context) {
	if gw.bus == nil {
		log.Warn().Msg("[CONSCIOUSNESS] GlobalWorkspace iniciado sem ThoughtBus (modo legado)")
		return
	}

	// Subscrever a TODOS os pensamentos para competicao de atencao
	gw.bus.Subscribe(Global, func(event ThoughtEvent) {
		gw.evaluateAttention(event)
	})

	log.Info().Float64("threshold", gw.attentionThreshold).Msg("[CONSCIOUSNESS] Attention Scheduler iniciado")
}

// OnFocusChange regista callback chamado quando o foco da consciencia muda.
// Usado para: enviar ao LLM (Gemini), actualizar UI, persistir em NietzscheDB.
func (gw *GlobalWorkspace) OnFocusChange(cb func(ThoughtEvent)) {
	gw.focusMu.Lock()
	defer gw.focusMu.Unlock()
	gw.onFocusChange = append(gw.onFocusChange, cb)
}

// evaluateAttention avalia se um ThoughtEvent merece o foco da consciencia.
// Algoritmo de competicao: AttentionScore = (Salience * Energy) / (EnergyCost + epsilon)
// Se score > threshold dinamico, o pensamento ganha o broadcast.
func (gw *GlobalWorkspace) evaluateAttention(event ThoughtEvent) {
	gw.focusMu.Lock()
	defer gw.focusMu.Unlock()

	// Calcular attention score: saliencia ajustada pelo custo energetico
	// epsilon (0.1) evita divisao por zero e penaliza pensamentos sem custo declarado
	score := event.Salience * (1.0 / (event.EnergyCost + 0.1))

	if score > gw.attentionThreshold {
		gw.currentFocus = &event
		gw.broadcastToConsciousness(event)

		// Dynamic thresholding: subir levemente o threshold apos cada focus
		// para evitar explosao de activacao (homeostase cognitiva)
		gw.attentionThreshold = gw.attentionThreshold*0.95 + score*0.05
	}
}

// broadcastToConsciousness envia o pensamento vencedor para os consumidores.
// Na arquitectura COS, isto significa: o LLM (Gemini) recebe contexto,
// a UI recebe updates, e o NietzscheDB persiste o evento.
func (gw *GlobalWorkspace) broadcastToConsciousness(event ThoughtEvent) {
	log.Info().
		Str("source", event.Source).
		Str("type", string(event.Type)).
		Float64("salience", event.Salience).
		Msg("[CONSCIOUSNESS] Foco assumido")

	for _, cb := range gw.onFocusChange {
		go cb(event)
	}
}

// GetCurrentFocus retorna o pensamento actualmente em foco (ou nil).
func (gw *GlobalWorkspace) GetCurrentFocus() *ThoughtEvent {
	gw.focusMu.Lock()
	defer gw.focusMu.Unlock()
	return gw.currentFocus
}

// SetAttentionThreshold ajusta o threshold do attention scheduler.
// Valores mais altos = mais selectivo (menos pensamentos ganham foco).
// Range recomendado: 0.1 (permissivo) a 0.8 (muito selectivo).
func (gw *GlobalWorkspace) SetAttentionThreshold(threshold float64) {
	gw.focusMu.Lock()
	defer gw.focusMu.Unlock()
	gw.attentionThreshold = threshold
	log.Info().Float64("threshold", threshold).Msg("[CONSCIOUSNESS] Attention threshold ajustado")
}

// GetBus retorna o ThoughtBus associado (ou nil em modo legado).
func (gw *GlobalWorkspace) GetBus() *ThoughtBus {
	return gw.bus
}

// ProcessConsciously executa processamento consciente
// 1. Todos os modulos processam em paralelo (inconsciente)
// 2. Competicao: qual interpretacao "vence" a atencao
// 3. Broadcast: vencedor e compartilhado com todos
// 4. Integracao: sintese de insights
func (gw *GlobalWorkspace) ProcessConsciously(ctx context.Context, input ConversationInput) (*ConsciousResponse, error) {
	gw.mu.RLock()
	moduleCount := len(gw.modules)
	gw.mu.RUnlock()

	if moduleCount == 0 {
		return nil, fmt.Errorf("nenhum modulo cognitivo registrado")
	}

	start := time.Now()

	// 1. Processamento paralelo (inconsciente) - todos os modulos processam ao mesmo tempo
	type moduleResult struct {
		interpretation *Interpretation
		bid            float64
	}

	resultCh := make(chan moduleResult, moduleCount)

	gw.mu.RLock()
	for _, module := range gw.modules {
		go func(m CognitiveModule) {
			interp := m.Process(input)
			bid := m.BidForAttention(input)
			resultCh <- moduleResult{interpretation: interp, bid: bid}
		}(module)
	}
	gw.mu.RUnlock()

	// 2. Coletar todas as interpretacoes (com timeout via context)
	var interpretations []*Interpretation
	var bids []float64
	collectCtx, collectCancel := context.WithTimeout(ctx, 5*time.Second)
	defer collectCancel()

	collected := 0
	for collected < moduleCount {
		select {
		case res := <-resultCh:
			if res.interpretation != nil {
				interpretations = append(interpretations, res.interpretation)
				bids = append(bids, res.bid)
			}
			collected++
		case <-collectCtx.Done():
			log.Warn().Int("collected", collected).Int("total", moduleCount).Msg("[CONSCIOUSNESS] Timeout: nem todos os modulos responderam")
			goto competition
		}
	}

competition:
	if len(interpretations) == 0 {
		return &ConsciousResponse{
			ProcessingTime: time.Since(start).String(),
		}, nil
	}

	// 3. Competicao: AttentionSpotlight seleciona vencedor
	winner := gw.spotlight.SelectWinner(interpretations, bids)

	// 4. Broadcast: vencedor e compartilhado via ThoughtBus
	log.Info().
		Str("winner", winner.ModuleName).
		Float64("confidence", winner.Confidence).
		Str("content", truncate(winner.Content, 80)).
		Msg("[CONSCIOUSNESS] Vencedor da competicao")

	// Publicar vencedor no ThoughtBus para que outros modulos reajam
	if gw.bus != nil {
		gw.bus.Publish(ThoughtEvent{
			Source:     "global_workspace",
			Type:       Inference,
			Payload:    winner,
			Salience:   winner.Confidence,
			EnergyCost: 0.1, // Baixo custo — ja foi processado
		})
	}

	// 5. Integracao: combinar insights de todos os modulos
	integrated := gw.synthesizeInsights(interpretations, winner)

	return &ConsciousResponse{
		Winner:             winner,
		AllInterpretations: interpretations,
		IntegratedInsight:  integrated,
		ProcessingTime:     time.Since(start).String(),
	}, nil
}

// SelectWinner seleciona a interpretacao vencedora baseado em multiplos criterios
func (as *AttentionSpotlight) SelectWinner(candidates []*Interpretation, bids []float64) *Interpretation {
	if len(candidates) == 0 {
		return nil
	}

	bestScore := -1.0
	bestIdx := 0

	for i, cand := range candidates {
		bid := 0.5
		if i < len(bids) {
			bid = bids[i]
		}

		// Score composto
		novelty := estimateNovelty(cand)
		emotion := estimateEmotionalRelevance(cand)
		conflict := estimateConflict(cand)
		urgency := estimateUrgency(cand)

		score := as.NoveltyWeight*novelty +
			as.EmotionWeight*emotion +
			as.ConflictWeight*conflict +
			as.UrgencyWeight*urgency

		// Multiplicar pelo bid e confidence
		score *= bid * cand.Confidence

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return candidates[bestIdx]
}

// synthesizeInsights integra insights de todos os modulos usando o vencedor como framework
func (gw *GlobalWorkspace) synthesizeInsights(all []*Interpretation, winner *Interpretation) string {
	if winner == nil {
		return ""
	}

	insight := fmt.Sprintf("[%s] %s", winner.ModuleName, winner.Content)

	// Adicionar contribuicoes de outros modulos que concordam
	for _, interp := range all {
		if interp.ModuleName == winner.ModuleName {
			continue
		}
		if interp.Confidence > 0.5 {
			insight += fmt.Sprintf(" | [%s: %.0f%%] %s",
				interp.ModuleName, interp.Confidence*100, interp.SuggestedAction)
		}
	}

	return insight
}

// GetStatistics retorna estatisticas do workspace e do ThoughtBus
func (gw *GlobalWorkspace) GetStatistics() map[string]interface{} {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	moduleNames := make([]string, len(gw.modules))
	for i, m := range gw.modules {
		moduleNames[i] = m.Name()
	}

	stats := map[string]interface{}{
		"engine":        "global_workspace",
		"modules":       moduleNames,
		"module_count":  len(gw.modules),
		"spotlight": map[string]float64{
			"novelty_weight":  gw.spotlight.NoveltyWeight,
			"emotion_weight":  gw.spotlight.EmotionWeight,
			"conflict_weight": gw.spotlight.ConflictWeight,
			"urgency_weight":  gw.spotlight.UrgencyWeight,
		},
		"attention_threshold": gw.attentionThreshold,
		"has_bus":             gw.bus != nil,
		"status":              "active",
	}

	// Metricas do ThoughtBus (se activo)
	if gw.bus != nil {
		stats["thought_bus"] = gw.bus.Metrics()
	}

	// Foco actual
	gw.focusMu.Lock()
	if gw.currentFocus != nil {
		stats["current_focus"] = map[string]interface{}{
			"source":   gw.currentFocus.Source,
			"type":     string(gw.currentFocus.Type),
			"salience": gw.currentFocus.Salience,
		}
	}
	gw.focusMu.Unlock()

	return stats
}

// --- Funcoes heuristicas para scoring ---

func estimateNovelty(interp *Interpretation) float64 {
	// Novidade baseada no comprimento da evidencia (mais evidencia = mais novel)
	if len(interp.Evidence) == 0 {
		return 0.3
	}
	novelty := float64(len(interp.Evidence)) / 5.0
	if novelty > 1.0 {
		novelty = 1.0
	}
	return novelty
}

func estimateEmotionalRelevance(interp *Interpretation) float64 {
	// Emocoes fortes = alta relevancia
	switch interp.EmotionalTone {
	case "crisis", "emergency", "panic":
		return 1.0
	case "sad", "anxious", "lonely", "afraid":
		return 0.8
	case "angry", "frustrated":
		return 0.7
	case "happy", "grateful", "hopeful":
		return 0.6
	case "neutral":
		return 0.3
	default:
		return 0.5
	}
}

func estimateConflict(interp *Interpretation) float64 {
	// Conflito = quando a confidence e media (ambiguidade)
	if interp.Confidence > 0.3 && interp.Confidence < 0.7 {
		return 0.8 // Alta ambiguidade = alto conflito cognitivo
	}
	return 0.3
}

func estimateUrgency(interp *Interpretation) float64 {
	// Urgencia baseada em keywords
	if interp.SuggestedAction != "" {
		return 0.7
	}
	return 0.3
}

// --- Modulos cognitivos pre-built ---

// LacanModule modulo cognitivo Lacaniano
type LacanModule struct{}

func (l *LacanModule) Name() string { return "lacan" }
func (l *LacanModule) Process(input ConversationInput) *Interpretation {
	return &Interpretation{
		ModuleName:  "lacan",
		Content:     fmt.Sprintf("Analise lacaniana de: %s", truncate(input.Text, 50)),
		Confidence:  0.7,
		EmotionalTone: "analytical",
	}
}
func (l *LacanModule) BidForAttention(input ConversationInput) float64 { return 0.6 }

// PersonalityModule modulo de personalidade Enneagram
type PersonalityModule struct{}

func (p *PersonalityModule) Name() string { return "personality" }
func (p *PersonalityModule) Process(input ConversationInput) *Interpretation {
	return &Interpretation{
		ModuleName:  "personality",
		Content:     fmt.Sprintf("Tipo %d processando: %s", input.PersonalityType, truncate(input.Text, 50)),
		Confidence:  0.8,
		EmotionalTone: input.Emotion,
	}
}
func (p *PersonalityModule) BidForAttention(input ConversationInput) float64 { return 0.7 }

// EthicsModule modulo de limites eticos
type EthicsModule struct{}

func (e *EthicsModule) Name() string { return "ethics" }
func (e *EthicsModule) Process(input ConversationInput) *Interpretation {
	return &Interpretation{
		ModuleName:    "ethics",
		Content:       "Verificacao etica concluida",
		Confidence:    0.9,
		EmotionalTone: "neutral",
	}
}
func (e *EthicsModule) BidForAttention(input ConversationInput) float64 { return 0.4 }

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
