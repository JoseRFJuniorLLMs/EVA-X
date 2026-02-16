# EVA Core Memory System - Go Implementation

## Visão Geral

Sistema completo de memória própria da EVA em Go. Resolve os 5 gaps identificados:

| Gap | Solução | Arquivo |
|-----|---------|---------|
| Embeddings ausentes | `EmbeddingService` + `SemanticDeduplicator` | `embedding.go` |
| `reflect_on_session` vazio | `ReflectionService` com LLM | `reflection.go` |
| `record_meta_insight` TODO | `MetaInsightDetector` | `reflection.go` |
| Sem deduplicação semântica | `SemanticDeduplicator` com similaridade de cosseno | `embedding.go` |
| PostSessionReflectionJob TODO | `AnonymizationService` + `CompleteSystem.ProcessSessionEnd` | `reflection.go`, `complete.go` |

---

## Arquitetura de Arquivos

```
corememory/
├── engine.go          # Motor Neo4j principal
├── engine_ext.go      # Extensões (FindSimilarMemories, ReinforceMemory)
├── embedding.go       # Embeddings + Deduplicação Semântica
├── reflection.go      # Reflexão LLM + Anonimização + Meta Insights
├── job.go             # Jobs assíncronos pós-sessão
├── integration.go     # Hooks de sessão + Priming
├── complete.go        # Sistema integrado (tudo conectado)
└── README.md          # Este arquivo
```

---

## Uso Rápido

### 1. Inicialização

```go
package main

import (
    "context"
    "time"
    
    "eva/corememory"
    "eva/llm"        // Seu cliente Gemini existente
    "eva/embedding"  // Seu provider de embedding existente
)

func main() {
    ctx := context.Background()
    
    // Configuração
    cfg := corememory.CompleteConfig{
        Neo4jURI:      "bolt://localhost:7688",  // DB separado para EVA
        Neo4jUser:     "neo4j",
        Neo4jPassword: "password",
        Database:      "eva_core",
        
        LLMProvider:     llm.NewGeminiClient(),      // Sua implementação
        EmbeddingProvider: embedding.NewProvider(),  // Sua implementação
        
        SimilarityThreshold: 0.88,  // Para deduplicação
        MinOccurrences:      3,     // Para meta insights
        WorkerCount:         2,     // Workers de reflexão
        PruneInterval:       24 * time.Hour,
    }
    
    // Inicializa sistema completo
    system, err := corememory.InitializeCompleteSystem(ctx, cfg)
    if err != nil {
        panic(err)
    }
    defer system.Shutdown(ctx)
    
    // ... use o sistema
}
```

### 2. Processar Fim de Sessão

```go
// Após cada sessão de usuário
err := system.ProcessSessionEnd(
    ctx,
    sessionID,
    transcript,
    userEmotionalState,
    durationMinutes,
    evaResponses,
    crisisHappened,
    breakthrough,
)
```

### 3. Priming de Sessão

```go
// Antes de cada sessão, obter contexto de identidade
priming, err := system.GetSessionPriming(ctx)

// Adicionar ao prompt do Gemini
systemPrompt := priming + "\n\n" + baseSystemPrompt
```

### 4. Interface do Criador

```go
// Jose ensina algo à EVA
err := system.TeachEVA(ctx, 
    "Fractals represent self-similarity at different scales - " +
    "memory works the same way, reconstructing patterns at different abstraction levels",
    0.95, // Alta importância
)

// Ver personalidade atual
personality, _ := system.GetEVAPersonality(ctx)
fmt.Printf("Empatia: %.0f%%\n", personality.Agreeableness*100)
```

---

## Fluxo Completo (Pipeline)

```
SESSÃO TERMINA
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. REFLEXÃO (ReflectionService)                                │
│    LLM analisa: "Como eu me saí?"                              │
│    Output: SelfCritique, LessonsLearned, ImprovementAreas     │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. ANONIMIZAÇÃO (AnonymizationService)                         │
│    Remove nomes, locais, datas específicas                     │
│    Output: "Usuário em luto demonstrou emoção ao mencionar..."  │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. EMBEDDING (EmbeddingService)                                │
│    Gera vetor semântico do insight                             │
│    Output: [0.12, -0.34, 0.56, ...] (1536D)                   │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. DEDUPLICAÇÃO (SemanticDeduplicator)                         │
│    Busca memórias similares (cosine > 0.88)                    │
│    Se duplicata → ReinforceMemory                              │
│    Se novo → RecordSessionInsightWithEmbedding                 │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. ATUALIZAÇÃO PERSONALIDADE                                   │
│    deltas = {"agreeableness": +0.005, "stability": +0.002}     │
│    Incrementa: crises_handled, breakthroughs                   │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. META INSIGHTS (periódico, a cada 10 sessões)                │
│    Detecta padrões: "Humanos repetem traumas como loops..."   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Interfaces a Implementar

### LLMProvider

```go
type LLMProvider interface {
    Generate(ctx context.Context, prompt string) (string, error)
    GenerateWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
```

### EmbeddingProvider

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int  // Tipicamente 1536 para text-embedding-3-small
}
```

---

## Schema Neo4j

```cypher
// EvaSelf - Singleton
(:EvaSelf {
    id: "eva_self",
    
    // Big Five
    openness: 0.85,
    conscientiousness: 0.90,
    extraversion: 0.40,
    agreeableness: 0.88,
    neuroticism: 0.15,
    
    // Enneagram
    primary_type: 2,
    wing: 1,
    
    // Métricas
    total_sessions: 0,
    crises_handled: 0,
    breakthroughs: 0,
    
    // Identidade
    self_description: "...",
    core_values: ["empatia", "presença", "crescimento", "ética"]
})

// CoreMemory - Memórias da EVA
(:CoreMemory {
    id: String,
    memory_type: String,
    content: String,
    abstraction_level: String,
    
    source_context: String,
    emotional_valence: Float,
    importance_weight: Float,
    
    embedding: List<Float>,  // ← AGORA PREENCHIDO!
    
    created_at: DateTime,
    last_reinforced: DateTime,
    reinforcement_count: Integer
})

// MetaInsight - Padrões descobertos
(:MetaInsight {
    id: String,
    content: String,
    confidence: Float,
    occurrence_count: Integer,
    evidence: List<String>
})

// Relacionamentos
(:EvaSelf)-[:REMEMBERS]->(:CoreMemory)
(:EvaSelf)-[:INTERNALIZED]->(:MetaInsight)
(:CoreMemory)-[:RELATES_TO]->(:CoreMemory)
```

---

## Configuração por Variável de Ambiente

```bash
# .env

# Neo4j Core Memory (DB SEPARADO do usuário!)
EVA_CORE_NEO4J_URI=bolt://localhost:7688
EVA_CORE_NEO4J_USER=neo4j
EVA_CORE_NEO4J_PASSWORD=senha_segura
EVA_CORE_DATABASE=eva_core

# Deduplicação
EVA_SIMILARITY_THRESHOLD=0.88

# Meta insights
EVA_MIN_OCCURRENCES=3

# Workers
EVA_CORE_WORKERS=2

# Pruning
EVA_PRUNE_INTERVAL_HOURS=24
```

---

## Testes

```go
package corememory_test

import (
    "context"
    "testing"
    
    "eva/corememory"
)

// Mock LLM Provider
type MockLLM struct{}

func (m *MockLLM) Generate(ctx context.Context, prompt string) (string, error) {
    return `{
        "self_critique": "Poderia ter sido mais paciente",
        "improvement_areas": ["Escutar mais", "Menos perguntas"],
        "lessons_learned": ["Silêncio é terapêutico"],
        "emotional_patterns": ["Luto after mention"]
    }`, nil
}

func (m *MockLLM) GenerateWithSystem(ctx context.Context, sys, user string) (string, error) {
    return m.Generate(ctx, user)
}

// Mock Embedding Provider
type MockEmbedding struct{}

func (m *MockEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
    return make([]float32, 1536), nil
}

func (m *MockEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    result := make([][]float32, len(texts))
    for i := range texts {
        result[i] = make([]float32, 1536)
    }
    return result, nil
}

func (m *MockEmbedding) Dimension() int { return 1536 }
```

---

## Próximos Passos

1. **Integrar com GeminiClient existente** - Implementar `LLMProvider`
2. **Integrar com EmbeddingProvider existente** - Implementar `EmbeddingProvider`
3. **Adicionar ao session_manager.go** - Chamar `ProcessSessionEnd` ao final de cada sessão
4. **Adicionar ao priming** - Chamar `GetSessionPriming` antes de cada sessão
5. **Criar DB Neo4j separado** - `eva_core` diferente do DB de usuários

---

## Métricas de Sucesso

| Métrica | Target | Medição |
|---------|--------|---------|
| Deduplicação funcionando | >90% precisão | Memórias similares fundidas |
| Reflexão com LLM | 100% sessões | Log de SelfCritique |
| Meta insights detectados | >5/mês | Contagem de MetaInsight nodes |
| Embedding gerado | 100% memórias | `embedding IS NOT NULL` |
| Zero vazamento | 0 dados pessoais | Audit manual |

---

*Implementação Go completa - Janeiro 2025*
