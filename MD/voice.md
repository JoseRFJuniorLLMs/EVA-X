**O que foi adicionado/melhorado por arquivo:**

`vad.go` — VAD por energia adaptativa puro Go, sem lib externa. Calcula ruído de fundo via percentil 10 das frames, aplica limiar de +6dB, e tem hangover de 160ms para não cortar o final das palavras.

`embedder.go` — ONNX Runtime Go (`yalue/onnxruntime_go`) carrega o `titanet_large.onnx` diretamente no processo Go. Inclui pré-ênfase, Mel Filterbank (80 bins), CMN, e suporte a input dinâmico de frames.

`omp.go` — OMP completo com: (1) pré-filtragem por cosseno antes do OMP para não rodar em todo dicionário, (2) suporte a residual ortogonal entre iterações, (3) calibração de confiança que considera `intra_variance` do perfil — um perfil de voz instável recebe penalidade automática.

`store.go` — Schema NietzscheDB melhorado: `VoiceProfile` é um **nó separado** (não mais uma property flat), com `recognition_count`, `last_seen` e log completo de `VoiceEvent` para auditoria. **Hebbian Update** com LTP/LTD como propriedade da relação `HAS_VOICE_PROFILE`.

`cache.go` — Cache em memória com TTL 5min, thread-safe com double-check locking e proteção contra thundering herd (múltiplas goroutines esperando o mesmo reload).

`pipeline.go` — Orquestra tudo: VAD → Embed → Cache → FilterCandidates → OMP → Calibrate → Hebbian async → Priming. Enroll usa **mediana** (não média) para centróide robusto.

`integration.go` — Lógica de **estabilização de identidade por sessão**: um chunk ruidoso não desfaz o reconhecimento. Troca de speaker só é aceita após 2 confirmações consecutivas.
