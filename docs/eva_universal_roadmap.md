# EVA Universal: Roadmap Técnico

**Visão:** Unificar a profundidade cognitiva do EVA-Mind (Krylov, Spectral, Lacan) com a onipresença digital do EVA-Memory (Supermemory), criando um **Guardião Digital Universal**.

> **Filosofia:** "O forte do Supermemory é ingestão + retrieval rápido. O forte do EVA é processamento cognitivo profundo. São complementares."

---

## Os 4 Elos Perdidos (The Missing Links)

Para transformar o EVA-Mind num sistema que realmente "pensa sobre o que sabe", precisamos implementar os seguintes mecanismos inspirados no cérebro humano, preenchendo as lacunas atuais entre o EVA-Mind e o Supermemory.

### 1. Smart Forgetting (Esquecimento Inteligente)

**Conceito:** Informação menos relevante desvanece gradualmente, enquanto conteúdo importante e frequentemente acessado se mantém nítido.

*   **Estado Atual (EVA-Mind):** Possui `temporal_decay.go` com decaimento exponencial cego `e^(-t/τ)`. Envelhece tudo uniformemente. O FIFO do Sliding Window remove o mais antigo, não o menos relevante.
*   **A Implementar:**
    *   Adicionar `access_count` e `last_accessed_at` em cada memória.
    *   Nova fórmula de decaimento: `score = importance * access_freq * e^(-t/τ)`.
    *   Substituir o FIFO cego por uma remoção baseada no **menor score composto**.

### 2. Recency Bias (Viés de Recência)

**Conceito:** O cérebro prioriza o que é útil *agora*. Memórias recentes devem ter peso maior que memórias antigas, dada a mesma similaridade semântica.

*   **Estado Atual (EVA-Mind):** Busca no Qdrant puramente por similaridade de cosseno. Uma memória de 5 minutos atrás compete igualmente com uma de 6 meses atrás.
*   **A Implementar:**
    *   Aplicar **Re-ranking** pós-retrieval na API (`search_memories`).
    *   Multiplicar o score de similaridade por um **fator de recência**.
    *   Implementar a lógica de `WaveletAttention` (`timeDecay := math.Exp(-mem.Age / timeConstant)`).

### 3. Context Rewriting (Reescrita de Contexto)

**Conceito:** Sono REM Artificial. O sistema deve reescrever resumos e encontrar ligações entre informações novas e antigas, resolvendo contradições ativamente.

*   **Estado Atual (EVA-Mind):** Memórias são estáticas (append-only). Contradições coexistem sem resolução (ex: "Gosto de azul" vs "Prefiro verde").
*   **A Implementar:**
    *   Criar um **Job Periódico** (`ConsolidateNightly`).
    *   Usar LLM para analisar clusters de memórias.
    *   Gerar **resumos atualizados** e criar **meta-memórias**.
    *   Resolver contradições e atualizar o grafo de conhecimento.

### 4. Hierarchical Memory Layers (Camadas Hierárquicas)

**Conceito:** Tiered Storage. Dados quentes em cache rápido, dados mornos em vetor, dados frios comprimidos/arquivados.

*   **Estado Atual (EVA-Mind):** Tudo vive no mesmo nível (Qdrant 64D + Neo4j). Não há distinção de "temperatura" de acesso.
*   **A Implementar (Sistema de 3 Tiers):**
    *   **Hot Tier:** Redis/In-Memory (últimas N interações, embedding completo ou 256D).
    *   **Warm Tier:** Qdrant 64D (semanas).
    *   **Cold Tier:** PostgreSQL/Resumos consolidados (meses+).
    *   **Cascade Query:** Procura no Hot -> Warm -> Cold.

---

## Estratégia de Integração

### Camada de Ingestão (Digital Memory)
*   **Responsável:** Componentes do **EVA-Memory (Supermemory)**.
*   **Ferramentas:** Extensão de Browser, Conectores (Notion, Drive), Servidor MCP.
*   **Função:** Capturar a "vida digital" bruta (links, textos, chats).

### Camada Profunda (Deep Mind)
*   **Responsável:** Motor **EVA-Mind (Krylov, Spectral, HMC)**.
*   **Função:** Processar, comprimir, esquecer inteligentemente e sonhar sobre os dados.

---

## Resumo do Plano de Ação

| Feature | Ação Técnica | Complexidade |
| :--- | :--- | :--- |
| **Smart Forgetting** | Atualizar fórmula de decay + eviction policy no Krylov | Baixa (Dias) |
| **Recency Bias** | Adicionar re-ranking step no endpoint de busca | Baixa (Dias) |
| **Hierarchical Layers** | Implementar Redis Cache + Lógica de Cascade | Média (1-2 Semanas) |
| **Context Rewriting** | Criar background workers para consolidação (Sonhar) | Alta (Transformacional) |
