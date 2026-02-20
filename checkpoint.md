# CHECKPOINT - EVA-Mind
**Data:** 2026-02-20
**AVISO: O modelo gemini-2.5-flash-native-audio-preview-12-2025 e INTOCAVEL**

---

## O QUE E O PROJETO
Backend Go principal do ecossistema EVA - sistema de voz IA em tempo real para idosos. O projeto MAIS complexo do ecossistema: 100K+ LOC em Go, com sistemas de memoria super-humanos, personalidade Enneagram/Lacan, 150+ tools, 12 agentes de swarm, e integracao com Neo4j, Qdrant, Redis, PostgreSQL.

**Tech Stack:** Go 1.23+, Gorilla Mux/WebSocket, Gemini SDK, PostgreSQL+pgvector, Neo4j (2 instancias), Qdrant, Redis, Firebase, Twilio, Prometheus, gRPC

---

## O QUE FUNCIONA EM PRODUCAO
- `/ws/browser` - Voz bidirecional mobile (PCM 16kHz/24kHz) com Gemini Native Audio
- `/ws/eva` - Chat texto web (Malaria-Angolar) com memoria meta-cognitiva Neo4j
- `/api/chat` - REST stateless para malaria
- Video WebRTC (signaling WebSocket + REST)
- Scheduler background (medicamentos, alertas)
- Escalation Service (Push -> Email -> SMS)
- MemoryStore (PG + Neo4j + Qdrant) com graceful degradation
- 12 subsistemas de memoria super-humana
- Personality Service (Big Five + Enneagram)
- 150+ tools via toolsHandler
- Swarm Orchestrator (12 agentes especializados)
- Autonomous Learner (background cada 6h)
- MCP Server (/mcp endpoint, 44 tools)
- OAuth Google (Gmail, Calendar, Drive)
- RAM Engine (interpretacoes + validacao historica via Claude)
- Situational Modulator (modulacao contextual de personalidade)
- FHIR R4 Adapter (/api/v1/fhir — interoperabilidade HL7)
- Krylov Subspace Memory (compressao 1536D → 64D, porta HTTP 50052)
- Memory Orchestrator (pipeline FDPN → Krylov → REM consolidation)
- Memory Scheduler (REM consolidation 3AM + Krylov maintenance 6h)
- Research Engine (pesquisa clinica longitudinal com anonimizacao LGPD)
- Emergency Swarm com notificacoes REAIS (Push + Email + SMS)
- Multi-LLM Service (Claude, GPT, DeepSeek como LLMs secundarios)
- CoreMemoryEngine (identidade pessoal da EVA, Neo4j :7688)
- Global Workspace (Baars' Cognitive Theory of Consciousness)

---

## SUBSISTEMAS INTEGRADOS (antes listados como "faltantes" — CORRIGIDO)
1. **RAM Engine** — JA estava integrado em main.go (secao 7.13) com adapters LLM/Embed/Retrieval
2. **Situational Modulator** — JA estava instanciado (secao 7.12), porem sem deps (nil, nil)
3. **FHIR Adapter** — JA estava registrado (linhas 654-656 do main.go original)
4. **Krylov Memory Manager** — INTEGRADO em 2026-02-20 (secao 7.8.1)
5. **Memory Orchestrator** — INTEGRADO em 2026-02-20 (secao 7.14, pipeline completo FDPN → Krylov → REM)
6. **Memory Scheduler** — INTEGRADO em 2026-02-20 (goroutine background, REM 3AM + Krylov 6h)
7. **Krylov HTTP Bridge** — INTEGRADO em 2026-02-20 (secao 7.15, porta 50052)
8. **Research Engine** — INTEGRADO em 2026-02-20 (secao 7.16, instanciado com db.Conn)

---

## BUGS CORRIGIDOS (2026-02-20)
1. **CREDENCIAIS NO GIT** — .env ja estava no .gitignore, CLAUDE_API_KEY esvaziada do .env
2. **Agendamentos NUNCA vistos** — CORRIGIDO: adicionado 'nao_atendido','aguardando_retry' ao status IN em unified_retrieval.go e browser_voice_handler.go
3. **Tipo agendamento errado** — CORRIGIDO: 'medicamento' → 'lembrete_medicamento' em unified_retrieval.go, aceita ambos formatos
4. **Coluna errada** — CORRIGIDO: medicamento_confirmado → medicamento_tomado em actions.go:221
5. **ORDER BY em coluna NULL** — CORRIGIDO: updated_at → atualizado_em em unified_retrieval.go
6. **Dados clinicos SEM criptografia** — PENDENTE (requer refactor maior, LGPD Art. 46)
7. **Emergencias so fazem log** — CORRIGIDO: AlertFamily real via dependency injection (push + email + SMS), wired em main.go com actions.AlertFamilyWithSeverity
8. **Race condition** — CORRIGIDO: per-connection write mutex (writeMu sync.Mutex) em video_websocket_handler.go
9. **dados_tarefa formato incompativel** — CORRIGIDO: fallback para formato legacy (description/medicamento keys) em unified_retrieval.go
10. **Dados sujos** — CORRIGIDO: sanitizeMedicalConditions() filtra termos invalidos (MORTO, FEIO, etc) em websocket.go

---

## BUGS PENDENTES
1. **Dados clinicos SEM criptografia** — violacao LGPD Art. 46 (requer crypto at-rest para PG + Neo4j)
2. **Situational Modulator sem deps** — instanciado com (nil, nil), precisa dos servicos reais
3. **RAM Engine sem FeedbackLoop** — FeedbackLoop nil (precisa adapters HebbianRealTime + Database)
4. **RAM Engine sem GraphStore** — HistoricalValidator sem validacao temporal (precisa adapter Neo4j)
---

## AUDIT FIXES (2026-02-20)

### Dead Tools Bridge (handlers.go)
- **Problema:** 7 tools declaradas no Gemini (`open_camera_analysis`, `manage_health_sheet`, `create_health_doc`, `request_ride`, `get_health_data`, `run_sql_select`, `change_voice`) nao tinham case no switch do `ExecuteTool`
- **Fix:** Adicionado `SwarmRouter` interface ao `ToolsHandler` com fallthrough no default case que roteia para o swarm orchestrator
- **Arquivo:** `internal/tools/handlers.go`

### Prometheus /metrics
- **Problema:** `/metrics` endpoint listado no README mas nunca registrado no router
- **Fix:** `router.Handle("/metrics", monitoring.PrometheusHandler())` adicionado em main.go
- **Arquivo:** `main.go`

### Research Engine REST Routes
- **Problema:** `researchEng` instanciado mas atribuido a `_ = researchEng` (sem uso)
- **Fix:** Criado `research_handler.go` com 4 endpoints REST, registrado via `research.RegisterRoutes(v1, researchEng)`
- **Endpoints:** POST/GET `/api/v1/research/cohorts`, GET `/{id}/report`, POST `/{id}/export`

### README Corrections
- `/self/*` corrigido para `/api/v1/self/*`
- Adicionados `/ws/eva` e `/ws/logs` nas rotas WebSocket
- Contagem de swarm agents: 10 corrigido para 12 (adicionados `scholar` e `selfawareness`)
- Adicionada secao de rotas Research Engine
- Go version: 1.21 corrigido para 1.24

### Self-Knowledge Seeds (core_memory_engine.go)
- Adicionados 6 seeds: Krylov Memory, Memory Orchestrator, Memory Scheduler, Research Engine, Scholar Agent, Selfawareness Agent
- Corrigido `cap_memoria_avancada`: removido "Zettelkasten e topologia persistente" (nao wired em main.go)

### gRPC Server (DECISAO)
- `internal/memory/grpc_server.go` existe mas nao e iniciado — HTTP Bridge na porta 50052 serve o mesmo proposito
- **Decisao:** Nao iniciar gRPC (incompleto — nenhum proto service registrado). Manter apenas HTTP Bridge.

---

## DEPENDENCIAS PRINCIPAIS
gorilla/mux 1.8.1, gorilla/websocket 1.5.3, google/generative-ai-go 0.20.1, lib/pq 1.10.9, neo4j-go-driver/v5 5.28.4, qdrant/go-client 1.15.2, redis/go-redis/v9 9.17.2, firebase.google.com/go/v4 4.15.2, twilio-go 1.29.0, gonum.org/v1/gonum (Krylov linear algebra)

---

## INFRAESTRUTURA NECESSARIA
- PostgreSQL 16 + pgvector
- Neo4j (porta 7687 - pacientes) + Neo4j (porta 7688 - memoria EVA)
- Qdrant (porta 6334 gRPC)
- Redis (porta 6379)
- Firebase serviceAccountKey.json
- Krylov HTTP Bridge (porta 50052)
