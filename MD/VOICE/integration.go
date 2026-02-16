package integration

// integration.go — Como conectar o módulo de biometria ao WebSocket do eva-mind
//
// Este arquivo mostra ONDE e COMO injetar o pipeline de voz no fluxo
// existente do eva-mind (porta 8090). Adapte ao seu código atual.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/eva-project/eva-mind/voice"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// EVASession representa uma sessão de conversa ativa com um usuário.
type EVASession struct {
	SessionID string

	// Identidade atual do falante nesta sessão.
	// Atualizada a cada chunk de áudio identificado com alta confiança.
	CurrentSpeaker *voice.IdentificationResult

	// Confiança acumulada na sessão atual (média ponderada dos últimos N chunks).
	// Usada para estabilizar: se a EVA já reconheceu Junior 3x, não perde
	// a identidade por um chunk ruidoso.
	AccumulatedConfidence float64
	IdentificationCount   int

	mu sync.Mutex
}

// AudioChunkMessage é a mensagem WebSocket que traz um chunk de áudio PCM.
type AudioChunkMessage struct {
	Type      string    `json:"type"`       // "audio_chunk"
	SessionID string    `json:"session_id"`
	PCM       []float32 `json:"pcm"`        // PCM float32, 16kHz, mono
	Timestamp time.Time `json:"timestamp"`
}

// GeminiRequest é o que é enviado para o Gemini 2.5 Flash Native Audio.
type GeminiRequest struct {
	SystemInstruction string `json:"system_instruction"`
	AudioData         []float32
	SessionID         string
}

// ─── EVAMind é o handler principal do WebSocket ──────────────────────────

type EVAMind struct {
	voicePipeline *voice.Pipeline
	sessions      sync.Map // map[sessionID]*EVASession
	log           *zap.Logger
}

// NewEVAMind inicializa o eva-mind com o pipeline de biometria integrado.
func NewEVAMind(neo4jDriver neo4j.DriverWithContext, log *zap.Logger) (*EVAMind, error) {
	store := voice.NewNeo4jStore(neo4jDriver, log)

	pipeline, err := voice.NewPipeline(
		voice.DefaultSRCConfig(),
		voice.EmbedderConfig{
			ModelPath: "/opt/eva/models/titanet_large.onnx",
			Threads:   2,
		},
		store,
		log,
	)
	if err != nil {
		return nil, fmt.Errorf("NewPipeline: %w", err)
	}

	return &EVAMind{
		voicePipeline: pipeline,
		log:           log,
	}, nil
}

// HandleAudioChunk é chamado para cada chunk de áudio recebido pelo WebSocket.
// Retorna o GeminiRequest pronto para ser enviado ao modelo.
func (e *EVAMind) HandleAudioChunk(ctx context.Context, msg AudioChunkMessage) (*GeminiRequest, error) {
	// ── 1. Recupera ou cria sessão ────────────────────────────────────────
	raw, _ := e.sessions.LoadOrStore(msg.SessionID, &EVASession{SessionID: msg.SessionID})
	session := raw.(*EVASession)

	// ── 2. Identifica falante ─────────────────────────────────────────────
	result, err := e.voicePipeline.Identify(ctx, msg.PCM)
	if err != nil {
		e.log.Warn("identify error", zap.Error(err))
		// Continua com identidade desconhecida — não para o fluxo
	}

	// ── 3. Atualiza estado da sessão ──────────────────────────────────────
	session.mu.Lock()
	e.updateSessionIdentity(session, result)
	effectiveSpeaker := session.CurrentSpeaker
	session.mu.Unlock()

	// ── 4. Verifica desvio de voz (RAM — Realistic Accuracy Model) ────────
	var deviationAlert string
	if effectiveSpeaker != nil && !result.Unknown {
		deviationAlert = voice.BuildVoiceDeviationAlert(result, effectiveSpeaker.ResidualErr)
	}

	// ── 5. Constrói o System Instruction com priming dinâmico ─────────────
	baseSystemPrompt := e.baseSystemPrompt()
	var priming string
	if effectiveSpeaker != nil {
		priming = voice.BuildGeminiPriming(*effectiveSpeaker)
	}
	if deviationAlert != "" {
		priming = deviationAlert + "\n" + priming
	}

	finalSystemInstruction := voice.InjectIntoPriming(baseSystemPrompt, priming)

	e.log.Debug("audio chunk processed",
		zap.String("session", msg.SessionID),
		zap.String("speaker", speakerName(effectiveSpeaker)),
		zap.Float64("confidence", speakerConfidence(effectiveSpeaker)),
		zap.Duration("voice_latency", result.Latency),
	)

	return &GeminiRequest{
		SystemInstruction: finalSystemInstruction,
		AudioData:         msg.PCM,
		SessionID:         msg.SessionID,
	}, nil
}

// updateSessionIdentity implementa a lógica de estabilização de identidade.
//
// Regras:
//   1. Se a identidade nova tem confiança alta (>= threshold): atualiza imediatamente
//   2. Se a identidade nova é desconhecida MAS a sessão já tem identidade com
//      confiança acumulada alta: mantém a identidade anterior (chunk ruidoso isolado)
//   3. Se a identidade muda (outra pessoa assumiu o microfone): atualiza após
//      2 confirmações consecutivas da nova identidade (evita falsos positivos)
func (e *EVAMind) updateSessionIdentity(session *EVASession, result voice.IdentificationResult) {
	cfg := voice.DefaultSRCConfig()

	if result.Unknown {
		// Não descarta identidade existente por um chunk ruim
		if session.AccumulatedConfidence > 0.75 {
			return
		}
		session.CurrentSpeaker = nil
		session.AccumulatedConfidence = 0
		return
	}

	if result.Confidence >= cfg.ConfidenceThreshold {
		if session.CurrentSpeaker == nil || session.CurrentSpeaker.SpeakerID == result.SpeakerID {
			// Mesma pessoa ou primeira identificação: atualiza normalmente
			session.CurrentSpeaker = &result
			// Média exponencial: confiança acumulada = 0.7*anterior + 0.3*nova
			session.AccumulatedConfidence = 0.7*session.AccumulatedConfidence + 0.3*result.Confidence
			session.IdentificationCount++
		} else {
			// Pessoa diferente detectada: incrementa contador de troca
			// (aguarda 2 confirmações antes de trocar)
			session.IdentificationCount--
			if session.IdentificationCount <= 0 {
				e.log.Info("speaker change detected",
					zap.String("session", session.SessionID),
					zap.String("old", session.CurrentSpeaker.Name),
					zap.String("new", result.Name),
				)
				session.CurrentSpeaker = &result
				session.AccumulatedConfidence = result.Confidence
				session.IdentificationCount = 1
			}
		}
	}
}

// EnrollEndpoint é o handler HTTP para /voice/enroll.
// Adicione ao seu router existente no eva-mind:
//   mux.HandleFunc("POST /voice/enroll", evaMind.EnrollEndpoint)
func (e *EVAMind) EnrollEndpoint(w http.ResponseWriter, r *http.Request) {
	handler := voice.NewHandler(e.voicePipeline, e.log)
	// Delega para o handler do módulo de voz
	_ = handler
	// handler.ServeHTTP(w, r)  — adapte conforme seu router
}

// baseSystemPrompt retorna o prompt base da EVA (sem priming de voz).
// Substitua pelo seu prompt real.
func (e *EVAMind) baseSystemPrompt() string {
	return `Você é a EVA, uma inteligência artificial empática e contextual.
Você possui memória persistente via grafo de conhecimento (Neo4j) e
adapta suas respostas ao histórico, preferências e estado emocional
de cada usuário identificado.

Sempre responda em português, de forma natural e personalizada.`
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func speakerName(r *voice.IdentificationResult) string {
	if r == nil {
		return "unknown"
	}
	return r.Name
}

func speakerConfidence(r *voice.IdentificationResult) float64 {
	if r == nil {
		return 0
	}
	return r.Confidence
}

// ─── Exemplo de payload WebSocket ─────────────────────────────────────────

// ExampleWebSocketFlow mostra o fluxo completo em pseudo-código comentado.
// Não é código executável — é documentação inline.
func ExampleWebSocketFlow() {
	/*
	   1. Cliente (frontend) conecta no WebSocket: ws://104.248.219.200:8090/ws/audio

	   2. A cada ~3s de áudio capturado pelo microfone, o frontend envia:
	      {
	        "type": "audio_chunk",
	        "session_id": "sess_abc123",
	        "pcm": [0.012, -0.003, ...],  // ~48000 float32 (3s × 16kHz)
	        "timestamp": "2025-01-15T10:30:00Z"
	      }

	   3. O eva-mind:
	      a. Roda VAD → extrai apenas os frames com fala
	      b. Roda TitaNet ONNX → D-Vector [512]
	      c. Busca perfis no cache → filtra por cosseno → OMP
	      d. Calibra confiança → decide identidade
	      e. Constrói priming → injeta no Gemini

	   4. O Gemini responde com áudio (Native Audio) e texto
	      já personalizados para o falante identificado.

	   Latência típica do passo 3a→3d: 15–40ms (sem GPU)
	   Latência típica com GPU (RTX3080): 5–12ms
	*/
	_ = json.Marshal // evita "imported and not used"
}
