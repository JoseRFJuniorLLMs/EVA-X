package knowledge

import (
	"context"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/cortex/gemini"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"fmt"
)

// GraphReasoningService orquestra o raciocínio clínico usando Neo4j e Gemini Thinking
type GraphReasoningService struct {
	neo4jClient  *graph.Neo4jClient
	geminiClient *gemini.Client
	cfg          *config.Config
	context      *ContextService // ✅ NOVO
}

func NewGraphReasoningService(cfg *config.Config, neo4jClient *graph.Neo4jClient, ctxService *ContextService) *GraphReasoningService {
	return &GraphReasoningService{
		neo4jClient: neo4jClient,
		cfg:         cfg,
		context:     ctxService,
	}
}

// AnalyzeGraphContext extrai o contexto do grafo e pede análise do Gemini
func (s *GraphReasoningService) AnalyzeGraphContext(ctx context.Context, idosoID int64, currentTopic string) (string, error) {
	// 1. Extrair Sub-grafo relevante (últimos nós ativados ou relacionados ao tópico)
	graphData, err := s.fetchPatientContext(ctx, idosoID, currentTopic)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar grafo: %w", err)
	}

	if graphData == "" {
		return "", nil // Sem contexto relevante no grafo
	}

	// 2. Construir Prompt de Thinking
	prompt := fmt.Sprintf(`
	Você é o Módulo de Raciocínio Clínico da EVA (Fractal Zeta Priming Network).
	
	CONTEXTO DO GRAFO (Neo4j):
	%s
	
	TÓPICO ATUAL: "%s"
	
	TAREFAS:
	1. Analise as relações causais no grafo (ex: Dor -> Humor).
	2. Use raciocínio psicanalítico e médico.
	3. Decida: Devemos focar no sintoma físico, na emoção ou em ambos?
	
	Responda APENAS com sua linha de raciocínio (Thoughts) e uma sugestão de abordagem técnica.
	`, graphData, currentTopic)

	// 3. Chamar Gemini Thinking (usando endpoint REST padrão por enquanto, simulando thinking via prompt)
	// Nota: O Client atual de streaming não é ideal para isso, usaremos helper REST se disponível ou criaremos um on-the-fly.
	analysis, err := gemini.AnalyzeText(s.cfg, prompt)
	if err != nil {
		return "", fmt.Errorf("erro na análise do Gemini: %w", err)
	}

	// ✅ FASE 3: Persistir Factual Memory
	if s.context != nil {
		go s.context.SaveAnalysis(context.Background(), idosoID, "GRAPH", analysis)
	}

	return analysis, nil
}

// fetchPatientContext busca nós conectados ao paciente e ao tópico recente
func (s *GraphReasoningService) fetchPatientContext(ctx context.Context, idosoID int64, topic string) (string, error) {
	cypher := `
	MATCH (p:Paciente {id: $idosoID})-[r*1..2]-(n)
	WHERE n.timestamp > datetime() - duration({days: 30})
	RETURN n.label AS Label, type(r[0]) AS Rel, n.name AS Name, n.value AS Value
	LIMIT 10
	`

	params := map[string]interface{}{
		"idosoID": idosoID,
	}

	records, err := s.neo4jClient.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return "", err
	}

	var contextStr string
	for _, rec := range records {
		label, _ := rec.Get("Label")
		rel, _ := rec.Get("Rel")
		name, _ := rec.Get("Name")
		val, _ := rec.Get("Value")

		contextStr += fmt.Sprintf("(%v) -[%v]-> (%v: %v)\n", "Paciente", rel, label, name)
		if val != nil {
			contextStr += fmt.Sprintf("   Detalhe: %v\n", val)
		}
	}

	return contextStr, nil
}
