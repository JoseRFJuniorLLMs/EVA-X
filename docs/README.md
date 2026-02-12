# EVA-Mind: Documentacao Tecnica

**Stack:** Go (eva-ai) + Python (eva-backend/Supermemory) + Qdrant + PostgreSQL + Neo4j + Browser Ext
**Servidor:** 104.248.219.200

---

## Indice

### Arquitetura de Memoria & Roadmap

| Documento | Conteudo |
|-----------|----------|
| `eva_universal_roadmap.md` | **[NOVO]** Roadmap do EVA Universal. Integração Deep Mind + Digital Memory. |
| `maldição_dimensionalidade_eva.md` | Evolucao completa da memoria EVA (7 capitulos). De SQL ingenuo ate Subespacos de Krylov. |
| `rank1_update_mathematics.md` | Fundamentos matematicos: Rank-1 Updates, Gram-Schmidt Modificado. |
| `integration_guide_sliding_window.md` | Guia de integracao Sliding Window Krylov no pipeline EVA. |

### Codigo de Referencia

| Arquivo | Conteudo |
|---------|----------|
| `reference/krylov_memory_manager.go.txt` | Implementacao original do KrylovMemoryManager (referencia). Codigo real em `internal/memory/krylov_manager.go`. |
| `reference/sliding_window_krylov.go.txt` | Versao anterior com sliding window. Mantida para referencia historica. |
| `reference/krylov_benchmark_test.go.txt` | Suite de benchmarks original. Testes reais em `internal/memory/krylov_manager_test.go`. |
| `memory_consolidation_api.py` | Endpoint FastAPI para consolidacao de memoria via gRPC. Referencia para o Python client em `clients/python/krylov_client.py`. |

---

## Componentes Implementados

### Krylov Memory Manager (`internal/memory/`)

Coracao do sistema de memoria. Comprime embeddings 1536D -> 64D com 97% recall.

- `krylov_manager.go` - Gram-Schmidt Modificado + Sliding Window FIFO + Rank-1 Updates
- `krylov_manager_test.go` - 12 testes, todos passando
- `grpc_server.go` - gRPC server na porta 50051 (KrylovService)
- `http_bridge.go` - HTTP/JSON bridge na porta 50052 (para FastAPI)

### Spectral Community Engine (`internal/cortex/spectral/`)

Clustering espectral do grafo Neo4j. Fractal para organizacao macro das memorias.

- `community.go` - Graph Laplacian + EigenSym + k-means espectral + persistencia Neo4j
- `fractal_dimension.go` - Dimensao fractal do espectro, lacunaridade, Hurst, classificacao hierarquica
- `community_test.go` - 13 testes + 2 benchmarks (100 nos em 892us, 500 nos em 115ms)

**Papel:** Krylov cuida do MICRO (busca vetorial), Spectral cuida do MACRO (comunidades de memoria).

### HMC Trajectory Engine (`internal/cortex/predictive/`)

Hamiltonian Monte Carlo substitui random walk na predicao de trajetorias clinicas.

- `hmc.go` - HMCSampler: energia potencial, gradiente numerico, leapfrog Stormer-Verlet, Metropolis-Hastings
- `trajectory_engine.go` - Motor de trajetoria com toggle HMC/random walk
- `hmc_test.go` - 6 testes + 3 benchmarks. 88% acceptance rate, |dH| = 0.000033

### Temporal Decay (`internal/cortex/lacan/`)

Envelhecimento temporal das conexoes no grafo Neo4j via e^(-t/tau).

- `temporal_decay.go` - Decay em significantes e relacoes, poda de conexoes, refresh batch

### Legacy Mode (`internal/legacy/`)

Imortalidade digital pos-morte com consent granular.

- `service.go` - LegacyService: ativacao pos-morte, herdeiros, personality snapshots, audit trail
- `026_legacy_mode.sql` - Migration com 4 tabelas, view, stored function

### gRPC Bridge (`proto/krylov/v1/`)

Protocolo Go <-> FastAPI para o KrylovService.

- `krylov.proto` - Protocol Buffers (CompressVector, ReconstructVector, BatchCompress, etc.)
- `clients/python/krylov_client.py` - Client HTTP + FastAPI router factory

---

## Metricas

| Metrica | Valor |
|---------|-------|
| Recall@10 (Krylov) | 97% |
| Compressao | 1536D -> 64D (24x) |
| Update time | 52us/op |
| Spectral clustering 100 nos | 892us |
| HMC acceptance rate | 88% |
| HMC energy conservation | |dH| = 0.000033 |
| Testes passando | 31+ (Krylov 12, HMC 6, Spectral 13) |

---

## Quick Start

```bash
# Build tudo
go build ./...

# Testes Krylov
go test -v ./internal/memory/ -timeout 120s

# Testes HMC
go test -v ./internal/cortex/predictive/ -timeout 60s

# Testes Spectral
go test -v ./internal/cortex/spectral/ -timeout 60s

# Benchmarks
go test -bench=. -benchmem ./internal/memory/
go test -bench=. -benchmem ./internal/cortex/spectral/
```
