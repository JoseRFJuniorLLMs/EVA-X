# Análise de Gaps Técnicos: EVA-Mind vs EVA-Memory

**Data:** 12/02/2026
**Escopo:** `internal/memory`, `internal/cortex/brain/memory.go`, `api_server.py`

## Veredito Geral
O código atual do EVA-Mind (`memory.go`) implementa um fluxo de **ingestão direta**:
`Texto -> Embedding -> Postgres/Qdrant/Neo4j`.

Não existem as camadas intermédias de processamento semântico (Extração de Factos, Versionamento, Grounding Temporal) descritas nos documentos do EVA-Memory.

## Análise Ponto a Ponto (Os 5 Gaps)

### 1. Atomic Memories (Fatos Atómicos)
*   **Status:** 🔴 **Ausente**
*   **Código Atual:** Em `internal/cortex/brain/memory.go` (linha 43), a função `SaveEpisodicMemory` recebe o `content` bruto e o envia diretamente para embedding e armazenamento.
*   **O que falta:** Um passo de processamento anterior que usa um LLM para quebrar o `content` em fatos isolados (ex: "O usuário gosta de azul") antes de gerar embeddings.

### 2. Relational Versioning (Knowledge Chains)
*   **Status:** 🔴 **Ausente**
*   **Código Atual:** No `memory.go`, a inserção no Postgres (linha 78) e Qdrant (linha 113) é um `INSERT` ou `UPSERT` cego. No Neo4j (linha 173), usa `StoreCausalMemory`, mas não há lógica de leitura prévia para verificar contradições (`UPDATES` vs `EXTENDS`).
*   **O que falta:** Lógica de "Query before Write". Antes de salvar, buscar memórias similares, pedir ao LLM para classificar a relação, e então salvar com a aresta correta.

### 3. Temporal Grounding (Dual Timestamp)
*   **Status:** 🔴 **Ausente**
*   **Código Atual:** `memory.go` usa `time.Now()` (linha 125, 169) para definir o timestamp. O campo `created_at` é a única referência temporal.
*   **O que falta:** Um campo `event_date` nos structs de memória e no schema do banco. Extração via LLM ("Quando isso aconteceu?") durante a ingestão.

### 4. Hybrid Search (Memory + Source Chunk)
*   **Status:** 🔴 **Ausente**
*   **Código Atual:** O `SaveEpisodicMemory` salva o conteúdo original no campo `content` do Qdrant (linha 124).
*   **Nuance:** Embora o conteúdo esteja lá, ele não está estruturado como "Source Chunk" vinculado a "Atomic Memories" porque não existem memórias atômicas. O retrieval atual traz o chunk inteiro.
*   **O que falta:** Estruturar o armazenamento para separar `Facto Atómico` (para busca vetorial precisa) de `Source Chunk` (para contexto rico).

### 5. Session-Based Ingestion
*   **Status:** 🔴 **Ausente**
*   **Código Atual:** `ProcessUserSpeech` (linha 22) processa cada input de fala individualmente e dispara `SaveEpisodicMemory` em uma goroutine (`go s.SaveEpisodicMemory`).
*   **O que falta:** Um buffer que acumule interações e processe o contexto da sessão em lote quando houver silêncio ou encerramento, para resolver correferências ("ele disse" -> "o João disse").

## Pontos Fortes do Código Atual (Para não perder)
1.  **Triple Store Robusto:** A integração simultânea (Postgres + Qdrant + Neo4j) já está funcional em `SaveEpisodicMemory`.
2.  **Resiliência:** Há lógica de retry no Qdrant e fallbacks no Postgres (linhas 86-97).
3.  **Hooks de Personalidade:** A atualização assíncrona da personalidade (linha 141) é um diferencial arquitetural importante.
4.  **Krylov Engine:** O `krylov_manager.go` é uma base matemática sólida para compressão, pronta para ser usada nos embeddings de memórias atômicas.

## Plano de Ação Recomendado (Next Context)
Para fechar os gaps sem reescrever tudo, recomendo criar um **pipeline de ingestão** que intercepta o `ProcessUserSpeech` antes de chamar o `SaveEpisodicMemory`.

1.  **Criar `IngestionPipeline` service.**
2.  **Passo 1:** Bufferizar inputs (Session-based).
3.  **Passo 2:** LLM Extract Facts (Atomic + Dual Timestamp).
4.  **Passo 3:** Check Conflicts (Relational Versioning).
5.  **Passo 4:** Chamar `SaveEpisodicMemory` para os fatos processados.
