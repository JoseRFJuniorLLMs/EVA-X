# EVA-Mind — Auditoria de Memoria (8 Fases)

**Data**: 2026-03-10
**Auditor**: Claude Opus 4.6

---

## Contexto

A auditoria revelou que a EVA tinha infraestrutura cognitiva sofisticada
(12 adapters, Lacanian engine, Krylov subspace, etc.) mas as collections
de memoria estavam vazias. O cerebro existia mas nao formava memorias.

## Diagnostico

| Collection | Estado Antes | Problema |
|---|---|---|
| memories | VAZIA | brain.Service nunca criado no main.go |
| signifier_chains | VAZIA | UnifiedRetrieval nunca instanciado |
| eva_learnings | VAZIA | AutonomousLearner sem curriculo |
| eva_curriculum | VAZIA | Sem seed script |
| speaker_embeddings | ERRO | Dim 3072 vs ECAPA-TDNN 192 |
| stories | VAZIA | Sem seed script |
| eva_self_knowledge | VAZIA | Seed script existia mas nunca executado |

---

## FASE 1: Wire brain.Service no main.go (CRITICO)

**Problema**: `handleBrowserVoice()` e `handleEvaChat()` chamavam apenas
`evaMemory.StoreTurn()` que escreve em `eva_core` (grafo). O pipeline
completo de memoria (`brain.SaveEpisodicMemoryWithContext()` com embeddings
+ vector search + importance scoring) existia mas nunca era chamado.

**Solucao**:
- Criado `brain.Service` no `main.go` apos criacao dos adapters.
- Adicionado `brainService *brain.Service` ao struct `SignalingServer`.
- Nos handlers, apos `evaMemory.StoreTurn()`, chamada async:
  ```go
  go func() {
      memCtx := brain.MemoryContext{Emotion: "neutral", Urgency: "low"}
      s.brainService.SaveEpisodicMemoryWithContext(0, role, text, time.Now(), false, memCtx)
  }()
  ```

**Ficheiros**: main.go, browser_voice_handler.go, eva_handler.go

**Resultado**: Cada conversa gera embeddings 3072D e fica disponivel para KNN search.

---

## FASE 2: Fix database.DB routing (CRITICO)

**Problema**: `database.DB.Insert()` escrevia SEMPRE em `eva_mind`:
```go
const evaMindCollection = "eva_mind"  // HARDCODED
```

**Solucao**: Adicionados 3 novos metodos (retrocompativeis):
- `InsertTo(ctx, collection, table, content)` — insere em collection especifica
- `NQLIn(ctx, collection, nql, params)` — NQL em collection especifica
- `QueryByLabelIn(ctx, collection, label, extraWhere, params, limit)` — query em collection especifica

**Ficheiro**: internal/brainstem/database/db.go

**Resultado**: Dados podem agora ser escritos em qualquer collection.

---

## FASE 3: Seed do Curriculo para AutonomousLearner

**Problema**: `AutonomousLearner.Start()` rodava a cada 6h mas
`nextPendingTopic()` retornava nil porque `eva_curriculum` estava vazio.

**Solucao**: Criado `cmd/seed_curriculum/main.go` com 31 topicos:
- 8 topicos clinicos (malaria, microscopia, TB, etc.)
- 5 topicos de psicologia (Lacan, gestalt, CBT, etc.)
- 5 topicos de tecnologia (HNSW, graph DBs, AI, etc.)
- 5 topicos de wellness (mindfulness, nutricao, etc.)
- 4 topicos de linguistica (PT-AO, terminologia medica)
- 4 topicos de cultura (historia angola, medicina tradicional)

**Ficheiro**: cmd/seed_curriculum/main.go (NOVO)

---

## FASE 4: Fix speaker_embeddings dimensao

**Problema**: ECAPA-TDNN produz embeddings de 192 dimensoes mas a collection
estava configurada com 3072 dimensoes.

**Solucao**: Alterado `DefaultCollections()`:
```go
{Name: "speaker_embeddings", Dim: 192, Metric: "cosine"}
```

**Ficheiro**: internal/brainstem/infrastructure/nietzsche/client.go

**NOTA**: Na VM, collection existente com 3072D precisa ser dropada e recriada.

---

## FASE 5: Wire UnifiedRetrieval no main.go

**Problema**: `UnifiedRetrieval` nunca instanciado. `TrackSignifierChain()`
nunca chamado. `signifier_chains` vazia.

**Solucao**: Criado `UnifiedRetrieval` no `main.go` e passado ao `brain.Service`.

**Ficheiro**: main.go

**Resultado**: Cada mensagem chama `Prime()` -> `TrackSignifierChain()` ->
popula `signifier_chains`.

---

## FASE 6: Criar seed_stories

**Problema**: `stories` nao tinha seed script. WisdomService nao encontrava
historias para terapia narrativa.

**Solucao**: Criado `cmd/seed_stories/main.go` com 20 historias:
- 8 fabulas terapeuticas originais
- 2 koans zen
- 2 historias de Nasrudin
- 2 poemas de Rumi
- 3 contos africanos
- 2 fabulas de Esopo

Cada historia recebe embedding 3072D via Gemini API para busca semantica.

**Ficheiro**: cmd/seed_stories/main.go (NOVO)

---

## FASE 7: Retry com exponential backoff

**Problema**: Falhas de embedding e vector upsert eram silenciosas (log + continue).

**Solucao**:
- `memory_context.go`: Embedding generation com `retryPkg.DoWithResult(ctx, FastConfig(), ...)`
- `storage.go`: Vector upsert com `retryPkg.Do(ctx, FastConfig(), ...)`
- Fallback: Se embedding falha apos retries, memoria salva sem vetor.

**Ficheiros**: internal/cortex/brain/memory_context.go,
              internal/hippocampus/memory/storage.go

---

## FASE 8: RBAC e Sandbox

**RBAC**: NietzscheDB suporta 3 niveis (Admin/Writer/Reader) via env vars:
- `NIETZSCHE_API_KEY_ADMIN` — full access
- `NIETZSCHE_API_KEY_WRITER` — read + write
- `NIETZSCHE_API_KEY_READER` — read only
- Atualmente desabilitado (para ativar em producao apos testes).

**Sandbox**: `EVA_WORKSPACE_DIR` mantem-se como limite de seguranca.
Uma IA com acesso root sem restricoes e perigoso.

---

## Verificacao (VM)

```bash
# Verificar collections apos deploy:
curl -s http://localhost:8080/api/collections | python3 -c "
import sys,json
for c in json.load(sys.stdin):
  if c['name'] in ['memories','eva_learnings','speaker_embeddings',
                    'stories','signifier_chains','eva_self_knowledge','eva_mind']:
    print(f\"{c['name']:30s} | {c['node_count']:>8} nodes\")
"

# Teste end-to-end: enviar mensagem a EVA e verificar que:
# 1. eva_core tem novo no (StoreTurn — ja funciona)
# 2. memories tem novo no com embedding (SaveEpisodicMemoryWithContext — NOVO)
# 3. signifier_chains tem novo no (TrackSignifierChain — NOVO)
```

## Passos VM Pendentes

1. `go run cmd/seed_knowledge/main.go` — popular eva_self_knowledge
2. `go run cmd/seed_stories/main.go` — popular stories
3. `go run cmd/seed_curriculum/main.go` — popular eva_curriculum
4. Drop + recreate speaker_embeddings collection (192D)
5. Restart EVA para ativar brain.Service
6. (Opcional) Ativar RBAC em /etc/nietzsche.env
