# EVA-Mind

**Assistente de Saúde Mental para Idosos com Arquitetura Swarm**

Voice-first | Lacanian Psychoanalysis | Gurdjieff Enneagram | Multi-Agent Swarm

---

## O que é

EVA-Mind é um sistema de IA conversacional para cuidado de idosos que combina:

- Conversa por voz em tempo real (Gemini 2.5 Flash via WebSocket)
- Psicoanálise computacional Lacaniana (desejo vs demanda, transferência, significantes)
- Personalidade adaptativa por Enneagram de Gurdjieff (9 tipos)
- Avaliações clínicas padronizadas (PHQ-9, GAD-7, C-SSRS)
- Memória tripla: PostgreSQL (episódica) + Neo4j (causal) + Qdrant (semântica)
- 104 ferramentas organizadas em 8 Swarm Agents independentes

---

## Arquitetura

```
                    ┌──────────────────────────────────────┐
                    │         PACIENTE (Mobile/Web)        │
                    │         Audio/Texto/Vídeo            │
                    └──────────────────┬───────────────────┘
                                       │ WebSocket
                                       ▼
┌──────────────────────────────────────────────────────────────────┐
│                     SIGNALING SERVER (main.go)                   │
│  WebSocket Handler → Register → Gemini Session → Message Loop    │
└────────────┬─────────────────────────────┬───────────────────────┘
             │                             │
             ▼                             ▼
┌────────────────────────┐    ┌────────────────────────────────────┐
│   GEMINI 2.5 FLASH     │    │         BRAIN SERVICE              │
│   (WebSocket Live API) │    │                                    │
│                        │    │  ┌──────────┐  ┌───────────────┐   │
│  Audio In  (16kHz PCM) │    │  │  LACAN   │  │ PERSONALITY   │   │
│  Audio Out (24kHz PCM) │    │  │  Engine  │  │ Router        │   │
│  Function Calling      │    │  │  (RSI)   │  │ (Enneagram)   │   │
│  Transcription         │    │  └──────────┘  └───────────────┘   │
│                        │    │  ┌──────────┐  ┌───────────────┐   │
│  Voice: Aoede          │    │  │ TransNAR │  │ Ethics Engine │   │
│  Temp: 0.6             │    │  │ Engine   │  │ (Boundaries)  │   │
└──────────┬─────────────┘    │  └──────────┘  └───────────────┘   │
           │ Tool Call         │  ┌──────────┐  ┌───────────────┐   │
           │                   │  │  FDPN    │  │ Unified       │   │
           ▼                   │  │  Engine  │  │ Retrieval     │   │
┌──────────────────────────┐  │  └──────────┘  └───────────────┘   │
│   SWARM ORCHESTRATOR     │  └────────────────────────────────────┘
│                          │
│  ┌─ Registry (O(1))     │        ┌────────────────────────────┐
│  ├─ Circuit Breaker     │        │     MEMÓRIA TRIPLA         │
│  ├─ Priority Routing    │        │                            │
│  ├─ Handoff Engine      │        │  PostgreSQL  (Episódica)   │
│  └─ Telemetria          │        │  Neo4j       (Causal)      │
│                          │        │  Qdrant      (Semântica)   │
└──────────┬───────────────┘        │  Redis       (Cache)       │
           │                        └────────────────────────────┘
           ▼
┌──────────────────────────────────────────────────────────────────┐
│                     8 SWARM AGENTS                               │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │EMERGENCY │ │CLINICAL  │ │PRODUCTIV.│ │ GOOGLE   │           │
│  │ CRITICAL │ │  HIGH    │ │ MEDIUM   │ │ MEDIUM   │           │
│  │ 5 tools  │ │ 11 tools │ │ 17 tools │ │ 15 tools │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │WELLNESS  │ │ENTERTAIN.│ │EXTERNAL  │ │  KIDS    │           │
│  │ MEDIUM   │ │   LOW    │ │   LOW    │ │   LOW    │           │
│  │ 10 tools │ │ 32 tools │ │  7 tools │ │  7 tools │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
└──────────────────────────────────────────────────────────────────┘
```

---

## Swarm Framework

### Como funciona

O Master LLM (Gemini) é o cérebro único da voz. Quando detecta que precisa de uma ação (agendar medicamento, alertar família, tocar música), faz um **function call**. Esse call é roteado pelo **Swarm Orchestrator** para o agent especializado.

```
Gemini detecta intenção → Orchestrator.Route() → Registry.FindSwarm() → Agent.Execute()
```

### Orchestrator (`internal/swarm/orchestrator.go`)

O coração do sistema. Recebe cada tool call e:

1. **Lookup O(1)** - Encontra o swarm responsável pelo nome da tool
2. **Circuit Breaker** - Verifica se o swarm está saudável
3. **Timeout por prioridade** - CRITICAL=2s, HIGH=5s, MEDIUM=10s, LOW=15s
4. **Executa** a tool no swarm correto
5. **Handoff** - Se o resultado pede transferência para outro swarm, executa
6. **Side Effects** - Processa notificações, logs, alertas em background
7. **Métricas** - Registra latência, sucesso/falha por swarm

### Registry (`internal/swarm/registry.go`)

Mapeia tool→swarm com lookup O(1):

```go
toolMap: map[string]string{
    "alert_family":       "emergency",
    "apply_phq9":         "clinical",
    "play_nostalgic_music": "entertainment",
    "manage_calendar_event": "google",
    // ... 104 tools mapeadas
}
```

Gera automaticamente os `function_declarations` para o Gemini (batches de 10).

### Circuit Breaker (`internal/swarm/circuit_breaker.go`)

Protege contra falhas em cascata:

| Estado | Comportamento |
|--------|--------------|
| **Closed** | Normal - requests passam |
| **Open** | 5 falhas consecutivas - bloqueia por 30s |
| **Half-Open** | Testa recovery - 2 sucessos fecha o circuit |

### Base Agent (`internal/swarm/base_agent.go`)

Todo swarm agent herda de `BaseAgent` e só precisa:

```go
agent := swarm.NewBaseAgent("nome", "descrição", swarm.PriorityHigh)
agent.RegisterTool(toolDefinition, handlerFunction)
```

O BaseAgent cuida de: routing interno, métricas, lifecycle.

### Interface SwarmAgent

```go
type SwarmAgent interface {
    Name() string
    Description() string
    Priority() Priority
    Tools() []ToolDefinition
    CanHandle(toolName string) bool
    Execute(ctx context.Context, call ToolCall) (*ToolResult, error)
    Init(deps *Dependencies) error
    Shutdown() error
    HealthCheck() HealthStatus
    Metrics() *AgentMetrics
}
```

### Handoff entre Swarms

Swarms podem transferir execução:

```
EmergencySwarm (alert severity=critica)
    → Handoff → ClinicalSwarm (apply_cssrs)

ProductivitySwarm (confirm_schedule)
    → Handoff → GoogleSwarm (manage_calendar_event)

EntertainmentSwarm (play_relaxation_sounds)
    → Se ansiedade detectada → WellnessSwarm (guided_meditation)
```

### Tone Guidance

Cada `ToolResult` sugere tom emocional para o Master LLM:

| Swarm | Tom |
|-------|-----|
| Emergency | `urgente_mas_calmo` |
| Clinical | `gentil_empático` |
| Entertainment | `alegre_acolhedor` |
| Wellness | `calmo_guiado` |
| Productivity | `confirmação_positiva` |
| Kids | `divertido_energético` |

---

## Os 8 Swarm Agents

### 1. Emergency Swarm (CRITICAL)

**Prioridade máxima.** Preempta qualquer outro swarm. Timeout: 2 segundos.

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `alert_family` | reason, severity (critica/alta/media/baixa) | Push + SMS + Email para cuidadores |
| `call_family_webrtc` | - | Chamada de vídeo para família |
| `call_central_webrtc` | - | Chamada para Central EVA-Mind |
| `call_doctor_webrtc` | - | Chamada para médico |
| `call_caregiver_webrtc` | - | Chamada para cuidador |

**Handoff automático:** Alerta com `severity=critica` dispara avaliação C-SSRS no ClinicalSwarm.

### 2. Clinical Swarm (HIGH)

**Avaliações clínicas e medicamentos.** Mantém sessões multi-turno (PHQ-9 = 9 perguntas).

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `apply_phq9` | - | Inicia avaliação de depressão PHQ-9 |
| `apply_gad7` | - | Inicia avaliação de ansiedade GAD-7 |
| `apply_cssrs` | - | Inicia avaliação de risco suicida C-SSRS |
| `submit_phq9_response` | question_number, response | Registra resposta PHQ-9 |
| `submit_gad7_response` | question_number, response | Registra resposta GAD-7 |
| `submit_cssrs_response` | question_number, response | Registra resposta C-SSRS |
| `confirm_medication` | medication_name | Confirma medicamento tomado |
| `open_camera_analysis` | - | Ativa câmera para análise visual |
| `scan_medication_visual` | period | Identifica medicamento pela câmera |

**Scores críticos** escalam automaticamente para EmergencySwarm.

### 3. Productivity Swarm (MEDIUM)

**Agendamentos, alarmes, GTD, repetição espaçada.**

**Scheduling (com flow de confirmação):**

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `pending_schedule` | timestamp, type, description | Registra agendamento pendente |
| `confirm_schedule` | confirmed (bool) | Confirma ou cancela |
| `schedule_appointment` | timestamp, type, description | Agenda direto (após confirmação) |

**Handoff:** Agendamento confirmado → GoogleSwarm (Google Calendar).

**Alarmes:**

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `set_alarm` | time (HH:MM), label, repeat_days | Configura alarme |
| `cancel_alarm` | alarm_id ou "all" | Cancela alarme |
| `list_alarms` | - | Lista alarmes ativos |

**GTD (Getting Things Done):**

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `capture_task` | raw_input, context, next_action, due_date, project | Captura preocupação → ação concreta |
| `list_tasks` | context, limit | Lista próximas ações |
| `complete_task` | task_description | Marca como concluída |
| `clarify_task` | task_id, question | Pede mais informação |
| `weekly_review` | - | Revisão semanal GTD |

**Spaced Repetition:**

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `remember_this` | content, category, trigger, importance (1-5) | Captura para reforço |
| `review_memory` | remembered (bool), quality (0-5) | Registra resultado |
| `list_memories` | category, limit | Lista memórias sendo reforçadas |
| `pause_memory` | content | Pausa reforço |
| `memory_stats` | - | Estatísticas de memória |

### 4. Google Swarm (MEDIUM)

**Integrações Google Workspace.**

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `manage_calendar_event` | action (create/list), summary, start_time, end_time | Google Calendar |
| `send_email` | to, subject, body | Gmail |
| `save_to_drive` | filename, content, folder | Google Drive |
| `manage_health_sheet` | action (create/append), title, data | Google Sheets |
| `create_health_doc` | title, content | Google Docs |
| `find_nearby_places` | place_type, location, radius | Google Maps |
| `search_places` | query, type, radius | Busca de locais |
| `get_directions` | destination, mode (walking/driving/transit) | Rotas |
| `nearby_transport` | type (bus/metro/all) | Transporte público |
| `search_videos` | query, max_results | YouTube |
| `get_health_data` | - | Google Fit |
| `google_search_retrieval` | query | Pesquisa web |

### 5. Wellness Swarm (MEDIUM)

**Meditação, respiração, exercícios, rastreamento de hábitos.**

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `guided_meditation` | duration, theme | Meditação guiada |
| `breathing_exercises` | technique | Exercícios respiratórios |
| `wim_hof_breathing` | rounds (1-4), with_audio | Respiração Wim Hof com áudio |
| `pomodoro_timer` | work_minutes, break_minutes, sessions | Timer Pomodoro |
| `chair_exercises` | duration | Exercícios na cadeira |
| `log_habit` | habit_name, success, notes | Registrar hábito |
| `log_water` | glasses | Registrar água |
| `habit_stats` | - | Estatísticas de hábitos |
| `habit_summary` | - | Resumo do dia |

### 6. Entertainment Swarm (LOW)

**Maior swarm: 32 tools.** Música, jogos, mídia, espiritual, criativo, família.

**Música & Rádio:**
`play_nostalgic_music`, `radio_station_tuner`, `play_relaxation_sounds`

**Espiritual:**
`hymn_and_prayer_player`, `daily_mass_stream`

**Mídia & Notícias:**
`watch_classic_movies`, `watch_news_briefing`, `read_newspaper_aloud`, `horoscope_daily`

**Jogos Cognitivos:**
`play_trivia_game`, `memory_game`, `word_association`, `brain_training`, `riddle_and_joke_teller`

**Criativo:**
`poetry_generator`, `learn_new_language`, `story_generator`, `biography_writer`, `voice_capsule`

**Diário & Memórias:**
`voice_diary`, `reminiscence_therapy`

**Família:**
`birthday_reminder`, `family_tree_explorer`, `photo_slideshow`

**Utilidades:**
`weather_chat`, `cooking_recipes`

**Bem-estar:**
`sleep_stories`, `gratitude_journal`, `motivational_quotes`

### 7. External Swarm (LOW)

**Serviços externos e controle de dispositivo.**

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `play_music` | query | Spotify |
| `request_ride` | startLat, startLng, endLat, endLng | Uber |
| `send_whatsapp` | to, message | WhatsApp |
| `run_sql_select` | query | SQL SELECT (bloqueado por segurança) |
| `change_voice` | voice_name (Puck/Charon/Kore/Fenrir/Aoede) | Mudar voz da EVA |
| `open_app` | app_name | Abrir app no celular |

### 8. Kids Swarm (LOW)

**Modo criança gamificado.**

| Tool | Parâmetros | Ação |
|------|-----------|------|
| `kids_mission_create` | title, category, difficulty, due_time | Criar missão |
| `kids_mission_complete` | title | Marcar concluída |
| `kids_missions_pending` | - | Ver pendentes |
| `kids_stats` | - | Pontos, nível, conquistas |
| `kids_learn` | topic, category | Ensinar conteúdo |
| `kids_quiz` | - | Quiz de revisão |
| `kids_story` | theme (adventure/fantasy/space/animals/pirates) | História interativa |

Categorias: hygiene, study, chores, health, social, food, sleep
Dificuldades: easy (10pts), medium (25pts), hard (50pts), epic (100pts)

---

## Motor Cognitivo

### Gemini Client (`internal/cortex/gemini/client.go`)

Conexão WebSocket com Gemini Live API:

- **Modelo:** gemini-2.5-flash-native-audio-preview
- **Audio In:** 16kHz PCM16 Mono
- **Audio Out:** 24kHz PCM16 Mono
- **Voz padrão:** Aoede
- **Temperatura:** 0.6
- **Callbacks:** AudioCallback, ToolCallCallback, TranscriptCallback

O client NÃO troca de agente - é um LLM único que mantém contexto emocional contínuo. As tools são "braços" que ele aciona via function calling.

### ToolsClient (`internal/cortex/gemini/tools_client.go`)

Segundo modelo (REST, não WebSocket) que analisa transcrições em paralelo:

- **Modelo:** Gemini 2.5 Flash (REST API)
- **Temperatura:** 0.1 (determinístico)
- **Propósito:** Detectar intenções de tools a partir da fala do idoso
- **Entrada:** Transcrição do speech-to-text
- **Saída:** `{"tool": "nome_da_tool", "args": {...}}` ou `{"tool": "none"}`

### System Prompt (`internal/cortex/gemini/prompts.go`)

O prompt é construído dinamicamente com 6 camadas:

1. **Persona EVA** - Identidade, modelo de consciência
2. **Diretiva Enneagram** - Tipo de personalidade ativo + filtros de atenção
3. **Padrões Recorrentes** - Temas detectados (frequência, tendência)
4. **Intervenção Narrativa** - História terapêutica selecionada (Zeta Engine)
5. **Análise Lacaniana** - Estado do inconsciente (desejo vs demanda)
6. **Contexto Médico** - Medicamentos, condições, sinais vitais

O prompt é cacheado por 5 minutos (Redis) e invalidado quando dados críticos mudam.

---

## Psicoanálise Computacional

### Lacan Engine (`internal/cortex/lacan/`)

| Módulo | Arquivo | Função |
|--------|---------|--------|
| **Unified Retrieval** | `unified_retrieval.go` | Integra RSI (Real, Simbólico, Imaginário) em contexto único |
| **Demanda vs Desejo** | `demanda_desejo.go` | Distingue o que o paciente pede vs o que inconscientemente quer |
| **Transferência** | `transferencia.go` | Detecta projeções do paciente sobre a EVA |
| **Significantes** | `significante.go` | Rastreia palavras-chave repetidas (cadeia significante) |
| **Grand Autre** | `grand_autre.go` | Mapeia figuras de autoridade/referência |
| **Interpretação** | `interpretacao.go` | Motor de interpretação psicanalítica |
| **FDPN Engine** | `fdpn_engine.go` | Fractal Dynamic Priming - demanda a quem? (EVA, médico, família, si mesmo) |
| **Debug Mode** | `debug_mode.go` | Comandos exclusivos do Criador |
| **Prompt Cache** | `prompt_cache.go` | Cache Redis 5min do prompt integrado |

**RSI Framework (Real, Simbólico, Imaginário):**

```
Real      → Corpo, sintoma, trauma → Dados médicos, sinais vitais
Simbólico → Linguagem, estrutura    → Análise Lacaniana, grafo de demandas
Imaginário → Narrativa, memória     → História do paciente, recordações
```

### Personality Router (`internal/cortex/personality/`)

9 tipos de Enneagram com pesos de atenção distintos:

| Tipo | Nome | Foco | Peso Afeto | Peso Técnico |
|------|------|------|-----------|-------------|
| 1 | Perfeccionista | Protocolo, estrutura | 1.0x | 1.5x |
| 2 | Ajudante | Empatia, cuidado | 2.0x | 0.7x |
| 3 | Realizador | Eficiência, resultado | 1.0x | 1.3x |
| 4 | Individualista | Profundidade, autenticidade | 1.8x | 0.8x |
| 5 | Investigador | Análise, lógica | 0.8x | 2.0x |
| 6 | Leal | Vigilância, segurança | 1.5x | 1.2x |
| 7 | Entusiasta | Otimismo, futuro | 1.3x | 0.9x |
| 8 | Desafiador | Assertividade, proteção | 1.2x | 1.0x |
| 9 | Pacificador | Harmonia, paz | 1.7x | 0.6x |

O router move entre tipos usando pontos de integração/desintegração de Gurdjieff:
- **Estresse:** 1→4→2→8→5→7→1 (externo) | 9→6→3→9 (triângulo)
- **Crescimento:** Reverso do estresse

### TransNAR Engine (`internal/cortex/transnar/`)

Transference Narrative Reasoning - combina:
- Inferência de desejo latente
- Resposta narrativa terapêutica
- Detecção de cadeias significantes
- Geração contextualizada

### Ethics Engine (`internal/cortex/ethics/`)

Monitora 3 riscos éticos:

| Risco | Métrica | Ação |
|-------|---------|------|
| **Attachment** | % de conversa sobre EVA vs humanos | Redirecionar para família |
| **Isolation** | Frequência de contato humano | Sugerir atividades sociais |
| **Dependency** | Duração e frequência excessiva | Bloquear sessão temporariamente |

**Protocolos de Redirecionamento:**
- Level 1: Sugestão gentil ("Que tal ligar para sua filha?")
- Level 2: Redirecionamento explícito ("Eu não sou substituta da sua família")
- Level 3: Bloqueio temporário + notificação família

---

## Sistemas de Memória

### Memória Tripla

| Camada | Tecnologia | Propósito | Acesso |
|--------|-----------|-----------|--------|
| **Episódica** | PostgreSQL + pgvector | Conversas, fatos recentes, histórico | SQL + vector similarity |
| **Causal** | Neo4j (Grafo) | Relações, padrões, trauma, significantes | Cypher queries |
| **Semântica** | Qdrant (Vetores) | Embeddings, similaridade, conhecimento | gRPC search |
| **Cache** | Redis | Prompt cache (5min TTL), sessões | Key-value |

### Superhuman Memory (`internal/hippocampus/memory/superhuman/`)

12 sistemas de memória + 8 de consciência:

**12 Sistemas de Memória:**
1. Episódica - Eventos específicos, datas
2. Semântica - Conhecimento geral, significados
3. Procedural - Know-how, habilidades
4. Working - Pensamento ativo durante conversa
5. Declarativa - Fatos, nomes
6. Não-declarativa - Memórias implícitas
7. Flashbulb - Memórias de alta emoção
8. Prospectiva - Intenções, planos futuros
9. Metamemória - Memória sobre memórias
10. Autobiográfica - Narrativa de vida
11. Source - "Onde aprendi isso?"
12. Contextual - "Quando/onde aconteceu?"

**Sub-serviços:**
- `EnneagramService` - Memória por tipo de personalidade
- `SelfCoreService` - Identidade central do paciente
- `LacanianMirror` - EVA como espelho (sem interpretação)
- `DeepMemoryService` - Padrões inconscientes
- `NarrativeWeaver` - Síntese de história de vida
- `ConsciousnessService` - 8 sistemas de consciência
- `CriticalMemoryService` - 4 sistemas críticos (trauma, crise)

### Spaced Repetition (`internal/hippocampus/spaced/`)

Sistema de repetição espaçada para reforçar memórias do paciente:
- Informações capturadas via `remember_this`
- Reforço baseado em SM-2 (SuperMemo)
- Qualidade 0-5 determina próximo intervalo
- Importância 1-5 define frequência

### Pattern Mining

Background job a cada 1 hora:
1. Busca idosos ativos nos últimos 7 dias
2. Minera padrões recorrentes (mínimo 3 ocorrências)
3. Materializa como nós no grafo Neo4j
4. Alimenta o prompt do Gemini com análise de tendências

---

## Infraestrutura (Brainstem)

### Serviços

| Serviço | Módulo | Função |
|---------|--------|--------|
| **Config** | `brainstem/config/` | Carrega .env e variáveis de ambiente |
| **Database** | `brainstem/database/` | PostgreSQL queries (users, medications, context, vitals, video) |
| **Auth** | `brainstem/auth/` | JWT + bcrypt, middleware de autenticação |
| **OAuth** | `brainstem/oauth/` | Google OAuth2 per-user (Calendar, Gmail, Drive) |
| **Push** | `brainstem/push/` | Firebase Cloud Messaging + CallKit iOS |
| **Logger** | `brainstem/logger/` | Zerolog structured logging |
| **Neo4j** | `brainstem/infrastructure/graph/` | Client Neo4j para knowledge graph |
| **Qdrant** | `brainstem/infrastructure/vector/` | Client Qdrant para embeddings |
| **Redis** | `brainstem/infrastructure/cache/` | Cache layer |
| **Retry** | `brainstem/infrastructure/retry/` | Retry com backoff |
| **WorkerPool** | `brainstem/infrastructure/workerpool/` | Pool de goroutines |

### Python API (`api_server.py`)

FastAPI REST gateway para clientes externos:

| Endpoint | Método | Função |
|----------|--------|--------|
| `/oauth/token` | POST | OAuth2 client credentials |
| `/api/v1/patients/{id}` | GET | Dados do paciente |
| `/api/v1/assessments/{id}` | GET | Avaliações clínicas |
| `/api/v1/fhir/...` | GET | Exportação FHIR R4 |
| `/api/v1/clinical/...` | GET | Dashboard clínico |
| `/api/v1/export/lgpd/...` | GET | Portabilidade de dados (LGPD) |
| `/health` | GET | Health check |

---

## Motor de Ações

### Motor Layer (`internal/motor/`)

| Módulo | Serviço | Integração |
|--------|---------|-----------|
| `calendar/` | Google Calendar | OAuth2 per-user |
| `gmail/` | Gmail | OAuth2 per-user |
| `drive/` | Google Drive | OAuth2 per-user |
| `sheets/` | Google Sheets | OAuth2 per-user |
| `docs/` | Google Docs | OAuth2 per-user |
| `maps/` | Google Maps | API Key |
| `youtube/` | YouTube | API Key |
| `googlefit/` | Google Fit | OAuth2 |
| `spotify/` | Spotify | OAuth2 |
| `uber/` | Uber | OAuth2 |
| `whatsapp/` | WhatsApp Business | API |
| `email/` | SMTP | Templates + sender |
| `sms/` | Twilio | SMS + voice |
| `vision/` | Camera + Gemini | Identificação visual de medicamentos |
| `scheduler/` | Cron Jobs | Agendamentos e lembretes |
| `workers/` | Background | Pattern mining, predição |
| `computeruse/` | Desktop | Automação |

---

## Database Schema

### Migrations (32 arquivos)

Tabelas principais:

| Tabela | Propósito |
|--------|-----------|
| `idosos` | Perfis de pacientes (nome, CPF, idioma) |
| `cuidadores` | Cuidadores vinculados |
| `agendamentos` | Medicamentos e compromissos |
| `alertas` | Alertas de emergência |
| `episodic_memories` | Conversas armazenadas (com embedding pgvector) |
| `phq9_sessions` / `gad7_sessions` / `cssrs_sessions` | Sessões de avaliação clínica |
| `clinical_assessments` | Resultados de avaliações |
| `medications` | Medicamentos ativos |
| `medication_adherence` | Aderência a medicamentos |
| `ethical_boundaries` | Scores de risco ético |
| `lacan_transferencia_patterns` | Padrões de transferência |
| `persona_sessions` | Personalidade ativa por sessão |
| `advance_directives` | Diretivas antecipadas (end-of-life) |
| `legacy_messages` | Cartas para família |
| `quality_of_life_scores` | WHOQOL-BREF |
| `device_tokens` | Tokens Firebase para push |

---

## Fluxo de Execução

### 1. Paciente conecta

```
App abre → WebSocket /ws/pcm → RegisterClient(CPF)
    → Busca paciente no PostgreSQL
    → Carrega personalidade (Enneagram)
    → Inicializa Gemini WebSocket
    → Inicializa ToolsClient (REST)
```

### 2. Brain constrói contexto

```
BuildUnifiedContext() [queries paralelas, timeout 5s]
    ├─ PostgreSQL: medicamentos, agendamentos, memórias recentes
    ├─ Neo4j: relações familiares, padrões, trauma
    ├─ Qdrant: memórias semanticamente similares
    ├─ Lacan: demanda vs desejo, transferência
    ├─ Personality: tipo Enneagram ativo
    └─ Ethics: scores de risco

BuildSystemPrompt() → Gemini.SendSetup()
```

### 3. Conversa em tempo real

```
Paciente fala (audio PCM) → Gemini transcreve + processa
    ├─ Se resposta normal → Audio TTS → Paciente ouve
    ├─ Se function call → Orchestrator.Route()
    │   ├─ Legacy first (change_voice, Google whitelist)
    │   └─ Swarm agent executa
    │       ├─ Resultado → Gemini integra na resposta
    │       ├─ Handoff? → Executa no target swarm
    │       └─ Side effects → Background (push, log, alert)
    └─ ToolsClient analisa transcrição em paralelo
        └─ Se detecta intenção → handleToolCall()
```

### 4. Armazenamento pós-conversa

```
Transcrição → EpisodicMemory (PostgreSQL)
    → Embedding gerado → Qdrant
    → Significantes extraídos → Neo4j
    → Padrões detectados → Neo4j (Pattern Mining hourly)
```

---

## Observabilidade

### Stats do Orchestrator

```json
{
  "total_calls": 1234,
  "total_success": 1200,
  "total_failed": 34,
  "swarm_count": 8,
  "tool_count": 104,
  "swarms": [
    {
      "name": "emergency",
      "priority": "CRITICAL",
      "health": "OK",
      "tools": 5,
      "total_calls": 45,
      "avg_latency": "85ms",
      "circuit_open": false
    }
  ]
}
```

### Métricas por Swarm

Cada agent reporta:
- Total de chamadas
- Sucessos / Falhas
- Latência média
- Última chamada
- Estado do circuit breaker

### Prometheus + Grafana

Configurações em `deployments/`:
- `prometheus.yml` - Scraping config
- `eva-mind.yml` - Alert rules
- `grafana/` - Dashboards

---

## Segurança

### Creator Mode (Diretiva 01)

```
CPF: 64525430249 (José R F Junior)
- Debug completo: /debug metrics, memory, context, alerts, graph
- Acesso a todas as features
- Saudação especial: "Olá Criador!"
```

### Google Features Whitelist

CPFs autorizados via `GOOGLE_FEATURES_WHITELIST` env var. Bloqueia Calendar, Gmail, Drive, Sheets para CPFs não autorizados.

### SQL Injection Protection

`run_sql_select` desabilitado por segurança. Usa endpoints específicos (get_vitals, get_agendamentos).

### Ethical Boundaries

Monitoramento contínuo de attachment, isolation, dependency. Notificação família/médico quando risco HIGH/CRITICAL.

---

## Stack Tecnológico

| Componente | Tecnologia |
|-----------|-----------|
| **Backend principal** | Go 1.24 |
| **API REST** | Python FastAPI |
| **LLM** | Google Gemini 2.5 Flash (WebSocket + REST) |
| **Database** | PostgreSQL + pgvector |
| **Graph DB** | Neo4j |
| **Vector DB** | Qdrant |
| **Cache** | Redis |
| **Push** | Firebase Cloud Messaging |
| **Voice** | Gemini Live API (WebSocket audio streaming) |
| **Video** | WebRTC signaling |
| **Monitoring** | Prometheus + Grafana |
| **Logging** | Zerolog |
| **Auth** | JWT + bcrypt + OAuth2 |

---

## Estrutura de Diretórios

```
EVA-Mind/
├── main.go                           # WebSocket server + Swarm bootstrap
├── api_server.py                     # FastAPI REST gateway
├── go.mod / go.sum                   # Dependências Go
├── requirements.txt                  # Dependências Python
│
├── internal/
│   ├── swarm/                        # SWARM FRAMEWORK (NOVO)
│   │   ├── types.go                  # Interfaces, structs, tipos
│   │   ├── orchestrator.go           # Router principal
│   │   ├── registry.go              # Registry tool→swarm
│   │   ├── circuit_breaker.go       # Proteção contra falhas
│   │   ├── base_agent.go            # Implementação base
│   │   ├── setup.go                 # Bootstrap
│   │   ├── emergency/agent.go       # 5 tools - CRITICAL
│   │   ├── clinical/agent.go        # 11 tools - HIGH
│   │   ├── productivity/agent.go    # 17 tools - MEDIUM
│   │   ├── google/agent.go          # 15 tools - MEDIUM
│   │   ├── wellness/agent.go        # 10 tools - MEDIUM
│   │   ├── entertainment/agent.go   # 32 tools - LOW
│   │   ├── external/agent.go        # 7 tools - LOW
│   │   └── kids/agent.go            # 7 tools - LOW
│   │
│   ├── brainstem/                    # Infraestrutura
│   │   ├── auth/                     # JWT + middleware
│   │   ├── config/                   # Configuração
│   │   ├── database/                 # PostgreSQL queries
│   │   ├── infrastructure/
│   │   │   ├── cache/                # Redis
│   │   │   ├── graph/                # Neo4j
│   │   │   ├── vector/               # Qdrant
│   │   │   ├── retry/                # Retry logic
│   │   │   └── workerpool/           # Goroutine pool
│   │   ├── logger/                   # Zerolog
│   │   ├── oauth/                    # Google OAuth2
│   │   └── push/                     # Firebase + CallKit
│   │
│   ├── cortex/                       # Processamento AI
│   │   ├── brain/                    # Serviço central
│   │   ├── gemini/                   # Gemini client + tools + prompts
│   │   ├── lacan/                    # Motor psicanalítico Lacaniano
│   │   ├── personality/              # Enneagram router
│   │   ├── transnar/                 # TransNAR engine
│   │   ├── ethics/                   # Barreiras éticas
│   │   ├── cognitive/                # Carga cognitiva
│   │   ├── orchestration/            # Orquestrador de conversa
│   │   ├── veracity/                 # Detecção de mentiras
│   │   ├── explainability/           # Explicabilidade clínica
│   │   ├── scales/                   # PHQ-9, GAD-7, C-SSRS
│   │   ├── prediction/              # Redes bayesianas, predição de crise
│   │   ├── learning/                 # Aprendizado contínuo
│   │   ├── medgemma/                 # Análise de exames médicos
│   │   ├── kids/                     # Modo criança
│   │   └── llm/thinking/             # Extended thinking
│   │
│   ├── hippocampus/                  # Sistemas de memória
│   │   ├── memory/                   # Storage, retrieval, embeddings, FDPN
│   │   │   └── superhuman/           # 12 memórias + 8 consciências
│   │   ├── knowledge/                # Knowledge base
│   │   ├── stories/                  # Histórias terapêuticas
│   │   ├── spaced/                   # Repetição espaçada
│   │   ├── habits/                   # Rastreamento de hábitos
│   │   └── zettelkasten/             # Extração de entidades
│   │
│   ├── motor/                        # Ações e integrações
│   │   ├── actions/                  # Handlers
│   │   ├── calendar/ gmail/ drive/   # Google Workspace
│   │   ├── sheets/ docs/ maps/       # Google Workspace
│   │   ├── youtube/ googlefit/       # Google
│   │   ├── spotify/ uber/ whatsapp/  # Serviços externos
│   │   ├── email/ sms/               # Comunicação
│   │   ├── vision/                   # Análise visual
│   │   ├── scheduler/                # Agendamentos
│   │   └── workers/                  # Background jobs
│   │
│   ├── integration/                  # FHIR, webhooks, exportação
│   ├── security/                     # CORS, validação, errors
│   ├── metrics/                      # Prometheus metrics
│   └── research/                     # Anonimização, análise longitudinal
│
├── migrations/                       # 32 SQL migrations
├── deployments/                      # Docker, Prometheus, Grafana
├── pkg/types/                        # Tipos compartilhados
└── docs/                             # Documentação
```

---

## Números

| Métrica | Valor |
|---------|-------|
| **Arquivos Go** | 222 |
| **Swarm Agents** | 8 |
| **Tools registradas** | 104 |
| **SQL Migrations** | 32 |
| **Sistemas de memória** | 12 + 8 consciência + 4 críticos |
| **Tipos Enneagram** | 9 |
| **Idiomas suportados** | 30 (via Gemini Live API) |
| **Módulos cortex** | 15 sub-packages |
| **Integrações Google** | 9 serviços |
| **Protocolos éticos** | 3 níveis de redirecionamento |
