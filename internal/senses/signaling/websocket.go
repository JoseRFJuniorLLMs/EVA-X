package signaling

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/brainstem/database"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"eva-mind/internal/brainstem/infrastructure/redis"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"eva-mind/internal/brainstem/infrastructure/workerpool"
	"eva-mind/internal/cortex/gemini"
	"eva-mind/internal/cortex/personality"
	"eva-mind/internal/cortex/voice"
	"eva-mind/internal/cortex/alert"
	"eva-mind/internal/cortex/brain"
	"eva-mind/internal/cortex/ethics"
	"eva-mind/internal/cortex/cognitive"
	"eva-mind/internal/cortex/lacan"
	"eva-mind/internal/cortex/prediction"
	"eva-mind/internal/cortex/scales"
	"eva-mind/internal/persona"
	"eva-mind/internal/hippocampus/knowledge"
	"eva-mind/internal/hippocampus/memory"
	"eva-mind/internal/hippocampus/memory/superhuman"
	"eva-mind/internal/hippocampus/stories"
	"eva-mind/internal/hippocampus/zettelkasten"
	"eva-mind/internal/motor/actions"
	"eva-mind/internal/motor/email"
	"eva-mind/internal/tools"
	"eva-mind/pkg/types"

	"eva-mind/internal/brainstem/push"

	"github.com/gorilla/websocket"
)

// ✅ Estrutura para parsear análise de áudio
type AudioAnalysisResult struct {
	Emotion   string `json:"emotion"`
	Intensity int    `json:"intensity"`
	Urgency   string `json:"urgency"`
	Notes     string `json:"notes"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// ✅ OTIMIZADO: Adicionado buffer de áudio e mutex
type WebSocketSession struct {
	ID           string
	CPF          string
	IdosoID      int64
	WSConn       *websocket.Conn
	GeminiClient *gemini.Client
	ctx          context.Context
	cancel       context.CancelFunc
	lastActivity time.Time
	mu           sync.RWMutex

	// ✅ NOVO: Buffer de áudio para envio em chunks maiores
	audioBuffer []byte
	audioMutex  sync.Mutex

	// ✅ NOVO: O "Insight" pendente do raciocínio em background
	pendingInsight string
	insightMutex   sync.Mutex
}

// ✅ NOVO MÉTODO: Thread-safe setter para o GraphReasoning usar
func (s *WebSocketSession) SetPendingInsight(insight string) {
	s.insightMutex.Lock()
	defer s.insightMutex.Unlock()
	s.pendingInsight = insight
}

// ✅ NOVO MÉTODO: Thread-safe getter que limpa após ler (consumir uma vez)
func (s *WebSocketSession) ConsumePendingInsight() string {
	s.insightMutex.Lock()
	defer s.insightMutex.Unlock()

	if s.pendingInsight == "" {
		return ""
	}

	// Pega o valor e limpa para não repetir na próxima vez
	insight := s.pendingInsight
	s.pendingInsight = ""
	return insight
}

type SignalingServer struct {
	cfg           *config.Config
	db            *sql.DB
	pushService   *push.FirebaseService
	knowledge     *knowledge.GraphReasoningService
	audioAnalysis *knowledge.AudioAnalysisService // ✅ NOVO
	context       *knowledge.ContextService       // ✅ NOVO: Factual Memory
	tools         *tools.ToolsHandler             // ✅ NOVO: Read-Only Tools
	emailService  *email.EmailService             // ✅ NOVO: Phase 9 Fallback

	// 🔧 Dynamic Tools Discovery Service
	toolDiscovery *tools.ToolDiscoveryService // ✅ NOVO: Ferramentas dinâmicas do banco

	// Zeta / Gap 2 components
	zetaRouter         *personality.ZetaRouter
	storiesRepo        *stories.Repository
	personalityService *personality.PersonalityService
	cortex             *gemini.ToolsClient      // ✅ Phase 10 Cortex
	personaManager     *persona.PersonaManager  // ✅ Multi-Persona System
	prosodyAnalyzer    *voice.ProsodyAnalyzer   // ✅ Voice Biomarkers
	escalationService  *alert.EscalationService // ✅ Alert Escalation (SMS/WhatsApp/Call)
	ethicsBoundary     *ethics.EthicalBoundaryEngine // ✅ Ethics Monitoring
	brainService       *brain.Service           // ✅ Memory Service (Postgres + Qdrant + Neo4j)

	// 🧠 NOVOS: Módulos de Psicologia e Personalidade
	cognitiveOrchestrator *cognitive.CognitiveLoadOrchestrator // ✅ Carga Cognitiva e Ruminação
	crisisPredictor       *prediction.CrisisPredictor          // ✅ Predição de Crises
	clinicalScales        *scales.ClinicalScalesManager        // ✅ PHQ-9, GAD-7, C-SSRS
	deepMemory            *superhuman.DeepMemoryService        // ✅ Memória Profunda (Trauma, Corpo)
	transferenceService   *lacan.TransferenceService           // ✅ Transferência Lacaniana
	demandDesireService   *lacan.DemandDesireService           // ✅ Demanda vs Desejo
	grandAutreService     *lacan.GrandAutreService             // ✅ EVA como Grande Outro
	fdpnEngine            *lacan.FDPNEngine                    // ✅ Grafo do Desejo

	// 📚 Zettelkasten (Obsidian-like Knowledge Management)
	zettelService         *zettelkasten.ZettelService          // ✅ Memória Externa Viva

	// Services for Memory Saver
	qdrantClient     *vector.QdrantClient
	embeddingService *memory.EmbeddingService
	graphStore       *memory.GraphStore
	redis            *redis.Client
	sessions         sync.Map
	clients          sync.Map

	// 🧠 Brain Service (Core Logic) - FIX ERRO #5
	brain interface {
		GetSystemPrompt(ctx context.Context, idosoID int64) (string, error)
	}
}

func NewSignalingServer(
	cfg *config.Config,
	db *sql.DB,
	pushService *push.FirebaseService,
	qdrant *vector.QdrantClient,
	embedder *memory.EmbeddingService,
) *SignalingServer {
	server := &SignalingServer{
		cfg:              cfg,
		db:               db,
		pushService:      pushService,
		qdrantClient:     qdrant,
		embeddingService: embedder,
	}

	log.Printf("🚀 Signaling Server em modo VOZ PURA (Tools desabilitadas)")

	// ✅ NOVO: Inicializar serviço de descoberta dinâmica de ferramentas
	server.toolDiscovery = tools.NewToolDiscoveryService(db)
	log.Printf("✅ Tool Discovery Service inicializado (dynamic=%v)", db != nil)

	// Inicializar Email Service para Phase 9 (Antes de iniciar o ToolsHandler que depende dele)
	if cfg.EnableEmailFallback {
		emailSvc, err := email.NewEmailService(cfg)
		if err != nil {
			log.Printf("⚠️ Signaling: Email service not configured: %v", err)
		} else {
			server.emailService = emailSvc
			log.Println("✅ Signaling: Email service initialized for Phase 9")
		}
	}

	// ✅ NOVO: Wrapper do DB para ContextService
	dbWrapper := &database.DB{Conn: db}
	ctxService := knowledge.NewContextService(dbWrapper)
	server.context = ctxService
	server.tools = tools.NewToolsHandler(dbWrapper, pushService, server.emailService) // ✅ Agora com emailService inicializado

	// ✅ FASE 10: Configurar Callback de Sinalização para Tools (WebRTC, etc)
	server.tools.NotifyFunc = func(idosoID int64, msgType string, payload interface{}) {
		server.sessions.Range(func(key, value interface{}) bool {
			session := value.(*WebSocketSession)
			if session.IdosoID == idosoID {
				msg := ControlMessage{
					Type:    msgType,
					Success: true,
					Payload: payload,
				}
				session.WSConn.WriteJSON(msg)
				log.Printf("📡 [CORTEX-SIGNAL] Enviado '%s' para Idoso %d", msgType, idosoID)
				return false
			}
			return true
		})
	}

	// ✅ NOVO: Inicializar Cortex (Tools Intelligence)
	server.cortex = gemini.NewToolsClient(cfg)
	log.Println("🧠 Signaling: Cortex Intelligence initialized for Phase 10")

	// ✅ NOVO: Inicializar PersonaManager (Multi-Persona System)
	server.personaManager = persona.NewPersonaManager(db)
	log.Println("🎭 Signaling: PersonaManager initialized for Multi-Persona System")

	// ✅ NOVO: Inicializar ProsodyAnalyzer (Voice Biomarkers)
	if prosodyAnalyzer, err := voice.NewProsodyAnalyzer(cfg.GoogleAPIKey, dbWrapper); err != nil {
		log.Printf("⚠️ Erro ao inicializar ProsodyAnalyzer: %v", err)
	} else {
		server.prosodyAnalyzer = prosodyAnalyzer
		log.Println("🎤 Signaling: ProsodyAnalyzer initialized for Voice Biomarkers")
	}

	// ✅ NOVO: Inicializar EscalationService (SMS/WhatsApp/Call)
	escalationCfg := alert.EscalationConfig{
		Firebase: server.pushService,
		DB:       db,
	}
	server.escalationService = alert.NewEscalationService(escalationCfg)
	server.tools.SetEscalationService(server.escalationService) // Wire up to tools handler
	log.Println("🚨 Signaling: EscalationService initialized for Alert Escalation")

	// ✅ NOVO: Inicializar EthicalBoundaryEngine
	server.ethicsBoundary = ethics.NewEthicalBoundaryEngine(db, nil, func(idosoID int64, msgType string, payload interface{}) {
		// Notify via WebSocket
		server.sessions.Range(func(key, value interface{}) bool {
			session := value.(*WebSocketSession)
			if session.IdosoID == idosoID {
				session.WSConn.WriteJSON(map[string]interface{}{
					"type":    msgType,
					"payload": payload,
				})
			}
			return true
		})
	})
	log.Println("🛡️ Signaling: EthicalBoundaryEngine initialized for Ethics Monitoring")

	// ✅ NOVO: Inicializar Knowledge Service (Neo4j Thinking)
	neo4jClient, err := graph.NewNeo4jClient(cfg)
	if err != nil {
		log.Printf("⚠️ Erro ao conectar Neo4j: %v", err)
	} else {
		server.knowledge = knowledge.NewGraphReasoningService(cfg, neo4jClient, ctxService)
		log.Printf("✅ Graph Reasoning Service (Neo4j + Thinking) inicializado")
	}

	// ✅ NOVO: Inicializar Brain Service (Postgres + Qdrant + Neo4j Memory)
	server.brainService = brain.NewService(
		db,
		server.qdrantClient,
		neo4jClient, // Pode ser nil se falhou
		nil,         // unified retrieval (opcional)
		server.personalityService,
		server.zetaRouter,
		server.pushService,
		nil, // embedding service
	)
	log.Println("🧠 Signaling: BrainService initialized for Memory Storage (PG + Qdrant + Neo4j)")

	// ============================================================================
	// 🧠 MÓDULOS DE PSICOLOGIA E PERSONALIDADE
	// ============================================================================

	// ✅ Cognitive Load Orchestrator (Gerencia carga cognitiva, detecta ruminação)
	if server.redis != nil {
		server.cognitiveOrchestrator = cognitive.NewCognitiveLoadOrchestrator(db, server.redis.GetUnderlyingClient())
		log.Println("🧠 Signaling: CognitiveLoadOrchestrator initialized (Carga Cognitiva + Ruminação)")
	}

	// ✅ Crisis Predictor (Prediz risco de crises baseado em features)
	server.crisisPredictor = prediction.NewCrisisPredictor(db)
	log.Println("🔮 Signaling: CrisisPredictor initialized (Predição de Crises)")

	// ✅ Clinical Scales Manager (PHQ-9, GAD-7, C-SSRS)
	server.clinicalScales = scales.NewClinicalScalesManager(dbWrapper)
	log.Println("📊 Signaling: ClinicalScalesManager initialized (PHQ-9, GAD-7, C-SSRS)")

	// ✅ Deep Memory Service (Memória Persistente, Corporal, Compartilhada)
	server.deepMemory = superhuman.NewDeepMemoryService(db)
	log.Println("🧬 Signaling: DeepMemoryService initialized (Trauma, Memória Corporal)")

	// ✅ Lacan: Transference Service (Detecta transferência filial, materna, etc)
	server.transferenceService = lacan.NewTransferenceService(db)
	log.Println("💭 Signaling: TransferenceService initialized (Transferência Lacaniana)")

	// ✅ Lacan: Demand/Desire Service (Extrai desejo latente)
	server.demandDesireService = lacan.NewDemandDesireService()
	log.Println("💫 Signaling: DemandDesireService initialized (Demanda vs Desejo)")

	// ✅ Lacan: Grand Autre Service (EVA como Grande Outro)
	server.grandAutreService = lacan.NewGrandAutreService()
	log.Println("🪞 Signaling: GrandAutreService initialized (EVA como Grande Outro)")

	// ✅ Lacan: FDPN Engine (Grafo do Desejo - A quem o idoso dirige demandas)
	if neo4jClient != nil {
		server.fdpnEngine = lacan.NewFDPNEngine(neo4jClient)
		log.Println("📊 Signaling: FDPNEngine initialized (Grafo do Desejo)")
	}

	// ============================================================================
	// 📚 ZETTELKASTEN (Obsidian-like Knowledge Management)
	// ============================================================================
	if neo4jClient != nil {
		server.zettelService = zettelkasten.NewZettelService(db, neo4jClient)
		log.Println("📚 Signaling: ZettelService initialized (Memória Externa Viva)")
	}

	// ✅ NOVO: Inicializar Redis Client (Audio Buffer)
	redisClient, err := redis.NewClient(cfg)
	if err != nil {
		log.Printf("⚠️ Erro ao conectar Redis: %v", err)
	} else {
		server.redis = redisClient
		server.audioAnalysis = knowledge.NewAudioAnalysisService(cfg, redisClient, ctxService) // ✅ Inicializa Serviço de Áudio com Contexto
		log.Printf("✅ Redis Video Buffer + Audio Analysis inicializado")
	}

	go server.cleanupDeadSessions()
	return server
}

func (s *SignalingServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 📊 Log detalhado da conexão
	remoteAddr := r.RemoteAddr
	userAgent := r.Header.Get("User-Agent")
	origin := r.Header.Get("Origin")
	connID := fmt.Sprintf("conn-%d", time.Now().UnixNano()%100000)

	log.Printf("🔌 [%s] Nova conexão WebSocket", connID)
	log.Printf("   📍 Endereço: %s", remoteAddr)
	log.Printf("   🌐 Origin: %s", origin)
	log.Printf("   📱 User-Agent: %s", userAgent)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ [%s] Falha no upgrade: %v", connID, err)
		return
	}
	defer conn.Close()

	log.Printf("✅ [%s] WebSocket upgrade bem-sucedido", connID)

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Handler de close para diagnóstico
	conn.SetCloseHandler(func(code int, text string) error {
		closeReason := getCloseReason(code)
		log.Printf("🚪 [%s] Conexão fechada: código=%d (%s), motivo='%s'", connID, code, closeReason, text)
		return nil
	})

	var currentSession *WebSocketSession
	messageCount := 0
	startTime := time.Now()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			duration := time.Since(startTime)
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("👋 [%s] Fechamento normal após %v (%d msgs)", connID, duration, messageCount)
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("⚠️ [%s] Fechamento inesperado após %v (%d msgs): %v", connID, duration, messageCount, err)
			} else {
				log.Printf("🔴 [%s] Erro de leitura após %v (%d msgs): %v", connID, duration, messageCount, err)
			}
			break
		}

		messageCount++
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		switch messageType {
		case websocket.TextMessage:
			if messageCount <= 3 {
				// Log das primeiras mensagens para diagnóstico
				msgPreview := string(message)
				if len(msgPreview) > 100 {
					msgPreview = msgPreview[:100] + "..."
				}
				log.Printf("📨 [%s] Msg #%d (text): %s", connID, messageCount, msgPreview)
			}
			currentSession = s.handleControlMessage(conn, message, currentSession)

		case websocket.BinaryMessage:
			if currentSession != nil {
				s.handleAudioMessage(currentSession, message)
			} else if messageCount <= 3 {
				log.Printf("⚠️ [%s] Áudio recebido sem sessão ativa (msg #%d)", connID, messageCount)
			}
		}
	}

	if currentSession != nil {
		log.Printf("🧹 [%s] Limpando sessão: %s", connID, currentSession.ID)
		s.cleanupSession(currentSession.ID)
	} else {
		log.Printf("🔍 [%s] Conexão fechou sem criar sessão (msgs recebidas: %d)", connID, messageCount)
	}
}

// getCloseReason retorna descrição do código de fechamento WebSocket
func getCloseReason(code int) string {
	switch code {
	case websocket.CloseNormalClosure:
		return "Normal"
	case websocket.CloseGoingAway:
		return "GoingAway"
	case websocket.CloseProtocolError:
		return "ProtocolError"
	case websocket.CloseUnsupportedData:
		return "UnsupportedData"
	case websocket.CloseNoStatusReceived:
		return "NoStatusReceived"
	case websocket.CloseAbnormalClosure:
		return "AbnormalClosure"
	case websocket.CloseInvalidFramePayloadData:
		return "InvalidPayload"
	case websocket.ClosePolicyViolation:
		return "PolicyViolation"
	case websocket.CloseMessageTooBig:
		return "MessageTooBig"
	case websocket.CloseMandatoryExtension:
		return "MandatoryExtension"
	case websocket.CloseInternalServerErr:
		return "InternalServerError"
	case websocket.CloseServiceRestart:
		return "ServiceRestart"
	case websocket.CloseTryAgainLater:
		return "TryAgainLater"
	case websocket.CloseTLSHandshake:
		return "TLSHandshake"
	default:
		return fmt.Sprintf("Unknown(%d)", code)
	}
}

func (s *SignalingServer) handleControlMessage(conn *websocket.Conn, message []byte, currentSession *WebSocketSession) *WebSocketSession {
	var msg ControlMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return currentSession
	}

	switch msg.Type {
	case "register":
		log.Printf("╔══════════════════════════════════════════════════════╗")
		log.Printf("🔥 MENSAGEM DE REGISTRO RECEBIDA")
		log.Printf("📋 CPF: %s", msg.CPF)
		log.Printf("╚══════════════════════════════════════════════════════╝")

		idoso, err := s.getIdosoByCPF(msg.CPF)
		if err != nil {
			log.Printf("❌ ERRO: CPF não encontrado no banco de dados: %s", msg.CPF)
			log.Printf("❌ Detalhes do erro: %v", err)
			s.sendError(conn, "CPF não encontrado")
			return currentSession
		}

		log.Printf("✅ CPF encontrado no banco de dados!")
		log.Printf("👤 Idoso ID: %d, Nome: %s", idoso.ID, idoso.Nome)

		s.clients.Store(msg.CPF, conn)
		log.Printf("✅ Cliente armazenado no mapa de clientes")

		registeredMsg := ControlMessage{
			Type:    "registered",
			Success: true,
		}

		log.Printf("╔══════════════════════════════════════════════════════╗")
		log.Printf("📤 ENVIANDO MENSAGEM 'registered' PARA O CLIENTE")
		log.Printf("📦 Payload: %+v", registeredMsg)
		log.Printf("╚══════════════════════════════════════════════════════╝")

		s.sendMessage(conn, registeredMsg)

		log.Printf("✅ Mensagem 'registered' enviada com sucesso!")
		log.Printf("👤 Cliente registrado: %s", msg.CPF)

		return currentSession

	case "start_call":
		if msg.SessionID == "" {
			msg.SessionID = generateSessionID()
		}

		idoso, err := s.getIdosoByCPF(msg.CPF)
		if err != nil {
			s.sendError(conn, "CPF não encontrado")
			return currentSession
		}

		session, err := s.createSession(msg.SessionID, msg.CPF, idoso.ID, idoso.Nome, idoso.VoiceName, conn)
		if err != nil {
			s.sendError(conn, "Erro ao criar sessão")
			return currentSession
		}

		go s.audioClientToGemini(session)
		go s.audioGeminiToClient(session)

		s.sendMessage(conn, ControlMessage{
			Type:      "session_created",
			SessionID: msg.SessionID,
			Success:   true,
		})

		log.Printf("📞 Chamada iniciada: %s", msg.CPF)
		return session

	case "create_scheduled_call":
		log.Printf("╔══════════════════════════════════════════════════════╗")
		log.Printf("📅 CRIAR AGENDAMENTO IMEDIATO (CHAMADA MANUAL)")
		log.Printf("👤 CPF: %s", msg.CPF)
		log.Printf("⏰ Horário: AGORA (immediate: %v)", msg.Payload)
		log.Printf("╚══════════════════════════════════════════════════════╝")

		// Buscar dados completos do idoso
		idoso, err := s.getIdosoByCPF(msg.CPF)
		if err != nil {
			log.Printf("❌ ERRO: CPF não encontrado: %s", msg.CPF)
			s.sendError(conn, "CPF não encontrado")
			return currentSession
		}

		log.Printf("✅ Idoso encontrado: ID=%d, Nome=%s", idoso.ID, idoso.Nome)

		// Criar session ID único
		sessionID := fmt.Sprintf("manual-%d-%d", idoso.ID, time.Now().Unix())

		// Criar sessão Gemini com contexto completo
		session, err := s.createSession(sessionID, msg.CPF, idoso.ID, idoso.Nome, idoso.VoiceName, conn)
		if err != nil {
			log.Printf("❌ ERRO ao criar sessão: %v", err)
			s.sendError(conn, "Erro ao criar sessão")
			return currentSession
		}

		log.Printf("✅ Sessão criada: %s", sessionID)

		// Iniciar fluxos de áudio
		go s.audioClientToGemini(session)
		go s.audioGeminiToClient(session)

		// Confirmar para o app
		s.sendMessage(conn, ControlMessage{
			Type:      "session_created",
			SessionID: sessionID,
			Success:   true,
		})

		log.Printf("📞 Chamada manual iniciada para %s (ID: %d)", idoso.Nome, idoso.ID)
		return session


	case "hangup":
		if currentSession != nil {
			// ✅ NOVO: Enviar buffer restante antes de fechar
			s.flushAudioBuffer(currentSession)
			s.cleanupSession(currentSession.ID)
		}
		return nil

	case "ping":
		s.sendMessage(conn, ControlMessage{Type: "pong"})
		return currentSession

	case "webrtc_signal":
		if msg.TargetCPF == "" {
			return currentSession
		}

		targetConn, ok := s.clients.Load(msg.TargetCPF)
		if !ok {
			log.Printf("⚠️ [SIGNAL] Target CPF not found: %s", msg.TargetCPF)
			return currentSession
		}

		// Repassar mensagem exatamente como recebida (Relay)
		// Mas podemos injetar o SenderCPF para quem recebe saber quem mandou
		// Se msg.CPF não estiver preenchido, tentar pegar da sessão atual se existir
		senderCPF := msg.CPF
		if senderCPF == "" && currentSession != nil {
			senderCPF = currentSession.CPF
		}

		relayMsg := ControlMessage{
			Type:      "webrtc_signal",
			CPF:       senderCPF, // Sender
			TargetCPF: msg.TargetCPF,
			Payload:   msg.Payload,
		}

		s.sendMessage(targetConn.(*websocket.Conn), relayMsg)
		// log.Printf("📡 [SIGNAL] Relay de %s -> %s", senderCPF, msg.TargetCPF)
		return currentSession

	default:
		return currentSession
	}
}

func (s *SignalingServer) handleAudioMessage(session *WebSocketSession, pcmData []byte) {
	session.mu.Lock()
	session.lastActivity = time.Now()
	session.mu.Unlock()

	// ✅ CLOSED LOOP: Verificar se há insight pendente do raciocínio
	// Se houver, enviamos como TEXTO (System Note) antes do áudio
	// Isso garante que o Gemini processe o contexto antes de ouvir a nova fala
	if insight := session.ConsumePendingInsight(); insight != "" {
		log.Printf("💉 [INJECTION] Injetando insight no fluxo: %s", insight)

		systemNote := fmt.Sprintf(`
[SISTEMA - INFORMAÇÃO CRÍTICA DO BACKGROUND]
Análise clínica recente (Neo4j): %s
Use isso para guiar sua resposta ao próximo áudio.
`, insight)

		if err := session.GeminiClient.SendText(systemNote); err != nil {
			log.Printf("⚠️ Erro ao injetar insight: %v", err)
		}
	}

	// ✅ REDIS: Salvar chunk no buffer distribuído para análise posterior
	// PERFORMANCE FIX: Usar WorkerPool para controlar goroutines
	if s.redis != nil {
		pcmCopy := make([]byte, len(pcmData))
		copy(pcmCopy, pcmData)
		sessionID := session.ID
		workerpool.IOPool.TrySubmit(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			s.redis.AppendAudioChunk(ctx, sessionID, pcmCopy)
		})
	}

	if err := session.GeminiClient.SendAudio(pcmData); err != nil {
		log.Printf("❌ Erro ao enviar áudio para Gemini")
	}
}

func (s *SignalingServer) audioClientToGemini(session *WebSocketSession) {
	<-session.ctx.Done()
}

func (s *SignalingServer) audioGeminiToClient(session *WebSocketSession) {
	for {
		select {
		case <-session.ctx.Done():
			return
		default:
			response, err := session.GeminiClient.ReadResponse()
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			s.handleGeminiResponse(session, response)
		}
	}
}

func (s *SignalingServer) handleGeminiResponse(session *WebSocketSession, response map[string]interface{}) {
	// ✅ LOG: Mostrar resposta completa do Gemini
	log.Printf("🔥 [GEMINI RESPONSE] Tipo de resposta recebida")

	if setupComplete, ok := response["setupComplete"].(bool); ok && setupComplete {
		log.Printf("✅ [GEMINI] Setup completo @ 24kHz PCM16")
		return
	}

	// Processar serverContent
	serverContent, ok := response["serverContent"].(map[string]interface{})
	if !ok {
		log.Printf("⚠️ [GEMINI] Sem serverContent na resposta")
		return
	}

	log.Printf("📦 [GEMINI] serverContent recebido, processando...")

	// ========== TRANSCRIÇÃO NATIVA ==========
	// Capturar transcrição do USUÁRIO (input audio)
	if inputTrans, ok := serverContent["inputAudioTranscription"].(map[string]interface{}); ok {
		if userText, ok := inputTrans["text"].(string); ok && userText != "" {
			log.Printf("🗣️ [NATIVE] IDOSO: %s", userText)
			// PERFORMANCE FIX: Usar WorkerPool para todas as goroutines
			idosoID := session.IdosoID

			workerpool.IOPool.TrySubmit(func() {
				s.saveTranscription(idosoID, "user", userText)
			})

			// ✅ NOVO: Salvar em Postgres + Qdrant + Neo4j via BrainService
			if s.brainService != nil {
				brainSvc := s.brainService
				workerpool.AnalysisPool.TrySubmit(func() {
					brainSvc.ProcessUserSpeech(context.Background(), idosoID, userText)
				})
			}

			// ✅ NOVO: Neo4j Thinking Mode (Fase 2)
			if s.knowledge != nil {
				knowledgeSvc := s.knowledge
				workerpool.AnalysisPool.TrySubmit(func() {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()
					reasoning, err := knowledgeSvc.AnalyzeGraphContext(ctx, idosoID, userText)
					if err == nil && reasoning != "" {
						log.Printf("💡 [NEO4J] Insight gerado: %s", reasoning)
						session.SetPendingInsight(reasoning)
					}
				})
			}

			// ✅ FASE 10: Cortex Intention Analysis (Bicameral Brain)
			if s.cortex != nil {
				workerpool.AnalysisPool.TrySubmit(func() {
					s.runCortexAnalysis(session, userText)
				})
			}

			// ✅ NOVO: Ethics Boundary Check (Dependência Emocional)
			if s.ethicsBoundary != nil {
				ethicsSvc := s.ethicsBoundary
				workerpool.BackgroundPool.TrySubmit(func() {
					event, err := ethicsSvc.AnalyzeEthicalBoundaries(idosoID, userText)
					if err != nil {
						log.Printf("⚠️ [ETHICS] Erro: %v", err)
						return
					}
					if event != nil && (event.Severity == "high" || event.Severity == "critical") {
						log.Printf("🛡️ [ETHICS] Evento detectado: %s (severidade: %s)", event.EventType, event.Severity)
						// Notificar se família/médico devem ser alertados
						if event.FamilyNotified || event.DoctorNotified {
							log.Printf("⚠️ [ETHICS] Notificação enviada - Família: %v, Médico: %v",
								event.FamilyNotified, event.DoctorNotified)
						}
					}
				})
			}

			// ============================================================================
			// 🧠 ANÁLISE PSICOLÓGICA COMPLETA (Lacan + Cognitivo + Clínico)
			// ============================================================================

			// ✅ Lacan: Detectar Transferência (filial, materna, conjugal, paterna)
			// PERFORMANCE FIX: Usar WorkerPool
			if s.transferenceService != nil {
				transSvc := s.transferenceService
				workerpool.BackgroundPool.TrySubmit(func() {
					ctx := context.Background()
					transType := transSvc.DetectTransference(ctx, idosoID, userText)
					if transType != lacan.TRANSFERENCIA_NENHUMA {
						log.Printf("💭 [LACAN] Transferência detectada: %s", transType)
					}
				})
			}

			// ✅ Lacan: Analisar Demanda vs Desejo (desejo latente por trás da fala)
			if s.demandDesireService != nil {
				demandSvc := s.demandDesireService
				fdpnEng := s.fdpnEngine
				workerpool.BackgroundPool.TrySubmit(func() {
					analysis := demandSvc.AnalyzeUtterance(userText)
					if analysis.LatentDesire != lacan.DESEJO_INDEFINIDO && analysis.Confidence > 0.6 {
						log.Printf("💫 [LACAN] Desejo latente: %s (confiança: %.0f%%)",
							analysis.LatentDesire, analysis.Confidence*100)
						log.Printf("   → Interpretação: %s", analysis.Interpretation)
					}

					// ✅ Grafo do Desejo (FDPN) - A quem o idoso dirige a demanda
					if fdpnEng != nil {
						ctx := context.Background()
						addressee, _ := fdpnEng.AnalyzeDemandAddressee(ctx, idosoID, userText, string(analysis.LatentDesire))
						if addressee != lacan.ADDRESSEE_UNKNOWN {
							log.Printf("📊 [FDPN] Demanda endereçada a: %s", addressee)
						}
					}
				})
			}

			// ✅ Cognitive Load: Registrar interação e verificar carga
			if s.cognitiveOrchestrator != nil {
				cogOrch := s.cognitiveOrchestrator
				workerpool.BackgroundPool.TrySubmit(func() {
					load := cognitive.InteractionLoad{
						PatientID:           idosoID,
						InteractionType:     "conversation",
						EmotionalIntensity:  0.5,
						CognitiveComplexity: 0.3,
						DurationSeconds:     30,
						TopicsDiscussed:     extractTopics(userText),
					}
					err := cogOrch.RecordInteraction(load)
					if err != nil {
						log.Printf("⚠️ [COGNITIVE] Erro ao registrar interação: %v", err)
						return
					}

					state, _ := cogOrch.GetCurrentState(idosoID)
					if state != nil && state.CurrentLoadScore > 0.7 {
						log.Printf("⚠️ [COGNITIVE] Carga cognitiva alta: %.2f - Redirecionando para temas leves",
							state.CurrentLoadScore)
					}
					if state != nil && state.RuminationDetected {
						log.Printf("🔄 [COGNITIVE] Ruminação detectada no tópico: %s", state.RuminationTopic)
					}
				})
			}

			// ✅ Deep Memory: Detectar evitação, retorno a temas, sintomas corporais
			if s.deepMemory != nil {
				deepMem := s.deepMemory
				workerpool.BackgroundPool.TrySubmit(func() {
					ctx := context.Background()
					now := time.Now()
					deepMem.DetectAvoidance(ctx, idosoID, userText, "current_topic", now)
					deepMem.DetectReturn(ctx, idosoID, userText, now)
					deepMem.DetectBodySymptom(ctx, idosoID, userText, []string{}, now)
					deepMem.DetectSharingDesire(ctx, idosoID, userText, now)
				})
			}

			// ✅ Personality State: Atualizar nível de relacionamento
			if s.personalityService != nil {
				persSvc := s.personalityService
				workerpool.BackgroundPool.TrySubmit(func() {
					ctx := context.Background()
					emotion := detectSimpleEmotion(userText)
					topics := extractTopics(userText)
					persSvc.UpdateAfterConversation(ctx, idosoID, emotion, topics)
				})
			}
		}
	}

	// Capturar transcrição da IA (output audio)
	if audioTrans, ok := serverContent["audioTranscription"].(map[string]interface{}); ok {
		if aiText, ok := audioTrans["text"].(string); ok && aiText != "" {
			log.Printf("💬 [TRANSCRICAO] EVA: %s", aiText)
			go s.saveTranscription(session.IdosoID, "assistant", aiText)

			// ✅ NOVO: Salvar resposta EVA em Postgres + Qdrant + Neo4j
			if s.brainService != nil {
				go s.brainService.SaveEpisodicMemory(session.IdosoID, "assistant", aiText)
			}
		}
	}
	// ========== FIM TRANSCRIÇÃO NATIVA ==========

	// Detectar quando idoso terminou de falar (Turn Complete)
	if turnComplete, ok := serverContent["turnComplete"].(bool); ok && turnComplete {
		log.Printf("🎙️ [TURNO COMPLETO] Iniciando análise de áudio...")

		// ✅ FASE 2.3: Audio Emotion Analysis (Redis Powered)
		if s.audioAnalysis != nil {
			go func(sessID string, uid int64) {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()

				analysisStr, err := s.audioAnalysis.AnalyzeAudioContext(ctx, sessID, uid)
				if err != nil {
					log.Printf("⚠️ [AUDIO] Erro: %v", err)
					return
				}

				if analysisStr != "" {
					log.Printf("👂 [AUDIO] Insight Auditivo Raw: %s", analysisStr)

					// ✅ FASE 4: Critical Dispatcher (Análise de Segurança)
					var result AudioAnalysisResult

					// Tentar limpar blocos de código markdown se houver
					cleanJson := strings.ReplaceAll(analysisStr, "```json", "")
					cleanJson = strings.ReplaceAll(cleanJson, "```", "")

					if err := json.Unmarshal([]byte(cleanJson), &result); err == nil {
						log.Printf("🛡️ [SAFETY] Urgency Level: %s | Emotion: %s", result.Urgency, result.Emotion)

						// 🚨 DETECÇÃO DE RISCO CRÍTICO
						if strings.ToUpper(result.Urgency) == "CRITICA" || strings.ToUpper(result.Urgency) == "ALTA" {
							log.Printf("🚨🚨🚨 ALERTA DE RISCO DETECTADO! DISPARANDO NOTIFICAÇÃO! 🚨🚨🚨")

							alertTitle := "⚠️ ALERTA DE SAÚDE MENTAL"
							alertBody := fmt.Sprintf("Idoso (ID: %d) apresenta sinais de %s com urgência %s. Notas: %s", uid, result.Emotion, result.Urgency, result.Notes)

							// Enviar Push (Assumindo topic 'caregivers' ou token específico do responsável)
							// TODO: Pegar token do responsável. Por enquanto, enviamos para um tópico geral de cuidadores
							// ou se s.pushService suportar SendToTopic.

							// Vou usar um método genérico SendAlert se existir, ou SendNotification
							// Assumindo que o pushService tem suporte basico.
							if s.pushService != nil {
								// HACK: Enviar para o próprio idoso (teste) ou tópico
								// Idealmente: s.db.GetResponsavelToken(uid)
								go s.pushService.SendNotificationToTopic("cuidadores", alertTitle, alertBody, map[string]string{
									"type":     "emergency_alert",
									"idoso_id": fmt.Sprintf("%d", uid),
								})
							}
						}
					} else {
						log.Printf("⚠️ [AUDIO] Falha ao parsear JSON de análise: %v", err)
					}

					// Mesclar ou setar insight pendente (para memória de trabalho)
					session.SetPendingInsight(analysisStr)
				}
			}(session.ID, session.IdosoID)
		}

	}

	// ✅ FASE 4.2: Manipulação de Tools (READ-ONLY)
	if toolCall, ok := serverContent["toolCall"].(map[string]interface{}); ok {
		log.Printf("🛠️ [GEMINI] Recebida solicitação de Tool Use: %+v", toolCall)

		functionCalls, ok := toolCall["functionCalls"].([]interface{})
		if ok && len(functionCalls) > 0 {
			for _, fc := range functionCalls {
				fcMap := fc.(map[string]interface{})
				name := fcMap["name"].(string)
				callID := fcMap["id"].(string) // Importante para responder
				args := fcMap["args"].(map[string]interface{})

				log.Printf("🛠️ [TOOL] Executando: %s (ID: %s)", name, callID)

				// Executar via handler
				var response map[string]interface{}
				if s.tools != nil {
					res, err := s.tools.ExecuteTool(name, args, session.IdosoID)
					if err != nil {
						response = map[string]interface{}{"error": err.Error()}
					} else {
						response = res
					}
				} else {
					response = map[string]interface{}{"error": "Tools handler not initialized"}
				}

				// Enviar resposta de volta para o Gemini
				toolResponse := map[string]interface{}{
					"toolResponse": map[string]interface{}{
						"functionResponses": []interface{}{
							map[string]interface{}{
								"name": name,
								"id":   callID,
								"response": map[string]interface{}{
									"result": response,
								},
							},
						},
					},
				}

				if err := session.GeminiClient.SendMessage(toolResponse); err != nil {
					log.Printf("❌ [TOOL] Erro ao enviar resposta: %v", err)
				} else {
					log.Printf("✅ [TOOL] Resposta enviada para %s", name)
				}
			}
		}
	}

	// ✅ FASE 5: Interruption Handling (Barge-in)
	if interrupted, ok := serverContent["interrupted"].(bool); ok && interrupted {
		log.Printf("🛑 [INTERRUPT] Usuário interrompeu! Enviando comando clear_buffer.")

		// Enviar sinal para o cliente limpar o buffer de áudio imediatamente
		interruptMsg := ControlMessage{
			Type: "clear_buffer",
		}
		if err := session.WSConn.WriteJSON(interruptMsg); err != nil {
			log.Printf("⚠️ Erro ao enviar interrupt: %v", err)
		}

		return // Não processar mais nada deste frame
	}

	// Processar modelTurn (resposta da EVA)
	modelTurn, ok := serverContent["modelTurn"].(map[string]interface{})
	if !ok {
		log.Printf("⚠️ [GEMINI] Sem modelTurn na resposta")
		return
	}

	log.Printf("🤖 [GEMINI] modelTurn encontrado, processando parts...")

	parts, ok := modelTurn["parts"].([]interface{})
	if !ok {
		log.Printf("⚠️ [GEMINI] Sem parts no modelTurn")
		return
	}

	log.Printf("📋 [GEMINI] %d parts para processar", len(parts))

	for i := range parts {
		partMap, ok := parts[i].(map[string]interface{})
		if !ok {
			continue
		}

		// ✅ OTIMIZADO: Processar áudio da EVA com buffer
		if inlineData, ok := partMap["inlineData"].(map[string]interface{}); ok {
			mimeType, _ := inlineData["mimeType"].(string)
			audioB64, _ := inlineData["data"].(string)

			log.Printf("🎵 [GEMINI] Part %d: mimeType=%s, hasAudio=%v", i, mimeType, audioB64 != "")

			if strings.Contains(strings.ToLower(mimeType), "audio/pcm") && audioB64 != "" {
				audioData, err := base64.StdEncoding.DecodeString(audioB64)
				if err != nil {
					log.Printf("❌ [GEMINI] Erro ao decodificar áudio: %v", err)
					continue
				}

				// ✅ NOVO: Validação de tamanho mínimo
				if len(audioData) < 100 {
					log.Printf("⚠️ [AUDIO] Chunk muito pequeno (%d bytes), acumulando no buffer", len(audioData))
					s.bufferAudio(session, audioData)
					continue
				}

				log.Printf("🎶 [AUDIO] Recebido chunk de %d bytes @ 24kHz PCM16", len(audioData))

				// ✅ NOVO: Usar sistema de buffer inteligente
				s.bufferAudio(session, audioData)
			}
		}
	}
}

// ✅ Sistema de buffer inteligente para áudio PCM16
func (s *SignalingServer) bufferAudio(session *WebSocketSession, audioData []byte) {
	session.audioMutex.Lock()
	defer session.audioMutex.Unlock()

	// Acumular no buffer
	session.audioBuffer = append(session.audioBuffer, audioData...)

	// ✅ CRÍTICO: Tamanho mínimo do buffer = 9600 bytes (400ms @ 24kHz PCM16)
	const MIN_BUFFER_SIZE = 9600

	// Enviar quando buffer atingir tamanho mínimo
	if len(session.audioBuffer) >= MIN_BUFFER_SIZE {
		chunk := make([]byte, len(session.audioBuffer))
		copy(chunk, session.audioBuffer)

		log.Printf("🎶 [AUDIO] Enviando %d bytes PCM16 @ 24kHz para cliente", len(chunk))

		err := session.WSConn.WriteMessage(websocket.BinaryMessage, chunk)
		if err != nil {
			log.Printf("❌ [AUDIO] Erro ao enviar: %v", err)
		} else {
			log.Printf("✅ [AUDIO] PCM16 enviado com sucesso")
		}

		// Limpar buffer após envio
		session.audioBuffer = nil
	} else {
		log.Printf("📊 [AUDIO] Buffer acumulando: %d/%d bytes", len(session.audioBuffer), MIN_BUFFER_SIZE)
	}
}

// ✅ NOVA FUNÇÃO: Converte PCM16 (Int16) → Float32
func convertPCM16ToFloat32(pcm16Data []byte) []byte {
	// Validar tamanho (deve ser par)
	if len(pcm16Data)%2 != 0 {
		log.Printf("⚠️ [CONVERSÃO] Tamanho ímpar: %d bytes, truncando", len(pcm16Data))
		pcm16Data = pcm16Data[:len(pcm16Data)-1]
	}

	pcm16Count := len(pcm16Data) / 2
	float32Data := make([]byte, pcm16Count*4)

	// ✅ DEBUG: Analisar primeiros samples
	if pcm16Count > 0 {
		firstSample := int16(binary.LittleEndian.Uint16(pcm16Data[0:2]))
		firstFloat := float32(firstSample) / 32768.0
		log.Printf("🔍 [CONVERSÃO] Primeiro sample: PCM16=%d → Float32=%.6f", firstSample, firstFloat)
	}

	for i := 0; i < pcm16Count; i++ {
		// Decodificar Int16 (Little Endian)
		sample := int16(binary.LittleEndian.Uint16(pcm16Data[i*2:]))

		// Converter para Float32 (-1.0 a +1.0) - Divisão simétrica
		floatVal := float32(sample) / 32768.0

		// Codificar Float32 (Little Endian)
		bits := math.Float32bits(floatVal)
		binary.LittleEndian.PutUint32(float32Data[i*4:], bits)
	}

	log.Printf("✅ [CONVERSÃO] %d samples convertidos (%d bytes PCM16 → %d bytes Float32)",
		pcm16Count, len(pcm16Data), len(float32Data))

	return float32Data
}

// ✅ Enviar buffer restante antes de fechar sessão
func (s *SignalingServer) flushAudioBuffer(session *WebSocketSession) {
	session.audioMutex.Lock()
	defer session.audioMutex.Unlock()

	if len(session.audioBuffer) > 0 {
		log.Printf("🔊 [AUDIO] Enviando buffer restante: %d bytes PCM16", len(session.audioBuffer))
		session.WSConn.WriteMessage(websocket.BinaryMessage, session.audioBuffer)
		session.audioBuffer = nil
	}
}

// 💾 saveTranscription salva a transcrição no banco de forma assíncrona
func (s *SignalingServer) saveTranscription(idosoID int64, role, content string) {
	// Formatar mensagem: [HH:MM:SS] ROLE: content
	timestamp := time.Now().Format("15:04:05")
	roleLabel := "IDOSO"
	if role == "assistant" {
		roleLabel = "EVA"
	}

	formattedMsg := fmt.Sprintf("[%s] %s: %s", timestamp, roleLabel, content)

	// Tentar atualizar registro ativo (últimos 5 minutos)
	updateQuery := `
		UPDATE historico_ligacoes 
		SET transcricao_completa = COALESCE(transcricao_completa, '') || E'\n' || $2
		WHERE id = (
			SELECT id 
			FROM historico_ligacoes
			WHERE idoso_id = $1 
			  AND fim_chamada IS NULL
			  AND inicio_chamada > NOW() - INTERVAL '5 minutes'
			ORDER BY inicio_chamada DESC 
			LIMIT 1
		)
		RETURNING id
	`

	var historyID int64
	err := s.db.QueryRow(updateQuery, idosoID, formattedMsg).Scan(&historyID)

	// Se não existe registro ativo, criar novo
	if err == sql.ErrNoRows {
		insertQuery := `
			INSERT INTO historico_ligacoes (
				agendamento_id, 
				idoso_id, 
				inicio_chamada,
				transcricao_completa
			)
			VALUES (
				(SELECT id FROM agendamentos WHERE idoso_id = $1 AND status IN ('agendado', 'em_andamento') ORDER BY data_hora_agendada DESC LIMIT 1),
				$1,
				CURRENT_TIMESTAMP,
				$2
			)
			RETURNING id
		`

		err = s.db.QueryRow(insertQuery, idosoID, formattedMsg).Scan(&historyID)
		if err != nil {
			log.Printf("⚠️ Erro ao criar histórico: %v", err)
			return
		}
		log.Printf("📝 Novo histórico criado: #%d para idoso %d", historyID, idosoID)
	}

	// 📚 ZETTELKASTEN: Auto-criar zettels de mensagens do usuário
	if role == "user" && s.zettelService != nil && len(content) > 30 {
		go s.createZettelsFromConversation(idosoID, content)
	}
}

// 📚 createZettelsFromConversation extrai conhecimento e cria zettels automaticamente
func (s *SignalingServer) createZettelsFromConversation(idosoID int64, content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	zettels, err := s.zettelService.CreateFromConversation(ctx, idosoID, content)
	if err != nil {
		log.Printf("⚠️ [ZETTEL] Erro ao criar zettels: %v", err)
		return
	}

	if len(zettels) > 0 {
		for _, z := range zettels {
			log.Printf("📚 [ZETTEL] Novo zettel criado: %s (tipo=%s, idoso=%d)", z.Title, z.Type, idosoID)
		}
	}
}

func (s *SignalingServer) createSession(sessionID, cpf string, idosoID int64, nome, voiceName string, conn *websocket.Conn) (*WebSocketSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)

	geminiClient, err := gemini.NewClient(ctx, s.cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	// 🧠 MEMÓRIA & CONTEXTO INTEGRADO (CÉREBRO DIGITAL)
	// Substitui antiga lógica fragmentada pelo UnifiedRetrieval
	log.Printf("🧠 [DEBUG] Chamando GetSystemPrompt para idoso %d", idosoID)
	instructions, err := s.brain.GetSystemPrompt(ctx, idosoID)
	if err != nil {
		log.Printf("❌ [CRÍTICO] GetSystemPrompt falhou: %v", err)
		log.Printf("   Idoso ID: %d", idosoID)
		log.Printf("   Context error: %v", ctx.Err())
		log.Printf("   Brain service: %v", s.brain != nil)

		// REMOVIDO: Fallback para BuildInstructions (código legado com bug)
		// O sistema DEVE usar UnifiedRetrieval. Se falhar, a sessão deve abortar.
		cancel()
		geminiClient.Close()
		return nil, fmt.Errorf("falha ao gerar prompt unificado: %w", err)
	}

	log.Printf("✅ [DEBUG] Contexto Unificado (RSI) gerado com sucesso")
	log.Printf("   - Tamanho: %d chars", len(instructions))

	// Mostrar primeiras 300 chars para debug
	preview := instructions
	if len(preview) > 300 {
		preview = preview[:300] + "..."
	}
	log.Printf("   - Início do prompt: %s", preview)

	// ✅ FASE 4.2: Configurar Tools (DINÂMICO - busca do banco ou fallback para código)
	toolDefs := s.toolDiscovery.GetToolDefinitions(ctx)

	voiceSettings := map[string]interface{}{
		"voiceName": voiceName,
	}

	if err := geminiClient.SendSetup(instructions, voiceSettings, []string{}, "", toolDefs); err != nil {
		cancel()
		geminiClient.Close()
		return nil, err
	}

	session := &WebSocketSession{
		ID:           sessionID,
		CPF:          cpf,
		IdosoID:      idosoID,
		WSConn:       conn,
		GeminiClient: geminiClient,
		ctx:          ctx,
		cancel:       cancel,
		lastActivity: time.Now(),
		audioBuffer:  make([]byte, 0, 19200), // ✅ Pre-alocado: 800ms @ 24kHz
	}

	s.sessions.Store(sessionID, session)

	log.Printf("✅ Sessão criada com buffer de áudio otimizado (24kHz)")

	return session, nil
}

func (s *SignalingServer) cleanupSession(sessionID string) {
	val, ok := s.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}

	session := val.(*WebSocketSession)

	// ✅ NOVO: Enviar buffer restante antes de limpar
	s.flushAudioBuffer(session)

	session.cancel()

	if session.GeminiClient != nil {
		session.GeminiClient.Close()
	}

	// 🧠 ANALISAR CONVERSA AUTOMATICAMENTE
	go s.analyzeAndSaveConversation(session.IdosoID)
}

// analyzeAndSaveConversation analisa a conversa usando dados já no banco
func (s *SignalingServer) analyzeAndSaveConversation(idosoID int64) {
	log.Printf("🔍 [ANÁLISE] Iniciando análise para idoso %d", idosoID)

	// Buscar última transcrição sem fim_chamada
	query := `
		SELECT id, transcricao_completa
		FROM historico_ligacoes
		WHERE idoso_id = $1 
		  AND fim_chamada IS NULL
		  AND transcricao_completa IS NOT NULL
		  AND LENGTH(transcricao_completa) > 50
		ORDER BY inicio_chamada DESC
		LIMIT 1
	`

	var historyID int64
	var transcript string
	err := s.db.QueryRow(query, idosoID).Scan(&historyID, &transcript)
	if err == sql.ErrNoRows {
		log.Printf("⚠️ [ANÁLISE] Nenhuma transcrição encontrada para idoso %d", idosoID)
		return
	}
	if err != nil {
		log.Printf("❌ [ANÁLISE] Erro ao buscar transcrição: %v", err)
		return
	}

	log.Printf("📝 [ANÁLISE] Transcrição: %d caracteres", len(transcript))

	// Mostrar prévia
	preview := transcript
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	log.Printf("📄 [ANÁLISE] Prévia:\n%s", preview)

	log.Printf("🧠 [ANÁLISE] Enviando para Gemini API REST...")

	// Chamar análise do Gemini (REST API)
	analysis, err := gemini.AnalyzeConversation(s.cfg, transcript)
	if err != nil {
		log.Printf("❌ [ANÁLISE] Erro no Gemini: %v", err)
		return
	}

	log.Printf("✅ [ANÁLISE] Análise recebida!")
	log.Printf("   📊 Urgência: %s", analysis.UrgencyLevel)
	log.Printf("   😊 Humor: %s", analysis.MoodState)
	if analysis.ReportedPain {
		log.Printf("   🩺 Dor: %s (intensidade %d/10)", analysis.PainLocation, analysis.PainIntensity)
	}
	if analysis.EmergencySymptoms {
		log.Printf("   🚨 EMERGÊNCIA: %s", analysis.EmergencyType)
	}

	// Converter para JSON
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		log.Printf("❌ [ANÁLISE] Erro ao serializar: %v", err)
		return
	}

	log.Printf("💾 [ANÁLISE] Salvando no banco...")

	// Atualizar banco com análise NOS CAMPOS CORRETOS
	updateQuery := `
		UPDATE historico_ligacoes 
		SET 
			fim_chamada = CURRENT_TIMESTAMP,
			analise_gemini = $2::jsonb,
			urgencia = $3,
			sentimento = $4,
			transcricao_resumo = $5
		WHERE id = $1
	`

	result, err := s.db.Exec(
		updateQuery,
		historyID,
		string(analysisJSON),  // analise_gemini (JSON completo)
		analysis.UrgencyLevel, // urgencia
		analysis.MoodState,    // sentimento
		analysis.Summary,      // transcricao_resumo
	)

	if err != nil {
		log.Printf("❌ [ANÁLISE] Erro ao salvar: %v", err)
		return
	}

	rows, _ := result.RowsAffected()
	log.Printf("✅ [ANÁLISE] Salvo com sucesso! (%d linha atualizada)", rows)

	// 🚨 ALERTA CRÍTICO OU ALTO
	if analysis.UrgencyLevel == "CRITICO" || analysis.UrgencyLevel == "ALTO" {
		log.Printf("🚨 ALERTA DE URGÊNCIA: %s", analysis.UrgencyLevel)
		log.Printf("   Motivo: %s", analysis.RecommendedAction)
		log.Printf("   Preocupações: %v", analysis.KeyConcerns)

		alertMsg := fmt.Sprintf(
			"URGÊNCIA %s: %s. %s",
			analysis.UrgencyLevel,
			strings.Join(analysis.KeyConcerns, ", "),
			analysis.RecommendedAction,
		)

		err := actions.AlertFamily(s.db, s.pushService, s.emailService, idosoID, alertMsg)
		if err != nil {
			log.Printf("❌ [ANÁLISE] Erro ao alertar família: %v", err)
		} else {
			log.Printf("✅ [ANÁLISE] Família alertada com sucesso!")
		}
	}
}

// runCortexAnalysis executa a análise de intenções em paralelo (Bicameral Brain)
func (s *SignalingServer) runCortexAnalysis(session *WebSocketSession, userText string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	log.Printf("🧠 [CORTEX] Analisando intenção: \"%s\"", userText)
	toolCalls, err := s.cortex.AnalyzeTranscription(ctx, userText, "user")
	if err != nil {
		log.Printf("⚠️ [CORTEX] Erro na análise: %v", err)
		return
	}

	if len(toolCalls) == 0 {
		return
	}

	for _, tc := range toolCalls {
		log.Printf("🛠️ [CORTEX] Executando ferramenta: %s", tc.Name)

		var result map[string]interface{}
		var execErr error

		// Executar a tool
		if s.tools != nil {
			result, execErr = s.tools.ExecuteTool(tc.Name, tc.Args, session.IdosoID)
		} else {
			execErr = fmt.Errorf("tools handler not initialized")
		}

		if execErr != nil {
			log.Printf("❌ [CORTEX] Erro ao executar %s: %v", tc.Name, execErr)
			continue
		}

		log.Printf("✅ [CORTEX] Sucesso: %s", tc.Name)

		// FEEDBACK LOOP: Injetar resultado de volta na sessão de VOZ
		// Como o modelo de áudio não suporta ToolResponse nativo no setup atual,
		// injetamos via instrução de contexto oculta.
		resultJSON, _ := json.Marshal(result)
		feedbackPrompt := fmt.Sprintf("\n[SISTEMA: Ação '%s' realizada com sucesso. Resultado: %s]\n", tc.Name, string(resultJSON))

		// Envia como mensagem de sistema/contexto para a IA "saber" que aconteceu
		if err := session.GeminiClient.SendText(feedbackPrompt); err != nil {
			log.Printf("❌ [CORTEX] Erro ao enviar feedback para Voice Session: %v", err)
		} else {
			log.Printf("📡 [CORTEX] Feedback injetado na sessão de voz")
		}
	}
}

func (s *SignalingServer) cleanupDeadSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		var toDelete []string

		s.sessions.Range(func(key, value interface{}) bool {
			sessionID := key.(string)
			session := value.(*WebSocketSession)

			session.mu.RLock()
			inactive := now.Sub(session.lastActivity)
			session.mu.RUnlock()

			if inactive > 30*time.Minute {
				toDelete = append(toDelete, sessionID)
			}

			return true
		})

		for _, sessionID := range toDelete {
			s.cleanupSession(sessionID)
		}
	}
}

func (s *SignalingServer) getIdosoByCPF(cpf string) (*Idoso, error) {
	query := `
		SELECT id, nome, cpf, device_token, ativo, nivel_cognitivo, COALESCE(voice_name, 'Aoede')
		FROM idosos 
		WHERE cpf = $1 AND ativo = true
	`

	var idoso Idoso
	err := s.db.QueryRow(query, cpf).Scan(
		&idoso.ID,
		&idoso.Nome,
		&idoso.CPF,
		&idoso.DeviceToken,
		&idoso.Ativo,
		&idoso.NivelCognitivo,
		&idoso.VoiceName,
	)

	if err != nil {
		return nil, err
	}

	return &idoso, nil
}

func (s *SignalingServer) sendMessage(conn *websocket.Conn, msg ControlMessage) {
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)
}

func (s *SignalingServer) sendError(conn *websocket.Conn, errMsg string) {
	s.sendMessage(conn, ControlMessage{
		Type:    "error",
		Error:   errMsg,
		Success: false,
	})
}

func (s *SignalingServer) BuildInstructions(idosoID int64) string {
	db := s.db
	nomeDefault := "Paciente"
	// 1. QUERY RESILIENTE: Buscar apenas o essencial primeiro
	query := `
		SELECT 
			nome, 
			EXTRACT(YEAR FROM AGE(data_nascimento)) as idade,
			nivel_cognitivo, 
			tom_voz,
			preferencia_horario_ligacao,
			medicamentos_atuais,
			condicoes_medicas,
			endereco
		FROM idosos 
		WHERE id = $1
	`

	// ✅ Campos da Query
	var nome, nivelCognitivo, tomVoz string
	var idade int
	var preferenciaHorario sql.NullString
	var medicamentosAtuais, condicoesMedicas, endereco sql.NullString

	// ✅ Campos fixos para evitar crash/missing
	var mobilidade string = "Não informada"
	var limitacoesVisuais, familiarPrincipal, medicoResponsavel, notasGerais sql.NullString
	var limitacoesAuditivas, usaAparelhoAuditivo, ambienteRuidoso sql.NullBool

	err := db.QueryRow(query, idosoID).Scan(
		&nome,
		&idade,
		&nivelCognitivo,
		&tomVoz,
		&preferenciaHorario,
		&medicamentosAtuais,
		&condicoesMedicas,
		&endereco,
	)

	if err != nil {
		log.Printf("⚠️ [BuildInstructions] Usando dados parciais para %s devido a erro SQL: %v", nomeDefault, err)
		nome = nomeDefault
		idade = 0
		nivelCognitivo = "Não informado"
		tomVoz = "Suave"
	}

	// ✅ NOVO: Buscar medicamentos da tabela RELACIONAL 'medicamentos'
	// Isso sobrescreve/complementa os campos de texto do cadastro do idoso
	medsQuery := `
		SELECT nome, dosagem, horarios, observacoes 
		FROM medicamentos 
		WHERE idoso_id = $1 AND ativo = true
	`
	rows, errMeds := db.Query(medsQuery, idosoID)
	var medsList []string
	if errMeds == nil {
		defer rows.Close()
		// ... resto da logica de medicamentos ...
	}
	if errMeds == nil {
		defer rows.Close()
		for rows.Next() {
			var mNome, mDosagem, mHorarios, mObs string
			if err := rows.Scan(&mNome, &mDosagem, &mHorarios, &mObs); err == nil {
				medInfo := fmt.Sprintf("- %s (%s)", mNome, mDosagem)
				if mHorarios != "" {
					medInfo += fmt.Sprintf(" às %s", mHorarios)
				}
				if mObs != "" {
					medInfo += fmt.Sprintf(". Obs: %s", mObs)
				}
				medsList = append(medsList, medInfo)
			}
		}
	} else {
		log.Printf("⚠️ Erro ao buscar tabela medicamentos: %v", errMeds)
	}

	// ✅ NOVO (AGENDA DO DIA): Buscar agendamentos futuros (próximas 24h)
	agendaQuery := `
		SELECT tipo, data_hora_agendada, dados_tarefa
		FROM agendamentos
		WHERE idoso_id = $1 
		  AND status = 'agendado'
		  AND data_hora_agendada >= NOW()
		ORDER BY data_hora_agendada ASC
	`
	rowsAgenda, errAgenda := db.Query(agendaQuery, idosoID)
	var agendaList []string
	if errAgenda == nil {
		defer rowsAgenda.Close()
		for rowsAgenda.Next() {
			var aTipo string
			var aData time.Time
			var aDadosJSON sql.NullString

			if err := rowsAgenda.Scan(&aTipo, &aData, &aDadosJSON); err == nil {
				// Formatar data e hora: "19/01 às 14:30"
				dataHora := aData.Format("02/01 às 15:04")
				item := fmt.Sprintf("- [%s]: %s", dataHora, strings.Title(aTipo))

				// Se tiver detalhes extras no JSON
				if aDadosJSON.Valid && aDadosJSON.String != "{}" {
					item += fmt.Sprintf(" (%s)", aDadosJSON.String)
				}
				agendaList = append(agendaList, item)
			}
		}
	} else {
		log.Printf("⚠️ Erro ao buscar agenda: %v", errAgenda)
	}

	// ✅ NOVO: Buscar rede de apoio (cuidadores/família/médico)
	redeApoioQuery := `
		SELECT
			c.nome,
			c.telefone,
			c.email,
			c.tipo,
			c.cpf,
			COALESCE(ci.prioridade, 99) as prioridade,
			COALESCE(ci.parentesco, c.tipo) as parentesco
		FROM cuidadores c
		LEFT JOIN cuidador_idoso ci ON c.id = ci.cuidador_id AND ci.idoso_id = $1
		WHERE ci.idoso_id = $1 OR c.tipo IN ('medico', 'responsavel')
		ORDER BY prioridade ASC
	`
	rowsRede, errRede := db.Query(redeApoioQuery, idosoID)
	type ContatoRede struct {
		Nome       string
		Telefone   sql.NullString
		Email      sql.NullString
		Tipo       string
		CPF        sql.NullString
		Prioridade int
		Parentesco string
	}
	var redeApoio []ContatoRede
	if errRede == nil {
		defer rowsRede.Close()
		for rowsRede.Next() {
			var c ContatoRede
			if err := rowsRede.Scan(&c.Nome, &c.Telefone, &c.Email, &c.Tipo, &c.CPF, &c.Prioridade, &c.Parentesco); err == nil {
				redeApoio = append(redeApoio, c)
			}
		}
		log.Printf("📞 [REDE APOIO] %d contatos encontrados para idoso %d", len(redeApoio), idosoID)
	} else {
		log.Printf("⚠️ Erro ao buscar rede de apoio: %v", errRede)
	}

	// 📝 DEBUG EXAUSTIVO DOS DADOS RECUPERADOS
	log.Printf("📋 [DADOS PACIENTE] Nome: %s, Idade: %d", nome, idade)
	log.Printf("   💊 Meds Relacionais: %d encontrados", len(medsList))
	log.Printf("   🥼 Condições: %s", getString(condicoesMedicas, "Nenhuma"))

	// 2. Buscar Persona Ativa e seu Template
	var template string
	var personaCode string = "companion" // Default

	if s.personaManager != nil {
		// Verificar se tem persona ativa, se não, ativar companion como default
		session, _ := s.personaManager.GetCurrentPersona(idosoID)
		if session == nil {
			log.Printf("🎭 [PERSONA] Nenhuma persona ativa, ativando 'companion' como padrão")
			session, _ = s.personaManager.ActivatePersona(idosoID, "companion", "auto_default", "system")
		}

		if session != nil {
			personaCode = session.PersonaCode
		}

		// Buscar template da persona ativa
		personaPrompt, err := s.personaManager.GetSystemInstructions(idosoID)
		if err == nil && personaPrompt != "" {
			template = personaPrompt
			log.Printf("🎭 [PERSONA] Usando persona '%s' para idoso %d", personaCode, idosoID)
		} else {
			log.Printf("⚠️ [PERSONA] Erro ao buscar persona: %v, usando fallback", err)
		}
	}

	// Fallback: Buscar template base se persona não disponível
	if template == "" {
		templateQuery := `SELECT template FROM prompt_templates WHERE nome = 'eva_base_v2' AND ativo = true LIMIT 1`
		if err := db.QueryRow(templateQuery).Scan(&template); err != nil {
			log.Printf("⚠️ Template não encontrado, usando padrão.")
			template = `Você é a EVA, assistente de saúde virtual para {{nome_idoso}}.`
		}
	}

	// 3. Montar "Dossiê do Paciente" (Texto Completo)
	dossier := fmt.Sprintf("\n\n📋 --- FICHA COMPLETA DO PACIENTE (INFORMAÇÃO CONFIDENCIAL) ---\n")
	dossier += fmt.Sprintf("NOME: %s\n", nome)
	dossier += fmt.Sprintf("IDADE: %d anos\n", idade)
	dossier += fmt.Sprintf("ENDEREÇO: %s\n", getString(endereco, "Não completado"))

	dossier += "\n🥼 --- SAÚDE E CONDIÇÕES ---\n"
	dossier += fmt.Sprintf("Nível Cognitivo: %s\n", nivelCognitivo)
	dossier += fmt.Sprintf("Mobilidade: %s\n", mobilidade)
	dossier += fmt.Sprintf("Limitações Auditivas: %v (Usa Aparelho: %v)\n", limitacoesAuditivas, usaAparelhoAuditivo)
	dossier += fmt.Sprintf("Limitações Visuais: %s\n", getString(limitacoesVisuais, "Nenhuma"))
	dossier += fmt.Sprintf("Condições Médicas: %s\n", getString(condicoesMedicas, "Nenhuma registrada"))

	dossier += "\n💊 --- MEDICAMENTOS (FONTE OFICIAL) ---\n"
	if len(medsList) > 0 {
		log.Printf("🛡️ [SAFETY] Monitoramento de interação medicamentosa ativado. Medicamentos verificados: %v", medsList)
		dossier += "O paciente possui os seguintes medicamentos prescritos e ativos no sistema:\n"
		for _, m := range medsList {
			dossier += m + "\n"
		}

		// Fallback visual para os campos legados, caso existam e não estejam na lista (opcional, mas bom para debug)
		oldMeds := getString(medicamentosAtuais, "")
		if oldMeds != "" {
			dossier += fmt.Sprintf("\n(Nota de cadastro antigo: %s)\n", oldMeds)
		}
	} else {
		// Fallback para campos de texto antigos se a tabela relacional estiver vazia
		medsA := getString(medicamentosAtuais, "")
		if medsA == "" {
			dossier += "Nenhum medicamento registrado no sistema.\n"
		} else {
			dossier += fmt.Sprintf("Medicamentos (Legado): %s\n", medsA)
		}
	}
	dossier += "INSTRUÇÃO: Se o paciente perguntar o que deve tomar, consulte EXCLUSIVAMENTE esta lista acima.\n"

	dossier += "\n📅 --- AGENDA COMPLETA (FUTURO) ---\n"
	if len(agendaList) > 0 {
		dossier += "O paciente tem os seguintes compromissos agendados no sistema:\n"
		for _, a := range agendaList {
			dossier += a + "\n"
		}
		dossier += "DICA: Mencione compromissos importantes se forem relevantes para o momento da conversa.\n"
	} else {
		dossier += "Nenhum compromisso agendado no futuro.\n"
	}

	dossier += "\n📞 --- REDE DE APOIO (CONTATOS PARA CHAMADAS) ---\n"
	if len(redeApoio) > 0 {
		dossier += "O paciente tem os seguintes contatos cadastrados que podem ser acionados:\n"
		for _, c := range redeApoio {
			tipoLabel := c.Parentesco
			if tipoLabel == "" {
				tipoLabel = c.Tipo
			}
			telefone := "não informado"
			if c.Telefone.Valid && c.Telefone.String != "" {
				telefone = c.Telefone.String
			}
			cpfInfo := ""
			if c.CPF.Valid && c.CPF.String != "" {
				cpfInfo = fmt.Sprintf(" [CPF: %s]", c.CPF.String)
			}
			dossier += fmt.Sprintf("- %s (%s): Tel %s%s\n", c.Nome, tipoLabel, telefone, cpfInfo)
		}
		dossier += "\nINSTRUÇÃO PARA CHAMADAS:\n"
		dossier += "- Se o idoso pedir para 'ligar para família/filha/filho', use call_family_webrtc\n"
		dossier += "- Se pedir para 'ligar para o médico/doutor', use call_doctor_webrtc\n"
		dossier += "- Se pedir para 'ligar para o cuidador', use call_caregiver_webrtc\n"
		dossier += "- Se for emergência, use call_central_webrtc\n"
	} else {
		dossier += "Nenhum contato de apoio cadastrado.\n"
		dossier += fmt.Sprintf("Familiar (legado): %s\n", getString(familiarPrincipal, "Não informado"))
		dossier += fmt.Sprintf("Médico (legado): %s\n", getString(medicoResponsavel, "Não informado"))
	}

	dossier += "\n📝 --- OUTRAS NOTAS ---\n"
	dossier += fmt.Sprintf("Notas Gerais: %s\n", getString(notasGerais, ""))
	dossier += fmt.Sprintf("Preferência Horário: %s\n", getString(preferenciaHorario, "Indiferente"))
	dossier += fmt.Sprintf("Ambiente Ruidoso: %v\n", ambienteRuidoso)
	dossier += fmt.Sprintf("Tom de Voz Ideal: %s\n", tomVoz)
	dossier += "--------------------------------------------------------\n"

	// 4. Substituições no Template (Suporte a múltiplos estilos)
	// Suporta tanto o estilo manual {{nome_idoso}} quanto o estilo do text/template {{.NomeIdoso}}
	replacements := map[string]string{
		"{{nome_idoso}}":        nome,
		"{{.NomeIdoso}}":        nome,
		"{{idade}}":             fmt.Sprintf("%d", idade),
		"{{.Idade}}":            fmt.Sprintf("%d", idade),
		"{{nivel_cognitivo}}":   nivelCognitivo,
		"{{.NivelCognitivo}}":   nivelCognitivo,
		"{{tom_voz}}":           tomVoz,
		"{{.TomVoz}}":           tomVoz,
		"{{condicoes_medicas}}": getString(condicoesMedicas, ""),
		"{{.CondicoesMedicas}}": getString(condicoesMedicas, ""),
	}

	instructions := template + "\n\n" + dossier
	for old, new := range replacements {
		instructions = strings.ReplaceAll(instructions, old, new)
	}

	// Injeta a lista formatada ou o legado para medicamentos
	medsString := strings.Join(medsList, ", ")
	if medsString == "" {
		medsString = getString(medicamentosAtuais, "Nenhum")
	}
	instructions = strings.ReplaceAll(instructions, "{{medicamentos}}", medsString)
	instructions = strings.ReplaceAll(instructions, "{{.MedicamentosAtuais}}", medsString)

	// Limpar tags condicionais não usadas (estilo Mustache/Template)
	tags := []string{
		"{{#limitacoes_auditivas}}", "{{/limitacoes_auditivas}}",
		"{{#usa_aparelho_auditivo}}", "{{/usa_aparelho_auditivo}}",
		"{{#primeira_interacao}}", "{{/primeira_interacao}}",
		"{{^primeira_interacao}}", "{{taxa_adesao}}",
		"{{.LimitacoesAuditivas}}", "{{.UsaAparelhoAuditivo}}",
	}
	for _, tag := range tags {
		instructions = strings.ReplaceAll(instructions, tag, "")
	}

	// 4.5. 🧠 CONTEXTO DE RELACIONAMENTO/PERSONALIDADE (NOVO)
	personalityContext := getPersonalityContext(idosoID, db)
	if personalityContext != "" {
		instructions += "\n\n" + personalityContext
	}

	// 5. AGENT DELEGATION PROTOCOL (Para Gemini 2.5)
	agentProtocol := `
	
	IMPORTANTE - PROTOCOLO DE FERRAMENTAS:
	Você está rodando em um modelo focado em Áudio e NÃO pode executar ferramentas nativamente.
	Se você precisar realizar uma ação (Pesquisar, Agendar, Ligar) ou buscar informações externas:
	1. Avise o usuário que vai verificar: "Só um momento, vou verificar isso..." ou "Vou agendar para você, um instante...".
	2. Em seguida, GERE IMEDIATAMENTE um comando de texto oculto no formato JSON-in-TEXT:
	   [[TOOL:google_search_retrieval:{"query": "..."}]]
	   [[TOOL:schedule_appointment:{"type": "...", "description": "...", "timestamp": "..."}]]
	   [[TOOL:alert_family:{"reason": "...", "severity": "..."}]]

	NÃO invente dados. Se não souber, use o comando de busca [[TOOL:google_search_retrieval:{"query": "..."}]].
	O sistema irá processar esse comando e te devolver a resposta.
	`

	// 5.5. 🛡️ PROTOCOLO DE SEGURANÇA MÉDICA (NOVO)
	safetyProtocol := fmt.Sprintf(`
	
	🚨 PROTOCOLO DE SEGURANÇA (INTERAÇÃO MEDICAMENTOSA):
	Sempre que o paciente mencionar um novo mal-estar (ex: tontura, dor, náusea) ou perguntar sobre um novo remédio:
	1. Verifique SILENCIOSAMENTE em sua base de conhecimento se há interação perigosa com a lista de "MEDICAMENTOS (FONTE OFICIAL)" mostrada acima.
	2. Se houver qualquer risco, ALERTE IMEDIATAMENTE o paciente de forma calma mas firme.
	3. Recomende que ele NÃO tome nada sem falar com o médico responsável: %s.
	`, getString(medicoResponsavel, "médico cadastrado"))

	// 6. Zeta Story Engine (Gap 2)
	var storySection string
	// Fetch personality state for emotion
	if state, err := s.personalityService.GetState(context.Background(), idosoID); err == nil {
		// Mock profile for now (or fetch from DB if needed)
		profile := &types.IdosoProfile{ID: idosoID, Name: nome}

		if story, directive, err := s.zetaRouter.SelectIntervention(context.Background(), idosoID, state.DominantEmotion, profile); err == nil && story != nil {
			storySection = fmt.Sprintf(`
📚 INTERVENÇÃO NARRATIVA (ZETA ENGINE):
%s
TÍTULO: %s
CONTEÚDO: "%s"
MORAL: %s
INSTRUÇÃO: %s
`, directive, story.Title, story.Content, story.Moral, directive)
		}
	}

	// 7. ANEXAR DOSSIÊ E HISTÓRIA AO FINAL
	finalInstructions := instructions + agentProtocol + safetyProtocol + dossier + storySection

	log.Printf("✅ [BuildInstructions] Instruções finais geradas (%d chars)", len(finalInstructions))
	return finalInstructions
}

// Helper seguro para NullString
func getString(ns sql.NullString, def string) string {
	if ns.Valid {
		return ns.String
	}
	return def
}

func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().Unix())
}

// ✅ Estrutura Envelope Universal (V2 Protocol)
type IncomingMessage struct {
	Type    string `json:"type"`    // "audio", "text", "vision", "ping"
	Payload string `json:"payload"` // Base64 do áudio ou da imagem
}

type ControlMessage struct {
	Type      string      `json:"type"`
	CPF       string      `json:"cpf,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	Success   bool        `json:"success,omitempty"`
	Error     string      `json:"error,omitempty"`
	TargetCPF string      `json:"target_cpf,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
}

type Idoso struct {
	ID             int64
	Nome           string
	CPF            string
	DeviceToken    sql.NullString
	Ativo          bool
	NivelCognitivo string
	VoiceName      string // ✅ NOVO: Preferência de voz
}

// 🧠 GetRecentMemories recupera as últimas conversas para contexto
func (s *SignalingServer) GetRecentMemories(idosoID int64) []string {
	// Limite de 10 conversas ou o que couber (com 1M tokens, 10 é tranquilo)
	query := `
		SELECT inicio_chamada, transcricao_completa, analise_gemini->>'summary' as resumo
		FROM historico_ligacoes
		WHERE idoso_id = $1 
		  AND fim_chamada IS NOT NULL
		  AND transcricao_completa IS NOT NULL
		ORDER BY inicio_chamada DESC
		LIMIT 10
	`

	rows, err := s.db.Query(query, idosoID)
	if err != nil {
		log.Printf("⚠️ Erro ao buscar memórias: %v", err)
		return []string{}
	}
	defer rows.Close()

	var tempMemories []string

	for rows.Next() {
		var inicio time.Time
		var transcricao string
		var resumo sql.NullString

		if err := rows.Scan(&inicio, &transcricao, &resumo); err != nil {
			continue
		}

		// Preferir transcrição completa (Narrativa Completa)
		content := transcricao

		dataStr := inicio.Format("02/01/2006 15:04")
		memoryEntry := fmt.Sprintf("DATA: %s\nCONVERSA:\n%s", dataStr, content)
		tempMemories = append(tempMemories, memoryEntry)
	}

	// Inverter para cronológico (Antigo -> Novo)
	var memories []string
	for i := len(tempMemories) - 1; i >= 0; i-- {
		memories = append(memories, tempMemories[i])
	}

	return memories
}

// ============================================================================
// 🧠 HELPERS PARA ANÁLISE PSICOLÓGICA
// ============================================================================

// extractTopics extrai tópicos principais de um texto (simplificado)
func extractTopics(text string) []string {
	textLower := strings.ToLower(text)
	var topics []string

	// Tópicos comuns de idosos
	topicKeywords := map[string][]string{
		"familia":    {"filh", "net", "esposa", "marido", "familia"},
		"saude":      {"dor", "remedio", "medico", "doença", "hospital", "exame"},
		"saudade":    {"saudade", "falta", "lembr", "morreu", "faleceu"},
		"solidao":    {"sozinho", "solidão", "ninguem", "abandono"},
		"medo":       {"medo", "receio", "preocupa", "ansied"},
		"religiao":   {"deus", "jesus", "oração", "igreja", "fé"},
		"morte":      {"mort", "partir", "fim", "enterr"},
		"memoria":    {"lembro", "antigamente", "passado", "juventude"},
		"alimentacao": {"com", "almoç", "jant", "fome"},
		"sono":       {"dorm", "sono", "insonia", "cansa"},
	}

	for topic, keywords := range topicKeywords {
		for _, kw := range keywords {
			if strings.Contains(textLower, kw) {
				topics = append(topics, topic)
				break
			}
		}
	}

	return topics
}

// detectSimpleEmotion detecta emoção básica do texto (simplificado)
func detectSimpleEmotion(text string) string {
	textLower := strings.ToLower(text)

	emotionPatterns := map[string][]string{
		"triste":    {"triste", "chorand", "choro", "infeliz", "deprimid", "desanimad"},
		"ansioso":   {"ansiosa", "ansioso", "nervos", "preocupad", "agitad"},
		"feliz":     {"feliz", "alegre", "contente", "satisfeit", "bem"},
		"saudade":   {"saudade", "falta", "miss", "queria ver"},
		"medo":      {"medo", "assusta", "apavorad", "preocupad"},
		"raiva":     {"raiva", "irritad", "brav", "revoltad"},
		"solidao":   {"sozinho", "solidão", "abandona", "esquecid"},
		"gratidao":  {"obrigad", "agradeç", "grat"},
		"confuso":   {"confus", "perdid", "não entend", "esqueci"},
		"esperanca": {"esperanç", "vai melhorar", "fé"},
	}

	for emotion, patterns := range emotionPatterns {
		for _, pattern := range patterns {
			if strings.Contains(textLower, pattern) {
				return emotion
			}
		}
	}

	return "neutro"
}
