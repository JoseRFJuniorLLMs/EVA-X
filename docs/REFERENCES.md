# EVA-Memory: Academic References

Algorithms and theoretical foundations used in the EVA-Memory system.

---

## Core Memory Architecture

### REM Sleep Consolidation
- **Rasch, B. & Born, J.** (2013). "About sleep's role in memory." *Physiological Reviews*, 93(2), 681-766.
- Applied in: `internal/memory/consolidation/rem_consolidator.go`
- Usage: Pipeline for converting episodic memories into semantic abstractions via spectral clustering and centroid abstraction during simulated "sleep" cycles.

### Synaptic Homeostasis Hypothesis
- **Tononi, G. & Cirelli, C.** (2006). "Sleep function and synaptic homeostasis." *Sleep Medicine Reviews*, 10(1), 49-62.
- Applied in: `internal/memory/consolidation/pruning.go`
- Usage: Nightly pruning of weak graph edges (~20% per cycle), maintaining network stability while removing low-activation connections.

### Ebbinghaus Forgetting Curve
- **Ebbinghaus, H.** (1885). *Über das Gedächtnis*. Leipzig: Duncker & Humblot.
- Applied in: `internal/cortex/lacan/temporal_decay.go`
- Usage: Exponential decay `weight = frequency × e^(-t/τ)` with τ=90 days for signifier relevance weighting.

### Spaced Repetition (SM-2)
- **Wozniak, P. & Gorzelanczyk, E.** (1994). "Optimization of repetition spacing in the practice of learning." *Acta Neurobiologiae Experimentalis*, 54, 59-62.
- Applied in: `internal/hippocampus/spaced/spaced_repetition.go`
- Usage: Adapted SM-2 algorithm for elderly patients with shorter maximum intervals (30 days) and importance-based initial intervals.

---

## Cognitive Architecture

### Global Workspace Theory
- **Baars, B.J.** (1988). *A Cognitive Theory of Consciousness*. Cambridge University Press.
- Applied in: `internal/cortex/consciousness/global_workspace.go`
- Usage: Competition mechanism where cognitive modules (Lacan, Personality, Ethics) bid for attention; winner broadcasts interpretation globally.

### Zettelkasten Knowledge Management
- **Luhmann, N.** (1992). "Kommunikation mit Zettelkästen." In *Universität als Milieu*, pp. 53-61.
- Applied in: `internal/hippocampus/zettelkasten/zettel_service.go`
- Usage: Linked knowledge cards (11 types) with bidirectional entity-based connections forming an external memory graph.

---

## Psychoanalytic Framework

### Lacanian Signifier Theory
- **Lacan, J.** (1966). *Écrits*. Paris: Éditions du Seuil.
- Applied in: `internal/cortex/lacan/significante.go`, `internal/cortex/transnar/signifier_chain.go`
- Usage: Tracking emotionally-charged recurring words (signifiers) in Neo4j. Interpellation triggers when frequency ≥ 5. RSI (Real, Symbolic, Imaginary) framework in unified retrieval.

### Personality Assessment
- **Funder, D.C.** (1997). *The Personality Puzzle*. W.W. Norton & Company.
- Applied in: `internal/cortex/personality/personality_service.go`
- Usage: Enneagram-based personality modeling with dynamic adaptation based on conversational patterns.

---

## New Implementations (2025-2026)

### Sleep-like Unsupervised Replay (SRC)
- **Tadros, T. et al.** (2022). "Sleep-like unsupervised replay reduces catastrophic forgetting in artificial neural networks." *Nature Communications*, 13, 7742.
- Applied in: `internal/memory/consolidation/selective_replay.go`
- Usage: Selective replay prioritizing high-activation, low-coherence (dissonant) memories to prevent catastrophic forgetting. Dissonance score = activation × (1 - coherence).

### Hebbian Learning
- **Hebb, D.O.** (1949). *The Organization of Behavior*. Wiley.
- Applied in: `internal/memory/consolidation/hebbian.go`
- Usage: "Neurons that fire together, wire together" — co-replayed memory edges receive weight boost during SRC consolidation cycles.

### Reflective Memory Management (RMM)
- **Wang, Y. et al.** (2025). "Reflective Memory Management for LLM Agents." *arXiv:2503.08026*.
- Applied in: `internal/hippocampus/memory/reflective_retrieval.go`
- Usage: Post-retrieval reflection re-ranking results by contextual relevance, temporal coherence, emotional alignment, and contradiction detection.

### Narrative Shift Detection
- **arXiv:2506.14836** (2025). "Detecting Narrative Shifts through Persistent Structures."
- Applied in: `internal/cortex/lacan/narrative_shift.go`
- Usage: Simplified embedding-based detection of abrupt topic changes, emotional flips, and topic circling (rumination). Cross-referenced with Lacan signifier system for clinical relevance.

### Topological Data Analysis in NLP (Survey)
- **Seifert, I. et al.** (2025). "Unveiling Topological Structures from Language: A Survey of TDA in NLP." *arXiv:2411.10298*.
- Referenced for: Persistent homology methods applicable to narrative gap detection and embedding compression.

### NeuroDream Framework
- **SSRN** (2024). "NeuroDream: A Sleep-Inspired Memory Consolidation Framework."
- Status: Theoretical inspiration for future "dream phase" — generative consolidation via latent embedding simulation.

### Autonomous Hippocampus-Neocortex Interactions
- **PNAS** (2022). "A Model of Autonomous Interactions between Hippocampus and Neocortex Driving Memory Consolidation."
- Referenced for: Theoretical validation of EVA's triple store architecture (PostgreSQL=hippocampus, Neo4j/Qdrant=neocortex).

---

## Memory Benchmarking

### Zep Temporal Knowledge Graph
- **Zep** (2025). "Zep: A Temporal Knowledge Graph Architecture for Agent Memory." *arXiv:2501.13956*.
- Applied in: `internal/benchmark/memory_benchmark.go`
- Usage: LongMemEval-inspired benchmark framework measuring Recall@K, MRR, and latency for EVA's retrieval system.

### Long-Term Memory Survey
- **arXiv:2512.13564** (2025). "Long Term Memory Survey."
- Referenced for: Taxonomy positioning EVA-Memory within the state of the art.

---

## Vector Compression

### Krylov Subspace Methods
- **Saad, Y.** (2011). *Numerical Methods for Large Eigenvalue Problems*. SIAM.
- Applied in: `internal/memory/krylov_manager.go`
- Usage: 1536D/3072D → 64D embedding compression via Krylov subspace projection with rank-1 updates. CRC32 checkpoint integrity.
