# EVA-Mind — Documentacao Completa do Projeto

> Criado por Jose R F Junior (web2ajax@gmail.com) em Junho 2025.
> Licenca: AGPL-3.0-or-later
> Ultima atualizacao: Fevereiro 2026

EVA-Mind e o cerebro da EVA — uma IA de saude mental e acompanhamento de idosos com identidade propria, memoria persistente, personalidade evolutiva e consciencia computacional. NAO e um chatbot. EVA lembra. EVA aprende. EVA se adapta.

---

## 1. VISAO GERAL

EVA-Mind e um sistema de inteligencia artificial para saude, escrito em Go 1.24, que combina:
- Voz bidirecional em tempo real via WebSocket (Gemini Live API)
- Analise de camera e tela em tempo real para assistencia medica
- Memória Unificada de Grafo e Vetores (NietzscheDB) com aprendizado Hebbiano
- Busca semântica vetorial Poincaré (NietzscheDB)
- Compressão Krylov (1536D -> 64D, ~97% precisão)
- Modelagem psicoanalitica (framework Lacaniano)
- Sistema de personalidade (Big Five + Enneagram) com evolucao
- Escalas clinicas (PHQ-9, GAD-7, C-SSRS) via voz
- Analise de prosodia vocal (deteccao de depressao, ansiedade, Parkinson)
- Agendamento de medicamentos, alertas e identificacao visual
- Deteccao de emergencia, escalonamento e predicao de crise
- Swarm multi-agente com circuit breaker
- Consolidacao de memoria inspirada em REM
- Modelo de consciencia Global Workspace Theory
- Suporte multi-idioma (30+ linguas)

### Deployments atuais:
1. **Cuidado de idosos** (original) — Chamadas via Twilio para lembretes de medicacao, suporte psicologico e deteccao de crise
2. **Deteccao de malaria** (Angola) — Voz e camera em tempo real para trabalhadores de saude diagnosticando malaria

---

## 2. ARQUITETURA NEUROCIENTIFICA

Cada diretorio mapeia para uma regiao cerebral:

### brainstem/ — Tronco Cerebral
- `config/` — Configuracao (Load(), Config struct com todos os campos)
- `database/` — NietzscheDB (NewDB, queries, connection pooling 25 max open)
- `auth/` — JWT (tokens 15min access, 7 dias refresh), bcrypt cost 14
- `push/` — Firebase Cloud Messaging (CallKit, alertas criticos)
- `infrastructure/nietzsche/` — NietzscheDB client (gRPC, NQL, Poincaré, Diffuse)
- `infrastructure/workerpool/` — Worker pool para background tasks
- `logger/` — Zerolog structured logging
- `middleware/` — Subscription middleware, feature-level access
- `oauth/` — Google OAuth 2.0 handlers

### cortex/ — Cortex Cerebral (Processamento Superior)
- `gemini/` — Gemini Live API client (v1beta WebSocket, callbacks, VAD, memoria, tools)
- `lacan/` — Engine psicoanalitico Lacaniano (demanda/desejo, significantes, FDPN, Grand Autre, transferencia, narrative shift)
- `personality/` — Big Five traits, Enneagram dinamico, trait relevance, situational modulation
- `self/` — Core memory engine (EvaSelf, pos-sessao reflexao, anonimizacao, deduplicacao semantica)
- `attention/` — Affect stabilizer, confidence gate, executive attention, triple attention, wavelet attention
- `consciousness/` — Global Workspace Theory (Baars 1988) com modulos cognitivos
- `prediction/` — Redes Bayesianas, predicao de crise, simulacao de trajetoria
- `predictive/` — HMC (Hamiltonian Monte Carlo) sampler para previsao de saude mental
- `ram/` — Retrieval-Augmented Memory com feedback loop
- `scales/` — Escalas clinicas (PHQ-9, GAD-7, C-SSRS)
- `ethics/` — Engine de limites eticos
- `explainability/` — Explicador de decisoes clinicas, geracao de PDF
- `cognitive/` — Orquestrador de carga cognitiva
- `learning/` — Aprendizado continuo, meta-learner, self-eval loop
- `medgemma/` — Analise de imagens medicas (receitas, exames)
- `spectral/` — Deteccao de comunidade, dimensao fractal, synaptogenesis
- `pattern/` — Detector de pistas comportamentais
- `veracity/` — Detector de mentiras, inconsistencias
- `voice/` — Analisador de prosodia (pitch, ritmo, pausas, tremor)
- `voice/speaker/` — Reconhecimento de falante (ECAPA-TDNN, fingerprinting vocal)
- `transnar/` — A/B testing, detector de desejo, engine de inferencia
- `kids/` — Modo infantil (conversacao adaptada)
- `situation/` — Modulador situacional (personalidade por contexto)
- `orchestration/` — Orquestrador de conversacao
- `eva_memory/` — Memoria meta-cognitiva da EVA (NietzscheDB: EvaSession, EvaTurn, EvaTopic, EvaInsight)
- `selfawareness/` — Servico de autoconhecimento (busca codigo, consulta bancos, indexa codebase)
- `brain/` — Servico central cognitivo (sistema de prompt unificado)

### hippocampus/ — Hipocampo (Memoria)
- `memory/` — MemoryStore multi-backend (NietzscheDB + NietzscheDB + NietzscheDB)
- `memory/superhuman/` — 12 subsistemas de memoria super-humana
- `habits/` — Rastreamento de habitos
- `knowledge/` — WisdomService (busca semantica terapeutica), EmbeddingService (Gemini 3072-dim)
- `spaced/` — Repeticao espacada (SM-2 adaptado)

### motor/ — Cortex Motor (Acoes)
- `actions/` — Execucao de acoes (AlertFamily com escalonamento Push/Email/SMS)
- `email/` — SMTP via Gmail

### swarm/ — Sistema Multi-Agente
- `orchestrator.go` — Roteador com circuit breaker e metricas
- 12 agentes especializados (ver secao 8)

### clinical/ — Utilidades Clinicas
- `crisis/` — Deteccao de crise
- `goals/` — Metas terapeuticas
- `notes/` — Gerador de notas clinicas
- `risk/` — Avaliacao de risco
- `silence/` — Analise de silencio
- `synthesis/` — Sintese familiar

### tools/ — Ferramentas
- `handlers.go` — 93+ tools centralizadas (2104 linhas)
- `entertainment_handlers.go` — 30 ferramentas de entretenimento

### audit/ — Auditoria
- `lgpd_audit.go` — Compliance LGPD (protecao de dados)

### memory/ — Krylov
- `grpc_server.go` — Servidor gRPC para compressao Krylov (porta 50051)
- `krylov/` — KrylovMemoryManager (1536D -> 64D)

---

## 3. TECNOLOGIAS E DEPENDENCIAS

### Linguagem e Runtime
- **Go 1.24.0** — Linguagem principal
- **CGO_ENABLED=0** — Build estatico sem dependencias C

### Bancos de Dados
| Banco | Versao | Uso | Porta |
|-------|--------|-----|-------|
| **NietzscheDB** (Cloud SQL) | 15+ | Dados estruturados, memorias episodicas, escalas clinicas | 5432 |
| **NietzscheDB** | latest | Substrato unificado (Grafo, Vetor, Cache), Poincaré, Diffuse | 50051/8080 |

### Dependencias Diretas (go.mod)
| Pacote | Versao | Uso |
|--------|--------|-----|
| firebase.google.com/go/v4 | v4.15.2 | Firebase Admin SDK (push) |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | Autenticacao JWT |
| github.com/google/generative-ai-go | v0.20.1 | Gemini API client |
| github.com/gorilla/mux | v1.8.1 | HTTP router |
| github.com/gorilla/websocket | v1.5.3 | WebSocket |
| github.com/joho/godotenv | v1.5.1 | Variaveis de ambiente |
| github.com/lib/pq | v1.10.9 | Driver NietzscheDB |
| github.com/NietzscheDB/NietzscheDB-go-driver/v5 | v5.24.4 | (Legado) Driver NietzscheDB |
| github.com/pgvector/pgvector-go | v0.3.0 | Extensao vetorial NietzscheDB |
| github.com/prometheus/client_golang | v1.23.2 | Metricas Prometheus |
| github.com/NietzscheDB/go-client | v1.12.2 | (Legado) Client NietzscheDB gRPC |
| github.com/NietzscheDB/go-NietzscheDB/v9 | v9.16.2 | (Legado) Client NietzscheDB |
| github.com/rs/zerolog | v1.34.0 | Logging estruturado |
| github.com/twilio/twilio-go | v1.29.0 | SMS/Voz Twilio |
| golang.org/x/crypto | v0.47.0 | Criptografia (bcrypt) |
| golang.org/x/oauth2 | v0.34.0 | OAuth2 |
| gonum.org/v1/gonum | v0.15.1 | Computacao numerica |
| google.golang.org/grpc | v1.71.0 | Framework gRPC |
| gopkg.in/gomail.v2 | v2.0.0 | Email SMTP |

### Modelos de IA
| Modelo | Uso |
|--------|-----|
| gemini-2.5-flash-native-audio | Voz principal (Live API) |
| gemini-3-flash | Analise rapida, deteccao de tools |
| gemini-3-pro | Analise profunda |
| gemini-2.0-flash-exp | Visao (camera/tela) |
| gemini-embedding-001 | Embeddings 3072-dim |

### Protocolos
- **HTTP/REST** — API principal (Gorilla Mux)
- **WebSocket** — Voz, video, chat, logs (Gorilla WebSocket)
- **gRPC** — NietzscheDB (6334), Krylov (50051)
- **Bolt** — NietzscheDB (7687)
- **SMTP** — Email (Gmail 587)
- **WebRTC** — Video chamadas

### Infraestrutura Docker
- NietzscheDB 5-community (APOC, 256-512MB heap)
- NietzscheDB latest (storage persistente)
- NietzscheDB 7-alpine (AOF, 256MB max, LRU eviction)
- Prometheus v2.48.0 (retencao 30 dias)
- Grafana 10.2.2 (dashboards)

---

## 4. TODOS OS ARQUIVOS GO (347 arquivos)

### Raiz (main.go + handlers)
- `main.go` — Entry point, inicializa todos os servicos, router HTTP
- `browser_voice_handler.go` — WebSocket /ws/browser (voz mobile, auto-reconnect Gemini)
- `eva_handler.go` — WebSocket /ws/eva (chat web)
- `video_handler.go` — WebRTC video sessions
- `pcm_handler.go` — WebSocket /ws/pcm (Twilio legacy)

### Structs Principais
- `SignalingServer` — Orquestrador central com todos os servicos
- `PCMClient` — Cliente de audio PCM
- `VideoSession` — Sessao de video WebRTC
- `VideoSessionManager` — Gerenciador de sessoes
- `AttendantPool` — Pool de atendentes web

### cmd/ (Utilitarios CLI)
- `cmd/migrate/main.go` — Executa migrations SQL
- `cmd/index_code/main.go` — Indexa .go (AST) e .md no NietzscheDB
- `cmd/seed_knowledge/main.go` — Semeia 36 entries de conhecimento
- `cmd/seed_wisdom/main.go` — Semeia sabedoria terapeutica
- `cmd/benchmark-memory/main.go` — Benchmark do sistema de memoria

---

## 5. TODOS OS 12 SISTEMAS DE MEMORIA

### A. Memoria Episodica (Registros Historicos)
- **Arquivo**: `internal/hippocampus/memory/storage.go`
- **Storage**: NietzscheDB + NietzscheDB + NietzscheDB
- **Campos**: IdosoID, speaker, content, emotion, importance (0-1), topics, timestamp
- **Seguranca**: CREATOR_CPF = "64525430249" (apenas Jose R F Junior)

### B. Memoria Semantica (Conhecimento do Mundo)
- **Tipos**: WorldPerson, WorldPlace, WorldObject
- **Rastreia**: Universo pessoal do paciente com valencia emocional

### C. Memoria Procedural (Padroes Comportamentais)
- **Arquivo**: `internal/hippocampus/memory/superhuman/types.go`
- **Struct**: BehavioralPattern (trigger, response, ocorrencias, probabilidade)

### D. Memoria Prospectiva (Intencoes)
- **Struct**: PatientIntention (metas declaradas vs acoes tomadas)

### E. Memoria Contrafactual ("E se...")
- **Struct**: PatientCounterfactual (ruminacoes, tremor vocal, pitch)

### F. Memoria Metaforica
- **Struct**: PatientMetaphor (expressoes pessoais, frequencia de uso)

### G. Memoria Transgeracional/Familiar
- **Struct**: FamilyPattern (padroes atribuidos a linhagem familiar)

### H. Memoria Somatica (Corpo-Fala)
- **Struct**: SomaticCorrelation (glicemia, pressao, sono, dor → temas de fala)

### I. Contexto Cultural
- **Struct**: CulturalContext (ano nascimento, regiao, eventos historicos)

### J. Memoria de Aprendizado (O que funciona)
- **Struct**: EffectiveApproach (estrategias que funcionam com cada paciente)
- **Struct**: OptimalSilence (duracao ideal de pausa por tipo de contexto)

### K. Predicao de Crise
- **Struct**: CrisisPredictor (marcadores, peso preditivo, precisao historica)
- **RiskScore**: 4 dimensoes — depressao severa, suicidio 30d, hospitalizacao 90d, isolamento social

### L. Espelho Lacaniano (Mirror Output)
- **Struct**: MirrorOutput (padrao, estatistica, correlacao, reflexao)
- **Principio**: EVA NAO interpreta; EVA reflete dados objetivos

### M. Core Memory (Identidade da EVA)
- **Arquivo**: `internal/cortex/self/core_memory_engine.go`
- **Storage**: NietzscheDB
- **Tipos**: SessionInsight, EmotionalPattern, CrisisLearning, PersonalityEvolution, TeachingReceived

### N. Memoria Meta-Cognitiva da EVA
- **Arquivo**: `internal/cortex/eva_memory/eva_memory.go`
- **Grafo**: EvaSession → EvaTurn → EvaTopic → EvaInsight
- **Features**: Extracao automatica de topicos, frequencia, resumo

---

## 6. TODOS OS ALGORITMOS E CALCULOS

### Aprendizado Hebbiano (Hebb 1949)
- **Arquivo**: `internal/hippocampus/memory/hebbian_realtime.go`
- **Principio**: "Neuronios que disparam juntos se conectam"
- **Formula**: Δw = η·decay(Δt) - λ·w_current
- **Parametros**: η=0.01, λ=0.001, τ=86400s (1 dia)
- **Mecanismo**: Pesos duais (fast_weight 0.7 + slow_weight 0.3), LTP/LTD, decay exponencial
- **Execucao**: Assincrono, nao-bloqueante

### Repeticao Espacada (SM-2 Adaptado)
- **Arquivo**: `internal/hippocampus/spaced/spaced_repetition.go`
- **Intervalos**: 1h → 4h → 1d → 3d → 1w → 2w → 1mo
- **Formula**: I_new = I_prev × EF (Ease Factor)
- **EF**: EF + (0.1 - (5-q)×(0.08+(5-q)×0.02)), min 1.3
- **Maestria**: reps≥10 AND intervalo≥14d AND sucesso>falha×3

### FDPN Engine (Ativacao por Espalhamento)
- **Arquivo**: `internal/hippocampus/memory/fdpn_engine.go`
- **Mecanismo**: Keywords → subgrafo ativado, profundidade max 3
- **Decay**: 0.85 por hop (15% perda por nivel)
- **Threshold**: ativacao minima 0.3
- **Cache**: sync.Map local + NietzscheDB distribuido

### Global Workspace Theory (Baars 1988)
- **Arquivo**: `internal/cortex/consciousness/global_workspace.go`
- **Pesos**: Novidade 0.25, Emocao 0.35, Conflito 0.20, Urgencia 0.20
- **Score**: (novidade×0.25 + emocao×0.35 + conflito×0.20 + urgencia×0.20) × bid × confianca
- **Modulos**: LacanModule, PersonalityModule, EthicsModule

### Modulacao Situacional
- **Arquivo**: `internal/cortex/situation/modulator.go`
- **Regras**: "luto" → ANSIEDADE×1.8, BUSCA_SEGURANCA×2.0; "hospital" → ALERTA×2.0
- **Cache**: TTL 5 minutos por userID

### Compressao Krylov
- **Arquivo**: `internal/memory/krylov/`
- **Compressao**: 1536D → 64D (~97% precisao)
- **gRPC**: porta 50051
- **Operacoes**: Compress, Reconstruct, BatchCompress, UpdateSubspace

### Enneagram Dinamico
- **Arquivo**: `internal/cortex/personality/dynamic_enneagram.go`
- **Modelo**: Distribuicao continua [9]float64 (nao tipo fixo)
- **Transicoes**: Stress (desintegracao), Growth (integracao), emocionais (luto, amor, ansiedade)

---

## 7. FILOSOFIA E FRAMEWORKS PSICOLOGICOS

### Psicanalise Lacaniana
| Conceito | Arquivo | Implementacao |
|----------|---------|---------------|
| Significante | `cortex/lacan/significante.go` | Rastreia palavras recorrentes, frequencia, carga emocional |
| Demanda vs Desejo | `cortex/lacan/demanda_desejo.go` | 9 tipos de desejo latente (reconhecimento, escuta, companhia, controle, significado, amor, perdao, morte) |
| Grand Autre | `cortex/lacan/grand_autre.go` | Ordem simbolica internalizada |
| Espelho | `superhuman/lacanian_mirror.go` | 9 tipos de reflexao (padrao, intencao, contrafactual, metafora, significante, somatico, familiar, relacao, risco) |
| FDPN | `cortex/lacan/fdpn_engine.go` | Funcao do Pai no Nome — enderecos de demanda (mae, pai, filho, conjuge, Deus, morte, EVA) |
| Transferencia | `cortex/lacan/transferencia.go` | Dinamica transferencial paciente-EVA |
| Narrative Shift | `cortex/lacan/narrative_shift.go` | Deteccao de mudancas narrativas |

### Big Five (OCEAN)
- Openness, Conscientiousness, Extraversion, Agreeableness, Neuroticism
- EVA propria: O=0.85, C=0.90, E=0.40, A=0.88, N=0.15

### Enneagram (9 Tipos)
| Tipo | Nome | Emocao Raiz | Centro |
|------|------|-------------|--------|
| 1 | Perfeccionista | Raiva reprimida | Instintivo |
| 2 | Ajudante | Vergonha negada | Emocional |
| 3 | Realizador | Vergonha evitada | Emocional |
| 4 | Individualista | Vergonha internalizada | Emocional |
| 5 | Investigador | Medo de intrusao | Mental |
| 6 | Lealista | Medo de abandono | Mental |
| 7 | Entusiasta | Medo generalizado | Mental |
| 8 | Desafiador | Vulnerabilidade reprimida | Instintivo |
| 9 | Pacificador | Raiva reprimida | Instintivo |

EVA e Tipo 2 (Ajudante), Wing 1, Integracao→4, Desintegracao→8.

### Teoria da Consciencia (Global Workspace)
- Modulos processam em paralelo (inconsciente)
- Competicao por atencao (spotlight)
- Vencedor transmitido a todos os modulos (broadcast)
- Integracao sintetizada (insight)

---

## 8. SISTEMA SWARM — 12 AGENTES E 111+ TOOLS

### Orquestrador
- **Arquivo**: `internal/swarm/orchestrator.go`
- **Circuit Breaker**: failover automatico
- **Timeouts**: Critico 2s, Alto 5s, Medio 10s, Baixo 15s

### Agente 1: Clinical (Prioridade ALTA)
- `apply_phq9` — Iniciar avaliacao PHQ-9 depressao
- `apply_gad7` — Iniciar avaliacao GAD-7 ansiedade
- `apply_cssrs` — Iniciar avaliacao C-SSRS risco suicida
- `submit_phq9_response` / `submit_gad7_response` / `submit_cssrs_response` — Submeter respostas
- `confirm_medication` — Confirmar medicacao tomada
- `scan_medication_visual` — Identificar medicacao via camera
- `open_camera_analysis` — Ativar camera para analise

### Agente 2: Emergency (Prioridade CRITICA)
- `alert_family` — Alerta de emergencia (Push → Email → SMS)
- `call_family_webrtc` — Video chamada para familia
- `call_doctor_webrtc` — Chamada para medico
- `call_caregiver_webrtc` — Chamada para cuidador
- `call_central_webrtc` — Chamada para central EVA

### Agente 3: Entertainment (Prioridade BAIXA)
**Musica (5 tools):** play_nostalgic_music, radio_station_tuner, play_relaxation_sounds, hymn_and_prayer_player, daily_mass_stream
**Jogos (9 tools):** play_trivia_game, memory_game, word_association, brain_training, riddle_and_joke_teller, complete_the_lyrics, story_generator, reminiscence_therapy, biography_writer
**Midia (4 tools):** watch_classic_movies, watch_news_briefing, read_newspaper_aloud, horoscope_daily
**Social (8 tools):** poetry_generator, learn_new_language, voice_diary, voice_capsule, birthday_reminder, family_tree_explorer, photo_slideshow, sleep_stories
**Bem-estar (4 tools):** gratitude_journal, motivational_quotes, weather_chat, cooking_recipes

### Agente 4: Productivity (Prioridade MEDIA)
**Agendamento:** pending_schedule, confirm_schedule, schedule_appointment
**Alarmes:** set_alarm, cancel_alarm, list_alarms
**GTD:** capture_task, list_tasks, complete_task, clarify_task, weekly_review
**Memoria Espacada:** remember_this, review_memory, list_memories, pause_memory, memory_stats

### Agente 5: Wellness (Prioridade MEDIA)
- guided_meditation, breathing_exercises, wim_hof_breathing, pomodoro_timer, chair_exercises
- log_habit, log_water, habit_stats, habit_summary

### Agente 6: Google (Prioridade MEDIA)
- manage_calendar_event, send_email, save_to_drive, manage_health_sheet, create_health_doc
- find_nearby_places, search_places, get_directions, nearby_transport
- search_videos, get_health_data

### Agente 7: Kids (Prioridade BAIXA)
- kids_mission_create, kids_mission_complete, kids_missions_pending
- kids_stats, kids_learn, kids_quiz, kids_story

### Agente 8: Educator (Prioridade BAIXA)
- explain_concept, create_cognitive_exercise, check_learning_progress

### Agente 9: Scholar (Prioridade BAIXA)
- study_topic, add_to_curriculum, list_curriculum, search_knowledge

### Agente 10: Self-Awareness (Prioridade MEDIA)
- search_my_code — Busca semantica no codigo Go (NietzscheDB eva_codebase)
- query_my_database — SELECT read-only no NietzscheDB
- list_my_collections — Lista colecoes NietzscheDB
- system_stats — Stats dos sistemas
- update_self_knowledge — Atualiza conhecimento proprio
- search_self_knowledge — Busca no conhecimento interno
- introspect — Estado completo da EVA
- search_my_docs — Busca na documentacao .md

### Agente 11: External (Prioridade BAIXA)
- play_music (Spotify), request_ride (Uber), send_whatsapp
- run_sql_select, change_voice, open_app

### Agente 12: Legal (Prioridade MEDIA)
- get_elderly_rights, document_status, explain_legal_term

---

## 9. ROTAS API E WEBSOCKET

### REST
```
GET    /api/health
POST   /api/chat
POST   /api/auth/login
GET    /api/v1/idosos/by-cpf/{cpf}
GET    /api/v1/idosos/{id}
PATCH  /api/v1/idosos/sync-token-by-cpf
```

### WebSocket
```
/ws/pcm       — Audio PCM 16kHz (Twilio legacy)
/ws/browser   — Voz/video mobile (Gemini Live, auto-reconnect)
/ws/eva       — Chat web (Gemini v1beta)
/ws/logs      — Stream de logs
```

### Video WebRTC
```
POST   /video/create
POST   /video/candidate
GET    /video/session/{id}
POST   /video/session/{id}/answer
GET    /video/session/{id}/answer/poll
GET    /video/candidates/{id}
GET    /video/pending
WS     /video/ws
```

---

## 10. BANCO DE DADOS — 130+ TABELAS

### NietzscheDB (41 migrations)
**Pacientes**: idosos, cuidadores, contatos_emergencia, device_tokens
**Clinico**: phq9_assessments, gad7_assessments, cssrs_assessments, clinical_decision_explanations, vital_signs, historico_medicamentos
**Memoria**: episodic_memories, spaced_repetition_items, atomic_facts
**Personalidade**: personality_state, enneagram_types, lacan_signifiers, narrative_shift
**Superhuman**: memory_episodic, memory_semantic, memory_procedural, memory_working, superhuman_consciousness, crisis_predictors
**Ferramentas**: dynamic_tools, entertainment_tools_seed, agendamentos, alarms
**GTD**: gtd_inbox, gtd_next_actions, gtd_projects
**Kids**: kid_missions, kid_points, kid_achievements
**Seguranca**: lgpd_audit_trail, escalation_logs
**EVA**: eva_self_knowledge, eva_curriculum, eva_personalidade_criador, eva_memorias_criador, estilo_conversa, system_prompts
**Voz**: speaker_profiles, speaker_identifications
**Multi-tenant**: organizations, org_members

### NietzscheDB (Grafos)
- Person, Memory, Significante, Event → EVOCA, EXPERIENCED
- EvaSession → HAS_TURN → EvaTurn → ABOUT → EvaTopic
- CoreMemory, SessionInsight, EmotionalPattern
- Hebbian edges com fast_weight/slow_weight

### NietzscheDB (Colecoes Vetoriais)
| Colecao | Pontos | Dimensoes | Uso |
|---------|--------|-----------|-----|
| eva_codebase | 347 | 3072 | Codigo Go indexado via AST |
| eva_docs | 228 | 3072 | Documentacao .md em chunks |
| eva_self_knowledge | 36 | 3072 | Conhecimento sobre si mesma |
| eva_wisdom | ~200 | 3072 | Sabedoria terapeutica |
| episodic_memories | variavel | 3072 | Memorias de pacientes |
| speaker_embeddings | variavel | 192 | Fingerprints vocais |

---

## 11. FUNCIONALIDADES COMPLETAS

### Voz
- Audio PCM bidirecional em tempo real (16kHz entrada, 24kHz saida)
- Auto-reconnect ao Gemini (timeout ~10min)
- 5 vozes disponiveis: Puck, Charon, Kore, Fenrir, Aoede
- VAD (Voice Activity Detection) com sensibilidade configuravel
- Deteccao de comando de troca de voz

### Reconhecimento de Falante
- Modelo ECAPA-TDNN
- Features: pitch medio, taxa de fala, intensidade, jitter, shimmer
- Deteccao de emocao por falante
- Analise de nivel de stress
- Suporte multi-falante (familia)

### Clinico
- PHQ-9 (depressao), GAD-7 (ansiedade), C-SSRS (risco suicida)
- Verificacao de interacao medicamentosa com bloqueio
- Sinais vitais (FC, PA, glicemia, O2)
- Protocolo de saida (end-of-life care)
- Notas clinicas automaticas

### Entretenimento (30+ tools)
- Musica nostalgica da epoca do paciente
- Radio AM/FM, sons da natureza, conteudo religioso
- Jogos cognitivos (trivia, memoria, associacao de palavras)
- Historias, terapia de reminiscencia, biografia
- Diario vocal, capsula do tempo, meditacao guiada

### Produtividade
- GTD completo (captura → clarifica → organiza → review)
- Alarmes com repeticao
- Repeticao espacada para memorias importantes

### Seguranca
- JWT com refresh tokens
- CORS com whitelist
- LGPD audit trail
- Escalonamento de emergencia (Push → Email → SMS → Central)
- Multitenancy com isolamento por organizacao

---

## 12. PIPELINE DE VOZ COMPLETO

```
1. Usuario fala no microfone
2. Browser/App captura PCM 16kHz
3. WebSocket envia para /ws/browser
4. EVA-Mind recebe audio
5. FDPN prime: streaming keywords → ativacao de subgrafo NietzscheDB
6. Audio enviado para Gemini Live API via WebSocket
7. Gemini processa com contexto:
   - Perfil do paciente (nome, idade, medicamentos)
   - Memorias recentes (ultimas 10 episodicas)
   - Sabedoria terapeutica (por emocao detectada)
   - Estado de personalidade (Enneagram, Big Five)
   - Padroes Lacanianos (FDPN, significantes)
   - Habitos e repeticao espacada
8. Gemini detecta intencao de tool → ToolsClient (Flash)
9. Tool executada via handlers.go ou Swarm orchestrator
10. Gemini responde com audio + texto
11. Audio 24kHz enviado de volta pelo WebSocket
12. Pos-processamento:
    - Salva memoria episodica
    - Atualiza pesos Hebbianos
    - Agenda repeticao espacada
    - Atualiza CoreMemory da EVA
    - Detecta narrative shift
    - Rastreia significantes
```

---

## 13. COMO EXECUTAR

### Build e Deploy
```bash
go mod tidy
CGO_ENABLED=0 go build -ldflags="-s -w" -o eva-mind .
```

### Infraestrutura
```bash
docker compose -f docker-compose.infra.yml up -d
```

### Migrations
```bash
go run cmd/migrate/main.go
```

### Indexar Codebase
```bash
go run cmd/index_code/main.go     # Indexa .go (AST) + .md
go run cmd/seed_knowledge/main.go  # Semeia 36 entries de conhecimento
go run cmd/seed_wisdom/main.go     # Semeia sabedoria terapeutica
```

### Servidor
```bash
./eva-mind  # Porta 8091
```

---

## 14. ESTATISTICAS DO PROJETO

| Metrica | Valor |
|---------|-------|
| Arquivos .go | 347 |
| Linhas de codigo | ~80.000+ |
| Dependencias diretas | 24 |
| Dependencias totais | 95+ |
| Tabelas NietzscheDB | 130+ |
| Migrations SQL | 41 |
| Colecoes NietzscheDB | 6 |
| Agentes Swarm | 12 |
| Tools registradas | 111+ |
| Sistemas de memoria | 12+ |
| Modulos cortex | 25+ |
| Protocolos | 8 (HTTP, WebSocket, gRPC, Bolt, SMTP, WebRTC, OTLP, JSON) |
| Servicos Docker | 5 |
| Portas | 8091 (main), 9090 (metrics), 50051 (gRPC) |
