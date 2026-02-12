# EVA-Mind: Documentação Técnica Consolidada

**Versão:** 2.0  
**Última Atualização:** 12/02/2026  
**Stack:** Go + PostgreSQL + Qdrant + Neo4j + Redis + Firebase

---

## 📋 Índice

1. [Visão Geral](#visão-geral)
2. [Arquitetura do Sistema](#arquitetura-do-sistema)
3. [Componentes Principais](#componentes-principais)
4. [Módulos Implementados](#módulos-implementados)
5. [Fluxo de Dados](#fluxo-de-dados)
6. [Métricas e Performance](#métricas-e-performance)
7. [Guia de Desenvolvimento](#guia-de-desenvolvimento)

---

## 🎯 Visão Geral

EVA-Mind é um sistema de IA conversacional para cuidado de idosos, construído com arquitetura modular inspirada em neurociência cognitiva. O sistema integra memória episódica/semântica, processamento emocional, análise clínica e swarm de agentes especializados.

### Princípios de Design

- **Modularidade**: Componentes independentes com interfaces bem definidas
- **Escalabilidade**: Arquitetura distribuída com suporte a múltiplos datastores
- **Observabilidade**: Logging estruturado e métricas Prometheus
- **Segurança**: LGPD-compliant com audit trail completo

---

## 🏗️ Arquitetura do Sistema

```
┌─────────────────────────────────────────────────────────────┐
│                     EVA-Mind Architecture                    │
└─────────────────────────────────────────────────────────────┘

┌──────────────┐
│   Mobile     │ ◄─── WebSocket/REST
│   Frontend   │
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────┐
│                      BRAINSTEM (Core)                         │
│  • Auth & OAuth                                               │
│  • Database Layer (PostgreSQL)                                │
│  • Push Notifications (Firebase)                              │
│  • Subscription Management                                    │
└──────────────────────────────────────────────────────────────┘
       │
       ├─────────────────┬─────────────────┬──────────────────┐
       ▼                 ▼                 ▼                  ▼
┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌──────────────┐
│   SENSES    │   │   CORTEX    │   │ HIPPOCAMPUS │   │    SWARM     │
│             │   │             │   │             │   │              │
│ • Signaling │   │ • Brain     │   │ • Memory    │   │ • Agents     │
│ • WebSocket │   │ • Attention │   │ • Knowledge │   │ • Tools      │
│ • Audio     │   │ • Learning  │   │ • Retrieval │   │ • Routing    │
│ • Video     │   │ • Conscious │   │ • Graph     │   │              │
└─────────────┘   └─────────────┘   └─────────────┘   └──────────────┘
       │                 │                 │                  │
       └─────────────────┴─────────────────┴──────────────────┘
                                │
                                ▼
       ┌────────────────────────────────────────────────────────┐
       │                   DATA LAYER                            │
       │                                                         │
       │  PostgreSQL  │  Qdrant  │  Neo4j  │  Redis  │  S3     │
       │  (Relational)│ (Vector) │ (Graph) │ (Cache) │ (Files) │
       └────────────────────────────────────────────────────────┘
```

---

## 🧠 Componentes Principais

### 1. BRAINSTEM (Infraestrutura)

**Localização:** `internal/brainstem/`

Camada base que gerencia autenticação, database, logging e push notifications.

#### Módulos:
- **Auth** (`auth/`): JWT, OAuth 2.0, middleware de autenticação
- **Database** (`database/`): Queries PostgreSQL, migrations
- **Push** (`push/`): Firebase Cloud Messaging, CallKit notifications
- **Config** (`config/`): Configuração centralizada
- **Logger** (`logger/`): Logging estruturado (zerolog)

#### Exemplo de Uso:
```go
// Autenticação
authSvc := auth.NewService(db, jwtSecret)
token, err := authSvc.Login(ctx, email, password)

// Push Notification
pushSvc := push.NewFirebaseService(firebaseApp)
err = pushSvc.SendNotification(ctx, deviceToken, title, body)
```

---

### 2. SENSES (Entrada/Saída)

**Localização:** `internal/senses/`

Gerencia comunicação com usuários via WebSocket, áudio e vídeo.

#### Módulos:
- **Signaling** (`signaling/`): WebSocket server, sessões, Gemini Live API
- **Audio** (`audio/`): Processamento de áudio, transcrição
- **Video** (`video/`): Chamadas de vídeo, gravação

#### Features Implementadas:
- ✅ WebSocket bidire

cional com Gemini Live API
- ✅ Transcrição em tempo real (Gemini)
- ✅ Análise de emoção via áudio (AudioAnalysisService)
- ✅ Sessões persistentes com context
- ✅ CallKit integration para iOS

#### Exemplo de Uso:
```go
// WebSocket Session
session := signaling.NewWebSocketSession(idosoID, conn)
session.SetAudioContext(emotion, urgency, intensity)

// Audio Analysis
audioSvc := knowledge.NewAudioAnalysisService(geminiClient)
result, err := audioSvc.AnalyzeAudioContext(ctx, sessionID, idosoID)
```

---

### 3. CORTEX (Processamento Cognitivo)

**Localização:** `internal/cortex/`

Núcleo de processamento cognitivo, incluindo memória, atenção, aprendizado e consciência.

#### Submódulos:

##### 3.1 Brain (`cortex/brain/`)
- **Memory** (`memory.go`): Processamento de fala do usuário, atomic facts
- **Service** (`service.go`): Orquestração de memória episódica/semântica
- **Context** (`context.go`): Gerenciamento de contexto conversacional

**Features:**
- ✅ Atomic fact extraction via LLM
- ✅ Dual timestamp (document_date + event_date)
- ✅ Dynamic importance calculation
- ✅ Emotion detection integration
- ✅ Krylov compression (3072D → 64D)

##### 3.2 Attention (`cortex/attention/`)
- **Executive** (`executive.go`): Gurdjieffian executive function
- **Wavelet Attention** (`wavelet_attention.go`): Multi-scale attention
- **Pattern Interrupt** (`pattern_interrupt.go`): Interrupção de padrões negativos
- **Affect Stabilizer** (`affect_stabilizer.go`): Estabilização emocional

**Features:**
- ✅ 3 centros de atenção (Intelectual, Emocional, Motor)
- ✅ Estratégias adaptativas (Reflective, Supportive, Pattern Interrupt)
- ✅ Confidence gating
- ✅ Minimal optimizer (economia de tokens)

##### 3.3 Consciousness (`cortex/consciousness/`)
- **Global Workspace** (`global_workspace.go`): Integração multi-modular

**Features:**
- ✅ Competição de módulos por atenção
- ✅ Broadcast do vencedor
- ✅ Síntese de insights cruzados

##### 3.4 Learning (`cortex/learning/`)
- **Meta-Learner** (`meta_learner.go`): Aprendizado sobre aprendizado
- **Strategy Evolution** (`strategy_evolution.go`): Evolução de estratégias

---

### 4. HIPPOCAMPUS (Memória)

**Localização:** `internal/hippocampus/`

Sistema de memória episódica/semântica com múltiplos datastores.

#### Módulos:

##### 4.1 Memory (`hippocampus/memory/`)
- **Storage** (`storage.go`): CRUD de memórias (PostgreSQL + Qdrant + Neo4j)
- **Retrieval** (`retrieval.go`): Busca semântica multi-tier
- **Graph** (`graph.go`): Operações no grafo Neo4j

**Features:**
- ✅ Triple storage (PG + Qdrant + Neo4j)
- ✅ Semantic search via embeddings
- ✅ Graph traversal para contexto
- ✅ Tiered retrieval (hot/warm/cold)

##### 4.2 Knowledge (`hippocampus/knowledge/`)
- **Audio Analysis** (`audio_analysis.go`): Análise emocional de áudio
- **Context Builder** (`context_builder.go`): Construção de contexto
- **Unified Retrieval** (`unified_retrieval.go`): Busca unificada RSI + FDPN

**Features:**
- ✅ Emotion detection (tristeza, ansiedade, alegria, etc.)
- ✅ Urgency classification (BAIXA, MEDIA, ALTA, CRITICA)
- ✅ Intensity scoring (1-10)
- ✅ Priming semântico (FDPN)

---

### 5. MEMORY (Consolidação e Compressão)

**Localização:** `internal/memory/`

Engines de consolidação, compressão e ingestion de memórias.

#### Módulos:

##### 5.1 Krylov (`memory/krylov_manager.go`)
- Compressão de embeddings 3072D → 64D
- Gram-Schmidt Modificado
- Sliding Window FIFO
- Rank-1 Updates

**Métricas:**
- Recall@10: 97%
- Compressão: 48x
- Update time: 52µs/op

##### 5.2 Consolidation (`memory/consolidation/`)
- **REM Consolidator** (`rem_consolidator.go`): Consolidação noturna
- **Pruning** (`pruning.go`): Poda sináptica
- **Synaptogenesis** (`synaptogenesis.go`): Criação de conexões

**Features:**
- ✅ Consolidação às 3h da manhã
- ✅ Replay de memórias quentes
- ✅ Clustering espectral
- ✅ Transferência episódica → semântica
- ✅ Poda de 20% das conexões fracas

##### 5.3 Ingestion (`memory/ingestion/`)
- **Pipeline** (`pipeline.go`): Pipeline de ingestão
- **Atomic Facts** (`atomic_facts.go`): Extração de fatos atômicos

**Features:**
- ✅ LLM-powered fact extraction
- ✅ Dual timestamp extraction
- ✅ Ambiguity resolution
- ✅ Confidence scoring

##### 5.4 Importance (`memory/importance/`)
- **Dynamic Scorer** (`dynamic_scorer.go`): Cálculo dinâmico de importância

**Features:**
- ✅ Keyword weighting
- ✅ Emotion weighting
- ✅ Urgency weighting
- ✅ Score clamping (0.0-1.0)

---

### 6. SWARM (Agentes Especializados)

**Localização:** `internal/swarm/`

Sistema de agentes especializados com routing inteligente.

#### Agentes Implementados:
1. **Emergency** (5 tools): Emergências médicas
2. **Clinical** (11 tools): Consultas, medicamentos, exames
3. **Productivity** (17 tools): Calendário, tarefas, lembretes
4. **Google** (15 tools): Busca, Gmail, Drive
5. **Wellness** (10 tools): Exercícios, meditação
6. **Entertainment** (32 tools): Música, vídeos, jogos
7. **External** (7 tools): Clima, notícias
8. **Kids** (7 tools): Jogos educativos

#### Features:
- ✅ Circuit breaker para falhas
- ✅ Retry logic
- ✅ Fallback strategies
- ✅ Tool validation
- ✅ Cellular division (auto-scaling)

---

### 7. CLINICAL (Análise Clínica)

**Localização:** `internal/clinical/`

Módulos especializados em análise clínica e detecção de riscos.

#### Módulos:
- **Crisis** (`crisis/`): Detecção e notificação de crises
- **Risk** (`risk/`): Detecção de riscos pediátricos
- **Silence** (`silence/`): Detecção de silêncio prolongado
- **Goals** (`goals/`): Tracking de metas terapêuticas
- **Notes** (`notes/`): Geração de notas clínicas
- **Synthesis** (`synthesis/`): Síntese de informações clínicas

---

### 8. LEGACY (Imortalidade Digital)

**Localização:** `internal/legacy/`

Sistema de legado digital pós-morte.

#### Features:
- ✅ Ativação pós-morte
- ✅ Gestão de herdeiros
- ✅ Personality snapshots
- ✅ Audit trail completo
- ✅ Consent granular

---

## 🔄 Fluxo de Dados

### Fluxo de Conversa (User → EVA)

```
1. USER fala via WebSocket
   ↓
2. SENSES/Signaling recebe áudio
   ↓
3. Gemini Live API transcreve
   ↓
4. AudioAnalysisService detecta emoção/urgência
   ↓
5. Session armazena contexto de áudio
   ↓
6. CORTEX/Brain processa fala
   ↓
7. Ingestion Pipeline extrai atomic facts
   ↓
8. HIPPOCAMPUS/Memory salva em PG + Qdrant + Neo4j
   ↓
9. Krylov comprime embeddings (3072D → 64D)
   ↓
10. Dynamic Scorer calcula importância
    ↓
11. CORTEX/Attention seleciona estratégia de resposta
    ↓
12. SWARM roteia para agente apropriado
    ↓
13. Agente executa tools e gera resposta
    ↓
14. Gemini sintetiza resposta final
    ↓
15. SENSES/Signaling envia áudio de volta
```

### Fluxo de Consolidação Noturna (REM)

```
1. Scheduler dispara às 3h da manhã
   ↓
2. REM Consolidator identifica memórias quentes
   ↓
3. Replay de memórias (simulação de sonho)
   ↓
4. Spectral Clustering agrupa memórias similares
   ↓
5. Abstração de comunidades em proto-conceitos
   ↓
6. Transferência episódica → semântica (Neo4j)
   ↓
7. Poda de 20% das conexões fracas
   ↓
8. Synaptogenesis cria novas conexões emergentes
```

---

## 📊 Métricas e Performance

### Memória
| Métrica | Valor |
|---------|-------|
| Recall@10 (Krylov) | 97% |
| Compressão | 3072D → 64D (48x) |
| Update time | 52µs/op |
| Storage reduction | ~80% (atomic facts + compression) |

### Atenção
| Métrica | Valor |
|---------|-------|
| Executive decision time | <100ms |
| Pattern interrupt accuracy | 85% |
| Affect stabilization rate | 78% |

### Consolidação
| Métrica | Valor |
|---------|-------|
| Nightly consolidation time | ~2h |
| Memory reduction | 70% |
| Synaptic pruning rate | 20% |
| Long-term recall improvement | +30% |

### Swarm
| Métrica | Valor |
|---------|-------|
| Total agents | 8 |
| Total tools | 104 |
| Circuit breaker threshold | 5 failures |
| Avg routing time | <50ms |

---

## 🛠️ Guia de Desenvolvimento

### Pré-requisitos
- Go 1.21+
- PostgreSQL 15+
- Qdrant 1.7+
- Neo4j 5.0+
- Redis 7.0+

### Setup Local

```bash
# Clone repositório
git clone https://github.com/your-org/eva-mind.git
cd eva-mind

# Instalar dependências
go mod download

# Configurar variáveis de ambiente
cp .env.example .env
# Editar .env com suas credenciais

# Rodar migrations
go run cmd/migrate/main.go up

# Iniciar servidor
go run cmd/integration_service/main.go
```

### Testes

```bash
# Todos os testes
go test ./...

# Testes de um módulo específico
go test -v ./internal/memory/

# Benchmarks
go test -bench=. -benchmem ./internal/memory/

# Coverage
go test -cover ./...
```

### Estrutura de Diretórios

```
eva-mind/
├── cmd/                    # Entry points
│   ├── integration_service/
│   └── migrate/
├── internal/               # Código privado
│   ├── brainstem/         # Infraestrutura
│   ├── senses/            # I/O
│   ├── cortex/            # Cognição
│   ├── hippocampus/       # Memória
│   ├── memory/            # Consolidação
│   ├── swarm/             # Agentes
│   ├── clinical/          # Análise clínica
│   └── legacy/            # Legado digital
├── pkg/                    # Código público
├── proto/                  # Protocol Buffers
├── migrations/             # SQL migrations
├── docs/                   # Documentação
└── tests/                  # Testes de integração
```

### Convenções de Código

- **Logging**: Use `zerolog` com níveis apropriados
- **Errors**: Sempre wrap errors com contexto
- **Context**: Sempre passe `context.Context` como primeiro parâmetro
- **Naming**: Use nomes descritivos, evite abreviações
- **Comments**: Documente funções públicas com godoc

### Deploy

```bash
# Build para produção
go build -o eva-mind cmd/integration_service/main.go

# Docker
docker build -t eva-mind:latest .
docker run -p 8080:8080 eva-mind:latest

# Kubernetes (Helm)
helm install eva-mind ./charts/eva-mind
```

---

## 📚 Referências

- **Neurociência**: Global Workspace Theory, Hierarchical Temporal Memory
- **Matemática**: Krylov Subspaces, Spectral Clustering, Fractal Dimension
- **Arquitetura**: Clean Architecture, Domain-Driven Design
- **Segurança**: LGPD, HIPAA, OAuth 2.0

---

**Última atualização:** 12/02/2026  
**Mantenedor:** Junior (Criador do Projeto EVA)  
**Licença:** Proprietária
