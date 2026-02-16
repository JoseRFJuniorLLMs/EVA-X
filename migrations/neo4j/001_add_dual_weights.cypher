// Migration 001: Add Dual Weight System (DHP)
// Adiciona slow_weight e fast_weight às arestas existentes
// Data: 2026-02-16

// 1. Adicionar propriedades slow_weight e fast_weight às arestas ASSOCIADO_COM
MATCH ()-[r:ASSOCIADO_COM]->()
WHERE r.slow_weight IS NULL OR r.fast_weight IS NULL
SET r.slow_weight = COALESCE(r.slow_weight, r.weight, 0.5),
    r.fast_weight = COALESCE(r.fast_weight, 0.5),
    r.weight = 0.3 * r.slow_weight + 0.7 * r.fast_weight,
    r.slow_ratio = 0.3,
    r.fast_ratio = 0.7,
    r.migrated_at = datetime(),
    r.dhp_migrated = true;

// 2. Adicionar propriedades às arestas CO_ACTIVATED
MATCH ()-[r:CO_ACTIVATED]->()
WHERE r.slow_weight IS NULL OR r.fast_weight IS NULL
SET r.slow_weight = COALESCE(r.slow_weight, r.weight, 0.5),
    r.fast_weight = COALESCE(r.fast_weight, 0.5),
    r.weight = 0.3 * r.slow_weight + 0.7 * r.fast_weight,
    r.slow_ratio = 0.3,
    r.fast_ratio = 0.7,
    r.migrated_at = datetime(),
    r.dhp_migrated = true;

// 3. Criar índice para fast_weight (performance)
CREATE INDEX edge_fast_weight IF NOT EXISTS
FOR ()-[r:ASSOCIADO_COM]-()
ON (r.fast_weight);

CREATE INDEX edge_last_activated IF NOT EXISTS
FOR ()-[r:ASSOCIADO_COM]-()
ON (r.last_activated);

// 4. Verificar migração
MATCH ()-[r:ASSOCIADO_COM|CO_ACTIVATED]->()
WHERE r.dhp_migrated = true
RETURN count(r) AS edges_migrated;

// Output esperado:
// edges_migrated: [número de arestas]

// Rollback (se necessário):
// MATCH ()-[r:ASSOCIADO_COM|CO_ACTIVATED]->()
// REMOVE r.slow_weight, r.fast_weight, r.slow_ratio, r.fast_ratio, r.dhp_migrated, r.migrated_at
// SET r.weight = COALESCE(r.weight, 0.5);
