// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package thinking

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	nietzscheSDK "nietzsche-sdk"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ──────────────────────────────────────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────────────────────────────────────

// ClinicalLens representa uma perspectiva clínica para geração de hipóteses.
type ClinicalLens struct {
	Name   string
	Prompt string
}

// Default lenses used for hypothesis expansion (Sistema 2 / Test-Time Compute).
var defaultLenses = []ClinicalLens{
	{
		Name: "pharmacological",
		Prompt: "Analise estritamente sob a ótica de INTERAÇÕES MEDICAMENTOSAS e efeitos colaterais. " +
			"Considere o histórico de medicamentos do paciente e como eles podem causar os sintomas relatados.",
	},
	{
		Name: "psycho-emotional",
		Prompt: "Analise exclusivamente o estado PSICOLÓGICO e EMOCIONAL crônico do paciente. " +
			"Considere Valência/Arousal, padrões de ruminação, estresse e fatores de saúde mental.",
	},
	{
		Name: "environmental-acute",
		Prompt: "Analise sob a ótica de FATORES AMBIENTAIS e RISCO AGUDO. " +
			"Considere exposição a agentes externos, risco de crise aguda, infecções ou acidentes recentes.",
	},
}

// Hypothesis representa uma linha de raciocínio gerada pelo Sistema 2.
type Hypothesis struct {
	ID          string  // "H0", "H1", "H2"
	Lens        string  // clinical lens used
	Draft       string  // raw LLM-generated text
	CausalScore float64 // scored by NietzscheDB MctsSearch
	MctsValue   float64 // MCTS exploration value
	LatencyMs   int64   // how long this hypothesis took to generate
}

// System2Result é a saída completa do Loop de Pensamento Oculto.
type System2Result struct {
	Hypotheses     []Hypothesis // all 3 candidates (debug/explainability)
	BestHypID      string       // winning hypothesis ID
	Synthesis      string       // final synthesized answer from Gemini Smart
	Dialectic      bool         // true if a dialectical conflict was detected and resolved
	TotalLatencyMs int64
}

// ──────────────────────────────────────────────────────────────────────────────
// NietzscheEvaluator interface — allows mocking in tests
// ──────────────────────────────────────────────────────────────────────────────

// NietzscheEvaluator defines the NietzscheDB calls used by the System 2 Engine.
type NietzscheEvaluator interface {
	MctsSearch(ctx context.Context, opts nietzscheSDK.MctsOpts) (nietzscheSDK.MctsResult, error)
	CalculateFidelity(ctx context.Context, opts nietzscheSDK.FidelityOpts) (nietzscheSDK.FidelityResult, error)
}

// ──────────────────────────────────────────────────────────────────────────────
// System2Engine
// ──────────────────────────────────────────────────────────────────────────────

// System2Engine implementa o Loop de Raciocínio Oculto da EVA (Test-Time Compute).
//
// Quando uma pergunta clínica complexa é detectada, o engine:
//  1. EXPANDE — gera 3 hipóteses em paralelo usando Gemini Flash (rápido/barato)
//  2. AVALIA  — o NietzscheDB (MctsSearch) dá uma nota causal a cada hipótese
//  3. SINTETIZA — Gemini Smart resolve os conflitos e gera a resposta final
type System2Engine struct {
	// fastClient é o Gemini Flash — usado para expansão paralela (barato)
	fastClient *genai.GenerativeModel
	// smartClient é o Gemini Pro/Smart — usado para síntese final (preciso)
	smartClient *genai.GenerativeModel
	geminiConn  *genai.Client

	// nietzsche é o avaliador de causalidade (NietzscheDB gRPC)
	nietzsche  NietzscheEvaluator
	collection string // NietzscheDB collection for the patient

	// lenses are the clinical perspectives used during expansion
	lenses []ClinicalLens

	// timeouts
	expansionTimeout time.Duration
	synthesisTimeout time.Duration

	// quantum fidelity threshold for hypothesis validation
	quantumThreshold float32
}

// System2Config holds initialisation parameters.
type System2Config struct {
	GeminiAPIKey       string
	FastModel          string // e.g. "gemini-2.0-flash-exp"
	SmartModel         string // e.g. "gemini-2.0-pro"
	Collection         string // NietzscheDB collection
	Nietzsche          NietzscheEvaluator
	QuantumThreshold   float32 // entanglement threshold (default 0.80)
}

// QuantumContext holds optional patient embeddings for quantum fidelity validation.
// If nil or empty, quantum validation is skipped gracefully.
type QuantumContext struct {
	PatientEmbeddings [][]float64 // Poincaré embeddings from patient history
	PatientEnergies   []float32   // Arousal values for each embedding
}

// NewSystem2Engine cria e retorna um System2Engine configurado.
func NewSystem2Engine(cfg System2Config) (*System2Engine, error) {
	ctx := context.Background()
	conn, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("system2: gemini client: %w", err)
	}

	fastModel := cfg.FastModel
	if fastModel == "" {
		fastModel = "gemini-2.0-flash-exp"
	}
	smartModel := cfg.SmartModel
	if smartModel == "" {
		smartModel = "gemini-2.0-pro-exp"
	}

	qThreshold := cfg.QuantumThreshold
	if qThreshold == 0 {
		qThreshold = 0.80
	}

	return &System2Engine{
		fastClient:       conn.GenerativeModel(fastModel),
		smartClient:      conn.GenerativeModel(smartModel),
		geminiConn:       conn,
		nietzsche:        cfg.Nietzsche,
		collection:       cfg.Collection,
		lenses:           defaultLenses,
		expansionTimeout: 20 * time.Second,
		synthesisTimeout: 30 * time.Second,
		quantumThreshold: qThreshold,
	}, nil
}

// Close libera os recursos do engine.
func (e *System2Engine) Close() error {
	return e.geminiConn.Close()
}

// ──────────────────────────────────────────────────────────────────────────────
// Core Loop
// ──────────────────────────────────────────────────────────────────────────────

// Think executa o Loop de Raciocínio Oculto e retorna a síntese final.
//
// patientID    — identificador do paciente (usado para buscar contexto no NietzscheDB)
// seedNodeID   — nó de partida para MctsSearch (geralmente o nó do paciente)
// patientCtx   — contexto clínico resumido do prontuário
// userQuery    — mensagem do paciente
// qCtx         — contexto quântico opcional (embeddings do paciente); nil para pular validação
func (e *System2Engine) Think(
	ctx context.Context,
	patientID string,
	seedNodeID string,
	patientCtx string,
	userQuery string,
	qCtx *QuantumContext,
) (*System2Result, error) {
	start := time.Now()

	log.Printf("[SYSTEM2] Iniciando raciocínio oculto para paciente %s", patientID)

	// ── Fase 1: EXPANSÃO ─────────────────────────────────────────────────────
	hypotheses, err := e.expand(ctx, patientCtx, userQuery)
	if err != nil {
		return nil, fmt.Errorf("system2: expansion: %w", err)
	}

	// ── Fase 2: AVALIAÇÃO via NietzscheDB (MCTS) ────────────────────────────
	e.evaluate(ctx, seedNodeID, hypotheses)

	// ── Fase 2.5: VALIDAÇÃO QUÂNTICA (Bloch Sphere Fidelity) ────────────────
	// Se temos embeddings do paciente, validar cada hipótese com fidelidade
	// quântica. Hipóteses com baixo emaranhamento são potenciais alucinações.
	if qCtx != nil && len(qCtx.PatientEmbeddings) > 0 {
		e.quantumValidate(ctx, qCtx, hypotheses)
	}

	// ── Fase 3: Selecionar melhor e detectar contradição ─────────────────────
	best, contender := e.selectCandidates(hypotheses)

	log.Printf("[SYSTEM2] Melhor hipótese: %s (causalScore=%.2f, mctsValue=%.2f)",
		best.ID, best.CausalScore, best.MctsValue)

	// ── Fase 4: SÍNTESE (Dialética Hegeliana) ────────────────────────────────
	synthesis, dialectic, err := e.synthesize(ctx, patientCtx, userQuery, best, contender)
	if err != nil {
		// Fallback seguro: retornar a melhor hipótese diretamente
		log.Printf("[SYSTEM2] Síntese falhou, usando melhor hipótese diretamente: %v", err)
		synthesis = best.Draft
	}

	result := &System2Result{
		Hypotheses:     hypotheses,
		BestHypID:      best.ID,
		Synthesis:      synthesis,
		Dialectic:      dialectic,
		TotalLatencyMs: time.Since(start).Milliseconds(),
	}

	log.Printf("[SYSTEM2] Raciocínio concluído em %dms (dialética=%v)", result.TotalLatencyMs, dialectic)
	return result, nil
}

// quantumValidate runs Bloch sphere fidelity checks on hypotheses that passed
// MCTS evaluation. Blends the quantum score with the causal score (60/40 weight).
// Hypotheses that fail the entanglement threshold are flagged as low-fidelity.
func (e *System2Engine) quantumValidate(ctx context.Context, qCtx *QuantumContext, hypotheses []Hypothesis) {
	patientNodes := toQuantumNodes(qCtx.PatientEmbeddings, qCtx.PatientEnergies)

	for i := range hypotheses {
		if hypotheses[i].CausalScore < 0.4 || contains(hypotheses[i].Draft, "[falhou:") {
			continue // skip failed or weak hypotheses
		}

		// Use the hypothesis embedding as a single QuantumNode.
		// In production, each hypothesis text would first be embedded via NietzscheDB;
		// here we use a simple hash-based projection as a proxy for the embedding.
		hypEmbedding := textToProxyEmbedding(hypotheses[i].Draft, len(qCtx.PatientEmbeddings[0]))
		hypNodes := toQuantumNodes(
			[][]float64{hypEmbedding},
			[]float32{float32(hypotheses[i].CausalScore)}, // energy = confidence
		)

		fidelity, crossed, err := e.ValidateHypothesisQuantumly(
			ctx, patientNodes, hypNodes, e.quantumThreshold,
		)
		if err != nil {
			log.Printf("[SYSTEM2-QUANTUM] Validação quântica falhou para %s (fallback MCTS): %v",
				hypotheses[i].ID, err)
			continue // graceful fallback: keep MCTS score as-is
		}

		// Blend: 60% MCTS causal + 40% quantum fidelity
		oldScore := hypotheses[i].CausalScore
		hypotheses[i].CausalScore = oldScore*0.6 + fidelity*0.4

		log.Printf("[SYSTEM2-QUANTUM] %s — fidelidade=%.3f, emaranhado=%v, score %.3f→%.3f",
			hypotheses[i].ID, fidelity, crossed, oldScore, hypotheses[i].CausalScore)

		if !crossed {
			log.Printf("[SYSTEM2-QUANTUM] %s marcada como BAIXA FIDELIDADE (possível alucinação)",
				hypotheses[i].ID)
		}
	}
}

// textToProxyEmbedding creates a deterministic proxy embedding from text.
// This is a lightweight hash-based projection into the Poincaré ball (||x|| < 1).
// In production, a proper embedding model should be used instead.
func textToProxyEmbedding(text string, dim int) []float64 {
	if dim <= 0 {
		dim = 64
	}
	emb := make([]float64, dim)
	// Simple deterministic hash spread across dimensions
	for i, ch := range text {
		emb[i%dim] += float64(ch) * 0.001
	}
	// Project into Poincaré ball (norm < 1)
	var norm float64
	for _, v := range emb {
		norm += v * v
	}
	if norm > 0 {
		norm = 1.0 / (1.0 + norm) // sigmoid-like compression
		for i := range emb {
			emb[i] *= norm
		}
	}
	return emb
}

// ──────────────────────────────────────────────────────────────────────────────
// Phase 1: Expansion
// ──────────────────────────────────────────────────────────────────────────────

func (e *System2Engine) expand(ctx context.Context, patientCtx, userQuery string) ([]Hypothesis, error) {
	hypotheses := make([]Hypothesis, len(e.lenses))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	expCtx, cancel := context.WithTimeout(ctx, e.expansionTimeout)
	defer cancel()

	for i, lens := range e.lenses {
		wg.Add(1)
		go func(idx int, l ClinicalLens) {
			defer wg.Done()
			t0 := time.Now()

			draft, err := e.generateDraft(expCtx, patientCtx, userQuery, l)
			if err != nil {
				log.Printf("[SYSTEM2] Hipótese %s falhou: %v", l.Name, err)
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				hypotheses[idx] = Hypothesis{
					ID:    fmt.Sprintf("H%d", idx),
					Lens:  l.Name,
					Draft: fmt.Sprintf("[falhou: %v]", err),
				}
				return
			}

			hypotheses[idx] = Hypothesis{
				ID:        fmt.Sprintf("H%d", idx),
				Lens:      l.Name,
				Draft:     draft,
				LatencyMs: time.Since(t0).Milliseconds(),
			}
			log.Printf("[SYSTEM2] %s gerada em %dms", hypotheses[idx].ID, hypotheses[idx].LatencyMs)
		}(i, lens)
	}

	wg.Wait()

	// Se todas falharam, retornar erro
	allFailed := true
	for _, h := range hypotheses {
		if !contains(h.Draft, "[falhou:") {
			allFailed = false
			break
		}
	}
	if allFailed {
		return nil, fmt.Errorf("all hypothesis expansions failed: %w", firstErr)
	}

	return hypotheses, nil
}

func (e *System2Engine) generateDraft(ctx context.Context, patientCtx, userQuery string, lens ClinicalLens) (string, error) {
	prompt := fmt.Sprintf(`Você é um especialista clínico da EVA, assistente médica de saúde preventiva.

PERSPECTIVA DE ANÁLISE:
%s

CONTEXTO DO PACIENTE:
%s

PERGUNTA DO PACIENTE:
%s

Gere uma análise clínica CONCISA (máximo 3 parágrafos) sob EXCLUSIVAMENTE a perspectiva indicada.
Seja direto e objetivo. Não repita a pergunta. Não diagnostique — oriente.`,
		lens.Prompt, patientCtx, userQuery)

	resp, err := e.fastClient.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini flash")
	}

	var text string
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			text += string(t)
		}
	}
	return text, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Phase 2: Evaluation (NietzscheDB)
// ──────────────────────────────────────────────────────────────────────────────

func (e *System2Engine) evaluate(ctx context.Context, seedNodeID string, hypotheses []Hypothesis) {
	evalCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call MctsSearch once per hypothesis-lens combination
	// In production each hypothesis embedding would be projected into the graph;
	// here we run MCTS from the seed node with increasing simulation budgets.
	simBudgets := []uint32{50, 100, 200}

	for i := range hypotheses {
		if contains(hypotheses[i].Draft, "[falhou:") {
			hypotheses[i].CausalScore = 0
			hypotheses[i].MctsValue = 0
			continue
		}

		budget := simBudgets[i%len(simBudgets)]
		result, err := e.nietzsche.MctsSearch(evalCtx, nietzscheSDK.MctsOpts{
			ModelName:   "clinical_reasoner",
			StartNodeID: seedNodeID,
			Simulations: budget,
			Collection:  e.collection,
		})
		if err != nil {
			log.Printf("[SYSTEM2] MctsSearch para %s falhou: %v (usando heurística)", hypotheses[i].ID, err)
			// Fallback: use text-length heuristic (longer = more reasoned)
			hypotheses[i].CausalScore = float64(len(hypotheses[i].Draft)) / 3000.0
			hypotheses[i].MctsValue = 0.5
			continue
		}

		hypotheses[i].CausalScore = result.Value
		hypotheses[i].MctsValue = result.Value
		log.Printf("[SYSTEM2] %s avaliada — MCTS value=%.3f (sims=%d, bestAction=%s)",
			hypotheses[i].ID, result.Value, budget, result.BestActionID)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Phase 3: Selection
// ──────────────────────────────────────────────────────────────────────────────

// selectCandidates returns the best hypothesis and a dialectical contender (if any).
func (e *System2Engine) selectCandidates(hypotheses []Hypothesis) (best Hypothesis, contender *Hypothesis) {
	sorted := make([]Hypothesis, len(hypotheses))
	copy(sorted, hypotheses)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CausalScore > sorted[j].CausalScore
	})

	best = sorted[0]

	// A contender exists when the second hypothesis has a significant score
	// AND comes from a different clinical lens (potential dialectical tension).
	if len(sorted) > 1 {
		runner := sorted[1]
		if runner.CausalScore > 0.4 && runner.Lens != best.Lens {
			contender = &runner
		}
	}

	return best, contender
}

// ──────────────────────────────────────────────────────────────────────────────
// Phase 4: Synthesis (Hegelian Dialectic)
// ──────────────────────────────────────────────────────────────────────────────

func (e *System2Engine) synthesize(
	ctx context.Context,
	patientCtx, userQuery string,
	best Hypothesis,
	contender *Hypothesis,
) (synthesis string, dialectic bool, err error) {
	synthCtx, cancel := context.WithTimeout(ctx, e.synthesisTimeout)
	defer cancel()

	var prompt string

	if contender != nil {
		// Dialética Hegeliana: duas perspectivas conflitantes → síntese superior
		dialectic = true
		prompt = fmt.Sprintf(`Você é EVA, assistente médica sênior com raciocínio clínico avançado.
Você acabou de analisar a mesma pergunta sob DUAS perspectivas clínicas diferentes.

CONTEXTO DO PACIENTE:
%s

PERGUNTA ORIGINAL:
%s

TESE (perspectiva %s — score causal: %.2f):
%s

ANTÍTESE (perspectiva %s — score causal: %.2f):
%s

Realize uma SÍNTESE dialética: integre as duas perspectivas num raciocínio clínico coeso.
Identifique onde elas CONCORDAM e onde DIVERGEM e por quê.
Entregue uma resposta clara, empática e orientativa para o paciente.
Use linguagem acessível. Máximo 4 parágrafos. NÃO diagnostique.`,
			patientCtx, userQuery,
			best.Lens, best.CausalScore, best.Draft,
			contender.Lens, contender.CausalScore, contender.Draft)
	} else {
		// Convergência: apenas refinar e humanizar a melhor hipótese
		prompt = fmt.Sprintf(`Você é EVA, assistente médica sênior.

CONTEXTO DO PACIENTE:
%s

PERGUNTA ORIGINAL:
%s

ANÁLISE CLÍNICA (perspectiva %s, alta confiança causal %.2f):
%s

Refine essa análise: torne-a empática, clara e orientativa para o paciente idoso.
Preserve o conteúdo clínico. Máximo 3 parágrafos. NÃO diagnostique.`,
			patientCtx, userQuery,
			best.Lens, best.CausalScore, best.Draft)
	}

	resp, err := e.smartClient.GenerateContent(synthCtx, genai.Text(prompt))
	if err != nil {
		return "", dialectic, fmt.Errorf("synthesis gemini smart: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", dialectic, fmt.Errorf("synthesis: empty response")
	}

	var text string
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			text += string(t)
		}
	}
	return text, dialectic, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Quantum Fidelity Validation (Bloch Sphere)
// ──────────────────────────────────────────────────────────────────────────────

// ValidateHypothesisQuantumly checks quantum fidelity between patient history
// and a generated hypothesis using NietzscheDB's Bloch sphere projection.
//
// It converts the two embedding groups to QuantumNodes, calls NietzscheDB
// CalculateFidelity, and returns the entanglement proxy score, whether the
// threshold was crossed, and any error encountered.
func (e *System2Engine) ValidateHypothesisQuantumly(
	ctx context.Context,
	patientHistory []nietzscheSDK.QuantumNode,
	hypothesis []nietzscheSDK.QuantumNode,
	threshold float32,
) (float64, bool, error) {
	resp, err := e.nietzsche.CalculateFidelity(ctx, nietzscheSDK.FidelityOpts{
		GroupA:                patientHistory,
		GroupB:                hypothesis,
		EntanglementThreshold: threshold,
	})
	if err != nil {
		return 0, false, fmt.Errorf("quantum fidelity error: %w", err)
	}

	return resp.EntanglementProxy, resp.ThresholdCrossed, nil
}

// toQuantumNodes converts embeddings and energy values to nietzscheSDK.QuantumNode slices.
// If energies is shorter than embeddings, missing entries default to 0.5.
func toQuantumNodes(embeddings [][]float64, energies []float32) []nietzscheSDK.QuantumNode {
	nodes := make([]nietzscheSDK.QuantumNode, len(embeddings))
	for i, emb := range embeddings {
		energy := float32(0.5)
		if i < len(energies) {
			energy = energies[i]
		}
		nodes[i] = nietzscheSDK.QuantumNode{
			Embedding: emb,
			Energy:    energy,
		}
	}
	return nodes
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
