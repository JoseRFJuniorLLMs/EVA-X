# 📋 PLANO DE IMPLEMENTAÇÃO - EVA-Mind RAM + Hebbian Extensions

**Versão:** 1.1 (com Situational Modulator como Fase E0)
**Data:** 2026-02-16
**Status:** ✅ Aprovado para execução
**Base:** SRC.md + mente.md + Auditoria da arquitetura atual

---

## 🎯 OBJETIVO

Transformar EVA-Mind de **"IA de laboratório"** (estática) para **"IA de acompanhamento"** (dinâmica), implementando 7 extensões críticas que fecham o gap entre banco de dados e cérebro sintético.

**Validações integradas do mente.md:**
- ✅ Hebbian Real-Time com safeguards (não saturação)
- ✅ SRC descartado (usar embedding similarity puro)
- ✅ **Situational Modulator como Fase E0 (PRIORIDADE MÁXIMA)**
- ✅ Performance garantida (<50ms overhead)
- ✅ Consolidação noturna como "corretor de excessos"

---

## 📊 GAPS IDENTIFICADOS vs. CÓDIGO ATUAL

| Gap | Impacto | Status Atual | Solução |
|-----|---------|--------------|---------|
| **0. Situational Modulator** | 🔴 **CRÍTICO** | NÃO EXISTE | **E0: Rules + cache + LLM light (<10ms) - CÓDIGO PRONTO** |
| **1. Hebbian Real-Time** | 🔴 ALTO | Existe em `hebbian.go`, só roda 1x/dia | Mover para pós-query com safeguards |
| **2. DHP (Dual Weights)** | 🔴 ALTO | Arestas têm UM peso | `slow_weight` + `fast_weight` |
| **3. Edge Zones → Ações** | 🟡 MÉDIO-ALTO | Pruning existe, preload/sugestão NÃO | 3 zonas com ações automáticas |
| **4. Entity Resolution** | 🟡 MÉDIO | Regex-only | Embedding similarity (SEM SRC) |
| **5. FDPN → Retrieval Boost** | 🟡 MÉDIO | FDPN e Retrieval não conversam | Injetar energia (+15% boost) |
| **6. RAM (Feedback Loop)** | 🔴 MÁXIMO | NÃO EXISTE | 3 interpretações + validação + feedback |

---

## 🚀 CRONOGRAMA FINAL (12 Semanas)

```
Semana 1:    FASE E0 (Situational Modulator)  ⚡ PRIORIDADE
Semana 2-3:  FASE A (Hebbian RT + DHP)
Semana 2-3:  FASE B (FDPN Boost) [PARALELO]
Semana 4-5:  FASE C (Edge Zones)
Semana 6-8:  FASE D (Entity Resolution SEM SRC)
Semana 9-12: FASE E1-E3 (RAM: alternativas + feedback)
```

---

## FASE E0: Situational Modulator ⚡ PRIORIDADE MÁXIMA

### Por que PRIMEIRO:
- ✅ Menor esforço (código 70% pronto no mente.md)
- ✅ Maior impacto imediato (personality dinâmica)
- ✅ Não depende de outras fases
- ✅ Fecha gap crítico: "agitação em aniversário ≠ hospital"

### Arquivo: `internal/cortex/situation/modulator.go`

```go
package situation

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "eva-mind/internal/brainstem/infrastructure/cache"
    "eva-mind/internal/cortex/llm"
)

type Situation struct {
    Stressors     []string  `json:"stressors"`
    SocialContext string    `json:"social_context"`
    TimeOfDay     string    `json:"time_of_day"`
    EmotionScore  float64   `json:"emotion_score"`
    Intensity     float64   `json:"intensity"`
}

type SituationalModulator struct {
    llm   llm.Provider
    cache *cache.NietzscheDBClient
}

func NewModulator(llm llm.Provider, cache *cache.NietzscheDBClient) *SituationalModulator {
    return &SituationalModulator{llm: llm, cache: cache}
}

func (m *SituationalModulator) Infer(ctx context.Context, userID string, recentText string) (Situation, error) {
    cacheKey := fmt.Sprintf("situation:%s", userID)
    if cached, err := m.cache.Get(ctx, cacheKey); err == nil {
        var sit Situation
        if json.Unmarshal([]byte(cached), &sit) == nil {
            return sit, nil
        }
    }

    sit := Situation{TimeOfDay: getTimeOfDay(time.Now())}
    sit.Stressors = extractStressors(recentText)
    sit.SocialContext = "familia"
    sit.EmotionScore = inferEmotion(recentText)

    m.cache.Set(ctx, cacheKey, sit, 5*time.Minute)
    return sit, nil
}

func (m *SituationalModulator) ModulateWeights(baseWeights map[string]float64, sit Situation) map[string]float64 {
    modulated := make(map[string]float64)
    for k, v := range baseWeights {
        modulated[k] = v
    }

    if contains(sit.Stressors, "luto") {
        modulated["ANSIEDADE"] *= 1.8
        modulated["BUSCA_SEGURANÇA"] *= 2.0
        modulated["EXTROVERSÃO"] *= 0.5
    }

    if contains(sit.Stressors, "hospital") {
        modulated["ALERTA"] *= 2.0
        modulated["BUSCA_SEGURANÇA"] *= 1.5
    }

    if sit.SocialContext == "sozinho" && sit.TimeOfDay == "madrugada" {
        modulated["SOLIDÃO"] *= 1.5
        modulated["ANSIEDADE"] *= 1.3
    }

    return modulated
}

func getTimeOfDay(t time.Time) string {
    hour := t.Hour()
    switch {
    case hour >= 0 && hour < 6: return "madrugada"
    case hour >= 6 && hour < 12: return "manha"
    case hour >= 12 && hour < 18: return "tarde"
    default: return "noite"
    }
}

func extractStressors(text string) []string {
    stressors := []string{}
    keywords := map[string]string{
        "faleceu": "luto", "morreu": "luto", "velório": "luto",
        "hospital": "hospital", "doente": "doença",
        "aniversário": "aniversário", "festa": "festa",
    }

    for keyword, stressor := range keywords {
        if strings.Contains(strings.ToLower(text), keyword) {
            stressors = append(stressors, stressor)
        }
    }
    return stressors
}

func inferEmotion(text string) float64 {
    positive := []string{"feliz", "alegre", "bom"}
    negative := []string{"triste", "mal", "ruim"}

    score := 0.0
    textLower := strings.ToLower(text)
    for _, word := range positive {
        if strings.Contains(textLower, word) { score += 0.3 }
    }
    for _, word := range negative {
        if strings.Contains(textLower, word) { score -= 0.3 }
    }
    return score
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item { return true }
    }
    return false
}
```

### Integração FDPN: `fdpn_engine.go`

```go
func (e *FDPNEngine) StreamingPrimeWithSituation(ctx context.Context, userID string, text string) error {
    sit, err := e.situationModulator.Infer(ctx, userID, text)
    if err != nil {
        return e.StreamingPrime(ctx, userID, text)
    }

    baseWeights := e.getBasePersonalityWeights(userID)
    modulatedWeights := e.situationModulator.ModulateWeights(baseWeights, sit)

    e.primeWithModulatedWeights(ctx, userID, text, modulatedWeights)

    if sit.Intensity > 0.8 && contains(sit.Stressors, "crise") {
        e.alertService.SendCritical(userID, "Possível crise detectada", sit)
    }

    return nil
}
```

---

## FASE A: Hebbian Real-Time + DHP (Semanas 2-3)

### A1. Hebbian Real-Time com Safeguards
**Arquivo:** `internal/hippocampus/memory/hebbian_realtime.go`

```go
type HebbianRealTime struct {
    NietzscheDB   *graph.NietzscheDBClient
    eta     float64  // 0.01
    lambda  float64  // 0.001 (safeguard)
    tau     float64  // 86400
    timeout time.Duration // 100ms
}

func (h *HebbianRealTime) UpdateWeights(ctx context.Context, patientID int64, nodeIDs []string) error {
    ctx, cancel := context.WithTimeout(ctx, h.timeout)
    defer cancel()

    for i := 0; i < len(nodeIDs)-1; i++ {
        for j := i+1; j < len(nodeIDs); j++ {
            deltaT := h.getTimeSinceLastActivation(nodeIDs[i], nodeIDs[j])
            decay := math.Exp(-deltaT / h.tau)
            deltaW := h.eta * decay - h.lambda * h.getCurrentWeight(nodeIDs[i], nodeIDs[j])

            h.updateNietzscheDB(ctx, nodeIDs[i], nodeIDs[j], deltaW)
        }
    }
    return nil
}
```

### A2. DHP - Dual Weights
**Migration NietzscheDB:**
```cypher
MATCH ()-[r:ASSOCIADO_COM]->()
SET r.slow_weight = COALESCE(r.slow_weight, r.weight),
    r.fast_weight = COALESCE(r.fast_weight, 0.5),
    r.weight = 0.3 * r.slow_weight + 0.7 * r.fast_weight
```

---

## FASE B: FDPN → Retrieval Boost (Semanas 2-3, paralelo)

### Modificação: `retrieval.go`

```go
func (r *RetrievalService) RetrieveHybrid(ctx context.Context, idosoID int64, query string, k int) ([]*SearchResult, error) {
    queryEmbedding, _ := r.embedder.GenerateEmbedding(ctx, query)

    activatedNodes, _ := r.fdpn.StreamingPrime(ctx, fmt.Sprintf("%d", idosoID), query)

    qResults, _ := r.NietzscheDB.Search(ctx, "memories", queryEmbedding, uint64(k), nil)

    for _, result := range qResults {
        nodeID := extractNodeID(result)
        if activatedNodes[nodeID] > 0 {
            result.Score *= (1.0 + 0.15 * activatedNodes[nodeID])
        }
    }

    go r.hebbianRT.UpdateWeights(ctx, idosoID, activatedNodeIDs)

    return allResults, nil
}
```

---

## FASE C: Edge Zones + Ações (Semanas 4-5)

### Arquivo: `internal/hippocampus/memory/edge_zones.go`

```go
type EdgeZone string

const (
    ZoneConsolidated EdgeZone = "consolidated" // w > 0.7
    ZoneEmerging     EdgeZone = "emerging"     // 0.3 < w < 0.7
    ZoneWeak         EdgeZone = "weak"         // w < 0.3
)

type EdgeClassifier struct {
    thresholdHigh float64
    thresholdLow  float64
}

func (c *EdgeClassifier) GetConsolidatedEdges(ctx context.Context, patientID int64) ([]AssociationEdge, error) {
    query := `
        MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM]-(n2)
        WHERE r.weight > 0.7
        RETURN n1.name, n2.name, r.weight
    `
    // ...
}
```

### API Endpoint: `api/associations_routes.go`

```go
func GetEmergingAssociations(c *gin.Context) {
    patientID := c.Param("patient_id")
    edges, _ := edgeClassifier.GetEmergingEdges(ctx, patientID)

    c.JSON(200, gin.H{
        "patient_id": patientID,
        "emerging_count": len(edges),
        "associations": edges,
    })
}
```

---

## FASE D: Entity Resolution SEM SRC (Semanas 6-8)

**Validação mente.md:** SRC inútil, usar embedding similarity puro.

### Arquivo: `internal/hippocampus/knowledge/entity_resolver.go`

```go
type EntityResolver struct {
    NietzscheDB    *graph.NietzscheDBClient
    embedder *EmbeddingService
}

func (r *EntityResolver) ResolveEntity(ctx context.Context, entityName string, contextText string) (*EntityResolutionResult, error) {
    contextEmb, _ := r.embedder.GenerateEmbedding(ctx, contextText)
    knownEntities, _ := r.getKnownEntities(ctx, entityName)
    bestMatch := r.findBestMatch(contextEmb, knownEntities)

    if bestMatch.Similarity > 0.85 {
        return &EntityResolutionResult{IsKnown: true, EntityID: bestMatch.ID}, nil
    } else if bestMatch.Similarity > 0.6 {
        return &EntityResolutionResult{ShouldAsk: true, Candidates: candidates}, nil
    }

    return &EntityResolutionResult{IsKnown: false}, nil
}
```

---

## FASE E1-E3: RAM (Semanas 9-12)

### E1. Geração de Alternativas
**Arquivo:** `internal/cortex/ram/interpretation_generator.go`

```go
func (g *InterpretationGenerator) GenerateAlternatives(ctx context.Context, patientID int64, behaviorCue string) ([]Interpretation, error) {
    prompt := fmt.Sprintf(`
        Paciente %d apresentou: "%s"

        Gere 3 interpretações:
        1. Conservadora (fatos)
        2. Moderada (padrões)
        3. Liberal (inferências)
    `, patientID, behaviorCue)

    alternatives, _ := g.gemini.Generate(ctx, prompt)

    for i, alt := range alternatives {
        alternatives[i].Confidence = g.validateAgainstHistory(ctx, patientID, alt)
    }

    return alternatives, nil
}
```

### E2. Validação Contra Histórico
```go
func (v *InterpretationValidator) Validate(ctx context.Context, patientID int64, interpretation string) (*ValidationResult, error) {
    memories, _ := v.retrieval.RetrieveHybridReflective(ctx, patientID, interpretation, 20)

    var supportCount, conflictCount int
    for _, mem := range memories {
        alignment := v.assessAlignment(interpretation, mem.Content)
        if alignment > 0.7 { supportCount++ }
        if alignment < 0.3 { conflictCount++ }
    }

    confidence := float64(supportCount) / float64(supportCount + conflictCount)
    isDubious := conflictCount >= 3

    return &ValidationResult{Confidence: confidence, IsDubious: isDubious}, nil
}
```

### E3. Feedback do Cuidador
**Arquivo:** `api/feedback_routes.go`

```go
func SubmitFeedback(c *gin.Context) {
    var req FeedbackRequest
    c.BindJSON(&req)

    interpretation, _ := ramService.GetInterpretation(ctx, req.InterpretationID)

    if req.IsUseful {
        hebbianRT.BoostMemories(ctx, interpretation.EvidenceMemoryIDs, 0.15)
    } else {
        hebbianRT.DecayMemories(ctx, interpretation.EvidenceMemoryIDs, 0.10)
    }

    feedbackStore.Save(ctx, &req)
    c.JSON(200, gin.H{"message": "Feedback registrado"})
}
```

---

## 📦 ENTREGÁVEIS

| Fase | Arquivos | Critério de Aceite |
|------|----------|-------------------|
| **E0** | `situation/modulator.go` | Weights modulados; <10ms latência |
| **A** | `hebbian_realtime.go`, `dual_weights.go` | Peso atualiza pós-query; safeguards OK |
| **B** | `retrieval.go` modificado | Boost +15% funciona |
| **C** | `edge_zones.go`, `associations_routes.go` | API retorna emerging |
| **D** | `entity_resolver.go` | "Maria" = "mãe Maria" (sim>0.85) |
| **E1-E3** | `interpretation_generator.go`, `feedback_routes.go` | 3 interpretações; feedback modula |

---

## 📈 MÉTRICAS DE SUCESSO

1. **E0:** Latência <10ms; cache hit >80%
2. **A:** Hebbian RT <50ms overhead
3. **B:** Recall de associações indiretas +30%
4. **C:** Consolidated edges no contexto 100% casos
5. **D:** Entity resolution accuracy >85%
6. **E:** Feedback positivo >70%

---

## ⚠️ RISCOS E MITIGAÇÕES

| Risco | Mitigação |
|-------|-----------|
| E0 latência | Rules-first, cache, LLM só ambíguos |
| A saturação | λ=0.001, timeout, normalização noturna |
| D merge incorreto | Threshold 0.85, flag revisão |

---

## 🔧 CONFIGURAÇÃO

```yaml
situational_modulator:
  cache_ttl_minutes: 5
  timeout_ms: 100

hebbian_realtime:
  eta: 0.01
  lambda: 0.001
  tau: 86400
  timeout_ms: 100

dual_weights:
  slow_ratio: 0.3
  fast_ratio: 0.7

edge_zones:
  threshold_high: 0.7
  threshold_low: 0.3

entity_resolution:
  high_confidence: 0.85
  medium_confidence: 0.60

ram:
  num_alternatives: 3
  validation_memory_count: 20
```

---

## 📞 PRÓXIMOS PASSOS

### Semana 1 (FASE E0) ⚡
1. Implementar `situation/modulator.go`
2. Integrar `fdpn_engine.go`
3. Testes: funeral, festa, hospital
4. Deploy staging

### Semana 2-3 (FASES A+B)
1. `hebbian_realtime.go` com safeguards
2. Migration NietzscheDB dual weights
3. FDPN boost no retrieval

---

**Status:** 🟢 Aprovado - início IMEDIATO Fase E0
**Código-fonte E0:** 70% pronto (mente.md)

**Esta é a engenharia de um cérebro sintético.** 🧠⚡