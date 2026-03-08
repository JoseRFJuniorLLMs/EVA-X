# Projeto de Migracao: EVA-Mind -> EVA (NietzscheDB)

**Data:** 2026-02-20
**Escopo:** Portar features novas do EVA-Mind para o EVA (que usa NietzscheDB)
**Ancestral comum:** commit `d51b2b9` (seed 12 new capabilities)

---

## Resumo Executivo

O EVA foi forkado do EVA-Mind em 2026-02-20 e migrado de NietzscheDB+NietzscheDB+NietzscheDB para NietzscheDB.
Desde o fork, o EVA-Mind recebeu **11 commits** com features e fixes criticos que o EVA ainda nao tem.

**Total de mudancas a portar:** 11 commits, ~15 arquivos, ~700 linhas novas
**Complexidade NietzscheDB:** Baixa (apenas 1 funcao precisa de adaptacao de Cypher -> NQL)

---

## Commits a Migrar (ordem de prioridade)

### PRIORIDADE CRITICA (quebra funcionalidade de voz)

#### 1. Enable function calling for Gemini native audio (`031a4b2`)
- **Arquivo:** `internal/cortex/gemini/client.go`
- **Mudanca:** Descomentar envio de tools no setup WebSocket do Gemini Live API
- **Impacto:** Sem isso, NENHUMA das 150+ tools funciona na sessao de voz
- **NietzscheDB:** Nenhuma adaptacao necessaria
- **Linhas:** -29, +13

#### 2. Remove SendText after tool execution (`1cb4b83`)
- **Arquivo:** `browser_voice_handler.go`
- **Mudanca:** Remover `SendText()` apos execucao de tool (causa policy violation 1008)
- **Impacto:** Sem isso, o modelo Gemini native audio CRASHA apos qualquer tool
- **NietzscheDB:** Nenhuma adaptacao necessaria
- **Linhas:** -9, +5

#### 3. Set idosoID in creator mode (`52e97b9`)
- **Arquivo:** `browser_voice_handler.go`
- **Mudanca:** Buscar `idosoID` do banco antes do setup do creator
- **Impacto:** Sem isso, tools nunca disparam para o criador (idosoID = 0)
- **NietzscheDB:** Nenhuma adaptacao necessaria
- **Linhas:** +5

---

### PRIORIDADE ALTA (features importantes)

#### 4. Inject 33 capabilities into voice prompt (`cfc6ea0`)
- **Arquivos:**
  - `internal/cortex/lacan/unified_retrieval.go` (+60 linhas)
  - `internal/tools/handlers.go` (-9, +2)
- **Mudanca:**
  - Novo campo `Capabilities string` no `UnifiedContext`
  - Nova funcao `getCapabilities()` que busca capabilities do CoreMemory
  - Injecao das capabilities no `buildIntegratedPrompt()` para TODOS os modos
  - Remover gate de debug dos tools (liberar todos em producao)
- **Impacto:** EVA para de dizer que "so sabe fazer coisas basicas"
- **ADAPTACAO NietzscheDB:**
  ```
  EVA-Mind (NietzscheDB Cypher):
    MATCH (m:CoreMemory) WHERE m.memory_type = 'capability' RETURN m.content ORDER BY m.id

  EVA (NietzscheDB NQL):
    u.graph.ExecuteNQL(ctx, "MATCH (m:CoreMemory) WHERE m.memory_type = 'capability' RETURN m", nil, "eva_core")
  ```
  - Trocar `u.NietzscheDB.ExecuteRead()` por `u.graph.ExecuteNQL()`
  - Extrair `content` do resultado NQL (formato diferente do NietzscheDB records)

#### 5. control_ui tool + media cards + identity injection (`e18431e`)
- **Arquivos:**
  - `internal/cortex/gemini/tools.go` (+39 linhas) - nova tool `control_ui`
  - `internal/cortex/gemini/tools_client.go` (+17 linhas) - handler `control_ui`
  - `browser_voice_handler.go` (+39 linhas) - identity injection + tool_event WS
  - `internal/tools/handlers.go` (+18 linhas) - handler `control_ui`
  - `internal/cortex/self/core_memory_engine.go` (+3 linhas) - 3 novos seeds
- **Mudanca:** Nova tool que controla a UI do browser (navegar paginas, trocar modo, etc.)
- **NietzscheDB:** Seeds de capabilities ja usam `MergeNode` no EVA - so adicionar strings

#### 6. nivel_cognitivo + tom_voz to voice prompt (`0e86e50`)
- **Arquivos:**
  - `internal/cortex/lacan/unified_retrieval.go` (+55, -10) - ler e injetar no prompt
  - `migrations/044_creator_profile_update.sql` (novo, 12 linhas)
- **Mudanca:** Ler `nivel_cognitivo` e `tom_voz` do banco e adaptar linguagem do prompt
- **NietzscheDB:** Nenhuma adaptacao (dados vem do NietzscheDB, nao do grafo)

#### 7. Google OAuth2 full access + Gmail watcher (`39772fd`)
- **Arquivos:**
  - `internal/brainstem/oauth/handlers.go` (+225 linhas refatoradas)
  - `internal/brainstem/oauth/service.go` (+85 linhas)
  - `internal/brainstem/database/oauth_queries.go` (+62 linhas)
  - `internal/motor/gmail/watcher.go` (NOVO, 188 linhas)
  - `internal/brainstem/config/config.go` (+6 linhas)
  - `main.go` (+28 linhas - rotas OAuth + Gmail watcher)
  - `migrations/045_google_email_column.sql` (NOVO, 9 linhas)
- **Mudanca:** OAuth expandido (Gmail, Drive, Calendar, Contacts), state HMAC com CPF, Gmail watcher polling 2min
- **NietzscheDB:** Nenhuma adaptacao (tudo NietzscheDB + Google APIs)

---

### PRIORIDADE MEDIA (melhorias)

#### 8. Real-time web search + swarm resilience (`26e8c0a`)
- **Arquivos:**
  - `internal/tools/handlers.go` (+74, -19) - productionTools whitelist + real Google Search
  - `internal/swarm/orchestrator.go` (+3, -3) - circuit breaker 10/15s
- **Mudanca:** Google Search grounding real (nao stub), scholar timeout 60s
- **NietzscheDB:** Nenhuma adaptacao necessaria

#### 9. web_realtime capability seed (`06043e4`)
- **Arquivo:** `internal/cortex/self/core_memory_engine.go` (+1 linha)
- **Mudanca:** Adicionar `cap_web_realtime` na lista de seeds
- **NietzscheDB:** Ja usa `MergeNode` - so adicionar string

---

### PRIORIDADE BAIXA (podem ser pulados)

#### 10. Audio transcription enable/remove (`3688db6` + `03ddb34`)
- **Efeito liquido: ZERO** (adicionou e depois removeu)
- **Acao:** PULAR - nao precisa migrar

#### 11. TriDB benchmarks (`22d5ecb`)
- **Arquivos:** `internal/benchmark/tridb_benchmark.go` (339 linhas), testes (606 linhas)
- **Acao:** OPCIONAL - benchmarks internos, nao afetam producao

---

## Mapeamento NietzscheDB -> NietzscheDB

Apenas **1 funcao** precisa de adaptacao real:

### `getCapabilities()` em `unified_retrieval.go`

**EVA-Mind (NietzscheDB):**
```go
func (u *UnifiedRetrieval) getCapabilities(ctx context.Context) string {
    if u.NietzscheDB == nil { return "" }
    query := `MATCH (m:CoreMemory) WHERE m.memory_type = 'capability' RETURN m.content AS content ORDER BY m.id`
    records, err := u.NietzscheDB.ExecuteRead(ctx, query, nil)
    // ...iterate records, record.Get("content")
}
```

**EVA (NietzscheDB):**
```go
func (u *UnifiedRetrieval) getCapabilities(ctx context.Context) string {
    if u.graph == nil { return "" }
    nql := `MATCH (m:CoreMemory) WHERE m.memory_type = 'capability' RETURN m`
    result, err := u.graph.ExecuteNQL(ctx, nql, nil, "eva_core")
    // ...iterate result nodes, extract "content" from properties
}
```

**Mudancas:**
1. `u.NietzscheDB` -> `u.graph` (ja existe no struct do EVA)
2. `ExecuteRead()` -> `ExecuteNQL()` com collection `"eva_core"`
3. Parsing de resultado: `record.Get("content")` -> extrair de `node.Properties["content"]`

---

## Novos Arquivos a Criar

| Arquivo | Origem | Linhas | Adaptacao |
|---------|--------|--------|-----------|
| `internal/motor/gmail/watcher.go` | EVA-Mind | 188 | Nenhuma (nao toca DB de grafo) |
| `migrations/044_creator_profile_update.sql` | EVA-Mind | 12 | Nenhuma (NietzscheDB puro) |
| `migrations/045_google_email_column.sql` | EVA-Mind | 9 | Nenhuma (NietzscheDB puro) |

---

## Ordem de Execucao Recomendada

### Fase 1: Fixes Criticos de Voz (30 min)
1. Portar `031a4b2` - enable function calling (gemini/client.go)
2. Portar `1cb4b83` - remove SendText (browser_voice_handler.go)
3. Portar `52e97b9` - set idosoID creator (browser_voice_handler.go)

### Fase 2: Capabilities + Identity (1h)
4. Portar `cfc6ea0` - getCapabilities() com adaptacao NietzscheDB
5. Portar `e18431e` - control_ui tool + seeds
6. Portar `06043e4` - web_realtime seed

### Fase 3: Cognitive + Search (1h)
7. Portar `0e86e50` - nivel_cognitivo + tom_voz
8. Portar `26e8c0a` - real-time web search + swarm

### Fase 4: OAuth + Gmail (1h30)
9. Portar `39772fd` - Google OAuth2 full + Gmail watcher
10. Criar migrations 044 e 045
11. Registrar rotas OAuth e iniciar Gmail watcher no main.go

---

## Riscos e Atencao

| Risco | Mitigacao |
|-------|-----------|
| NQL syntax diferente de Cypher | Testar `getCapabilities()` isoladamente antes de integrar |
| Conflitos no `main.go` | EVA ja tem OAuth basico wired; expandir, nao duplicar |
| `browser_voice_handler.go` divergiu bastante | Aplicar patches cirurgicamente, nao copiar arquivo inteiro |
| Migrations 044/045 podem conflitar com numeracao EVA | Verificar ultimo numero de migration no EVA antes de criar |

---

## Verificacao Pos-Migracao

- [ ] `go build ./...` compila sem erros
- [ ] Tools aparecem no setup WebSocket do Gemini
- [ ] Voz funciona sem crash de policy violation
- [ ] Creator mode detecta e executa tools
- [ ] Capabilities aparecem no system prompt (testar via /ws/eva)
- [ ] control_ui tool registrada e funcional
- [ ] nivel_cognitivo/tom_voz injetados no prompt
- [ ] Google OAuth2 flow completo (authorize -> callback -> status)
- [ ] Gmail watcher inicia e poll funciona
- [ ] Web search real via Google Search grounding
- [ ] Swarm circuit breaker com parametros 10/15s
