# Documentação de Mudanças: Arquitetura de Memória Fractal

**Data:** 12/02/2026
**Autor:** Antigravity (IA)
**Objetivo:** Reativar as camadas cognitivas (REM e Pruning) via alinhamento de esquema no Neo4j.

---

## 1. Problema Identificado
Existia uma desconexão entre a camada de **Ingestão** (que salva novos dados) e a camada de **Consolidação/Poda** (que organiza os dados).
*   **Ingestão**: Estava salvando nós como `:Event` com a relação `:EXPERIENCED`.
*   **Consolidação (V2)**: Estava procurando nós como `:Memory` com a relação `:HAS_MEMORY`.
*   **Resultado**: O sistema "rodava mas não via nada", impedindo o sono REM artificial e a limpeza de memórias irrelevantes.

---

## 2. Mudanças Técnicas

### A. Alinhamento de Esquema (Neo4j)
| Arquivo | Componente | Mudança Principal |
| :--- | :--- | :--- |
| `rem_consolidator.go` | **REM Sleep** | Queries Cypher alteradas de `:Memory` para `:Event`. Relação alterada para `:EXPERIENCED`. |
| `pruning.go` | **Synaptic Pruning** | Alinhamento de tipos de relação para incluir `:EXPERIENCED` no percurso de poda. |
| `graph_store.go` | **Ingestion Service** | Inclusão de metadados obrigatórios em cada novo evento. |

### B. Injeção de Metadados de Ingestão (`graph_store.go`)
Todo novo nó `:Event` agora nasce com os seguintes campos essenciais para os algoritmos cognitivos:
```cypher
type: 'episodic',
activation_score: 1.0
```
*   **Rationale**: O `activation_score` é o combustível do motor REM. Memórias com alto score são escolhidas para "sonhar" (consolidação semântica). Sem esse campo, o filtro de "hot memories" retornava nulo.

---

## 3. Impacto no Sistema

1.  **REM Consolidation Ativado**: O job de 6 horas agora identifica memórias episódicas reais e gera nós `:SemanticMemory` (abstrações), ligando-os aos eventos originais via `:ABSTRACTED_FROM`.
2.  **Eficiência de Armazenamento**: A poda sináptica agora consegue "envelhecer" e remover conexões fracas no grafo Neo4j, mantendo apenas o que é reforçado pelo uso.
3.  **Memória Fractal**: O sistema agora segue a hierarquia correta:
    *   **Level 1**: `:Event` (Dados brutos, episódicos).
    *   **Level 2**: `:SemanticMemory` (Conceitos abstraídos pelo REM).
    *   **Level 3**: Conexões reforçadas pelo `Synaptogenesis`.

---

## 4. Próximos Passos
*   [ ] **Monitoramento**: Verificar a taxa de compressão (episódico -> semântico) após 24h de uso.
*   [ ] **Feedback Loop**: Ajustar o `activation_score` baseado no input emocional (se a emoção for forte, o score inicial deve ser > 1.0).

---
*Documento gerado automaticamente para o projeto EVA-Mind.*
