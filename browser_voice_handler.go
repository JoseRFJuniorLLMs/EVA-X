// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	gemini "eva-mind/internal/cortex/gemini"
	"net/http"
	"strings"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// ============================================================================
// geminiApp — Voz e Video via WebSocket para App Mobile (EVA-Mobile)
// ============================================================================
// Consumer:  geminiApp
// Rota:      /ws/browser
// Client:    internal/gemini (v1alpha, simples)
// Frontend:  App mobile EVA-Mobile
// Protocolo: WebSocket — audio PCM (16kHz in, 24kHz out) + video JPEG + texto
// Memoria:   Meta-cognitiva via Neo4j (carrega no inicio, salva transcricoes)
// CRITICO:   Protocolo WebSocket NAO pode mudar — app mobile depende
// Ver:       GEMINI_ARCHITECTURE.md para documentacao completa

// browserWSUpgrader permite conexoes de browsers (CORS flexivel)
var browserWSUpgrader = gws.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Browser clients de qualquer origem
	},
}

// browserMessage formato de mensagem browser <-> server
type browserMessage struct {
	Type string `json:"type"`           // "audio", "text", "config", "status"
	Data string `json:"data,omitempty"` // base64 PCM para audio
	Text string `json:"text,omitempty"` // texto para subtitles/chat
}

// handleBrowserVoice lida com WebSocket de voz vindo do browser
// Protocolo simples:
//   Browser envia: {"type":"audio","data":"base64_pcm_16khz"}
//   Browser envia: {"type":"config","text":"system_prompt"} (opcional, no inicio)
//   Server envia:  {"type":"audio","data":"base64_pcm_24khz"}
//   Server envia:  {"type":"text","text":"transcricao"}
//   Server envia:  {"type":"status","text":"ready|speaking|listening"}
func (s *SignalingServer) handleBrowserVoice(w http.ResponseWriter, r *http.Request) {
	conn, err := browserWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Browser WS upgrade failed")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Cria sessao Gemini usando cortex/gemini (v1beta, producao)
	geminiClient, err := gemini.NewClient(ctx, s.cfg)
	if err != nil {
		log.Error().Err(err).Msg("Erro ao criar cortex/gemini client para browser")
		conn.WriteJSON(browserMessage{Type: "status", Text: "error: " + err.Error()})
		return
	}
	defer geminiClient.Close()

	sessionID := "browser-" + time.Now().Format("20060102150405")

	// Carregar memoria meta-cognitiva do Neo4j
	var memories []string
	if s.evaMemory != nil {
		if err := s.evaMemory.StartSession(ctx, sessionID); err != nil {
			log.Warn().Err(err).Msg("[BROWSER] Falha ao registrar sessao no Neo4j")
		}
		metaCognition, err := s.evaMemory.LoadMetaCognition(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("[BROWSER] Falha ao carregar memoria meta-cognitiva")
		} else if metaCognition != "" {
			memories = []string{metaCognition}
			log.Info().Str("session", sessionID).Msg("[BROWSER] Memoria meta-cognitiva injetada")
		}
	}

	// Contexto hardcoded baseado no livro "A Malaria em Angola" - Dr. Ketsio Vaz
	malariaContext := `Voce e a EVA, assistente de voz do Sistema de Gestao de Malaria de Angola.
Voce possui memoria meta-cognitiva: lembra de conversas anteriores e reconhece padroes.
Se tiver memorias de sessoes passadas, use-as para contextualizar suas respostas.
Responda em portugues, de forma breve e conversacional (2-3 frases por resposta).
Voce domina todo o conteudo do livro "A Malaria em Angola" do Dr. Ketsio Vaz.

=== CLASSIFICACAO CLINICA ===
Malaria Simples: febre, cefaleia, mialgias, nauseas, vomitos, esplenomegalia. Sem disfuncao de orgao.
Malaria Complicada/Grave (OMS): formas assexuadas no sangue com manifestacao potencialmente fatal.
Complicacoes: malaria cerebral, anemia severa, insuficiencia renal, acidose metabolica, hipoglicemia, choque, edema pulmonar.
Evolucao: Aguda (P. vivax/ovale), Fulminante (P. falciparum - risco de morte), Cronica (P. malariae - glomerulonefrite).

=== ESPECIES EM ANGOLA ===
P. falciparum: 92% dos casos, incubacao 9-14 dias, mais grave e letal.
P. vivax: 7%, incubacao 12-18 dias.
P. malariae: 1%, incubacao 18-40 dias.
P. ovale: presente mas nao documentado oficialmente, incubacao 9-18 dias.
P. knowlesi: raro, incubacao 20-30 dias.
Vector principal: complexo Anopheles gambiae (melas e arabiensis) e Anopheles funestus.

=== ENDEMIA MALARICA ===
Holoendemica: esplenomegalia >75%, parasitemia >75%.
Hiperendemica: 51-75%.
Mesoendemica: 11-50%.
Hipoendemica: 0-10%.
Angola: malaria endemica em todo o pais, tres estratos epidemiologicos.

=== MALARIA ESTAVEL vs INSTAVEL ===
Estavel: endemicidade forte, vector antropofilo, imunidade forte, epidemias improvaveis, P. falciparum.
Instavel: endemicidade fraca/moderada, vector pouco antropofilo, imunidade variavel, epidemias frequentes, P. vivax.

=== VIGILANCIA EPIDEMIOLOGICA ===
Caso Suspeito: quadro febril em area endemica ou deslocacao nos ultimos 8-30 dias.
Caso Confirmado: confirmacao laboratorial.
Caso Autoctone: contraido na zona de diagnostico.
Caso Importado: contraido fora da zona.
Caso Introduzido: secundario de caso importado.
Caso Induzido: transfusao ou inoculacao parenteral.
Caso Criptico: area sem transmissao, origem nao identificavel.

=== DIAGNOSTICO ===
Clinico: febre + area endemica. Deve SEMPRE ser confirmado laboratorialmente.
Gota Espessa (GE): padrao-ouro, detecta parasitemia baixa, quantifica parasitas.
Esfregaco Sanguineo: identifica especie de Plasmodium.
Teste Rapido (TDR): detecta HRP-2 (P. falciparum) ou pLDH. Resultado em 15-20 min.
Coloracoes: Giemsa (padrao), May-Grunwald-Giemsa, Field, Leishman.
Parasitemia: leve (<1%), moderada (1-5%), grave (>5% ou >200.000/uL).

=== INSUFICIENCIA RENAL POR MALARIA ===
Leve: creatinina 3.1-5.0 mg/dl.
Moderada: creatinina 5.1-7.0 mg/dl.
Severa: creatinina >7.0 mg/dl.

=== TRATAMENTO - MALARIA NAO COMPLICADA ===
1a linha: ACT (Combinacoes Terapeuticas a Base de Artemisinina).
- Artemeter + Lumefantrina (AL): 1a escolha em Angola.
  Adulto (>35kg): 4 comprimidos, 6 doses em 3 dias (0h, 8h, 24h, 36h, 48h, 60h).
- Artesunato + Amodiaquina (AS+AQ): alternativa.
- DHA + Piperaquina (DHA+PPQ): alternativa.
Primaquina: dose unica 0.25mg/kg no 1o dia para eliminar gametocitos (contraindicada em gravidas e <6 meses).
Controlo de cura: gota espessa no dia 3, 7, 14, 28.

Nao recomendados para P. falciparum: monoterapia com artemisinina, cloroquina (resistencia), sulfadoxina-pirimetamina isolada.

=== TRATAMENTO - MALARIA GRAVE ===
EMERGENCIA MEDICA - internar imediatamente.
1a linha: Artesunato EV/IM.
- Dose: 2.4 mg/kg nos tempos 0h, 12h, 24h, depois 1x/dia ate tolerar via oral.
- Minimo 24h de artesunato parenteral antes de trocar para ACT oral.
2a linha (se artesunato indisponivel): Artemeter IM 3.2 mg/kg dose inicial, depois 1.6 mg/kg/dia.
3a linha (ultimo recurso): Quinino EV 20 mg/kg dose de ataque em 4h, depois 10 mg/kg 8/8h.
Apos estabilizacao: completar com ACT oral (AL ou AS+AQ) por 3 dias.

=== COMPLICACOES E MANEJO ===
Malaria Cerebral: Glasgow <11 adulto ou Blantyre <3 crianca. Artesunato EV, NÃO usar corticoides.
Anemia Severa: Hb <5g/dl ou Ht <15%. Transfusao de concentrado de hemacias.
Hipoglicemia: glicemia <40mg/dl. Dextrose 50% EV (1ml/kg adulto), manter perfusao com dextrose 5-10%.
Acidose: pH <7.25 ou bicarbonato <15. Corrigir desidratacao, tratar causa base.
Edema Pulmonar: elevar cabeceira, restringir liquidos, furosemida, oxigenio.
Insuficiencia Renal: hidratacao cuidadosa, dialise se necessario.
Choque: reposicao volemica, vasopressores se necessario, antibioticos (suspeitar sepse).

=== MALARIA NA GRAVIDEZ ===
Alto risco: anemia materna severa, baixo peso ao nascer, aborto, malaria congenita, MFIU.
1o trimestre: Quinino oral 10mg/kg 8/8h por 7 dias + Clindamicina.
2o e 3o trimestre: ACT (Artemeter+Lumefantrina).
Malaria grave na gravidez: Artesunato EV (mesmo protocolo).
TIP (Tratamento Intermitente Preventivo): Sulfadoxina-Pirimetamina a partir do 2o trimestre.

=== QUIMIOPROFILAXIA PARA VIAJANTES ===
Doxiciclina: 100mg/dia, iniciar 1 dia antes, manter 4 semanas apos. CI: gravidas, <8 anos.
Mefloquina: 250mg/semana, iniciar 2 semanas antes. CI: epilepsia, disturbios psiquiatricos.
Atovaquona+Proguanil: 1cp/dia, iniciar 1 dia antes, manter 7 dias apos.

=== PREVENCAO E CONTROLO ===
REMTI: redes mosquiteiras tratadas com inseticida (principal medida individual).
PIDOM: pulverizacao intra-domiciliaria com inseticida residual.
GVI: gestao vectorial integrada.
Protecao pessoal: repelentes, roupas compridas, telas nas janelas, evitar exposicao ao amanhecer/entardecer.

=== FLUXO DO SISTEMA ===
Tecnico coleta amostra de sangue → IA analisa imagem microscopica → Sistema detecta parasitas e especie → Medico revisa resultado → Prescricao de tratamento segundo protocolo.`

	// Setup com cortex/gemini (5 params: instructions, voiceSettings, memories, initialAudio, toolsDef)
	err = geminiClient.SendSetup(
		malariaContext,
		map[string]interface{}{
			"voiceName":    "Aoede",
			"languageCode": "pt-BR",
		},
		memories, // memoria meta-cognitiva carregada do Neo4j
		"",       // initialAudio
		nil,      // toolsDef
	)
	if err != nil {
		log.Error().Err(err).Msg("Erro ao configurar Gemini para browser")
		conn.WriteJSON(browserMessage{Type: "status", Text: "error: setup failed"})
		return
	}

	log.Info().Str("session", sessionID).Int("memories", len(memories)).Msg("Browser voice session started (cortex/gemini)")

	// Notifica browser que esta pronto
	conn.WriteJSON(browserMessage{Type: "status", Text: "ready"})

	var writeMu sync.Mutex
	errChan := make(chan error, 2)

	// Buffer para acumular transcricao da resposta da EVA (para salvar no Neo4j)
	var responseAccum strings.Builder

	// Goroutine: Gemini -> Browser (audio responses)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				resp, err := geminiClient.ReadResponse()
				if err != nil {
					errChan <- err
					return
				}

				// setupComplete
				if _, ok := resp["setupComplete"]; ok {
					continue
				}

				serverContent, ok := resp["serverContent"].(map[string]interface{})
				if !ok {
					continue
				}

				// Interrupcao
				if interrupted, ok := serverContent["interrupted"].(bool); ok && interrupted {
					writeMu.Lock()
					conn.WriteJSON(browserMessage{Type: "status", Text: "interrupted"})
					writeMu.Unlock()
					responseAccum.Reset()
					continue
				}

				// Turn complete
				if turnComplete, ok := serverContent["turnComplete"].(bool); ok && turnComplete {
					writeMu.Lock()
					conn.WriteJSON(browserMessage{Type: "status", Text: "turn_complete"})
					writeMu.Unlock()

					// Salvar resposta acumulada no Neo4j
					if s.evaMemory != nil && responseAccum.Len() > 0 {
						go s.evaMemory.StoreTurn(ctx, sessionID, "assistant", responseAccum.String())
					}
					responseAccum.Reset()
					continue
				}

				// Transcricao do input do usuario
				if inputTrans, ok := serverContent["inputAudioTranscription"].(map[string]interface{}); ok {
					if text, ok := inputTrans["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Text: text, Data: "user"})
						writeMu.Unlock()
						// Salvar transcricao do usuario no Neo4j
						if s.evaMemory != nil {
							go s.evaMemory.StoreTurn(ctx, sessionID, "user", text)
						}
					}
				}

				// Transcricao do output do modelo (audio -> texto)
				if outputTrans, ok := serverContent["outputAudioTranscription"].(map[string]interface{}); ok {
					if text, ok := outputTrans["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Text: text})
						writeMu.Unlock()
						responseAccum.WriteString(text)
					}
				}

				// Audio e texto do modelo
				modelTurn, ok := serverContent["modelTurn"].(map[string]interface{})
				if !ok {
					continue
				}

				parts, ok := modelTurn["parts"].([]interface{})
				if !ok {
					continue
				}

				for _, p := range parts {
					part, ok := p.(map[string]interface{})
					if !ok {
						continue
					}

					// Texto (subtitles)
					if text, ok := part["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Text: text})
						writeMu.Unlock()
					}

					// Audio inline
					if inlineData, ok := part["inlineData"].(map[string]interface{}); ok {
						if audioB64, ok := inlineData["data"].(string); ok {
							writeMu.Lock()
							conn.WriteJSON(browserMessage{Type: "audio", Data: audioB64})
							writeMu.Unlock()
						}
					}
				}
			}
		}
	}()

	// Goroutine: Browser -> Gemini (audio input)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, msgBytes, err := conn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}

				var msg browserMessage
				if err := json.Unmarshal(msgBytes, &msg); err != nil {
					continue
				}

				switch msg.Type {
				case "audio":
					// Decode base64 PCM do browser (16kHz, 16-bit)
					pcmData, err := base64.StdEncoding.DecodeString(msg.Data)
					if err != nil {
						continue
					}
					geminiClient.SendAudio(pcmData)

				case "video":
					// Frame JPEG da camera do browser (1 FPS)
					jpegData, err := base64.StdEncoding.DecodeString(msg.Data)
					if err != nil {
						continue
					}
					geminiClient.SendImage(jpegData)

				case "text":
					// Mensagem de texto direta
					if msg.Text != "" {
						if s.evaMemory != nil {
							go s.evaMemory.StoreTurn(ctx, sessionID, "user", msg.Text)
						}
						geminiClient.SendText(msg.Text)
					}

				case "config":
					// Permite reconfigurar system prompt mid-session
					log.Info().Str("session", sessionID).Msg("Browser sent config update")
				}
			}
		}
	}()

	// Espera erro de qualquer goroutine
	sessionErr := <-errChan

	// Finalizar sessao no Neo4j
	if s.evaMemory != nil {
		s.evaMemory.EndSession(ctx, sessionID)
		go s.evaMemory.DetectPatterns(context.Background())
	}

	log.Info().Str("session", sessionID).Err(sessionErr).Msg("Browser voice session ended")
}
