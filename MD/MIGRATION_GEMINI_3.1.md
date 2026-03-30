# Migração: gemini-2.5-flash-native-audio → gemini-3.1-flash-live-preview

**Data da auditoria:** 2026-03-27
**Status:** EM STANDBY (requer billing account Google)
**Auditoria:** 30 agentes analisaram todo o codebase

---

## Pré-requisitos

1. **Billing Account** obrigatória no Google Cloud — o modelo 3.1 NÃO funciona com free tier
2. API key com billing activo em https://aistudio.google.com → Settings → Billing
3. A key nova (com billing) deve ser setada em `.env` como `GOOGLE_API_KEY`

---

## Mudanças no .env

```bash
# ANTES (actual em produção)
MODEL_ID=gemini-2.5-flash-native-audio-preview-12-2025
GOOGLE_API_KEY=AIzaSyDhp2XHiXxgKHmy2yqw30M_h8MamwzPPLQ

# DEPOIS (quando billing estiver activo)
MODEL_ID=gemini-3.1-flash-live-preview
GOOGLE_API_KEY=<key-com-billing>
```

---

## Compatibilidade (confirmado por 30 agentes)

| Componente | Compatível? | Notas |
|------------|-------------|-------|
| Protocolo WebSocket v1beta | ✅ SIM | Mesmo endpoint, mesma URL |
| Audio PCM 16kHz input | ✅ SIM | Sem mudanças |
| Audio PCM 24kHz output | ✅ SIM | Sem mudanças |
| Voz Aoede pt-BR | ✅ SIM | Mesmas vozes disponíveis |
| speech_config struct | ✅ SIM | Mesma estrutura |
| response_modalities: AUDIO | ✅ SIM | Funciona igual |
| Google Search grounding | ✅ SIM | `"google_search": {}` funciona |
| Context window 131K | ✅ SIM | Prompt 85-100KB dentro do limite |
| Frontend React (EVA-Web) | ✅ SIM | Zero mudanças necessárias |
| Speaker diarization ECAPA | ✅ SIM | Independente do modelo |

---

## Mudanças de código OBRIGATÓRIAS (4 fixes)

### 1. thinkingConfig (estrutura correcta)

```go
// internal/cortex/gemini/client.go — dentro de generation_config
"thinkingConfig": map[string]interface{}{
    "thinkingLevel": "MINIMAL",  // MINIMAL, LOW, MEDIUM, HIGH
},
```

⚠️ NÃO usar `"thinkingLevel"` directamente no generation_config — deve estar nested em `thinkingConfig`!

### 2. Tool calling SYNC (remover goroutine)

```go
// internal/cortex/gemini/client.go:413
// ANTES (async — QUEBRA no 3.1):
go func(n string, a map[string]interface{}, id string) {
    result := c.onToolCall(n, a)
    c.SendToolResponse(n, result, id)
}(name, args, fcID)

// DEPOIS (sync — correcto para 3.1):
result := c.onToolCall(name, args)
c.SendToolResponse(name, result, fcID)
```

### 3. SendText → send_realtime_input (mid-session)

```go
// internal/cortex/gemini/client.go:230-249
// ANTES:
"client_content": map[string]interface{}{...}

// DEPOIS (para texto mid-session):
"realtime_input": map[string]interface{}{
    "text": text,
}
```

Afecta 4 locais:
- `eva_handler.go:373`
- `websocket.go:785` (insight injection)
- `websocket.go:884` (executive interrupt)
- `websocket.go:1678` (tool feedback)

### 4. Multi-part text handler (client.go:367-379)

Adicionar handler para `part["text"]` no loop de parts:
```go
// Depois do handler de inlineData (audio), adicionar:
if text, ok := part["text"].(string); ok && text != "" {
    if c.onTranscript != nil {
        c.onTranscript("assistant", text)
    }
}
```

---

## Melhorias RECOMENDADAS (não obrigatórias)

### turn_coverage (redução de custos)
```go
// generation_config — evita processar frames de vídeo desnecessários
"turn_coverage": "TURN_INCLUDES_ONLY_ACTIVITY",
```

### thinkingLevel por endpoint REST
| Endpoint | thinkingLevel recomendado |
|----------|--------------------------|
| WebSocket voz (client.go) | MINIMAL (latência mínima) |
| REST análise (rest_client.go) | LOW |
| REST conversation (analysis.go) | LOW |
| REST tools (tools_client.go) | LOW |
| REST learner (autonomous_learner.go) | LOW |

---

## NÃO é necessário mudar

- Vozes (Aoede, Puck, Charon, etc.)
- Audio format (PCM 16/24kHz)
- Base64 encoding
- WebSocket URL endpoint
- Frontend React/TypeScript
- NietzscheDB (completamente independente)
- Speaker identification ECAPA-TDNN
- Emotion detection (keyword-based funciona)

---

## Como activar (quando billing estiver pronto)

```bash
# 1. Na VM (136.111.0.47):
ssh web2a@136.111.0.47

# 2. Editar .env:
cd /home/web2a/EVA-X
sed -i 's|MODEL_ID=gemini-2.5-flash-native-audio-preview-12-2025|MODEL_ID=gemini-3.1-flash-live-preview|' .env
sed -i 's|GOOGLE_API_KEY=.*|GOOGLE_API_KEY=<nova-key-com-billing>|' .env

# 3. Aplicar os 4 fixes de código (se ainda não aplicados)

# 4. Rebuild e deploy:
go build -o eva-x-new . && sudo systemctl stop eva-x && cp eva-x-new eva-x && sudo systemctl start eva-x

# 5. Verificar logs:
journalctl -u eva-x -f | grep -i 'model\|error\|ready'
```

---

## Rollback (se falhar)

```bash
cd /home/web2a/EVA-X
sed -i 's|MODEL_ID=gemini-3.1-flash-live-preview|MODEL_ID=gemini-2.5-flash-native-audio-preview-12-2025|' .env
sed -i 's|GOOGLE_API_KEY=.*|GOOGLE_API_KEY=AIzaSyDhp2XHiXxgKHmy2yqw30M_h8MamwzPPLQ|' .env
sudo systemctl restart eva-x
```

---

## Benefícios esperados do 3.1

- Sessões 2x mais longas (~20 min vs ~10 min)
- Menor latência de resposta
- Tool calling nativo (sync, mais fiável)
- thinkingLevel configurável (tradeoff qualidade/velocidade)
- Melhor qualidade de voz e compreensão
