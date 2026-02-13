# Auditoria Completa: Interrupções de Conversação EVA-Mind

**Data**: 2026-02-13  
**Objetivo**: Identificar e documentar todas as causas de interrupções abruptas nas conversas, onde EVA para de falar mas a ligação não cai imediatamente.

---

## Resumo Executivo

Após auditoria completa recursiva do sistema de conversação EVA-Mind, foram identificados **7 pontos críticos de falha silenciosa** que causam interrupções abruptas durante conversas:

1. **Erro silencioso no listener Gemini** - Retorna sem notificar o cliente
2. **Timeout de leitura excessivo** - 5 minutos é muito longo
3. **Canal de áudio bloqueado** - Drops silenciosos sem notificação
4. **Falta de heartbeat** - Conexão pode ser fechada por proxies
5. **Erros de parsing JSON** - Falhas silenciosas no processamento
6. **Cleanup sem notificação** - Fecha conexão sem avisar mobile
7. **Timeout de inatividade longo** - 5 minutos sem feedback

---

## Arquitetura Analisada

### Fluxo de Comunicação

```
Mobile App (WebSocket)
    ↓
SignalingServer (main.go)
    ↓
GeminiClient (internal/cortex/gemini/client.go)
    ↓
Gemini Live API (WebSocket)
    ↓
← Audio Response ←
    ↓
SendCh (channel buffer: 256)
    ↓
handleClientSend
    ↓
Mobile App (Audio Player)
```

### Componentes Críticos

1. **HandleWebSocket** - Aceita conexões e cria PCMClient
2. **handleClientMessages** - Processa mensagens do mobile (register, start_call, audio)
3. **setupGeminiSession** - Configura sessão Gemini com callbacks
4. **listenGemini** - Loop que lê respostas do Gemini
5. **processGeminiResponse** - Processa áudio e transcrições
6. **handleClientSend** - Envia áudio para mobile via WebSocket
7. **monitorClientActivity** - Monitora timeout de inatividade
8. **cleanupClient** - Limpa recursos ao desconectar

---

## Pontos de Falha Identificados

### 🔴 1. Gemini Client - Erro Silencioso em `listenGemini`

**Arquivo**: `main.go:1878-1893`

```go
func (s *SignalingServer) listenGemini(client *PCMClient) {
    for client.active.Load() {
        resp, err := client.GeminiClient.ReadResponse()
        if err != nil {
            if client.active.Load() {
                log.Printf("⚠️ Gemini read error: %v", err)
            }
            return  // ❌ PROBLEMA: Retorna sem notificar o cliente!
        }
        s.processGeminiResponse(client, resp)
    }
}
```

**Impacto**: Quando o Gemini WebSocket falha (timeout, erro de rede, limite de tokens), a função retorna silenciosamente. O cliente mobile continua conectado ao WebSocket do servidor, mas não recebe mais áudio porque o listener parou.

**Sintomas**:
- EVA para de falar abruptamente
- Conexão WebSocket permanece ativa
- Logs mostram "Gemini read error" mas mobile não é notificado
- Usuário fica esperando sem feedback

---

### 🔴 2. Gemini Client - ReadDeadline de 5 Minutos

**Arquivo**: `internal/cortex/gemini/client.go:263`

```go
const readTimeout = 5 * time.Minute
```

**Impacto**: Se o Gemini parar de responder (por qualquer motivo), o sistema espera até 5 minutos antes de detectar o problema. Durante esse tempo, o usuário fica em silêncio sem feedback.

**Sintomas**:
- Pausas longas sem resposta
- Usuário não sabe se EVA está processando ou travada
- Timeout só detectado após 5 minutos

---

### 🔴 3. SendCh Channel Bloqueado

**Arquivo**: `main.go:1191-1196`

```go
func(audioBytes []byte) {
    select {
    case client.SendCh <- audioBytes:
        // OK
    default:
        log.Printf("⚠️ Canal cheio, dropando áudio para %s", client.CPF)
        // ❌ PROBLEMA: Dropa áudio silenciosamente, sem notificar cliente
    }
}
```

**Impacto**: Se o canal `SendCh` estiver cheio (buffer de 256), o áudio é descartado silenciosamente. Isso pode acontecer se `handleClientSend` estiver lento ou bloqueado.

**Sintomas**:
- Áudio cortado ou com gaps
- Logs mostram "Canal cheio, dropando áudio"
- Mobile não recebe notificação de problema de rede
- Experiência degradada sem feedback

---

### 🔴 4. Falta de Heartbeat/Keepalive no Gemini WebSocket

**Arquivo**: `internal/cortex/gemini/client.go`

O cliente Gemini não implementa ping/pong ou heartbeat. Se a conexão com o Gemini ficar idle, pode ser fechada silenciosamente por proxies ou firewalls.

**Impacto**: Conexões idle podem ser fechadas por infraestrutura de rede intermediária sem que o sistema detecte.

**Sintomas**:
- Conexão "fantasma" que parece ativa mas não funciona
- Timeouts inesperados após períodos de silêncio
- Erros de leitura após pausas longas

---

### 🔴 5. Erro de Parsing JSON Não Notifica Cliente

**Arquivo**: `main.go:1895-1899`

```go
func (s *SignalingServer) processGeminiResponse(client *PCMClient, resp map[string]interface{}) {
    serverContent, ok := resp["serverContent"].(map[string]interface{})
    if !ok {
        return  // ❌ PROBLEMA: Retorna silenciosamente
    }
```

**Impacto**: Se o Gemini enviar uma resposta malformada, o processamento para sem notificar o usuário.

**Sintomas**:
- EVA para de responder sem motivo aparente
- Logs não mostram erro claro
- Usuário não recebe feedback

---

### 🔴 6. Contexto Cancelado Não Notifica Mobile

**Arquivo**: `main.go:2029-2045`

```go
func (s *SignalingServer) cleanupClient(client *PCMClient) {
    log.Printf("🧹 Cleanup: %s", client.CPF)
    client.cancel()
    // ... fecha conexões
    // ❌ PROBLEMA: Não envia mensagem de erro para o mobile antes de fechar
}
```

**Impacto**: Quando o servidor decide fazer cleanup (timeout, erro), fecha a conexão sem avisar o mobile primeiro.

**Sintomas**:
- Desconexão abrupta sem aviso
- Mobile não pode mostrar mensagem apropriada
- Má experiência do usuário

---

### 🔴 7. Timeout de Inatividade Muito Longo

**Arquivo**: `main.go:2020`

```go
if time.Since(client.lastActivity) > 5*time.Minute {
```

**Impacto**: O sistema espera 5 minutos de inatividade antes de fazer cleanup. Se o Gemini parar de responder, o usuário fica 5 minutos em silêncio.

**Sintomas**:
- Sessões "mortas" permanecem ativas por muito tempo
- Recursos não são liberados rapidamente
- Usuário fica esperando sem saber o que fazer

---

## Correções Propostas

### Prioridade P0 (Crítico - Implementar Imediatamente)

#### 1. Notificar Mobile em Caso de Erro do Gemini

```go
func (s *SignalingServer) listenGemini(client *PCMClient) {
    for client.active.Load() {
        resp, err := client.GeminiClient.ReadResponse()
        if err != nil {
            if client.active.Load() {
                log.Printf("⚠️ Gemini read error: %v", err)
                // ✅ Notificar mobile
                s.sendJSON(client, map[string]interface{}{
                    "type": "error",
                    "code": "gemini_connection_lost",
                    "message": "Conexão com IA perdida. Reconectando...",
                })
                // ✅ Tentar reconectar
                if err := s.reconnectGemini(client, 3); err != nil {
                    s.sendJSON(client, map[string]interface{}{
                        "type": "error",
                        "code": "gemini_reconnect_failed",
                        "message": "Não foi possível reconectar. Por favor, reinicie a chamada.",
                    })
                }
            }
            return
        }
        s.processGeminiResponse(client, resp)
    }
}
```

#### 2. Reduzir ReadDeadline para 30 Segundos

```go
const readTimeout = 30 * time.Second  // Era: 5 * time.Minute
```

### Prioridade P1 (Alta - Implementar em Seguida)

#### 3. Notificar Mobile Antes de Cleanup

```go
func (s *SignalingServer) cleanupClient(client *PCMClient) {
    log.Printf("🧹 Cleanup: %s", client.CPF)
    
    // ✅ Notificar antes de fechar
    s.sendJSON(client, map[string]interface{}{
        "type": "session_ended",
        "reason": "cleanup",
        "message": "Sessão encerrada",
    })
    time.Sleep(100 * time.Millisecond)
    
    client.cancel()
    // ... resto do cleanup
}
```

#### 4. Melhorar Tratamento de Canal Cheio

```go
// Adicionar contador de drops
audioDropCount := atomic.AddInt64(&client.audioDropCount, 1)
if audioDropCount%10 == 0 {
    s.sendJSON(client, map[string]interface{}{
        "type": "warning",
        "code": "audio_buffer_full",
        "message": "Conexão lenta detectada",
    })
}
```

#### 5. Implementar Reconexão Automática

```go
func (s *SignalingServer) reconnectGemini(client *PCMClient, maxRetries int) error {
    backoff := 1 * time.Second
    for i := 0; i < maxRetries; i++ {
        if err := s.setupGeminiSession(client, "Aoede"); err == nil {
            s.sendJSON(client, map[string]interface{}{
                "type": "reconnected",
                "message": "Conexão restaurada",
            })
            return nil
        }
        time.Sleep(backoff)
        backoff *= 2
    }
    return fmt.Errorf("falha após %d tentativas", maxRetries)
}
```

### Prioridade P2 (Média - Melhorias)

#### 6. Implementar Heartbeat no Gemini WebSocket

```go
func (c *Client) StartHeartbeat(ctx context.Context) {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    
    c.conn.SetPongHandler(func(appData string) error {
        return c.conn.SetReadDeadline(time.Now().Add(readTimeout))
    })
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
                return
            }
        }
    }
}
```

#### 7. Adicionar Métricas e Logging Detalhado

```go
type Metrics struct {
    ConnectionErrors   int64
    ReadTimeouts       int64
    AudioChunksRecv    int64
    AudioDropped       int64
    LastActivityTime   time.Time
}
```

#### 8. Health Check Endpoint

```go
api.HandleFunc("/health/connections", func(w http.ResponseWriter, r *http.Request) {
    status := make(map[string]interface{})
    for cpf, client := range signalingServer.clients {
        status[cpf] = map[string]interface{}{
            "active": client.active.Load(),
            "gemini_connected": client.GeminiClient != nil,
            "last_activity": client.lastActivity,
        }
    }
    json.NewEncoder(w).Encode(status)
})
```

---

## Plano de Testes

### Testes Automatizados

1. **Teste de Timeout do Gemini**
   - Simular servidor que não responde
   - Verificar que timeout ocorre em 30s
   - Verificar que erro é propagado

2. **Teste de Canal Cheio**
   - Criar cliente com canal pequeno
   - Enviar mais áudio do que o buffer
   - Verificar notificação de warning

### Testes Manuais

1. **Simular Perda de Conexão Gemini**
   - Bloquear tráfego para `generativelanguage.googleapis.com`
   - Verificar mensagem "Reconectando..."
   - Verificar tentativas de reconexão

2. **Simular Rede Lenta**
   - Limitar banda para 10kbps
   - Verificar warning "Conexão lenta detectada"
   - Verificar que sistema continua funcionando

3. **Timeout de Inatividade**
   - Ficar em silêncio por 5 minutos
   - Verificar mensagem "Sessão encerrada"
   - Verificar fechamento gracioso

---

## Cronograma Estimado

- **P0 (Crítico)**: 2-3 horas
- **P1 (Alta)**: 4-6 horas
- **P2 (Média)**: 6-8 horas
- **Testes**: 4 horas
- **Total**: ~20 horas (2-3 dias)

---

## Monitoramento em Produção

### Comandos de Diagnóstico

```bash
# Verificar erros de conexão Gemini
journalctl -u eva-mind -f | grep "Gemini read error"

# Verificar drops de áudio
journalctl -u eva-mind -f | grep "Canal cheio"

# Verificar reconexões
journalctl -u eva-mind -f | grep "Tentativa de reconexão"

# Verificar sessões ativas
curl http://localhost:8080/api/health/connections
```

### Métricas a Monitorar

- Taxa de erros de conexão Gemini
- Frequência de drops de áudio
- Tempo médio de reconexão
- Número de sessões ativas
- Latência de resposta

---

## Conclusão

A auditoria identificou que as interrupções abruptas são causadas principalmente por **falhas silenciosas** em múltiplos pontos do sistema. As correções propostas focam em:

1. **Notificação proativa** - Avisar o usuário quando algo der errado
2. **Recuperação automática** - Tentar reconectar antes de desistir
3. **Timeouts adequados** - Detectar problemas rapidamente (30s vs 5min)
4. **Feedback contínuo** - Manter usuário informado do estado da conexão

Implementando as correções P0 e P1, espera-se reduzir drasticamente as interrupções abruptas e melhorar significativamente a experiência do usuário.
