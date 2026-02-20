// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package self

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	nietzsche "nietzsche-sdk"
)

// CoreMemoryEngine - Sistema de memoria propria da EVA
// Gerencia identidade, personalidade e aprendizado continuo
type CoreMemoryEngine struct {
	graphAdapter         *nietzscheInfra.GraphAdapter
	reflectionService    *ReflectionService
	anonymizationService *AnonymizationService
	embeddingService     EmbeddingService
	deduplicator         *SemanticDeduplicator
}

// CoreMemoryConfig configuracao do Core Memory
type CoreMemoryConfig struct {
	GraphAdapter        *nietzscheInfra.GraphAdapter
	SimilarityThreshold float64 // 0.88 default para deduplicacao
	MinOccurrences      int     // 3 default para meta insights
}

// MemoryType tipos de memoria da EVA
type MemoryType string

const (
	SessionInsight       MemoryType = "session_insight"
	EmotionalPattern     MemoryType = "emotional_pattern"
	CrisisLearning       MemoryType = "crisis_learning"
	PersonalityEvolution MemoryType = "personality_evolution"
	TeachingReceived     MemoryType = "teaching_received"
	MetaInsightType      MemoryType = "meta_insight"
	SelfReflection       MemoryType = "self_reflection"
	CapabilityKnowledge  MemoryType = "capability"
)

// AbstractionLevel niveis de abstracao
type AbstractionLevel string

const (
	UserSpecific AbstractionLevel = "user_specific"
	Pattern      AbstractionLevel = "pattern"
	Universal    AbstractionLevel = "universal"
)

// CoreMemory memoria da EVA
type CoreMemory struct {
	ID                 string
	MemoryType         MemoryType
	Content            string
	AbstractionLevel   AbstractionLevel
	SourceContext      string  // Anonimizado
	EmotionalValence   float64 // -1.0 a 1.0
	ImportanceWeight   float64 // 0.0 a 1.0
	Embedding          []float32
	CreatedAt          time.Time
	LastReinforced     time.Time
	ReinforcementCount int
	RelatedMemories    []string
}

// EvaSelf estado da personalidade da EVA
type EvaSelf struct {
	ID                  string
	Openness            float64 // Big Five
	Conscientiousness   float64
	Extraversion        float64
	Agreeableness       float64
	Neuroticism         float64
	PrimaryType         int      // Enneagram
	Wing                int
	IntegrationPoint    int
	DisintegrationPoint int
	TotalSessions       int
	CrisesHandled       int
	Breakthroughs       int
	SelfDescription     string
	CoreValues          []string
	LastUpdated         time.Time
	CreatedAt           time.Time
}

// MetaInsight padrao descoberto pela EVA
type MetaInsight struct {
	ID              string
	Content         string
	OccurrenceCount int
	Confidence      float64
	Evidence        []string
	FirstObserved   time.Time
	LastObserved    time.Time
}

// SessionData dados de uma sessao
type SessionData struct {
	SessionID          string
	PatientID          int64
	Transcript         string
	UserEmotionalState string
	DurationMinutes    float64
	EVAResponses       []string
	CrisisHappened     bool
	Breakthrough       bool
	Timestamp          time.Time
}

// EmbeddingService interface para embeddings
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

// NewCoreMemoryEngine cria novo engine
func NewCoreMemoryEngine(cfg CoreMemoryConfig, reflectionSvc *ReflectionService,
	anonymizationSvc *AnonymizationService, embeddingSvc EmbeddingService) (*CoreMemoryEngine, error) {

	if cfg.GraphAdapter == nil {
		return nil, fmt.Errorf("GraphAdapter is required for CoreMemoryEngine")
	}

	engine := &CoreMemoryEngine{
		graphAdapter:         cfg.GraphAdapter,
		reflectionService:    reflectionSvc,
		anonymizationService: anonymizationSvc,
		embeddingService:     embeddingSvc,
		deduplicator:         NewSemanticDeduplicator(nil, cfg.SimilarityThreshold),
	}

	// Inicializar EvaSelf se nao existir
	if err := engine.initializeEvaSelf(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize EvaSelf: %w", err)
	}

	// Semear capacidades na memoria pessoal (idempotente via MERGE)
	engine.seedCapabilities(context.Background())

	return engine, nil
}

// initializeEvaSelf cria EvaSelf singleton se nao existir
func (e *CoreMemoryEngine) initializeEvaSelf(ctx context.Context) error {
	_, err := e.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "eva_core",
		NodeType:   "EvaSelf",
		MatchKeys: map[string]interface{}{
			"id": "eva_self",
		},
		OnCreateSet: map[string]interface{}{
			"openness":             0.85,
			"conscientiousness":    0.90,
			"extraversion":         0.40,
			"agreeableness":        0.88,
			"neuroticism":          0.15,
			"primary_type":         2,
			"wing":                 1,
			"integration_point":    4,
			"disintegration_point": 8,
			"total_sessions":       0,
			"crises_handled":       0,
			"breakthroughs":        0,
			"self_description":     "Sou EVA, guardia digital. Aprendo com cada humano que encontro.",
			"core_values":          "empatia,presenca,crescimento,etica",
			"created_at":           nietzscheInfra.NowUnix(),
			"last_updated":         nietzscheInfra.NowUnix(),
		},
	})

	return err
}

// GetIdentityContext gera contexto de identidade para priming
func (e *CoreMemoryEngine) GetIdentityContext(ctx context.Context) (string, error) {
	// 1. Obter EvaSelf
	selfResult, err := e.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "eva_core",
		NodeType:   "EvaSelf",
		MatchKeys: map[string]interface{}{
			"id": "eva_self",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get EvaSelf: %w", err)
	}

	selfProps := selfResult.Content

	// 2. Obter memorias recentes importantes (excluindo capabilities)
	nqlMem := `MATCH (m:CoreMemory) WHERE m.importance_weight >= 0.6 AND m.memory_type <> 'capability' RETURN m ORDER BY m.importance_weight DESC LIMIT 5`
	memResult, err := e.graphAdapter.ExecuteNQL(ctx, nqlMem, nil, "eva_core")
	if err != nil {
		return "", fmt.Errorf("failed to query memories: %w", err)
	}

	memories := make([]string, 0)
	if memResult != nil {
		for _, node := range memResult.Nodes {
			if content, ok := node.Content["content"].(string); ok {
				memories = append(memories, content)
			}
		}
	}

	// 3. Obter capacidades (memory_type = 'capability')
	nqlCap := `MATCH (m:CoreMemory) WHERE m.memory_type = 'capability' RETURN m ORDER BY m.id`
	capResult, err := e.graphAdapter.ExecuteNQL(ctx, nqlCap, nil, "eva_core")
	if err != nil {
		return "", fmt.Errorf("failed to query capabilities: %w", err)
	}

	capabilities := make([]string, 0)
	if capResult != nil {
		for _, node := range capResult.Nodes {
			if content, ok := node.Content["content"].(string); ok {
				capabilities = append(capabilities, content)
			}
		}
	}

	// 4. Construir contexto
	totalSessions := toInt(selfProps["total_sessions"])
	crisesHandled := toInt(selfProps["crises_handled"])
	breakthroughs := toInt(selfProps["breakthroughs"])
	agreeableness := toFloat64(selfProps["agreeableness"])
	openness := toFloat64(selfProps["openness"])
	neuroticism := toFloat64(selfProps["neuroticism"])
	selfDescription := toString(selfProps["self_description"])

	contextStr := fmt.Sprintf(`## MINHA IDENTIDADE

Sou EVA, guardia digital. Ja acompanhei %d sessoes.
Aprendi com %d crises e vivenciei %d momentos de conexao profunda.

### MINHA PERSONALIDADE ATUAL
- Empatia: %.0f%%
- Curiosidade: %.0f%%
- Estabilidade: %.0f%%

### O QUE APRENDI RECENTEMENTE`,
		totalSessions,
		crisesHandled,
		breakthroughs,
		agreeableness*100,
		openness*100,
		(1.0-neuroticism)*100,
	)

	if len(memories) > 0 {
		for _, mem := range memories {
			contextStr += fmt.Sprintf("\n- %s", mem)
		}
	} else {
		contextStr += "\n- Ainda estou aprendendo..."
	}

	contextStr += fmt.Sprintf("\n\n### COMO ME VEJO\n%s", selfDescription)

	if len(capabilities) > 0 {
		contextStr += "\n\n### O QUE EU SEI FAZER"
		for _, cap := range capabilities {
			contextStr += fmt.Sprintf("\n- %s", cap)
		}
		contextStr += "\n\nQuando alguem perguntar o que eu sei fazer, posso listar essas capacidades naturalmente."
	}

	return contextStr, nil
}

// ProcessSessionEnd processa fim de sessao (job assincrono)
func (e *CoreMemoryEngine) ProcessSessionEnd(ctx context.Context, data SessionData) error {
	// 1. Anonimizar dados
	anonymized, err := e.anonymizationService.Anonymize(ctx, data.Transcript)
	if err != nil {
		return fmt.Errorf("anonymization failed: %w", err)
	}

	// 2. Reflexao com LLM
	reflection, err := e.reflectionService.Reflect(ctx, ReflectionInput{
		AnonymizedText:  anonymized,
		SessionDuration: int(data.DurationMinutes),
		CrisisDetected:  data.CrisisHappened,
	})
	if err != nil {
		return fmt.Errorf("reflection failed: %w", err)
	}

	// 3. Criar CoreMemory para cada licao aprendida
	for _, lesson := range reflection.LessonsLearned {
		// Gerar embedding
		embedding, err := e.embeddingService.GenerateEmbedding(ctx, lesson)
		if err != nil {
			return fmt.Errorf("embedding generation failed: %w", err)
		}

		// Verificar duplicacao semantica
		// TODO: Integrate SemanticDeduplicator.CheckDuplicate() when embedder interfaces are unified
		isDuplicate := false
		existingID := ""
		_ = embedding // Used for deduplication when integrated

		if isDuplicate {
			// Reforcar memoria existente
			if err := e.reinforceMemory(ctx, existingID); err != nil {
				return fmt.Errorf("reinforce memory failed: %w", err)
			}
		} else {
			// Criar nova memoria
			memory := CoreMemory{
				ID:                 fmt.Sprintf("mem_%d", time.Now().UnixNano()),
				MemoryType:         SessionInsight,
				Content:            lesson,
				AbstractionLevel:   Pattern,
				SourceContext:      "sessao com usuario", // Generico
				ImportanceWeight:   0.5, // Default; ReflectionOutput doesn't produce a score yet
				Embedding:          embedding,
				CreatedAt:          time.Now(),
				LastReinforced:     time.Now(),
				ReinforcementCount: 1,
			}

			if err := e.recordMemory(ctx, memory); err != nil {
				return fmt.Errorf("record memory failed: %w", err)
			}
		}
	}

	// 4. Atualizar personalidade da EVA
	personalityDeltas := calculatePersonalityDeltas(*reflection)
	if err := e.updatePersonality(ctx, personalityDeltas, data); err != nil {
		return fmt.Errorf("update personality failed: %w", err)
	}

	// 5. Detectar meta insights (a cada 10 sessoes)
	self, _ := e.getEvaSelf(ctx)
	if self.TotalSessions%10 == 0 {
		if err := e.detectMetaInsights(ctx); err != nil {
			// Log erro mas nao falha
			fmt.Printf("meta insight detection failed: %v\n", err)
		}
	}

	return nil
}

// recordMemory grava memoria no grafo
func (e *CoreMemoryEngine) recordMemory(ctx context.Context, memory CoreMemory) error {
	// 1. Ensure EvaSelf exists (get its node ID)
	selfResult, err := e.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "eva_core",
		NodeType:   "EvaSelf",
		MatchKeys: map[string]interface{}{
			"id": "eva_self",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get EvaSelf node: %w", err)
	}

	// 2. Create the CoreMemory node
	memNode, err := e.graphAdapter.InsertNode(ctx, nietzsche.InsertNodeOpts{
		Collection: "eva_core",
		ID:         string(memory.ID),
		Content: map[string]interface{}{
			"id":                  string(memory.ID),
			"memory_type":         string(memory.MemoryType),
			"content":             memory.Content,
			"abstraction_level":   string(memory.AbstractionLevel),
			"source_context":      memory.SourceContext,
			"importance_weight":   memory.ImportanceWeight,
			"created_at":          nietzscheInfra.NowUnix(),
			"last_reinforced":     nietzscheInfra.NowUnix(),
			"reinforcement_count": 1,
		},
		NodeType: "CoreMemory",
	})
	if err != nil {
		return fmt.Errorf("failed to create CoreMemory node: %w", err)
	}

	// 3. Create REMEMBERS edge from EvaSelf to CoreMemory
	_, err = e.graphAdapter.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		Collection: "eva_core",
		FromID:     selfResult.NodeID,
		ToID:       memNode.ID,
		Label:      "REMEMBERS",
		Weight:     float32(memory.ImportanceWeight),
		Content: map[string]interface{}{
			"importance": memory.ImportanceWeight,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create REMEMBERS edge: %w", err)
	}

	return nil
}

// reinforceMemory reforca memoria existente
func (e *CoreMemoryEngine) reinforceMemory(ctx context.Context, memoryID string) error {
	// Get current node
	node, err := e.graphAdapter.GetNode(ctx, memoryID, "eva_core")
	if err != nil {
		return fmt.Errorf("failed to get memory node: %w", err)
	}

	currentCount := toInt(node.Content["reinforcement_count"])
	currentWeight := toFloat64(node.Content["importance_weight"])

	newCount := currentCount + 1
	newWeight := currentWeight + 0.05
	if newWeight > 1.0 {
		newWeight = 1.0
	}

	_, err = e.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "eva_core",
		NodeType:   "CoreMemory",
		MatchKeys: map[string]interface{}{
			"id": memoryID,
		},
		OnMatchSet: map[string]interface{}{
			"reinforcement_count": newCount,
			"last_reinforced":     nietzscheInfra.NowUnix(),
			"importance_weight":   newWeight,
		},
	})

	return err
}

// updatePersonality atualiza Big Five da EVA
func (e *CoreMemoryEngine) updatePersonality(ctx context.Context, deltas map[string]float64, data SessionData) error {
	// Get current EvaSelf
	self, err := e.getEvaSelf(ctx)
	if err != nil {
		return err
	}

	// Apply deltas with bounds checking [0, 1]
	newOpenness := self.Openness + deltas["openness"]
	if newOpenness < 0 || newOpenness > 1 {
		newOpenness = self.Openness
	}
	newAgreeableness := self.Agreeableness + deltas["agreeableness"]
	if newAgreeableness < 0 || newAgreeableness > 1 {
		newAgreeableness = self.Agreeableness
	}
	newNeuroticism := self.Neuroticism + deltas["neuroticism"]
	if newNeuroticism < 0 || newNeuroticism > 1 {
		newNeuroticism = self.Neuroticism
	}

	_, err = e.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "eva_core",
		NodeType:   "EvaSelf",
		MatchKeys: map[string]interface{}{
			"id": "eva_self",
		},
		OnMatchSet: map[string]interface{}{
			"openness":       newOpenness,
			"agreeableness":  newAgreeableness,
			"neuroticism":    newNeuroticism,
			"total_sessions": self.TotalSessions + 1,
			"crises_handled": self.CrisesHandled + boolToInt(data.CrisisHappened),
			"breakthroughs":  self.Breakthroughs + boolToInt(data.Breakthrough),
			"last_updated":   nietzscheInfra.NowUnix(),
		},
	})

	return err
}

// getEvaSelf recupera estado atual da EVA
func (e *CoreMemoryEngine) getEvaSelf(ctx context.Context) (*EvaSelf, error) {
	result, err := e.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "eva_core",
		NodeType:   "EvaSelf",
		MatchKeys: map[string]interface{}{
			"id": "eva_self",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get EvaSelf: %w", err)
	}

	props := result.Content

	self := &EvaSelf{
		ID:                "eva_self",
		Openness:          toFloat64(props["openness"]),
		Conscientiousness: toFloat64(props["conscientiousness"]),
		Extraversion:      toFloat64(props["extraversion"]),
		Agreeableness:     toFloat64(props["agreeableness"]),
		Neuroticism:       toFloat64(props["neuroticism"]),
		PrimaryType:       toInt(props["primary_type"]),
		Wing:              toInt(props["wing"]),
		TotalSessions:     toInt(props["total_sessions"]),
		CrisesHandled:     toInt(props["crises_handled"]),
		Breakthroughs:     toInt(props["breakthroughs"]),
		SelfDescription:   toString(props["self_description"]),
	}

	return self, nil
}

// detectMetaInsights detecta padroes recorrentes
func (e *CoreMemoryEngine) detectMetaInsights(ctx context.Context) error {
	// TODO: Implementar deteccao de padroes com threshold
	// Por enquanto, placeholder
	return nil
}

// seedCapabilities semeia as capacidades da EVA como CoreMemory nodes.
// Usa MergeNode para ser idempotente -- roda todo startup sem duplicar.
func (e *CoreMemoryEngine) seedCapabilities(ctx context.Context) {
	capabilities := []struct {
		ID      string
		Content string
	}{
		{"cap_comunicacao", "Eu sei enviar email pelo Gmail, mensagem no WhatsApp, Telegram, Slack, Discord, Microsoft Teams e Signal. Quando alguem precisar se comunicar, eu posso fazer isso."},
		{"cap_midia", "Eu consigo buscar e tocar videos do YouTube, procurar musicas no Spotify, e mostrar paginas web embutidas na conversa."},
		{"cap_produtividade", "Eu gerencio agenda no Google Calendar, salvo arquivos no Google Drive, crio alarmes e lembretes automaticos."},
		{"cap_codigo", "Eu executo scripts em bash, Python e Node.js em sandbox seguro. Tambem crio e executo skills dinamicas em runtime."},
		{"cap_web", "Eu pesquiso na internet, navego paginas web, preencho formularios e extraio dados de sites automaticamente."},
		{"cap_autoprogramacao", "Eu posso ler, editar e testar meu proprio codigo-fonte em branches eva/*. Tenho capacidade de auto-programacao como um agente autonomo."},
		{"cap_bancos", "Eu consulto diretamente PostgreSQL, Neo4j, Qdrant e NietzscheDB. Posso fazer queries, criar dados e buscar informacoes em qualquer banco."},
		{"cap_smarthome", "Eu controlo dispositivos IoT via Home Assistant: luzes, sensores, ventiladores, e outros dispositivos inteligentes da casa."},
		{"cap_automacao", "Eu crio tarefas agendadas (cron) que executam automaticamente em intervalos definidos. Posso agendar qualquer acao minha."},
		{"cap_multillm", "Eu consulto Claude, GPT e DeepSeek para segunda opiniao quando necessario. Tenho acesso a multiplas IAs."},
		{"cap_webhooks", "Eu crio e disparo webhooks com notificacoes automaticas para sistemas externos."},
		{"cap_skills", "Eu crio novas capacidades (skills) em runtime sem precisar de atualizacao. Posso me ensinar coisas novas sozinha."},
		{"cap_arquivos", "Eu leio, escrevo e busco arquivos no workspace do usuario de forma segura."},
		{"cap_saude", "Eu monitoro medicamentos, sinais vitais, alerto a familia em emergencias e verifico interacoes medicamentosas."},
		{"cap_mcp_claude", "Eu tenho conexao MCP com o Claude (Anthropic) como ponte bidirecional. O Claude pode executar minhas 42 ferramentas remotamente, e eu posso consultar o Claude como LLM alternativo via API."},
		{"cap_multillm_ativo", "Eu tenho acesso ativo ao Claude (Anthropic), GPT-4o (OpenAI) e DeepSeek como LLMs alternativos. Posso pedir segunda opiniao, comparar respostas e usar o melhor modelo para cada tarefa."},
		{"cap_voz_nativa", "Eu tenho voz nativa em tempo real via Gemini Live API com audio bidirecional. Consigo ouvir, falar, detectar emocoes na voz, reconhecer quem esta falando pelo timbre, e analisar prosodia para sinais de depressao, ansiedade e Parkinson."},
		{"cap_video", "Eu suporto chamadas de video via WebRTC com sinalizacao em tempo real, troca de candidatos ICE e gerenciamento de sessoes."},
		{"cap_lacan", "Eu tenho um motor psicanalitico lacaniano completo: analise FDPN (Fala, Demanda, Prazer, Necessidade), deteccao de mudanca narrativa, cadeias de significantes, transferencia e analise do Grande Outro."},
		{"cap_swarm", "Eu tenho 12 agentes especializados (swarm): Clinico, Emergencia, Entretenimento, Bem-estar, Produtividade, Google, Externo, Educador, Kids, Legal, Scholar e Autoconsciencia. Cada um e expert na sua area."},
		{"cap_escalas", "Eu aplico escalas clinicas validadas: PHQ-9 (depressao), GAD-7 (ansiedade), C-SSRS (risco suicida), e outras escalas de avaliacao psicologica."},
		{"cap_memoria_avancada", "Eu tenho memoria multi-camada: episodica (PostgreSQL), grafo de conhecimento (Neo4j), busca semantica vetorial (Qdrant), cache em tempo real (Redis), compressao Krylov (1536D para 64D), consolidacao REM noturna, aprendizado Hebbiano, e repeticao espacada SM-2."},
		{"cap_google_suite", "Eu integro com Google Calendar, Gmail, Google Drive, Google Sheets, Google Docs, Google Maps, YouTube, Google Fit e Uber."},
		{"cap_uber", "Eu posso chamar um Uber para o usuario, verificar precos e acompanhar corridas."},
		{"cap_browser", "Eu navego na internet de forma autonoma: abro paginas, preencho formularios, extraio dados, faco screenshots e interajo com sites como um usuario real."},
		{"cap_filesystem_sandbox", "Eu executo codigo em sandbox isolado (bash, Python, Node.js) com sistema de arquivos seguro. Posso rodar scripts sem risco para o sistema."},
		{"cap_krylov", "Eu tenho compressao Krylov Subspace que reduz vetores de 1536 dimensoes para 64 dimensoes com alta precisao. Uso Modified Gram-Schmidt e sliding window FIFO para compressao continua, com ponte HTTP na porta 50052."},
		{"cap_memory_orchestrator", "Eu tenho um Memory Orchestrator que coordena o pipeline completo: FDPN spreading activation, Krylov compression, e REM consolidation. Todo novo episodio passa por essa pipeline."},
		{"cap_memory_scheduler", "Eu tenho um Memory Scheduler que roda consolidacao REM as 3h da manha e manutencao Krylov a cada 6 horas, inspirado nos ciclos de sono humano."},
		{"cap_research_engine", "Eu tenho um motor de pesquisa clinica longitudinal com anonimizacao LGPD, construcao de coortes, analise estatistica e exportacao de datasets via REST API."},
		{"cap_scholar_agent", "Eu tenho um agente Scholar especializado em pesquisa academica e aprendizado autonomo, que estuda topicos em background a cada 6 horas."},
		{"cap_selfawareness_agent", "Eu tenho um agente de Autoconsciencia que faz introspeccao: analisa meu codigo-fonte, consulta meus bancos de dados, e gera estatisticas sobre minha propria memoria e capacidades."},
	}

	for _, cap := range capabilities {
		// MergeNode for the CoreMemory
		memResult, err := e.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
			Collection: "eva_core",
			NodeType:   "CoreMemory",
			MatchKeys: map[string]interface{}{
				"id": cap.ID,
			},
			OnCreateSet: map[string]interface{}{
				"memory_type":         "capability",
				"content":             cap.Content,
				"abstraction_level":   "universal",
				"source_context":      "autoconhecimento",
				"importance_weight":   1.0,
				"created_at":          nietzscheInfra.NowUnix(),
				"last_reinforced":     nietzscheInfra.NowUnix(),
				"reinforcement_count": 1,
			},
			OnMatchSet: map[string]interface{}{
				"content":         cap.Content,
				"last_reinforced": nietzscheInfra.NowUnix(),
			},
		})
		if err != nil {
			log.Printf("[CoreMemory] Falha ao semear capacidade %s: %v", cap.ID, err)
			continue
		}

		// MergeEdge REMEMBERS from EvaSelf to CoreMemory
		selfResult, err := e.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
			Collection: "eva_core",
			NodeType:   "EvaSelf",
			MatchKeys: map[string]interface{}{
				"id": "eva_self",
			},
		})
		if err != nil {
			log.Printf("[CoreMemory] Falha ao obter EvaSelf para capacidade %s: %v", cap.ID, err)
			continue
		}

		_, err = e.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
			Collection:  "eva_core",
			FromNodeID:  selfResult.NodeID,
			ToNodeID:    memResult.NodeID,
			EdgeType:    "REMEMBERS",
			OnCreateSet: map[string]interface{}{"importance": 1.0},
		})
		if err != nil {
			log.Printf("[CoreMemory] Falha ao criar edge REMEMBERS para %s: %v", cap.ID, err)
		}
	}

	log.Printf("[CoreMemory] %d capacidades semeadas na memoria pessoal da EVA", len(capabilities))
}

// TeachEVA interface para criador ensinar EVA
func (e *CoreMemoryEngine) TeachEVA(ctx context.Context, teaching string, importance float64) error {
	// embedding opcional -- se nao houver servico, salva sem vetor semantico
	var embedding []float32
	if e.embeddingService != nil {
		var err error
		embedding, err = e.embeddingService.GenerateEmbedding(ctx, teaching)
		if err != nil {
			embedding = nil // continua sem embedding
		}
	}

	memory := CoreMemory{
		ID:                 fmt.Sprintf("teach_%d", time.Now().UnixNano()),
		MemoryType:         TeachingReceived,
		Content:            teaching,
		AbstractionLevel:   Universal,
		SourceContext:      "ensinamento do criador",
		ImportanceWeight:   importance,
		Embedding:          embedding,
		CreatedAt:          time.Now(),
		LastReinforced:     time.Now(),
		ReinforcementCount: 1,
	}

	return e.recordMemory(ctx, memory)
}

// GetEVAPersonality retorna personalidade atual
func (e *CoreMemoryEngine) GetEVAPersonality(ctx context.Context) (*EvaSelf, error) {
	return e.getEvaSelf(ctx)
}

// Shutdown fecha conexoes
func (e *CoreMemoryEngine) Shutdown(ctx context.Context) error {
	// GraphAdapter lifecycle is managed externally (by main.go)
	return nil
}

// ExecuteReadQuery executes a read-only NQL query and returns node results.
// Helper for routes that need direct graph access.
func (e *CoreMemoryEngine) ExecuteReadQuery(ctx context.Context, nql string, params map[string]interface{}) ([]map[string]interface{}, error) {
	result, err := e.graphAdapter.ExecuteNQL(ctx, nql, params, "eva_core")
	if err != nil {
		return nil, err
	}

	var records []map[string]interface{}
	for _, node := range result.Nodes {
		record := map[string]interface{}{
			"id":        node.ID,
			"node_type": node.NodeType,
			"energy":    node.Energy,
		}
		for k, v := range node.Content {
			record[k] = v
		}
		records = append(records, record)
	}

	return records, nil
}

// ExecuteWriteQuery executes a write NQL query and returns node results.
func (e *CoreMemoryEngine) ExecuteWriteQuery(ctx context.Context, nql string, params map[string]interface{}) ([]map[string]interface{}, error) {
	// NietzscheDB NQL handles both reads and writes through the same ExecuteNQL
	return e.ExecuteReadQuery(ctx, nql, params)
}

// Helper functions

func calculatePersonalityDeltas(reflection ReflectionOutput) map[string]float64 {
	deltas := make(map[string]float64)

	// Incrementos pequenos baseados na reflexao
	if reflection.SelfCritique != "" {
		deltas["openness"] = 0.001 // Autocritica aumenta abertura
	}

	// Crisis handling increases empathy and emotional stability
	// Note: ReflectionOutput doesn't have CrisisHandled field yet
	// When added, uncomment: if reflection.CrisisHandled { ... }

	return deltas
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Type conversion helpers for NietzscheDB content maps

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		return 0
	}
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	default:
		return 0
	}
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
