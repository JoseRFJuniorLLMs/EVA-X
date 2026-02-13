package transnar

import (
	"context"
	"eva-mind/internal/cortex/lacan"
	"eva-mind/internal/hippocampus/memory"
	"eva-mind/internal/cortex/personality"
	"fmt"
	"log"
)

// Engine é o motor principal do TransNAR
type Engine struct {
	analyzer   *Analyzer
	detector   *DesireDetector
	lacanSvc   *lacan.SignifierService
	zetaRouter *personality.PersonalityRouter
	fdpnEngine *memory.FDPNEngine
}

// NewEngine cria uma nova instância do TransNAR Engine
func NewEngine(
	lacanSvc *lacan.SignifierService,
	zetaRouter *personality.PersonalityRouter,
	fdpnEngine *memory.FDPNEngine,
) *Engine {
	return &Engine{
		analyzer:   NewAnalyzer(),
		detector:   NewDesireDetector(),
		lacanSvc:   lacanSvc,
		zetaRouter: zetaRouter,
		fdpnEngine: fdpnEngine,
	}
}

// InferDesire é o método principal que orquestra toda a inferência
func (e *Engine) InferDesire(
	ctx context.Context,
	userID int64,
	text string,
	currentPersonality personality.EneagramType,
) *DesireInference {

	// 1. Analisar cadeia significante
	chain := e.analyzer.Analyze(text)

	log.Printf("[TransNAR] Cadeia: Words=%v, Emotions=%v, Negations=%v, Modals=%v, Intensity=%.2f",
		chain.Words, chain.Emotions, chain.Negations, chain.Modals, chain.Intensity)

	// 2. Buscar histórico de significantes (Lacan)
	signifiers, err := e.lacanSvc.GetKeySignifiers(ctx, userID, 10)
	if err != nil {
		log.Printf("[TransNAR] Erro ao buscar significantes: %v", err)
		signifiers = []lacan.Signifier{} // Continuar sem histórico
	}

	// 3. Detectar desejo latente
	desire := e.detector.Detect(ctx, chain, signifiers, currentPersonality)

	log.Printf("[TransNAR] 🧠 Desejo inferido: %s (confiança: %.2f) - %s",
		desire.Desire, desire.Confidence, desire.Reasoning)

	return desire
}

// InferDesireWithContext versão estendida que retorna contexto completo
func (e *Engine) InferDesireWithContext(
	ctx context.Context,
	userID int64,
	text string,
	currentPersonality personality.EneagramType,
) (*DesireInference, *SignifierChain, []lacan.Signifier) {

	chain := e.analyzer.Analyze(text)
	signifiers, _ := e.lacanSvc.GetKeySignifiers(ctx, userID, 10)
	desire := e.detector.Detect(ctx, chain, signifiers, currentPersonality)

	return desire, chain, signifiers
}

// ShouldInterpellate decide se EVA deve interpelar o significante
// (fazer uma pergunta sobre o desejo latente)
func (e *Engine) ShouldInterpellate(desire *DesireInference) bool {
	// Interpelar se:
	// 1. Confiança >= 0.65 (reduzido de 0.7 para produção)
	// 2. Desejo não é "unknown"
	// 3. CRÍTICO: Se desejo é RELIEF (pulsão de morte), sempre interpelar
	if desire.Desire == DesireRelief {
		return desire.Confidence > 0.5 // Threshold mais baixo para casos críticos
	}
	return desire.Confidence >= 0.65 && desire.Desire != DesireUnknown
}

// GetInterpellationPrompt gera um prompt para o LLM baseado no desejo inferido
func (e *Engine) GetInterpellationPrompt(
	desire *DesireInference,
	chain *SignifierChain,
) string {

	basePrompt := fmt.Sprintf(`
[ANÁLISE TRANSNAR - DESEJO LATENTE DETECTADO]

Desejo Inferido: %s
Confiança: %.0f%%
Raciocínio: %s

INSTRUÇÕES PARA RESPOSTA:
1. NÃO responda diretamente à demanda explícita
2. Endereça o DESEJO LATENTE identificado acima
3. Use uma das estratégias lacanianas:
   - INTERPELAÇÃO: Pergunte sobre o significante repetido
   - REFLEXÃO: Espelhe a emoção subjacente
   - PONTUAÇÃO: Dê sentido ao que foi dito
4. Seja empática mas não bajuladora (anti-sycophancy)

Exemplo de resposta adequada:
"Percebo que isso te preocupa. O que especificamente te deixa inseguro sobre isso?"
`,
		GetDesireDescription(desire.Desire),
		desire.Confidence*100,
		desire.Reasoning,
	)

	// Adicionar contexto específico por tipo de desejo
	switch desire.Desire {
	case DesireSecurity:
		basePrompt += "\nFOCO: Transmitir segurança e explorar a fonte do medo."

	case DesireConnection:
		basePrompt += "\nFOCO: Validar o sentimento de solidão e oferecer presença."

	case DesireAutonomy:
		basePrompt += "\nFOCO: Respeitar a autonomia mas manter limites de segurança."

	case DesireRecognition:
		basePrompt += "\nFOCO: Reconhecer o valor da pessoa sem reforçar dependência."

	case DesireRelief:
		basePrompt += "\nFOCO: Oferecer alívio emocional e validar o sofrimento."
	}

	return basePrompt
}
