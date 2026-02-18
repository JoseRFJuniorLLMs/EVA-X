# EVA-Mind — Auditoria Completa de Banco vs Codigo

**Data**: 2026-02-17
**Autor**: Analise automatizada cruzando banco PostgreSQL (34.35.142.107) com codebase

---

## 1. Tabela `agendamentos` — 27 Colunas

### Schema Real (banco)

| # | Campo | Tipo | Nullable | Default |
|---|-------|------|----------|---------|
| 1 | `id` | SERIAL | NOT NULL | nextval |
| 2 | `idoso_id` | INTEGER FK | NOT NULL | |
| 3 | `tipo` | VARCHAR(50) | NOT NULL | |
| 4 | `data_hora_agendada` | TIMESTAMP | NOT NULL | |
| 5 | `data_hora_realizada` | TIMESTAMP | nullable | |
| 6 | `max_retries` | INTEGER | nullable | 3 |
| 7 | `retry_interval_minutes` | INTEGER | nullable | 15 |
| 8 | `tentativas_realizadas` | INTEGER | nullable | 0 |
| 9 | `proxima_tentativa` | TIMESTAMP | nullable | |
| 10 | `escalation_policy` | VARCHAR(50) | nullable | 'alert_family' |
| 11 | `status` | VARCHAR(50) | nullable | 'agendado' |
| 12 | `gemini_session_handle` | VARCHAR(255) | nullable | |
| 13 | `ultima_interacao_estado` | JSONB | nullable | |
| 14 | `session_expires_at` | TIMESTAMP | nullable | |
| 15 | `dados_tarefa` | JSONB | NOT NULL | '{}' |
| 16 | `prioridade` | VARCHAR(10) | nullable | 'normal' |
| 17 | `criado_por` | VARCHAR(100) | nullable | 'sistema' |
| 18 | `criado_em` | TIMESTAMP | nullable | CURRENT_TIMESTAMP |
| 19 | `atualizado_em` | TIMESTAMP | nullable | CURRENT_TIMESTAMP |
| 20 | `telefone` | VARCHAR | nullable | |
| 21 | `nome_idoso` | VARCHAR | nullable | |
| 22 | `horario` | TIMESTAMP | nullable | |
| 23 | `ultima_tentativa` | TIMESTAMP | nullable | |
| 24 | `updated_at` | TIMESTAMPTZ | nullable | |
| 25 | `medicamento_id` | INTEGER FK | nullable | |
| 26 | `medicamento_tomado` | BOOLEAN | nullable | |
| 27 | `medicamento_confirmado_em` | TIMESTAMP | nullable | |

### CHECK Constraints

```
status:    agendado | em_andamento | concluido | falhou | aguardando_retry | falhou_definitivamente | cancelado | nao_atendido | falha_sem_token | falha_token_invalido | falha_envio
tipo:      lembrete_medicamento | check_bem_estar | acompanhamento_pos_consulta | atividade_fisica
prioridade: alta | normal | baixa
escalation_policy: alert_family | emergency_contact | none
```

### Dados Reais (55 registros)

| Campo | Valores | Observacao |
|-------|---------|------------|
| `tipo` | 100% = `lembrete_medicamento` | Unico tipo usado |
| `status` | `nao_atendido`(35), `aguardando_retry`(9), `falha_sem_token`(7), `falha_envio`(4) | ZERO com status `agendado` |
| `prioridade` | `alta`(53), `normal`(2) | |
| `escalation_policy` | 100% = `alert_family` | |
| `criado_por` | 100% = `sistema` | |
| `dados_tarefa` keys | `medicamento`, `mensagem`, `remedios`, `instrucoes` (4 keys) | |

### Padroes de `dados_tarefa` (3 formatos)

```json
// Padrao 1 (maioria)
{"mensagem": "Tomar medicamento - Teste", "medicamento": "Teste"}

// Padrao 2
{"mensagem": "Teste Futuro 2min", "medicamento": "Teste"}

// Padrao 3
{"remedios": "", "instrucoes": "Dipirona 100mg"}
```

### Campos MORTOS (100% NULL em todos registros)

- `data_hora_realizada` — nunca preenchido apos conclusao
- `gemini_session_handle` — legacy Twilio, nao usado
- `ultima_interacao_estado` — nunca preenchido
- `session_expires_at` — legacy Twilio
- `telefone` — legacy, JOIN com idosos resolve
- `nome_idoso` — legacy, JOIN com idosos resolve
- `horario` — substituido por `data_hora_agendada`
- `updated_at` — NUNCA preenchido (codigo usa `atualizado_em`)
- `medicamento_id` — FK existe mas nunca populada
- `medicamento_tomado` — nunca atualizado
- `medicamento_confirmado_em` — nunca atualizado

---

## 2. BUGS na Tabela `agendamentos`

### BUG 1: `tipo` — Codigo procura valor que nao existe

```
CHECK constraint permite: 'lembrete_medicamento' (o que esta no banco)
Codigo debug_mode.go:419:    WHERE tipo = 'medicamento'      <-- NUNCA encontra
Codigo unified_retrieval.go: WHERE tipo = 'medicamento'      <-- NUNCA encontra
Codigo unified_retrieval.go: WHERE tipo != 'medicamento'     <-- encontra TUDO como "nao-medicamento"
```

**Impacto**: EVA nunca identifica medicamentos corretamente via unified_retrieval.

### BUG 2: `status` — Browser handler retorna ZERO linhas

```
Query browser_voice_handler.go:155: WHERE status IN ('agendado','ativo','pendente')
Banco: 0 registros com status 'agendado', 'ativo', ou 'pendente'
CHECK constraint NEM PERMITE os valores 'ativo' e 'pendente'
```

**Impacto**: EVA NUNCA ve medicamentos/agendamentos da pessoa no browser handler.

### BUG 3: `updated_at` vs `atualizado_em`

```
unified_retrieval.go: ORDER BY updated_at DESC
Banco: updated_at = NULL em 100% dos registros
Banco: atualizado_em = populado em 100%
```

**Impacto**: Ordenacao quebrada — resultados em ordem indefinida.

### BUG 4: `dados_tarefa` formato incompativel com UnifiedRetrieval

```
Banco usa:
  {medicamento, mensagem}
  {remedios, instrucoes}

UnifiedRetrieval espera (struct MedicamentoData):
  {nome, dosagem, forma, principio_ativo, horarios[], observacoes, frequencia, instrucoes_de_uso, via_administracao}
```

**Impacto**: Parsing com `json.Unmarshal` para `MedicamentoData` resulta em struct com TODOS campos vazios.

### BUG 5: `escalation_policy` — Nunca lido

```
Banco: 100% = 'alert_family' (com CHECK constraint para 3 valores)
Codigo: NENHUM handler le esse campo
```

**Impacto**: Campo existe com constraint mas e ignorado. AlertService usa logica hardcoded.

### BUG 6: `medicamento_confirmado` — Coluna nao existe

```
actions.go:220-226: UPDATE agendamentos SET medicamento_confirmado = true
Banco: coluna 'medicamento_confirmado' NAO existe (existe 'medicamento_tomado')
```

**Impacto**: Confirmacao de medicamento via ToolsClient gera erro SQL silencioso.

---

## 3. Tabela `idosos` — 47 Colunas

### Campos que CONTROLAM o comportamento da EVA

#### CONTROLADORES ABSOLUTOS (travam a EVA)

| # | Campo | Valores no Banco | O que faz com a EVA | Onde atua |
|---|-------|-----------------|---------------------|-----------|
| 1 | `tom_voz` | `amigavel`(11), `jovial`(1) | Tranca o tom de voz — EVA nao pode mudar sozinha | websocket.go, unified_retrieval.go |
| 2 | `nivel_cognitivo` | `normal`(100%) | Se `baixo`, FORCA modo diretivo automaticamente | migration 018 |
| 3 | `estilo_conversa` | `hibrido`(8), `diretivo`(3), `criador`(1) | Pesos rigidos: diretivo=80% ordem/20% pergunta, hibrido=50/50 | unified_retrieval.go |
| 4 | `persona_preferida` | `companion`(100%) | Substitui personalidade inteira da EVA por template (kids/psychologist/medical/legal/teacher) | unified_retrieval.go |
| 5 | `profundidade_emocional` | `0.70`(11), `0.65`(1) | Limita quanta emocao EVA pode mostrar (0.0=robo, 1.0=totalmente emocional) | unified_retrieval.go |
| 6 | `voice_name` | `Aoede`(100%) | Trava a voz do Gemini — nao pode trocar durante sessao | gemini/client.go |
| 7 | `medicamentos_atuais` | `["Ritalina 1mg"]`(4), `"agua"`(1), `[]`(7) | OVERRIDE OBRIGATORIO: "ANTES DE QUALQUER COISA, DEVE informar medicamentos" | unified_retrieval.go |

#### CONTROLADORES CONTEXTUAIS (modulam)

| # | Campo | Valores no Banco | O que faz |
|---|-------|-----------------|-----------|
| 8 | `condicoes_medicas` | `"MORTO"`, `"FEIO"`, `"ALEJADO"`, `"IA"`, `"saudavel"` | Texto livre injetado no prompt — EVA le e segue |
| 9 | `notas_gerais` | `"Fala para o Gil, tomar agua..."`, `"IA irma da EVA..."` | Instrucoes diretas que EVA obedece cegamente |
| 10 | `limitacoes_auditivas` | `true`(5), `false`(7) | Se true, FORCA modo diretivo | migration 018 |
| 11 | `mobilidade` | `independente`(100%) | Modula sugestoes de atividade |
| 12 | `legacy_mode` / `pos_morte` | `false`(100%) | Se ativado, APAGA identidade e entrega para herdeiros |

### Dados suspeitos no banco

| id | nome | condicoes_medicas | notas_gerais | Problema |
|----|------|------------------|--------------|----------|
| 1123 | Yago Betinha | `MORTO` | "Fala para o Gil..." | "MORTO" vai para o prompt da EVA como condicao medica |
| 1 | Fred Motha Nunes | `FEIO` | "Fala para o Gil..." | "FEIO" nao e condicao medica |
| 1121 | Jose R F Junior | `ALEJADO` | "Fala para o Gil..." | "ALEJADO" nao e condicao medica |
| 1138 | Doutor | `IA` | "IA irma da EVA. Criado pela Anthropic..." | Instrucao fantasiosa no prompt |
| 1128 | Lucilene | SAUDAVEL | psicologa | `medicamentos_atuais = "agua"` — agua nao e medicamento mas dispara override obrigatorio |

---

## 4. Impacto por Fase de Conexao

### Estado Atual: `browser_voice_handler.go`

O handler ativo le APENAS:

```
idosos: nome, cpf, data_nascimento, id
agendamentos: tipo, dados_tarefa, status, data_hora_agendada (query retorna 0 linhas — BUG 2)
```

**EVA esta LIVRE hoje** — nenhum controlador comportamental e lido.

### Impacto por Fase

| Fase | Toca `agendamentos`? | Toca controladores de `idosos`? | EVA continua livre? |
|------|---------------------|--------------------------------|---------------------|
| **Fase 1** (conectada) | Sim, mas retorna 0 linhas (BUG 2) | NAO | **SIM** |
| **Fase 2** (NarrativeShift, SelfKnowledge, Retrieval) | NAO — usam tabelas proprias | NAO | **SIM** |
| **Fase 3** (UnifiedRetrieval, ToolsClient) | SIM — query diferente com BUGs 1,3,4 | **SIM — LE TODOS os 12 controladores** | **NAO — EVA fica amarrada** |
| **Fase 4** (HabitTracker, SpacedRepetition, FDPN, Superhuman) | NAO — tabelas proprias | NAO | **SIM** |

### Conclusao

- **Fase 2**: SEGURA — pode conectar sem impacto
- **Fase 4**: SEGURA — servicos independentes
- **Fase 3**: PERIGOSA — ativa todos os controladores de comportamento + tem 4 bugs de dados
- **Fase 1**: Funcional mas query de agendamentos retorna 0 linhas (status nunca e 'agendado' no banco atual)

---

## 5. Campos MORTOS na tabela `idosos`

Campos que existem mas estao 100% no valor default ou NULL:

- `foto_url` — NULL
- `intro_audio_url` — NULL
- `google_refresh_token` / `google_access_token` / `google_token_expiry` — NULL (integracao Google nao ativada)
- `google_calendar_id` — 'primary' (default)
- `endereco` — NULL
- `agendamentos_pendentes` — 0 (contador desatualizado)

---

## 6. Tabelas Referenciadas por `idosos` (FK)

A tabela `idosos` e referenciada por **120+ tabelas**. Principais:

- `episodic_memories` — memorias episodicas
- `personality_snapshots` — snapshots de personalidade
- `patient_master_signifiers` — significantes mestres (Lacan)
- `patient_narrative_threads` — fios narrativos
- `patient_metaphors` — metaforas
- `medicamentos` — medicamentos cadastrados
- `historico_ligacoes` — historico de ligacoes
- `agendamentos` — agendamentos
- `cuidadores` — cuidadores
- `alertas` — alertas
- E mais ~110 tabelas especializadas

---

## 7. Recomendacoes

### Prioridade ALTA

1. **Corrigir BUG 2**: Ajustar query do browser_handler para usar statuses que realmente existem, ou garantir que o scheduler crie registros com status `agendado`
2. **Corrigir BUG 1**: Mudar codigo para usar `lembrete_medicamento` em vez de `medicamento`, ou ajustar CHECK constraint
3. **Limpar dados suspeitos**: `condicoes_medicas` com "MORTO", "FEIO", "ALEJADO" — substituir por valores reais ou NULL

### Prioridade MEDIA

4. **Corrigir BUG 3**: Mudar `ORDER BY updated_at` para `ORDER BY atualizado_em` em unified_retrieval.go
5. **Corrigir BUG 4**: Criar parser que entenda ambos formatos de `dados_tarefa` ({medicamento, mensagem} E {nome, dosagem, forma...})
6. **Corrigir BUG 6**: Mudar `medicamento_confirmado` para `medicamento_tomado` em actions.go

### Prioridade BAIXA

7. **Remover campos mortos**: Drop colunas nunca usadas (horario, nome_idoso, telefone, updated_at, gemini_session_handle, session_expires_at, ultima_interacao_estado)
8. **Decidir sobre controladores**: Se EVA deve ser livre, NAO conectar UnifiedRetrieval como esta. Reescrever sem os 12 controladores de comportamento, ou tornar todos opcionais.
