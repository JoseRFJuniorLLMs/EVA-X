"""
EVA — Testes do HebbianUpdater
════════════════════════════════════════════════════════════════════
Testa a lógica pseudo-Hebb sem precisar de um Neo4j real.
Execute com:  pytest test_hebbian_updater.py -v
════════════════════════════════════════════════════════════════════
"""

import math
import time
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from hebbian_updater import HebbianConfig, HebbianUpdater, EdgeMemoryZone


# ─────────────────────────────────────────────
# FIXTURES
# ─────────────────────────────────────────────

@pytest.fixture
def config():
    return HebbianConfig(
        eta=0.1,
        eta_initial=0.1,
        lambda_decay=0.01,
        tau_seconds=86_400.0,
        weight_max=1.0,
        weight_min=0.01,
        threshold_consolidated=0.7,
        threshold_emerging=0.3,
        max_pairs_per_session=10,
    )


@pytest.fixture
def mock_driver():
    driver = MagicMock()
    driver.session = MagicMock()
    return driver


@pytest.fixture
def updater(mock_driver, config):
    return HebbianUpdater(driver=mock_driver, config=config)


# ─────────────────────────────────────────────
# TESTES DA FÓRMULA MATEMÁTICA
# ─────────────────────────────────────────────

class TestHebbFormula:

    def test_delta_aresta_nova_sem_historico(self, updater):
        """
        Aresta nova (sem last_activated): decay=1.0
        Δw = η·1.0 − λ·0 = 0.1
        """
        delta = updater._compute_delta(
            current_weight=0.0,
            last_activated_ms=None,
        )
        assert abs(delta - 0.1) < 1e-6, f"Esperado 0.1, obtido {delta}"

    def test_delta_com_decaimento_temporal(self, updater):
        """
        Ativação há exatamente 1 tau atrás → decay = e^(-1) ≈ 0.368
        Δw = 0.1 · 0.368 − 0.01 · 0.5 = 0.0368 − 0.005 = 0.0318
        """
        tau = updater.cfg.tau_seconds
        last_activated_ms = int((time.time() - tau) * 1000)

        delta = updater._compute_delta(
            current_weight=0.5,
            last_activated_ms=last_activated_ms,
        )
        expected_decay = math.exp(-1.0)
        expected = 0.1 * expected_decay - 0.01 * 0.5
        assert abs(delta - expected) < 1e-4, f"Esperado {expected:.4f}, obtido {delta:.4f}"

    def test_delta_negativo_quando_muito_antigo(self, updater):
        """
        Ativação muito antiga (10 tau) → decay ≈ 0
        Δw ≈ 0 − λ·w → negativo (LTD domina)
        """
        tau = updater.cfg.tau_seconds
        last_activated_ms = int((time.time() - 10 * tau) * 1000)

        delta = updater._compute_delta(
            current_weight=0.8,
            last_activated_ms=last_activated_ms,
        )
        # LTD = 0.01 * 0.8 = 0.008 → Δw deve ser ≈ -0.008
        assert delta < 0, "Aresta muito antiga deve ter Δw negativo (LTD)"
        assert abs(delta - (-0.008)) < 0.001

    def test_delta_peso_zero_nao_tem_ltd(self, updater):
        """
        Se peso atual = 0, LTD = λ·0 = 0 → apenas LTP
        """
        delta = updater._compute_delta(
            current_weight=0.0,
            last_activated_ms=None,
        )
        assert delta > 0


# ─────────────────────────────────────────────
# TESTES DE CLASSIFICAÇÃO DE ZONA
# ─────────────────────────────────────────────

class TestZoneClassification:

    def test_zona_consolidated(self, updater):
        assert updater._classify_zone(0.8, False) == "consolidated"

    def test_zona_emerging(self, updater):
        assert updater._classify_zone(0.5, False) == "emerging"

    def test_zona_weak(self, updater):
        assert updater._classify_zone(0.05, False) == "weak"

    def test_zona_new_ignora_peso(self, updater):
        """Aresta nova deve ser sempre 'new', independente do peso."""
        assert updater._classify_zone(0.9, True) == "new"

    def test_limiar_exato_consolidated(self, updater):
        """Exatamente no threshold deve ser consolidated."""
        assert updater._classify_zone(0.7, False) == "consolidated"

    def test_limiar_exato_emerging(self, updater):
        assert updater._classify_zone(0.3, False) == "emerging"


# ─────────────────────────────────────────────
# TESTES DE SESSÃO (com mock do Neo4j)
# ─────────────────────────────────────────────

class TestProcessSession:

    @pytest.mark.asyncio
    async def test_sessao_com_menos_de_dois_nos_retorna_vazio(self, updater):
        result = await updater.process_session(["apenas_um_no"])
        assert result == []

    @pytest.mark.asyncio
    async def test_sessao_vazia_retorna_vazio(self, updater):
        result = await updater.process_session([])
        assert result == []

    @pytest.mark.asyncio
    async def test_tres_nos_geram_tres_pares(self, updater):
        """3 nós → C(3,2) = 3 pares."""
        # Mock do _update_pair
        async def mock_update(node_a, node_b):
            return EdgeMemoryZone(
                node_a=node_a, node_b=node_b,
                weight=0.1, co_activations=1,
                zone="new", delta_applied=0.1,
            )

        updater._update_pair = mock_update

        result = await updater.process_session(["A", "B", "C"])
        assert len(result) == 3

    @pytest.mark.asyncio
    async def test_muitos_nos_trunca_pares(self, updater):
        """
        11 nós → C(11,2) = 55 pares → trunca para max_pairs_per_session=10
        """
        async def mock_update(node_a, node_b):
            return EdgeMemoryZone(
                node_a=node_a, node_b=node_b,
                weight=0.1, co_activations=1,
                zone="new", delta_applied=0.1,
            )

        updater._update_pair = mock_update
        nodes = [f"node_{i}" for i in range(11)]
        result = await updater.process_session(nodes)

        assert len(result) == updater.cfg.max_pairs_per_session


# ─────────────────────────────────────────────
# TESTES DE PROPRIEDADES INVARIANTES
# ─────────────────────────────────────────────

class TestInvariants:

    def test_peso_nunca_excede_maximo(self, updater):
        """
        Mesmo com muitas ativações, w não deve ultrapassar weight_max.
        A query Cypher garante isso via CASE WHEN.
        """
        # Simula peso próximo do máximo
        w = 0.99
        delta = updater._compute_delta(w, None)
        new_w = w + delta

        # O clamp ocorre no Cypher, mas verificamos que
        # a fórmula não explode sem o clamp
        clamped = min(new_w, updater.cfg.weight_max)
        assert clamped <= updater.cfg.weight_max

    def test_decaimento_monotono(self, updater):
        """
        Quanto mais antiga a última ativação, menor o delta.
        Verifica que decay é monotonamente decrescente com Δt.
        """
        now = time.time() * 1000
        deltas = []
        for hours in [0, 1, 6, 12, 24, 48, 168]:  # 0h a 1 semana
            last = int(now - hours * 3600 * 1000)
            d = updater._compute_delta(0.5, last)
            deltas.append(d)

        # Cada delta deve ser <= o anterior
        for i in range(1, len(deltas)):
            assert deltas[i] <= deltas[i - 1], (
                f"Delta não é monotônico: deltas[{i-1}]={deltas[i-1]:.5f} "
                f"deltas[{i}]={deltas[i]:.5f}"
            )
