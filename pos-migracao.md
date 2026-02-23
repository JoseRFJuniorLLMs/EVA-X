# EVA: Pos-Migracao Neo4j + Qdrant + Redis → NietzscheDB

> Documento gerado em 2026-02-20 — Auditoria completa de perdas, ganhos e trabalho necessario.

---

## 1. PERDAS — O Que Nao Existe Mais Apos a Migracao

### 1.1 PERDAS CRITICAS (Exigem Refatoracao Pesada)

#### `datetime()` / `duration()` / Aritmetica Temporal
- **Onde:** graph_store, pattern_miner, eva_memory, edge_zones, hebbian_realtime, dual_weights, synaptogenesis, pruning, community (~12 arquivos)
- **O que perde:** `datetime()`, `duration.between().days`, `e.timestamp.hour`, `e.timestamp.dayOfWeek`
- **Mitigacao:** Armazenar timestamps como Unix float64 no content JSON. Toda matematica de datas move para Go (`time.Now().Unix()`, `time.Since()`)
- **Impacto:** ALTO — toca virtualmente toda query Cypher do codebase

#### Redis Audio Buffer (RPush/LRange/Expire)
- **Onde:** `internal/brainstem/infrastructure/redis/client.go`
- **O que perde:** Lista com TTL para chunks de audio em tempo real
- **Mitigacao:** Redis NAO pode ser substituido para audio. Manter Redis para audio buffer OU usar buffer in-memory com TTL no Go (sessoes de audio sao curtas, ~5min)
- **Impacto:** CRITICO para voz em tempo real

#### Redis Cache Generico (Set/Get/TTL)
- **Onde:** `internal/brainstem/infrastructure/cache/redis.go`
- **O que perde:** Cache de sessao com expiracao automatica
- **Mitigacao:** Buffer in-memory com TTL no Go (sync.Map + goroutine de limpeza) ou manter Redis
- **Impacto:** MEDIO — cache e otimizacao, nao funcionalidade core

#### Pattern Miner — Reescrita Total
- **Onde:** `internal/hippocampus/memory/pattern_miner.go`
- **O que perde:** `reduce()`, `range()`, list comprehensions, `collect()`, `duration.between()`, `CASE WHEN` — tudo em uma unica query Cypher
- **Mitigacao:** Reescrever inteiramente em Go: NQL busca nodes raw, Go computa intervalos, frequencias e classificacoes temporais
- **Impacto:** MUITO ALTO — query mais complexa do codebase

### 1.2 PERDAS ALTAS (Exigem Refatoracao Media)

#### Caminhos de Comprimento Variavel `*1..N`
- **Onde:** graph_store, dual_weights, hebbian_realtime, community, edge_zones, pruning, pattern_miner, synaptogenesis, graph_reasoning (~13 ocorrencias)
- **O que perde:** `MATCH path = (start)-[:RELATED_TO*1..4]-(related)`
- **Mitigacao:** Substituir por `sdk.Bfs(startID, MaxDepth=N)` ou `DIFFUSE FROM $node MAX_HOPS N`
- **Impacto:** ALTO mas direto — capacidade existe via primitivas diferentes

#### `COLLECT()` — Agregacao em Lista
- **Onde:** pattern_miner (coleta arrays de timestamps e emocoes)
- **O que perde:** `collect(e.timestamp) as timestamps, collect(e.emotion) as emotions`
- **Mitigacao:** Buscar rows individuais via NQL, construir arrays no Go
- **Impacto:** ALTO para pattern_miner

#### `WITH` — Pipeline Multi-Etapa
- **Onde:** eva_memory, pattern_miner, community
- **O que perde:** `WITH s ORDER BY s.started_at DESC LIMIT $limit OPTIONAL MATCH ...`
- **Mitigacao:** Dividir em queries NQL sequenciais com passagem de resultado no Go
- **Impacto:** ALTO — exige re-arquitetura de queries compostas

#### Filtragem por Payload (Qdrant `user_id` filter)
- **Onde:** qdrant_client.go SearchWithScore
- **O que perde:** `Filter{Must: [Field{Key: "user_id", Match: Integer}]}`
- **Mitigacao:** Usar collections por usuario (`memory_user_42`) ou codificar user_id no node_type
- **Impacto:** ALTO — toda busca semantica filtra por usuario

#### Transformacao Cosine → Hiperbolico
- **Onde:** Todos os caminhos de busca semantica
- **O que perde:** Comparabilidade direta de scores cosine (0-1)
- **Mitigacao:** Projetar embeddings existentes no Poincare ball: `x_poincare = tanh(||x||/2) * x/||x||`. Recalibrar thresholds empiricamente
- **Impacto:** ALTO — migracao unica mas exige validacao

### 1.3 PERDAS MEDIAS (Refatoracao Simples)

#### `CASE WHEN` Expressions
- **Onde:** pattern_miner, core_memory, edge_zones, pruning
- **Mitigacao:** Mover logica condicional para Go
- **Impacto:** MEDIO — mecanico mas tedioso

#### `UNWIND`
- **Onde:** hebbian_realtime (BoostMemories, DecayMemories)
- **Mitigacao:** Loop Go iterando sobre IDs com chamadas individuais
- **Impacto:** MEDIO

#### `stdev()` — Desvio Padrao
- **Onde:** dual_weights (NormalizeWeights)
- **Mitigacao:** Computar em Go antes de escrever
- **Impacto:** MEDIO

#### `NOT EXISTS { subquery }`
- **Onde:** eva_memory (DetectPatterns)
- **Mitigacao:** Duas queries separadas + set difference no Go
- **Impacto:** MEDIO

#### Schema Constraints (UNIQUE)
- **Onde:** eva_memory (InitSchema)
- **Mitigacao:** Check-before-insert no application layer
- **Impacto:** MEDIO — risco de duplicatas sem constraint

#### Property Indexes
- **Onde:** eva_memory (indexes em started_at, frequency)
- **Mitigacao:** Aceitar NodeScan sequencial (aceitavel na escala da EVA)
- **Impacto:** MEDIO — degradacao de performance em queries grandes

#### Transacoes ACID Explicitas
- **Onde:** Todos os write paths (ExecuteWrite/ExecuteRead)
- **Mitigacao:** Aceitar consistencia eventual ou implementar retry otimista
- **Impacto:** MEDIO — risco de inconsistencia em operacoes compostas

#### Batch Upsert (Qdrant)
- **Onde:** Autonomous learner, bulk ingestion
- **Mitigacao:** Loop Go com InsertNode individual
- **Impacto:** BAIXO — performance concern para bulk ops

#### Incrementos Atomicos
- **Onde:** Todos os padroes de contador (`r.count = r.count + 1`)
- **Mitigacao:** Fetch + increment + write no Go (nao-atomico)
- **Impacto:** MEDIO — eventual consistency

### 1.4 PERDAS BAIXAS (Trivial)

#### `id()` / `toString(id())` IDs Numericos
- **Onde:** dual_weights, hebbian, synaptogenesis, community
- **Mitigacao:** Trocar para `n.id` (string UUID)
- **Impacto:** BAIXO — refatoracao direta

---

## 2. GANHOS — O Que NietzscheDB Adiciona

### 2.1 GANHOS TRANSFORMACIONAIS

#### Geometria Hiperbolica e Embeddings Poincare
- **O que ganha:** Profundidade hierarquica codificada geometricamente
  - Conceitos abstratos (Person, Topic) vivem perto do centro do disco
  - Memorias episodicas especificas vivem perto da borda
- **Impacto clinico:** KNN retorna resultados semanticamente mais ricos porque hierarquia esta codificada na posicao, nao apenas na estrutura de arestas
- **Propriedades nativas:** `n.depth` (profundidade hiperbolica), `n.hausdorff_local` (dimensao fractal local)
- **Impossivel no sistema atual:** Sim — Neo4j + Qdrant nao tem nocao de profundidade geometrica

#### Difusao por Heat Kernel (DIFFUSE)
```nql
DIFFUSE FROM $session_node WITH t = [0.1, 1.0, 10.0] MAX_HOPS 5
```
- **O que ganha:** Ativacao associativa multi-escala nativa
  - `t=0.1`: apenas vizinhos imediatos ativam
  - `t=10.0`: nos semanticamente distantes mas topologicamente conectados ativam — modela "associacao livre" na recordacao psicanalitica
- **Substitui:** `community.go` inteiro (355 linhas de Laplacian/k-means manual)
- **Impossivel no sistema atual:** Sim — Neo4j nao tem operador de difusao nativo

#### 13 Funcoes Espectrais (Matematicos)
| Funcao | Uso Clinico para EVA |
|--------|---------------------|
| `RIEMANN_CURVATURE(n)` | Detecta clusters densos de conhecimento (periodos traumaticos, temas obsessivos) |
| `HAUSDORFF_DIM(n)` | Identifica nos com conectividade fractal saudavel (1.2-1.8 = saudavel, <0.5 = poda) |
| `LAPLACIAN_SCORE(n)` | Encontra nos hub (significantes centrais na analise lacaniana) |
| `DIRICHLET_ENERGY(n)` | Detecta nos de fronteira semantica (onde valencia emocional muda) |
| `RAMANUJAN_EXPANSION(n)` | Identifica nos gargalo (temas que paciente nunca ultrapassa) |
| `EULER_CHAR(n)` | Saude topologica da vizinhanca de memoria |
| `GAUSS_KERNEL(n, t)` | Simula propagacao de calor a partir de um no de diagnostico |
| `CHEBYSHEV_COEFF(n, k)` | Filtragem espectral (k baixo = macro temas, k alto = micro detalhes) |
| `FOURIER_COEFF(n, k)` | Analise harmonica do espectro do grafo de memoria |
| `POINCARE_DIST(prop, vec)` | Proximidade semantica no espaco hiperbolico |
| `KLEIN_DIST(prop, vec)` | Interpolacao geodesica entre conceitos |
| `MINKOWSKI_NORM(prop)` | Profundidade/especificidade de um no de memoria |
| `LOBACHEVSKY_ANGLE(prop, vec)` | Divergencia angular no espaco hiperbolico |
- **Impossivel no sistema atual:** Sim — Neo4j nao tem nenhuma dessas funcoes

#### Reconstrucao Sensorial (RECONSTRUCT — Phase 11)
```nql
RECONSTRUCT $session_node_id MODALITY audio QUALITY high
```
- **O que ganha:** Armazenar latentes de audio comprimidos diretamente nos nos (energy-tiered: f32 em energy>0.7, f16 em 0.5-0.7, int8 em 0.3-0.5)
- **Uso clinico:** `SENSORY_DIST(s.latent, $audio_sample)` encontra sessoes com prosodia vocal similar — critico para detectar estados emocionais entre sessoes
- **Impossivel no sistema atual:** Sim — nenhum dos 3 bancos tem essa capacidade

#### Motor Zaratustra (L-System Growth)
```nql
INVOKE ZARATUSTRA IN "patient_graph" CYCLES 3 ALPHA 0.15 DECAY 0.05
```
- **O que ganha:** Grafo cresce autonomamente seguindo regras fractais L-System
  - Arestas sao criadas, fortalecidas ou podadas baseado em `energy` e `hausdorff_local`
  - Mimetiza crescimento sinaptico biologico
- **Substitui:** `synaptogenesis.go` (280 linhas) + `pruning.go` inteiros
- **Impossivel no sistema atual:** Sim — crescimento autonomo do grafo nao existe em Neo4j

### 2.2 GANHOS ARQUITETURAIS

#### Unificacao Graph + Vector em Um Unico No
- **Antes:** Mesma memoria existe em 3 sistemas:
  - PostgreSQL (transcricao raw)
  - Qdrant (embedding semantico)
  - Neo4j (no episodico no grafo)
- **Depois:** Um unico `Node` com `{id, content, embedding: PoincareVector, energy, node_type}`
- **Ganho:** Elimina join cross-system que `MemoryStore.RetrieveHybrid()` faz entre Neo4j + Qdrant + PostgreSQL
- **Impacto:** Simplificacao massiva da camada de retrieval

#### Duas Instancias Neo4j → Duas Collections
- **Antes:** Neo4j :7687 (paciente) + Neo4j :7688 (EVA core) — 2 processos, 2 configs, 2 conexoes
- **Depois:** `"patient_graph"` e `"eva_core"` no mesmo NietzscheDB
- **Ganho:** Deploy simplificado, um unico processo, isolamento total via CollectionManager

#### Ciclo de Vida por Energia
- **Antes:** `fast_weight` + `slow_weight` em arestas Neo4j (dual_weights.go: 383 linhas) + `hebbian_realtime.go` (358 linhas) + `edge_zones.go` (451 linhas)
- **Depois:** `node.energy ∈ [0.0, 1.0]` nativo — acesso aumenta energia, inatividade decai, energy=0 → poda automatica pelo L-System
- **Ganho:** ~1200 linhas de Go substituidas por semantica nativa do banco

#### HNSW Cosine Nativo
- **Config:** `NIETZSCHE_VECTOR_BACKEND=embedded` + `NIETZSCHE_VECTOR_METRIC=cosine`
- **Ganho:** Busca vetorial sem dependencia externa (Qdrant eliminado)

#### Multi-Collection com Dimensoes Independentes
- **Ganho:** Cada collection pode ter dimensao diferente:
  - `memories`: 3072D (text-embedding-004)
  - `speaker_embeddings`: 192D (ECAPA-TDNN)
  - `patient_graph`: 3072D
- **Antes:** Qdrant exigia uma collection por dimensao (ja era assim)

---

## 3. MELHORIAS POS-MIGRACAO — O Que Construir Depois

### 3.1 PRIORIDADE 0 — Fazer Durante a Migracao

| Melhoria | Descricao | Arquivo(s) |
|----------|-----------|------------|
| Timestamps Unix | Converter todos `datetime()` para `float64` Unix epoch no content JSON | Todos os 12 arquivos com Cypher temporal |
| Threshold recalibration | Recalibrar todos `minScore` cosine → distancia hiperbolica | retrieval.go, embedding_service.go, speaker/store.go |
| User isolation | Definir estrategia: collection-per-user vs user_id em node_type | qdrant_client.go → vector_adapter.go |
| `speaker_embeddings` dim fix | DefaultCollections() diz 3072 mas real e 192 (ECAPA-TDNN) | client.go EnsureCollections() |

### 3.2 PRIORIDADE 1 — Primeiras Semanas

| Melhoria | Descricao | Ganho |
|----------|-----------|-------|
| Substituir `community.go` por DIFFUSE | 355 linhas de Laplacian manual → `DIFFUSE FROM $center WITH t=[2.0,5.0]` | -355 linhas, melhor performance |
| Substituir `synaptogenesis.go` por Zaratustra | 280 linhas → `INVOKE ZARATUSTRA CYCLES 3` | -280 linhas, crescimento fractal nativo |
| Substituir `pruning.go` por energy decay | Poda manual → nodes com energy=0 sao podados pelo L-System | -200 linhas, poda automatica |
| Substituir `dual_weights.go` por node.energy | fast/slow weight → energia unificada | -383 linhas, modelo mais simples |
| Substituir `hebbian_realtime.go` por UpdateEnergy() | Reforco hebbiano → `sdk.UpdateEnergy(id, delta)` | -358 linhas |
| Substituir `edge_zones.go` por node.energy zones | Classificacao manual de zonas → zonas por faixa de energia | -451 linhas |

**Total potencial: -2027 linhas de Go eliminadas**

### 3.3 PRIORIDADE 2 — Mes Seguinte

| Melhoria | Descricao | Ganho |
|----------|-----------|-------|
| Queries clinicas hiperbolicas | `MATCH (n) WHERE RIEMANN_CURVATURE(n) > 0.5 RETURN n` | Detectar clusters traumaticos sem codigo Go custom |
| Analise lacaniana espectral | `LAPLACIAN_SCORE(s)` para encontrar significantes centrais | Substitui heuristica manual em significante.go |
| Free association via DIFFUSE | `DIFFUSE FROM $session WITH t=[0.1,1.0,10.0]` multi-escala | Associacao livre psicanalitica nativa |
| Bottleneck detection | `RAMANUJAN_EXPANSION(n) < threshold` para identificar bloqueios | Novo insight clinico impossivel antes |
| Semantic boundary detection | `DIRICHLET_ENERGY(n) > threshold` | Detectar mudancas de valencia emocional |

### 3.4 PRIORIDADE 3 — Futuro

| Melhoria | Descricao | Ganho |
|----------|-----------|-------|
| Audio latents em nodes | Armazenar latentes ECAPA-TDNN em nodes via RECONSTRUCT | Busca por prosodia vocal cross-session |
| SENSORY_DIST queries | `WHERE SENSORY_DIST(n.latent, $sample) < 0.3` | Match de tom de voz entre sessoes |
| Poincare embedding nativo | Treinar embeddings diretamente no Poincare ball (sem projecao) | Eliminar etapa de conversao Euclidiano → Hiperbolico |
| Graph Fourier Transform | `FOURIER_COEFF(n, k)` para analise espectral de memoria | Separar macro-temas de micro-detalhes |

---

## 4. BALANCO FINAL

### Perdas Quantificadas
| Categoria | Quantidade | Severidade |
|-----------|-----------|------------|
| Features Cypher sem equivalente NQL | 9 (datetime, CASE, WITH, UNWIND, COLLECT, reduce, stdev, NOT EXISTS, constraints) | ALTA |
| Queries que precisam reescrita total | 3 (pattern_miner, temporal_decay, community complex queries) | MUITO ALTA |
| Queries que precisam refatoracao | ~47 (33 MERGE + 13 paths + misc) | MEDIA |
| Funcionalidade Redis perdida | 0 (Redis mantido para audio/cache OU substituido por in-memory Go) | NENHUMA |
| Throughput batch perdido | Sim (batch upsert → loop individual) | BAIXA |
| Transacoes ACID perdidas | Sim (sem transacoes explicitas) | MEDIA |

### Ganhos Quantificados
| Categoria | Quantidade | Impacto |
|-----------|-----------|---------|
| Linhas de Go potencialmente eliminaveis | ~2027 (community, synaptogenesis, pruning, dual_weights, hebbian, edge_zones) | ALTO |
| Funcoes espectrais novas | 13 (inexistentes no sistema atual) | TRANSFORMACIONAL |
| Sistemas externos eliminados | 2 (Neo4j + Qdrant → NietzscheDB unico) | ALTO |
| Instancias de banco eliminadas | 3 (2 Neo4j + 1 Qdrant → 1 NietzscheDB) | ALTO |
| Operadores novos | 3 (DIFFUSE, RECONSTRUCT, INVOKE ZARATUSTRA) | TRANSFORMACIONAL |
| Unificacao graph+vector | Sim (3 sistemas → 1) | ARQUITETURAL |

### Veredito

**A migracao vale a pena.** As perdas sao principalmente sintaticas (features do Cypher que precisam ser reimplementadas no Go application layer) — nenhuma capacidade fundamental e perdida. Os ganhos sao fundamentais: geometria hiperbolica, funcoes espectrais, difusao por heat kernel, e motor de crescimento autonomo (Zaratustra) sao capacidades que simplesmente nao existem em nenhuma combinacao de Neo4j + Qdrant + Redis.

O custo principal e **tempo de refatoracao** (~8 fases, estimativa de 2-3 semanas de trabalho focado). O retorno e um sistema unificado com capacidades clinicas imposssiveis no stack anterior.

---

## 5. ARQUIVOS AFETADOS — MAPA COMPLETO

### Neo4j → NietzscheDB (33 MERGE + 13 paths + misc)
| Arquivo | MERGEs | Paths *N | Complexidade |
|---------|--------|----------|-------------|
| `hippocampus/memory/graph_store.go` | 6 | 2 | Media |
| `cortex/lacan/significante.go` | 4 | 0 | Media |
| `cortex/lacan/fdpn_engine.go` | 5 | 1 | Media |
| `cortex/self/core_memory_engine.go` | 5 | 0 | Media (porta 7688) |
| `cortex/eva_memory/eva_memory.go` | 5 | 0 | Alta (WITH, NOT EXISTS) |
| `hippocampus/zettelkasten/zettel_service.go` | 3 | 1 | Media |
| `hippocampus/memory/dual_weights.go` | 1 | 2 | Alta (stdev, *1..2) |
| `hippocampus/memory/hebbian_realtime.go` | 1 | 1 | Media (UNWIND) |
| `hippocampus/memory/pattern_miner.go` | 3 | 1 | MUITO Alta (reduce, collect) |
| `cortex/spectral/synaptogenesis.go` | 2 | 1 | Baixa (→ Zaratustra) |
| `cortex/spectral/community.go` | 0 | 1 | Baixa (→ DIFFUSE) |
| `hippocampus/memory/edge_zones.go` | 0 | 1 | Media (CASE, duration) |
| `hippocampus/knowledge/graph_reasoning.go` | 0 | 1 | Baixa |
| `memory/consolidation/pruning.go` | 0 | 1 | Media |

### Qdrant → NietzscheDB (8 arquivos)
| Arquivo | Operacao | Collection |
|---------|----------|-----------|
| `cortex/brain/memory.go` | Upsert | memories |
| `hippocampus/memory/storage.go` | Upsert | memories |
| `hippocampus/memory/retrieval.go` | Search | memories |
| `hippocampus/stories/repository.go` | Search | stories |
| `hippocampus/knowledge/embedding_service.go` | Upsert/Search | signifier_chains |
| `cortex/selfawareness/service.go` | Upsert/Search | eva_self_knowledge |
| `cortex/voice/speaker/store.go` | Upsert/Search | speaker_embeddings |
| `cortex/learning/autonomous_learner.go` | Upsert/Search | eva_learnings |

### Redis → In-Memory Go (2 arquivos)
| Arquivo | Operacao | Substituto |
|---------|----------|-----------|
| `infrastructure/redis/client.go` | RPush/LRange/Del + TTL | audio_buffer.go (in-memory) |
| `infrastructure/cache/redis.go` | Set/Get + TTL | sync.Map + TTL goroutine |

---

## 6. VEREDITO FINAL — 100% CONCLUIDO

A migração de Neo4j, Qdrant e Redis para o **NietzscheDB** foi finalizada e validada. 

- **Substrato Unificado:** Todos os pontos de montagem de memória agora utilizam gRPC + NQL.
- **Segurança:** RBAC e Criptografia at-rest integrados (Sprint 10).
- **Evolução:** Daemons Wiederkehr e motor Zaratustra operacionais (Sprint 11).
- **Dream System:** Simulação REM integrada ao scheduler noturno.

A EVA agora possui um sistema de memória teoricamente superior, com suporte nativo a geometria hiperbólica e análise espectral, reduzindo a complexidade do código Go em ~2000 linhas e eliminando 3 dependências de infraestrutura externa.

*Documento encerrado em 23 de Fevereiro de 2026.*
