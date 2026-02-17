"""
EVA — Hebbian Updater Module
════════════════════════════════════════════════════════════════════
Implementa a regra pseudo-Hebb para atualização dinâmica de pesos
de arestas no grafo Neo4j da Memória Episódica do EVA.

Inspirado em:
  - Hebb (1949): "Neurons that fire together, wire together"
  - Differentiable Hebbian Plasticity (DHP): pesos lentos + rápidos
  - Associative Knowledge Graphs (SSAKGs): weighted edges sem treino prévio
  - Hopfield Networks: LTP/LTD e capacidade C = 0.14·N

Regra central:
  Δw(A→B) = η · freq(A,B) · decay(Δt) − λ · w(A→B)
             └─ LTP (potenciação) ─┘  └─ LTD (depressão) ─┘
════════════════════════════════════════════════════════════════════
"""

import math
import time
import logging
from itertools import combinations
from dataclasses import dataclass, field
from typing import Optional

from neo4j import AsyncGraphDatabase, AsyncDriver

logger = logging.getLogger("eva.hebbian")

# ─────────────────────────────────────────────
# CONFIGURAÇÃO CENTRAL
# ─────────────────────────────────────────────

@dataclass
class HebbianConfig:
    """
    Todos os hiperparâmetros da dinâmica Hebb.
    Ajuste sem tocar no código de lógica.
    """

    # Taxa de aprendizado (η) — quanto cada co-ativação reforça a aresta
    eta: float = 0.05

    # Peso inicial de uma aresta nova (bootstrap)
    eta_initial: float = 0.1

    # Fator de regularização L2 (λ) — freio contra saturação (LTD)
    lambda_decay: float = 0.01

    # Meia-vida temporal em segundos (τ) — curva de esquecimento
    # Ex: 86400 = 1 dia; associações não reforçadas perdem força em ~dias
    tau_seconds: float = 86_400.0

    # Peso máximo permitido (evita runaway potentiation)
    weight_max: float = 1.0

    # Peso mínimo; abaixo disso a aresta é candidata a pruning
    weight_min: float = 0.01

    # Limiar para "associação consolidada" (pré-carrega na memória de trabalho)
    threshold_consolidated: float = 0.7

    # Limiar para "associação emergente" (sugere ao usuário)
    threshold_emerging: float = 0.3

    # Máximo de pares processados por sessão (evita explosão O(n²))
    max_pairs_per_session: int = 50


# ─────────────────────────────────────────────
# MODELO DE ZONA DE MEMÓRIA
# ─────────────────────────────────────────────

@dataclass
class EdgeMemoryZone:
    """
    Classificação do estado de uma aresta após atualização.
    Usada pelo EVA para decidir o que fazer com a associação.
    """
    node_a: str
    node_b: str
    weight: float
    co_activations: int
    zone: str  # "consolidated" | "emerging" | "weak" | "new"
    delta_applied: float


# ─────────────────────────────────────────────
# QUERIES CYPHER
# ─────────────────────────────────────────────

CYPHER_UPSERT_EDGE = """
MERGE (a:Memory {id: $node_a})
MERGE (b:Memory {id: $node_b})
MERGE (a)-[r:ASSOCIADO_COM]->(b)
ON CREATE SET
    r.weight             = $eta_initial,
    r.co_activation_count = 1,
    r.created_at         = $now,
    r.last_activated     = $now,
    r.delta_last         = $eta_initial
ON MATCH SET
    r.weight             = CASE
                             WHEN r.weight + $delta > $weight_max THEN $weight_max
                             WHEN r.weight + $delta < $weight_min THEN $weight_min
                             ELSE r.weight + $delta
                           END,
    r.co_activation_count = r.co_activation_count + 1,
    r.last_activated     = $now,
    r.delta_last         = $delta
RETURN r.weight AS weight, r.co_activation_count AS co_activations
"""

CYPHER_GET_EDGE = """
MATCH (a:Memory {id: $node_a})-[r:ASSOCIADO_COM]->(b:Memory {id: $node_b})
RETURN r.weight AS weight,
       r.co_activation_count AS co_activations,
       r.last_activated AS last_activated
"""

CYPHER_PRUNE_WEAK = """
MATCH ()-[r:ASSOCIADO_COM]->()
WHERE r.weight < $weight_min
  AND r.last_activated < $cutoff_timestamp
DELETE r
RETURN count(r) AS pruned
"""

CYPHER_TOP_ASSOCIATIONS = """
MATCH (a:Memory {id: $node_id})-[r:ASSOCIADO_COM]->(b:Memory)
WHERE r.weight >= $threshold
RETURN b.id AS neighbor, r.weight AS weight
ORDER BY r.weight DESC
LIMIT $top_k
"""


# ─────────────────────────────────────────────
# MOTOR PRINCIPAL
# ─────────────────────────────────────────────

class HebbianUpdater:
    """
    Motor de atualização pseudo-Hebb para o grafo EVA.

    Uso básico:
        updater = HebbianUpdater(driver, config)
        zones   = await updater.process_session(["node_uuid_1", "node_uuid_2", "node_uuid_3"])

    Cada vez que uma query Neo4j ativa um conjunto de nós juntos,
    chame process_session() com os IDs desses nós.
    """

    def __init__(
        self,
        driver: AsyncDriver,
        config: Optional[HebbianConfig] = None,
        db_name: str = "neo4j",
    ):
        self.driver = driver
        self.cfg = config or HebbianConfig()
        self.db_name = db_name

    # ── Fórmula central ──────────────────────────────────────────────
    def _compute_delta(
        self,
        current_weight: float,
        last_activated_ms: Optional[int],
    ) -> float:
        """
        Δw = η · decay(Δt) − λ · w

        decay(Δt) = e^(−Δt / τ)   ← curva de esquecimento de Ebbinghaus
        LTP = η · decay            ← potenciação via co-ativação
        LTD = λ · w                ← depressão por regularização
        """
        now_ms = int(time.time() * 1000)

        if last_activated_ms:
            delta_t_sec = (now_ms - last_activated_ms) / 1000.0
            decay = math.exp(-delta_t_sec / self.cfg.tau_seconds)
        else:
            decay = 1.0  # aresta nova: sem decaimento

        ltp = self.cfg.eta * decay
        ltd = self.cfg.lambda_decay * current_weight
        return ltp - ltd

    # ── Classificação de zona ────────────────────────────────────────
    def _classify_zone(self, weight: float, is_new: bool) -> str:
        if is_new:
            return "new"
        if weight >= self.cfg.threshold_consolidated:
            return "consolidated"
        if weight >= self.cfg.threshold_emerging:
            return "emerging"
        return "weak"

    # ── Atualiza um par de nós ───────────────────────────────────────
    async def _update_pair(
        self,
        node_a: str,
        node_b: str,
    ) -> EdgeMemoryZone:
        async with self.driver.session(database=self.db_name) as session:

            # 1. Busca estado atual da aresta (se existir)
            existing = await session.run(
                CYPHER_GET_EDGE, node_a=node_a, node_b=node_b
            )
            record = await existing.single()

            if record:
                current_weight = record["weight"]
                last_activated = record["last_activated"]
                is_new = False
            else:
                current_weight = 0.0
                last_activated = None
                is_new = True

            # 2. Calcula Δw pela regra pseudo-Hebb
            delta = self._compute_delta(current_weight, last_activated)

            # 3. Aplica no Neo4j (MERGE — cria ou atualiza)
            now_ms = int(time.time() * 1000)
            result = await session.run(
                CYPHER_UPSERT_EDGE,
                node_a=node_a,
                node_b=node_b,
                eta_initial=self.cfg.eta_initial,
                delta=delta,
                weight_max=self.cfg.weight_max,
                weight_min=self.cfg.weight_min,
                now=now_ms,
            )
            row = await result.single()
            new_weight = row["weight"]
            co_activations = row["co_activations"]

            zone = self._classify_zone(new_weight, is_new)

            logger.debug(
                "Hebb update (%s→%s): w=%.4f Δ=%.4f zone=%s co_act=%d",
                node_a, node_b, new_weight, delta, zone, co_activations,
            )

            return EdgeMemoryZone(
                node_a=node_a,
                node_b=node_b,
                weight=new_weight,
                co_activations=co_activations,
                zone=zone,
                delta_applied=delta,
            )

    # ── Processa uma sessão inteira ──────────────────────────────────
    async def process_session(
        self,
        activated_node_ids: list[str],
    ) -> list[EdgeMemoryZone]:
        """
        Ponto de entrada principal.

        Recebe os IDs de todos os nós ativados em uma query/sessão
        e atualiza todos os pares via regra Hebb.

        Returns lista de EdgeMemoryZone — use para:
          - zone == "consolidated" → pré-carregar no contexto do EVA
          - zone == "emerging"     → sugerir ao usuário
          - zone == "new"          → logar como expansão associativa
          - zone == "weak"         → candidato a pruning futuro
        """
        if len(activated_node_ids) < 2:
            return []

        # Gera pares únicos (A,B) e (B,A) — grafo não-direcionado tratado
        # como bidirecional para capturar associação em ambas direções
        pairs = list(combinations(activated_node_ids, 2))

        # Limita explosão combinatória
        if len(pairs) > self.cfg.max_pairs_per_session:
            logger.warning(
                "Sessão com %d pares — truncando para %d",
                len(pairs), self.cfg.max_pairs_per_session,
            )
            pairs = pairs[: self.cfg.max_pairs_per_session]

        results = []
        for node_a, node_b in pairs:
            zone = await self._update_pair(node_a, node_b)
            results.append(zone)

        # Log resumo
        zone_counts = {}
        for r in results:
            zone_counts[r.zone] = zone_counts.get(r.zone, 0) + 1
        logger.info("Sessão Hebb processada: %s", zone_counts)

        return results

    # ── Busca vizinhos mais fortes de um nó ─────────────────────────
    async def get_top_associations(
        self,
        node_id: str,
        top_k: int = 10,
        min_threshold: Optional[float] = None,
    ) -> list[dict]:
        """
        Retorna os nós mais fortemente associados a node_id.
        Útil para o EVA pré-carregar contexto relevante.
        """
        threshold = min_threshold or self.cfg.threshold_emerging
        async with self.driver.session(database=self.db_name) as session:
            result = await session.run(
                CYPHER_TOP_ASSOCIATIONS,
                node_id=node_id,
                threshold=threshold,
                top_k=top_k,
            )
            return [dict(r) async for r in result]

    # ── Pruning periódico de arestas fracas ─────────────────────────
    async def prune_weak_edges(
        self,
        max_age_days: float = 30.0,
    ) -> int:
        """
        Remove arestas com peso abaixo do mínimo E sem ativação recente.
        Chame via scheduler (ex: APScheduler, a cada 24h).

        Returns número de arestas removidas.
        """
        cutoff_ms = int(
            (time.time() - max_age_days * 86_400) * 1000
        )
        async with self.driver.session(database=self.db_name) as session:
            result = await session.run(
                CYPHER_PRUNE_WEAK,
                weight_min=self.cfg.weight_min,
                cutoff_timestamp=cutoff_ms,
            )
            row = await result.single()
            pruned = row["pruned"] if row else 0
            logger.info("Pruning: %d arestas removidas.", pruned)
            return pruned


# ─────────────────────────────────────────────
# FÁBRICA — para injeção de dependência no FastAPI
# ─────────────────────────────────────────────

_updater_instance: Optional[HebbianUpdater] = None


def create_hebbian_updater(
    neo4j_uri: str,
    neo4j_user: str,
    neo4j_password: str,
    config: Optional[HebbianConfig] = None,
) -> HebbianUpdater:
    """
    Cria (e cacheia) o singleton do HebbianUpdater.
    Chame em startup() do FastAPI.
    """
    global _updater_instance
    if _updater_instance is None:
        driver = AsyncGraphDatabase.driver(
            neo4j_uri,
            auth=(neo4j_user, neo4j_password),
        )
        _updater_instance = HebbianUpdater(driver, config)
        logger.info("HebbianUpdater inicializado → %s", neo4j_uri)
    return _updater_instance


def get_hebbian_updater() -> HebbianUpdater:
    """Dependency para injetar no FastAPI via Depends()."""
    if _updater_instance is None:
        raise RuntimeError("HebbianUpdater não foi inicializado. Chame create_hebbian_updater() no startup.")
    return _updater_instance
