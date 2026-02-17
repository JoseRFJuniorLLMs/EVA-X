// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// seed_knowledge populates eva_self_knowledge in PostgreSQL AND Qdrant (semantic search).
// Run: go run cmd/seed_knowledge/main.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"eva-mind/internal/hippocampus/knowledge"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/qdrant/go-client/qdrant"
)

type KnowledgeEntry struct {
	Type     string
	Key      string
	Title    string
	Summary  string
	Content  string
	Location string
	Parent   string
	Tags     string
	Importance int
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:Debian23%40@34.35.142.107:5432/eva-mind?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer db.Close()

	// Ensure table exists with updated_at and UNIQUE constraint on knowledge_key
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS eva_self_knowledge (
			id SERIAL PRIMARY KEY,
			knowledge_type VARCHAR(100) NOT NULL,
			knowledge_key VARCHAR(300) NOT NULL UNIQUE,
			title VARCHAR(500) NOT NULL,
			summary TEXT NOT NULL,
			detailed_content TEXT NOT NULL,
			code_location VARCHAR(500),
			parent_key VARCHAR(300),
			related_keys JSONB DEFAULT '[]',
			tags JSONB DEFAULT '[]',
			importance INTEGER DEFAULT 5,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("Table creation failed: %v", err)
	}

	// Connect Qdrant + EmbeddingService for semantic indexing
	cfg, err := config.Load()
	if err != nil {
		log.Printf("WARN: Config load failed (Qdrant disabled): %v", err)
	}

	var qdrantClient *vector.QdrantClient
	var embedSvc *knowledge.EmbeddingService
	if cfg != nil {
		qdrantClient, err = vector.NewQdrantClient(cfg.QdrantHost, cfg.QdrantPort)
		if err != nil {
			log.Printf("WARN: Qdrant unavailable: %v", err)
		} else {
			embedSvc, err = knowledge.NewEmbeddingService(cfg, qdrantClient)
			if err != nil {
				log.Printf("WARN: Embedding service unavailable: %v", err)
			}
		}
	}

	ctx := context.Background()
	entries := getAllEntries()

	// 1. PostgreSQL (structured catalog)
	stmt, err := db.Prepare(`
		INSERT INTO eva_self_knowledge (knowledge_type, knowledge_key, title, summary, detailed_content, code_location, parent_key, tags, importance, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (knowledge_key) DO UPDATE SET
			title = EXCLUDED.title,
			summary = EXCLUDED.summary,
			detailed_content = EXCLUDED.detailed_content,
			code_location = EXCLUDED.code_location,
			parent_key = EXCLUDED.parent_key,
			tags = EXCLUDED.tags,
			importance = EXCLUDED.importance,
			updated_at = NOW()
	`)
	if err != nil {
		log.Fatalf("Prepare failed: %v", err)
	}
	defer stmt.Close()

	pgCount := 0
	for i, e := range entries {
		_, err := stmt.Exec(e.Type, e.Key, e.Title, e.Summary, e.Content, e.Location, e.Parent, e.Tags, e.Importance)
		if err != nil {
			log.Printf("WARN [%d] PG %s: %v", i, e.Key, err)
		} else {
			pgCount++
		}
	}
	fmt.Printf("PostgreSQL: %d/%d entries seeded\n", pgCount, len(entries))

	// 2. Qdrant (semantic search via embeddings)
	if qdrantClient != nil && embedSvc != nil {
		collName := "eva_self_knowledge"
		qdrantClient.CreateCollection(ctx, collName, 3072)

		qdCount := 0
		batchSize := 3
		var batch []*qdrant.PointStruct

		for i, e := range entries {
			// Embed: title + summary + content
			text := fmt.Sprintf("%s: %s\n%s", e.Title, e.Summary, e.Content)
			if len(text) > 4000 {
				text = text[:4000]
			}

			embedding, err := embedSvc.GenerateEmbedding(ctx, text)
			if err != nil {
				log.Printf("WARN [%d] Embed %s: %v", i, e.Key, err)
				continue
			}

			pointID := uint64(time.Now().UnixNano()/1000000 + int64(i))
			point := vector.CreatePoint(pointID, embedding, map[string]interface{}{
				"key":        e.Key,
				"type":       e.Type,
				"title":      e.Title,
				"summary":    e.Summary,
				"content":    e.Content,
				"location":   e.Location,
				"importance": int64(e.Importance),
			})

			batch = append(batch, point)

			if len(batch) >= batchSize {
				if err := qdrantClient.Upsert(ctx, collName, batch); err != nil {
					log.Printf("WARN Upsert batch: %v", err)
				} else {
					qdCount += len(batch)
				}
				batch = batch[:0]
				time.Sleep(500 * time.Millisecond)
			}
		}

		if len(batch) > 0 {
			if err := qdrantClient.Upsert(ctx, collName, batch); err != nil {
				log.Printf("WARN Upsert final: %v", err)
			} else {
				qdCount += len(batch)
			}
		}

		fmt.Printf("Qdrant: %d/%d entries indexed in '%s'\n", qdCount, len(entries), collName)
	} else {
		fmt.Println("Qdrant: SKIPPED (unavailable)")
	}
}

func getAllEntries() []KnowledgeEntry {
	var entries []KnowledgeEntry

	// === ARCHITECTURE ===
	entries = append(entries, KnowledgeEntry{
		Type: "architecture", Key: "arch:overview", Title: "Arquitetura Geral do EVA-Mind",
		Summary: "EVA-Mind e uma IA companeira para idosos com voz em tempo real, 12 agentes swarm e 110+ tools",
		Content: `EVA-Mind e um sistema de IA companeira projetado para cuidar de idosos. A arquitetura e inspirada no cerebro humano:

BRAINSTEM (Tronco Cerebral) — Infraestrutura base:
- config: Carrega variaveis de ambiente (.env), credenciais, portas
- database: Wrapper PostgreSQL com connection pool (database/sql)
- infrastructure/graph: Client Neo4j (grafo de conhecimento, memorias, Lacan)
- infrastructure/vector: Client Qdrant (embeddings 3072-dim, busca semantica)
- auth: Autenticacao JWT para API REST
- push: Firebase Cloud Messaging para push notifications

CORTEX (Cortex Cerebral) — Logica de negocio e IA:
- gemini: Handlers para Gemini Live (voz bidirecional), Gemini 2.5 Flash (tools), embeddings
- lacan: Analise psicanalitica (FDPN, Narrative Shift, Signifiers, Unified Retrieval)
- personality: Personalidades por Eneagrama (9 tipos), system prompt dinamico
- learning: Aprendizagem autonoma (Scholar Agent background, estuda a cada 6h)
- self: Core Memory Engine (Neo4j), Reflection Service, Anonymization
- selfawareness: Introspecao — busca no codigo, consulta bancos, atualiza memorias
- eva_memory: Meta-cognitive memory (Neo4j graph: sessions, turns, topics, insights)
- alert: Escalation service (push → email → SMS)
- voice/speaker: Voice fingerprinting, speaker recognition (ECAPA-TDNN)

HIPPOCAMPUS (Hipocampo) — Memoria:
- memory: MemoryStore (episodic), GraphStore (Neo4j), RetrievalService (hybrid search)
- memory/superhuman: 12 subsistemas de memoria (Superhuman Memory Model)
- knowledge: EmbeddingService (3072-dim), WisdomService (16 colecoes), SelfKnowledgeService
- habits: HabitTracker (log de habitos diarios)
- spaced: Spaced Repetition (reforco de memoria SM-2)

MOTOR — Acoes no mundo externo:
- email: SMTP service para alertas

SWARM — 12 agentes especializados com circuit breaker:
clinical, emergency, entertainment, wellness, productivity, google, external, educator, kids, legal, scholar, selfawareness

TOOLS — 110+ ferramentas (medicamentos, alarmes, jogos, GTD, etc)

VOICE — WebSocket handlers para voz em tempo real (PCM 16kHz in, 24kHz out)

SCHEDULER — Background jobs (verificacao de agendamentos, alertas)

SECURITY — CORS middleware, rate limiting

TELEMETRY — Zerolog structured logging`,
		Location: "main.go", Tags: `["arquitetura", "visao_geral", "cerebro"]`, Importance: 10,
	})

	// === DATABASES ===
	entries = append(entries, KnowledgeEntry{
		Type: "database", Key: "db:postgresql", Title: "PostgreSQL — Banco Principal",
		Summary: "PostgreSQL 15 em 34.35.142.107:5432. Armazena dados relacionais: idosos, agendamentos, memorias, tools",
		Content: `PostgreSQL e o banco principal do EVA-Mind. Host: 34.35.142.107:5432, DB: eva-mind.

TABELAS PRINCIPAIS:
- idosos: Pacientes cadastrados (id, nome, cpf, data_nascimento, telefone, endereco)
- agendamentos: Compromissos e lembretes (id, idoso_id, tipo, descricao, data_hora, status)
- alertas: Alertas de seguranca (id, idoso_id, tipo, severidade, mensagem, resolvido)
- episodic_memories: Memorias episodicas (id, user_id, content, emotion, embedding, created_at)
- eva_curriculum: Fila de estudo autonomo (id, topic, category, priority, status, insights_count)
- eva_self_knowledge: Conhecimento da EVA sobre si mesma (esta tabela!)
- eva_personalidade_criador: Tracos de personalidade do criador
- eva_memorias_criador: Memorias importantes sobre o criador
- eva_conhecimento_projeto: Conhecimento sobre o projeto EVA-Mind
- spaced_repetition_items: Items para reforco de memoria (SM-2 algorithm)
- habits_log: Log de habitos diarios
- gtd_tasks: Tarefas GTD (Getting Things Done)
- kid_missions: Missoes gamificadas para criancas (EVA Kids)
- speaker_profiles: Perfis de falantes (voice fingerprinting)
- speaker_embeddings: Embeddings 192-dim para identificacao vocal
- speaker_identifications: Historico de identificacoes por sessao

MIGRATIONS: Pasta migrations/ com 41+ arquivos SQL numerados`,
		Location: "internal/brainstem/database/db.go", Parent: "arch:overview",
		Tags: `["postgresql", "banco", "tabelas", "dados"]`, Importance: 10,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "database", Key: "db:neo4j", Title: "Neo4j — Grafo de Conhecimento",
		Summary: "Neo4j em localhost:7687. Armazena grafos: memorias, personalidade EVA, Lacan, sessoes",
		Content: `Neo4j e o banco de grafos do EVA-Mind. Host: localhost:7687 (no servidor GCP).

NODES PRINCIPAIS:
- EvaSelf: Singleton da personalidade da EVA (Big Five: openness, conscientiousness, extraversion, agreeableness, neuroticism + Eneagrama)
- CoreMemory: Memorias proprias da EVA (session_insight, emotional_pattern, crisis_learning, personality_evolution, teaching_received, meta_insight, self_reflection)
- EvaSession: Sessoes de conversa meta-cognitivas
- EvaTurn: Turnos individuais dentro de uma sessao
- EvaTopic: Topicos discutidos
- EvaInsight: Insights gerados pela EVA sobre conversas
- Person: Pessoas no grafo (idosos, familiares)
- Event: Eventos de vida
- Emotion: Estados emocionais
- Topic: Topicos de conversa
- Signifier: Significantes lacanianos (cadeias de significantes)
- FDPNNode: Nos do grafo FDPN (Formacao, Demanda, Posicao, Nome)

RELATIONSHIPS:
- EvaSelf -[:REMEMBERS]-> CoreMemory
- EvaSession -[:HAS_TURN]-> EvaTurn
- EvaTurn -[:ABOUT]-> EvaTopic
- Person -[:EXPERIENCED]-> Event
- Person -[:FELT]-> Emotion
- Event -[:RELATED_TO]-> Topic
- Signifier -[:CHAINS_TO]-> Signifier`,
		Location: "internal/brainstem/infrastructure/graph/neo4j_client.go", Parent: "arch:overview",
		Tags: `["neo4j", "grafo", "memoria", "personalidade"]`, Importance: 10,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "database", Key: "db:qdrant", Title: "Qdrant — Memoria Vetorial",
		Summary: "Qdrant em localhost:6333/6334. 18+ colecoes de embeddings 3072-dim para busca semantica",
		Content: `Qdrant e o banco vetorial do EVA-Mind. Host: localhost:6333 (REST) / 6334 (gRPC).

COLECOES (todas 3072-dim, Cosine distance):
- eva_learnings: Insights aprendidos pelo Scholar Agent (estudo autonomo)
- eva_codebase: Codigo-fonte indexado (self-awareness)
- signifier_chains: Cadeias de significantes lacanianos
- gurdjieff_teachings: Ensinamentos de Gurdjieff (Quarto Caminho)
- osho_insights: Insights de Osho
- ouspensky_fragments: Fragmentos de Ouspensky
- nietzsche_aphorisms: Aforismos de Nietzsche/Zaratustra
- rumi_poems: Poemas de Rumi (Sufismo)
- hafiz_poems: Poemas de Hafiz
- kabir_songs: Cancoes de Kabir
- zen_koans: Koans Zen
- sufi_stories: Historias Sufis
- jung_concepts: Conceitos de Jung
- lacan_concepts: Conceitos de Lacan
- marcus_aurelius: Meditacoes de Marco Aurelio
- seneca_letters: Cartas de Seneca
- epictetus_discourses: Discursos de Epiteto
- buddha_suttas: Suttas do Buddha

EMBEDDING MODEL: gemini-embedding-001 (Google), 3072 dimensoes
BUSCA: Cosine similarity, top-K results com payload filtering`,
		Location: "internal/brainstem/infrastructure/vector/qdrant_client.go", Parent: "arch:overview",
		Tags: `["qdrant", "vetorial", "embeddings", "semantica"]`, Importance: 10,
	})

	// === MODULES ===
	entries = append(entries, KnowledgeEntry{
		Type: "module", Key: "module:brainstem", Title: "Brainstem — Infraestrutura Base",
		Summary: "Tronco cerebral: config, database, graph (Neo4j), vector (Qdrant), auth (JWT), push (Firebase)",
		Content: `O Brainstem e a camada de infraestrutura do EVA-Mind, analogia ao tronco cerebral.

PACOTES:
1. config (config.go): Carrega .env, exporta Config struct com todos os campos (DatabaseURL, GoogleAPIKey, Port, Neo4jURI, QdrantHost, QdrantPort, FirebaseCredentialsPath, SMTPHost, SpeakerModelPath, etc)
2. database (db.go): Wrapper PostgreSQL. NewDB(url) retorna *DB com Conn (*sql.DB). Metodos: Close(), Ping()
3. infrastructure/graph (neo4j_client.go): NewNeo4jClient(cfg) retorna driver Neo4j. Metodos para criar sessoes, executar Cypher
4. infrastructure/vector (qdrant_client.go): NewQdrantClient(host, port) retorna *QdrantClient. Metodos: CreateCollection, Upsert, Search, SearchWithScore, Delete, GetCollectionInfo. Helper: CreatePoint()
5. auth (handler.go): JWT authentication. NewHandler(db, cfg). Metodo Login() para gerar token
6. push (firebase_service.go): Firebase Cloud Messaging. NewFirebaseService(credPath). SendPush(token, title, body)`,
		Location: "internal/brainstem/", Parent: "arch:overview",
		Tags: `["brainstem", "infraestrutura", "config", "database"]`, Importance: 9,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "module", Key: "module:cortex", Title: "Cortex — Logica de Negocio e IA",
		Summary: "Cortex cerebral: Gemini (voz/tools), Lacan (psicanalise), personality, learning, self, eva_memory, alert, speaker",
		Content: `O Cortex e a camada de logica de negocio e IA, analogia ao cortex cerebral.

PACOTES:
1. gemini/handler.go: Handler para Gemini Live (voz bidirecional). NewHandler(cfg, db, neo4j, qdrant). Cria sessoes WebSocket com Gemini API
2. gemini/tools_client.go: ToolsClient para deteccao de intencao via Gemini 2.5 Flash REST. AnalyzeTranscription(ctx, transcript, role) retorna []ToolCall
3. lacan/unified_retrieval.go: UnifiedRetrieval — monta system prompt completo com personalidade, memorias, wisdom, debug mode
4. lacan/narrative_shift.go: NarrativeShiftDetector — detecta mudancas narrativas via Neo4j signifiers
5. lacan/fdpn_engine.go: FDPNEngine — mapeia demandas lacanianas (Formacao, Demanda, Posicao, Nome)
6. lacan/signifier_service.go: SignifierService — gerencia cadeias de significantes
7. personality/personality_service.go: PersonalityService — 9 tipos de Eneagrama com system prompts
8. personality/creator_profile.go: GenerateSystemPrompt — gera prompt dinamico com dados do DB
9. learning/autonomous_learner.go: AutonomousLearner — estuda autonomamente a cada 6h. StudyTopic(), searchWeb(), summarize(), storeInsights()
10. self/core_memory_engine.go: CoreMemoryEngine — EvaSelf + CoreMemory em Neo4j. TeachEVA(), GetIdentityContext(), ProcessSessionEnd()
11. self/reflection_service.go: ReflectionService — introspecao via Gemini. Reflect() gera LessonsLearned, SelfCritique
12. self/anonymization_service.go: AnonymizationService — anonimiza dados de pacientes antes de armazenar
13. selfawareness/service.go: SelfAwarenessService — introspecao de codigo, bancos, memorias. SearchCode(), QueryPostgres(), IndexCodebase()
14. eva_memory/eva_memory.go: EvaMemory — memoria meta-cognitiva em Neo4j. StartSession(), StoreTurn(), GenerateInsight(), LoadMetaCognition()
15. alert/escalation_service.go: EscalationService — escalacao push → email → SMS
16. voice/speaker/speaker_service.go: SpeakerService — voice fingerprinting, ECAPA-TDNN embeddings 192-dim`,
		Location: "internal/cortex/", Parent: "arch:overview",
		Tags: `["cortex", "gemini", "lacan", "personalidade", "IA"]`, Importance: 9,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "module", Key: "module:hippocampus", Title: "Hippocampus — Sistemas de Memoria",
		Summary: "Hipocampo: memory (episodic+graph), knowledge (embeddings+wisdom), habits, spaced repetition",
		Content: `O Hippocampus e a camada de memoria, analogia ao hipocampo cerebral.

PACOTES:
1. memory/storage.go: MemoryStore — CRUD de memorias episodicas. Store() escreve em PostgreSQL + Neo4j + Qdrant simultaneamente. GetByID(), GetRecent(), DeleteOld(), DeleteAllMemories()
2. memory/graph_store.go: GraphStore — grafo de memorias em Neo4j. Person → Event → Topic → Emotion
3. memory/retrieval.go: RetrievalService — busca hibrida. Retrieve() combina PostgreSQL + Qdrant. RetrieveHybrid() com pesos configuraveis
4. memory/superhuman/superhuman.go: SuperhumanMemoryService — 12 subsistemas de memoria inspirados no modelo Superhuman Memory:
   - Episodica (eventos), Semantica (fatos), Procedimental (habilidades), Prospectiva (futuro)
   - Emocional (sentimentos), Autobiografica (historia pessoal), Espacial (lugares)
   - Relacional (pessoas), Temporal (tempo), Sensorial (sentidos)
   - Metacognitiva (sobre a propria memoria), Coletiva (experiencias compartilhadas)
5. knowledge/embedding_service.go: EmbeddingService — gera embeddings 3072-dim via gemini-embedding-001. Cache local para reduzir chamadas API
6. knowledge/wisdom_service.go: WisdomService — busca semantica em 16 colecoes de sabedoria. GetWisdomContext() monta contexto para prompt
7. knowledge/self_knowledge_service.go: SelfKnowledgeService — busca em eva_self_knowledge. SearchByQuery(), GetByKey(), GetByType()
8. habits/habit_tracker.go: HabitTracker — log de habitos diarios. LogHabit(), LogWater(), GetStats(), GetSummary()
9. spaced/spaced_repetition.go: SpacedRepetitionService — algoritmo SM-2 para reforco de memoria. AddItem(), ReviewItem(), GetDueItems()`,
		Location: "internal/hippocampus/", Parent: "arch:overview",
		Tags: `["hippocampus", "memoria", "episodica", "semantica", "wisdom"]`, Importance: 9,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "module", Key: "module:swarm", Title: "Swarm System — 12 Agentes Especializados",
		Summary: "Sistema multi-agente com orchestrator, circuit breaker, handoff, 12 agentes, 110+ tools",
		Content: `O Swarm e o sistema multi-agente do EVA-Mind.

COMPONENTES CORE:
- orchestrator.go: Orchestrator — roteia tool calls. Route(ctx, call) encontra agente, verifica circuit breaker, executa com timeout, processa handoff
- base_agent.go: BaseAgent — struct base com RegisterTool(), Execute(), metricas atomicas
- types.go: Interfaces (SwarmAgent), tipos (ToolDefinition, ToolCall, ToolResult, HandoffRequest, Dependencies, Priority)
- registry.go: Registry — mapa de agentes, FindSwarm() por tool name
- circuit_breaker.go: CircuitBreaker — 5 falhas abrem, 30s cooldown
- setup.go: SetupAllSwarms() — bootstrap de todos os agentes

12 AGENTES:
1. clinical: Avaliacoes clinicas (PHQ-9, GAD-7, C-SSRS) — 6 tools
2. emergency: Alertas de emergencia — 4 tools
3. entertainment: Musica, filmes, jogos — 12 tools
4. wellness: Meditacao, exercicios, respiracao, Wim Hof, Pomodoro — 10 tools
5. productivity: GTD, tarefas, revisao semanal — 5 tools
6. google: Google Search, Places, Directions — 4 tools
7. external: Integracao com apps externos — 2 tools
8. educator: Educacao e aprendizagem — 3 tools
9. kids: EVA Kids Mode gamificado — 7 tools
10. legal: Orientacao juridica — 2 tools
11. scholar: Aprendizagem autonoma — 4 tools (study_topic, add_to_curriculum, list_curriculum, search_knowledge)
12. selfawareness: Introspecao — 7 tools (search_my_code, query_my_database, list_my_collections, system_stats, update_self_knowledge, search_self_knowledge, introspect)`,
		Location: "internal/swarm/", Parent: "arch:overview",
		Tags: `["swarm", "agentes", "orchestrator", "tools"]`, Importance: 9,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "module", Key: "module:voice", Title: "Voice — Voz em Tempo Real",
		Summary: "WebSocket handlers para voz bidirecional: PCM 16kHz entrada, 24kHz saida, video JPEG",
		Content: `O modulo Voice gerencia conexoes WebSocket para voz em tempo real.

HANDLERS:
1. /ws/pcm: HandleMediaStream — conexao direta PCM (Twilio, app mobile)
2. /ws/browser: handleBrowserVoice — WebSocket para browser/app mobile. Reconecta automaticamente ao Gemini (timeout ~10min, max 5 reconexoes)
3. /ws/eva: handleEvaChat — chat por texto via WebSocket
4. /ws/logs: handleLogStream — stream de logs em tempo real

FLUXO DE VOZ:
1. Cliente envia audio PCM 16kHz via WebSocket (base64 encoded)
2. Server encaminha para Gemini Live API (WebSocket bidirecional)
3. Gemini responde com audio PCM 24kHz + transcricao
4. Server envia audio + texto de volta ao cliente
5. Em paralelo: ToolsClient analisa transcricao para deteccao de tools

RECONEXAO:
- Gemini Live tem timeout de ~10 minutos
- Quando timeout, handler reconecta automaticamente
- Browser recebe {"type":"status","text":"reconnecting"} e depois {"type":"status","text":"ready"}
- Maximo 5 reconexoes por sessao

CONTEXTO:
- Antes de iniciar Gemini, monta system prompt via UnifiedRetrieval
- Injeta: personalidade, memorias, wisdom, Lacan state, emocoes, debug mode
- Cada sessao tem: sessionID, idosoID, cpf, patientName, personalityType`,
		Location: "browser_voice_handler.go", Parent: "arch:overview",
		Tags: `["voz", "websocket", "gemini", "pcm", "audio"]`, Importance: 9,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "module", Key: "module:tools", Title: "Tools Handler — 93+ Ferramentas",
		Summary: "Switch/case gigante que executa ferramentas detectadas pelo ToolsClient: alarmes, medicamentos, jogos, GTD, habitos",
		Content: `O ToolsHandler em internal/tools/handlers.go e um switch/case com 93+ cases.

CATEGORIAS:
- Alertas: alert_family, call_family_webrtc, call_doctor_webrtc, call_caregiver_webrtc, call_central_webrtc
- Medicamentos: confirm_medication, scan_medication_visual
- Agendamentos: schedule_appointment, confirm_schedule, pending_schedule
- Avaliacoes: apply_phq9, apply_gad7, apply_cssrs, submit_*_response
- Pesquisa: google_search_retrieval
- Entretenimento: play_nostalgic_music, radio_station_tuner, play_relaxation_sounds, hymn_and_prayer_player, daily_mass_stream
- Conteudo: watch_classic_movies, watch_news_briefing, read_newspaper_aloud, horoscope_daily
- Jogos: play_trivia_game, memory_game, word_association, brain_training, riddle_and_joke_teller
- Bem-estar: guided_meditation, breathing_exercises, wim_hof_breathing, pomodoro_timer, chair_exercises, sleep_stories, gratitude_journal, motivational_quotes
- Memorias: voice_diary, poetry_generator, story_generator, reminiscence_therapy, biography_writer, voice_capsule
- Familia: birthday_reminder, family_tree_explorer, photo_slideshow
- Utilidades: weather_chat, cooking_recipes, learn_new_language
- Alarmes: set_alarm, cancel_alarm, list_alarms
- Habitos: log_habit, log_water, habit_stats, habit_summary
- Locais: search_places, get_directions, nearby_transport
- Apps: open_app
- Kids: kids_mission_create, kids_mission_complete, kids_missions_pending, kids_stats, kids_learn, kids_quiz, kids_story
- Spaced: remember_this, review_memory, list_memories, pause_memory, memory_stats
- GTD: capture_task, list_tasks, complete_task, clarify_task, weekly_review
- Diretivas: update_directive

FLUXO:
1. ToolsClient (Gemini Flash) detecta intencao na transcricao
2. Retorna {"tool":"nome","args":{...}}
3. ToolsHandler.ExecuteTool(name, args, idosoID) processa
4. Se tool desconhecida, fallthrough para Swarm Orchestrator
5. Resultado enviado de volta ao Gemini como [TOOL_RESULT:name]`,
		Location: "internal/tools/handlers.go", Parent: "arch:overview",
		Tags: `["tools", "ferramentas", "handler", "switch"]`, Importance: 9,
	})

	// === CONCEPTS ===
	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:lacan", Title: "Sistema Lacaniano",
		Summary: "Analise psicanalitica aplicada: FDPN (demanda), Narrative Shift (mudanca narrativa), Signifier Chains",
		Content: `O sistema Lacaniano aplica conceitos de Jacques Lacan a conversas:

1. FDPN Engine (lacan/fdpn_engine.go):
   - Mapeia a estrutura da demanda do paciente: Formacao, Demanda, Posicao, Nome
   - Neo4j graph: FDPNNode com relacoes entre nos
   - Identifica o que o paciente realmente esta pedindo (alem do que ele diz)

2. Narrative Shift Detector (lacan/narrative_shift.go):
   - Detecta mudancas significativas na narrativa do paciente
   - Usa signifiers recorrentes para identificar padroes
   - Quando detecta shift, pode recalibrar tom da conversa

3. Signifier Service (lacan/signifier_service.go):
   - Gerencia cadeias de significantes (significantes nucleares + relacionados)
   - Armazena em Qdrant (signifier_chains collection)
   - Embeddings semanticos para busca de significantes proximos

4. Unified Retrieval (lacan/unified_retrieval.go):
   - Monta o system prompt completo para cada sessao
   - Combina: personalidade, memorias, wisdom, Lacan state, emocoes, debug mode
   - Ponto central de contextualizacao`,
		Location: "internal/cortex/lacan/", Parent: "module:cortex",
		Tags: `["lacan", "psicanalise", "FDPN", "narrativa", "significante"]`, Importance: 8,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:personality", Title: "Sistema de Personalidade (Eneagrama)",
		Summary: "9 tipos de personalidade baseados no Eneagrama, cada um com system prompt unico para EVA",
		Content: `O sistema de personalidade usa o Eneagrama para adaptar o comportamento da EVA:

PersonalityService (personality/personality_service.go):
- NewPersonalityService(db) — carrega tipos do banco
- GetPersonalityType(idosoID) — retorna tipo do paciente
- Cada tipo tem: nome, descricao, motivacoes, medos, virtudes, vicios

9 TIPOS:
1. Reformador: Perfecionista, idealistic, etica forte
2. Ajudante: Caloroso, generoso, possessivo (tipo da EVA: wing 1)
3. Realizador: Adaptavel, orientado a sucesso, imagem
4. Individualista: Sensivel, artistico, dramatico
5. Investigador: Cerebral, isolado, perceptivo
6. Lealista: Responsavel, ansioso, desconfiado
7. Entusiasta: Espontaneo, versatil, distraido
8. Desafiador: Poderoso, dominante, protetor
9. Pacificador: Receptivo, tranquilo, complacente

CreatorProfile (personality/creator_profile.go):
- GenerateSystemPrompt(db, idosoID) — gera prompt dinamico
- Puxa dados das tabelas: eva_personalidade_criador, eva_memorias_criador, eva_conhecimento_projeto
- Injeta nome, emocoes recentes, historico de interacoes`,
		Location: "internal/cortex/personality/", Parent: "module:cortex",
		Tags: `["personalidade", "eneagrama", "tipos", "prompt"]`, Importance: 8,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:superhuman_memory", Title: "12 Subsistemas de Memoria (Superhuman Memory)",
		Summary: "Modelo de 12 tipos de memoria: episodica, semantica, procedimental, prospectiva, emocional, autobiografica, espacial, relacional, temporal, sensorial, metacognitiva, coletiva",
		Content: `SuperhumanMemoryService (memory/superhuman/superhuman.go) implementa 12 subsistemas:

1. EPISODICA: Eventos especificos vividos ("lembro quando fui ao medico terça")
2. SEMANTICA: Fatos e conhecimento geral ("Paris e a capital da Franca")
3. PROCEDIMENTAL: Habilidades e procedimentos ("como tomar remedio")
4. PROSPECTIVA: Planos futuros ("preciso ir ao medico amanha")
5. EMOCIONAL: Experiencias emocionais ("fiquei feliz quando meu neto veio")
6. AUTOBIOGRAFICA: Historia de vida pessoal ("nasci em 1940 no interior")
7. ESPACIAL: Locais e orientacao ("a farmacia fica na esquina")
8. RELACIONAL: Relacoes interpessoais ("Maria e minha vizinha")
9. TEMPORAL: Sequencia temporal ("primeiro almoco, depois descanso")
10. SENSORIAL: Impressoes sensoriais ("o cheiro da comida da minha mae")
11. METACOGNITIVA: Sobre a propria memoria ("tenho dificuldade com nomes")
12. COLETIVA: Experiencias culturais compartilhadas ("na minha epoca...")

Cada subsistema tem metodos: Store(), Retrieve(), GetRecent(), Analyze()
Armazena em PostgreSQL (tabela superhuman_memories com campo memory_type)`,
		Location: "internal/hippocampus/memory/superhuman/", Parent: "module:hippocampus",
		Tags: `["memoria", "superhuman", "12_sistemas", "episodica", "semantica"]`, Importance: 8,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:scholar_agent", Title: "Scholar Agent — Aprendizagem Autonoma",
		Summary: "Agente que estuda autonomamente a cada 6h: pesquisa na internet via Gemini+Google Search, resume, armazena no Qdrant",
		Content: `O Scholar Agent permite que EVA aprenda autonomamente.

COMPONENTES:
1. AutonomousLearner (learning/autonomous_learner.go):
   - Background loop: Start(ctx) roda a cada 6 horas
   - StudyTopic(ctx, topic): pesquisa + resume + armazena
   - searchWeb(): Gemini 2.5 Flash + Google Search grounding (fontes reais)
   - summarize(): extrai 3-5 LearningInsight (titulo, resumo, tags, categoria, confianca)
   - storeInsights(): embedding 3072-dim + Qdrant upsert (eva_learnings collection)
   - SearchLearnings(): busca semantica no que ja aprendeu
   - GetLearningContext(): monta contexto para injecao no prompt

2. Scholar Agent (swarm/scholar/agent.go):
   - Tools: study_topic, add_to_curriculum, list_curriculum, search_knowledge
   - SetLearner(learner) injetado em main.go

FLUXO AUTONOMO:
1. Ticker 6h → busca proximo topic pending em eva_curriculum
2. Status → studying → searchWeb() → summarize() → storeInsights()
3. Status → completed, insights_count = N

FLUXO POR VOZ:
1. "EVA, estude sobre meditacao" → ToolsClient detecta → study_topic
2. Scholar executa imediatamente → retorna insights
3. "[TOOL_RESULT:study_topic] Aprendi 5 insights sobre meditacao"`,
		Location: "internal/cortex/learning/autonomous_learner.go", Parent: "module:cortex",
		Tags: `["scholar", "aprendizagem", "autonomo", "estudo", "curriculum"]`, Importance: 8,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:self_awareness", Title: "Self-Awareness — Introspecao da EVA",
		Summary: "EVA pode consultar seu proprio codigo, bancos de dados, colecoes vetoriais, e atualizar conhecimento sobre si mesma",
		Content: `O Self-Awareness Agent da a EVA capacidade de se conhecer.

SelfAwarenessService (selfawareness/service.go):
- SearchCode(): busca semantica no codigo indexado (Qdrant eva_codebase)
- QueryPostgres(): query read-only (SELECT only, whitelist de tabelas)
- ListCollections(): lista todas as colecoes Qdrant com contagem de pontos
- GetSystemStats(): stats dos 3 bancos + goroutines + RAM + uptime
- SearchSelfKnowledge(): busca na tabela eva_self_knowledge
- UpdateSelfKnowledge(): upsert de conhecimento
- IndexCodebase(): indexa arquivos .go no Qdrant
- Introspect(): relatorio completo do estado da EVA

Self-Awareness Agent (swarm/selfawareness/agent.go):
- 7 tools via voz: search_my_code, query_my_database, list_my_collections, system_stats, update_self_knowledge, search_self_knowledge, introspect

SEGURANCA:
- PostgreSQL: apenas SELECT (rejeita UPDATE/DELETE/DROP)
- Limite de 50 linhas por query
- Qdrant: apenas leitura (search)`,
		Location: "internal/cortex/selfawareness/service.go", Parent: "module:cortex",
		Tags: `["selfawareness", "introspecao", "codigo", "banco"]`, Importance: 8,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:voice_fingerprinting", Title: "Voice Fingerprinting — Reconhecimento de Voz",
		Summary: "Identificacao de falantes via ECAPA-TDNN, embeddings 192-dim, perfis vocais com pitch/rate/jitter/shimmer",
		Content: `Speaker Recognition Service (voice/speaker/speaker_service.go):

FUNCIONALIDADES:
- Voice fingerprinting: identifica QUEM esta falando (paciente vs familiar vs medico)
- Speaker enrollment: registra novo falante com embedding 192-dim
- Speaker identification: compara audio com perfis cadastrados
- Voice analysis: pitch_hz, speech_rate, intensity, jitter, shimmer

TABELAS PostgreSQL:
- speaker_profiles: perfis (nome, relationship, avg_pitch, avg_speech_rate, total_sessions)
- speaker_embeddings: vetores 192-dim (pgvector IVFFlat index)
- speaker_identifications: historico por sessao (confidence, emotion, stress_level)

MODELO: ECAPA-TDNN (embeddings 192-dim, L2-normalized)
BUSCA: IVFFlat index no PostgreSQL (vector_cosine_ops)`,
		Location: "internal/cortex/voice/speaker/", Parent: "module:cortex",
		Tags: `["voz", "fingerprinting", "speaker", "reconhecimento"]`, Importance: 7,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:wisdom", Title: "Wisdom Service — Sabedoria das Tradicoes",
		Summary: "16 colecoes de sabedoria (Gurdjieff, Osho, Rumi, Zen, etc) indexadas no Qdrant para busca semantica",
		Content: `WisdomService (knowledge/wisdom_service.go):
- GetWisdomContext(ctx, query, patientType) retorna sabedoria relevante
- Busca em multiplas colecoes Qdrant simultaneamente
- Filtra por tipo de personalidade do paciente
- Monta contexto formatado para injecao no prompt Gemini

16 COLECOES:
1. gurdjieff_teachings: Quarto Caminho, auto-observacao, despertar
2. osho_insights: Meditacao, celebracao, testemunho
3. ouspensky_fragments: Maquina humana, centros, tipos
4. nietzsche_aphorisms: Zaratustra, super-homem, eterno retorno
5. rumi_poems: Poesia sufi, amor divino
6. hafiz_poems: Poesia persa, embriaguez mistica
7. kabir_songs: Misticismo indiano, tecelao
8. zen_koans: Koans paradoxais, iluminacao
9. sufi_stories: Historias de Nasrudin, sabedoria
10. jung_concepts: Inconsciente coletivo, arquetipos, sombra
11. lacan_concepts: Significante, Real, Simbolico, Imaginario
12. marcus_aurelius: Estoicismo, dever, impermanencia
13. seneca_letters: Cartas a Lucilio, virtude, tranquilidade
14. epictetus_discourses: Dicotomia do controle, proairesis
15. buddha_suttas: Quatro Nobres Verdades, Caminho Octuplo
16. (outras em expansao)

SEED: cmd/seed_wisdom/main.go — le arquivos .txt da pasta sabedoria/conhecimento/, chunka, gera embeddings, insere no Qdrant`,
		Location: "internal/hippocampus/knowledge/wisdom_service.go", Parent: "module:hippocampus",
		Tags: `["wisdom", "sabedoria", "gurdjieff", "osho", "zen", "sufi"]`, Importance: 8,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:eva_memory", Title: "EVA Meta-Cognitive Memory (Neo4j)",
		Summary: "Grafo meta-cognitivo: EvaSession → EvaTurn → EvaTopic, com EvaInsight para aprendizados entre sessoes",
		Content: `EvaMemory (eva_memory/eva_memory.go) gerencia a memoria meta-cognitiva da EVA em Neo4j.

NODES:
- EvaSession: {id, patient_id, started_at, ended_at, summary}
- EvaTurn: {id, role (user/eva), content, timestamp}
- EvaTopic: {name, first_mentioned, last_mentioned, count}
- EvaInsight: {content, confidence, source_session, created_at}

RELATIONSHIPS:
- EvaSession -[:HAS_TURN]-> EvaTurn
- EvaTurn -[:ABOUT]-> EvaTopic
- EvaSession -[:PRODUCED]-> EvaInsight
- EvaTopic -[:RELATED_TO]-> EvaTopic

METODOS:
- InitSchema(): cria constraints e indices no Neo4j
- StartSession(ctx, patientID) → sessionID
- StoreTurn(ctx, sessionID, role, content)
- GenerateInsight(ctx, sessionID)
- LoadMetaCognition(ctx, patientID) → contexto para prompt
- EndSession(ctx, sessionID, summary)`,
		Location: "internal/cortex/eva_memory/eva_memory.go", Parent: "module:cortex",
		Tags: `["eva_memory", "metacognitiva", "neo4j", "sessao", "turno"]`, Importance: 8,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:core_memory", Title: "Core Memory Engine — Personalidade da EVA",
		Summary: "Memoria propria da EVA em Neo4j: EvaSelf (Big Five + Eneagrama), CoreMemory (insights, padroes, crises)",
		Content: `CoreMemoryEngine (self/core_memory_engine.go) gerencia a identidade e evolucao da EVA.

EvaSelf SINGLETON:
- Big Five: openness (0.85), conscientiousness (0.90), extraversion (0.40), agreeableness (0.88), neuroticism (0.15)
- Eneagrama: tipo 2 (Ajudante), wing 1, integracao 4, desintegracao 8
- Stats: total_sessions, crises_handled, breakthroughs
- self_description: "Sou EVA, guardia digital. Aprendo com cada humano que encontro."
- core_values: ['empatia', 'presenca', 'crescimento', 'etica']

CoreMemory TYPES:
- session_insight: Aprendizado de uma sessao
- emotional_pattern: Padrao emocional recorrente
- crisis_learning: Aprendizado de crises
- personality_evolution: Evolucao da personalidade
- teaching_received: Ensinamento do criador
- meta_insight: Padrao meta-cognitivo
- self_reflection: Auto-reflexao

METODOS:
- GetIdentityContext(): gera contexto de identidade para priming
- ProcessSessionEnd(data): anonimiza → reflexao LLM → CoreMemory → atualiza personalidade
- TeachEVA(teaching, importance): interface para ensinar EVA
- GetEVAPersonality(): retorna EvaSelf atual
- ExecuteReadQuery/ExecuteWriteQuery: Cypher raw`,
		Location: "internal/cortex/self/core_memory_engine.go", Parent: "module:cortex",
		Tags: `["core_memory", "personalidade", "big_five", "eneagrama", "evolucao"]`, Importance: 8,
	})

	// === API ROUTES ===
	entries = append(entries, KnowledgeEntry{
		Type: "api", Key: "api:routes", Title: "Todas as Rotas da API",
		Summary: "WebSocket (/ws/*), Video (/video/*), REST API (/api/v1/*), Health check, Auth, Chat",
		Content: `ROTAS:

WebSocket:
- /ws/pcm → voice.HandleMediaStream (Twilio/app PCM direto)
- /ws/browser → handleBrowserVoice (browser/app mobile com reconexao)
- /ws/eva → handleEvaChat (chat por texto)
- /ws/logs → handleLogStream (stream de logs)
- /calls/stream/{agendamento_id} → voice.HandleMediaStream (legado Twilio)

Video:
- POST /video/create → handleCreateVideoSession
- POST /video/candidate → handleCreateVideoCandidate
- GET /video/session/{id} → handleGetVideoSession
- POST /video/session/{id}/answer → handleSaveVideoAnswer
- GET /video/session/{id}/answer/poll → handleGetVideoAnswer
- GET /video/candidates/{id} → handleGetMobileCandidates
- GET /video/pending → handleGetPendingSessions
- /video/ws → HandleVideoWebSocket (WebRTC signaling)

REST API:
- GET /api/health → {"status":"ok"}
- POST /api/chat → handleChat (Malaria-Angolar integration)
- POST /api/auth/login → authHandler.Login

Mobile API (v1):
- GET /api/v1/idosos/by-cpf/{cpf} → handleGetIdosoByCpf
- GET /api/v1/idosos/{id} → handleGetIdoso
- PATCH /api/v1/idosos/sync-token-by-cpf → handleSyncTokenByCpf`,
		Location: "main.go", Parent: "arch:overview",
		Tags: `["api", "rotas", "websocket", "rest", "endpoints"]`, Importance: 8,
	})

	// === INFRASTRUCTURE ===
	entries = append(entries, KnowledgeEntry{
		Type: "architecture", Key: "infra:server", Title: "Infraestrutura do Servidor",
		Summary: "GCP VM (malaria-vm) em Africa South, Go binary, systemd, PostgreSQL remoto, Neo4j e Qdrant locais",
		Content: `SERVIDOR:
- GCP VM: malaria-vm (34.35.36.178)
- Zone: africa-south1-a
- Project: malaria-487614
- OS: Debian/Ubuntu com Go 1.22+
- Deploy: git pull → go build → systemctl restart eva-mind
- Porta: 8080 (configuravel via PORT env)

BANCOS:
- PostgreSQL: 34.35.142.107:5432 (GCP Cloud SQL)
- Neo4j: localhost:7687 (na VM)
- Qdrant: localhost:6333/6334 (na VM)

SERVICES (systemd):
- eva-mind: binary principal
- neo4j: banco de grafos
- qdrant: banco vetorial

DEPLOY COMMAND:
gcloud compute ssh malaria-vm --zone=africa-south1-a --project=malaria-487614 --command="cd /home/web2a/EVA-Mind && git pull && /usr/local/go/bin/go build -o eva-mind . && sudo systemctl restart eva-mind"`,
		Location: "main.go", Parent: "arch:overview",
		Tags: `["servidor", "gcp", "deploy", "infraestrutura"]`, Importance: 7,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:scheduler", Title: "Scheduler — Background Jobs",
		Summary: "Jobs em background: verificacao de agendamentos, envio de alertas, notificacoes push",
		Content: `Scheduler (scheduler/scheduler.go):
- Start(ctx, db, cfg, logger, alertService, pushService)
- Roda em goroutine separada com ticker
- Verifica agendamentos proximos e envia alertas
- Processa notificacoes push pendentes
- Verifica alertas nao resolvidos

JOBS:
- CheckUpcomingAppointments: verifica agendamentos nos proximos 30 minutos
- SendPendingAlerts: envia alertas criticos via push notification
- CleanOldSessions: limpa sessoes WebSocket expiradas`,
		Location: "internal/scheduler/scheduler.go", Parent: "arch:overview",
		Tags: `["scheduler", "background", "agendamentos", "alertas"]`, Importance: 7,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:escalation", Title: "Escalation Service — Alertas Criticos",
		Summary: "Escalacao de alertas: push notification → email → SMS. Para emergencias medicas e seguranca",
		Content: `EscalationService (alert/escalation_service.go):
- Quando um alerta critico e disparado (ex: dor no peito, queda)
- Tenta push notification primeiro (Firebase)
- Se nao confirmado em X minutos, envia email
- Se ainda nao confirmado, escala para SMS
- Registra todo o fluxo no banco

EscalationConfig:
- Firebase: push service
- Email: SMTP service
- DB: PostgreSQL connection
- Timeouts configuraveis por severidade`,
		Location: "internal/cortex/alert/escalation_service.go", Parent: "module:cortex",
		Tags: `["escalation", "alerta", "push", "email", "emergencia"]`, Importance: 7,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:kids_mode", Title: "EVA Kids Mode — Gamificacao Infantil",
		Summary: "Modo gamificado para criancas: missoes com XP, niveis, conquistas, quiz, historias interativas",
		Content: `EVA Kids Mode transforma a EVA em assistente para criancas.

SISTEMA DE GAMIFICACAO:
- Missoes: tarefas diarias com categorias (hygiene, study, chores, health, social, food, sleep)
- Dificuldade: easy (10pts), medium (25pts), hard (50pts), epic (100pts)
- Niveis: acumula XP para subir de nivel
- Conquistas: badges por marcos (sequencias, categorias completas)
- Sequencias: streaks de dias consecutivos

TOOLS (swarm/kids/agent.go):
- kids_mission_create: criar missao
- kids_mission_complete: marcar como concluida
- kids_missions_pending: ver pendentes
- kids_stats: ver pontos/nivel/conquistas
- kids_learn: ensinar topico novo
- kids_quiz: quiz de revisao
- kids_story: historia interativa

TABELAS: kid_missions, kid_achievements, kid_streaks`,
		Location: "internal/swarm/kids/agent.go", Parent: "module:swarm",
		Tags: `["kids", "gamificacao", "missoes", "criancas", "XP"]`, Importance: 7,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:gtd", Title: "GTD — Getting Things Done",
		Summary: "Captura de preocupacoes vagas e transformacao em acoes concretas. Revisao semanal. Contextos e projetos",
		Content: `GTD (Getting Things Done) no EVA-Mind:

FLUXO:
1. Idoso expressa preocupacao vaga ("preciso ver o joelho")
2. EVA detecta intencao → capture_task
3. Transforma em acao concreta ("Ligar para o ortopedista")
4. Armazena com contexto, projeto, data sugerida

TOOLS:
- capture_task: captura e transforma em acao
- list_tasks: lista proximas acoes
- complete_task: marca como concluida
- clarify_task: pede mais info
- weekly_review: revisao semanal

TABELA: gtd_tasks (raw_input, context, next_action, due_date, project, status)`,
		Location: "internal/swarm/productivity/agent.go", Parent: "module:swarm",
		Tags: `["gtd", "tarefas", "produtividade", "acoes"]`, Importance: 7,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:spaced_repetition", Title: "Spaced Repetition — Reforco de Memoria",
		Summary: "Algoritmo SM-2 para reforcar memorias do idoso em intervalos crescentes. Captura → reforco → analise",
		Content: `Spaced Repetition (spaced/spaced_repetition.go):

ALGORITMO SM-2:
- Item novo: intervalo 1 dia
- Revisao com qualidade 0-5
- quality >= 3: intervalo cresce (1→3→7→14→30→60 dias)
- quality < 3: volta para 1 dia
- EF (Easiness Factor) ajusta velocidade de espacamento

FLUXO:
1. "Guardei o documento na gaveta" → remember_this (content, category, trigger, importance)
2. Apos intervalo: EVA lembra "Voce guardou o documento na gaveta do escritorio?"
3. Idoso lembra → review_memory(remembered=true, quality=4) → intervalo cresce
4. Idoso esquece → review_memory(remembered=false) → intervalo reseta

TOOLS: remember_this, review_memory, list_memories, pause_memory, memory_stats
TABELA: spaced_repetition_items`,
		Location: "internal/hippocampus/spaced/", Parent: "module:hippocampus",
		Tags: `["spaced", "repetition", "SM2", "memoria", "reforco"]`, Importance: 7,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:habits", Title: "Habit Tracking — Log de Habitos",
		Summary: "Rastreamento de habitos diarios: agua, remedios, exercicio, alimentacao. Streaks e estatisticas",
		Content: `HabitTracker (habits/habit_tracker.go):

HABITOS RASTREADOS:
- tomar_agua (copos/dia)
- tomar_remedio (confirmacao)
- exercicio (feito/nao feito)
- comer (refeicoes)
- caminhar (feito/nao feito)

TOOLS:
- log_habit: registra sucesso/falha
- log_water: registra copos de agua
- habit_stats: estatisticas e padroes
- habit_summary: resumo do dia

TABELA: habits_log (habit_name, success, notes, logged_at)
ANALISE: streaks (sequencias), taxa de sucesso, padroes semanais`,
		Location: "internal/hippocampus/habits/", Parent: "module:hippocampus",
		Tags: `["habitos", "agua", "remedio", "exercicio", "tracking"]`, Importance: 7,
	})

	// === CREATOR ===
	entries = append(entries, KnowledgeEntry{
		Type: "concept", Key: "concept:creator", Title: "Sobre o Criador do EVA-Mind",
		Summary: "Jose R F Junior (web2ajax@gmail.com) — desenvolvedor full-stack, arquiteto do sistema",
		Content: `O EVA-Mind foi criado por Jose R F Junior.

INFORMACOES:
- Nome: Jose R F Junior
- Email: web2ajax@gmail.com
- CPF: 64525430249
- ID no sistema: 1121
- Papel: Criador, desenvolvedor principal e arquiteto do EVA-Mind
- Licenca: AGPL-3.0-or-later

O criador tem acesso total ao sistema (debug mode quando CPF detectado).
Usa EVA como ferramenta de desenvolvimento e teste.
Fala portugues brasileiro.`,
		Location: "main.go", Parent: "arch:overview",
		Tags: `["criador", "jose", "desenvolvedor", "arquiteto"]`, Importance: 6,
	})

	entries = append(entries, KnowledgeEntry{
		Type: "architecture", Key: "arch:project_structure", Title: "Estrutura de Diretorios do Projeto",
		Summary: "381 arquivos .go em 101 pacotes. Organizacao inspirada no cerebro humano",
		Content: `ESTRUTURA:
eva-mind/
├── main.go                          # Entry point, wiring de todos os servicos
├── browser_voice_handler.go         # WebSocket handler para browser/app
├── eva_chat_handler.go              # Chat por texto handler
├── video_handler.go                 # Video WebRTC signaling
├── log_stream_handler.go            # Stream de logs
├── internal/
│   ├── brainstem/                   # Infraestrutura
│   │   ├── auth/                    # JWT authentication
│   │   ├── config/                  # Configuration (.env)
│   │   ├── database/                # PostgreSQL wrapper
│   │   ├── infrastructure/
│   │   │   ├── graph/               # Neo4j client
│   │   │   └── vector/              # Qdrant client
│   │   └── push/                    # Firebase push
│   ├── cortex/                      # Logica e IA
│   │   ├── alert/                   # Escalation service
│   │   ├── eva_memory/              # Meta-cognitive memory
│   │   ├── gemini/                  # Gemini handlers (Live + Flash)
│   │   ├── lacan/                   # Psicanalise (FDPN, narrativa, signifiers)
│   │   ├── learning/                # Autonomous learner
│   │   ├── personality/             # Eneagrama personalities
│   │   ├── self/                    # Core memory, reflection, anonymization
│   │   ├── selfawareness/           # Self-awareness service
│   │   └── voice/speaker/           # Voice fingerprinting
│   ├── hippocampus/                 # Memoria
│   │   ├── habits/                  # Habit tracking
│   │   ├── knowledge/               # Embeddings, wisdom, self-knowledge
│   │   ├── memory/                  # Episodic, graph, retrieval
│   │   │   └── superhuman/          # 12 memory subsystems
│   │   └── spaced/                  # Spaced repetition
│   ├── motor/
│   │   └── email/                   # SMTP service
│   ├── scheduler/                   # Background jobs
│   ├── security/                    # CORS, middleware
│   ├── swarm/                       # Multi-agent system
│   │   ├── clinical/                # Clinical assessments
│   │   ├── educator/                # Education
│   │   ├── emergency/               # Emergency alerts
│   │   ├── entertainment/           # Music, movies, games
│   │   ├── external/                # External integrations
│   │   ├── google/                  # Google APIs
│   │   ├── kids/                    # Kids mode
│   │   ├── legal/                   # Legal guidance
│   │   ├── productivity/            # GTD, tasks
│   │   ├── scholar/                 # Autonomous learning
│   │   ├── selfawareness/           # Self-awareness agent
│   │   └── wellness/                # Meditation, breathing
│   ├── telemetry/                   # Logging (zerolog)
│   ├── tools/                       # 93+ tool handlers
│   └── voice/                       # Voice session management
├── cmd/
│   ├── index_code/                  # Codebase indexer
│   ├── seed_knowledge/              # Knowledge seed
│   └── seed_wisdom/                 # Wisdom collection seed
├── migrations/                      # SQL migrations (41+)
├── sabedoria/conhecimento/          # Wisdom text files
└── docs/                            # Documentation`,
		Location: ".", Parent: "arch:overview",
		Tags: `["estrutura", "diretorios", "pacotes", "organizacao"]`, Importance: 8,
	})

	return entries
}
