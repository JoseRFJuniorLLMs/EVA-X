# EVA-Mind: Arquitetura dos 3 Clientes Gemini

## Visao Geral

O EVA-Mind possui **3 consumidores distintos** da API Gemini, cada um com comportamento,
protocolo e proposito diferentes. Todos compartilham a mesma infraestrutura (config, DB, etc.)
mas usam clients e rotas separadas.

```
                    ┌─────────────────────────────────────────────┐
                    │              EVA-Mind Server                 │
                    │              (main.go :8091)                 │
                    └─────────┬──────────┬──────────┬─────────────┘
                              │          │          │
                 ┌────────────┤    ┌─────┤    ┌─────┤
                 │            │    │     │    │     │
          ┌──────▼──────┐ ┌──▼────▼─┐ ┌─▼────▼──────┐
          │ geminiWeb   │ │geminiApp│ │geminiSem    │
          │ /ws/eva     │ │/ws/     │ │Memoria      │
          │ WebSocket   │ │browser  │ │/api/chat    │
          │ TEXT chat   │ │WebSocket│ │REST HTTP    │
          └──────┬──────┘ └────┬────┘ └──────┬──────┘
                 │             │              │
          ┌──────▼──────┐ ┌───▼──────┐ ┌─────▼───────┐
          │cortex/gemini│ │ internal/│ │cortex/gemini│
          │  v1beta WS  │ │ gemini   │ │  v1beta REST│
          │  producao   │ │ v1alpha  │ │  AnalyzeText│
          └──────┬──────┘ └────┬─────┘ └──────┬──────┘
                 │             │               │
                 └─────────────┼───────────────┘
                               │
                    ┌──────────▼──────────┐
                    │   Google Gemini API  │
                    │  generativelanguage  │
                    │  .googleapis.com     │
                    └─────────────────────┘
```

---

## 1. geminiWeb — Chat de Texto via WebSocket

**Proposito:** Pagina `/eva` no Malaria-Angolar (web). Chat de texto com a EVA,
com suporte a memoria e ferramentas.

| Item | Valor |
|------|-------|
| **Handler** | `eva_handler.go` → `handleEvaChat()` |
| **Rota** | `/ws/eva` |
| **Protocolo** | WebSocket (texto bidirecional) |
| **Client Go** | `internal/cortex/gemini` (v1beta, producao) |
| **Frontend** | `Malaria-Angolar/frontend/src/pages/EvaPage.tsx` |
| **Modalidade** | AUDIO (Gemini responde com audio, handler extrai transcricao de texto) |
| **Streaming** | Sim — resposta chega em chunks de texto |
| **Memoria** | Preparado (param `memories` no SendSetup, futuro: carregar do memoryStore) |
| **Tools** | Preparado (param `toolsDef` no SendSetup, futuro: tools de malaria) |
| **VAD** | Configurado (START_SENSITIVITY_LOW, END_SENSITIVITY_LOW) |

### Protocolo WebSocket geminiWeb

```
Browser → Server:  {"type":"text", "text":"pergunta do usuario"}
Server → Browser:  {"type":"text", "text":"chunk da resposta"}        (streaming)
Server → Browser:  {"type":"status","text":"ready"}                   (sessao pronta)
Server → Browser:  {"type":"status","text":"turn_complete"}           (resposta finalizada)
Server → Browser:  {"type":"status","text":"interrupted"}             (interrupcao)
Server → Browser:  {"type":"status","text":"error: ..."}              (erro)
```

### Fluxo

1. Browser abre WebSocket em `/ws/eva`
2. Server cria `cortex/gemini.NewClient()` com API key
3. Server envia `SendSetup()` com contexto medico de malaria (5 params)
4. Server envia `{"type":"status","text":"ready"}` ao browser
5. Browser envia texto → Server faz `geminiClient.SendText()`
6. Gemini responde com chunks → Server extrai texto de `modelTurn.parts` e `outputAudioTranscription`
7. Server envia chunks como `{"type":"text"}` ao browser
8. Ao receber `turnComplete`, server envia `{"type":"status","text":"turn_complete"}`

### Arquivos

- `eva_handler.go` — handler WebSocket, goroutines de leitura/escrita
- `internal/cortex/gemini/client.go` — client WebSocket v1beta com mutex, callbacks, VAD
- `frontend/src/pages/EvaPage.tsx` — interface React com chat streaming

---

## 2. geminiApp — Voz e Video via WebSocket (Mobile)

**Proposito:** App mobile EVA-Mobile. Streaming de audio e video em tempo real.
**NAO MODIFICAR** — o app mobile depende deste codigo.

| Item | Valor |
|------|-------|
| **Handler** | `browser_voice_handler.go` → `handleBrowserVoice()` |
| **Rota** | `/ws/browser` |
| **Protocolo** | WebSocket (audio + video + texto bidirecional) |
| **Client Go** | `internal/gemini` (v1alpha, simples) |
| **Frontend** | App mobile (EVA-Mobile) |
| **Modalidade** | AUDIO (PCM 16kHz entrada, PCM 24kHz saida) + VIDEO (JPEG 1FPS) |
| **Streaming** | Sim — audio em tempo real |
| **Memoria** | Nao |
| **Tools** | Nao |
| **VAD** | Nao configurado (usa defaults do Gemini) |

### Protocolo WebSocket geminiApp

```
Browser → Server:  {"type":"audio",  "data":"base64_pcm_16khz"}      (audio do microfone)
Browser → Server:  {"type":"video",  "data":"base64_jpeg"}            (frame da camera)
Browser → Server:  {"type":"text",   "text":"mensagem"}               (texto direto)
Browser → Server:  {"type":"config", "text":"system_prompt"}          (reconfigurar)
Server → Browser:  {"type":"audio",  "data":"base64_pcm_24khz"}      (audio da EVA)
Server → Browser:  {"type":"text",   "text":"transcricao", "data":"user"}  (transcricao do usuario)
Server → Browser:  {"type":"text",   "text":"transcricao"}            (transcricao da EVA)
Server → Browser:  {"type":"status", "text":"ready|turn_complete|interrupted"}
```

### Fluxo

1. App abre WebSocket em `/ws/browser`
2. Server cria `internal/gemini.NewClient()` (v1alpha)
3. Server envia `SendSetup()` com contexto medico (2 params: context, tools)
4. Server envia `{"type":"status","text":"ready"}`
5. App envia audio PCM base64 → Server faz `geminiClient.SendAudio()`
6. App envia frames JPEG → Server faz `geminiClient.SendImage()`
7. Gemini responde com audio + texto → Server envia ambos ao app
8. Server envia transcricoes (input do usuario + output do modelo)

### Arquivos

- `browser_voice_handler.go` — handler WebSocket para mobile (AUDIO + VIDEO)
- `internal/gemini/client.go` — client WebSocket v1alpha simples (sem mutex, sem callbacks)
- `internal/gemini/multimodal_client.go` — suporte a media chunks
- `internal/voice/session_manager.go` — gerenciamento de sessoes de voz

---

## 3. geminiSemMemoria — Chat REST Stateless

**Proposito:** Endpoint REST para o Malaria-Angolar. Pergunta e resposta simples,
sem sessao, sem streaming, sem memoria.

| Item | Valor |
|------|-------|
| **Handler** | `chat_handler.go` → `handleChat()` |
| **Rota** | `POST /api/chat` |
| **Protocolo** | REST HTTP (request/response) |
| **Client Go** | `internal/cortex/gemini` → `AnalyzeText()` (REST, nao WebSocket) |
| **Frontend** | Malaria-Angolar (qualquer componente que use `/api/chat`) |
| **Modalidade** | TEXT (entrada e saida) |
| **Streaming** | Nao — resposta completa em uma unica resposta HTTP |
| **Memoria** | Nao |
| **Tools** | Nao |
| **VAD** | N/A (nao usa audio) |

### Protocolo REST geminiSemMemoria

```
POST /api/chat
Content-Type: application/json

Request:  {"cpf":"12345678900", "message":"Como tratar malaria grave?"}
Response: {"response":"Artesunato EV 2.4 mg/kg...", "cpf":"12345678900"}
```

### Fluxo

1. Frontend faz POST `/api/chat` com mensagem e CPF opcional
2. Handler busca dados do paciente no PostgreSQL (se CPF fornecido)
3. Monta system prompt com contexto de malaria + dados do paciente
4. Chama `gemini.AnalyzeText()` (REST API v1beta, nao WebSocket)
5. Retorna resposta completa em JSON

### Arquivos

- `chat_handler.go` — handler REST, monta prompt, chama AnalyzeText
- `internal/cortex/gemini/rest_client.go` — client REST para Gemini v1beta

---

## Comparacao dos 3 Consumidores

| Caracteristica | geminiWeb | geminiApp | geminiSemMemoria |
|----------------|-----------|-----------|------------------|
| **Rota** | `/ws/eva` | `/ws/browser` | `POST /api/chat` |
| **Protocolo** | WebSocket | WebSocket | REST HTTP |
| **Client** | cortex/gemini | internal/gemini | cortex/gemini REST |
| **API Gemini** | v1beta WS | v1alpha WS | v1beta REST |
| **Audio** | Nao (texto) | Sim (PCM 16/24kHz) | Nao |
| **Video** | Nao | Sim (JPEG) | Nao |
| **Streaming** | Sim | Sim | Nao |
| **Memoria** | Futuro (preparado) | Nao | Nao |
| **Tools** | Futuro (preparado) | Nao | Nao |
| **Sessao** | Sim (WebSocket) | Sim (WebSocket) | Nao (stateless) |
| **VAD** | Sim (LOW) | Nao | N/A |
| **Thread-safe** | Sim (mutex) | Nao | N/A |
| **Frontend** | EvaPage.tsx | App Mobile | Malaria-Angolar |

---

## Pacotes Go Envolvidos

### `internal/gemini/` — Client Simples (v1alpha)

```
internal/gemini/
├── client.go              # WebSocket client simples, SendSetup(context, tools)
├── analysis.go            # ConversationAnalysis struct, AnalyzeTranscript()
├── multimodal_client.go   # SendMediaChunk(), SendMediaBatch()
├── multimodal_client_test.go
└── tools.go               # Definicoes de schemas de tools
```

- **Usado por:** geminiApp (`browser_voice_handler.go`)
- **API:** v1alpha WebSocket
- **Caracteristicas:** Simples, sem mutex, sem callbacks, sem VAD
- **SendSetup:** 2 parametros (context, tools)
- **CRITICO:** NAO MODIFICAR — app mobile depende deste codigo

### `internal/cortex/gemini/` — Client Producao (v1beta)

```
internal/cortex/gemini/
├── client.go              # WebSocket client thread-safe, SendSetup(5 params)
├── handler.go             # ProcessResponse(), tool execution
├── rest_client.go         # AnalyzeText(), AnalyzeAudio() via REST
├── analysis.go            # ConversationAnalysis com metricas de saude
├── actions.go             # AlertFamily, ConfirmMedication, ScheduleAppointment
├── prompts.go             # Templates de system prompts
├── tools.go               # Definicoes extensas de ferramentas (22KB)
└── tools_client.go        # Client REST para execucao de tools (20KB)
```

- **Usado por:** geminiWeb (`eva_handler.go`) + geminiSemMemoria (`chat_handler.go`)
- **API:** v1beta WebSocket + v1beta REST
- **Caracteristicas:** Mutex, callbacks, VAD, tools, memoria, analise de saude
- **SendSetup:** 5 parametros (instructions, voiceSettings, memories, initialAudio, toolsDef)

---

## Handlers no Root (package main)

| Arquivo | Consumer | Funcao Principal |
|---------|----------|------------------|
| `eva_handler.go` | geminiWeb | `handleEvaChat()` — WebSocket texto /ws/eva |
| `browser_voice_handler.go` | geminiApp | `handleBrowserVoice()` — WebSocket voz /ws/browser |
| `chat_handler.go` | geminiSemMemoria | `handleChat()` — REST /api/chat |

---

## Regras de Modificacao

1. **`browser_voice_handler.go`** e **`internal/gemini/`**: NAO MODIFICAR.
   O app mobile (EVA-Mobile) depende diretamente. Qualquer mudanca quebra o app.

2. **`eva_handler.go`**: Pode evoluir livremente. Adicionar memoria, tools, etc.

3. **`chat_handler.go`**: Pode evoluir. E stateless, sem risco de quebrar sessoes.

4. **`internal/cortex/gemini/`**: Evoluir com cuidado — e usado por geminiWeb E
   geminiSemMemoria. Mudancas no client WebSocket nao afetam o REST e vice-versa.

---

## Memoria Meta-Cognitiva (Neo4j)

### Pacote: `internal/cortex/eva_memory/`

A EVA possui memoria meta-cognitiva via Neo4j. Ela sabe o que sabe,
lembra conversas passadas e reconhece padroes.

### Grafo Neo4j

```
(:EvaSession {id, started_at, ended_at, turn_count, status})
    │
    ├──[:HAS_TURN]──▶ (:EvaTurn {id, role, content, timestamp})
    │                      │
    │                      └──[:ABOUT]──▶ (:EvaTopic {name, frequency, first_seen, last_seen})
    │
    └──[:DISCUSSED]──▶ (:EvaTopic)

(:EvaInsight {id, content, type, created_at})
    │
    └──[:ABOUT]──▶ (:EvaTopic)
```

### Nodes

| Node | Proposito |
|------|-----------|
| **EvaSession** | Uma sessao de conversa completa |
| **EvaTurn** | Uma mensagem (user ou assistant) |
| **EvaTopic** | Topico discutido (diagnostico, tratamento, etc.) |
| **EvaInsight** | Padrao ou insight detectado pela EVA |

### Relationships

| Relacao | Significado |
|---------|-------------|
| `(Session)-[:HAS_TURN]->(Turn)` | Sessao contem turno |
| `(Turn)-[:ABOUT]->(Topic)` | Turno trata de topico |
| `(Session)-[:DISCUSSED]->(Topic)` | Sessao discutiu topico |
| `(Insight)-[:ABOUT]->(Topic)` | Insight sobre topico |

### Extracao de Topicos (Keyword-Based)

12 dominios de topicos extraidos automaticamente por keywords:
`diagnostico`, `tratamento`, `malaria_grave`, `epidemiologia`, `especies`,
`prevencao`, `gravidez`, `pediatria`, `laboratorio`, `vetor`, `angola`, `sistema`

### Fluxo Meta-Cognitivo

```
1. Usuario conecta → /ws/eva
2. EvaMemory.StartSession() → cria :EvaSession no Neo4j
3. EvaMemory.LoadMetaCognition() → carrega ultimas sessoes, topicos frequentes, insights
4. Memorias injetadas no system_instruction via SendSetup(memories)
5. A cada mensagem do usuario → EvaMemory.StoreTurn("user", texto)
6. A cada resposta completa → EvaMemory.StoreTurn("assistant", resposta)
7. Topicos extraidos automaticamente e conectados ao grafo
8. Ao desconectar → EvaMemory.EndSession() + DetectPatterns()
```

### Formato da Memoria Injetada

```
=== MEMORIA META-COGNITIVA DA EVA ===
Eu lembro das conversas anteriores. Uso esse conhecimento para dar respostas mais contextualizadas.

=== CONVERSAS RECENTES ===
- 2026-02-17T10:30:00Z (8 turnos): diagnostico, tratamento
- 2026-02-16T14:00:00Z (4 turnos): epidemiologia, angola

=== TOPICOS QUE DOMINO (por frequencia) ===
- tratamento (15x mencionado)
- diagnostico (12x mencionado)
- malaria_grave (8x mencionado)

=== TOPICOS RECENTES (ultimos 7 dias) ===
tratamento, diagnostico, especies

=== MEUS INSIGHTS ===
- [pattern] O topico 'tratamento' foi discutido 15 vezes. Tenho bastante experiencia neste assunto.
```

### Arquivos

| Arquivo | Proposito |
|---------|-----------|
| `internal/cortex/eva_memory/eva_memory.go` | Servico completo: Store, Load, Detect, Insights |
| `internal/cortex/gemini/client.go` | SendSetup injeta memories no system_instruction |
| `eva_handler.go` | Integra EvaMemory no ciclo de vida do WebSocket |
| `main.go` | Inicializa EvaMemory e conecta GraphStore |

---

## Evolucao Planejada

### geminiWeb (proximos passos)
- [x] Memoria meta-cognitiva via Neo4j
- [x] Memorias injetadas no system_instruction
- [x] Historico de conversas persistido no Neo4j
- [x] Deteccao automatica de padroes
- [ ] Ativar tools de malaria (consulta de dados, prescricoes)
- [ ] Adicionar `response_modalities: ["TEXT"]` para respostas puramente texto

### geminiApp (estavel)
- [ ] Migrar de v1alpha para v1beta quando o app mobile estiver pronto para testes
- [ ] Avaliar adicionar VAD para evitar eco do viva-voz

### geminiSemMemoria (estavel)
- [ ] Considerar cache de respostas frequentes
- [ ] Adicionar contexto de dados epidemiologicos em tempo real
