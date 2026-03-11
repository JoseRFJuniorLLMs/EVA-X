# NietzscheDB Integration Guide

**Data**: 2026-03-10

---

## Overview

NietzscheDB is EVA's sole database. All data previously stored in
PostgreSQL, Neo4j, and Qdrant has been migrated to NietzscheDB.

- **gRPC**: port 50051 (primary data plane)
- **HTTP**: port 8080 (dashboard + REST API)
- **Data**: /var/lib/nietzsche/collections/ (~14 collections, ~13 GB)
- **Binary**: /usr/local/bin/nietzsche-server (Rust, GPU-compiled)
- **Config**: /etc/nietzsche.env

---

## Go SDK Usage

EVA uses the Go SDK (`nietzsche-sdk` module) for all database operations.

```go
import nietzsche "nietzsche-sdk"

// Connect
client, err := nietzsche.NewClient("localhost:50051")
defer client.Close()

// Insert node
id, err := client.InsertNode(ctx, nietzsche.InsertNodeOpts{
    ID:         "uuid-here",
    Content:    map[string]interface{}{"key": "value"},
    NodeType:   "Semantic",
    Collection: "eva_mind",
    Embedding:  []float32{0.1, 0.2, ...},  // optional
})

// Query via NQL
result, err := client.Query(ctx, nql, params, "eva_mind")

// KNN Search
results, err := client.KnnSearch(ctx, collection, embedding, k)

// Merge (upsert)
id, err := client.MergeNode(ctx, nietzsche.MergeNodeOpts{
    Collection: "eva_mind",
    NodeType:   "Semantic",
    MatchKeys:  map[string]interface{}{"name": "EVA"},
    OnMatchSet: map[string]interface{}{"energy": 0.9},
    OnCreateSet: map[string]interface{}{"name": "EVA", "energy": 0.5},
})
```

---

## EVA Adapter Layer

### database.DB (internal/brainstem/database/db.go)

Relational-style wrapper over NietzscheDB gRPC:

| Method | Collection | Purpose |
|---|---|---|
| Insert(ctx, table, content) | eva_mind | Patient data, schedules, meds |
| InsertTo(ctx, coll, table, content) | Any | Learnings, curriculum, stories |
| NQL(ctx, nql, params) | eva_mind | Raw NQL queries |
| NQLIn(ctx, coll, nql, params) | Any | NQL on specific collection |
| QueryByLabel(ctx, label, where, params, limit) | eva_mind | Find by node_label |
| QueryByLabelIn(ctx, coll, label, where, params, limit) | Any | Find in specific coll |
| Update(ctx, table, matchKeys, updates) | eva_mind | MergeNode-based update |
| GetNodeByID(ctx, table, pgID) | eva_mind | Direct node lookup |

### VectorAdapter (internal/brainstem/infrastructure/nietzsche/)

| Method | Purpose |
|---|---|
| Upsert(ctx, collection, id, embedding, payload) | Insert/update vector |
| Search(ctx, collection, embedding, k) | KNN search |
| Delete(ctx, collection, id) | Remove vector |

### GraphAdapter

| Method | Purpose |
|---|---|
| InsertEdge(ctx, from, to, edgeType, weight, meta) | Create edge |
| GetNeighbors(ctx, nodeID, direction, edgeType) | Adjacency query |
| BFS(ctx, startID, maxDepth) | Breadth-first traversal |
| Dijkstra(ctx, startID, endID) | Shortest path |

### ManifoldAdapter

| Method | Purpose |
|---|---|
| Synthesis(ctx, thesisID, antithesisID) | Riemann sphere dialectics |
| CausalNeighbors(ctx, nodeID, timeWindow) | Minkowski light-cone |
| KleinPath(ctx, startID, endID) | Klein geodesic pathfinding |

---

## Collections Reference

| Collection | Dim | Metric | Purpose |
|---|---|---|---|
| eva_mind | 3072 | poincare | Primary relational store (migrated from PostgreSQL) |
| eva_core | 3072 | poincare | Interaction graph (conversation turns + edges) |
| memories | 3072 | cosine | Episodic memory with embeddings |
| signifier_chains | 3072 | cosine | Lacanian signifier tracking |
| speaker_embeddings | 192 | cosine | ECAPA-TDNN voiceprints |
| stories | 3072 | cosine | Therapeutic narratives |
| eva_self_knowledge | 3072 | cosine | EVA's identity and capabilities |
| eva_learnings | 3072 | cosine | Autonomous learner output |
| eva_curriculum | 128 | cosine | Study curriculum |
| patient_graph | 3072 | poincare | Patient relationship graphs |
| eva_cache | 2 | cosine | Key-value cache |
| malaria | 3072 | poincare | Malaria Angola clinical data |
| aesop_fables | 3072 | cosine | Aesop's fables |
| zen_koans | 3072 | cosine | Zen koans |

---

## NQL Gotchas

1. **Custom types**: `MATCH (n:User)` only works for 4 built-in types
   (Episodic, Semantic, Concept, DreamSnapshot). Custom types like "User"
   are parsed as Semantic. Use `node_label` field instead.

2. **MergeNode**: `find_node_by_content()` compares string representation
   of NodeType. Must pass `NodeType: "Semantic"` on wire, not custom strings.
   Add `node_label` to matchKeys for safe content-level matching.

3. **HNSW dimension**: All nodes in a collection need same-dimension
   coordinates. Relational data (no embedding) gets zero-filled coords.

4. **Identifiers**: NQL identifiers CANNOT start with `_`.
   Use `node_label` not `_label`.

---

## Retry Strategy

All NietzscheDB operations use the retry package:

| Config | MaxRetries | InitialBackoff | MaxBackoff | Use Case |
|---|---|---|---|---|
| FastConfig | 2 | 50ms | 500ms | Embedding generation, vector upsert |
| DefaultConfig | 3 | 100ms | 10s | General database operations |
| SlowConfig | 5 | 500ms | 30s | Backup, heavy batch operations |

Retryable errors: timeout, connection refused, rate limit, 429/502/503/504.
Permanent errors: invalid, unauthorized, forbidden, not found, 400/401/403/404.

---

## Security Configuration

### Enable RBAC (VM: /etc/nietzsche.env)

```env
# Uncomment to enable authentication:
# NIETZSCHE_API_KEY_ADMIN=<generate-secure-key>
# NIETZSCHE_API_KEY_WRITER=<generate-secure-key>
# NIETZSCHE_API_KEY_READER=<generate-secure-key>
```

### Enable Encryption at Rest

```env
NIETZSCHE_ENCRYPTION_KEY=<base64-encoded-32-byte-key>
```

AES-256-GCM with HKDF-SHA256 per column family.
Random 12-byte nonce per encryption operation.
