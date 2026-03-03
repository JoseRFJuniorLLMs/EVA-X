# EVA-X Sprint 1 — Auditoria & Cleanup

**Data**: 2026-03-03
**Status**: PENDENTE
**Total issues**: 72 bugs/legacy + 5 optimizacoes NietzscheDB

---

## PRIORIDADE 1 — CRITICAL (9 issues)

### Seguranca (AUTH = ZERO)
- [ ] **M1**: AuthMiddleware existe mas NUNCA e aplicado — todos endpoints abertos
  - File: `main.go` (routing section)
  - Fix: Aplicar `AuthMiddleware` no subrouter `/api/v1/`
- [ ] **R1**: Dados pacientes (`/api/v1/idosos/*`) sem autenticacao — violacao LGPD
  - File: `main.go` linhas 699-702
  - Fix: Proteger com AuthMiddleware
- [ ] **C1-CONFIG**: JWT_SECRET pode ser vazio — tokens forjados com key vazia
  - File: `internal/brainstem/config/config.go` linha 209
  - Fix: `getEnvRequired` deve fazer `log.Fatal` se vazio, nao warning
- [ ] **H1**: OAuth HMAC secret hardcoded `"eva-oauth-state-secret-2026"`
  - File: `internal/brainstem/config/config.go` linha 224
  - Fix: Tornar obrigatorio via env var

### Race Conditions (Voice Layer)
- [ ] **E1**: `atomic.Store(TurnID, TurnID+1)` — read nao-atomico + store atomico
  - File: `internal/voice/handler.go` linha 248
  - Fix: `atomic.AddUint64(&audioSess.TurnID, 1)`
- [ ] **E2**: WebSocket writes concorrentes sem mutex
  - File: `internal/voice/handler.go` linha 209
  - Fix: Adicionar `sync.Mutex` para `conn.WriteJSON`

### Legacy PostgreSQL (Core)
- [ ] **L1**: `main.go` importa `database/sql` + `lib/pq`, abre conexao PostgreSQL
  - File: `main.go` linhas 8, 81, 199-216
- [ ] **L2**: `legacy_compat.go` (240 linhas) — SQL raw PostgreSQL
  - File: `internal/brainstem/database/legacy_compat.go`
- [ ] **L3**: `device_tokens.go` (395 linhas) — push tokens 100% PostgreSQL
  - File: `internal/brainstem/push/device_tokens.go`

---

## PRIORIDADE 2 — HIGH (20 issues)

### Bugs de Runtime
- [ ] **E3**: Goroutine leak em `log_handler.go` — ping goroutine nunca sai
- [ ] **E4**: WebSocket writes concorrentes em `log_handler.go` (ping vs scanner)
- [ ] **E5**: `escalateToEmergency` nil pointer em `GetConnection()` — crash durante emergencia
  - File: `cascade_handler.go` linha 182
- [ ] **E6**: `responseAccum` strings.Builder sem sync em `browser_voice_handler.go`
- [ ] **E7**: `evaResponses` slice append sem sync em `eva_handler.go`

### SQL Injection
- [ ] **A1**: `sql_adapter.go` `DropTable()` — table name nao sanitizado
  - File: `internal/brainstem/infrastructure/nietzsche/sql_adapter.go` linha 59
- [ ] **A2**: `sql_adapter.go` `Insert()` — columns e values nao escapados
  - File: `internal/brainstem/infrastructure/nietzsche/sql_adapter.go` linhas 78-85

### Performance
- [ ] **A3**: N+1 gRPC calls em `vector_adapter.go` Search() — 1 KNN + N GetNode
  - File: `internal/brainstem/infrastructure/nietzsche/vector_adapter.go`

### Config Hardcoded
- [ ] **H2**: `IdosoID=0` default 1 (admin) em `tools_handler.go` — privilege escalation
- [ ] **H3**: `IdosoID=0` default 1 (admin) em `video_handlers_helper.go`
- [ ] **C2-CONFIG**: `Validate()` exige DATABASE_URL (legacy) mas nao valida NietzscheGRPCAddr

### Legacy PostgreSQL (Extenso)
- [ ] **L4**: `db.go` — dual personality PostgreSQL + NietzscheDB
- [ ] **L5**: `idosos_handler.go` linhas 29-32, 74-77 — PostgreSQL queries
- [ ] **L6**: `idosos_handler.go` linha 117 — PostgreSQL UPDATE
- [ ] **L7**: `cascade_handler.go` linhas 23-28, 178 — cuidadores + alertas via PostgreSQL

### Security
- [ ] **M2**: WebSocket upgraders `CheckOrigin: return true` — aceita qualquer origin
- [ ] **M3**: `/ws/logs` sem auth — stream de logs com CPFs e session IDs
- [ ] **R2**: `/metrics` Prometheus sem auth
- [ ] **R3**: `/mcp` sem auth — acesso a embeddings e vector search

---

## PRIORIDADE 3 — MEDIUM (26 issues)

### Dados Perdidos Silenciosamente
- [ ] **E17**: `StoreTurn` em `eva_handler.go` usa ctx cancelado — conversas perdidas
- [ ] **E18**: `StoreTurn` em `browser_voice_handler.go` usa ctx cancelado
- [ ] **E19**: `llm/service.go` ignora error de `json.Unmarshal` — respostas vazias
- [ ] **E20**: `llm/service.go` ignora error de `json.Marshal` e `http.NewRequest`

### Adapter Bugs
- [ ] **A4**: `client.Search()` ignora parametro `query` — devolve N primeiros nodes
- [ ] **A5**: `audio_buffer.go` `cleanupLoop()` sem context — goroutine leak
- [ ] **A6**: Query usa `$_label` (underscore) — NQL nao suporta

### Panics
- [ ] **E13**: `oauth/handlers.go` `cpf[:3]` panic se CPF < 3 chars
- [ ] **E14**: `oauth/handlers.go` mesmo panic noutro handler

### WebSocket
- [ ] **E15**: `video_websocket_handler.go` RouteSignal sem write mutex
- [ ] **E16**: `handleVideoCascade` bloqueia goroutine ate 7.5 minutos sem ctx.Done()

### Legacy PostgreSQL (Mais)
- [ ] **L8**: `browser_voice_handler.go` — medication schedules via PostgreSQL
- [ ] **L9**: `main.go` — 9 servicos inicializados com `db.Conn`
- [ ] **M8**: Multi-tenancy isolation depende de PostgreSQL

### Config/Security
- [ ] **E11**: Context key string `"user"` — risco de colisao
- [ ] **E12**: Auth handlers sem Content-Type header
- [ ] **C3**: Mesmo JWT_SECRET para access e refresh tokens
- [ ] **C4**: gRPC sem TLS (`RequireTransportSecurity()` = false)
- [ ] **H4**: Email hardcoded `web2ajax@gmail.com` como SMTPFromEmail
- [ ] **H5**: Audio buffer hardcoded a `eva_core`
- [ ] **M5**: Token extraction case-sensitive ("Bearer " vs "bearer ")
- [ ] **M6**: Refresh token no body em vez de HttpOnly cookie
- [ ] **M7**: CORS headers parciais em origens nao permitidas
- [ ] **E21**: Potential deadlock em `video_websocket_handler.go` RegisterClient

### Dead Code
- [ ] **D1**: `audioBuffer`, `algoAdapter`, `cacheAdapter` criados mas suprimidos com `_ =`
- [ ] **D2**: `UpdateLastLogin` e no-op — body vazio, retorna nil

---

## PRIORIDADE 4 — LOW (17 issues)

- [ ] **E8**: `strings.Title` deprecated em `voice_change_helper.go`
- [ ] **E9**: `handleChat` nao drena r.Body
- [ ] **E10**: `defer r.Body.Close()` depois de ReadAll em `media_handler.go`
- [ ] **E22**: Missing Content-Type em error response `idosos_handler.go`
- [ ] **E23**: CPF e token via URL query params (security log exposure)
- [ ] **E24**: `defer rows.Close()` dentro de if — DB connection mantida durante voice session
- [ ] **E25**: `personality_update_helper.go` dead code, nil pointer se chamado
- [ ] **E26**: Video sessions nunca expiram — memory leak
- [ ] **E27**: `research_handler.go` permite file path arbitrario (arbitrary file write)
- [ ] **L10**: `queries.go` importa `database/sql` desnecessariamente
- [ ] **D3**: `rand.Seed` deprecated (Go 1.20+)
- [ ] **D4**: `IsValidationError()` sempre retorna false
- [ ] **D5**: `contains()`/`indexOf()` reimplementam stdlib
- [ ] **D6**: `ServiceDomain` nunca configurado — URLs Twilio quebrados
- [ ] **H6**: `maxBridgeClients = 50` hardcoded
- [ ] **H7**: `CausalHistory` maxDepth hardcoded 10
- [ ] **H8**: Claude model hardcoded `claude-sonnet-4-6`
- [ ] **A7**: Bubble sort O(n^2) em `vector_adapter.go`
- [ ] **A8**: Medication safety check so cobre `2x/dia`
- [ ] **M9**: `ValidateFirebaseToken` retorna true quando Firebase nao configurado
- [ ] **R5**: OAuth routes comentadas — oauthSvc criado mas nao usado
- [ ] **R6**: Rota legacy `/calls/stream/{agendamento_id}` duplicada

---

## OPTIMIZACOES NietzscheDB (Remover duplicacao)

### Remover Agora
- [ ] **OPT-1**: Remover `CacheStore` de `audio_buffer.go` (linhas 201-299)
  - Duplica `CacheAdapter` — mesmas RPCs CacheSet/CacheGet
  - Migrar callers para `CacheAdapter`

### Simplificar Quando Go SDK Atualizado
- [ ] **OPT-2**: `vector_adapter.go` — remover `cosineSimilarity()`, `sortByScoreDesc()`, `SearchFiltered()`
  - Prereq: Adicionar KnnFilter ao Go SDK (`KnnFilterMatch`/`KnnFilterRange`)
  - Depois: substituir client-side re-ranking por single KnnSearch filtrado
- [ ] **OPT-3**: `vector_adapter.go` Search() — eliminar N+1 GetNode pattern
  - Prereq: Mesmo (KnnFilter com user_id push-down)

### Simplificar Futuro
- [ ] **OPT-4**: `named_vectors_adapter.go` (665 linhas) — sub-collection workaround
  - Prereq: Expor Named Vector RPCs no nietzsche.proto
  - Depois: Reduzir a ~100 linhas de thin SDK calls
- [ ] **OPT-5**: Resolver duplicacao `Zaratustra` e `Sleep` entre EVA e NietzscheDB server
  - Server ja roda Zaratustra a cada 10min — EVA chama de novo a cada 6h com params diferentes
  - Mesma duplicacao com Sleep
  - Fix: Unificar num so ponto (server OU EVA), nao ambos
  - **NOTA**: `SynapticPruning` NAO e duplicacao — EVA prune edges por idade+ativacao (comportamental), NietzscheDB prune nodes por energia (disabled atualmente). Sao coisas diferentes.

---

## Estatisticas Legado PostgreSQL

**Ficheiros com SQL raw PostgreSQL (a migrar para NietzscheDB):**

| Ficheiro | Linhas SQL | Funcoes Afetadas |
|----------|:----------:|------------------|
| `legacy_compat.go` | ~240 | GetIdosoByID, GetCallContext, GetPendingCalls, advisory locks, agendamentos |
| `device_tokens.go` | ~395 | RegisterToken, GetTokens, DeactivateToken, CleanExpired, Stats |
| `idosos_handler.go` | ~30 | handleGetIdosoByCpf, handleGetIdoso, handleSyncTokenByCpf |
| `cascade_handler.go` | ~25 | handleVideoCascade, escalateToEmergency |
| `browser_voice_handler.go` | ~15 | medication schedule loading |
| `main.go` | ~20 | 9 servicos recebem db.Conn |
| `multitenancy/middleware.go` | ~10 | ValidateIsolation |
| **TOTAL** | **~735** | **~20 funcoes** |

**Servicos em main.go que recebem db.Conn (a migrar):**
1. habitTracker
2. spacedSvc
3. superhumanSvc
4. autonomousLearner
5. selfAwareSvc
6. memOrchestrator
7. researchEng
8. mcpServer
9. fhirHandler

---

## O Que NAO Precisa Mudar (Validado)

A camada de adapters NietzscheDB esta **bem desenhada**:
- `client.go` — wrapper essencial com NQL rewriter
- `graph_adapter.go` — delegacao correcta com collection defaults
- `manifold_adapter.go` — delegacao pura (Synthesis, CausalChain, KleinPath)
- `algo_adapter.go` — orquestracao paralela de algoritmos
- `nql_rewriter.go` — preenche gap do NQL (4 tipos built-in)
- `narrative_adapter.go` — composicao (NARRATE + PageRank + Louvain)
- `wiederkehr_adapter.go` — templates EVA-especificos
- `sensory_adapter.go` — convenience por modalidade
- `security_adapter.go` — compliance LGPD/HIPAA
- `cdc_listener.go` — bridge WebSocket para Perspektive 3D
- `backup_service.go` — scheduling automatico
- Servicos cortex/evolution — orquestracao EVA-especifica (dream eval, sleep, zaratustra)
