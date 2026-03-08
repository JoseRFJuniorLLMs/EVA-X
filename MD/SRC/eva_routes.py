"""
EVA — Rotas FastAPI com integração Hebb
════════════════════════════════════════════════════════════════════
Mostra como plugar o HebbianUpdater no fluxo real de busca do EVA.

Fluxo:
  1. Query chega em POST /memory/search
  2. NietzscheDB retorna nós relevantes
  3. HebbianUpdater.process_session() reforça as arestas dos nós retornados
  4. Zonas "consolidated" são injetadas no contexto da resposta
════════════════════════════════════════════════════════════════════
"""

import os
import logging
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, Depends, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from hebbian_updater import (
    HebbianConfig,
    HebbianUpdater,
    EdgeMemoryZone,
    create_hebbian_updater,
    get_hebbian_updater,
)

logger = logging.getLogger("eva.routes")


# ─────────────────────────────────────────────
# SCHEMAS
# ─────────────────────────────────────────────

class MemorySearchRequest(BaseModel):
    query: str
    session_id: str
    top_k: int = 10
    run_hebb: bool = True          # pode desligar para debug


class MemorySearchResponse(BaseModel):
    results: list[dict]
    hebb_summary: dict             # estatísticas das zonas
    preloaded_context: list[dict]  # vizinhos consolidados


class PruneRequest(BaseModel):
    max_age_days: float = 30.0


# ─────────────────────────────────────────────
# LIFESPAN (startup / shutdown)
# ─────────────────────────────────────────────

@asynccontextmanager
async def lifespan(app: FastAPI):
    # ── Startup ──
    config = HebbianConfig(
        eta=float(os.getenv("HEBB_ETA", "0.05")),
        lambda_decay=float(os.getenv("HEBB_LAMBDA", "0.01")),
        tau_seconds=float(os.getenv("HEBB_TAU_SECONDS", "86400")),
        threshold_consolidated=float(os.getenv("HEBB_THRESHOLD_CONSOL", "0.7")),
        threshold_emerging=float(os.getenv("HEBB_THRESHOLD_EMERG", "0.3")),
    )
    create_hebbian_updater(
        NietzscheDB_uri=os.getenv("NietzscheDB_URI", "bolt://localhost:7687"),
        NietzscheDB_user=os.getenv("NietzscheDB_USER", "NietzscheDB"),
        NietzscheDB_password=os.getenv("NietzscheDB_PASSWORD", "password"),
        config=config,
    )
    logger.info("EVA startup completo.")
    yield

    # ── Shutdown ──
    logger.info("EVA shutdown.")


# ─────────────────────────────────────────────
# APP
# ─────────────────────────────────────────────

app = FastAPI(title="EVA Memory API", lifespan=lifespan)


# ─────────────────────────────────────────────
# ROTA PRINCIPAL: Busca + Hebb
# ─────────────────────────────────────────────

@app.post("/memory/search", response_model=MemorySearchResponse)
async def memory_search(
    req: MemorySearchRequest,
    hebb: HebbianUpdater = Depends(get_hebbian_updater),
):
    """
    Busca na memória episódica do EVA e atualiza pesos Hebb.

    O fluxo pseudo-Hebb acontece DEPOIS da busca,
    reforçando as conexões entre o que foi ativado junto.
    """

    # ── 1. Sua lógica de busca NietzscheDB existente ──────────────────────
    # Substitua este bloco pelo seu código real de busca semântica
    memory_results = await _mock_NietzscheDB_search(req.query, req.top_k)
    activated_ids = [r["id"] for r in memory_results]

    # ── 2. Atualização Hebb ─────────────────────────────────────────
    hebb_zones: list[EdgeMemoryZone] = []
    if req.run_hebb and len(activated_ids) >= 2:
        hebb_zones = await hebb.process_session(activated_ids)

    # ── 3. Pré-carregar contexto consolidado ────────────────────────
    preloaded_context = []
    for node_id in activated_ids[:3]:   # top-3 nós da busca
        neighbors = await hebb.get_top_associations(
            node_id,
            top_k=5,
            min_threshold=hebb.cfg.threshold_consolidated,
        )
        preloaded_context.extend(neighbors)

    # ── 4. Resumo das zonas para observabilidade ────────────────────
    zone_summary: dict[str, int] = {}
    for z in hebb_zones:
        zone_summary[z.zone] = zone_summary.get(z.zone, 0) + 1

    return MemorySearchResponse(
        results=memory_results,
        hebb_summary={
            "total_pairs_updated": len(hebb_zones),
            "zones": zone_summary,
        },
        preloaded_context=preloaded_context,
    )


# ─────────────────────────────────────────────
# ROTA: Pruning periódico
# ─────────────────────────────────────────────

@app.post("/memory/prune")
async def memory_prune(
    req: PruneRequest,
    hebb: HebbianUpdater = Depends(get_hebbian_updater),
):
    """
    Remove arestas fracas e antigas do grafo.
    Chame via cron/scheduler (ex: diariamente às 03:00).
    """
    pruned = await hebb.prune_weak_edges(max_age_days=req.max_age_days)
    return {"pruned_edges": pruned}


# ─────────────────────────────────────────────
# ROTA: Debug — vizinhos de um nó
# ─────────────────────────────────────────────

@app.get("/memory/node/{node_id}/associations")
async def get_associations(
    node_id: str,
    top_k: int = 10,
    hebb: HebbianUpdater = Depends(get_hebbian_updater),
):
    """Inspeciona os vizinhos mais fortes de um nó — útil para debug."""
    neighbors = await hebb.get_top_associations(node_id, top_k=top_k)
    return {"node_id": node_id, "associations": neighbors}


# ─────────────────────────────────────────────
# MOCK — substitua pelo NietzscheDB real
# ─────────────────────────────────────────────

async def _mock_NietzscheDB_search(query: str, top_k: int) -> list[dict]:
    """
    Placeholder: substitua pela sua busca semântica real no NietzscheDB.
    Deve retornar lista de dicts com ao menos {"id": str, ...}.
    """
    return [
        {"id": f"node_{i}", "content": f"Resultado {i} para '{query}'", "score": 1.0 - i * 0.1}
        for i in range(min(top_k, 5))
    ]
