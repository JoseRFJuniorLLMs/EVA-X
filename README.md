EVA-Mind
========

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
	- Screen and camera analysis for medical assistance
	- Patient memory graphs (Neo4j)
	- Semantic memory search (Qdrant vector database)
	- Psychoanalytic context modeling (Lacanian framework)
	- Personality system (Big Five + Enneagram)
	- Medication scheduling and alerting
	- Emergency detection and escalation
	- Multi-language support (30+ languages)

	The system currently serves two deployments:

	1. Elderly care (original) - Voice calls via Twilio for
	   medication reminders and psychological support.

	2. Malaria detection (Angola) - Real-time voice and screen
	   assistance for healthcare workers diagnosing malaria
	   from microscope images.


ARCHITECTURE
------------

	EVA-Mind follows a neuroscience-inspired architecture:

	brainstem/     - Configuration, bootstrap
	cortex/        - Higher-order processing (Gemini clients, Lacanian
	               analysis, personality routing, self-memory)
	hippocampus/   - Knowledge graphs (Neo4j), memory retrieval
	senses/        - Input processing (WebSocket signaling, audio)
	motor/         - Output and scheduling (calls, alerts, push)
	gemini/        - Gemini WebSocket client (v1alpha, voice + video)
	voice/         - Voice session management, audio processing
	tools/         - Function calling and tool execution
	swarm/         - Multi-agent coordination

	Two Gemini WebSocket clients exist:

	internal/gemini/         - Lean client for browser sessions and
	                          alerts. Uses v1alpha API. Stateless.

	internal/cortex/gemini/  - Full-featured client for production
	                          voice calls. Uses v1beta API. Thread-safe
	                          with callbacks, VAD tuning, memory support.

	Both are actively used. They are not duplicates.


BUILDING
--------

	Prerequisites:

	- Go 1.21 or later
	- Neo4j 5.x (two instances: patients on 7687, EVA self on 7688)
	- Qdrant vector database
	- PostgreSQL 15+
	- Google Gemini API key

	Build:

		go build -o eva-mind .

	Run:

		./eva-mind

	The server starts on port 8091 by default.


CONFIGURATION
-------------

	EVA-Mind reads from a .env file in the working directory.
	Required variables:

		DATABASE_URL          - PostgreSQL connection string
		NEO4J_URI             - Neo4j bolt URI (patients)
		NEO4J_PASSWORD        - Neo4j password
		GOOGLE_API_KEY        - Gemini API key
		MODEL_ID              - Gemini model for voice
		                        (e.g. gemini-2.5-flash-native-audio-preview-12-2025)
		PORT                  - Server port (default: 8091)

	Optional:

		QDRANT_URL            - Qdrant endpoint
		CORE_MEMORY_NEO4J_URI - Separate Neo4j for EVA's own memory
		TWILIO_ACCOUNT_SID    - For outbound voice calls
		TWILIO_AUTH_TOKEN     - Twilio auth
		TWILIO_PHONE_NUMBER   - Caller ID for scheduled calls
		FIREBASE_CREDENTIALS  - Push notification service key


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
		{"type": "status", "text": "ready|interrupted|turn_complete|error"}


EVA'S MEMORY SYSTEM
-------------------

	EVA has two separate memory systems:

	1. Patient memory (Neo4j port 7687)
	   - Per-patient knowledge graphs
	   - Medical context, conditions, medications
	   - Conversation history (episodic memory)
	   - Semantic embeddings in Qdrant

	2. EVA's own memory (Neo4j port 7688)
	   - EvaSelf node with Big Five personality traits
	   - CoreMemory nodes from post-session reflection
	   - MetaInsight nodes from cross-session pattern detection
	   - All data anonymized, no PII

	After each session, EVA reflects on what she learned,
	anonymizes the data, and stores it as her own memory.
	Her personality evolves based on cumulative experience.


CONTEXT PIPELINE
----------------

	For each voice session, EVA's context is assembled from
	multiple sources in parallel:

	1. Lacanian analysis of the user's speech
	2. Medical context from Neo4j (conditions, medications)
	3. Patient metadata from PostgreSQL (name, language, persona)
	4. Scheduled medications from agendamentos table
	5. Recent episodic memories (last 15 turns, 7-day window)
	6. Semantic signifier chains from Qdrant
	7. Therapeutic stories from wisdom knowledge base

	All of this is merged into a single system instruction
	sent to the Gemini WebSocket API.


DEPLOYMENTS
-----------

	Malaria Angola:
		VM: 34.35.36.178 (GCP africa-south1-a)
		Frontend: Nginx + React (HTTPS, self-signed cert)
		Backend: EVA-Mind on port 8091
		Detection: Go backend on port 8080
		WebSocket proxy: Nginx /ws/browser -> 8091

	EVA Elderly Care:
		Twilio voice calls -> EVA-Mind WebSocket
		Scheduled calls via internal scheduler
		Push notifications via Firebase


API ENDPOINTS
-------------

	Voice:
		GET  /ws/browser              - Browser WebSocket (voice + video)
		POST /api/chat                - Text chat API

	Self:
		GET  /self/personality        - EVA's Big Five + Enneagram
		GET  /self/identity           - EVA's context for priming
		GET  /self/memories           - List EVA's own memories
		POST /self/memories/search    - Semantic search in EVA's memory
		POST /self/teach              - Teach EVA directly
		POST /self/session/process    - Post-session reflection

	Health:
		GET  /health                  - Health check
		GET  /metrics                 - Prometheus metrics


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
	  Demand/desire analysis for clinical context.


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
