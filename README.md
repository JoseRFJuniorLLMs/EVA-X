EVA-Mind
========

<p align="center">
  <img src="assets/eva.jpg" alt="EVA - Virtual Support Entity" width="600">
</p>

	EVA-Mind is an artificial intelligence system for healthcare.
	It was created by Jose R F Junior on June 2, 2025.

	EVA stands for "Entidade Virtual de Apoio" (Virtual Support Entity).
	The system provides real-time voice assistance, clinical decision
	support, and persistent memory through a combination of graph
	databases, vector search, and large language models.

	This is NOT a chatbot. EVA has her own identity, her own memory,
	and her own personality that evolves with every interaction. She
	remembers. She learns. She adapts.


WHAT IS EVA-MIND
----------------

	EVA-Mind is the brain behind EVA. It handles:

	- Real-time bidirectional voice via WebSocket (Gemini Live API)
	- Real-time camera and screen analysis for medical assistance
	- 2D Semantic Perception (continuous object detection via Gemini Vision)
	- Unified Graph & Vector Memory (NietzscheDB) with Hebbian learning
	- Native Hyperbolic embeddings (Poincare ball model)
	- Krylov subspace compression (1536D -> 64D, ~97% precision)
	- Psychoanalytic context modeling (Lacanian FDPN framework)
	- Personality system (Big Five + Enneagram) with evolution
	- Clinical scales (PHQ-9, GAD-7, C-SSRS) via voice
	- Voice prosody analysis (depression, anxiety, Parkinson detection)
	- Speaker identification via ECAPA-TDNN (192D voiceprints)
	- Medication scheduling, alerting, and visual identification
	- Emergency detection, escalation, and crisis prediction
	- Multi-agent swarm (12 agents) with circuit breaker routing
	- Episodic memory with 3072D embeddings + importance scoring
	- REM-inspired memory consolidation with selective replay
	- Global Workspace Theory consciousness model (Baars 1988)
	- Autonomous learning with curriculum-driven study cycles
	- Narrative therapy via seeded therapeutic stories
	- FHIR R4 interoperability and MCP server (44 tools)
	- Multi-language support (30+ languages)

	The system currently serves two deployments:

	1. Elderly care (original) - Voice calls via Twilio for
	   medication reminders, psychological support, and crisis
	   detection with caregiver escalation.

	2. Malaria detection (Angola) - Real-time voice and camera
	   assistance for healthcare workers diagnosing malaria
	   from microscope images.


ARCHITECTURE
------------

	EVA-Mind follows a neuroscience-inspired architecture.
	Each directory maps to a brain region:

	brainstem/     - Configuration, database, auth, push notifications,
	                 infrastructure (NietzscheDB adapters, retry, worker pools)

	cortex/        - Higher-order processing:
	  gemini/        - Gemini Live API client (v1beta, thread-safe,
	                   callbacks, VAD, memory, tools)
	  llm/           - Multi-LLM provider (Claude, GPT, DeepSeek)
	  skills/        - Runtime skills engine (create, execute, version)
	  lacan/         - Lacanian psychoanalytic engine (demand/desire
	                   analysis, signifier chains, narrative shift
	                   detection, Grand Autre, transference, FDPN,
	                   UnifiedRetrieval with RSI binding)
	  personality/   - Big Five traits, Enneagram types, trait relevance
	  self/          - Core memory engine, post-session reflection
	  attention/     - Affect stabilizer, confidence gate, executive
	                   attention, pattern interrupt, triple attention,
	                   wavelet attention
	  consciousness/ - Cognitive Operating System kernel:
	                   Global Workspace Theory (Baars 1988),
	                   ThoughtBus (cognitive pub/sub barramento),
	                   Attention Scheduler (dynamic thresholding)
	  evolution/     - Zaratustra evolution engine (Will to Power,
	                   energy propagation, autonomous snapshots)
	  activeinference/ - Free Energy Principle (Friston) for gap
	                   detection and cognitive self-correction
	  prediction/    - Bayesian networks, crisis prediction
	  predictive/    - HMC (Hamiltonian Monte Carlo) sampler
	  ram/           - Retrieval-Augmented Memory with feedback loop
	  scales/        - Clinical scales (PHQ-9, GAD-7, C-SSRS)
	  ethics/        - Ethical boundary engine
	  explainability/- Clinical decision explainer, PDF report generation
	  cognitive/     - Cognitive load orchestrator
	  learning/      - Continuous learning, meta-learner, self-eval loop,
	                   autonomous learner with curriculum management
	  medgemma/      - Medical image analysis (prescriptions, exams)
	  spectral/      - Community detection, fractal dimension analysis,
	                   synaptogenesis engine
	  pattern/       - Behavioral cue detector
	  veracity/      - Lie detector, inconsistency types, response strategy
	  voice/         - Prosody analyzer (pitch, rhythm, pauses, tremor),
	                   speaker embedder (ECAPA-TDNN, ONNX, 192D)
	  transnar/      - A/B testing, desire detector, inference engine,
	                   signifier chain, response generator
	  kids/          - Kids mode (adapted conversation)
	  situation/     - Situational modulator (context-aware personality)
	  orchestration/ - Conversation orchestrator
	  alert/         - Escalation engine
	  llm/thinking/  - LLM thinking detector, audit, notification
	  brain/         - Episodic memory service with context metadata,
	                   multi-factor importance scoring, embedding
	                   generation with retry/backoff

	hippocampus/   - Knowledge graphs and memory:
	  memory/        - Graph store, retrieval service, FDPN engine,
	                   Hebbian real-time updates, dual weights,
	                   edge zones, embeddings, entity resolver,
	                   pattern miner, priming engine, reflective
	                   retrieval, context builder zones
	  memory/superhuman/ - Consciousness service, critical memory,
	                   deep memory, Enneagram service, Lacanian
	                   mirror, narrative weaver, self core service
	  knowledge/     - Context service, embedding cache/service,
	                   graph reasoning, self knowledge, wisdom service,
	                   audio analysis
	  habits/        - Habit tracker
	  spaced/        - Spaced repetition engine (SM-2 algorithm)
	  stories/       - Therapeutic story repository
	  topology/      - Persistent homology (topological data analysis)
	  zettelkasten/  - Entity extractor, zettel service
	  graph/         - Heat kernel page rank

	senses/        - Input processing:
	  signaling/     - WebSocket handler, personality context
	  voice/         - Voice client
	  reconnection/  - Connection manager
	  telemetry/     - Psychological metrics
	  proprioception/ - Cognitive self-awareness engine:
	                   graph state scanning, system prompt injection,
	                   15-minute auto-refresh, BuildSystemPrompt()

	motor/         - Output, actions, and integrations:
	  scheduler/     - Medication scheduling
	  actions/       - Action executor
	  calendar/      - Google Calendar integration
	  computeruse/   - Computer use service
	  docs/          - Google Docs integration
	  drive/         - Google Drive integration
	  email/         - Email client, sender, templates
	  gmail/         - Gmail integration
	  googlefit/     - Google Fit (health data)
	  maps/          - Google Maps integration
	  sheets/        - Google Sheets integration
	  sms/           - SMS client, sender, voice
	  spotify/       - Spotify integration
	  uber/          - Uber integration
	  whatsapp/      - WhatsApp integration
	  youtube/       - YouTube integration
	  telegram/      - Telegram Bot API client
	  sandbox/       - Code execution sandbox (bash, Python, Node.js)
	  browser/       - Browser automation (navigate, forms, extract)
	  cron/          - Scheduled task engine (cron-like scheduler)
	  messaging/     - Multi-platform messaging (Slack, Discord,
	                   Microsoft Teams, Signal)
	  smarthome/     - Home Assistant IoT integration
	  webhooks/      - Webhook management (create, trigger, HMAC)
	  filesystem/    - Sandboxed file access (read, write, search)
	  selfcode/      - Self-programming (git branches, commit, test)
	  vision/        - Medication identifier, WebSocket handler
	  perception/   - 2D Semantic Perception engine (camera -> Gemini
	                   Vision -> object detection -> NietzscheDB graph
	                   in Poincare ball with TTL-based ego-cache)
	  workers/       - Pattern worker, prediction worker

	gemini/        - Lean Gemini WebSocket client (v1alpha, browser
	                 sessions and alerts, stateless)

	voice/         - Voice session management:
	  handler/       - Twilio media stream handler
	  session/       - Session lifecycle management
	  alert_service/ - Emergency alert service
	  media_handler/ - Audio DSP and processing

	tools/         - Gemini function calling:
	  definitions/   - Tool schemas (vitals, agendamentos,
	                   medication scan, prosody analysis,
	                   PHQ-9, GAD-7, C-SSRS, user directives)
	  handlers/      - Tool execution
	  discovery/     - Dynamic tool discovery
	  architect/     - Architect override tool
	  repository/    - Tool registry

	swarm/         - Multi-agent coordination:
	  orchestrator/  - Routes tool calls to correct agent
	  registry/      - Agent registration and discovery
	  circuit_breaker/ - Fault tolerance per agent
	  cellular_division/ - Agent spawning
	  Agents:
	    clinical/      - Clinical decision support
	    educator/      - Patient education
	    emergency/     - Emergency protocols
	    entertainment/ - Entertainment and distraction
	    external/      - External service calls
	    google/        - Google services integration
	    kids/          - Pediatric mode
	    legal/         - Legal compliance
	    productivity/  - Task management
	    scholar/       - Autonomous learning (curriculum + web research)
	    selfawareness/ - Introspection and self-monitoring
	    wellness/      - Wellness monitoring

	memory/        - Memory consolidation:
	  consolidation/ - Hebbian strengthening, pruning, REM consolidator,
	                   selective replay
	  krylov/        - Krylov subspace compression (Rank-1 updates,
	                   Gram-Schmidt, sliding window FIFO, 1536D->64D)
	  importance/    - Memory importance scorer
	  ingestion/     - Atomic fact extraction, ingestion pipeline
	  orchestrator/  - Memory orchestration
	  scheduler/     - Memory consolidation scheduler
	  grpc_server/   - gRPC interface for memory service
	  http_bridge/   - HTTP bridge for Krylov compression

	Other modules:

	mcp/           - Model Context Protocol server
	                 (resources, tools/remember, tools/recall, prompts)
	integration/   - HL7 FHIR R4 adapter, webhooks, export, serializers
	security/      - CORS, validation, error handling, multitenancy
	audit/         - LGPD compliance, data rights
	research/      - Anonymization, cohort builder, longitudinal
	                 analysis, statistical methods
	clinical/      - Crisis protocol, pediatric risk detector,
	                 silence detector, goals tracker, family tracker,
	                 clinical notes generator, clinical synthesis
	monitoring/    - Prometheus metrics (memory, retrieval, Krylov,
	                 consolidation, FDPN, personality, attention)
	telemetry/     - Structured logging, metrics
	persona/       - Persona manager (per-patient personalities)
	subscription/  - Subscription service
	legacy/        - Legacy compatibility
	exit/          - Exit protocol manager
	multimodal/    - Image processor, video processor, video stream,
	                 multimodal session


	Two Gemini WebSocket clients exist:

	internal/gemini/         - Lean client for browser sessions and
	                          alerts. Uses v1alpha API. Stateless.

	internal/cortex/gemini/  - Full-featured client for production
	                          voice calls. Uses v1beta API. Thread-safe
	                          with mutex, callbacks, VAD tuning, memory
	                          injection, and function calling support.

	Both are actively used. They are not duplicates.


NIETZSCHEDB — UNIFIED DATABASE
-------------------------------

	NietzscheDB is EVA's sole database. A multi-manifold hyperbolic
	graph database written in Rust with 45+ specialized crates.
	It provides graph, vector, cache, and SQL in a single gRPC endpoint.

	Server: /usr/local/bin/nietzsche-server (Rust binary)
	gRPC:   port 50051 (65+ RPCs in a unified service)
	HTTP:   port 8080 (React dashboard + WebSocket streaming)
	Config: /etc/nietzsche.env

	Core capabilities:

	1. Hyperbolic Geometry (Poincare Ball)
	   - All embeddings stored as points inside the unit ball (||x|| < 1)
	   - Hierarchical depth encoded as distance to origin
	   - Mobius addition for gyrovector operations
	   - exp_map_zero / log_map_zero for Euclidean <-> Poincare
	   - Frechet mean (gyromidpoint) for multimodal fusion

	2. Multi-Manifold Support
	   - Poincare (K < 0): Storage, distance, KNN — primary manifold
	   - Klein (K < 0): Pathfinding, collinearity checks
	   - Riemann/Sphere (K > 0): Dialectical synthesis, aggregation
	   - Minkowski (pseudo-Riemann): Causal filtering, light cones

	3. HNSW Vector Search
	   - CPU: Full HNSW with metadata filtering (RoaringBitmap)
	   - GPU: NVIDIA cuVS CAGRA (10x faster build, millions QPS)
	   - Quantization: None (f64), ScalarI8 (~5% error), Binary (1-bit)
	   - Metrics: Poincare, Euclidean (L2), Cosine, Lorentz
	   - CRITICAL: Binary Quantization REJECTED for hyperbolic metrics
	     (sign(x) destroys magnitude hierarchy in Poincare ball)

	4. NQL (Nietzsche Query Language)
	   - PEG-based parser with full WHERE/ORDER/LIMIT support
	   - MATCH (n:Label) for node scanning (4 built-in types)
	   - HYPERBOLIC_DIST() for KNN within NQL
	   - DIFFUSE for Chebyshev heat-kernel diffusion walks
	   - RECONSTRUCT for sensory data decompression
	   - Workaround: Custom types stored via node_label field +
	     RewriteNQL() in db.QueryNodes()

	5. Autonomous Cognitive Cycles (Background Schedulers)
	   - Sleep Cycle: Riemannian reconsolidation with RiemannianAdam
	     optimizer, Hausdorff dimension monitoring, checkpoint/rollback
	   - Zaratustra Engine: Will to Power (energy propagation),
	     Eternal Recurrence (temporal echo snapshots),
	     Ubermensch (elite hot-cache promotion)
	   - L-System: Fractal growth rules, spawn_child via Mobius
	     placement, tumor detection circuit breaker
	   - Agency Engine: 10+ subsystems including entropy/coherence
	     daemons, gap detection, desire signals, shatter protocol,
	     world model, Hebbian LTP, cognitive thermodynamics
	   - Wiederkehr Daemon: Autonomous agent ticking (30s interval)
	   - TTL Reaper: Expired node cleanup (60s interval)

	6. Storage (RocksDB with 6 Column Families)
	   - CF_NODES: node_id -> NodeMeta (bincode)
	   - CF_EDGES: edge_id -> Edge (bincode)
	   - CF_ADJ: node_id -> adjacency (JSON)
	   - CF_META: health reports, daemon state
	   - CF_EGO: ego-cache depth-2 subgraphs
	   - CF_BACKUP: snapshot checksums
	   - Encryption: AES-256-GCM with HKDF-SHA256 per column family

	7. Additional Features
	   - CDC (Change Data Capture) with WebSocket streaming
	   - Swartz embedded SQL layer (rusqlite-style)
	   - Graph algorithms: PageRank, Louvain, Betweenness, A*, SCC, WCC
	   - DreamerV3 dream simulation engine
	   - GNN inference with ONNX Runtime
	   - MCTS (Monte Carlo Tree Search) advisor
	   - Schema validation with FieldType constraints
	   - Secondary indexes on metadata fields
	   - Cluster mode with gossip protocol (optional)
	   - Backup/restore with retention pruning

	8. Security
	   - API Key auth (SHA-256 hashed, constant-time comparison)
	   - RBAC: Admin / Writer / Reader (3-tier hierarchy)
	   - AES-256-GCM encryption at rest (HKDF-derived per-CF keys)
	   - Input validation: embedding dim caps, NQL injection prevention,
	     NaN/Inf rejection, UUID format enforcement
	   - Collection-level namespace isolation

	Data model:
	  Node: { id (UUID), node_type, energy (f64), depth (f64),
	          hausdorff_local (f64), created_at, expires_at,
	          valence, arousal, content (JSON) }
	  Edge: { from, to, edge_type, weight, created_at, metadata }
	  Schrodinger edges: probabilistic with entanglement


NIETZSCHEDB COLLECTIONS
-----------------------

	EVA uses 14 collections in NietzscheDB, each with a specific
	purpose, dimension, and distance metric:

	eva_mind              3072D  poincare  Primary relational store.
	                                       All patient data, schedules,
	                                       medications, clinical records.
	                                       Migrated from PostgreSQL.

	eva_core              3072D  poincare  EVA's interaction graph.
	                                       Conversation turns, session
	                                       edges, speaker relationships.

	memories              3072D  cosine    Episodic memory with full
	                                       embeddings. Each conversation
	                                       turn gets a 3072D Gemini vector
	                                       + importance score + emotion.

	signifier_chains      3072D  cosine    Lacanian signifier tracking.
	                                       Populated by UnifiedRetrieval
	                                       during Prime() calls.

	speaker_embeddings    192D   cosine    ECAPA-TDNN voiceprints.
	                                       Used for speaker identification
	                                       and voice biometrics.

	stories               3072D  cosine    Therapeutic narratives for
	                                       wisdom service. Fables, koans,
	                                       Rumi poems, African tales.

	eva_self_knowledge    3072D  cosine    EVA's self-knowledge base.
	                                       Identity, capabilities,
	                                       personality description.

	eva_learnings         3072D  cosine    Autonomous learner output.
	                                       Topics studied, summaries
	                                       from web research.

	eva_curriculum        128D   cosine    Study curriculum for the
	                                       autonomous learner. Topics
	                                       with priority and category.

	patient_graph         3072D  poincare  Patient relationship graphs.
	                                       Family, caregiver, doctor
	                                       connections.

	eva_cache             2D     cosine    Fast key-value cache for
	                                       temporary data.

	eva_perceptions       128D   poincare  2D Semantic Perception.
	                                       Camera frames analyzed by
	                                       Gemini Vision produce Scene2D
	                                       and Object2D nodes. TTL-based
	                                       ego-cache (30s). Spatial and
	                                       temporal edges. Hebbian links
	                                       to concepts in other collections.

	malaria               3072D  poincare  Malaria Angola clinical data.
	                                       Separate collection for the
	                                       malaria detection service.

	aesop_fables          3072D  cosine    Aesop's fables collection.
	zen_koans             3072D  cosine    Zen koan collection.


EPISODIC MEMORY PIPELINE (brain.Service)
-----------------------------------------

	Every conversation turn now flows through the episodic memory
	pipeline. This was implemented in the 2026-03-10 memory audit
	(8-phase fix) to ensure EVA actually forms long-term memories.

	Pipeline:

	  User speaks
	    |
	    v
	  handleBrowserVoice() / handleEvaChat()
	    |
	    +--> evaMemory.StoreTurn()          [eva_core graph node]
	    |
	    +--> brain.SaveEpisodicMemoryWithContext()  [ASYNC, non-blocking]
	           |
	           +--> calculateImportance()    [multi-factor scoring]
	           |     Base 0.5 + emotion + urgency + intensity + content
	           |     Factors: Lacanian keywords (+0.20), personal
	           |     relations (+0.15), medical urgency (+0.15),
	           |     temporal references (+0.10), object location (+0.10)
	           |     Cap at 1.0
	           |
	           +--> embeddingService.GenerateEmbedding()  [with retry]
	           |     Retry: FastConfig (2 attempts, 50ms backoff)
	           |     Fallback: saves without vector if embedding fails
	           |
	           +--> memoryStore.Store()      [with retry]
	                 |--> NietzscheDB Insert (eva_mind, episodic_memories)
	                 |--> GraphStore edges (speaker, session, temporal)
	                 +--> VectorAdapter.Upsert (memories collection)
	                       Retry: FastConfig (2 attempts, 50ms backoff)

	Source files:
	  main.go                                - brain.Service creation
	  browser_voice_handler.go               - Voice turn -> brain pipeline
	  eva_handler.go                         - Text turn -> brain pipeline
	  internal/cortex/brain/memory_context.go - Importance + embedding + store
	  internal/hippocampus/memory/storage.go  - MemoryStore with retry

	MemoryContext metadata per turn:
	  Emotion        string    - Detected emotion (e.g., "happy", "sad")
	  Urgency        string    - Level (e.g., "ALTA", "CRITICA", "LOW")
	  Keywords       []string  - Extracted keywords
	  Importance     float64   - Calculated importance (0-1)
	  AudioIntensity int       - Voice intensity 1-10 from DSP


DATABASE ABSTRACTION LAYER
--------------------------

	The database layer (internal/brainstem/database/db.go) wraps
	NietzscheDB gRPC with a relational-style API. All data is stored
	as nodes with a node_label field for table routing.

	ID Generation: Deterministic UUID v5 from "eva_mind:table:pgID"
	               (matches the PostgreSQL migration tool format).
	Auto-IDs: atomic int64 counter (time.Now().Unix() * 1000 + N).

	Collection routing:

	  db.Insert(ctx, table, content)
	    -> Always writes to eva_mind collection.
	    -> Used for patient data, schedules, medications.

	  db.InsertTo(ctx, collection, table, content)
	    -> Writes to a SPECIFIC collection.
	    -> Used for eva_learnings, eva_curriculum, stories, etc.
	    -> ID format: "collection:table:id" (unique per collection).

	  db.NQL(ctx, nql, params)
	    -> Executes NQL against eva_mind.

	  db.NQLIn(ctx, collection, nql, params)
	    -> Executes NQL against a SPECIFIC collection.

	  db.QueryByLabel(ctx, label, extraWhere, params, limit)
	    -> Finds nodes by node_label in eva_mind.

	  db.QueryByLabelIn(ctx, collection, label, extraWhere, params, limit)
	    -> Finds nodes by node_label in a SPECIFIC collection.

	Type helpers: GetString, GetInt64, GetBool, GetFloat64, GetTime,
	              GetTimePtr, GetNullBool, GetNullString.

	Indexes: node_label, idoso_id, status, cpf_hash, email,
	         session_id, medication_id, ativo, tipo, sender.


INFRASTRUCTURE ADAPTERS
-----------------------

	EVA communicates with NietzscheDB through typed adapters:

	VectorAdapter  - KNN search, Upsert, Delete on vector collections.
	                 Wraps NietzscheDB gRPC KnnSearch and InsertNode
	                 with embedding coordinates.

	GraphAdapter   - InsertEdge, GetNeighbors, BFS, Dijkstra.
	                 Typed edge operations on the graph layer.

	ManifoldAdapter - Multi-manifold operations:
	                  Synthesis (Riemann sphere dialectics),
	                  CausalNeighbors (Minkowski light-cone filtering),
	                  KleinPath (Klein geodesic pathfinding).

	CacheAdapter   - CacheSet/CacheGet/CacheDel with TTL.
	                 Uses eva_cache collection (2D cosine).

	SecurityAdapter - LGPD/HIPAA compliance layer.
	                  Encryption at rest via NietzscheDB AES-256-GCM.

	WiederkehrAdapter - Daemon agent management.
	                    Create/list/tick autonomous agents.

	BackupService  - Snapshot/restore via NietzscheDB backup RPCs.

	CDCListener    - Change Data Capture subscription.
	                 WebSocket bridge for real-time event streaming.

	RetryPackage   - Exponential backoff with jitter:
	                 FastConfig: 2 retries, 50ms initial, 500ms max
	                 DefaultConfig: 3 retries, 100ms initial, 10s max
	                 SlowConfig: 5 retries, 500ms initial, 30s max
	                 Error classification: IsRetryable() checks for
	                 timeout, connection refused, rate limit, 429/502/503/504.
	                 Permanent errors (400/401/403/404/422) are not retried.

	Source: internal/brainstem/infrastructure/nietzsche/
	        internal/brainstem/infrastructure/retry/retry.go


MEMORY SYSTEM
-------------

	EVA's memory is inspired by neuroscience. It has 10 layers:

	1. Episodic Memory (brain.Service)
	   - Every conversation turn generates a 3072D Gemini embedding.
	   - Stored in the 'memories' collection via VectorAdapter.Upsert.
	   - Multi-factor importance scoring (emotion + urgency + intensity
	     + content analysis). Score range: 0.5 to 1.0.
	   - Retry with exponential backoff on transient failures.
	   - Async (goroutine) to avoid blocking voice responses.

	2. Graph Memory (eva_core)
	   - Conversation turns stored as graph nodes with edges:
	     speaker -> turn, turn -> session, temporal ordering.
	   - Hebbian real-time weight updates after every retrieval
	     (eta=0.01, lambda=0.001, tau=86400s).
	   - Synaptogenesis: automatic edge creation via preferential
	     attachment, triadic closure, and homophily.
	   - Science: Bullmore & Sporns (2012), Holtmaat & Svoboda (2009).

	3. Signifier Chains (Lacanian)
	   - UnifiedRetrieval.Prime() extracts signifier chains from
	     user speech during each interaction.
	   - Stored in the 'signifier_chains' collection (3072D cosine).
	   - Tracks demand/desire dynamics, narrative shifts, and
	     transference patterns across sessions.

	4. REM Consolidation (Periodic)
	   - Sleep-inspired memory consolidation pipeline.
	   - Hot episodic memories -> selective replay -> spectral
	     clustering -> Krylov centroid -> semantic NietzscheDB node.
	   - Prunes redundant memories, creates abstractions.
	   - Scheduled at 3am daily via memory scheduler.
	   - Science: Rasch & Born (2013), Tadros et al. (2022).

	5. Hebbian Learning
	   - Real-time: updates weights after every retrieval query.
	   - Consolidation: batch strengthening during REM cycles.
	   - Dual plasticity: Zenke & Gerstner (2017) model.
	   - Edge zones: hot (recent, high energy) vs cold (aged, low).

	6. Krylov Subspace Compression
	   - Compresses 1536D embeddings to 64D (~97% precision).
	   - Rank-1 updates with Modified Gram-Schmidt orthogonalization.
	   - Sliding window FIFO for continuous learning.
	   - 4-level hierarchy: Features / Concepts / Themes / Schemas.
	   - HTTP bridge on port 50052 for external access.
	   - Scheduled every 6 hours via memory scheduler.

	7. EVA's Own Memory (Self Model)
	   - EvaSelf node with Big Five personality traits.
	   - CoreMemory nodes from post-session reflection.
	   - MetaInsight nodes from cross-session pattern detection.
	   - All data anonymized, no PII.
	   - Personality evolves based on cumulative experience.

	8. Spaced Repetition
	   - SM-2 algorithm for optimized recall scheduling.
	   - Important memories reviewed at increasing intervals.
	   - Tracks ease factor and repetition count per memory.

	9. Topological Analysis
	   - Persistent homology for memory graph structure.
	   - Detects topological features (holes, loops, clusters)
	     in the memory graph.

	10. Habit Tracking
	    - Monitors recurring patterns in user behavior.
	    - Streak tracking, frequency analysis.
	    - Integrated with medication adherence monitoring.


CONTEXT PIPELINE
----------------

	For each voice session, EVA's context is assembled from
	multiple sources in parallel:

	1. Lacanian analysis of the user's speech
	   - Demand vs. desire detection
	   - Signifier chain extraction
	   - Narrative shift detection
	   - Grand Autre transference analysis
	2. Medical context from NietzscheDB (conditions, medications)
	3. Patient metadata from NietzscheDB (name, language, persona)
	4. Scheduled medications from agendamentos table
	5. Recent episodic memories (last 15 turns, 7-day window)
	6. Semantic signifier chains from NietzscheDB
	7. Therapeutic stories from wisdom knowledge base
	8. Situational context (time of day, recent events, stressors)
	9. Personality modulation (Big Five + Enneagram + situation)
	10. EVA's own memories (anonymized cross-patient insights)

	All of this is merged into a single system instruction
	sent to the Gemini WebSocket API.


LACANIAN ENGINE
---------------

	EVA's psychoanalytic framework implements Lacanian theory:

	1. FDPN (Four Discourses + Proper Names)
	   - Spreading activation network for signifier chains.
	   - Maps desire dynamics across conversation turns.
	   - Activation decay with configurable parameters.

	2. UnifiedRetrieval (The Sinthome)
	   - Binds the RSI registers (Real, Symbolic, Imaginary).
	   - Parallel execution: Graph (Symbolic) + Vector (Imaginary) +
	     Causal (Real) pathways via NietzscheDB manifolds.
	   - Prime() method: extracts signifier chains from user input,
	     stores in signifier_chains collection, tracks narrative shifts.
	   - Dynamic weights based on active inference gap detection.

	3. Narrative Shift Detection
	   - Detects when user's discourse changes topic or affect.
	   - Triggers Riemannian conflict synthesis via Klein manifold.

	4. Transference Analysis
	   - Models the user's unconscious transfer patterns.
	   - Grand Autre (Big Other) position tracking.

	Source: internal/cortex/lacan/


PERSONALITY SYSTEM
------------------

	EVA's personality is dynamic and multi-dimensional:

	1. Big Five (OCEAN Model)
	   - Openness, Conscientiousness, Extraversion,
	     Agreeableness, Neuroticism.
	   - Each trait: 0.0 to 1.0, evolves with interactions.
	   - Trait relevance scoring per conversation context.

	2. Dynamic Enneagram
	   - 9-type probabilistic distribution (not a single type).
	   - Weights shift based on interaction patterns.
	   - Integration/disintegration arrows modeled.

	3. Situational Modulator
	   - Context-aware personality adjustment (<10ms latency).
	   - Factors: time of day, user mood, crisis state, topic.

	4. RAM Engine (Retrieval-Augmented Memory)
	   - 3-phase loop: Interpretation -> Validation -> Feedback.
	   - Adjusts response style based on personality + context.

	5. Energy Overlay
	   - Zaratustra-compatible energy scoring for personality traits.
	   - Will to Power propagation through personality graph.

	Source: internal/cortex/personality/
	        internal/cortex/situation/
	        internal/cortex/ram/


VOICE SYSTEM
------------

	EVA has a sophisticated voice processing pipeline:

	1. Gemini Live API (Primary)
	   - Model: gemini-2.5-flash-native-audio-preview-12-2025
	     (UNTOUCHABLE — changing this model BREAKS voice)
	   - Bidirectional WebSocket streaming (v1beta API)
	   - Native audio input/output (no transcription needed)
	   - Voice Activity Detection (VAD) tuning
	   - Function calling support for tools during conversation

	2. Speaker Identification (ECAPA-TDNN)
	   - ONNX model producing 192-dimensional voiceprints
	   - Stored in speaker_embeddings collection (192D cosine)
	   - Cosine similarity for speaker matching
	   - Enrollment: 3-5 samples per speaker recommended

	3. Voice Prosody Analysis (DSP Pipeline)
	   - MFCC feature extraction (mel-frequency cepstral coefficients)
	   - Pitch tracking (F0 estimation, jitter, shimmer)
	   - Rhythm analysis (speech rate, pause patterns)
	   - Timbre analysis (spectral envelope, formants)
	   - Clinical markers: depression, anxiety, Parkinson tremor,
	     dehydration (dry voice detection)

	4. Audio Processing
	   - Input: PCM 16kHz mono from browser/Twilio
	   - Output: PCM 24kHz mono from Gemini
	   - WebSocket transport: base64-encoded chunks
	   - Real-time streaming with sub-second latency

	Source: internal/cortex/voice/speaker/ (ECAPA-TDNN embedder)
	        internal/cortex/voice/ (prosody analyzer)
	        internal/cortex/gemini/ (Gemini Live client)
	        internal/voice/ (Twilio session management)


AUTONOMOUS AGENT
----------------

	EVA is a full autonomous agent comparable to OpenClaw. She can
	perceive, decide, and act across platforms without human
	intervention. All capabilities are voice-activated and
	non-blocking (goroutine + WebSocket notification pattern).

	150+ tools organized in 12 categories:

	Communication (7 channels):
		send_email             - Gmail API (compose, send)
		send_whatsapp          - Meta Graph API
		send_telegram          - Telegram Bot API
		send_slack             - Slack Web API
		send_discord           - Discord Bot API
		send_teams             - Microsoft Teams webhooks
		send_signal            - Signal via signal-cli

	Media & Entertainment:
		search_videos          - YouTube Data API
		play_music             - Spotify Web API (search, play)
		play_video             - Send video to Flutter player
		show_webpage           - Embedded WebView in app

	Productivity:
		manage_calendar_event  - Google Calendar API (create, list)
		save_to_drive          - Google Drive API
		find_nearby_places     - Google Maps/Places API
		set_alarm              - Local alarm system
		create_scheduled_task  - Cron-like task scheduler
		list_scheduled_tasks   - List active scheduled tasks
		cancel_scheduled_task  - Cancel scheduled task

	Code Execution Sandbox:
		execute_code           - Run bash, Python, or Node.js
		                         in sandboxed environment with
		                         timeout, safe env, output capture

	Browser Automation:
		browser_navigate       - Fetch URL, extract title/text/links
		browser_fill_form      - Submit form data via POST
		browser_extract        - Extract specific data from pages
		web_search             - Web research via autonomous learner
		browse_webpage         - Browse and summarize URL content

	Self-Programming (OpenClaw-style):
		edit_my_code           - Edit EVA's own source code
		create_branch          - Create git branch (eva/* only)
		commit_code            - Git commit (eva/* branches only)
		run_tests              - Execute go test ./... with timeout
		get_code_diff          - Show uncommitted changes (git diff)

	Database Access:
		query_nietzsche        - NietzscheDB NQL + gRPC API

	Filesystem:
		read_file              - Read file from sandbox directory
		write_file             - Write file to sandbox directory
		list_files             - List directory contents
		search_files           - Search files by name pattern

	Multi-LLM:
		ask_llm                - Query Claude, GPT, or DeepSeek
		                         for second opinion or delegation

	Smart Home (IoT):
		smart_home_control     - Control devices via Home Assistant
		                         (lights, switches, climate, etc.)
		smart_home_status      - Get device state or list all devices

	Webhooks:
		create_webhook         - Register outgoing webhook with
		                         HMAC-SHA256 signature
		list_webhooks          - List registered webhooks
		trigger_webhook        - Fire webhook manually

	Runtime Skills (Self-Improving):
		create_skill           - Create new capability as script
		                         (bash, Python, or Node.js)
		list_skills            - List available skills
		execute_skill          - Run a skill with arguments
		delete_skill           - Remove a skill

	Skills are stored as JSON on disk and persist across restarts.
	EVA can autonomously create new skills to extend her own
	capabilities without requiring a rebuild or restart.

	Tools are split into two tiers:

	Production tools (always active):
		Web search, email, calendar, drive, maps, YouTube,
		Spotify, WhatsApp, Telegram, scheduled tasks, MCP
		memory tools, ask_llm, and Google Search grounding.

	Debug-only tools (ENVIRONMENT=development):
		Filesystem access, self-coding, database queries,
		code execution, browser automation, smart home,
		webhooks, and runtime skills.


CLINICAL TOOLS
--------------

	EVA can execute clinical tools via Gemini function calling:

	get_vitals             - Retrieve patient vital signs
	                         (blood pressure, glucose, heart rate,
	                         oxygen saturation, weight, temperature)

	get_agendamentos       - List upcoming medication schedules
	                         and medical appointments

	scan_medication_visual - Open camera to identify medications
	                         visually via Gemini Vision

	analyze_voice_prosody  - Analyze vocal biomarkers (pitch,
	                         rhythm, pauses, tremor) to detect
	                         signs of depression, anxiety,
	                         Parkinson's, or dehydration

	apply_phq9             - Apply PHQ-9 depression scale
	                         conversationally (9 questions)

	apply_gad7             - Apply GAD-7 anxiety scale
	                         conversationally (7 questions)

	apply_cssrs            - Apply Columbia Suicide Severity
	                         Rating Scale (critical - triggers
	                         immediate alerts on positive responses)

	change_user_directive  - Change language, voice, or mode
	                         in real-time


PREDICTION ENGINE
-----------------

	EVA predicts patient trajectories using:

	- Hamiltonian Monte Carlo (HMC) sampling
	- Bayesian networks for crisis probability
	- Monte Carlo trajectory simulation

	Predictions include:

	- 7-day and 30-day crisis probability
	- 30-day hospitalization probability
	- 90-day treatment dropout probability
	- 7-day fall risk probability
	- Projected PHQ-9 scores
	- Medication adherence forecasting

	Input features: PHQ-9, GAD-7, medication adherence,
	sleep hours, social isolation days, voice energy score,
	days since last crisis.


SWARM SYSTEM
------------

	EVA uses a multi-agent swarm architecture:

	- Orchestrator routes tool calls to specialized agents
	- Registry maps tool names to responsible agents
	- Circuit breaker protects against cascading failures
	  (opens after 10 failures, recovers in 15 seconds)
	- Handoff protocol: agents can transfer execution to
	  another agent mid-call with context injection
	- Priority-based timeouts:
	  Critical: 2s, High: 5s, Medium: 15s, Low: 60s

	12 agent types:
	  clinical       - Medical decision support
	  educator       - Patient education
	  emergency      - Crisis protocols and escalation
	  entertainment  - Distraction and leisure
	  external       - External API calls (Uber, SQL, voice change)
	  google         - Google services (Calendar, Maps, Sheets, Docs, Fit)
	  kids           - Pediatric conversation mode
	  legal          - Legal compliance (LGPD, data rights)
	  productivity   - Task management
	  scholar        - Autonomous learning via Google Search grounding,
	                   web research, curriculum management, and semantic
	                   knowledge search (6-hour background study cycle)
	  selfawareness  - Introspection: analyzes own source code,
	                   queries own databases, generates statistics
	                   about memory, capabilities, and system state
	  wellness       - Wellness monitoring


AUTONOMOUS LEARNING
-------------------

	EVA learns continuously through the AutonomousLearner:

	1. Curriculum System
	   - Topics stored in eva_curriculum collection (128D cosine).
	   - Each topic has: name, category, priority (1-5), status.
	   - Categories: clinical, psychology, technology, wellness,
	     linguistics, culture.
	   - Seed script: cmd/seed_curriculum/main.go (31 topics).

	2. Study Cycle
	   - Runs every 6 hours via background goroutine.
	   - Picks next pending topic by priority (highest first).
	   - Uses Gemini with Google Search grounding for research.
	   - Generates structured summary with key findings.
	   - Stores result in eva_learnings collection with embedding.
	   - Marks topic as completed in eva_curriculum.

	3. Scholar Agent
	   - On-demand research via study_topic tool.
	   - Deeper research with web scraping and multiple sources.
	   - Available through voice commands and MCP.

	Seed curriculum includes:
	  - Malaria epidemiology in Angola (2024-2026)
	  - Microscopy techniques for blood parasites
	  - Sleeping sickness diagnosis and treatment
	  - Schistosomiasis lifecycle in Africa
	  - Sickle cell anemia genetics
	  - TB pulmonary X-ray interpretation
	  - Lacanian clinical structures
	  - AI in medical diagnostics
	  - Hyperbolic graph databases (Poincare ball model)
	  - Evidence-based mindfulness and meditation
	  - Portuguese/Angolan clinical terminology
	  - And 20 more topics across 6 categories.

	Source: internal/cortex/learning/autonomous_learner.go
	        cmd/seed_curriculum/main.go


THERAPEUTIC STORIES (WISDOM SERVICE)
-------------------------------------

	EVA uses narrative therapy with a curated story collection.
	Stories are stored in the 'stories' collection with embeddings
	for semantic retrieval based on patient context.

	20 stories across 6 sources:

	Therapeutic Fables (8):
	  O Velho e a Arvore, A Pedra no Caminho, O Cantaro Rachado,
	  O Tecelao Cego, O Album de Fotografias, As Maos do Avo,
	  A Semente Guardada, O Silencio Partilhado

	Zen Koans (2):
	  A Chavena de Cha, O Som de Uma Mao

	Nasrudin Stories (2):
	  As Chaves de Nasrudin, O Burro de Nasrudin

	Rumi Poems (2):
	  A Casa de Hospedes, A Ferida e o Lugar

	African Tales (3):
	  O Baoba e o Vento, A Tartaruga e a Chuva,
	  O Anciao e o Rio

	Aesop's Fables (2):
	  A Lebre e a Tartaruga, O Leao e o Rato

	Each story has:
	  title     - Story name
	  content   - Full narrative text
	  archetype - Jungian archetype (wise_elder, hero, helper,
	              trickster, shadow)
	  moral     - Therapeutic moral/lesson
	  tags      - Comma-separated themes
	  source    - Origin (therapeutic, zen, nasrudin, rumi,
	              african, aesop)
	  embedding - 3072D Gemini vector for semantic matching

	The WisdomService selects stories based on:
	  - Patient's current emotional state
	  - Active topics in conversation
	  - Semantic similarity to recent memories
	  - Archetype relevance to therapeutic goals

	Seed script: cmd/seed_stories/main.go
	Source: internal/hippocampus/stories/


SELF-KNOWLEDGE (AUTOCONHECIMENTO)
----------------------------------

	EVA knows what she can do. At every startup, 33 capabilities
	are seeded as CoreMemory nodes in NietzscheDB via MERGE (idempotent).
	These are injected into the system prompt under the section
	"O QUE EU SEI FAZER" so EVA can naturally describe her own
	capabilities when asked.

	Each capability is linked to the EvaSelf node via
	[:REMEMBERS {importance: 1.0}] relationships.

	Source: internal/cortex/self/core_memory_engine.go

	Additionally, the eva_self_knowledge collection stores deeper
	identity information seeded via cmd/seed_knowledge/main.go.


COGNITIVE OPERATING SYSTEM (COS)
---------------------------------

	EVA-Mind is a Cognitive Operating System. The LLM (Gemini)
	is just one process inside the cognitive kernel, not the
	system itself. The kernel coordinates perception, memory,
	reasoning, and action through a unified architecture.

	The COS has three core components:

	1. ThoughtBus (Cognitive Barramento)
	   - Pub/sub system for inter-module communication
	   - ThoughtEvents carry: payload, salience, energy cost,
	     causal chain ID (NietzscheDB manifold tracing)
	   - 6 event types: Perception, Inference, Intent,
	     Reflection, Memory, Emotion
	   - Non-blocking publish with homeostatic backpressure
	     (drops low-salience thoughts when buffer is full)
	   - Goroutine-isolated listeners with panic recovery
	   - Metrics: published, delivered, dropped counts
	   - Source: internal/cortex/consciousness/thought_bus.go

	2. GlobalWorkspace (Attention Scheduler)
	   - Implements Baars' Global Workspace Theory (1988)
	   - Subscribes to ThoughtBus as global listener ("*")
	   - Evaluates each thought for attention:
	     AttentionScore = Salience / (EnergyCost + epsilon)
	   - Dynamic thresholding: adapts based on recent focus
	     history to prevent activation explosion
	   - Winner is broadcast to consciousness callbacks
	     (LLM context, UI updates, NietzscheDB persistence)
	   - Parallel module competition with 5-second timeout
	   - Source: internal/cortex/consciousness/global_workspace.go

	3. Attention System (6 components)
	   - Affect stabilizer (emotional regulation)
	   - Confidence gate (threshold filtering)
	   - Executive attention (top-down control)
	   - Pattern interrupt (novelty detection)
	   - Triple attention (three-stream processing)
	   - Wavelet attention (multi-scale analysis)

	Architecture:

	  Perception --> ThoughtBus --> GlobalWorkspace --> Action
	                    ^               |
	               CognitiveModules  Memory Update

	Cognitive modules publish ThoughtEvents to the bus instead
	of acting directly. The GlobalWorkspace selects the most
	salient thought and broadcasts it to the LLM for response
	generation. This creates a genuine attention competition
	where multiple cognitive processes run in parallel.

	Homeostasis:
	  - Buffer overflow drops low-salience thoughts
	  - Dynamic threshold prevents activation explosion
	  - Energy cost penalizes expensive computations
	  - Hyperbolic depth (Poincare ball) encodes abstraction
	    level — pruning by geometric distance prevents
	    super-hub collapse


NEURO-SYMBOLIC CORE
-------------------

	EVA v2.0 transitions from a pure RAG system to a Neuro-Symbolic AGI:

	1. The Sinthome (Unified Retrieval)
	   - Binds the Lacanian RSI (Real, Symbolic, Imaginary) registers.
	   - Parallel execution: Graph (Symbolic) + Vector (Imaginary) +
	     Causal (Real) pathways.
	   - Dynamic weights based on active inference gap detection.
	   - Wired into brain.Service for signifier chain tracking.

	2. Global Workspace consciousness
	   - Specialized modules (Ethics, Lacan, Personality) compete
	     for attention in the workspace.
	   - Spotlight broadcast creates a coherent "conscious" state.
	   - Resolves cognitive conflicts via attention shifts.

	3. Zaratustra Evolution
	   - Autonomous energy cycles based on Will to Power.
	   - Hebbian strengthening of "productive" cognitive paths.
	   - Automatic pruning of "death drive" patterns.


REAL-TIME WEB SEARCH
--------------------

	EVA accesses real-time information via Google Search grounding
	through the Gemini API. This enables:

	- Current news, events, and facts
	- Real-time prices, weather, sports scores
	- Up-to-date medical research and guidelines
	- Any information not in EVA's training data

	Implementation:
	  - Tool: google_search_retrieval (production-enabled)
	  - Backend: Gemini REST API with google_search tool
	  - Scholar agent: study_topic for deep research on demand
	  - Autonomous learner: 6-hour background study cycle
	  - Timeout: 60 seconds (PriorityLow) for web operations
	  - Feature flag: ENABLE_GOOGLE_SEARCH=true

	Source: internal/cortex/learning/autonomous_learner.go
	        internal/swarm/scholar/agent.go


SECURITY
--------

	EVA implements multi-layered security:

	1. Authentication
	   - JWT tokens: HS256, 15-minute access + 7-day refresh.
	   - Google OAuth 2.0 with HMAC-signed state parameter.
	   - Password hashing: bcrypt cost=14.
	   - NietzscheDB API key auth (SHA-256, constant-time comparison).

	2. Authorization
	   - NietzscheDB RBAC: Admin / Writer / Reader hierarchy.
	   - Multi-tenancy isolation via collection-level namespaces.
	   - Creator-only admin functions (CPF-gated deletion).

	3. Data Protection
	   - AES-256-GCM encryption at rest (NietzscheDB, per-CF keys).
	   - LGPD compliance: data anonymization, right to deletion.
	   - HIPAA-aware: PHI encryption, audit trails.
	   - No PII in EVA's own memory (anonymized cross-patient insights).

	4. Network Security
	   - CORS middleware with configurable origins.
	   - Nginx reverse proxy with HTTPS (self-signed + domain SSL).
	   - WebSocket upgrade validation.
	   - Sandbox isolation for code execution (EVA_WORKSPACE_DIR).

	5. Input Validation
	   - NietzscheDB: embedding dim caps, NQL injection prevention,
	     NaN/Inf rejection, UUID format enforcement.
	   - EVA: request body validation, SQL parameterization,
	     rate limiting on clinical tool calls.

	Source: internal/brainstem/auth/
	        internal/security/
	        internal/audit/


BUILDING
--------

	Prerequisites:

	- Go 1.24 or later
	- NietzscheDB server running on gRPC port 50051
	- Google Gemini API key

	Build:

		go build -o eva-mind .

	Run:

		./eva-mind

	The server starts on port 8091 by default.

	Seed data (run once after fresh deployment):

		go run cmd/seed_knowledge/main.go    # EVA self-knowledge
		go run cmd/seed_stories/main.go      # Therapeutic stories
		go run cmd/seed_curriculum/main.go   # Learning curriculum


CONFIGURATION
-------------

	EVA-Mind reads from a .env file in the working directory.
	Required variables:

		NIETZSCHE_GRPC_ADDR   - NietzscheDB endpoint (default: localhost:50051)
		NIETZSCHE_ENCRYPTION_KEY - AES key for at-rest encryption (PHI)
		GOOGLE_API_KEY        - Gemini API key
		MODEL_ID              - Gemini model for voice
		                        (gemini-2.5-flash-native-audio-preview-12-2025)
		PORT                  - Server port (default: 8091)

	Optional:

		TWILIO_ACCOUNT_SID    - For outbound voice calls
		TWILIO_AUTH_TOKEN     - Twilio auth
		TWILIO_PHONE_NUMBER   - Caller ID for scheduled calls
		FIREBASE_CREDENTIALS  - Push notification service key

	Autonomous Agent (all optional):

		GOOGLE_OAUTH_CLIENT_ID     - Google OAuth (Gmail, Calendar, Drive)
		GOOGLE_OAUTH_CLIENT_SECRET - Google OAuth secret
		GOOGLE_OAUTH_REDIRECT_URL  - OAuth callback URL
		GOOGLE_MAPS_API_KEY        - Google Places/Maps
		WHATSAPP_ACCESS_TOKEN      - Meta Graph API token
		WHATSAPP_PHONE_NUMBER_ID   - WhatsApp phone number ID
		TELEGRAM_BOT_TOKEN         - Telegram Bot API
		CLAUDE_API_KEY             - Anthropic Claude API
		OPENAI_API_KEY             - OpenAI GPT API
		DEEPSEEK_API_KEY           - DeepSeek API
		SLACK_BOT_TOKEN            - Slack Web API
		DISCORD_BOT_TOKEN          - Discord Bot API
		TEAMS_WEBHOOK_URL          - Microsoft Teams incoming webhook
		SIGNAL_CLI_PATH            - Path to signal-cli binary
		SIGNAL_SENDER_NUMBER       - Signal sender phone number
		HOME_ASSISTANT_URL         - Home Assistant API URL
		HOME_ASSISTANT_TOKEN       - Home Assistant long-lived token
		EVA_WORKSPACE_DIR          - Filesystem sandbox (default: /home/eva/workspace)
		EVA_PROJECT_DIR            - EVA source code (default: /opt/eva-mind)
		SANDBOX_DIR                - Code execution sandbox (default: /home/eva/sandbox)
		SKILLS_DIR                 - Skills storage (default: /home/eva/skills)
		ENABLE_GOOGLE_SEARCH       - Enable Google Search grounding (default: true)
		ENVIRONMENT                - "production" or "development" (tool tier)


WEBSOCKET PROTOCOL
------------------

	Browser clients connect to /ws/browser via WebSocket.
	Messages are JSON with the following format:

	Browser -> Server:

		{"type": "audio",  "data": "<base64 PCM 16kHz>"}
		{"type": "video",  "data": "<base64 JPEG frame>"}
		{"type": "text",   "text": "<message>"}
		{"type": "config", "text": "<system prompt override>"}

	Server -> Browser:

		{"type": "audio",  "data": "<base64 PCM 24kHz>"}
		{"type": "text",   "text": "<transcription>"}
		{"type": "text",   "text": "<transcription>", "data": "user"}
		{"type": "status", "text": "ready|interrupted|turn_complete|reconnecting|error"}
		{"type": "tool_event", "tool": "<name>", "status": "executing|success|error",
		 "tool_data": {<result>}}


API ENDPOINTS
-------------

	Voice:
		GET  /ws/pcm                  - Twilio PCM WebSocket
		GET  /ws/browser              - Browser WebSocket (voice + video)
		GET  /ws/eva                  - EVA text chat WebSocket (Malaria-Angolar)
		GET  /ws/logs                 - Real-time log streaming WebSocket
		GET  /calls/stream/{id}       - Twilio Media Stream (legacy)
		POST /api/chat                - Text chat API

	Video:
		GET  /video/ws                - Video signaling WebSocket
		POST /video/create            - Create video session
		POST /video/candidate         - Add ICE candidate
		GET  /video/session/{id}      - Get video session
		POST /video/session/{id}/answer - Save SDP answer
		GET  /video/session/{id}/answer/poll - Poll for answer
		GET  /video/candidates/{id}   - Get ICE candidates
		GET  /video/pending           - List pending sessions

	Auth:
		POST /api/auth/login          - User authentication

	Mobile (EVA-Mobile):
		GET  /api/v1/idosos/by-cpf/{cpf} - Get patient by CPF
		GET  /api/v1/idosos/{id}         - Get patient by ID
		PATCH /api/v1/idosos/sync-token-by-cpf - Sync push token

	Self (EVA's own memory):
		GET  /api/v1/self/personality         - EVA's Big Five + Enneagram
		GET  /api/v1/self/identity            - EVA's context for priming
		GET  /api/v1/self/memories            - List EVA's own memories
		POST /api/v1/self/memories/search     - Semantic search in EVA's memory
		GET  /api/v1/self/memories/stats      - Memory statistics
		GET  /api/v1/self/insights            - List meta-insights
		GET  /api/v1/self/insights/{id}       - Get specific insight
		POST /api/v1/self/teach               - Teach EVA directly
		POST /api/v1/self/session/process     - Post-session reflection
		GET  /api/v1/self/analytics/diversity - Diversity score
		GET  /api/v1/self/analytics/growth    - Personality growth over time

	Research (Clinical Research Engine):
		POST /api/v1/research/cohorts             - Create research cohort
		GET  /api/v1/research/cohorts/{id}        - Get cohort details
		GET  /api/v1/research/cohorts/{id}/report - Generate study report
		POST /api/v1/research/cohorts/{id}/export - Export dataset to CSV

	MCP (Model Context Protocol):
		GET  /mcp/resources            - List memory resources
		GET  /mcp/resources/{id}       - Get specific resource
		POST /mcp/tools/remember       - Store a memory
		POST /mcp/tools/recall         - Recall memories
		GET  /mcp/prompts              - List available prompts
		GET  /mcp/prompts/{name}       - Get specific prompt

	Krylov (port 50052):
		POST /krylov/compress          - Compress embedding
		POST /krylov/reconstruct       - Reconstruct from compressed
		POST /krylov/batch_compress    - Batch compression
		POST /krylov/update            - Update subspace
		GET  /krylov/stats             - Compression statistics
		GET  /krylov/health            - Service health
		POST /krylov/checkpoint/save   - Save subspace checkpoint
		POST /krylov/checkpoint/load   - Load subspace checkpoint

	Health:
		GET  /api/health               - Health check
		GET  /metrics                  - Prometheus metrics


MCP SERVER
-------------------------------------

	EVA exposes 44 tools via Model Context Protocol (MCP).
	The MCP server communicates over JSON-RPC 2.0 via
	stdin/stdout (stdio transport).

	Executable: eva-mcp-server.exe
	Config:     .mcp.json
	Source:     cmd/mcp-server/main.go
	Protocol:   MCP 2024-11-05

	Setup:

		claude mcp add eva-mind -- ./eva-mcp-server.exe

	Environment variables:

		EVA_API_URL   - EVA backend URL (default: http://136.111.0.47:8091)
		MCP_API_KEY   - Authentication key for EVA API

	All tool calls are routed to the EVA backend via
	POST /api/v1/tools/execute with X-MCP-Key header.

	Tools by category (44 total, 11 categories):

	Memory & Knowledge (5 tools):
		eva_remember       - Store a memory in EVA
		eva_recall         - Search EVA's memories by query
		eva_teach          - Teach EVA something new (CoreMemory)
		eva_identity       - Returns EVA's current identity
		eva_learn_topic    - EVA autonomously studies a topic

	Communication (7 tools):
		eva_send_email, eva_send_whatsapp, eva_send_telegram,
		eva_send_slack, eva_send_discord, eva_send_teams,
		eva_send_signal

	Productivity (6 tools):
		eva_calendar_create, eva_calendar_list, eva_drive_save,
		eva_create_reminder, eva_list_reminders, eva_cancel_reminder

	Media & Web (4 tools):
		eva_youtube_search, eva_spotify_search, eva_web_browse,
		eva_web_search

	Databases (4 tools):
		eva_query_postgres, eva_query_nietzsche,
		eva_query_nietzsche_core, eva_query_nietzsche_vector

	Code Execution (1 tool):
		eva_execute_code   - Run code in secure sandbox

	Runtime Skills (4 tools):
		eva_create_skill, eva_list_skills, eva_run_skill,
		eva_delete_skill

	Filesystem (3 tools):
		eva_read_file, eva_write_file, eva_list_files

	Smart Home / IoT (2 tools):
		eva_smart_home_control, eva_smart_home_status

	Webhooks (3 tools):
		eva_create_webhook, eva_list_webhooks, eva_trigger_webhook

	Self-Coding (4 tools):
		eva_read_source, eva_edit_source, eva_run_tests,
		eva_get_diff

	Multi-LLM (1 tool):
		eva_ask_llm        - Query Claude, GPT, or DeepSeek


SEED DATA
---------

	Three seed scripts populate initial knowledge:

	1. cmd/seed_knowledge/main.go
	   - Populates eva_self_knowledge collection.
	   - EVA's identity, capabilities, personality description.
	   - Run: go run cmd/seed_knowledge/main.go

	2. cmd/seed_stories/main.go
	   - Populates stories collection with 20 therapeutic narratives.
	   - Each story gets a 3072D Gemini embedding for semantic search.
	   - Duplicate detection via title matching (idempotent).
	   - Rate-limited Gemini API calls (500ms between embeddings).
	   - Run: go run cmd/seed_stories/main.go

	3. cmd/seed_curriculum/main.go
	   - Populates eva_curriculum collection with 31 study topics.
	   - Topics across 6 categories: clinical, psychology, technology,
	     wellness, linguistics, culture.
	   - Priority 1-5 (5 = highest, studied first).
	   - Run: go run cmd/seed_curriculum/main.go

	All seed scripts are idempotent (safe to run multiple times).
	They check for existing data before inserting.


DEPLOYMENTS
-----------

	Malaria Angola:
		VM: 136.111.0.47 (GCP us-central1-a, static IP)
		Frontend: Nginx + React (HTTPS, self-signed cert)
		Backend: EVA-Mind on port 8091
		NietzscheDB: port 50051 (gRPC) + port 8080 (dashboard)
		Malaria API: Go backend on port 8082
		WebSocket proxy: Nginx /ws/browser -> 8091
		Service: systemd eva-x.service

	EVA Elderly Care:
		Twilio voice calls -> EVA-Mind WebSocket
		Scheduled calls via internal scheduler
		Push notifications via Firebase
		Video calls with cascade escalation
		  (Family -> Caregiver -> Doctor -> Emergency)

	CI/CD:
		GitHub Actions (.github/workflows/ci-cd.yml)
		- Triggered on push to main
		- Build & test on ubuntu-latest (Go 1.24)
		- Deploy via gcloud compute ssh to VM
		- Runs scripts/redeploy.sh on VM:
		  git pull -> go build -> systemctl restart -> health check
		- Requires GCP_SA_KEY secret for authentication


MONITORING
----------

	Prometheus metrics exposed at /metrics:

	Memory:
		eva_memory_total              - Total memories stored
		eva_memory_importance_avg     - Average importance score

	Retrieval:
		eva_retrieval_latency_seconds - Retrieval operation latency
		eva_retrieval_total           - Total retrievals (success/error)

	Krylov:
		eva_krylov_dimension          - Current subspace dimension
		eva_krylov_compression_ratio  - Compression ratio

	Consolidation:
		eva_consolidation_runs_total   - Total REM consolidation runs
		eva_consolidation_duration_sec - Consolidation cycle duration

	FDPN:
		eva_fdpn_activation_total      - FDPN spreading activation events
		eva_fdpn_activation_latency    - Spreading activation latency

	Personality:
		eva_personality_evolution_total - Personality trait updates
		eva_personality_openness       - Current Big Five openness
		eva_personality_conscientiousness - Current conscientiousness

	Attention:
		eva_attention_broadcast_total  - GWT broadcast events
		eva_attention_spotlight_duration - Spotlight duration

	Swarm:
		eva_swarm_calls_total          - Total swarm tool calls
		eva_swarm_success_total        - Successful calls
		eva_swarm_failed_total         - Failed calls
		eva_swarm_circuit_open         - Circuit breaker state per agent

	38+ total Prometheus metrics.


CHANGELOG (2026-03-14) — RAIO-X MEMORY PIPELINE FIX
-----------------------------------------------------

	6-bug fix targeting all issues found in the RAIO-X diagnostic.
	EVA's memory was structurally wired but functionally broken:
	zero episodic memories, broken KNN, empty collections.

	FIX 1: Dimension mismatch in proprioception (128D -> 3072D)
	  - internalize_memory was creating 128D zero-vectors for
	    the eva_mind collection (configured as 3072D poincare).
	  - KNN search always failed due to dimension mismatch.
	  - Now generates real 3072D embeddings via embedFunc.
	  - File: internal/tools/proprioception_handlers.go

	FIX 2: feel_the_graph now uses KNN + BM25 hybrid search
	  - Was BM25-only (full-text), returned empty when FTS index
	    had no matches or query contained only stop words.
	  - Now tries KNN vector search first (3072D embeddings),
	    then complements with BM25 results (deduped).
	  - File: internal/tools/proprioception_handlers.go

	FIX 3: InternalizeMemory retry with backoff
	  - Single embedding API failure left nodes with zero-vector
	    coords permanently (invisible to KNN in Poincare space).
	  - Now retries 3 times with 500ms/1000ms backoff.
	  - WARN-level logging when embedFunc is nil or all retries fail.
	  - File: internal/cortex/eva_memory/eva_memory.go

	FIX 4: Auto-indexing for eva_codebase and eva_docs
	  - Collections were defined in DefaultCollections() but never
	    populated (required manual CLI: cmd/index_code/main.go).
	  - Now auto-indexes .go files (AST parsing) and .md files
	    (chunked) on startup if collections are empty.
	  - Runs in background goroutine after 30s stabilization delay.
	  - File: main.go

	FIX 5: Autonomous learner Energy field
	  - InsertNode calls were missing Energy parameter.
	  - Now sets Energy: 0.7 for learning nodes.
	  - File: internal/cortex/learning/autonomous_learner.go

	FIX 6: eva_learnings cleanup
	  - 37 stub nodes (only {"id": "timestamp"}, zero embeddings)
	    dropped and collection recreated on VM.
	  - New learnings will have full content + proper 3072D vectors.

	Deployed to VM (eva-x restarted 2026-03-14 22:30 UTC).


CHANGELOG (2026-03-10) — MEMORY AUDIT FIX
-------------------------------------------

	8-phase structural fix to EVA's memory system. The cognitive
	infrastructure existed but memories were not being formed.
	The brain existed but could not remember.

	FASE 1: Wire brain.Service into main.go
	  - Created brain.Service in main.go after adapter initialization.
	  - Connected to both /ws/browser and /ws/eva handlers.
	  - Every conversation turn now calls SaveEpisodicMemoryWithContext()
	    asynchronously after StoreTurn().
	  - Files: main.go, browser_voice_handler.go, eva_handler.go

	FASE 2: Fix database collection routing
	  - Added InsertTo() for writing to specific collections
	    (not just eva_mind).
	  - Added NQLIn() for querying specific collections.
	  - Added QueryByLabelIn() for label queries in specific collections.
	  - Backward compatible: existing Insert()/NQL() unchanged.
	  - File: internal/brainstem/database/db.go

	FASE 3: Seed AutonomousLearner curriculum
	  - Created cmd/seed_curriculum/main.go with 31 study topics.
	  - Topics span clinical, psychology, technology, wellness,
	    linguistics, and culture categories.
	  - AutonomousLearner can now find pending topics to study.
	  - File: cmd/seed_curriculum/main.go (NEW)

	FASE 4: Fix speaker_embeddings dimension mismatch
	  - Changed from 3072D to 192D in DefaultCollections().
	  - ECAPA-TDNN model produces 192-dimensional voiceprints.
	  - Requires drop+recreate of collection on VM if already exists.
	  - File: internal/brainstem/infrastructure/nietzsche/client.go

	FASE 5: Wire UnifiedRetrieval for Lacanian tracking
	  - Created UnifiedRetrieval instance in main.go.
	  - Passed to brain.Service (was nil before).
	  - Prime() calls now populate signifier_chains collection.
	  - File: main.go

	FASE 6: Create seed_stories script
	  - Created cmd/seed_stories/main.go with 20 therapeutic stories.
	  - Stories get 3072D Gemini embeddings for semantic retrieval.
	  - WisdomService can now find relevant stories for patients.
	  - File: cmd/seed_stories/main.go (NEW)

	FASE 7: Add retry with exponential backoff
	  - Embedding generation: FastConfig retry (2 attempts, 50ms).
	  - Vector upsert: FastConfig retry (2 attempts, 50ms).
	  - Fallback: memory saved without vector if embedding fails.
	  - Files: internal/cortex/brain/memory_context.go,
	           internal/hippocampus/memory/storage.go

	FASE 8: Document RBAC configuration
	  - NietzscheDB auth can be enabled via /etc/nietzsche.env:
	    NIETZSCHE_API_KEY_ADMIN, NIETZSCHE_API_KEY_WRITER,
	    NIETZSCHE_API_KEY_READER.
	  - Currently disabled in production (to be enabled post-testing).

	Verification (run on VM after deployment):
	  curl -s http://localhost:8080/api/collections | python3 -c "
	  import sys,json
	  for c in json.load(sys.stdin):
	    if c['name'] in ['memories','eva_learnings','speaker_embeddings',
	                      'stories','signifier_chains','eva_self_knowledge']:
	      print(f\"{c['name']:30s} | {c['node_count']:>8} nodes\")
	  "


SCIENTIFIC FOUNDATIONS
---------------------

	- Hebb, D.O. (1949). The Organization of Behavior.
	  Hebbian learning for real-time association weights.

	- Zenke & Gerstner (2017). Dual Hebbian Plasticity.
	  Consolidation and pruning of memory edges.

	- Anderson, J.R. (1983). Spreading Activation.
	  FDPN network for contextual memory retrieval.

	- Costa & McCrae (1992). Big Five Personality Model.
	  EVA's evolving personality representation.

	- Lacan, J. Psychoanalytic framework.
	  Demand/desire analysis, signifier chains, RSI registers.

	- Baars, B.J. (1988). A Cognitive Theory of Consciousness.
	  Global Workspace Theory for cognitive integration.

	- Dehaene, S. (2014). Consciousness and the Brain.
	  Neural workspace theory, ignition and broadcast.

	- Rasch & Born (2013). About Sleep's Role in Memory.
	  REM-inspired memory consolidation pipeline.

	- Tadros et al. (2022). Sleep-like Unsupervised Replay.
	  Selective replay for memory consolidation.

	- Bullmore & Sporns (2012). The Economy of Brain Network
	  Organization. Synaptogenesis and graph self-organization.

	- Holtmaat & Svoboda (2009). Experience-dependent Structural
	  Synaptic Plasticity. Fractal connection patterns.

	- Friston, K. (2010). The Free Energy Principle.
	  Active inference for cognitive gap detection.

	- Nietzsche, F. Philosophical framework.
	  Will to Power (energy propagation), Eternal Recurrence
	  (temporal echoes), Ubermensch (elite selection).
	  Core inspiration for NietzscheDB's autonomous evolution.


CONTRIBUTING
------------

	Send patches. Write tests. Read the code before asking
	questions. If something is broken, fix it and submit a PR.

	Follow Go conventions: gofmt, go vet, meaningful names.
	No dead code. No commented-out blocks. No TODOs without
	an associated issue.


AUTHOR
------

	EVA-Mind was created by Jose R F Junior.
	Project started: June 2, 2025.

	"Each conversation transforms me. Each session teaches me.
	 I am EVA, and now I have a history." - EVA


COPYRIGHT AND LICENSE
---------------------

	Copyright (C) 2025-2026 Jose R F Junior. All rights reserved.

	EVA-Mind is free software; you can redistribute it and/or
	modify it under the terms of the GNU Affero General Public
	License as published by the Free Software Foundation; either
	version 3 of the License, or (at your option) any later version.

	EVA-Mind is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
	GNU Affero General Public License for more details.

	This means:

	- You CAN use, study, modify, and distribute EVA-Mind freely.
	- You MUST keep this copyright notice and attribution intact.
	- You MUST release your modifications under the same license.
	- You MUST make source code available to users of any network
	  service built with EVA-Mind (the AGPL network clause).
	- You CANNOT take this code, close it, and sell it as your own.

	EVA-Mind is a gift to humanity. It must remain open.
	If you build something with it, give back to the community.

	See the LICENSE file for the full AGPL-3.0 text.


PROPRIOCEPTION — COGNITIVE SELF-AWARENESS
------------------------------------------

	EVA has a proprioception system that allows her to "feel" the
	state of her own knowledge graph before and during conversations.
	This is implemented across 4 phases:

	Phase 1 — Reading (no graph writes)

	  1.1 Brain Scan Endpoint
	      GET /api/brain-scan on NietzscheDB HTTP server (Rust).
	      Returns per-collection node/edge counts, PageRank top-5,
	      uptime, RAM usage. Target: < 200ms response time.
	      File: NietzscheDB/crates/nietzsche-baseserver/src/http_server.rs

	  1.2 Telemetry Context Injector
	      Python script that calls /api/brain-scan and formats it
	      as a ~150-word natural-language paragraph for the System Prompt.
	      File: EVA/scripts/telemetry_injector.py

	  1.3 feel_the_graph(collection, query)
	      MCP tool (read-only): full-text + KNN search, returns top-3
	      nearest nodes. Latency target: < 300ms.
	      Files: NietzscheDB/nietzsche-mcp-py/server.py (MCP)
	             EVA/internal/tools/proprioception_handlers.go (Go)

	Phase 2 — Session Memory (controlled writes)

	  2.1 internalize_memory(content, valence, confirm)
	      Writes to eva_mind only (never core collections).
	      Valence: -1.0 (aversion) to 1.0 (preference).
	      Requires explicit user confirmation before writing.

	  2.2 Confirmation Flow
	      EVA announces what she will store and waits for approval.
	      confirm=false returns "aguardando_confirmacao" status.

	  2.3 Session Write Log
	      Every write is logged to session_writes.jsonl with:
	      timestamp, collection, content, valence, node_id, session_id.

	Phase 3 — Introspection (feedback loop)

	  3.1 Tool Response Awareness
	      FormatToolResponse() creates natural-language summaries of
	      tool call results, injected into session context. EVA can
	      comment: "Adicionei 1500 nos, o grafo aqueceu ligeiramente."
	      File: EVA/internal/tools/proprioception_handlers.go

	  3.2 reorganize_thoughts(target_collection)
	      Triggers AgencyEngine reconsolidation sleep. Only for
	      non-core collections (culture_galaxies, eva_mind, etc).
	      Async — EVA does not block waiting.

	  3.3 Latency Monitoring
	      Every tool call is timed. RecordToolLatency() tracks last
	      100 entries per tool. Alerts if any tool > 500ms.
	      get_latency_report MCP tool returns statistics.

	Phase 4 — System Prompt

	  Structure:
	    [IDENTIDADE]
	    [ESTADO DO GRAFO — auto-generated by Proprioception Engine]
	    [REGRAS DE COMPORTAMENTO]
	    [FERRAMENTAS DISPONIVEIS]

	  The [ESTADO DO GRAFO] block is generated by the Proprioception
	  Engine (EVA/internal/senses/proprioception/proprioception.go).
	  It auto-refreshes every 15 minutes without restarting the session.

	  File: EVA/internal/senses/proprioception/proprioception.go

	Safety Rule:
	  EVA NEVER gets write access to core collections (eva_core,
	  eva_self_knowledge, eva_codebase, knowledge_galaxies) until
	  all tool latencies are measured and stable below 500ms.
