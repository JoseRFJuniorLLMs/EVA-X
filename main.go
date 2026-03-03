// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eva/internal/brainstem/auth"
	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/brainstem/oauth"
	"eva/internal/brainstem/push"
	"eva/internal/cortex/alert"
	"eva/internal/cortex/consciousness"
	"eva/internal/cortex/eva_memory"
	"eva/internal/cortex/gemini"
	"eva/internal/cortex/lacan"
	"eva/internal/cortex/learning"
	"eva/internal/cortex/llm"
	"eva/internal/cortex/personality"
	"eva/internal/cortex/ram"
	evaSelf "eva/internal/cortex/self"
	"eva/internal/cortex/selfawareness"
	"eva/internal/cortex/situation"
	"eva/internal/cortex/skills"
	"eva/internal/cortex/voice/speaker"
	"eva/internal/hippocampus/habits"
	"eva/internal/hippocampus/knowledge"
	"eva/internal/hippocampus/memory"
	"eva/internal/hippocampus/memory/superhuman"
	"eva/internal/hippocampus/spaced"
	"eva/internal/integration"
	"eva/internal/mcp"
	internalmemory "eva/internal/memory"
	"eva/internal/memory/krylov"
	memscheduler "eva/internal/memory/scheduler"
	"eva/internal/monitoring"
	"eva/internal/motor/actions"
	"eva/internal/motor/browser"
	"eva/internal/motor/cron"
	"eva/internal/motor/email"
	"eva/internal/motor/filesystem"
	gmailpkg "eva/internal/motor/gmail"
	"eva/internal/motor/messaging"
	"eva/internal/motor/sandbox"
	"eva/internal/motor/selfcode"
	"eva/internal/motor/smarthome"
	"eva/internal/motor/telegram"
	"eva/internal/motor/webhooks"
	"eva/internal/research"
	"eva/internal/scheduler"
	"eva/internal/security"
	"eva/internal/swarm"
	"eva/internal/swarm/clinical"
	"eva/internal/swarm/educator"
	"eva/internal/swarm/emergency"
	"eva/internal/swarm/entertainment"
	"eva/internal/swarm/external"
	swarmgoogle "eva/internal/swarm/google"
	"eva/internal/swarm/kids"
	"eva/internal/swarm/legal"
	"eva/internal/swarm/productivity"
	"eva/internal/swarm/scholar"
	swarmself "eva/internal/swarm/selfawareness"
	"eva/internal/swarm/wellness"
	"eva/internal/telemetry"
	"eva/internal/tools"
	"eva/internal/voice"
	"eva/pkg/crypto"
	nietzsche "nietzsche-sdk"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

type PCMClient struct {
	CPF     string
	IdosoID int64
}

// --- RAM Engine Adapters (bridge between existing services and ram interfaces) ---

type ramLLMAdapter struct{ svc *llm.Service }

func (a *ramLLMAdapter) GenerateText(ctx context.Context, prompt string, temperature float64) (string, error) {
	resp, err := a.svc.Ask("claude", prompt)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

func (a *ramLLMAdapter) GenerateMultiple(ctx context.Context, prompt string, n int, temperature float64) ([]string, error) {
	results := make([]string, 0, n)
	for i := 0; i < n; i++ {
		text, err := a.GenerateText(ctx, prompt, temperature)
		if err != nil {
			return nil, err
		}
		results = append(results, text)
	}
	return results, nil
}

type ramEmbedAdapter struct{ svc *knowledge.EmbeddingService }

func (a *ramEmbedAdapter) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return a.svc.GenerateEmbedding(ctx, text)
}

type ramRetrievalAdapter struct{ store *memory.MemoryStore }

func (a *ramRetrievalAdapter) RetrieveRelevant(ctx context.Context, patientID int64, query string, k int) ([]ram.Memory, error) {
	memories, err := a.store.GetRecent(ctx, patientID, k)
	if err != nil {
		return nil, err
	}
	result := make([]ram.Memory, len(memories))
	for i, m := range memories {
		result[i] = ram.Memory{
			ID:        m.ID,
			Content:   m.Content,
			Timestamp: m.Timestamp,
			Score:     m.Importance,
		}
	}
	return result, nil
}

type SignalingServer struct {
	db                 *database.DB
	cfg                *config.Config
	alertService       *voice.AlertService
	geminiHandler      *gemini.Handler
	voiceHandler       *voice.Handler
	pushService        *push.FirebaseService
	memoryStore        *memory.MemoryStore
	personalityService *personality.PersonalityService
	narrativeShift     *lacan.NarrativeShiftDetector
	vsm                *VideoSessionManager
	evaMemory          *eva_memory.EvaMemory
	wisdomService      *knowledge.WisdomService
	fdpnEngine         *lacan.FDPNEngine
	habitTracker       *habits.HabitTracker
	spacedRepetition   *spaced.SpacedRepetitionService
	superhumanMemory   *superhuman.SuperhumanMemoryService
	toolsHandler       *tools.ToolsHandler
	toolsClient        *gemini.ToolsClient
	swarmOrchestrator  *swarm.Orchestrator
	autonomousLearner  *learning.AutonomousLearner
	speakerSvc         *speaker.SpeakerService
	coreMemory         *evaSelf.CoreMemoryEngine
	globalWorkspace    *consciousness.GlobalWorkspace
	situationMod       *situation.SituationalModulator
	energyFeeder       *situation.EnergyFeeder
	ramEngine          *ram.RAMEngine
	gmailWatcher       *gmailpkg.Watcher
}

func main() {
	// 1. Configuração e Logger
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Falha ao carregar configurações")
	}
	logger := telemetry.NewLogger(cfg.Environment)

	// 1.1 LGPD Art. 46: initialize field-level encryption
	if err := crypto.Init(); err != nil {
		log.Warn().Err(err).Msg("Crypto init — encryption disabled, sensitive data stored in plaintext")
	} else {
		log.Info().Msg("LGPD Art. 46: AES-256-GCM field encryption enabled")
	}

	// 2. Infraestrutura (DBs)
	// NietzscheDB — single unified database (graph + vector + cache + relational)
	nzClient, err := nietzscheInfra.NewClient(cfg.NietzscheGRPCAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("NietzscheDB indisponivel — banco unico obrigatorio")
	}
	defer nzClient.Close()

	// Ensure all EVA collections exist (idempotent)
	if err := nzClient.EnsureCollections(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("Falha ao criar collections no NietzscheDB")
	}

	// Legacy PostgreSQL connection REMOVED — all data lives in NietzscheDB.
	// Modules that still reference db.Conn receive nil and must handle it gracefully.
	var legacyConn *sql.DB

	// Primary DB: NietzscheDB with optional NietzscheDB fallback
	db := database.NewNietzscheDB(nzClient.SDK(), legacyConn)
	defer db.Close()

	// Ensure indexes for eva_mind collection
	if err := db.EnsureIndexes(context.Background()); err != nil {
		log.Warn().Err(err).Msg("Failed to create eva_mind indexes (non-fatal)")
	}

	// Create adapters
	graphAdapter := nietzscheInfra.NewGraphAdapter(nzClient, "patient_graph")
	vectorAdapter := nietzscheInfra.NewVectorAdapter(nzClient)
	// audioBuffer and algoAdapter removed — created but never used (D1)
	evaGraphAdapter := nietzscheInfra.NewGraphAdapter(nzClient, "eva_core")
	manifoldAdapter := nietzscheInfra.NewManifoldAdapter(nzClient)
	backupService := nietzscheInfra.NewBackupService(nzClient, 24*time.Hour)
	cdcListener := nietzscheInfra.NewCDCListener(nzClient)
	cdcBridge := nietzscheInfra.NewWebSocketBridge(cdcListener, nzClient)
	cacheAdapter := nietzscheInfra.NewCacheAdapter(nzClient, cfg.NietzscheCacheCollection)
	log.Info().Str("collection", cfg.NietzscheCacheCollection).Msg("NietzscheDB CacheAdapter created")

	// NietzscheDB Security & Wiederkehr
	securityAdapter := nietzscheInfra.NewSecurityAdapter(nietzscheInfra.SecurityConfig{
		EncryptionKey: cfg.NIETZSCHE_ENCRYPTION_KEY,
		RBACEnabled:   cfg.NIETZSCHE_RBAC_ENABLED,
		APIKeyAdmin:   cfg.NIETZSCHE_API_KEY_ADMIN,
		APIKeyWriter:  cfg.NIETZSCHE_API_KEY_WRITER,
		APIKeyReader:  cfg.NIETZSCHE_API_KEY_READER,
	})
	if err := securityAdapter.ValidateEncryptionForPHI(); err != nil {
		log.Warn().Err(err).Msg("Security warning: compliance issues detected")
	}

	wiederkehrAdapter := nietzscheInfra.NewWiederkehrAdapter(nzClient)

	// Register default daemons (Sprint 11.2)
	go func() {
		time.Sleep(10 * time.Second) // Wait for server to be fully ready
		ctx := context.Background()
		// Energy Guard: prevent hotspots
		_ = wiederkehrAdapter.CreateEnergyGuard(ctx, "patient_graph", 0.85, "1h")
		// Decay Reaper: implement natural forgetting
		_ = wiederkehrAdapter.CreateDecayReaper(ctx, "patient_graph", 0.05, "24h")
		log.Info().Msg("NietzscheDB Wiederkehr daemons registered (EnergyGuard, DecayReaper)")
	}()

	// 3. Serviços Base
	pushService, err := push.NewFirebaseService(cfg.FirebaseCredentialsPath)
	if err != nil {
		log.Error().Err(err).Msg("Firebase indisponível - Push notifications desabilitadas. Alertas de emergência podem falhar!")
	}
	alertService := voice.NewAlertService(db, cfg, logger)

	// 4. Cortex (Logica de Negocio e IA)
	geminiHandler := gemini.NewHandler(cfg, db, graphAdapter, vectorAdapter)

	// 5. Voice Handler (WebSocket & DSP)
	voiceHandler := voice.NewHandler(db, cfg, logger, alertService, geminiHandler)
	voice.InitSessionManager(logger)

	// 6. Memory & Personality Stores
	graphStore := memory.NewGraphStore(graphAdapter, cfg)
	log.Info().Msg("GraphStore conectado ao NietzscheDB (patient_graph)")
	memoryStore := memory.NewMemoryStore(db.Conn, graphStore, vectorAdapter)
	personalitySvc := personality.NewPersonalityService(evaGraphAdapter)

	// 6.2 Wisdom Service (busca semantica em colecoes de sabedoria)
	var wisdomSvc *knowledge.WisdomService
	embedSvc, embedErr := knowledge.NewEmbeddingService(cfg, vectorAdapter)
	if embedErr != nil {
		log.Warn().Err(embedErr).Msg("EmbeddingService indisponivel - Wisdom desabilitada")
	} else {
		// Wire NietzscheDB CacheAdapter into embedding caches
		embedSvc.SetCacheAdapter(cacheAdapter)
		wisdomSvc = knowledge.NewWisdomService(vectorAdapter, embedSvc)
		log.Info().Msg("Wisdom Service inicializado")
	}

	// 6.1 EVA Meta-Cognitive Memory (NietzscheDB patient_graph)
	evaMemSvc := eva_memory.New(graphAdapter)
	log.Info().Msg("EVA Meta-Cognitive Memory inicializada (NietzscheDB)")

	// 6.3 Core Memory Engine — memoria pessoal da EVA (NietzscheDB eva_core)
	var coreMemoryEngine *evaSelf.CoreMemoryEngine
	reflectionSvc, reflectErr := evaSelf.NewReflectionService(cfg.GoogleAPIKey, cfg.GeminiAnalysisModel)
	if reflectErr != nil {
		log.Warn().Err(reflectErr).Msg("ReflectionService indisponivel - CoreMemory desabilitado")
	} else {
		anonSvc, anonErr := evaSelf.NewAnonymizationService(evaSelf.AnonymizationConfig{
			GeminiAPIKey:      cfg.GoogleAPIKey,
			ModelName:         cfg.GeminiAnalysisModel,
			UseRegexFilters:   true,
			PreserveStructure: true,
		})
		if anonErr != nil {
			log.Warn().Err(anonErr).Msg("AnonymizationService indisponivel - CoreMemory desabilitado")
		} else {
			coreMemoryEngine, err = evaSelf.NewCoreMemoryEngine(evaSelf.CoreMemoryConfig{
				GraphAdapter:        evaGraphAdapter,
				SimilarityThreshold: 0.88,
				MinOccurrences:      3,
			}, reflectionSvc, anonSvc, nil)
			if err != nil {
				log.Warn().Err(err).Msg("CoreMemoryEngine indisponivel - identidade EVA sem persistencia")
				coreMemoryEngine = nil
			} else {
				defer coreMemoryEngine.Shutdown(context.Background())
				log.Info().Msg("CoreMemoryEngine inicializado — EVA tem memoria propria (NietzscheDB eva_core)")
			}
		}
	}

	// 7. Cognitive Services
	signifierService := lacan.NewSignifierService(graphAdapter)
	narrativeShiftDetector := lacan.NewNarrativeShiftDetector(graphAdapter, signifierService)
	log.Info().Msg("Narrative Shift Detector initialized")

	// 7.1 FDPN Engine (Lacan demand address mapping)
	fdpnEng := lacan.NewFDPNEngine(graphAdapter)
	log.Info().Msg("FDPN Engine inicializado")

	// 7.2 Habit Tracker + Spaced Repetition
	habitTracker := habits.NewHabitTracker(db.Conn)
	spacedSvc := spaced.NewSpacedRepetitionService(db.Conn)
	log.Info().Msg("📊 Habit Tracker + Spaced Repetition inicializados")

	// 7.3 Superhuman Memory Service (12 subsistemas de memoria)
	superhumanSvc := superhuman.NewSuperhumanMemoryService(db.Conn)
	log.Info().Msg("🌟 Superhuman Memory Service inicializado")

	// 7.4 Email Service (SMTP para alertas)
	emailSvc, err := email.NewEmailService(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("EmailService indisponivel - alertas por email desabilitados")
	} else {
		log.Info().Msg("📧 Email Service inicializado")
	}

	// 7.5 Tools Handler (120+ tools — medicamentos, alarmes, jogos, GTD, etc)
	toolsHandler := tools.NewToolsHandler(db, pushService, emailSvc)
	toolsHandler.SetNietzscheClient(nzClient)
	toolsHandler.SetManifoldAdapter(manifoldAdapter)
	if embedSvc != nil {
		toolsHandler.SetEmbedFunc(embedSvc.GenerateEmbedding)
	}
	toolsHandler.SetSpacedService(spacedSvc)
	toolsHandler.SetHabitTracker(habitTracker)
	toolsHandler.SetSecurityAdapter(securityAdapter)
	toolsHandler.SetWiederkehrAdapter(wiederkehrAdapter)

	// 🔒 Novas ferramentas (Gmail, YouTube, Filesystem, SelfCode, etc) só em debug
	toolsHandler.SetDebugMode(cfg.Environment == "development")
	log.Info().Str("environment", cfg.Environment).Bool("debug_tools", cfg.Environment == "development").Msg("🔒 Debug mode para novas ferramentas")

	// EscalationService (escalation de alertas: push → email → SMS)
	escalationSvc := alert.NewEscalationService(alert.EscalationConfig{
		Firebase: pushService,
		Email:    emailSvc,
		DB:       db.Conn,
	})
	toolsHandler.SetEscalationService(escalationSvc)

	// ✅ OAuth Service (Google APIs: Gmail, YouTube, Calendar, Drive)
	oauthSvc := oauth.NewService(
		cfg.GoogleOAuthClientID,
		cfg.GoogleOAuthClientSecret,
		cfg.GoogleOAuthRedirectURL,
		cfg.OAuthStateSecret,
	)
	toolsHandler.SetOAuthService(oauthSvc)

	// ✅ WhatsApp (Meta Graph API)
	if cfg.WhatsAppAccessToken != "" {
		toolsHandler.SetWhatsAppConfig(cfg.WhatsAppAccessToken, cfg.WhatsAppPhoneNumberID)
		log.Info().Msg("💬 WhatsApp Meta API configurado")
	}

	// ✅ Telegram Bot (envio + recepção de mensagens)
	var telegramSvc *telegram.Service
	if cfg.TelegramBotToken != "" {
		telegramSvc = telegram.NewService(cfg.TelegramBotToken)
		toolsHandler.SetTelegramService(telegramSvc)
		log.Info().Msg("📱 Telegram Bot configurado")
	}

	// ✅ Filesystem Access (sandboxed)
	fsSvc := filesystem.NewService(cfg.EVAWorkspaceDir)
	toolsHandler.SetFilesystemService(fsSvc)
	log.Info().Msgf("📂 Filesystem Service: %s", cfg.EVAWorkspaceDir)

	// ✅ Self-Coding (OpenClaw-style)
	selfcodeSvc := selfcode.NewService(cfg.EVAProjectDir)
	toolsHandler.SetSelfCodeService(selfcodeSvc)
	log.Info().Msgf("💻 Self-Code Service: %s", cfg.EVAProjectDir)

	// ✅ Google Maps API Key
	if cfg.GoogleMapsAPIKey != "" {
		toolsHandler.SetMapsAPIKey(cfg.GoogleMapsAPIKey)
		log.Info().Msg("📍 Google Maps API configurado")
	}

	// ✅ Sandbox (Code Execution — bash, python, node)
	sandboxSvc := sandbox.NewService(cfg.SandboxDir)
	toolsHandler.SetSandboxService(sandboxSvc)
	log.Info().Msgf("🖥️ Sandbox Service: %s", cfg.SandboxDir)

	// ✅ Browser Automation
	browserSvc := browser.NewService()
	toolsHandler.SetBrowserService(browserSvc)
	log.Info().Msg("🌐 Browser Service inicializado")

	// ✅ Cron / Scheduled Tasks
	cronSvc := cron.NewService()
	cronSvc.SetExecutor(func(toolName string, args map[string]interface{}, idosoID int64) (map[string]interface{}, error) {
		return toolsHandler.ExecuteTool(toolName, args, idosoID)
	})
	cronSvc.Start()
	toolsHandler.SetCronService(cronSvc)
	log.Info().Msg("⏰ Cron Service iniciado")

	// ✅ Multi-LLM (Claude, GPT, DeepSeek)
	llmSvc := llm.NewService()
	if cfg.ClaudeAPIKey != "" {
		llmSvc.AddProvider("claude", cfg.ClaudeAPIKey, "https://api.anthropic.com", "claude-sonnet-4-6")
		log.Info().Msg("🤖 LLM Provider: Claude configurado")
	}
	if cfg.OpenAIAPIKey != "" {
		llmSvc.AddProvider("gpt", cfg.OpenAIAPIKey, "https://api.openai.com", "gpt-4o")
		log.Info().Msg("🤖 LLM Provider: GPT configurado")
	}
	if cfg.DeepSeekAPIKey != "" {
		llmSvc.AddProvider("deepseek", cfg.DeepSeekAPIKey, "https://api.deepseek.com", "deepseek-chat")
		log.Info().Msg("🤖 LLM Provider: DeepSeek configurado")
	}
	toolsHandler.SetLLMService(llmSvc)

	// ✅ Messaging Channels
	if cfg.SlackBotToken != "" {
		toolsHandler.SetSlackService(messaging.NewSlackService(cfg.SlackBotToken))
		log.Info().Msg("💬 Slack configurado")
	}
	if cfg.DiscordBotToken != "" {
		toolsHandler.SetDiscordService(messaging.NewDiscordService(cfg.DiscordBotToken))
		log.Info().Msg("💬 Discord configurado")
	}
	if cfg.TeamsWebhookURL != "" {
		toolsHandler.SetTeamsService(messaging.NewTeamsService(cfg.TeamsWebhookURL))
		log.Info().Msg("💬 Teams configurado")
	}
	if cfg.SignalSenderNum != "" {
		toolsHandler.SetSignalService(messaging.NewSignalService(cfg.SignalCLIPath, cfg.SignalSenderNum))
		log.Info().Msg("💬 Signal configurado")
	}

	// ✅ Smart Home (Home Assistant)
	if cfg.HomeAssistantToken != "" {
		smartHomeSvc := smarthome.NewService(cfg.HomeAssistantURL, cfg.HomeAssistantToken)
		toolsHandler.SetSmartHomeService(smartHomeSvc)
		log.Info().Msgf("🏠 Smart Home: %s", cfg.HomeAssistantURL)
	}

	// ✅ Webhooks
	webhookSvc := webhooks.NewService()
	toolsHandler.SetWebhookService(webhookSvc)
	log.Info().Msg("🔗 Webhook Service inicializado")

	// ✅ Skills (Self-Improving Runtime)
	skillsSvc := skills.NewService(cfg.SkillsDir)
	skillsSvc.SetRunner(func(ctx context.Context, language, code string, timeout time.Duration) (string, int, error) {
		result, err := sandboxSvc.Execute(ctx, language, code, timeout)
		if err != nil {
			return "", 1, err
		}
		return result.Output, result.ExitCode, nil
	})
	toolsHandler.SetSkillsService(skillsSvc)
	log.Info().Msgf("🧩 Skills Service: %s (%d skills carregadas)", cfg.SkillsDir, len(skillsSvc.List()))

	// NietzscheDB eva_core collection (gRPC :50051)
	toolsHandler.SetEvaCoreAdapter(evaGraphAdapter)
	log.Info().Msg("Tools Handler inicializado (150+ tools, NietzscheDB)")

	// 7.6 Tools Client (Gemini 2.5 Flash REST — deteccao de intencao de tool)
	toolsClient := gemini.NewToolsClient(cfg)
	log.Info().Msg("🔍 Tools Client inicializado (Gemini Flash)")

	// 7.7 Autonomous Learner (aprendizagem autonoma — pesquisa, estuda e memoriza)
	autonomousLearner := learning.NewAutonomousLearner(db.Conn, cfg, vectorAdapter, embedSvc)
	toolsHandler.SetAutonomousLearner(func(ctx context.Context, topic string) (interface{}, error) {
		return autonomousLearner.StudyTopic(ctx, topic)
	})
	log.Info().Msg("Autonomous Learner inicializado")

	// 7.8 Self-Awareness Service (introspecao — codigo, bancos, memorias)
	selfAwareSvc := selfawareness.NewSelfAwarenessService(db.Conn, vectorAdapter, embedSvc, cfg)
	selfAwareAgent := swarmself.New()
	selfAwareAgent.SetService(selfAwareSvc)
	log.Info().Msg("Self-Awareness Service inicializado")

	// 7.8.1 Krylov Subspace Memory (1536D → 64D compression via modified Gram-Schmidt)
	krylovMgr := krylov.NewKrylovMemoryManager(1536, 64, 1000)
	log.Info().Msg("🔢 Krylov Memory Manager inicializado (1536D → 64D)")

	// 7.9 Swarm Orchestrator (12 agentes especializados + circuit breaker)
	scholarAgent := scholar.New()
	scholarAgent.SetLearner(autonomousLearner)

	swarmDeps := &swarm.Dependencies{
		DB:           db,
		Nietzsche:    nzClient,
		Graph:        graphAdapter,
		Vector:       vectorAdapter,
		Push:         pushService,
		Config:       cfg,
		GoogleAPIKey: cfg.GoogleAPIKey,
		Krylov:       krylovMgr,
		AlertFamily: func(ctx context.Context, userID int64, reason, severity string) error {
			return actions.AlertFamilyWithSeverity(db.Conn, pushService, emailSvc, userID, reason, severity)
		},
	}
	orchestrator := swarm.NewOrchestrator(swarmDeps)
	if err := swarm.SetupAllSwarms(orchestrator,
		clinical.New(),
		emergency.New(),
		entertainment.New(),
		wellness.New(),
		productivity.New(),
		swarmgoogle.New(),
		external.New(),
		educator.New(),
		kids.New(),
		legal.New(),
		scholarAgent,
		selfAwareAgent,
	); err != nil {
		log.Error().Err(err).Msg("Falha ao inicializar Swarm System")
	}

	// Bridge ToolsHandler -> Swarm Orchestrator (tools sem case no switch: open_camera_analysis, change_voice, etc)
	toolsHandler.SetSwarmRouter(orchestrator)
	log.Info().Msg("🐝 Swarm bridge configurado no ToolsHandler")

	// 7.10 Speaker Recognition Service (Voice Fingerprinting + Timbre Analysis)
	speakerSvc, err := speaker.NewSpeakerService(db, vectorAdapter, cfg.SpeakerModelPath)
	if err != nil {
		log.Warn().Err(err).Msg("Speaker service unavailable - voice fingerprinting disabled")
	} else {
		log.Info().Msg("Speaker Recognition Service initialized")
	}

	// 7.11 Global Workspace (Baars' Cognitive Theory of Consciousness)
	globalWS := consciousness.NewGlobalWorkspace()
	globalWS.RegisterModule(&consciousness.LacanModule{})
	globalWS.RegisterModule(&consciousness.PersonalityModule{})
	globalWS.RegisterModule(&consciousness.EthicsModule{})
	log.Info().Msg("🧠 Global Workspace inicializado (3 modulos cognitivos)")

	// 7.12 Situational Modulator (detecta contexto e modula pesos de personalidade)
	situationMod := situation.NewModulator(nil, nil)
	energyFeeder := situation.NewEnergyFeeder(nzClient)
	log.Info().Msg("🎭 Situational Modulator + Energy Feeder inicializados")

	// 7.13 RAM Engine (Realistic Accuracy Model — interpretacoes + validacao historica)
	var ramEng *ram.RAMEngine
	if llmSvc != nil && embedSvc != nil {
		interpGen := ram.NewInterpretationGenerator(
			&ramLLMAdapter{svc: llmSvc},
			&ramEmbedAdapter{svc: embedSvc},
			&ramRetrievalAdapter{store: memoryStore},
		)
		histValidator := ram.NewHistoricalValidator(
			&ramRetrievalAdapter{store: memoryStore},
			&ramEmbedAdapter{svc: embedSvc},
			nil, // GraphStore — será wired quando NER estiver completo
		)
		ramEng = ram.NewRAMEngine(interpGen, histValidator, nil, nil)
		log.Info().Msg("🎯 RAM Engine inicializado (interpretacoes + validacao historica)")
	} else {
		log.Warn().Msg("⚠️ RAM Engine desabilitado (LLM ou EmbeddingService indisponivel)")
	}

	// 7.14 Memory Orchestrator (Voice -> FDPN -> Krylov -> Spectral -> REM consolidation)
	hippoFDPN := memory.NewFDPNEngine(graphAdapter, nil)
	memOrchestrator := internalmemory.NewMemoryOrchestrator(db.Conn, graphAdapter, vectorAdapter, hippoFDPN, krylovMgr)
	log.Info().Msg("Memory Orchestrator inicializado (FDPN -> Krylov -> REM)")

	// 7.15 Krylov HTTP Bridge (porta 50052 — bridge HTTP/JSON para compressao vetorial)
	krylovBridge := internalmemory.NewKrylovHTTPBridge(krylovMgr, 50052)
	krylovBridge.StartAsync()
	log.Info().Msg("🔌 Krylov HTTP Bridge iniciado na porta 50052")

	// 7.16 Research Engine (pesquisa clinica longitudinal com anonimizacao LGPD)
	researchEng := research.NewResearchEngine(db.Conn)
	log.Info().Msg("🔬 Research Engine inicializado")

	// 8. SignalingServer
	server := &SignalingServer{
		db:                 db,
		cfg:                cfg,
		alertService:       alertService,
		geminiHandler:      geminiHandler,
		voiceHandler:       voiceHandler,
		pushService:        pushService,
		memoryStore:        memoryStore,
		personalityService: personalitySvc,
		narrativeShift:     narrativeShiftDetector,
		vsm:                NewVideoSessionManager(),
		evaMemory:          evaMemSvc,
		wisdomService:      wisdomSvc,
		fdpnEngine:         fdpnEng,
		habitTracker:       habitTracker,
		spacedRepetition:   spacedSvc,
		superhumanMemory:   superhumanSvc,
		toolsHandler:       toolsHandler,
		toolsClient:        toolsClient,
		swarmOrchestrator:  orchestrator,
		autonomousLearner:  autonomousLearner,
		speakerSvc:         speakerSvc,
		coreMemory:         coreMemoryEngine,
		globalWorkspace:    globalWS,
		situationMod:       situationMod,
		energyFeeder:       energyFeeder,
		ramEngine:          ramEng,
	}

	// Gmail Watcher — DISABLED temporarily (Google OAuth requires valid HTTPS + domain)
	// TODO: Re-enable when domain + Let's Encrypt is configured
	// server.gmailWatcher = gmailpkg.NewWatcher(
	// 	2*time.Minute,
	// 	func(idosoID int64) (string, error) {
	// 		return toolsHandler.GetGoogleAccessToken(idosoID)
	// 	},
	// 	func(idosoID int64, msgType string, payload interface{}) {
	// 		toolsHandler.NotifyBrowser(idosoID, msgType, payload)
	// 	},
	// )
	// log.Info().Msg("Gmail Watcher configurado (poll cada 2 min)")

	// 9. Router & Servidor HTTP
	router := mux.NewRouter()

	// Middleware
	router.Use(security.CORSMiddleware(security.DefaultCORSConfig()))

	// Rotas WebSocket
	router.HandleFunc("/ws/pcm", server.voiceHandler.HandleMediaStream)
	router.HandleFunc("/ws/browser", server.handleBrowserVoice)
	router.HandleFunc("/ws/eva", server.handleEvaChat)
	router.HandleFunc("/ws/logs", server.handleLogStream)
	router.HandleFunc("/ws/perspektive", func(w http.ResponseWriter, r *http.Request) {
		handlePerspektiveWS(w, r, cdcBridge)
	})
	// Rota legado para Twilio Media Stream
	router.HandleFunc("/calls/stream/{agendamento_id}", server.voiceHandler.HandleMediaStream)

	// Rotas de Vídeo (Helpers)
	router.HandleFunc("/video/ws", HandleVideoWebSocket(server.vsm))
	router.HandleFunc("/video/create", server.handleCreateVideoSession).Methods("POST")
	router.HandleFunc("/video/candidate", server.handleCreateVideoCandidate).Methods("POST")
	router.HandleFunc("/video/session/{id}", server.handleGetVideoSession).Methods("GET")
	router.HandleFunc("/video/session/{id}/answer", server.handleSaveVideoAnswer).Methods("POST")
	router.HandleFunc("/video/session/{id}/answer/poll", server.handleGetVideoAnswer).Methods("GET")
	router.HandleFunc("/video/candidates/{id}", server.handleGetMobileCandidates).Methods("GET")
	router.HandleFunc("/video/pending", server.handleGetPendingSessions).Methods("GET")

	// API Routes
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Chat (Malaria-Angolar integration)
	api.HandleFunc("/chat", server.handleChat).Methods("POST")

	// Auth
	authHandler := auth.NewHandler(db, cfg)
	api.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	// Mobile API (EVA-Mobile) — protected by JWT auth
	v1 := api.PathPrefix("/v1").Subrouter()
	v1.Use(auth.AuthMiddleware(cfg.JWTSecret))
	v1.HandleFunc("/idosos/by-cpf/{cpf}", server.handleGetIdosoByCpf).Methods("GET")
	v1.HandleFunc("/idosos/{id}", server.handleGetIdoso).Methods("GET")
	v1.HandleFunc("/idosos/sync-token-by-cpf", server.handleSyncTokenByCpf).Methods("PATCH")

	// OAuth routes — DISABLED temporarily (Google OAuth requires valid HTTPS + domain)
	// TODO: Re-enable when domain + Let's Encrypt is configured
	// oauthHandler := oauth.NewHandler(oauthSvc, db, cfg.FrontendBaseURL)
	// v1.HandleFunc("/oauth/authorize", oauthHandler.HandleAuthorize).Methods("GET")
	// v1.HandleFunc("/oauth/callback", oauthHandler.HandleCallback).Methods("GET")
	// v1.HandleFunc("/oauth/token-exchange", oauthHandler.HandleTokenExchange).Methods("POST")
	// v1.HandleFunc("/idosos/by-cpf/{cpf}/google-status", oauthHandler.HandleGoogleStatus).Methods("GET")
	// v1.HandleFunc("/idosos/by-cpf/{cpf}/google-disconnect", oauthHandler.HandleGoogleDisconnect).Methods("POST")
	// log.Info().Msg("OAuth Google routes registered: /api/v1/oauth/*")

	// MCP Server — Model Context Protocol
	mcpServer := mcp.NewServer(db.Conn)
	if embedSvc != nil {
		mcpServer.SetEmbeddingFunc(func(ctx context.Context, text string) ([]float32, error) {
			return embedSvc.GenerateEmbedding(ctx, text)
		})
		mcpServer.SetVectorSearchFunc(func(ctx context.Context, collection string, vec []float32, limit int) ([]mcp.VectorResult, error) {
			searchResults, err := vectorAdapter.Search(ctx, collection, vec, limit, 0)
			if err != nil {
				return nil, err
			}
			results := make([]mcp.VectorResult, 0, len(searchResults))
			for _, r := range searchResults {
				content := ""
				if c, ok := r.Payload["content"]; ok {
					if s, ok := c.(string); ok {
						content = s
					}
				}
				results = append(results, mcp.VectorResult{
					ID:      0,
					Score:   float32(r.Score),
					Content: content,
				})
			}
			return results, nil
		})
		log.Info().Msg("MCP Server com busca vetorial ativada (NietzscheDB)")
	}
	router.PathPrefix("/mcp").Handler(mcpServer)
	log.Info().Msg("🔌 MCP Server montado em /mcp")

	// FHIR R4 Endpoints (HL7 interoperability)
	fhirHandler := integration.NewFHIRHandler(db.Conn)
	integration.RegisterFHIRRoutes(router, fhirHandler)
	log.Info().Msg("🏥 FHIR R4 endpoints registrados em /api/v1/fhir")

	// Prometheus metrics (protected by bearer token)
	router.Handle("/metrics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := os.Getenv("EVA_METRICS_TOKEN")
		if token != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		monitoring.PrometheusHandler().ServeHTTP(w, r)
	}))
	log.Info().Msg("📊 Prometheus metrics registrado em /metrics")

	// Tool Execution REST endpoint (para MCP stdio server)
	v1.HandleFunc("/tools/execute", server.handleToolExecute).Methods("POST")
	log.Info().Msg("🔧 Tool execution endpoint: POST /api/v1/tools/execute")

	// Core Memory — identidade e memória pessoal da EVA (/api/v1/self/*)
	if coreMemoryEngine != nil {
		evaSelf.RegisterRoutes(v1, coreMemoryEngine)
		log.Info().Msg("🧬 Rotas /api/v1/self/* registradas (CoreMemory)")
	} else {
		log.Warn().Msg("⚠️ Rotas /api/v1/self/* não registradas (CoreMemoryEngine indisponivel)")
	}

	// Research Engine REST routes (/api/v1/research/*)
	research.RegisterRoutes(v1, researchEng)
	log.Info().Msg("🔬 Research Engine rotas REST registradas em /api/v1/research/*")

	// 7. Scheduler (Background Jobs)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("CRITICO: Scheduler panic - background jobs parados")
			}
		}()
		scheduler.Start(ctx, db, cfg, log.Logger, alertService, pushService)
	}()

	// Autonomous Learner (background — estuda a cada 6h)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("CRITICO: Autonomous Learner panic")
			}
		}()
		autonomousLearner.Start(ctx)
	}()

	// Memory Scheduler (nightly REM consolidation 3AM + Krylov maintenance 6h)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("CRITICO: Memory Scheduler panic")
			}
		}()
		memSched := memscheduler.NewMemoryScheduler(memOrchestrator)
		memSched.Start(ctx)
	}()
	log.Info().Msg("Memory Scheduler iniciado (REM 3AM + Krylov 6h)")

	// Telegram Bot Polling (recebe mensagens e responde via Gemini)
	if telegramSvc != nil {
		chatHandler := telegram.NewChatHandler(
			func(prompt string) (string, error) {
				return gemini.AnalyzeText(cfg, prompt)
			},
			"eva_malaria_bot",
		)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("CRITICO: Telegram Polling panic")
				}
			}()
			telegramSvc.StartPolling(ctx, chatHandler.HandleMessage)
		}()
		// Limpeza periodica de historico de chat
		go func() {
			ticker := time.NewTicker(30 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					chatHandler.CleanupOldHistory()
				}
			}
		}()
		log.Info().Msg("📱 Telegram Bot polling iniciado (Gemini 2.5 Flash)")
	}

	// 8. Start Server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// cacheAdapter is used by embedSvc.SetCacheAdapter — no suppression needed

	// NietzscheDB Backup Service (daily automated backups)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("CRITICO: Backup Service panic")
			}
		}()
		backupService.Start(ctx)
	}()
	log.Info().Msg("NietzscheDB Backup Service started (daily)")

	// NietzscheDB CDC Listener + WebSocket Bridge (change data capture for audit log + Perspektive)
	cdcListener.Subscribe("eva_core", func(event nietzsche.CDCEvent) {
		log.Debug().
			Str("event_type", event.EventType).
			Str("entity_id", event.EntityID).
			Str("collection", event.Collection).
			Msg("[CDC] Change event")
	})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("CRITICO: CDC Bridge panic")
			}
		}()
		cdcBridge.Start(ctx, []string{"patient_graph", "eva_core"})
	}()
	log.Info().Msg("NietzscheDB CDC Bridge started (patient_graph, eva_core) — /ws/perspektive ready")

	go func() {
		log.Info().Msgf("EVA rodando na porta %s (NietzscheDB)", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Erro no servidor HTTP")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("Desligando graciosamente...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("Erro ao desligar servidor")
	}
}
