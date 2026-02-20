// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/brainstem/infrastructure/workerpool"
)

type Handler struct {
	cfg            *config.Config
	db             *database.DB
	graphAdapter   *nietzscheInfra.GraphAdapter   // NietzscheDB GraphAdapter (substitui Neo4j)
	vectorAdapter  *nietzscheInfra.VectorAdapter  // NietzscheDB VectorAdapter (substitui Qdrant)
	toolsClient    *ToolsClient                   // Cliente REST separado para Tools
}

func NewHandler(cfg *config.Config, db *database.DB, graphAdapter *nietzscheInfra.GraphAdapter, vectorAdapter *nietzscheInfra.VectorAdapter) *Handler {
	return &Handler{
		cfg:           cfg,
		db:            db,
		graphAdapter:  graphAdapter,
		vectorAdapter: vectorAdapter,
		toolsClient:   NewToolsClient(cfg),
	}
}

// ProcessResponse processa a resposta bruta do WebSocket do Gemini
// Retorna: (audioBytes, turnUpdated bool)
func (h *Handler) ProcessResponse(ctx context.Context, session interface{}, resp map[string]interface{}, currentTurnID uint64) ([]byte, bool) {
	// Asserção de tipo segura para evitar dependência circular se possível,
	// ou defina uma interface Session. Aqui assumimos interface{} para flexibilidade.

	serverContent, ok := resp["serverContent"].(map[string]interface{})
	if !ok {
		return nil, false
	}

	modelTurn, ok := serverContent["modelTurn"].(map[string]interface{})
	if !ok {
		return nil, false
	}

	parts, ok := modelTurn["parts"].([]interface{})
	if !ok {
		return nil, false
	}

	var combinedAudio []byte
	turnComplete := false

	// Verifica flags de turno
	if tc, ok := serverContent["turnComplete"].(bool); ok && tc {
		turnComplete = true
	}

	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		// 1. Áudio Inline
		if inlineData, ok := part["inlineData"].(map[string]interface{}); ok {
			if b64, ok := inlineData["data"].(string); ok {
				data, err := base64.StdEncoding.DecodeString(b64)
				if err == nil {
					combinedAudio = append(combinedAudio, data...)
				}
			}
		}

		// 2. Texto / Comandos (Protocolo de Delegação)
		if text, ok := part["text"].(string); ok {
			// Regex para capturar [[TOOL:nome:{arg}]]
			// Workaround para modelos de áudio que não suportam tools nativas via WS
			re := regexp.MustCompile(`\[\[TOOL:(\w+):({.*?})\]\]`)
			matches := re.FindStringSubmatch(text)

			if len(matches) == 3 {
				toolName := matches[1]
				argsJSON := matches[2]

				// Executar tool em goroutine via WorkerPool para não bloquear áudio
				workerpool.BackgroundPool.Submit(ctx, func() {
					h.executeToolAndInjectBack(ctx, session, toolName, argsJSON, currentTurnID)
				})
			}

			// Logar transcrição para análise futura
			if len(text) > 0 && text[0] != '[' { // Ignora comandos internos
				log.Printf("🤖 EVA (Transcrição): %s", text)
			}
		}
	}

	return combinedAudio, turnComplete
}

// executeToolAndInjectBack executa a ferramenta e devolve o resultado como prompt de sistema
func (h *Handler) executeToolAndInjectBack(ctx context.Context, session interface{}, name, argsJSON string, turnID uint64) {
	// Parse args
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		log.Printf("❌ Erro JSON tool: %v", err)
		return
	}

	log.Printf("🛠️ Executando Tool: %s (Turno %d)", name, turnID)

	// Simulação de execução (aqui chamaria o service real de tools)
	// Para este exemplo, apenas formatamos um sucesso simulado
	result := map[string]string{"status": "success", "executed_at": "now"}

	resultJSON, _ := json.Marshal(result)

	// Injection: Envia o resultado como texto de sistema para o modelo saber o que aconteceu
	// O modelo (SafeSession) deve ter um método SendText
	if s, ok := session.(interface{ SendText(string) error }); ok {
		feedbackMsg := fmt.Sprintf("[SISTEMA: Ferramenta '%s' executada. Resultado: %s]", name, string(resultJSON))
		s.SendText(feedbackMsg)
	}
}
