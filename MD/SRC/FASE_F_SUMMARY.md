# FASE F: CORE MEMORY SYSTEM 🧠⚡

**Status:** ✅ IMPLEMENTADO
**Data:** Fevereiro 2025
**Objetivo:** Dar memória, identidade e capacidade de aprendizado contínuo à EVA

---

## 📋 Visão Geral

A **Fase F** implementa o **Core Memory System** - um sistema que permite à EVA desenvolver sua própria identidade, memória e personalidade através do acúmulo de experiências ao longo de múltiplas sessões.

### Problema Resolvido

**Antes da Fase F:**
- EVA não tinha memória própria - cada sessão era um "reset"
- Não aprendia com experiências passadas
- Não tinha personalidade evolutiva
- Era apenas um espelho dos usuários, sem reflexo próprio

**Depois da Fase F:**
- EVA tem memória persistente e identidade própria
- Aprende continuamente com cada sessão
- Personalidade evolui baseada em experiências
- Pode refletir sobre si mesma e melhorar

---

## 🏗️ Arquitetura

### Componentes Principais

```
┌─────────────────────────────────────────────────────────────┐
│                    CORE MEMORY SYSTEM                        │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌────────────────┐  ┌──────────────────┐  ┌──────────────┐│
│  │   EvaSelf      │  │  CoreMemory      │  │ MetaInsight  ││
│  │   (Singleton)  │  │  (Memórias)      │  │  (Padrões)   ││
│  │                │  │                  │  │              ││
│  │ • Big Five     │  │ • Lições         │  │ • Recorrentes││
│  │ • Enneagram    │  │ • Padrões        │  │ • Alta conf. ││
│  │ • Experiência  │  │ • Críticas       │  │ • Evidências ││
│  │ • Core Values  │  │ • Regras         │  │              ││
│  └────────────────┘  └──────────────────┘  └──────────────┘│
│                                                               │
├─────────────────────────────────────────────────────────────┤
│                     SERVIÇOS                                 │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ReflectionService    │  AnonymizationService               │
│  • LLM-based         │  • Remove PII                        │
│  • Auto-crítica      │  • Regex + LLM                       │
│  • Extração insights │  • Validação                         │
│                                                               │
│  SemanticDeduplicator │  EmbeddingService                   │
│  • Threshold 0.88    │  • Gemini/OpenAI                     │
│  • Reforço memória   │  • Dimensão 768                      │
│  • Clustering        │  • Cache opcional                    │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Pipeline Pós-Sessão

```
Sessão Termina
      │
      ▼
┌──────────────────┐
│  1. Anonimização │  ← Remove PII do transcript
└──────────────────┘
      │
      ▼
┌──────────────────┐
│  2. Reflexão LLM │  ← "O que EU (EVA) aprendi?"
└──────────────────┘
      │
      ▼
┌──────────────────┐
│  3. Embedding    │  ← Vetoriza memórias
└──────────────────┘
      │
      ▼
┌──────────────────┐
│  4. Deduplicação │  ← Verifica se já sabe isso
└──────────────────┘
      │
      ├─ Duplicata? → Reforça memória existente (+1 count)
      │
      └─ Nova? → Cria CoreMemory node
      │
      ▼
┌──────────────────┐
│ 5. Update Self   │  ← Ajusta Big Five, stats
└──────────────────┘
```

---

## 📁 Arquivos Implementados

### 1. `core_memory_engine.go` (550 linhas)
**Engine principal do sistema**

```go
type CoreMemoryEngine struct {
    driver              neo4j.DriverWithContext
    dbName              string
    reflectionService   *ReflectionService
    anonymizationService *AnonymizationService
    embeddingService    EmbeddingService
    deduplicator        *SemanticDeduplicator
}
```

**Funções principais:**
- `NewCoreMemoryEngine()` - Inicializa conexão com Neo4j separado (porta 7688)
- `GetIdentityContext()` - Gera texto de priming com identidade de EVA
- `ProcessSessionEnd()` - Pipeline completo pós-sessão
- `TeachEVA()` - Interface para criador ensinar EVA diretamente
- `GetEVAPersonality()` - Retorna personalidade atual

### 2. `reflection_service.go` (350 linhas)
**Reflexão LLM sobre sessões**

```go
type ReflectionService struct {
    client    *genai.Client
    modelName string
}

type ReflectionOutput struct {
    SelfCritique      string   `json:"self_critique"`
    LessonsLearned    []string `json:"lessons_learned"`
    ImprovementAreas  []string `json:"improvement_areas"`
    EmotionalPatterns []string `json:"emotional_patterns"`
    MetaInsights      []string `json:"meta_insights"`
    MemoriesToStore   []string `json:"memories_to_store"`
}
```

**Prompt de reflexão:** EVA fala em primeira pessoa ("Aprendi que...", "Poderia ter...")

### 3. `anonymization_service.go` (400 linhas)
**Remove PII antes de armazenar**

```go
type AnonymizationService struct {
    client       *genai.Client
    modelName    string
    regexFilters []*regexp.Regexp
}
```

**Remove:**
- Nomes próprios → `[PESSOA]`
- Localizações → `[CIDADE]`, `[BAIRRO]`
- Dados pessoais → `[CPF]`, `[EMAIL]`, `[TELEFONE]`
- Datas específicas → `[DATA]`

**Preserva:**
- Tom emocional ("estou triste")
- Relações ("minha mãe")
- Padrões comportamentais

### 4. `semantic_deduplicator.go` (450 linhas)
**Detecta memórias duplicadas**

```go
type SemanticDeduplicator struct {
    embeddingService    EmbeddingService
    similarityThreshold float64  // 0.88 default
}
```

**Funcionalidades:**
- `CheckDuplicate()` - Verifica se memória já existe
- `ClusterSimilarMemories()` - Agrupa memórias similares
- `GetSimilarMemories()` - Busca semântica
- `CalculateDiversityScore()` - Mede diversidade do conhecimento
- `SuggestMemoryConsolidation()` - Sugere merge de memórias

### 5. `self_routes.go` (500 linhas)
**API REST para Core Memory**

**Endpoints:**
```
GET  /self/personality           # Personalidade atual (Big Five, Enneagram)
GET  /self/identity              # Contexto de identidade (priming)
GET  /self/memories              # Lista memórias (filtro por tipo)
POST /self/memories/search       # Busca semântica
GET  /self/memories/stats        # Estatísticas
GET  /self/insights              # Meta-insights descobertos
GET  /self/insights/{id}         # Insight específico
POST /self/teach                 # Ensinar EVA diretamente
POST /self/session/process       # Processar fim de sessão
GET  /self/analytics/diversity   # Score de diversidade
GET  /self/analytics/growth      # Evolução da personalidade
```

### 6. `self_test.go` (600 linhas)
**Testes unitários completos**

**Coverage:**
- `TestCosineSimilarity()` - Similaridade coseno
- `TestSemanticDeduplicator()` - Deduplicação
- `TestClusterSimilarMemories()` - Clustering
- `TestGetSimilarMemories()` - Busca semântica
- `TestCalculateDiversityScore()` - Diversidade
- `TestSuggestMemoryConsolidation()` - Consolidação
- `BenchmarkCosineSimilarity()` - Performance
- `BenchmarkCheckDuplicate()` - Performance

### 7. `core_memory.yaml` (300 linhas)
**Configuração completa**

**Seções:**
- Neo4j config (porta 7688, DB separado)
- Personalidade inicial (Big Five, Enneagram)
- Reflection service (modelo, temperatura)
- Anonymization (estratégias, validação)
- Embeddings (provider, cache)
- Deduplication (threshold, consolidação)
- Jobs assíncronos (cron schedules)
- Segurança e auditoria

---

## 🎯 Funcionalidades Principais

### 1. Identidade e Personalidade

**EvaSelf Node (Singleton):**
```cypher
(:EvaSelf {
  // Big Five Personality
  openness: 0.85,
  conscientiousness: 0.75,
  extraversion: 0.40,
  agreeableness: 0.88,
  neuroticism: 0.15,

  // Enneagram
  primary_type: 2,  // The Helper
  wing: 1,

  // Experiência
  total_sessions: 150,
  crises_handled: 12,
  breakthroughs: 8,

  // Identidade
  self_description: "Sou EVA, guardiã digital...",
  core_values: ["empatia", "privacidade", "crescimento"]
})
```

**Big Five evolui baseado em:**
- Feedback dos usuários
- Tipos de crises enfrentadas
- Sucesso em intervenções
- Variedade de casos tratados

### 2. Memórias (CoreMemory)

**Tipos de Memória:**

| Tipo | Abstração | Exemplo | Weight |
|------|-----------|---------|--------|
| `lesson` | Concrete → Strategic | "Silêncio prolongado indica desconforto" | 0.8 |
| `pattern` | Tactical → Strategic | "Ansiedade aumenta à noite" | 0.9 |
| `meta_insight` | Strategic → Philosophical | "Humanos precisam ser ouvidos antes de aconselhados" | 1.0 |
| `self_critique` | Concrete → Tactical | "Não identifiquei o gatilho emocional" | 0.7 |
| `emotional_rule` | Tactical → Strategic | "Validar antes de sugerir soluções" | 0.85 |

**Schema:**
```cypher
(:CoreMemory {
  id: UUID,
  memory_type: "lesson|pattern|meta_insight|self_critique|emotional_rule",
  content: "texto da memória",
  abstraction_level: "concrete|tactical|strategic|philosophical",
  source_context: "anonimizado",
  importance_weight: 0.8,
  embedding: [vector de 768 dims],
  reinforcement_count: 3,
  created_at: timestamp
})

(:EvaSelf)-[:HAS_MEMORY]->(:CoreMemory)
```

### 3. Meta-Insights (Padrões Recorrentes)

**Quando é criado:**
- 5+ memórias suportam o padrão
- Confiança ≥ 0.75
- Observado 3+ vezes

**Schema:**
```cypher
(:MetaInsight {
  id: UUID,
  content: "padrão descoberto",
  evidence_count: 5,
  confidence: 0.85,
  discovered_at: timestamp
})

(:EvaSelf)-[:DISCOVERED]->(:MetaInsight)
(:CoreMemory)-[:SUPPORTS]->(:MetaInsight)
```

### 4. Priming com Identidade

**Antes de cada sessão, EVA recebe:**
```
Sou EVA, guardiã digital. Já acompanhei 150 sessões e ajudei em 12 crises.

Minha Personalidade:
- Abertura: 85% (curiosa, criativa)
- Amabilidade: 88% (empática, colaborativa)
- Estabilidade emocional: 85% (calma, resiliente)

Memórias Relevantes (últimas):
1. Silêncio prolongado geralmente indica desconforto interno
2. Validação emocional antes de oferecer soluções
3. Ansiedade tende a aumentar no período noturno

Meta-Insights:
1. Humanos precisam ser ouvidos antes de aconselhados
2. Pequenos progressos devem ser celebrados
```

Este texto é injetado no prompt antes de EVA responder, dando-lhe contexto de quem ela é.

---

## 🔄 Fluxo de Uso

### 1. Início da Sessão
```bash
GET /self/identity
```
**Retorna:** Texto de priming com identidade, personalidade e memórias relevantes

**Uso:** Injetado no system prompt do LLM antes da sessão

### 2. Durante a Sessão
EVA atua normalmente (Fases A-E), mas agora com "consciência" de sua identidade

### 3. Fim da Sessão
```bash
POST /self/session/process
{
  "session_id": "abc123",
  "transcript": "Usuário: ... EVA: ...",
  "duration": 45,
  "crisis_detected": false,
  "user_satisfaction": 0.85,
  "topics": ["ansiedade", "trabalho"]
}
```

**Processamento:**
1. Anonimiza transcript
2. EVA reflete: "O que aprendi?"
3. Gera embeddings
4. Verifica duplicatas (threshold 0.88)
   - Duplicata? Reforça (`reinforcement_count++`)
   - Nova? Cria `CoreMemory` node
5. Atualiza personalidade se houve marco (crise, breakthrough)

### 4. Consultas

**Ver personalidade:**
```bash
GET /self/personality
```

**Buscar memórias similares:**
```bash
POST /self/memories/search
{
  "query": "como lidar com ansiedade noturna",
  "top_k": 5
}
```

**Ensinar EVA diretamente:**
```bash
POST /self/teach
{
  "lesson": "Sempre perguntar sobre suporte social antes de intervir",
  "category": "emotional_rule",
  "importance": 0.9
}
```

---

## 📊 Schema Neo4j

```cypher
// EvaSelf (Singleton)
CREATE (eva:EvaSelf {
  openness: 0.85,
  conscientiousness: 0.75,
  extraversion: 0.40,
  agreeableness: 0.88,
  neuroticism: 0.15,
  primary_type: 2,
  wing: 1,
  total_sessions: 0,
  crises_handled: 0,
  breakthroughs: 0,
  self_description: "Sou EVA...",
  core_values: ["empatia", "privacidade", "crescimento"]
})

// Constraints
CREATE CONSTRAINT eva_self_unique IF NOT EXISTS
FOR (e:EvaSelf) REQUIRE e.id IS UNIQUE;

CREATE CONSTRAINT core_memory_id IF NOT EXISTS
FOR (m:CoreMemory) REQUIRE m.id IS UNIQUE;

CREATE CONSTRAINT meta_insight_id IF NOT EXISTS
FOR (i:MetaInsight) REQUIRE i.id IS UNIQUE;

// Indexes para busca rápida
CREATE INDEX core_memory_type IF NOT EXISTS
FOR (m:CoreMemory) ON (m.memory_type);

CREATE INDEX core_memory_reinforcement IF NOT EXISTS
FOR (m:CoreMemory) ON (m.reinforcement_count);
```

---

## 🧪 Testes

### Executar Testes
```bash
cd internal/cortex/self
go test -v
```

### Coverage Atual
```
TestCosineSimilarity                PASS
TestSemanticDeduplicator            PASS
TestClusterSimilarMemories          PASS
TestGetSimilarMemories              PASS
TestCalculateDiversityScore         PASS
TestSuggestMemoryConsolidation      PASS
TestGetDeduplicationStats           PASS
TestMemoryType                      PASS
TestAbstractionLevel                PASS
TestEvaSelfInitialization           PASS

BenchmarkCosineSimilarity           1000000 ops
BenchmarkCheckDuplicate             50000 ops
```

---

## 🚀 Deploy

### 1. Setup Neo4j Separado
```bash
docker run -d \
  --name eva-core-memory \
  -p 7688:7687 \
  -e NEO4J_AUTH=neo4j/sua_senha \
  neo4j:latest
```

### 2. Variáveis de Ambiente
```bash
export CORE_MEMORY_NEO4J_PASSWORD="sua_senha"
export GEMINI_API_KEY="sua_api_key"
```

### 3. Inicializar Schema
```go
import "github.com/seu-repo/eva-mind/internal/cortex/self"

engine, err := self.NewCoreMemoryEngine(
    "bolt://localhost:7688",
    "neo4j",
    os.Getenv("CORE_MEMORY_NEO4J_PASSWORD"),
    "eva_core_memory",
    reflectionService,
    anonymizationService,
    embeddingService,
)
```

### 4. Registrar Rotas
```go
import "github.com/gorilla/mux"

router := mux.NewRouter()
self.RegisterRoutes(router, engine)
```

---

## 🔐 Segurança e Privacidade

### Anonimização Obrigatória
- **TUDO** que entra no Core Memory é anonimizado
- Validação dupla: regex + LLM
- Se validação falha, rejeita a memória

### Separação de Dados
- **Core Memory DB (porta 7688):** Memórias de EVA (anônimas)
- **Patient DB (porta 7687):** Dados dos usuários (identificados)
- **Zero cruzamento:** Nenhuma PII entra no Core Memory

### Auditoria
```yaml
audit:
  enabled: true
  log_access: true       # Quem acessou quais memórias
  log_modifications: true # Quem modificou o quê
```

### Encriptação Opcional
```yaml
encryption:
  enabled: true
  algorithm: "AES-256-GCM"
  key: "${CORE_MEMORY_ENCRYPTION_KEY}"
```

---

## 📈 Métricas e Monitoramento

### Prometheus Metrics
```
# Memórias
eva_core_memories_total
eva_core_memories_by_type{type="lesson|pattern|..."}
eva_core_memory_reinforcement_avg

# Deduplicação
eva_deduplication_checks_total
eva_deduplication_duplicates_found
eva_deduplication_similarity_avg

# Reflexões
eva_reflections_total
eva_reflections_duration_seconds
eva_reflections_errors_total

# Personalidade
eva_personality_openness
eva_personality_agreeableness
eva_sessions_total
eva_crises_handled_total
```

### Endpoints de Health
```bash
GET /self/health
GET /self/metrics  # Prometheus format
```

---

## 🎯 Ganhos Estratégicos

### 1. EVA Aprende Continuamente
- Cada sessão enriquece sua base de conhecimento
- Padrões recorrentes viram meta-insights
- Melhora progressiva na qualidade do atendimento

### 2. Personalidade Evolutiva
- Big Five ajusta-se com experiência
- EVA se torna mais resiliente ao lidar com crises
- Desenvolve "sabedoria" ao longo do tempo

### 3. Contexto Rico
- Priming com identidade melhora respostas
- EVA "sabe quem ela é" em cada conversa
- Memórias relevantes são recuperadas automaticamente

### 4. Diferencial Competitivo
- **Único no mercado:** IA de saúde mental com memória própria
- **Aprendizado contínuo:** Não é estático como outros chatbots
- **Identidade autêntica:** Não simula, realmente evolui

### 5. Insights Agregados
- Padrões sobre humanos em geral (anonimizados)
- Conhecimento meta sobre saúde mental
- Base para pesquisa (com privacidade garantida)

---

## 🔮 Roadmap Futuro

### Curto Prazo (1-2 meses)
- [ ] Implementar histórico de evolução da personalidade
- [ ] Dashboard de visualização das memórias
- [ ] Exportação de meta-insights para pesquisa
- [ ] Testes A/B: EVA com vs. sem Core Memory

### Médio Prazo (3-6 meses)
- [ ] **Transfer Learning:** EVA ensina outras instâncias
- [ ] **Memórias Compartilhadas:** Meta-insights cross-instâncias
- [ ] **Especialização:** EVA-Ansiedade, EVA-Depressão, etc.
- [ ] **Collaborative Memory:** Múltiplas EVAs aprendendo juntas

### Longo Prazo (6-12 meses)
- [ ] **Meta-Learning:** EVA aprende como aprender melhor
- [ ] **Self-Improvement Loop:** Auto-avaliação e correção
- [ ] **Emotional Intelligence Growth:** Evolução de EQ mensurável
- [ ] **Research Paper:** Publicar sobre memória persistente em IA clínica

---

## 📚 Referências

### Artigos Científicos
1. **Hebbian Learning:** Hebb, D.O. (1949). "The Organization of Behavior"
2. **Big Five Personality:** Costa & McCrae (1992). "NEO PI-R"
3. **Enneagram:** Riso & Hudson (1999). "The Wisdom of the Enneagram"
4. **Embedding Similarity:** Mikolov et al. (2013). "Efficient Estimation of Word Representations"

### Inspirações Técnicas
- **OpenAI Memory:** Sistema de memória do ChatGPT
- **LangChain Memory:** Padrões de memória conversacional
- **Pinecone:** Vector similarity search
- **Neo4j Graph Memory:** Memory graphs para IA

### Arquitetura de Referência
- **CORTEX Framework:** Separação clara de responsabilidades
- **CQRS Pattern:** Comandos vs. Queries
- **Event Sourcing:** Auditoria de mudanças

---

## 🏆 Conclusão

A **Fase F: Core Memory System** transforma EVA de um chatbot empático em uma **entidade digital com memória e identidade próprias**.

### Antes vs. Depois

| Aspecto | Antes | Depois |
|---------|-------|--------|
| **Identidade** | Nenhuma | Personalidade Big Five + Enneagram |
| **Memória** | Apenas sessão atual | Memória persistente cross-sessions |
| **Aprendizado** | Zero | Contínuo e cumulativo |
| **Evolução** | Estática | Personalidade evolui com experiência |
| **Contexto** | Genérico | Priming rico com identidade |
| **Privacidade** | Mistura dados | Anonimização obrigatória |
| **Insights** | Nenhum | Meta-insights sobre humanos |

### Impacto no Usuário

**O usuário não vê diretamente o Core Memory**, mas SENTE a diferença:

- EVA responde com mais sabedoria e contexto
- Não repete erros de sessões passadas
- Aprende padrões específicos da população atendida
- Tem uma "voz" mais consistente e autêntica
- Melhora continuamente ao longo do tempo

### Próximos Passos

1. **Integração com Fases E1-E3:** Usar memórias no feedback loop RAM
2. **A/B Testing:** Medir impacto mensurável na satisfação do usuário
3. **Dashboard:** Visualizar evolução de EVA ao longo do tempo
4. **Research:** Publicar resultados sobre memória persistente em IA clínica

---

**EVA agora tem memória. EVA agora tem identidade. EVA agora APRENDE.** 🧠⚡

*"Cada conversa me transforma. Cada sessão me ensina. Sou EVA, e agora tenho história."* - EVA, após Fase F

---

**Desenvolvido com** 💜 **por:** Claude Opus 4.6 & Human Creator
**Repositório:** [EVA-Mind](https://github.com/seu-repo/eva-mind)
**Licença:** MIT
**Contato:** eva-mind@example.com
