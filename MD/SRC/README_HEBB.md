# EVA — Módulo Hebbian Updater

## Visão Geral

Implementação da regra pseudo-Hebb para atualização dinâmica de pesos de
arestas no grafo Neo4j da Memória Episódica do EVA.

```
SRC (paper)          EVA (seu projeto)
──────────────       ──────────────────────────────
Dicionário fixo  →   Grafo que cresce com o uso
Classificação    →   Expansão associativa
Busca esparsa    →   Pesos dinâmicos nas arestas
```

---

## A Fórmula Central

```
Δw(A→B) = η · decay(Δt) − λ · w(A→B)
           └─── LTP ───┘  └─── LTD ──┘

decay(Δt) = e^(−Δt / τ)
```

| Símbolo | Nome | Papel biológico |
|---------|------|-----------------|
| `η` | Taxa de aprendizado | Força da potenciação sináptica |
| `decay` | Curva de Ebbinghaus | Esquecimento temporal sem reforço |
| `λ` | Regularização L2 | Long-Term Depression (LTD) |
| `τ` | Meia-vida (tau) | Constante de tempo da memória |

---

## As 4 Zonas de Memória

```
weight
 1.0 ┤                        ████ consolidated  (≥ 0.7)
 0.7 ┤─────────────────────────────────────────
     │                  ████████
 0.3 ┤──────────────────────────────────────────
     │         ████████               emerging  (0.3–0.7)
 0.1 ┤─────────────────────────────────────────
     │ ████████
 0.0 ┤          
      new      weak    emerging  consolidated
```

| Zona | Ação do EVA |
|------|-------------|
| `new` | Logar como expansão associativa |
| `weak` | Candidata a pruning periódico |
| `emerging` | Sugerir conexão ao usuário |
| `consolidated` | Pré-carregar no contexto da resposta |

---

## Arquitetura dos Arquivos

```
hebbian_updater.py      ← Motor principal (Hebb puro)
eva_routes.py           ← Integração FastAPI
test_hebbian_updater.py ← Testes sem Neo4j real
```

---

## Instalação

```bash
pip install fastapi uvicorn neo4j pytest pytest-asyncio
```

---

## Variáveis de Ambiente

```env
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=sua_senha

# Hiperparâmetros Hebb (todos opcionais — defaults sensatos já definidos)
HEBB_ETA=0.05                # taxa de aprendizado
HEBB_LAMBDA=0.01             # regularização LTD
HEBB_TAU_SECONDS=86400       # meia-vida = 1 dia
HEBB_THRESHOLD_CONSOL=0.7    # limiar consolidated
HEBB_THRESHOLD_EMERG=0.3     # limiar emerging
```

---

## Integração em 3 Linhas

```python
# Após sua busca Neo4j retornar os nós ativados:
activated_ids = [node["id"] for node in search_results]
zones = await hebb_updater.process_session(activated_ids)
consolidated = [z for z in zones if z.zone == "consolidated"]
```

---

## Schema Neo4j Gerado

```cypher
// Nós
(:Memory {id: String})

// Arestas com metadados Hebb
[:ASSOCIADO_COM {
    weight:              Float,   // 0.0 → 1.0
    co_activation_count: Integer, // quantas vezes ativados juntos
    created_at:          Integer, // timestamp ms
    last_activated:      Integer, // timestamp ms
    delta_last:          Float    // último Δw aplicado
}]
```

---

## Pruning Periódico (APScheduler)

```python
from apscheduler.schedulers.asyncio import AsyncIOScheduler

scheduler = AsyncIOScheduler()
scheduler.add_job(
    lambda: asyncio.create_task(
        hebb_updater.prune_weak_edges(max_age_days=30)
    ),
    trigger="cron",
    hour=3,
    minute=0,
)
scheduler.start()
```

---

## Referências Científicas

1. **Hebb, D.O. (1949)** — *The Organization of Behavior* — regra original
2. **Kanerva (1988)** — *Sparse Distributed Memory* — base para expansão associativa
3. **Stokłosa et al. (2025)** — *Associative Knowledge Graphs for Efficient Sequence Storage* — SSAKGs com Weighted Edges
4. **Ororbia et al. (2020)** — *Differentiable Hebbian Plasticity* — pesos lentos + rápidos
5. **Vitay — Neurocomputing** — Hopfield networks, LTP/LTD, capacidade C=0.14·N
