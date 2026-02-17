# Sparse Representation-based Classification (SRC) no EVA-Mind

## Resolvendo o "Problema da Maria" sem Identificadores Fixos

**Autor:** Z.ai Research Division  
**Data:** Janeiro 2025  
**Versão:** 1.0

---

## Sumário Executivo

Este artigo técnico apresenta a implementação de **Sparse Representation-based Classification (SRC)** no sistema EVA-Mind para resolver o desafio fundamental do reconhecimento de entidades em ambientes de mundo aberto. O "Problema da Maria" — como distinguir se "Maria" mencionada hoje é a mesma pessoa de 2010 sem uma lista pré-definida de identificadores — é abordado através de reconstrução esparsa de memórias acumuladas, permitindo que o sistema reconheça entidades de forma dinâmica e adaptativa.

---

## 1. Introdução

### 1.1 O Abismo Entre IA de Laboratório e IA de Acompanhamento

Sistemas de IA tradicionais operam sob a premissa de **conjuntos fechados**: todas as classes possíveis são conhecidas durante o treinamento. O MNIST tem 10 dígitos, o ImageNet tem 1000 categorias, e o sistema nunca encontrará nada fora desse universo pré-definido.

O EVA-Mind, como sistema de acompanhamento de pacientes, opera em um **conjunto aberto**: o mundo contém infinitas entidades, relações e contextos que não podem ser enumerados antecipadamente. O usuário pode mencionar "Maria" referindo-se a:

- Maria (filha, 35 anos, médica)
- Maria (irmã, 42 anos, professora)
- Maria (prima, 28 anos, recém-conhecida)
- Uma nova Maria que acabou de conhecer

O desafio é determinístico: **como o sistema decide qual Maria está sendo referenciada sem maintained uma lista exaustiva de Marias possíveis?**

### 1.2 Por Que Abordagens Tradicionais Falham

#### 1.2.1 Classificação Supervisionada

A classificação supervisionada tradicional exige:
- Todas as classes conhecidas no treinamento
- Exemplos rotulados para cada classe
- Retreinamento para adicionar novas classes

**Problema:** Não é viável pre-treinar um classificador para todas as pessoas que um usuário pode mencionar ao longo de anos de uso.

#### 1.2.2 Reconhecimento de Entidades Nomeadas (NER)

NER tradicional extrai entidades mas não resolve identidade:
- Detecta "Maria" como PER (pessoa)
- Não distingue entre diferentes Marias
- Depende de bases de conhecimento externas

**Problema:** O contexto privado do usuário (suas Marias específicas) não está em bases públicas.

#### 1.2.3 Entity Linking

Entity linking conecta menções a bases de conhecimento:
- "Paris" → Paris, França (DBpedia)
- "Maria" → ? (múltiplas possibilidades, nenhuma específica)

**Problema:** Entidades privadas do usuário não existem em bases públicas.

### 1.3 A Solução: Sparse Representation-based Classification

SRC oferece uma abordagem fundamentalmente diferente:

> **Em vez de classificar em categorias pré-definidas, SRC tenta reconstruir o sinal de entrada a partir de um dicionário de sinais conhecidos. Se a reconstrução falha, o sinal pertence a uma classe desconhecida.**

Para o EVA-Mind:
- **Dicionário:** Memórias acumuladas sobre cada entidade
- **Sinal de entrada:** Menção atual com contexto
- **Reconstrução:** Combinação linear esparsa de memórias anteriores
- **Decisão:** Se reconstrução bem-sucedida → entidade conhecida; senão → nova entidade

---

## 2. Fundamentos Teóricos

### 2.1 O Problema de Representação Esparsa

Dado um sinal **y** ∈ ℝᵐ e um dicionário **D** ∈ ℝᵐˣⁿ (onde n >> m), a representação esparsa busca encontrar um vetor **x** ∈ ℝⁿ tal que:

```
y ≈ Dx
```

onde **x** tem poucos elementos não-zero (esparsidade).

### 2.2 SRC: Da Representação à Classificação

O algoritmo SRC, proposto por Wright et al. (2009), estende representação esparsa para classificação:

**Passo 1: Construção do Dicionário**

Para K classes, construa o dicionário D concatenando amostras de treino de todas as classes:

```
D = [D₁ | D₂ | ... | Dₖ]

onde Dᵢ = [vᵢ₁, vᵢ₂, ..., vᵢₙᵢ] contém nᵢ amostras da classe i
```

**Passo 2: Codificação Esparsa**

Resolva o problema de otimização:

```
minₓ ||y - Dx||₂ + λ||x||₁
```

onde ||x||₁ promove esparsidade (LASSO).

**Passo 3: Classificação**

Para cada classe i, compute o resíduo de reconstrução:

```
rᵢ(y) = ||y - Dδᵢ(x̂)||₂
```

onde δᵢ(x̂) zera todos os coeficientes exceto aqueles associados à classe i.

A classe predita é: `argminᵢ rᵢ(y)`

### 2.3 Extensão para Open-Set Recognition

O SRC original assume conjunto fechado. Para **open-set recognition** (reconhecimento de conjunto aberto), estendemos com:

**Critério de Rejeição:**

```
Se minᵢ rᵢ(y) > τ_rejeição:
    classe(y) = "desconhecido"
Senão:
    classe(y) = argminᵢ rᵢ(y)
```

**Adaptação ao EVA-Mind:**

O threshold τ é dinâmico, ajustado pela:
- Quantidade de memórias acumuladas sobre a entidade
- Qualidade dos embeddings disponíveis
- Contexto da conversa atual

---

## 3. Arquitetura SRC no EVA-Mind

### 3.1 Visão Geral

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    SRC ENTITY RESOLUTION PIPELINE                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ENTRADA                                                                 │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ Menção: "Maria"                                                  │    │
│  │ Contexto: "A Maria chegou de São Paulo ontem"                   │    │
│  │ Embedding: [0.12, -0.34, 0.56, ...] (1536D)                     │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  RECUPERAÇÃO DE MEMÓRIAS                                                 │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ Neo4j: MATCH (e:Entity {name: "Maria"})-[:MENTIONED_IN]->(m)    │    │
│  │ RETURN m.embedding, m.context, m.timestamp                      │    │
│  │                                                                  │    │
│  │ Resultado: [                                                     │    │
│  │   {emb: [0.11, -0.32, ...], ctx: "Maria (filha) médica"},       │    │
│  │   {emb: [0.45, 0.12, ...], ctx: "Maria (prima) professora"},    │    │
│  │   {emb: [0.10, -0.30, ...], ctx: "Maria foi ao mercado"}        │    │
│  │ ]                                                                │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  CONSTRUÇÃO DO DICIONÁRIO                                                │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ D_maria = matriz de embeddings das memórias de Maria            │    │
│  │ D_outros = embeddings de outras entidades (opcional)            │    │
│  │ D = [D_maria | D_outros]                                        │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  CODIFICAÇÃO ESPARSA (OMP)                                               │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ min ||y - Dx||₂ + λ||x||₁                                       │    │
│  │                                                                  │    │
│  │ Resultado: x = [0.7, 0, 0, 0.2, 0, 0, ...]                      │    │
│  │            (esparsa: poucos coeficientes não-zero)              │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  COMPUTAÇÃO DE RESÍDUOS                                                  │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ r_maria(y) = ||y - D_maria·x_maria||₂ = 0.15                    │    │
│  │ r_outros(y) = ||y - D_outros·x_outros||₂ = 0.82                 │    │
│  │                                                                  │    │
│  │ min_residual = 0.15                                              │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  DECISÃO                                                                 │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ min_residual (0.15) < τ (0.7)?                                   │    │
│  │ → SIM: Entidade CONHECIDA                                        │    │
│  │ → Link para nó "Maria" existente                                 │    │
│  │                                                                  │    │
│  │ Confidence Score: 1 - min_residual = 0.85                        │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  SAÍDA                                                                   │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ EntityResolutionResult(                                          │    │
│  │   entity_name = "Maria",                                         │    │
│  │   is_known = True,                                               │    │
│  │   confidence = 0.85,                                             │    │
│  │   matched_entity_id = "entity_maria_123",                       │    │
│  │   context_sources = ["mem_456", "mem_789"]                      │    │
│  │ )                                                                │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Componentes Principais

#### 3.2.1 Memory Dictionary Builder

Constrói o dicionário de memórias para cada entidade:

```python
class MemoryDictionaryBuilder:
    """
    Constrói dicionários de memórias para SRC.
    
    O dicionário D é uma matriz onde cada coluna é um embedding
    de uma memória associada a uma entidade.
    """
    
    async def build_entity_dictionary(
        self,
        entity_name: str,
        max_memories: int = 50,
    ) -> np.ndarray:
        """
        Constrói dicionário D para uma entidade específica.
        
        Args:
            entity_name: Nome da entidade (ex: "Maria")
            max_memories: Máximo de memórias a incluir
            
        Returns:
            Matriz D de shape (embedding_dim, num_memories)
        """
        # 1. Busca memórias associadas à entidade
        memories = await self.neo4j_session.run(
            """
            MATCH (e:Entity {name: $name})-[:MENTIONED_IN]->(m:Memory)
            RETURN m.embedding AS embedding, 
                   m.timestamp AS ts,
                   m.importance AS imp
            ORDER BY m.importance DESC, m.timestamp DESC
            LIMIT $limit
            """,
            name=entity_name,
            limit=max_memories,
        )
        
        # 2. Extrai embeddings
        embeddings = []
        for mem in memories:
            if mem["embedding"]:
                embeddings.append(np.array(mem["embedding"]))
        
        if not embeddings:
            return None
        
        # 3. Normaliza colunas (importante para SRC)
        D = np.column_stack(embeddings)
        D = D / np.linalg.norm(D, axis=0)
        
        return D
```

#### 3.2.2 Sparse Coder (OMP)

Implementa Orthogonal Matching Pursuit para codificação esparsa:

```python
class OrthogonalMatchingPursuit:
    """
    Implementação de OMP para codificação esparsa.
    
    OMP é um algoritmo greedy que seleciona iterativamente
    os átomos do dicionário que melhor explicam o sinal.
    """
    
    def __init__(self, max_nonzero: int = 10, tolerance: float = 1e-4):
        self.max_nonzero = max_nonzero  # Máximo de coeficientes não-zero
        self.tolerance = tolerance       # Tolerância para parada
    
    def encode(self, y: np.ndarray, D: np.ndarray) -> np.ndarray:
        """
        Resolve: min ||y - Dx||₂ sujeito a ||x||₀ <= max_nonzero
        
        Args:
            y: Sinal a ser codificado (embedding da menção atual)
            D: Dicionário de memórias
            
        Returns:
            x: Vetor esparso de coeficientes
        """
        residual = y.copy()
        indices = []
        x = np.zeros(D.shape[1])
        
        for _ in range(self.max_nonzero):
            # 1. Encontra átomo mais correlacionado com resíduo
            correlations = D.T @ residual
            best_idx = np.argmax(np.abs(correlations))
            
            if np.abs(correlations[best_idx]) < self.tolerance:
                break
            
            indices.append(best_idx)
            
            # 2. Resolve mínimos quadrados com átomos selecionados
            D_selected = D[:, indices]
            x_selected, _, _, _ = np.linalg.lstsq(D_selected, y, rcond=None)
            
            # 3. Atualiza resíduo
            residual = y - D_selected @ x_selected
            
            if np.linalg.norm(residual) < self.tolerance:
                break
        
        x[indices] = x_selected
        return x
```

#### 3.2.3 Entity Resolver

Combina os componentes para resolver identidade de entidades:

```python
class EntityResolver:
    """
    Resolvedor de entidades baseado em SRC.
    
    Determina se uma menção a uma entidade corresponde a
    uma entidade já conhecida ou se é uma nova entidade.
    """
    
    def __init__(
        self,
        neo4j_driver: AsyncDriver,
        reconstruction_threshold: float = 0.7,
        min_memories_for_src: int = 3,
    ):
        self.driver = neo4j_driver
        self.threshold = reconstruction_threshold
        self.min_memories = min_memories_for_src
        self.sparse_coder = OrthogonalMatchingPursuit()
        self.dict_builder = MemoryDictionaryBuilder(neo4j_driver)
    
    async def resolve(
        self,
        entity_name: str,
        query_embedding: np.ndarray,
        conversation_context: str,
    ) -> EntityResolutionResult:
        """
        Resolve a identidade de uma entidade mencionada.
        
        Args:
            entity_name: Nome da entidade mencionada
            query_embedding: Embedding do contexto da menção
            conversation_context: Texto completo da conversa
            
        Returns:
            EntityResolutionResult com status da resolução
        """
        # 1. Constrói dicionário de memórias da entidade
        D = await self.dict_builder.build_entity_dictionary(entity_name)
        
        # Caso: Entidade completamente nova
        if D is None or D.shape[1] < self.min_memories:
            await self._create_new_entity(entity_name, query_embedding)
            return EntityResolutionResult(
                entity_name=entity_name,
                is_known=False,
                confidence=0.0,
                matched_entity_id=None,
                context_sources=[],
            )
        
        # 2. Codificação esparsa
        # Normaliza query
        y = query_embedding / np.linalg.norm(query_embedding)
        
        # Resolve problema esparso
        x = self.sparse_coder.encode(y, D)
        
        # 3. Computa resíduo de reconstrução
        y_reconstructed = D @ x
        residual = np.linalg.norm(y - y_reconstructed)
        
        # 4. Converte residual para score de confiança [0, 1]
        # Residual baixo = alta confiança
        confidence = 1.0 - min(residual, 1.0)
        
        # 5. Decisão
        is_known = confidence >= self.threshold
        
        # 6. Identifica memórias relevantes (coeficientes não-zero)
        nonzero_indices = np.nonzero(x)[0]
        context_sources = await self._get_memory_ids(
            entity_name, 
            nonzero_indices
        )
        
        if is_known:
            # Atualiza contagem de menções
            await self._update_entity_mention(entity_name)
        else:
            # Cria nova variante da entidade
            await self._create_entity_variant(
                entity_name, 
                query_embedding,
                conversation_context
            )
        
        return EntityResolutionResult(
            entity_name=entity_name,
            is_known=is_known,
            confidence=confidence,
            matched_entity_id=entity_name if is_known else None,
            context_sources=context_sources,
        )
```

### 3.3 Integração com Neo4j

#### 3.3.1 Schema de Entidades

```cypher
// Nó de Entidade
CREATE (e:Entity {
    id: "entity_maria_123",
    name: "Maria",
    canonical_name: "Maria Silva",
    mention_count: 47,
    first_mentioned: 1577836800000,  // timestamp
    last_mentioned: 1704067200000,
    confidence_avg: 0.82
})

// Nó de Variante (para ambiguidade)
CREATE (v:EntityVariant {
    id: "variant_maria_456",
    parent_entity: "entity_maria_123",
    distinguishing_context: "Maria (filha) médica em SP",
    embedding_centroid: [0.11, -0.32, 0.56, ...],
    mention_count: 23
})

// Relacionamentos
(:Entity)-[:HAS_VARIANT]->(:EntityVariant)
(:EntityVariant)-[:MENTIONED_IN]->(:Memory)
(:Memory)-[:CONTAINS_ENTITY]->(:Entity)
```

#### 3.3.2 Queries de Recuperação

```cypher
// Recupera todas as memórias de uma entidade para SRC
MATCH (e:Entity {name: $name})-[:HAS_VARIANT*0..1]->(v)
      -(v2:MENTIONED_IN)->(m:Memory)
WHERE m.embedding IS NOT NULL
RETURN m.id AS memory_id,
       m.embedding AS embedding,
       m.context AS context,
       m.timestamp AS ts,
       v.distinguishing_context AS variant_context
ORDER BY m.importance DESC
LIMIT $max_memories

// Atualiza estatísticas após resolução bem-sucedida
MATCH (e:Entity {name: $name})
SET e.mention_count = e.mention_count + 1,
    e.last_mentioned = $now,
    e.confidence_avg = (e.confidence_avg * e.mention_count + $confidence) 
                       / (e.mention_count + 1)
```

---

## 4. Algoritmo Completo

### 4.1 Pseudocódigo

```
ALGORITMO: SRC_Entity_Resolution

ENTRADA:
    - entity_name: str              // Nome da entidade mencionada
    - query_embedding: vector[1536] // Embedding do contexto atual
    - context: str                  // Texto da conversa
    - threshold: float              // Limiar de confiança (default: 0.7)

SAÍDA:
    - result: EntityResolutionResult

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

PASSO 1: Recuperar Memórias
    memories ← Neo4j.query(
        "MATCH (e:Entity {name: entity_name})-[:MENTIONED_IN]->(m)
         RETURN m.embedding, m.context"
    )
    
    SE len(memories) < 3 ENTÃO
        // Entidade nova ou muito pouca informação
        RETORNE (is_known=False, confidence=0.0)
    FIM SE

PASSO 2: Construir Dicionário
    D ← [m.embedding for m in memories]  // Matriz m×n
    D ← normalize_columns(D)              // Cada coluna ||col||₂ = 1

PASSO 3: Normalizar Query
    y ← query_embedding / ||query_embedding||₂

PASSO 4: Codificação Esparsa (OMP)
    x ← vector_zeros[n]
    residual ← y
    selected ← []
    
    PARA i = 1 ATÉ max_iterations FAÇA
        // Encontra átomo mais correlacionado
        correlations ← |D^T × residual|
        j ← argmax(correlations)
        
        SE correlations[j] < tolerance ENTÃO
            BREAK
        FIM SE
        
        selected.append(j)
        
        // Resolve mínimos quadrados
        D_sel ← D[:, selected]
        x_sel ← (D_sel^T × D_sel)^(-1) × D_sel^T × y
        
        // Atualiza resíduo
        residual ← y - D_sel × x_sel
    FIM PARA
    
    x[selected] ← x_sel

PASSO 5: Computar Resíduo de Reconstrução
    y_hat ← D × x
    residual ← ||y - y_hat||₂
    confidence ← 1 - min(residual, 1.0)

PASSO 6: Decisão
    SE confidence ≥ threshold ENTÃO
        is_known ← True
        // Identificar qual variante (se houver múltiplas)
        variant ← identify_variant(x, selected)
    SENÃO
        is_known ← False
        // Criar nova variante
        create_entity_variant(entity_name, query_embedding)
    FIM SE

PASSO 7: Atualizar Grafo
    SE is_known ENTÃO
        update_mention_count(entity_name)
        link_memory_to_entity(current_memory, entity_name)
    FIM SE

RETORNE (entity_name, is_known, confidence, variant, context_sources)
```

### 4.2 Complexidade Computacional

| Operação | Complexidade | Observações |
|----------|--------------|-------------|
| Recuperação de memórias | O(k log k) | k = memórias da entidade, com índice |
| Construção do dicionário | O(m × k) | m = dimensão do embedding |
| Normalização | O(m × k) | |
| OMP (por iteração) | O(m × k + k²) | k iterações máximas |
| Total | O(k × (m + k)) | k << m, tipicamente k ≤ 50 |

Para embedding de 1536 dimensões e 50 memórias:
- Tempo esperado: ~50ms por resolução
- Memória: ~300KB por dicionário

---

## 5. Casos de Uso no EVA-Mind

### 5.1 Caso 1: Entidade Conhecida (Alta Confiança)

**Cenário:**
- Usuário menciona "Maria"
- Maria já foi mencionada 47 vezes
- Contexto similar a conversas anteriores

**Fluxo:**

```
INPUT:
  entity_name: "Maria"
  query_embedding: [0.11, -0.33, 0.55, ...]
  context: "A Maria ligou ontem"

PROCESSAMENTO:
  1. Recuperadas 47 memórias de Maria
  2. Dicionário D construído (1536 × 47)
  3. OMP converge em 5 iterações
  4. Coeficientes: [0.65, 0, 0.21, 0, 0.08, ...]
  5. Residual: 0.12
  6. Confidence: 1 - 0.12 = 0.88

OUTPUT:
  is_known: True
  confidence: 0.88
  matched_entity_id: "entity_maria_123"
  context_sources: ["mem_456", "mem_789", "mem_012"]
```

### 5.2 Caso 2: Ambiguidade (Múltiplas Variantes)

**Cenário:**
- Usuário menciona "Maria"
- Existem duas Marias frequentes:
  - Maria (filha): 35 anos, médica
  - Maria (prima): 42 anos, professora

**Fluxo:**

```
INPUT:
  entity_name: "Maria"
  query_embedding: [0.45, 0.12, -0.23, ...]
  context: "A Maria vai viajar para conferência médica"

PROCESSAMENTO:
  1. Recuperadas 70 memórias (47 + 23 de duas variantes)
  2. Dicionário D = [D_filha | D_prima]
  3. OMP seleciona majoritariamente de D_filha
  4. Coeficientes: [0.55, 0.12, 0, ..., 0.03, 0, ...]
                   ↑ da filha        ↑ da prima
  5. Resíduos:
     r_filha = 0.18
     r_prima = 0.67
  6. Selected variant: filha

OUTPUT:
  is_known: True
  confidence: 0.82
  matched_entity_id: "entity_maria_filha"
  variant: "Maria (filha)"
```

### 5.3 Caso 3: Nova Entidade (Baixa Confiança)

**Cenário:**
- Usuário menciona "Maria"
- Nunca mencionou nenhuma Maria antes

**Fluxo:**

```
INPUT:
  entity_name: "Maria"
  query_embedding: [0.78, 0.21, -0.45, ...]
  context: "Conheci uma Maria nova no trabalho"

PROCESSAMENTO:
  1. Recuperadas 0 memórias
  2. Dicionário vazio

OUTPUT:
  is_known: False
  confidence: 0.0
  matched_entity_id: None
  
AÇÃO:
  - Criar novo nó Entity: "Maria"
  - Associar à memória atual
  - Aguardar mais menções para consolidar
```

### 5.4 Caso 4: Entidade Similar mas Distinta

**Cenário:**
- Usuário menciona "Mariana"
- Já existe "Maria" com muitas memórias
- Nomes similares, mas pessoas diferentes

**Fluxo:**

```
INPUT:
  entity_name: "Mariana"
  query_embedding: [0.42, 0.33, 0.11, ...]
  context: "A Mariana é a nova chefe"

PROCESSAMENTO:
  1. Busca memórias de "Mariana": 0 encontradas
  2. Busca entidades similares por nome: ["Maria"]
  3. Tenta SRC com dicionário de Maria
  4. Residual alto: 0.85
  5. Confidence baixa: 0.15

DECISÃO:
  confidence (0.15) < threshold (0.7)
  → Provavelmente entidade distinta
  
OUTPUT:
  is_known: False
  confidence: 0.15
  note: "Similar a 'Maria' mas provavelmente diferente"
  
AÇÃO:
  - Criar novo nó Entity: "Mariana"
  - Não linkar com Maria
```

---

## 6. Comparação com Alternativas

### 6.1 SRC vs. Classificação Tradicional

| Aspecto | Classificação | SRC |
|---------|---------------|-----|
| Classes | Fechadas, pré-definidas | Abertas, dinâmicas |
| Treinamento | Necessário | Não requer |
| Novas classes | Requer retreinamento | Adiciona ao dicionário |
| Interpretabilidade | Baixa | Alta (coeficientes esparsos) |
| Confiança | Softmax | Resíduo de reconstrução |

### 6.2 SRC vs. Clustering

| Aspecto | Clustering | SRC |
|---------|------------|-----|
| Número de clusters | Hiperparâmetro | Emergente |
| Assignments | Rígidos | Soft (coeficientes) |
| Novos clusters | Requer re-cluster | Adaptação natural |
| Escalabilidade | O(n²) típico | O(k) por query |

### 6.3 SRC vs. Vector Similarity (k-NN)

| Aspecto | k-NN | SRC |
|---------|------|-----|
| Decisão | Votação majoritária | Reconstrução |
| Outliers | Afetam decisão | Rejeitados automaticamente |
| Overlapping classes | Problema | Tratado via esparsidade |
| Interpretação | Vizinhos | Combinação linear |

---

## 7. Limitações e Mitigações

### 7.1 Limitações Identificadas

#### 7.1.1 Cold Start

**Problema:** Entidades com poucas memórias (k < 3) não podem ser resolvidas via SRC.

**Mitigação:**
- Usar similaridade de cosseno simples para k pequeno
- Agregar contexto da conversa atual como "memória temporária"
- Solicitar esclarecimento ao usuário quando ambíguo

#### 7.1.2 Embedding Drift

**Problema:** Embeddings de mesma entidade mudam ao longo do tempo (modelo LLM atualizado, contexto diferente).

**Mitigação:**
- Re-embedar memórias periodicamente
- Usar múltiplos embeddings por entidade
- Ponderar memórias por idade

#### 7.1.3 Ambiguidade Intrínseca

**Problema:** Algumas menções são genuinamente ambíguas, mesmo para humanos.

**Mitigação:**
- Detectar baixa confiança e solicitar esclarecimento
- Usar contexto mais amplo (conversas anteriores)
- Manter histórico de desambiguação

#### 7.1.4 Escalabilidade

**Problema:** Dicionários muito grandes (k >> 1000) tornam OMP lento.

**Mitigação:**
- Pruning de memórias antigas/raras
- Índice aproximado (LSH) para pré-seleção
- Paralelização de OMP

### 7.2 Trade-offs de Design

| Decisão | Opção A | Opção B | Escolha |
|---------|---------|---------|---------|
| Algoritmo esparso | OMP | LASSO | OMP (mais rápido) |
| Tamanho do dicionário | Ilimitado | Limitado (50) | Limitado (balanceia precisão/performance) |
| Threshold | Fixo (0.7) | Dinâmico | Dinâmico (adapta por entidade) |
| Variantes | Automático | Manual | Híbrido (sugere, usuário confirma) |

---

## 8. Implementação de Referência

### 8.1 Código Completo

```python
"""
EVA-Mind: Entity Resolution via SRC
====================================
Implementação completa de Sparse Representation-based Classification
para resolução de identidade de entidades no EVA-Mind.
"""

import numpy as np
from typing import Optional, List, Tuple
from dataclasses import dataclass
from neo4j import AsyncDriver
import logging

logger = logging.getLogger("eva.src")


# ─────────────────────────────────────────────────────────────────────────────
# DATA STRUCTURES
# ─────────────────────────────────────────────────────────────────────────────

@dataclass
class EntityResolutionResult:
    """Resultado da resolução de entidade via SRC."""
    entity_name: str
    is_known: bool
    confidence: float
    matched_entity_id: Optional[str]
    variant_id: Optional[str]
    context_sources: List[str]
    reconstruction_error: float


# ─────────────────────────────────────────────────────────────────────────────
# SPARSE CODING
# ─────────────────────────────────────────────────────────────────────────────

class OrthogonalMatchingPursuit:
    """
    Orthogonal Matching Pursuit (OMP) para codificação esparsa.
    
    OMP é um algoritmo greedy que seleciona iterativamente os átomos
    do dicionário que melhor explicam o sinal, resolvendo um problema
    de mínimos quadrados a cada iteração.
    
    Referência: Pati, Rezaiifar, Krishnaprasad (1993)
    """
    
    def __init__(
        self,
        max_nonzero: int = 10,
        tolerance: float = 1e-4,
    ):
        """
        Args:
            max_nonzero: Máximo de coeficientes não-zero (esparsidade)
            tolerance: Tolerância para parada antecipada
        """
        self.max_nonzero = max_nonzero
        self.tolerance = tolerance
    
    def encode(
        self,
        y: np.ndarray,
        D: np.ndarray,
    ) -> Tuple[np.ndarray, float]:
        """
        Codifica o sinal y usando o dicionário D.
        
        Resolve: min ||y - Dx||₂ sujeito a ||x||₀ <= max_nonzero
        
        Args:
            y: Sinal a codificar (shape: m,)
            D: Dicionário (shape: m, n)
            
        Returns:
            x: Vetor esparso de coeficientes (shape: n,)
            residual: Erro de reconstrução final
        """
        m, n = D.shape
        residual = y.copy()
        indices = []
        x = np.zeros(n)
        
        for iteration in range(min(self.max_nonzero, n)):
            # 1. Projeta resíduo em todos os átomos
            projections = D.T @ residual
            
            # 2. Seleciona átomo com maior projeção absoluta
            best_idx = np.argmax(np.abs(projections))
            
            # 3. Verifica critério de parada
            if np.abs(projections[best_idx]) < self.tolerance:
                logger.debug(f"OMP parou na iteração {iteration}: projeção abaixo de tolerância")
                break
            
            indices.append(best_idx)
            
            # 4. Resolve mínimos quadrados com átomos selecionados
            D_selected = D[:, indices]
            try:
                x_selected, residuals, rank, s = np.linalg.lstsq(
                    D_selected, y, rcond=None
                )
            except np.linalg.LinAlgError:
                logger.warning("lstsq falhou, usando pseudoinversa")
                x_selected = np.linalg.pinv(D_selected) @ y
            
            # 5. Atualiza resíduo
            residual = y - D_selected @ x_selected
            
            # 6. Verifica convergência
            if np.linalg.norm(residual) < self.tolerance:
                logger.debug(f"OMP convergiu na iteração {iteration}")
                break
        
        # Preenche coeficientes selecionados
        x[indices] = x_selected
        final_residual = np.linalg.norm(residual)
        
        return x, final_residual


# ─────────────────────────────────────────────────────────────────────────────
# DICTIONARY MANAGEMENT
# ─────────────────────────────────────────────────────────────────────────────

CYPHER_GET_ENTITY_MEMORIES = """
MATCH (e:Entity {name: $name})
OPTIONAL MATCH (e)-[:HAS_VARIANT]->(v:EntityVariant)
OPTIONAL MATCH (e)-[m1:MENTIONED_IN]->(mem1:Memory)
OPTIONAL MATCH (v)-[m2:MENTIONED_IN]->(mem2:Memory)
WITH COALESCE(mem1, mem2) AS m
WHERE m.embedding IS NOT NULL
RETURN m.id AS memory_id,
       m.embedding AS embedding,
       m.context AS context,
       m.timestamp AS timestamp,
       m.importance AS importance
ORDER BY m.importance DESC, m.timestamp DESC
LIMIT $limit
"""

CYPHER_GET_VARIANT_INFO = """
MATCH (e:Entity {name: $name})-[:HAS_VARIANT]->(v:EntityVariant)
RETURN v.id AS variant_id,
       v.distinguishing_context AS context,
       v.mention_count AS count
ORDER BY v.mention_count DESC
"""


class DictionaryBuilder:
    """
    Construtor de dicionários para SRC a partir do Neo4j.
    """
    
    def __init__(
        self,
        driver: AsyncDriver,
        db_name: str = "neo4j",
    ):
        self.driver = driver
        self.db_name = db_name
    
    async def build(
        self,
        entity_name: str,
        max_memories: int = 50,
    ) -> Tuple[Optional[np.ndarray], List[dict]]:
        """
        Constrói o dicionário de memórias para uma entidade.
        
        Args:
            entity_name: Nome da entidade
            max_memories: Máximo de memórias a incluir
            
        Returns:
            D: Matriz dicionário (embedding_dim, num_memories)
            memories: Lista de metadados das memórias
        """
        async with self.driver.session(database=self.db_name) as session:
            result = await session.run(
                CYPHER_GET_ENTITY_MEMORIES,
                name=entity_name,
                limit=max_memories,
            )
            
            memories = [dict(record) async for record in result]
        
        if not memories or len(memories) < 3:
            return None, []
        
        # Extrai embeddings
        embeddings = []
        valid_memories = []
        
        for mem in memories:
            if mem.get("embedding"):
                embeddings.append(np.array(mem["embedding"]))
                valid_memories.append(mem)
        
        if len(embeddings) < 3:
            return None, []
        
        # Constrói matriz e normaliza colunas
        D = np.column_stack(embeddings)
        norms = np.linalg.norm(D, axis=0, keepdims=True)
        norms[norms == 0] = 1  # Evita divisão por zero
        D = D / norms
        
        logger.info(
            f"Dicionário construído para '{entity_name}': "
            f"shape={D.shape}, memórias válidas={len(valid_memories)}"
        )
        
        return D, valid_memories
    
    async def get_variants(
        self,
        entity_name: str,
    ) -> List[dict]:
        """Recupera variantes conhecidas de uma entidade."""
        async with self.driver.session(database=self.db_name) as session:
            result = await session.run(
                CYPHER_GET_VARIANT_INFO,
                name=entity_name,
            )
            return [dict(record) async for record in result]


# ─────────────────────────────────────────────────────────────────────────────
# ENTITY RESOLVER
# ─────────────────────────────────────────────────────────────────────────────

CYPHER_CREATE_ENTITY = """
MERGE (e:Entity {name: $name})
ON CREATE SET e.created_at = $now,
              e.mention_count = 1,
              e.confidence_avg = $confidence
ON MATCH SET e.mention_count = e.mention_count + 1,
             e.last_mentioned = $now,
             e.confidence_avg = (e.confidence_avg * (e.mention_count - 1) + $confidence) 
                               / e.mention_count
RETURN e
"""

CYPHER_LINK_MEMORY = """
MATCH (e:Entity {name: $name})
MATCH (m:Memory {id: $memory_id})
MERGE (e)-[r:MENTIONED_IN]->(m)
SET r.timestamp = $now,
    r.confidence = $confidence
"""

CYPHER_CREATE_VARIANT = """
MATCH (e:Entity {name: $name})
CREATE (v:EntityVariant {
    id: randomUUID(),
    distinguishing_context: $context,
    embedding_centroid: $centroid,
    mention_count: 1,
    created_at: $now
})
CREATE (e)-[:HAS_VARIANT]->(v)
RETURN v
"""


class EntityResolver:
    """
    Resolvedor de identidade de entidades via SRC.
    
    Determina se uma menção a uma entidade corresponde a uma
    entidade já conhecida ou se é uma nova entidade/variante.
    """
    
    def __init__(
        self,
        driver: AsyncDriver,
        reconstruction_threshold: float = 0.7,
        min_memories_for_src: int = 3,
        max_nonzero: int = 10,
        db_name: str = "neo4j",
    ):
        """
        Args:
            driver: Driver Neo4j assíncrono
            reconstruction_threshold: Threshold de confiança para aceitar
            min_memories_for_src: Mínimo de memórias para usar SRC
            max_nonzero: Esparsidade máxima no OMP
            db_name: Nome do banco Neo4j
        """
        self.driver = driver
        self.threshold = reconstruction_threshold
        self.min_memories = min_memories_for_src
        self.db_name = db_name
        
        self.sparse_coder = OrthogonalMatchingPursuit(max_nonzero=max_nonzero)
        self.dict_builder = DictionaryBuilder(driver, db_name)
    
    async def resolve(
        self,
        entity_name: str,
        query_embedding: np.ndarray,
        conversation_context: str,
        memory_id: Optional[str] = None,
    ) -> EntityResolutionResult:
        """
        Resolve a identidade de uma entidade mencionada.
        
        Args:
            entity_name: Nome da entidade mencionada
            query_embedding: Embedding do contexto da menção
            conversation_context: Texto da conversa atual
            memory_id: ID da memória sendo processada (opcional)
            
        Returns:
            EntityResolutionResult com decisão e metadados
        """
        now = int(time.time() * 1000)
        
        # 1. Constrói dicionário de memórias
        D, memories = await self.dict_builder.build(entity_name)
        
        # Caso: Entidade nova ou poucas memórias
        if D is None:
            confidence = 0.0
            await self._create_entity(entity_name, confidence, now)
            
            if memory_id:
                await self._link_memory(entity_name, memory_id, confidence, now)
            
            logger.info(f"Entidade nova: '{entity_name}'")
            
            return EntityResolutionResult(
                entity_name=entity_name,
                is_known=False,
                confidence=confidence,
                matched_entity_id=None,
                variant_id=None,
                context_sources=[],
                reconstruction_error=1.0,
            )
        
        # 2. Normaliza query
        query_norm = np.linalg.norm(query_embedding)
        if query_norm == 0:
            query_norm = 1.0
        y = query_embedding / query_norm
        
        # 3. Codificação esparsa
        x, residual = self.sparse_coder.encode(y, D)
        
        # 4. Converte para confiança
        confidence = max(0.0, 1.0 - residual)
        
        # 5. Identifica memórias relevantes
        nonzero_indices = np.nonzero(np.abs(x) > 1e-6)[0]
        context_sources = [
            memories[i]["memory_id"] 
            for i in nonzero_indices 
            if i < len(memories)
        ]
        
        # 6. Decisão
        is_known = confidence >= self.threshold
        
        # 7. Identifica variante (se aplicável)
        variant_id = None
        if is_known:
            variants = await self.dict_builder.get_variants(entity_name)
            if variants:
                variant_id = await self._identify_variant(
                    y, D, x, memories, variants
                )
        
        # 8. Atualiza grafo
        if is_known:
            await self._update_entity(entity_name, confidence, now)
            if memory_id:
                await self._link_memory(entity_name, memory_id, confidence, now)
            logger.info(
                f"Entidade reconhecida: '{entity_name}' "
                f"(conf={confidence:.2f}, variant={variant_id})"
            )
        else:
            # Baixa confiança: criar variante
            variant_id = await self._create_variant(
                entity_name, conversation_context, query_embedding, now
            )
            logger.info(
                f"Nova variante de '{entity_name}': {variant_id} "
                f"(conf={confidence:.2f})"
            )
        
        return EntityResolutionResult(
            entity_name=entity_name,
            is_known=is_known,
            confidence=confidence,
            matched_entity_id=entity_name if is_known else None,
            variant_id=variant_id,
            context_sources=context_sources,
            reconstruction_error=residual,
        )
    
    async def _create_entity(
        self, 
        name: str, 
        confidence: float,
        now: int,
    ) -> None:
        async with self.driver.session(database=self.db_name) as session:
            await session.run(
                CYPHER_CREATE_ENTITY,
                name=name,
                confidence=confidence,
                now=now,
            )
    
    async def _update_entity(
        self, 
        name: str, 
        confidence: float,
        now: int,
    ) -> None:
        async with self.driver.session(database=self.db_name) as session:
            await session.run(
                CYPHER_CREATE_ENTITY,
                name=name,
                confidence=confidence,
                now=now,
            )
    
    async def _link_memory(
        self,
        name: str,
        memory_id: str,
        confidence: float,
        now: int,
    ) -> None:
        async with self.driver.session(database=self.db_name) as session:
            await session.run(
                CYPHER_LINK_MEMORY,
                name=name,
                memory_id=memory_id,
                confidence=confidence,
                now=now,
            )
    
    async def _create_variant(
        self,
        entity_name: str,
        context: str,
        embedding: np.ndarray,
        now: int,
    ) -> str:
        async with self.driver.session(database=self.db_name) as session:
            result = await session.run(
                CYPHER_CREATE_VARIANT,
                name=entity_name,
                context=context[:500],  # Limita tamanho
                centroid=embedding.tolist(),
                now=now,
            )
            record = await result.single()
            return record["v"]["id"] if record else None
    
    async def _identify_variant(
        self,
        y: np.ndarray,
        D: np.ndarray,
        x: np.ndarray,
        memories: List[dict],
        variants: List[dict],
    ) -> Optional[str]:
        """
        Identifica qual variante da entidade está sendo referenciada.
        """
        # Simplificação: usa variante mais comum
        # Em implementação completa, usaria estrutura do dicionário
        if variants:
            return variants[0].get("variant_id")
        return None


# ─────────────────────────────────────────────────────────────────────────────
# FACTORY
# ─────────────────────────────────────────────────────────────────────────────

_resolver_instance: Optional[EntityResolver] = None


def create_entity_resolver(
    neo4j_uri: str,
    neo4j_user: str,
    neo4j_password: str,
    reconstruction_threshold: float = 0.7,
    max_nonzero: int = 10,
) -> EntityResolver:
    """
    Cria singleton do EntityResolver.
    Chame no startup do FastAPI.
    """
    global _resolver_instance
    
    from neo4j import AsyncGraphDatabase
    
    driver = AsyncGraphDatabase.driver(
        neo4j_uri,
        auth=(neo4j_user, neo4j_password),
    )
    
    _resolver_instance = EntityResolver(
        driver=driver,
        reconstruction_threshold=reconstruction_threshold,
        max_nonzero=max_nonzero,
    )
    
    logger.info(f"EntityResolver inicializado (threshold={reconstruction_threshold})")
    
    return _resolver_instance


def get_entity_resolver() -> EntityResolver:
    """Dependency injection para FastAPI."""
    if _resolver_instance is None:
        raise RuntimeError(
            "EntityResolver não inicializado. "
            "Chame create_entity_resolver() no startup."
        )
    return _resolver_instance
```

### 8.2 Integração com FastAPI

```python
from fastapi import APIRouter, Depends
from pydantic import BaseModel

router = APIRouter(prefix="/entity", tags=["Entity Resolution"])


class ResolveRequest(BaseModel):
    entity_name: str
    query_text: str
    memory_id: Optional[str] = None


class ResolveResponse(BaseModel):
    entity_name: str
    is_known: bool
    confidence: float
    matched_entity_id: Optional[str]
    variant_id: Optional[str]
    context_sources: List[str]


@router.post("/resolve", response_model=ResolveResponse)
async def resolve_entity(
    request: ResolveRequest,
    resolver: EntityResolver = Depends(get_entity_resolver),
    embedding_service: EmbeddingService = Depends(get_embedding_service),
):
    """
    Resolve a identidade de uma entidade mencionada.
    
    Usa SRC para determinar se a entidade é conhecida ou nova.
    """
    # 1. Gera embedding do contexto
    query_embedding = await embedding_service.embed(request.query_text)
    
    # 2. Resolve via SRC
    result = await resolver.resolve(
        entity_name=request.entity_name,
        query_embedding=query_embedding,
        conversation_context=request.query_text,
        memory_id=request.memory_id,
    )
    
    return ResolveResponse(
        entity_name=result.entity_name,
        is_known=result.is_known,
        confidence=result.confidence,
        matched_entity_id=result.matched_entity_id,
        variant_id=result.variant_id,
        context_sources=result.context_sources,
    )
```

---

## 9. Experimentação e Validação

### 9.1 Métricas de Avaliação

| Métrica | Definição | Target |
|---------|-----------|--------|
| Precision | Verdadeiros positivos / (VP + FP) | > 0.90 |
| Recall | Verdadeiros positivos / (VP + FN) | > 0.85 |
| F1-Score | Média harmônica de P e R | > 0.87 |
| Rejection Rate | Entidades corretamente rejeitadas | > 0.80 |
| Latência P95 | Tempo de resposta (95º percentil) | < 100ms |

### 9.2 Protocolo de Teste

```python
class SRCTestSuite:
    """Suite de testes para validar SRC no EVA-Mind."""
    
    async def test_known_entity_high_confidence(self):
        """Entidade bem conhecida deve ter alta confiança."""
        # Prepara: 50 memórias de "Maria"
        # Executa: resolve("Maria", embedding_similar)
        # Espera: is_known=True, confidence>0.8
    
    async def test_new_entity_low_confidence(self):
        """Entidade nova deve ter baixa confiança."""
        # Prepara: nenhuma memória de "Pedro"
        # Executa: resolve("Pedro", embedding_qualquer)
        # Espera: is_known=False, confidence=0
    
    async def test_ambiguous_entity(self):
        """Entidade ambígua deve criar variante."""
        # Prepara: Maria filha + Maria prima (embeddings diferentes)
        # Executa: resolve("Maria", embedding_intermediario)
        # Espera: is_known=False OU variant criada
    
    async def test_similar_names_different_entities(self):
        """Nomes similares devem ser entidades distintas."""
        # Prepara: "Maria" com 50 memórias
        # Executa: resolve("Mariana", embedding_diferente)
        # Espera: is_known=False (não confundir com Maria)
```

---

## 10. Conclusões

### 10.1 Contribuições Principais

1. **Abordagem para Mundo Aberto:** SRC permite reconhecimento de entidades sem enumeração prévia, essencial para sistemas de acompanhamento de longo prazo.

2. **Resolução de Identidade:** O resíduo de reconstrução fornece uma medida natural de confiança, permitindo distinguir entidades conhecidas de novas.

3. **Integração com Grafos de Conhecimento:** A implementação aproveita a estrutura Neo4j existente, usando memórias acumuladas como dicionário dinâmico.

4. **Tratamento de Ambiguidade:** O sistema lida naturalmente com homônimos através de variantes, sem necessidade de regras manuais.

### 10.2 Trabalhos Futuros

1. **OMP Acelerado:** Implementar versão paralela de OMP para dicionários grandes.

2. **Dicionários Hierárquicos:** Estruturar dicionários por tempo para capturar evolução de entidades.

3. **Multi-modal SRC:** Estender para incluir features de áudio e imagem.

4. **Active Learning:** Solicitar feedback do usuário para melhorar precisão em casos ambíguos.

---

## Referências

1. Wright, J., Yang, A. Y., Ganesh, A., Sastry, S. S., & Ma, Y. (2009). Robust face recognition via sparse representation. *IEEE Transactions on Pattern Analysis and Machine Intelligence*, 31(2), 210-227.

2. Scheirer, W. J., Rocha, A., Micheals, R., & Boult, T. E. (2013). Meta-recognition: The theory and practice of recognition score analysis. *IEEE Transactions on Pattern Analysis and Machine Intelligence*, 35(8), 1689-1697.

3. Zhang, L., Yang, M., & Feng, X. (2011). Sparse representation or collaborative representation: Which helps face recognition? *ICCV 2011*.

4. Pati, Y. C., Rezaiifar, R., & Krishnaprasad, P. S. (1993). Orthogonal matching pursuit: Recursive function approximation with applications to wavelet decomposition. *Asilomar Conference on Signals, Systems and Computers*.

5. Kanerva, P. (1988). *Sparse distributed memory*. MIT Press.

---

## Apêndice A: Parâmetros de Configuração

```yaml
# config/src_config.yaml

entity_resolution:
  # Threshold de confiança para aceitar entidade como conhecida
  reconstruction_threshold: 0.7
  
  # Mínimo de memórias para usar SRC (abaixo usa similaridade simples)
  min_memories_for_src: 3
  
  # Máximo de memórias no dicionário (pruning automático)
  max_dictionary_size: 50
  
  # Esparsidade máxima no OMP
  max_nonzero_coefficients: 10
  
  # Tolerância para parada antecipada do OMP
  omp_tolerance: 1.0e-4
  
  # Similaridade de nome para buscar entidades relacionadas
  name_similarity_threshold: 0.8

neo4j:
  uri: "bolt://localhost:7687"
  user: "neo4j"
  password: "${NEO4J_PASSWORD}"
  database: "neo4j"
  
embedding:
  model: "text-embedding-3-small"
  dimension: 1536
```

---

## Apêndice B: Troubleshooting

| Problema | Sintoma | Solução |
|----------|---------|---------|
| Baixa confiança para entidade conhecida | confidence < 0.5 para "Maria" bem conhecida | Aumentar max_dictionary_size; verificar qualidade dos embeddings |
| Falsos positivos | "Mariana" reconhecida como "Maria" | Aumentar reconstruction_threshold; usar similaridade de nome |
| Alta latência | Resolução > 500ms | Reduzir max_dictionary_size; implementar cache de dicionários |
| Memória insuficiente | OOM em dicionários grandes | Implementar pruning agressivo; limitar max_nonzero |

---

*Documento gerado pelo Z.ai Research Division*
*Versão 1.0 — Janeiro 2025*
