# Memory Pipeline Integration - Analysis

## Current State: Components Exist but NOT Integrated ❌

### Components Found

| Component | File | Status |
|-----------|------|--------|
| **FDPN Engine** | `internal/hippocampus/memory/fdpn_engine.go` | ✅ Exists |
| **Krylov Manager** | `internal/memory/krylov_manager.go` | ✅ Exists |
| **REM Consolidator** | `internal/memory/consolidation/rem_consolidator.go` | ✅ Exists |
| **Spectral Clustering** | ❌ NOT FOUND | ❌ Missing |

---

## Problem: Isolated Components

### Current Flow (BROKEN)
```
Voice Input
  ↓
  ? (no integration)
  ↓
FDPN (operates independently)
  ↓
  ? (no connection)
  ↓
Krylov (operates independently)
  ↓
  ? (no connection)
  ↓
REM (scheduled but not connected)
```

### What's Missing
1. **No orchestrator** connecting components
2. **No data flow** between stages
3. **REM scheduler exists** but not properly integrated
4. **Spectral clustering missing** (mentioned in REM but not implemented)

---

## Solution: Memory Orchestrator

### New Flow (FIXED)
```
Voice Input
  ↓
[MemoryOrchestrator.IngestMemory]
  ↓
STEP 1: FDPN.StreamingPrime()
  → Activates relevant subgraphs
  ↓
STEP 2: Krylov.CompressVector()
  → 1536D → 64D compression
  ↓
STEP 3: Krylov.UpdateSubspace()
  → Updates basis with new vector
  ↓
STEP 4: Store in Qdrant (compressed)
  ↓
STEP 5: Store in PostgreSQL

... at 3 AM daily ...

[MemoryScheduler.RunNightlyConsolidation]
  ↓
REM.ConsolidateAll()
  → Cluster similar memories
  → Create semantic nodes
  → Prune redundancies
  ↓
Krylov.MemoryConsolidation()
  → Reorthogonalize basis
```

---

## Files Created

### 1. Memory Orchestrator
**File**: `internal/memory/orchestrator.go`

**Purpose**: Connects all pipeline components

**Key Methods**:
- `IngestMemory()` - Processes new memory through full pipeline
- `RunNightlyConsolidation()` - Executes REM + Krylov consolidation
- `GetPipelineStatus()` - Returns status of all components

### 2. Memory Scheduler
**File**: `internal/memory/scheduler/memory_scheduler.go`

**Purpose**: Schedules periodic memory operations

**Schedules**:
- **3 AM daily**: REM consolidation
- **Every 6h**: Krylov reorthogonalization

---

## Integration Example

### Before (Broken)
```go
// Components exist but don't talk to each other
fdpn.StreamingPrime(ctx, userID, text)  // ❌ Isolated
krylov.CompressVector(embedding)         // ❌ Isolated
// REM never runs automatically             ❌ Not scheduled
```

### After (Fixed)
```go
// Single entry point
orchestrator.IngestMemory(ctx, userID, content, embedding)
// ✅ FDPN activates
// ✅ Krylov compresses
// ✅ Krylov updates subspace
// ✅ Stores in DB

// Automatic nightly consolidation at 3 AM
// ✅ REM consolidates
// ✅ Krylov reorthogonalizes
```

---

## Still Missing: Spectral Clustering

The REM consolidator mentions "Spectral clustering" but uses simple cosine similarity instead.

**Current**: `clusterBySimilarity()` - Simple threshold-based clustering  
**Intended**: Spectral clustering with graph Laplacian

**Recommendation**: Current approach is good enough for MVP. Spectral clustering can be added later if needed.

---

## Next Steps

### To Integrate in main.go

```go
// In NewSignalingServer()

// Create memory orchestrator
memoryOrchestrator := memory.NewMemoryOrchestrator(
    db,
    neo4jClient,
    qdrantClient,
    fdpnEngine,
    krylovManager,
)

// Create and start memory scheduler
memoryScheduler := scheduler.NewMemoryScheduler(memoryOrchestrator)
go memoryScheduler.Start(serverCtx)

// When ingesting new memory (e.g., from voice)
err := memoryOrchestrator.IngestMemory(ctx, userID, content, embedding)
```

---

## Summary

**Before**: Components existed but operated in isolation ❌  
**After**: Unified pipeline with automatic scheduling ✅

**Pipeline**: Voice → FDPN → Krylov → Store → (3 AM) REM → Prune  
**Scheduler**: Runs automatically at 3 AM daily + Krylov maintenance every 6h
