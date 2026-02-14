# Plano de Mudança Crítica: Estabilização de Voz (Smart Suppression)

## 1. Motivo da Mudança (O Problema)
O sistema atual sofre de interrupções acidentais quando usado em **viva-voz**. O som da IA sai pelo alto-falante, volta pelo microfone e o Gemini (super sensível) acha que o usuário está interrompendo, parando a fala da EVA prematuramente.

## 2. A Solução (V5 - Smart Suppression)
Baseado no funcionamento da versão Web e nas melhores práticas da Google API, implementaremos uma abordagem cirúrgica:
- **Tuning de VAD:** Calibrar o servidor Gemini para ser menos sensível a ruídos baixos (eco).
- **Digital Ducking:** Reduzir o ganho do microfone em 70% apenas enquanto a EVA fala. Isso permite que o eco não dispare a interrupção, mas a voz real do usuário sim.
- **Interruption Sync:** Garantir que o comando `clear_buffer` seja processado instantaneamente ao detectar interrupção legítima.

## 3. Arquivos Afetados

### A. `internal/cortex/gemini/client.go`
- **O que muda:** Adição da configuração `automatic_activity_detection` no método `SendSetup`.
- **Objetivo:** Definir `start_of_speech_sensitivity` como `LOW`.

### B. `internal/senses/signaling/websocket.go`
- **O que muda:** 
  1. Adição de campo `State` na struct `WebSocketSession`.
  2. Implementação do loop de ducking de áudio na função `handleAudioMessage`.
  3. Sincronização de estado (Speaking/Listening) nas funções de resposta.
- **Objetivo:** Controlar o volume do microfone e rastrear se a IA está ativa.

## 4. Plano de Verificação
1. Iniciar conversa em viva-voz.
2. Deixar a EVA falar uma frase longa (ela não deve se interromper com o próprio som).
3. Interromper a EVA falando alto (ela deve parar e ouvir, como na versão Web).

## 5. Plano de Rollback (Voltar atrás)
Para desfazer tudo e voltar ao estado original, execute:
1. Remover o campo `input_config` do `setup` no `client.go`.
2. Remover o campo `State` e os métodos `SetState/GetState` no `websocket.go`.
3. Remover o loop de processamento de `pcmData` dentro do `handleAudioMessage` no `websocket.go`.

---
**Status:** PRONTO PARA EXECUÇÃO CIRÚRGICA.
