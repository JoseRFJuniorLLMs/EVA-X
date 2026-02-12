# Comparativo: EVA-Mind vs EVA-Memory

**Baseado na análise dos documentos:** `eva-memory.txt`, `eva-memory-teste.txt`, `eva-memory-memoria-avaliar.txt`, `pesquisa.txt`.

## Resumo Executivo

A análise revela que o **EVA-Mind não está "atrás" do EVA-Memory** em profundidade cognitiva; pelo contrário, possui mecanismos matemáticos e neurobiológicos (Krylov, Spectral, Lacan) muito mais avançados.

O **EVA-Memory (Supermemory)** vence na utilidade prática imediata: ingestão de dados (Browser, MCP) e gestão simples de relevância (Recency/Frequency).

A estratégia vencedora identificada nos textos é a **Inversão**: Usar o EVA-Memory como "sensores" (olhos e ouvidos digitais) e o EVA-Mind como o "cérebro" (processamento profundo).

---

## Tabela de Comparação Técnica

| Recurso / Conceito | EVA-Memory (Princípios Supermemory) | EVA-Mind (Implementação Atual) | Veredito & Ação |
| :--- | :--- | :--- | :--- |
| **Smart Forgetting** | Decaimento simples por Tempo + Frequência. | **Decaimento Exponencial** (`temporal_decay.go`) + **Poda Espectral Fractal** (detecta comunidades irrelevantes). | **EVA-Mind Superior.** <br> *Ação:* Adicionar metadados explícitos (`access_count`) para refinar o algoritmo existente. |
| **Recency Bias** | "Boost" temporal simples no retrieval. | **Busca Híbrida** + **Wavelet Attention** (atenção multi-escala temporal de 5min a 1 semana). | **Conceito Avançado, Prática Ausente.** <br> *Ação:* Implementar `RetrieveWithBias` simples agora (Passo 1 do "Elan Musk"). |
| **Context Rewriting** | Resumos atualizados continuamente. | **Sinaptogênese** (Co-ativação cria arestas) + **Motor Lacaniano** (infere o "não dito"). | **EVA-Mind Mais Profundo.** <br> *Ação:* Criar job `ConsolidateNightly` para rodar essas inferências ciclicamente. |
| **Hierarchical Layers** | Tiered Storage: Hot (KV) → Warm (Vector) → Cold (S3). | **Triple Store:** PostgreSQL (Episódico) + Neo4j (Causal) + Qdrant (Semântico). | **Abordagens Diferentes.** <br> *Ação:* Manter Triple Store mas adicionar Redis para "Hot Layer" (memória de trabalho). |
| **Ingestion (Captura)** | **Browser Extension**, **MCP Server**, Conectores (Drive/Notion). | API Python (Upload manual/clínico). | **EVA-Memory Superior.** <br> *Ação Crítica:* Portar `internal/mcp` e Browser Extension para o EVA-Mind. |
| **Filosofia** | "Segundo Cérebro" para produtividade. | "Guardião Digital" para saúde mental e imortalidade (Lacan/Gurdjieff). | **Complementares.** <br> O digital (Memory) alimenta o psicanalítico (Mind). |

---

## O "Caminho do Imortal" (Roadmap Sintetizado)

Baseado no feedback do "Elan Musk" (`eva-memory-teste.txt`):

1.  **Passo 0 (Benchmark):** Medir o estado atual. Quantas memórias são lixo? Qual o Recall real? (Criar `BASELINE.md`).
2.  **Passo 1 (Quick Wins):** Implementar `Smart Forgetting` e `Recency Bias` no código Go/Python.
3.  **Passo 2 (Abertura):** Portar o **MCP Server** do EVA-Memory para permitir que o Claude/Cursor interaja com o EVA-Mind.
4.  **Passo 3 (Deep Tech):** Ativar a `Wavelet Attention` e `Consolidação REM` (Sonho) para processamento noturno.
