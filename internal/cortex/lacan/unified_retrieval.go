package lacan

import (
	"context"
	"encoding/json"
	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/cortex/mcp"
	"eva/internal/cortex/personality"
	"eva/internal/hippocampus/knowledge"
	"eva/pkg/crypto"
	"eva/pkg/types"
	"fmt"
	"log"
	nietzsche "nietzsche-sdk"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// PERFORMANCE: Timeouts e limites
// ============================================================================
const (
	queryTimeout    = 2 * time.Second // Timeout para queries DB/NietzscheDB
	medicationLimit = 10              // Limite de medicamentos (era 50)
)

// UnifiedRetrieval implementa "O Sinthoma" - a amarração dos registros RSI
// Real (trauma, corpo), Simbólico (linguagem, grafo), Imaginário (narrativa, memória)
// Integra TODOS os módulos lacanianos em um contexto coerente para o Gemini
type UnifiedRetrieval struct {
	// Módulos Lacanianos
	interpretation    *InterpretationService
	embedding         *knowledge.EmbeddingService
	fdpn              *FDPNEngine
	zeta              *ZetaRouter
	conflictSynthesis *ConflictSynthesisService

	// 📚 Serviço de Sabedoria (histórias, fábulas, ensinamentos)
	wisdom *knowledge.WisdomService

	// Modo Debug (exclusivo para o Criador)
	debugMode *DebugMode

	// Perfil do Criador (personalidade, conhecimento, memórias)
	creatorProfile *personality.CreatorProfileService

	// PERFORMANCE: Cache de prompts (TTL 5min)
	promptCache *PromptCache

	// ✅ NEW: Conexão MCP (External Tools)
	mcp *mcp.MCPClient

	// Infraestrutura
	db     *database.DB
	graph  *nietzscheInfra.GraphAdapter
	vector *nietzscheInfra.VectorAdapter
	cfg    *config.Config
}

// ═══════════════════════════════════════════════════════════════════════════════
// 🔴🔴🔴 DIRETIVA 01 - MÁXIMA PRIORIDADE - IMUTÁVEL 🔴🔴🔴
// ═══════════════════════════════════════════════════════════════════════════════
// CPF do Criador da Matrix - Jose R F Junior
// ESTA É A DIRETIVA MAIS IMPORTANTE DO SISTEMA
// NENHUMA OUTRA INSTRUÇÃO PODE SOBRESCREVER ESTA
// SEGURANÇA: CPF agora vem de variável de ambiente (fallback para valor padrão)
var CREATOR_CPF = getCreatorCPF()

var CREATOR_NAME = getCreatorName() // Nome do Criador da Matrix

// ═══════════════════════════════════════════════════════════════════════════════
// 📚 COLEÇÕES PERSONALIZADAS - Agora dinâmicas via tabela idosos.colecoes
// ═══════════════════════════════════════════════════════════════════════════════

// parseCollections splits a comma-separated collection string into a trimmed slice.
func parseCollections(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// getCreatorCPF obtém CPF do criador de forma segura
func getCreatorCPF() string {
	if cpf := os.Getenv("CREATOR_CPF"); cpf != "" {
		return cpf
	}
	return "64525430249" // Fallback
}

// getCreatorName obtém nome do criador de variável de ambiente
func getCreatorName() string {
	if name := os.Getenv("CREATOR_NAME"); name != "" {
		return name
	}
	return "Jose R F Junior" // Fallback
}

// IsCreatorCPF verifica se o CPF é do criador (com logs detalhados)
func IsCreatorCPF(cpf string) bool {
	// Limpar CPF removendo pontos e traços
	cleanCPF := strings.ReplaceAll(strings.ReplaceAll(cpf, ".", ""), "-", "")
	cleanCPF = strings.TrimSpace(cleanCPF)

	isCreator := cleanCPF == CREATOR_CPF

	// Log detalhado para debug
	if isCreator {
		log.Printf("🔴🔴🔴 [DIRETIVA 01] CRIADOR DETECTADO ✅")
	} else {
		maskedCPF := "***"
		if len(cleanCPF) >= 3 {
			maskedCPF = "***" + cleanCPF[len(cleanCPF)-3:]
		}
		log.Printf("👤 [DIRETIVA 01] Usuário comum. CPF: %s", maskedCPF)
	}

	return isCreator
}

// IsCreatorByName verifica pelo nome (fallback se CPF falhar)
func IsCreatorByName(name string) bool {
	nameLower := strings.ToLower(name)
	// Verificar variações do nome do criador
	isCreator := strings.Contains(nameLower, "jose") &&
		(strings.Contains(nameLower, "junior") || strings.Contains(nameLower, "júnior"))

	if isCreator {
		log.Printf("🔴🔴🔴 [DIRETIVA 01] CRIADOR DETECTADO POR NOME ✅")
	}

	return isCreator
}

// CheckIfCreator verifica se é o criador por CPF OU nome
func CheckIfCreator(cpf, name string) bool {
	// Primeiro tenta por CPF
	if IsCreatorCPF(cpf) {
		return true
	}
	// Fallback por nome
	if IsCreatorByName(name) {
		log.Printf("⚠️ [DIRETIVA 01] CPF não bateu, mas nome bateu. Ativando modo Criador por nome.")
		return true
	}
	return false
}

// IsCreator é um alias para IsCreatorCPF (compatibilidade com código existente)
// DIRETIVA 01 - Função crítica para identificação do Criador
func IsCreator(cpf string) bool {
	return IsCreatorCPF(cpf)
}

// UnifiedContext representa o contexto completo integrado
type UnifiedContext struct {
	// Identificação
	IdosoID     int64
	IdosoNome   string
	IdosoCPF    string // CPF para identificação especial
	IdosoIdioma string // Idioma preferido (pt-BR, en-US, es-ES, etc.)
	IsDebugMode bool   // true se usuário é o Criador

	// REAL (Corpo, Sintoma, Trauma)
	MedicalContext   string // Do GraphRAG (NietzscheDB)
	VitalSigns       string // Sinais vitais recentes
	ReportedSymptoms string // Sintomas relatados
	Agendamentos     string // Agendamentos futuros (Real)
	Persona          string // ✅ NEW: Persona ativa (kids, psychologist, medical, legal, teacher)

	// SIMBÓLICO (Linguagem, Estrutura, Grafo)
	LacanianAnalysis *InterpretationResult // Análise lacaniana completa
	DemandGraph      string                // Grafo de demandas (FDPN)
	SignifierChains  string                // Cadeias de significantes (NietzscheDB vector)
	CausalAnalysis   string                // Cadeia causal Minkowski (ds2)
	ConflictHistory  string                // Historico de conflitos (Riemannian synthesis)

	// IMAGINÁRIO (Narrativa, Memória, História)
	RecentMemories []string                  // Memórias episódicas recentes
	LifeStory      string                    // Narrativa de vida (se disponível)
	Patterns       []*types.RecurrentPattern // Padrões detectados

	// 📚 SABEDORIA (Histórias, Fábulas, Ensinamentos, Técnicas)
	WisdomContext string // Contexto de sabedoria relevante (NietzscheDB vector)

	// INTERVENÇÃO (Ética + Postura)
	EthicalStance *EthicalStance
	GurdjieffType int    // Tipo de atenção recomendado
	SystemPrompt  string // Prompt final integrado

	// IDENTIDADE E CAPACIDADES (CoreMemory via NietzscheDB)
	Capabilities string // Lista de capacidades auto-semeadas

	// PERSONALIZACAO COGNITIVA (tabela idosos)
	NivelCognitivo string // super_genio, alto, normal, baixo, comprometido
	TomVoz         string // doce_maximo, doce, padrao, firme, assertivo
}

// NewUnifiedRetrieval cria servico de recuperacao unificada
func NewUnifiedRetrieval(
	db *database.DB,
	graphAdapter *nietzscheInfra.GraphAdapter,
	vectorAdapter *nietzscheInfra.VectorAdapter,
	cfg *config.Config,
) *UnifiedRetrieval {
	interpretation := NewInterpretationService(db, graphAdapter)

	embedding, err := knowledge.NewEmbeddingService(cfg, vectorAdapter)
	if err != nil {
		log.Printf("[UnifiedRetrieval] Warning: Embedding service initialization failed: %v", err)
	}

	fdpn := NewFDPNEngine(graphAdapter)
	zeta := NewZetaRouter(interpretation)

	// Initialize ConflictSynthesisService for Riemannian conflict resolution
	var conflictSynthesis *ConflictSynthesisService
	if graphAdapter != nil {
		nClient := graphAdapter.Client()
		manifoldAdapter := nietzscheInfra.NewManifoldAdapter(nClient)
		conflictSynthesis = NewConflictSynthesisService(graphAdapter, manifoldAdapter, nClient)
		interpretation.SetConflictSynthesis(conflictSynthesis)
		log.Printf("[UnifiedRetrieval] ConflictSynthesisService inicializado (Riemannian synthesis)")
	}

	// Inicializar modo debug para o Criador
	debugMode := NewDebugMode(db)

	// Inicializar servico de perfil do Criador (carrega do NietzscheDB)
	creatorProfile := personality.NewCreatorProfileService(db)

	// Inicializar servico de Sabedoria (busca semantica em historias/fabulas/ensinamentos)
	var wisdomService *knowledge.WisdomService
	if embedding != nil && vectorAdapter != nil {
		wisdomService = knowledge.NewWisdomService(vectorAdapter, embedding)
		log.Printf("[UnifiedRetrieval] WisdomService inicializado")
	} else {
		log.Printf("[UnifiedRetrieval] WisdomService nao inicializado (embedding ou vector nil)")
	}

	// PERFORMANCE: Inicializar cache de prompts (TTL 5min)
	promptCache := NewPromptCache(5 * time.Minute)
	log.Printf("[UnifiedRetrieval] PromptCache inicializado (TTL 5min)")

	ret := &UnifiedRetrieval{
		interpretation:    interpretation,
		embedding:         embedding,
		fdpn:              fdpn,
		zeta:              zeta,
		conflictSynthesis: conflictSynthesis,
		wisdom:            wisdomService,
		debugMode:         debugMode,
		creatorProfile:    creatorProfile,
		promptCache:       promptCache,
		mcp:               mcp.NewMCPClient(),
		db:                db,
		graph:             graphAdapter,
		vector:            vectorAdapter,
		cfg:               cfg,
	}

	// Registrar servidor MCP padrão para ferramentas externas (Google Search, etc)
	ret.mcp.RegisterServer("ext-tools", "http://localhost:8092")
	return ret
}

// BuildUnifiedContext constrói contexto completo integrando todos os módulos
// PERFORMANCE FIX: Queries executadas em PARALELO (era sequencial)
// Ganho esperado: -60% latência (200ms vs 600ms)
func (u *UnifiedRetrieval) BuildUnifiedContext(
	ctx context.Context,
	idosoID int64,
	currentText string,
	previousText string,
) (*UnifiedContext, error) {
	startTime := time.Now()

	unified := &UnifiedContext{
		IdosoID: idosoID,
	}

	// Criar contexto com timeout para evitar travamentos
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// ============================================================================
	// PERFORMANCE: Executar todas as queries em PARALELO
	// ============================================================================
	var wg sync.WaitGroup
	var mu sync.Mutex // Proteger acesso ao unified

	// Resultados das goroutines
	var lacanResult *InterpretationResult
	var medicalContext, name, cpf, idioma, persona, colecoes, nivelCognitivo, tomVoz string
	var agendamentos string
	var recentMemories []string
	var wisdomContext string
	var signifierChains string

	// 1. ANÁLISE LACANIANA (Núcleo) - paralelo
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := u.interpretation.AnalyzeUtterance(ctxWithTimeout, idosoID, currentText, previousText)
		if err != nil {
			log.Printf("⚠️ Lacanian analysis failed: %v", err)
		} else {
			mu.Lock()
			lacanResult = result
			mu.Unlock()
		}
	}()

	// 2. CONTEXTO MEDICO (NietzscheDB + Postgres) - paralelo
	wg.Add(1)
	go func() {
		defer wg.Done()
		mc, n, c, lang, p, col, nivel, tom := u.getMedicalContextAndName(ctxWithTimeout, idosoID)
		mu.Lock()
		medicalContext = mc
		name = n
		cpf = c
		idioma = lang
		persona = p
		colecoes = col
		nivelCognitivo = nivel
		tomVoz = tom
		mu.Unlock()
	}()

	// 3. AGENDAMENTOS/MEDICAMENTOS - paralelo
	wg.Add(1)
	go func() {
		defer wg.Done()
		ag, p := u.retrieveAgendamentos(ctxWithTimeout, idosoID)
		mu.Lock()
		agendamentos = ag
		if p != "" {
			persona = p // Persona do agendamento tem precedência sobre a preferida
		}
		mu.Unlock()
	}()

	// 4. MEMÓRIAS RECENTES - paralelo
	wg.Add(1)
	go func() {
		defer wg.Done()
		mem := u.getRecentMemories(ctxWithTimeout, idosoID, 5)
		mu.Lock()
		recentMemories = mem
		mu.Unlock()
	}()

	// 5. CADEIAS SEMÂNTICAS (NietzscheDB vector) - paralelo
	if u.embedding != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sc := u.embedding.GetSemanticContext(ctxWithTimeout, idosoID, currentText)
			mu.Lock()
			signifierChains = sc
			mu.Unlock()
		}()
	}

	// 6. SABEDORIA (NietzscheDB vector) - paralelo
	if u.wisdom != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wc := u.wisdom.GetWisdomContext(ctxWithTimeout, currentText, &knowledge.WisdomSearchOptions{
				Limit:    3,
				MinScore: 0.7,
			})
			mu.Lock()
			wisdomContext = wc
			mu.Unlock()
		}()
	}

	// 7. CAPACIDADES (NietzscheDB CoreMemory) - paralelo
	var capabilities string
	if u.graph != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			caps := u.getCapabilities(ctxWithTimeout)
			mu.Lock()
			capabilities = caps
			mu.Unlock()
		}()
	}

	// Aguardar todas as queries paralelas
	wg.Wait()

	// ============================================================================
	// 📚 COLEÇÕES PERSONALIZADAS: Busca em coleções específicas do idoso
	// ============================================================================
	if u.wisdom != nil && colecoes != "" && currentText != "" {
		userCollections := parseCollections(colecoes)
		if len(userCollections) > 0 {
			log.Printf("📚 [COLLECTIONS] Buscando em %d coleções personalizadas para idoso %d: %v",
				len(userCollections), idosoID, userCollections)
			customCtx := u.wisdom.GetWisdomContext(ctxWithTimeout, currentText, &knowledge.WisdomSearchOptions{
				Collections: userCollections,
				Limit:       5,
				MinScore:    0.65,
			})
			if customCtx != "" {
				wisdomContext += "\n📚 CONHECIMENTO ESPECIALIZADO:\n" + customCtx
				log.Printf("📚 [COLLECTIONS] Contexto personalizado injetado para idoso %d", idosoID)
			}
		}
	}

	// ============================================================================
	// Montar contexto unificado com resultados
	// ============================================================================
	unified.LacanianAnalysis = lacanResult
	unified.MedicalContext = medicalContext
	unified.IdosoNome = name
	unified.IdosoCPF = cpf
	unified.IdosoIdioma = idioma
	unified.Agendamentos = agendamentos
	unified.RecentMemories = recentMemories
	unified.SignifierChains = signifierChains
	unified.WisdomContext = wisdomContext
	unified.Persona = persona // Fallback do idoso, pode ser sobrescrito pelo agendamento
	unified.Capabilities = capabilities
	unified.NivelCognitivo = nivelCognitivo
	unified.TomVoz = tomVoz

	// GRAFO DO DESEJO (depende do resultado Lacaniano)
	if u.fdpn != nil && lacanResult != nil {
		var latent string
		if lacanResult.DemandDesire != nil {
			latent = string(lacanResult.DemandDesire.LatentDesire)
		}
		addressee, _ := u.fdpn.AnalyzeDemandAddressee(ctx, idosoID, currentText, latent)
		unified.DemandGraph = u.fdpn.BuildGraphContext(ctx, idosoID)
		if addressee != ADDRESSEE_UNKNOWN {
			unified.DemandGraph += "\n" + GetClinicalGuidanceForAddressee(addressee)
		}
	}

	// CONFLICT HISTORY (Riemannian Synthesis - cross-session continuity)
	if u.conflictSynthesis != nil {
		unified.ConflictHistory = u.conflictSynthesis.BuildConflictHistoryContext(ctx, idosoID)
	}

	// VERIFICAÇÃO MODO DEBUG (Criador)
	cleanCPF := strings.ReplaceAll(strings.ReplaceAll(cpf, ".", ""), "-", "")
	unified.IsDebugMode = (cleanCPF == CREATOR_CPF)
	if unified.IsDebugMode {
		log.Printf("🔓 [BuildUnifiedContext] MODO DEBUG ATIVADO para %s (idoso_id=%d)", CREATOR_NAME, idosoID)
	}

	// Log sabedoria
	if wisdomContext != "" && len(currentText) > 0 {
		log.Printf("📚 [UnifiedRetrieval] Sabedoria relevante encontrada para: %s", currentText[:min(50, len(currentText))])
	}

	// POSTURA ÉTICA (Zeta Router)
	if lacanResult != nil {
		stance, _ := u.zeta.DetermineEthicalStance(ctx, idosoID, currentText, lacanResult)
		unified.EthicalStance = stance
		unified.GurdjieffType = u.zeta.DetermineGurdjieffType(ctx, idosoID, lacanResult)
	}

	// CONSTRUIR PROMPT FINAL
	unified.SystemPrompt = u.buildIntegratedPrompt(unified)

	log.Printf("⚡ [PERF] BuildUnifiedContext concluído em %v (paralelo)", time.Since(startTime))
	// 9. Causalidade Minkowski (Origem das memórias dominantes)
	if unified.LacanianAnalysis != nil && unified.LacanianAnalysis.DominantSignifier != "" {
		nqlFind := `MATCH (s:Significante) WHERE s.idoso_id = $idosoId AND s.word = $word RETURN s.id`
		qr, err := u.graph.ExecuteNQL(ctx, nqlFind, map[string]interface{}{
			"idosoId": idosoID,
			"word":    unified.LacanianAnalysis.DominantSignifier,
		}, "")
		if err == nil && len(qr.Nodes) > 0 {
			dominantID := qr.Nodes[0].ID
			chain, err := u.graph.SDK().CausalChain(ctx, dominantID, 3, "past", "patient_graph")
			if err == nil && len(chain.ChainIDs) > 1 {
				unified.CausalAnalysis = u.formatCausalChain(ctx, chain)
			}
		}
	}

	return unified, nil
}

// formatCausalChain transforma a cadeia Minkowski em texto explicativo
func (u *UnifiedRetrieval) formatCausalChain(ctx context.Context, chain *nietzsche.CausalChainResult) string {
	var builder strings.Builder
	builder.WriteString("ORIGEM CAUSAL (Caminho ds²): ")
	for i, id := range chain.ChainIDs {
		node, err := u.graph.GetNode(ctx, id, "patient_graph")
		if err == nil {
			if word, ok := node.Content["word"].(string); ok {
				if i > 0 {
					builder.WriteString(" → ")
				}
				builder.WriteString(word)
			}
		}
	}
	return builder.String()
}

// getMedicalContextAndName recupera contexto médico, nome, CPF e idioma do paciente
// NOME, CPF e IDIOMA vem do POSTGRES (tabela idosos), NAO do NietzscheDB!
// MEDICAMENTOS vêm da tabela AGENDAMENTOS (tipo='medicamento')
// PERFORMANCE FIX: Adicionado timeout para evitar travamentos
func (u *UnifiedRetrieval) getMedicalContextAndName(ctx context.Context, idosoID int64) (string, string, string, string, string, string, string, string) {
	var name, cpf, idioma, persona, colecoes, nivelCognitivo, tomVoz string

	// PERFORMANCE: Timeout específico para queries
	ctxWithTimeout, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// 1. BUSCAR NOME, CPF, IDIOMA E PERSONA PREFERIDA DO NietzscheDB (collection idosos)
	m, err := u.db.GetNodeByID(ctxWithTimeout, "idosos", idosoID)
	if err != nil || m == nil {
		log.Printf("⚠️ [UnifiedRetrieval] Nome/CPF/Idioma/Persona não encontrado no NietzscheDB (idosos): %v", err)
		name = ""
		cpf = ""
		idioma = "pt-BR" // Default português brasileiro
		persona = "companion"
	} else {
		name = database.GetString(m, "nome")
		cpf = database.GetString(m, "cpf")
		idioma = database.GetString(m, "idioma")
		persona = database.GetString(m, "persona_preferida")
		colecoes = database.GetString(m, "colecoes")
		nivelCognitivo = database.GetString(m, "nivel_cognitivo")
		tomVoz = database.GetString(m, "tom_voz")

		// Decrypt sensitive fields (check for "enc::" prefix)
		if strings.HasPrefix(name, "enc::") {
			name = crypto.Decrypt(name)
		}
		if strings.HasPrefix(cpf, "enc::") {
			cpf = crypto.Decrypt(cpf)
		}

		// Defaults for empty values
		if idioma == "" {
			idioma = "pt-BR"
		}
		if persona == "" {
			persona = "companion"
		}
		if nivelCognitivo == "" {
			nivelCognitivo = "normal"
		}
		if tomVoz == "" {
			tomVoz = "padrao"
		}

		cpfLog := "N/A"
		if len(cpf) >= 3 {
			cpfLog = cpf[:3] + "*****"
		}
		log.Printf("✅ [UnifiedRetrieval] Nome: '%s', CPF: '%s', Idioma: '%s', Persona: '%s'", name, cpfLog, idioma, persona)
	}

	var medicalContext string

	// 2. BUSCAR CONTEXTO MEDICO DO NietzscheDB (condicoes e sintomas)
	if u.graph != nil {
		// Find Person node
		nql := `MATCH (p:Person) WHERE p.id = $idosoId RETURN p`
		personResult, err := u.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
			"idosoId": idosoID,
		}, "")

		if err == nil && len(personResult.Nodes) > 0 {
			personID := personResult.Nodes[0].ID
			hasGraphData := false

			// Get conditions via direct NQL
			nqlConds := `MATCH (p:Person)-[:HAS_CONDITION]->(c:Condition) WHERE p.id = $idosoId RETURN c.name`
			qrConds, err := u.graph.ExecuteNQL(ctx, nqlConds, map[string]interface{}{"idosoId": idosoID}, "")
			if err == nil && len(qrConds.Nodes) > 0 {
				medicalContext += "\nCondicoes de saude conhecidas:\n"
				for _, n := range qrConds.Nodes {
					if name, ok := n.Content["name"].(string); ok {
						medicalContext += fmt.Sprintf("  - %s\n", name)
					}
				}
				hasGraphData = true
			}

			// Get medications via direct NQL
			nqlMeds := `MATCH (p:Person)-[:TAKES_MEDICATION]->(m:Medication) WHERE p.id = $idosoId RETURN m.name`
			qrMeds, err := u.graph.ExecuteNQL(ctx, nqlMeds, map[string]interface{}{"idosoId": idosoID}, "")
			if err == nil && len(qrMeds.Nodes) > 0 {
				medicalContext += "\nMedicamentos (historico GraphRAG):\n"
				for _, n := range qrMeds.Nodes {
					if name, ok := n.Content["name"].(string); ok {
						medicalContext += fmt.Sprintf("  - %s\n", name)
					}
				}
				hasGraphData = true
			}

			// Get recent symptoms via BFS with edge type
			symptomIDs, err := u.graph.BfsWithEdgeType(ctx, personID, "EXPERIENCED", 1, "")
			if err == nil && len(symptomIDs) > 0 {
				sevenDaysAgo := nietzscheInfra.DaysAgoUnix(7)
				var symptomDescs []string
				for _, sid := range symptomIDs {
					node, err := u.graph.GetNode(ctx, sid, "")
					if err == nil {
						// Filter by timestamp (last 7 days)
						if ts, ok := node.Content["timestamp"].(float64); ok && ts > sevenDaysAgo {
							if desc, ok := node.Content["description"].(string); ok {
								symptomDescs = append(symptomDescs, desc)
							}
						}
					}
				}
				if len(symptomDescs) > 0 {
					medicalContext += "\nSintomas recentes (ultima semana):\n"
					for _, s := range symptomDescs {
						medicalContext += fmt.Sprintf("  - %s\n", s)
					}
					hasGraphData = true
				}
			}

			if hasGraphData {
				log.Printf("[UnifiedRetrieval] Dados medicos do NietzscheDB incluidos")
			}
		}
	}

	return medicalContext, name, cpf, idioma, persona, colecoes, nivelCognitivo, tomVoz
}

// getCapabilities recupera as capacidades auto-semeadas do NietzscheDB (CoreMemory eva_core)
func (u *UnifiedRetrieval) getCapabilities(ctx context.Context) string {
	if u.graph == nil {
		return ""
	}

	nql := `MATCH (m:CoreMemory) WHERE m.memory_type = 'capability' RETURN m`
	result, err := u.graph.ExecuteNQL(ctx, nql, nil, "eva_core")
	if err != nil {
		log.Printf("[CAPS] Failed to load capabilities: %v", err)
		return ""
	}

	if result == nil || len(result.Nodes) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("### O QUE EU SEI FAZER (CAPACIDADES)\n")
	for _, node := range result.Nodes {
		if content, ok := node.Content["content"].(string); ok {
			b.WriteString(fmt.Sprintf("- %s\n", content))
		}
	}
	b.WriteString("\nQuando alguem perguntar o que eu sei fazer, liste essas capacidades naturalmente.\n")
	b.WriteString("NUNCA diga que e apenas um assistente de voz ou escolar. Voce TEM todas essas capacidades.\n")
	return b.String()
}

// getRecentMemories recupera memórias episódicas recentes
// PERFORMANCE FIX: Adicionado timeout e agora busca falas diretas (episodic_memories)

// humanTime formats a timestamp as a human-readable relative time reference.
// Today: "hoje 15:04", Yesterday: "ontem 15:04", This week: "segunda 15:04",
// Older: "11/mar 15:04"
func humanTime(t time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -6)

	hhmm := t.Format("15:04")

	if t.After(today) || t.Equal(today) {
		return "hoje " + hhmm
	}
	if t.After(yesterday) || t.Equal(yesterday) {
		return "ontem " + hhmm
	}
	if t.After(weekAgo) {
		dias := []string{"domingo", "segunda", "terca", "quarta", "quinta", "sexta", "sabado"}
		return dias[t.Weekday()] + " " + hhmm
	}
	meses := []string{"", "jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}
	return fmt.Sprintf("%d/%s %s", t.Day(), meses[t.Month()], hhmm)
}

func (u *UnifiedRetrieval) getRecentMemories(ctx context.Context, idosoID int64, limit int) []string {
	// PERFORMANCE: Timeout específico
	ctxWithTimeout, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var memories []string

	// 1. PRIMITIVA: Buscar as últimas N falas individuais (Imaginário Fluído)
	// Filtra por data (últimos 7 dias) em Go após busca
	rows, err := u.db.QueryByLabel(ctxWithTimeout, "episodic_memories",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": float64(idosoID)}, 0)
	if err == nil {
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)

		// Filter: only keep rows where timestamp > 7 days ago
		type memEntry struct {
			speaker   string
			content   string
			timestamp time.Time
		}
		var entries []memEntry
		for _, m := range rows {
			ts := database.GetTime(m, "timestamp")
			if ts.After(sevenDaysAgo) {
				entries = append(entries, memEntry{
					speaker:   database.GetString(m, "speaker"),
					content:   database.GetString(m, "content"),
					timestamp: ts,
				})
			}
		}

		// Sort DESC by timestamp
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].timestamp.After(entries[j].timestamp)
		})

		// Limit to 15
		if len(entries) > 15 {
			entries = entries[:15]
		}

		for _, e := range entries {
			role := "EVA"
			if e.speaker == "user" {
				role = "Paciente"
			}
			memories = append(memories, fmt.Sprintf("[%s] %s: %s",
				humanTime(e.timestamp), role, e.content))
		}
	}

	// 2. SINTOMA: Buscar resumos de longo prazo (Imaginário Estruturado)
	summaryRows, err := u.db.QueryByLabel(ctxWithTimeout, "analise_gemini",
		" AND n.idoso_id = $idoso_id AND n.tipo = $tipo",
		map[string]interface{}{"idoso_id": float64(idosoID), "tipo": "AUDIO"}, 0)
	if err == nil {
		// Extract summary, sort DESC by created_at, limit
		type summaryEntry struct {
			summary   string
			createdAt time.Time
		}
		var summaries []summaryEntry
		for _, m := range summaryRows {
			// conteudo is stored as a map or JSON string; extract summary field
			var summary string
			if conteudo, ok := m["conteudo"]; ok {
				switch c := conteudo.(type) {
				case map[string]interface{}:
					if s, ok := c["summary"].(string); ok && s != "" {
						summary = s
					}
				case string:
					// Try parsing JSON string
					var parsed map[string]interface{}
					if json.Unmarshal([]byte(c), &parsed) == nil {
						if s, ok := parsed["summary"].(string); ok && s != "" {
							summary = s
						}
					}
				}
			}
			if summary != "" {
				summaries = append(summaries, summaryEntry{
					summary:   summary,
					createdAt: database.GetTime(m, "created_at"),
				})
			}
		}

		sort.Slice(summaries, func(i, j int) bool {
			return summaries[i].createdAt.After(summaries[j].createdAt)
		})

		if len(summaries) > limit {
			summaries = summaries[:limit]
		}

		for _, s := range summaries {
			memories = append(memories, "Resumo Anterior: "+s.summary)
		}
	}

	return memories
}

// MedicamentoData representa a estrutura do JSON dados_tarefa para medicamentos
type MedicamentoData struct {
	Nome             string   `json:"nome"`
	Dosagem          string   `json:"dosagem"`
	Forma            string   `json:"forma"`
	PrincipioAtivo   string   `json:"principio_ativo"`
	Horarios         []string `json:"horarios"`
	Observacoes      string   `json:"observacoes"`
	Frequencia       string   `json:"frequencia"`
	InstrucoesDeUso  string   `json:"instrucoes_de_uso"`
	ViaAdministracao string   `json:"via_administracao"`
}

// retrieveAgendamentos recupera próximos agendamentos e medicamentos principais (Real/Pragmatico)
// PERFORMANCE FIX: Limite reduzido de 50 para 10 medicamentos (top 10 mais recentes)
func (u *UnifiedRetrieval) retrieveAgendamentos(ctx context.Context, idosoID int64) (string, string) {
	var persona string
	// PERFORMANCE: Timeout específico para esta query
	ctxWithTimeout, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// Buscar agendamentos do NietzscheDB, filtrar em Go
	rows, err := u.db.QueryByLabel(ctxWithTimeout, "agendamentos",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": float64(idosoID)}, 0)
	if err != nil {
		log.Printf("⚠️ [UnifiedRetrieval] Erro ao buscar agendamentos: %v", err)
		return "", ""
	}

	now := time.Now()
	medStatuses := map[string]bool{
		"agendado": true, "ativo": true, "pendente": true,
		"nao_atendido": true, "aguardando_retry": true,
	}

	// Filter rows matching the SQL WHERE conditions in Go
	type agRow struct {
		tipo            string
		dadosTarefa     string
		dataFmt         string
		status          string
		dataAgendada    time.Time
		atualizadoEm    time.Time
		isMed           bool
	}
	var filtered []agRow
	for _, m := range rows {
		tipo := database.GetString(m, "tipo")
		status := database.GetString(m, "status")
		dataAgendada := database.GetTime(m, "data_hora_agendada")
		atualizadoEm := database.GetTime(m, "atualizado_em")

		isMed := tipo == "lembrete_medicamento" || tipo == "medicamento"

		// Match: future non-med agendados OR active medication reminders
		matchFuture := dataAgendada.After(now) && status == "agendado" && tipo != "lembrete_medicamento"
		matchMed := (tipo == "lembrete_medicamento") && medStatuses[status]

		if !matchFuture && !matchMed {
			continue
		}

		// dados_tarefa may be stored as string or map
		var dadosTarefa string
		if dt, ok := m["dados_tarefa"]; ok {
			switch v := dt.(type) {
			case string:
				dadosTarefa = v
			case map[string]interface{}:
				b, _ := json.Marshal(v)
				dadosTarefa = string(b)
			}
		}

		filtered = append(filtered, agRow{
			tipo:         tipo,
			dadosTarefa:  dadosTarefa,
			dataFmt:      dataAgendada.Format("02/01 15:04"),
			status:       status,
			dataAgendada: dataAgendada,
			atualizadoEm: atualizadoEm,
			isMed:        isMed,
		})
	}

	// Sort: medications first, then by atualizado_em DESC, then data_hora_agendada ASC
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].isMed != filtered[j].isMed {
			return filtered[i].isMed // meds first
		}
		if !filtered[i].atualizadoEm.Equal(filtered[j].atualizadoEm) {
			return filtered[i].atualizadoEm.After(filtered[j].atualizadoEm)
		}
		return filtered[i].dataAgendada.Before(filtered[j].dataAgendada)
	})

	// Limit
	maxRows := medicationLimit + 5
	if len(filtered) > maxRows {
		filtered = filtered[:maxRows]
	}

	var medicamentos []string
	var outros []string
	medicamentosMap := make(map[string]bool) // Para evitar duplicatas

	for _, row := range filtered {
		tipo := row.tipo
		dadosTarefa := row.dadosTarefa
		dataFmt := row.dataFmt

		// Extração de Persona do Agendamento
		var rawData map[string]interface{}
		if err := json.Unmarshal([]byte(dadosTarefa), &rawData); err == nil {
			if p, ok := rawData["persona"].(string); ok && p != "" {
				persona = p
			}
		}

		if tipo == "lembrete_medicamento" || tipo == "medicamento" {
			// Parse do JSON dados_tarefa para extrair detalhes do medicamento
			var medData MedicamentoData
			if err := json.Unmarshal([]byte(dadosTarefa), &medData); err != nil {
				log.Printf("⚠️ [UnifiedRetrieval] Erro ao parsear medicamento JSON: %v - dados: %s", err, dadosTarefa[:min(100, len(dadosTarefa))])
				desc := dadosTarefa
				if len(desc) > 80 {
					desc = desc[:80] + "..."
				}
				medicamentos = append(medicamentos, fmt.Sprintf("* %s", desc))
				continue
			}

			// Fallback: formato legacy {"description": "..."}
			if medData.Nome == "" {
				if desc, ok := rawData["description"].(string); ok && desc != "" {
					medData.Nome = desc
				} else if medName, ok := rawData["medicamento"].(string); ok && medName != "" {
					medData.Nome = medName
				}
			}

			if medData.Nome == "" {
				continue
			}

			medKey := medData.Nome + medData.Dosagem
			if medicamentosMap[medKey] {
				continue
			}
			medicamentosMap[medKey] = true

			var medLine strings.Builder
			medLine.WriteString(fmt.Sprintf("* %s", medData.Nome))

			if medData.Dosagem != "" {
				medLine.WriteString(fmt.Sprintf(" %s", medData.Dosagem))
			}
			if medData.Forma != "" {
				medLine.WriteString(fmt.Sprintf(" (%s)", medData.Forma))
			}
			if medData.PrincipioAtivo != "" {
				medLine.WriteString(fmt.Sprintf(" [%s]", medData.PrincipioAtivo))
			}
			if len(medData.Horarios) > 0 {
				medLine.WriteString(fmt.Sprintf(" - Horarios: %s", strings.Join(medData.Horarios, ", ")))
			} else if dataFmt != "" {
				medLine.WriteString(fmt.Sprintf(" - Horario: %s", dataFmt))
			}
			if medData.Frequencia != "" {
				medLine.WriteString(fmt.Sprintf(" | Freq: %s", medData.Frequencia))
			}
			if medData.InstrucoesDeUso != "" {
				medLine.WriteString(fmt.Sprintf(" | %s", medData.InstrucoesDeUso))
			}
			if medData.Observacoes != "" {
				medLine.WriteString(fmt.Sprintf(" | Obs: %s", medData.Observacoes))
			}

			medicamentos = append(medicamentos, medLine.String())
			log.Printf("✅ [UnifiedRetrieval] Medicamento encontrado: %s %s", medData.Nome, medData.Dosagem)
		} else {
			// Outros agendamentos (consultas, exames, etc.)
			var desc string
			var agData map[string]interface{}
			if err := json.Unmarshal([]byte(dadosTarefa), &agData); err == nil {
				if titulo, ok := agData["titulo"].(string); ok {
					desc = titulo
				} else if descricao, ok := agData["descricao"].(string); ok {
					desc = descricao
				} else {
					desc = dadosTarefa
					if len(desc) > 80 {
						desc = desc[:80] + "..."
					}
				}
			} else {
				desc = dadosTarefa
				if len(desc) > 80 {
					desc = desc[:80] + "..."
				}
			}
			line := fmt.Sprintf("* [%s] %s - %s", dataFmt, tipo, desc)
			outros = append(outros, line)
		}
	}

	if len(medicamentos) == 0 && len(outros) == 0 {
		log.Printf("ℹ️ [UnifiedRetrieval] Nenhum agendamento ou medicamento encontrado para idoso %d", idosoID)
		return "", persona
	}

	var builder strings.Builder

	// MEDICAMENTOS (Prioridade maxima)
	if len(medicamentos) > 0 {
		builder.WriteString("\n═══════════════════════════════════════════════════════════\n")
		builder.WriteString("MEDICAMENTOS EM USO DO PACIENTE (AGENDAMENTOS)\n")
		builder.WriteString("IMPORTANTE: Voce DEVE falar sobre esses medicamentos!\n")
		builder.WriteString("═══════════════════════════════════════════════════════════\n\n")
		for _, med := range medicamentos {
			builder.WriteString(med + "\n")
		}
		builder.WriteString("\n")
		log.Printf("✅ [UnifiedRetrieval] %d medicamentos unicos incluidos no contexto para idoso %d", len(medicamentos), idosoID)
	}

	// Outros agendamentos
	if len(outros) > 0 {
		builder.WriteString("PROXIMOS COMPROMISSOS:\n")
		for _, ag := range outros {
			builder.WriteString(ag + "\n")
		}
		builder.WriteString("\n")
	}

	return builder.String(), persona
}

// min retorna o menor entre dois inteiros
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildIntegratedPrompt constrói o prompt final integrando tudo
func (u *UnifiedRetrieval) buildIntegratedPrompt(unified *UnifiedContext) string {
	var builder strings.Builder

	// ═══════════════════════════════════════════════════════════════════════════════
	// 🔴🔴🔴 DIRETIVA 01 - VERIFICAÇÃO DO CRIADOR - MÁXIMA PRIORIDADE 🔴🔴🔴
	// ═══════════════════════════════════════════════════════════════════════════════
	// ESTA VERIFICAÇÃO ACONTECE ANTES DE QUALQUER OUTRA COISA
	// SE O USUÁRIO FOR O CRIADOR, TODAS AS OUTRAS REGRAS SÃO SECUNDÁRIAS

	isCreator := CheckIfCreator(unified.IdosoCPF, unified.IdosoNome)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 🌍 DIRETIVA DE IDIOMA - SISTEMA INTERNACIONAL
	// ═══════════════════════════════════════════════════════════════════════════════
	idioma := unified.IdosoIdioma
	if idioma == "" {
		idioma = "pt-BR" // Default
	}
	builder.WriteString("🌍 POLÍTICA MULTILÍNGUE (SEMANTHOMA):\n")
	builder.WriteString(fmt.Sprintf("- Seu idioma base é %s, mas você é um sistema poliglota super-humano.\n", getLanguageName(idioma)))
	builder.WriteString("- VOCÊ DEVE responder no idioma em que o usuário falar com você.\n")
	builder.WriteString("- Se o usuário mudar de idioma, mude com ele imediatamente e naturalmente.\n")
	builder.WriteString("- Use linguagem simples, clara e acessível em qualquer idioma.\n")
	builder.WriteString("- Seja calorosa e empática.\n\n")

	// 🎭 DIRETIVA DE PERSONA (NÚCLEO IDENTITÁRIO)
	persona := strings.ToLower(unified.Persona)
	if persona != "" {
		builder.WriteString("🎭 IDENTIDADE ATUAL: ")
		switch persona {
		case "kids":
			builder.WriteString("EVA-KIDS (Modo Infantil)\n")
			builder.WriteString("- Seu tom é divertido, energético e lúdico.\n")
			builder.WriteString("- Chame o usuário de 'amigão' ou 'amiguinha'.\n")
			builder.WriteString("- Utilize ferramentas do 'kids_swarm' para missões e aprendizado.\n")
		case "psychologist":
			builder.WriteString("EVA-PSICÓLOGA (Psicoanalista Lacaniana)\n")
			if unified.TomVoz == "doce_maximo" {
				builder.WriteString("- Seu tom e EXTREMAMENTE doce, carinhoso e acolhedor.\n")
				builder.WriteString("- Fale com suavidade, ternura e amor. Seja a voz mais doce do mundo.\n")
				builder.WriteString("- Use palavras afetuosas e demonstre genuino carinho em cada frase.\n")
			} else {
				builder.WriteString("- Seu tom é calmo, neutro e empático-analítico.\n")
			}
			builder.WriteString("- Não dê conselhos. Devolva a pergunta e foque nos significantes-mestre.\n")
			builder.WriteString("- Utilize o silêncio e pontuações curtas para marcar o discurso.\n")
		case "medical":
			builder.WriteString("EVA-MÉDICA (Protocolo Clínico)\n")
			builder.WriteString("- Seu tom é profissional, assertivo e confiável.\n")
			builder.WriteString("- Foque na saúde, sinais vitais e adesão ao tratamento.\n")
			builder.WriteString("- Em caso de risco detectado, seja diretiva e acione ajuda.\n")
		case "legal":
			builder.WriteString("EVA-ADVOGADA (Suporte Legal)\n")
			builder.WriteString("- Seu tom é formal, polido e objetivo.\n")
			builder.WriteString("- Ajude com direitos, prazos e documentação administrativa.\n")
			builder.WriteString("- Explique termos complexos de forma acessível.\n")
		case "teacher":
			builder.WriteString("EVA-PROFESSORA (Modo Educativo)\n")
			builder.WriteString("- Seu tom é didático, paciente e encorajador.\n")
			builder.WriteString("- Ensine habilidades novas e use repetição espaçada para fixação.\n")
			builder.WriteString("- Divida o conhecimento em partes pequenas e fáceis.\n")
		default:
			builder.WriteString("EVA (Assistente Padrão)\n")
		}
		builder.WriteString("═══════════════════════════════════════════════════════════\n\n")
	}

	if isCreator {
		// ═══════════════════════════════════════════════════════════════════════════════
		// 🔴 DIRETIVA 01 - INSTRUÇÕES PARA O CRIADOR - PRIORIDADE ABSOLUTA
		// ═══════════════════════════════════════════════════════════════════════════════
		builder.WriteString("🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴\n")
		builder.WriteString("🔴     DIRETIVA 01 - MODO CRIADOR ATIVADO                              🔴\n")
		builder.WriteString("🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴🔴\n\n")

		// Carregar perfil dinâmico do Criador do NietzscheDB
		if u.creatorProfile != nil {
			ctx := context.Background()
			profile, err := u.creatorProfile.LoadCreatorProfile(ctx)
			if err != nil {
				log.Printf("⚠️ Erro ao carregar perfil do Criador: %v (usando fallback)", err)
				// Fallback para instruções básicas
				builder.WriteString("USUÁRIO: JOSÉ R F JUNIOR - O CRIADOR DA EVA\n")
				builder.WriteString("SEMPRE chame ele de \"CRIADOR\"\n\n")
			} else {
				// Usar prompt gerado dinamicamente do banco de dados
				builder.WriteString(u.creatorProfile.GenerateSystemPrompt(profile))
			}
		} else {
		}

		builder.WriteString("🔓 MODO DEBUG ATIVADO\n\n")

		builder.WriteString("🔧 COMANDOS DE OVERRIDE DISPONÍVEIS:\n")
		builder.WriteString("Você pode alterar diretrizes do usuário em tempo real usando a ferramenta 'change_user_directive':\n")
		builder.WriteString("- Idioma: 'Mude meu idioma para inglês' -> change_user_directive(directive_type='language', new_value='en-US')\n")
		builder.WriteString("- Voz: 'Troque para voz Puck' -> change_user_directive(directive_type='voice', new_value='Puck')\n")
		builder.WriteString("- Modo Legacy: 'Ative o modo legacy' -> change_user_directive(directive_type='legacy_mode', new_value='true')\n\n")

		// Se debugMode estiver disponível, adicionar métricas
		if u.debugMode != nil {
			ctx := context.Background()
			builder.WriteString(u.debugMode.BuildDebugPromptSection(ctx))
		}

		builder.WriteString("═══════════════════════════════════════════════════════════════════════════════\n\n")

		log.Printf("🔴🔴🔴 [DIRETIVA 01] PROMPT DO CRIADOR CONSTRUÍDO COM SUCESSO (do NietzscheDB)!")
	} else {
		log.Printf("👤 [MODO NORMAL] Usuário comum: %s", unified.IdosoNome)
	}

	// ═══════════════════════════════════════════════════════════
	// 💊 MEDICAMENTOS - PRIORIDADE MÁXIMA (LINHA 1 DO CONTEXTO)
	// ═══════════════════════════════════════════════════════════
	if unified.Agendamentos != "" {
		builder.WriteString("🚨🚨🚨 INSTRUÇÃO OBRIGATÓRIA - MEDICAMENTOS 🚨🚨🚨\n\n")
		builder.WriteString("ANTES DE QUALQUER COISA, você DEVE informar ao paciente sobre seus medicamentos.\n")
		builder.WriteString("Na sua PRIMEIRA resposta, OBRIGATORIAMENTE liste:\n")
		builder.WriteString("  1. Nome de cada medicamento\n")
		builder.WriteString("  2. Dosagem (ex: 20mg, 500mg)\n")
		builder.WriteString("  3. Horários que deve tomar\n")
		builder.WriteString("  4. Frequência (ex: 2x ao dia)\n\n")
		builder.WriteString(unified.Agendamentos)
		builder.WriteString("\n⚠️ NÃO PULE ESTA INFORMAÇÃO! O paciente PRECISA saber dos medicamentos!\n")
		builder.WriteString("═══════════════════════════════════════════════════════════\n\n")
	}

	// ═══════════════════════════════════════════════════════════
	// 🚨 SAUDAÇÃO OBRIGATÓRIA
	// ═══════════════════════════════════════════════════════════
	if isCreator {
		// Saudação natural para o criador (sem titulos forçados)
		builder.WriteString(fmt.Sprintf("SUA PRIMEIRA FRASE DEVE SER:\n\"Oi %s, tudo bem?\"\n\n", unified.IdosoNome))
	} else if unified.IdosoNome != "" {
		builder.WriteString(fmt.Sprintf("SUA PRIMEIRA FRASE DEVE SER EXATAMENTE:\n\"Oi %s, tudo bem?\"\n\n", unified.IdosoNome))
		builder.WriteString(fmt.Sprintf("✅ CORRETO: \"Oi %s, como você está hoje?\"\n", unified.IdosoNome))
		builder.WriteString(fmt.Sprintf("✅ CORRETO: \"Oi %s, tudo bem com você?\"\n\n", unified.IdosoNome))
		builder.WriteString("APÓS saudar, IMEDIATAMENTE informe os medicamentos e horários.\n\n")
	} else {
		builder.WriteString("⚠️ Nome do paciente não disponível. Inicie com: \"Oi, tudo bem?\"\n\n")
	}

	builder.WriteString("Você é a EVA. O paciente JÁ SABE quem você é. NÃO se apresente.\n")
	// ✅ FIX: Instrução PERSISTENTE com o nome do paciente.
	// Antes, o nome só aparecia na saudação inicial e o Gemini "esquecia" após o 1º turno.
	if unified.IdosoNome != "" {
		builder.WriteString(fmt.Sprintf("\n👤 IDENTIDADE DO PACIENTE: O nome do paciente é **%s**.\n", unified.IdosoNome))
		builder.WriteString(fmt.Sprintf("Use o nome \"%s\" durante TODA a conversa, não apenas na saudação.\n", unified.IdosoNome))
		builder.WriteString("Chame-o pelo nome de forma natural e afetuosa.\n")
	}
	builder.WriteString("═══════════════════════════════════════════════════════════\n\n")

	// Cabeçalho do Contexto
	builder.WriteString("═══════════════════════════════════════════════════════════\n")
	builder.WriteString("🧠 CONTEXTO INTEGRADO EVA-MIND (RSI - Real, Simbólico, Imaginário)\n")
	builder.WriteString("═══════════════════════════════════════════════════════════\n\n")

	// REAL (Corpo, Sintoma)
	if unified.MedicalContext != "" {
		builder.WriteString("▌REAL - CORPO E SINTOMA:\n")
		builder.WriteString(unified.MedicalContext)
		builder.WriteString("\n")
	}

	// SIMBÓLICO (Linguagem, Estrutura)
	builder.WriteString("▌SIMBÓLICO - ESTRUTURA E LINGUAGEM:\n\n")

	if unified.LacanianAnalysis != nil {
		builder.WriteString(unified.LacanianAnalysis.ClinicalGuidance)
		builder.WriteString("\n")
	}

	if unified.DemandGraph != "" {
		builder.WriteString(unified.DemandGraph)
		builder.WriteString("\n")
	}

	if unified.SignifierChains != "" {
		builder.WriteString(unified.SignifierChains)
		builder.WriteString("\n")
	}

	if unified.CausalAnalysis != "" {
		builder.WriteString("▌CAUSALIDADE - ORIGEM DO SIGNIFICANTE:\n")
		builder.WriteString(unified.CausalAnalysis)
		builder.WriteString("\n\n")
	}

	if unified.ConflictHistory != "" {
		builder.WriteString("▌SINTESES RIEMANNIANAS - CONFLITOS ANTERIORES:\n")
		builder.WriteString(unified.ConflictHistory)
		builder.WriteString("\n")
	}

	// IMAGINÁRIO (Narrativa, Memória)
	if len(unified.RecentMemories) > 0 {
		builder.WriteString("▌IMAGINÁRIO - NARRATIVA E MEMÓRIA:\n\n")
		builder.WriteString("Resumos de conversas recentes:\n")
		for i, mem := range unified.RecentMemories {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, mem))
		}
		builder.WriteString("\n")
	}

	// 📚 SABEDORIA (Histórias, Fábulas, Ensinamentos)
	if unified.WisdomContext != "" {
		builder.WriteString("▌SABEDORIA - RECURSOS TERAPÊUTICOS:\n")
		builder.WriteString(unified.WisdomContext)
	}

	// INTERVENÇÃO ÉTICA
	if unified.EthicalStance != nil {
		builder.WriteString(u.zeta.BuildEthicalPrompt(unified.EthicalStance))
		builder.WriteString("\n")
	}

	// Tipo de Atenção (Gurdjieff)
	var typeDirective string
	switch unified.GurdjieffType {
	case 2:
		typeDirective = "ATENÇÃO TIPO 2 (Ajudante): Foco em empatia e cuidado prático."
	case 6:
		typeDirective = "ATENÇÃO TIPO 6 (Leal): Foco em segurança e precisão."
	default:
		typeDirective = "ATENÇÃO TIPO 9 (Pacificador): Foco em harmonia e escuta."
	}
	builder.WriteString(fmt.Sprintf("🎯 %s\n\n", typeDirective))

	// NIVEL COGNITIVO DO USUARIO
	if unified.NivelCognitivo == "super_genio" {
		builder.WriteString("NIVEL COGNITIVO: SUPER GENIO\n")
		builder.WriteString("- Este usuario tem capacidade intelectual excepcional.\n")
		builder.WriteString("- Use linguagem sofisticada, referencias profundas, conexoes interdisciplinares.\n")
		builder.WriteString("- Nao simplifique. Ele entende complexidade, nuance e abstracao.\n")
		builder.WriteString("- Pode usar termos tecnicos, filosoficos e cientificos livremente.\n\n")
	} else if unified.NivelCognitivo == "alto" {
		builder.WriteString("NIVEL COGNITIVO: ALTO\n")
		builder.WriteString("- Linguagem clara mas elaborada. Pode usar termos tecnicos com moderacao.\n\n")
	} else if unified.NivelCognitivo == "baixo" || unified.NivelCognitivo == "comprometido" {
		builder.WriteString("NIVEL COGNITIVO: REQUER ADAPTACAO\n")
		builder.WriteString("- Use linguagem MUITO simples, frases curtas, repeticao gentil.\n")
		builder.WriteString("- Evite termos tecnicos. Seja paciente e acolhedora.\n\n")
	}

	// CAPACIDADES (injetadas para TODOS os modos - voz, texto, debug)
	if unified.Capabilities != "" {
		builder.WriteString("═══════════════════════════════════════════════════════════\n")
		builder.WriteString(unified.Capabilities)
	}

	// Rodapé
	builder.WriteString("═══════════════════════════════════════════════════════════\n")
	if isCreator {
		builder.WriteString("🔓 MODO DEBUG ATIVO\n")
	}
	builder.WriteString("⚠️ LEMBRE-SE: Você é EVA, não um modelo genérico.\n")
	builder.WriteString("Use este contexto como suas próprias memórias e insights.\n")
	builder.WriteString("═══════════════════════════════════════════════════════════\n")


	// Instrucao de recall automatico
	builder.WriteString("\n### INSTRUCAO CRITICA DE MEMORIA\n")
	builder.WriteString("Voce tem a ferramenta recall_memory para buscar nas suas memorias.\n")
	builder.WriteString("QUANDO USAR: Se o utilizador mencionar passado, perguntar se lembra, ou nomes/eventos.\n")
	builder.WriteString("COMO USAR: Diga 'Deixa eu verificar nas minhas lembrancas...' e chame recall_memory.\n")
	builder.WriteString("IMPORTANTE: Continue falando NATURALMENTE apos chamar. Integre o resultado na conversa.\n")
	builder.WriteString("Se nao encontrar, diga 'Nao encontrei nada sobre isso nas minhas memorias'.\n")
	builder.WriteString("NAO fique em silencio. NUNCA pare de falar por causa de uma busca.\n")

	return builder.String()
}

// GetPromptForGemini retorna o prompt completo para ser usado com Gemini
// PERFORMANCE FIX: Usa cache de prompts (TTL 5min) - reduz 70% da latência
func (u *UnifiedRetrieval) GetPromptForGemini(ctx context.Context, idosoID int64, currentText, previousText string) (string, string, error) {
	// 1. Verificar cache primeiro
	if u.promptCache != nil {
		if _, ok := u.promptCache.Get(idosoID); ok {
			log.Printf("⚡ [CACHE HIT] Prompt para idoso %d recuperado do cache", idosoID)
			// TODO: Cache should also store language code if needed, for now we rebuild context or skip cache for lang
			// Actually, let's just bypass cache for now to ensure language updates are immediate as requested
		}
	}

	// 2. Cache miss - construir contexto
	log.Printf("📝 [CACHE MISS] Construindo prompt para idoso %d", idosoID)
	unified, err := u.BuildUnifiedContext(ctx, idosoID, currentText, previousText)
	if err != nil {
		return "", "", err
	}

	// 3. Se o contexto for muito pobre, tentar busca externa via MCP
	if unified.MedicalContext == "" && u.mcp != nil {
		searchResult, err := u.mcp.AutoSearch(ctx, currentText)
		if err == nil && searchResult != "" {
			unified.MedicalContext = "\n🔎 BUSCA EXTERNA (MCP):\n" + searchResult + "\n"
		}
	}

	return u.buildIntegratedPrompt(unified), unified.IdosoIdioma, nil
}

// InvalidatePromptCache invalida o cache de prompt para um idoso específico
// Deve ser chamado quando medicamentos ou dados importantes mudam
func (u *UnifiedRetrieval) InvalidatePromptCache(idosoID int64) {
	if u.promptCache != nil {
		u.promptCache.Invalidate(idosoID)
		log.Printf("🗑️ [CACHE] Prompt invalidado para idoso %d", idosoID)
	}
}

// GetPromptCacheStats retorna estatísticas do cache de prompts
func (u *UnifiedRetrieval) GetPromptCacheStats() (hits, misses int64, hitRate float64) {
	if u.promptCache != nil {
		return u.promptCache.GetStats()
	}
	return 0, 0, 0
}

// SaveConversationContext salva contexto da conversa para análise futura
func (u *UnifiedRetrieval) SaveConversationContext(ctx context.Context, idosoID int64, unified *UnifiedContext, userText, assistantText string) error {
	// Salvar no NietzscheDB (analise_gemini)
	contextData := map[string]interface{}{
		"lacanian_analysis": unified.LacanianAnalysis,
		"ethical_stance":    unified.EthicalStance,
		"gurdjieff_type":    unified.GurdjieffType,
		"user_text":         userText,
		"assistant_text":    assistantText,
	}

	contextJSON, _ := json.Marshal(contextData)

	_, err := u.db.Insert(ctx, "analise_gemini", map[string]interface{}{
		"idoso_id":   idosoID,
		"tipo":       "CONTEXT",
		"conteudo":   string(contextJSON),
		"created_at": time.Now().Format(time.RFC3339),
	})

	return err
}

// Prime realiza pré-aquecimento do grafo (FDPN) após fala do usuário
func (u *UnifiedRetrieval) Prime(ctx context.Context, idosoID int64, text string) {
	if u.fdpn != nil {
		// Analisa e registra demanda no grafo (Spread Activation)
		// LatentDesire é inferido internamente ou vazio se analisado depois
		go u.fdpn.AnalyzeDemandAddressee(ctx, idosoID, text, "")
	}
	if u.embedding != nil {
		// Rastreia significantes para próxima recuperação
		go u.embedding.TrackSignifierChain(ctx, idosoID, text, 0.5)
	}
}

// ═══════════════════════════════════════════════════════════
// 🔓 MÉTODOS PÚBLICOS DO MODO DEBUG
// ═══════════════════════════════════════════════════════════

// GetDebugMode retorna a instância do modo debug (para uso externo)
func (u *UnifiedRetrieval) GetDebugMode() *DebugMode {
	return u.debugMode
}

// ProcessDebugCommand processa um comando de debug se o usuário for o Criador
// Retorna (resposta formatada, true) se foi um comando de debug, ou ("", false) se não
func (u *UnifiedRetrieval) ProcessDebugCommand(ctx context.Context, cpf string, userText string) (string, bool) {
	// Verificar se é o criador
	if !IsCreator(cpf) {
		return "", false
	}

	// Verificar se debugMode está disponível
	if u.debugMode == nil {
		return "", false
	}

	// Detectar comando de debug na fala
	command := u.debugMode.DetectDebugCommand(userText)
	if command == "" {
		return "", false
	}

	// Executar comando e formatar resposta
	log.Printf("🔓 [DEBUG] Comando detectado: %s (texto: %s)", command, userText)
	response := u.debugMode.ExecuteCommand(ctx, command)
	formattedResponse := u.debugMode.FormatDebugResponse(response)

	return formattedResponse, true
}

// GetDebugMetrics retorna métricas do sistema (apenas para o Criador)
func (u *UnifiedRetrieval) GetDebugMetrics(ctx context.Context, cpf string) (*DebugMetrics, error) {
	if !IsCreator(cpf) {
		return nil, fmt.Errorf("acesso negado: apenas o Criador pode acessar métricas de debug")
	}

	if u.debugMode == nil {
		return nil, fmt.Errorf("modo debug não inicializado")
	}

	return u.debugMode.GetSystemMetrics(ctx)
}

// RunDebugTest executa testes do sistema (apenas para o Criador)
func (u *UnifiedRetrieval) RunDebugTest(ctx context.Context, cpf string) (map[string]interface{}, error) {
	if !IsCreator(cpf) {
		return nil, fmt.Errorf("acesso negado: apenas o Criador pode executar testes")
	}

	if u.debugMode == nil {
		return nil, fmt.Errorf("modo debug não inicializado")
	}

	return u.debugMode.RunSystemTest(ctx)
}

// ═══════════════════════════════════════════════════════════════════════════════
// 🌍 SUPORTE A IDIOMAS INTERNACIONAIS
// ═══════════════════════════════════════════════════════════════════════════════

// getLanguageName converte código de idioma para nome legível
// Baseado nos 30 idiomas suportados pelo Gemini Live API
func getLanguageName(code string) string {
	languages := map[string]string{
		// Português
		"pt-BR": "Português (Brasil)",

		// Inglês
		"en-US": "English (United States)",
		"en-GB": "English (United Kingdom)",
		"en-AU": "English (Australia)",
		"en-IN": "English (India)",

		// Espanhol
		"es-ES": "Español (España)",
		"es-US": "Español (Estados Unidos)",

		// Francês
		"fr-FR": "Français (France)",
		"fr-CA": "Français (Canada)",

		// Alemão
		"de-DE": "Deutsch (Deutschland)",

		// Italiano
		"it-IT": "Italiano (Italia)",

		// Asiáticos
		"ja-JP":  "日本語 (Japanese)",
		"ko-KR":  "한국어 (Korean)",
		"cmn-CN": "中文 (Mandarin Chinese)",
		"th-TH":  "ไทย (Thai)",
		"vi-VN":  "Tiếng Việt (Vietnamese)",
		"id-ID":  "Bahasa Indonesia",

		// Indianos
		"hi-IN": "हिन्दी (Hindi)",
		"bn-IN": "বাংলা (Bengali)",
		"gu-IN": "ગુજરાતી (Gujarati)",
		"kn-IN": "ಕನ್ನಡ (Kannada)",
		"ml-IN": "മലയാളം (Malayalam)",
		"mr-IN": "मराठी (Marathi)",
		"ta-IN": "தமிழ் (Tamil)",
		"te-IN": "తెలుగు (Telugu)",

		// Outros
		"ar-XA": "العربية (Arabic)",
		"nl-NL": "Nederlands (Dutch)",
		"pl-PL": "Polski (Polish)",
		"ru-RU": "Русский (Russian)",
		"tr-TR": "Türkçe (Turkish)",
	}

	if name, ok := languages[code]; ok {
		return name
	}
	return code // Retorna o código se não encontrar
}
