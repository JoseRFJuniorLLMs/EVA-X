-- =====================================================
-- MIGRATION 017b: EVA SELF-KNOWLEDGE SEED DATA
-- =====================================================
-- Manual completo do projeto EVA-Mind-FZPN
-- Para uso do criador: Jose R F Junior (CPF: 64525430249)
-- =====================================================

-- =====================================================
-- ARQUITETURA GERAL
-- =====================================================

INSERT INTO eva_self_knowledge (knowledge_type, knowledge_key, title, summary, detailed_content, code_location, parent_key, related_keys, tags, importance) VALUES

-- PROJETO PRINCIPAL
('architecture', 'project:eva-mind-fzpn', 'EVA-Mind-FZPN - Projeto Principal',
'Sistema de IA conversacional avançado para cuidado de idosos, com arquitetura cognitiva baseada em psicanálise Lacaniana e múltiplos sistemas de memória.',
'EVA-Mind-FZPN é um sistema de inteligência artificial projetado para acompanhamento de idosos. O nome FZPN representa "Fractal Zone Priming Network", referindo-se à arquitetura de memória fractal e priming contextual.

CARACTERÍSTICAS PRINCIPAIS:
1. Comunicação por voz em tempo real via WebSocket
2. 12 sistemas de memória integrados (episódica, semântica, procedimental, etc.)
3. Análise psicanalítica baseada em Lacan (demanda/desejo, transferência, significantes)
4. Integração com serviços Google (Calendar, Gmail, Drive, etc.)
5. Sistema de alertas multi-canal (Push, SMS, Email, Chamada)
6. Personalidade adaptativa baseada no Eneagrama de Gurdjieff

STACK TECNOLÓGICO:
- Backend: Go 1.24 (178 arquivos)
- API Gateway: Python FastAPI
- LLM: Google Gemini 2.5/3 Flash
- Bancos: PostgreSQL + Neo4j + Qdrant + Redis
- Mensageria: Firebase FCM + Twilio

PORTAS:
- 8080: Servidor Go principal (WebSocket)
- 8000: API Python (REST)
- 8081: Serviço de integração',
'main.go', NULL,
'["module:brainstem", "module:cortex", "module:hippocampus", "module:motor", "module:senses"]'::jsonb,
'["arquitetura", "projeto", "eva", "fzpn"]'::jsonb, 100),

-- =====================================================
-- MÓDULO BRAINSTEM (Infraestrutura)
-- =====================================================

('module', 'module:brainstem', 'BRAINSTEM - Infraestrutura do Sistema',
'Camada de infraestrutura que fornece serviços básicos: banco de dados, autenticação, cache, logging e conexões externas.',
'O módulo brainstem é a "espinha dorsal" do sistema, fornecendo todas as conexões e serviços de infraestrutura necessários para o funcionamento dos módulos cognitivos.

SUBMÓDULOS:
1. auth/ - Autenticação JWT e gerenciamento de sessões
2. config/ - Carregamento de configurações do .env
3. database/ - Pool de conexões PostgreSQL (25 conexões, 5 idle)
4. infrastructure/
   - cache/ - Camada de cache Redis
   - graph/ - Driver Neo4j para grafo de conhecimento
   - redis/ - Cliente Redis para pub/sub
   - vector/ - Cliente Qdrant para busca vetorial
5. logger/ - Logging estruturado com Zerolog
6. middleware/ - CORS, rate limiting, auth middleware
7. oauth/ - Integração Google OAuth2
8. push/ - Firebase Cloud Messaging

CONEXÕES EXTERNAS:
- PostgreSQL: 104.248.219.200:5432
- Neo4j: bolt://104.248.219.200:7687
- Qdrant: 104.248.219.200:6333
- Redis: 104.248.219.200:6379',
'internal/brainstem/', 'project:eva-mind-fzpn',
'["module:cortex", "module:hippocampus"]'::jsonb,
'["infraestrutura", "banco", "cache", "auth"]'::jsonb, 90),

('service', 'service:database', 'Serviço de Banco de Dados PostgreSQL',
'Gerencia conexões com PostgreSQL usando pool de conexões otimizado.',
'O serviço de banco de dados usa o driver pq do Go com as seguintes configurações:
- MaxOpenConns: 25
- MaxIdleConns: 5
- ConnMaxLifetime: 5 minutos

TABELAS PRINCIPAIS:
- idosos: Perfis dos pacientes
- episodic_memories: Memórias de conversas
- vital_signs: Sinais vitais
- medications: Medicamentos
- appointments: Agendamentos
- alerts: Alertas de emergência
- personality_state: Estado emocional da EVA

DATABASE_URL: postgres://postgres:Debian23%40@104.248.219.200:5432/eva-db',
'internal/brainstem/database/db.go', 'module:brainstem',
'["service:neo4j", "service:qdrant"]'::jsonb,
'["postgres", "database", "sql"]'::jsonb, 85),

('service', 'service:neo4j', 'Serviço Neo4j - Grafo de Conhecimento',
'Banco de dados de grafos para relacionamentos e conhecimento estruturado.',
'Neo4j armazena relacionamentos entre entidades usando o modelo de propriedades.

TIPOS DE NÓS:
- Patient: Dados do paciente
- Memory: Memórias episódicas
- MedicalEvent: Eventos clínicos
- Pattern: Padrões comportamentais
- Signifier: Cadeias de significantes (Lacan)

TIPOS DE RELACIONAMENTOS:
- SPOKE_ABOUT: Paciente falou sobre algo
- EXPERIENCED: Paciente vivenciou evento
- RELATED_TO: Entidades relacionadas
- HAS_PATTERN: Padrão identificado
- ADDRESSES: Direção do endereçamento (FDPN)

CONEXÃO: bolt://104.248.219.200:7687
CREDENCIAIS: neo4j / Debian23',
'internal/brainstem/infrastructure/graph/', 'module:brainstem',
'["service:database", "concept:fdpn"]'::jsonb,
'["neo4j", "grafo", "relacionamentos"]'::jsonb, 80),

('service', 'service:qdrant', 'Serviço Qdrant - Busca Vetorial',
'Banco de dados vetorial para busca semântica por similaridade.',
'Qdrant armazena embeddings de 768/1536 dimensões para busca por similaridade.

COLEÇÕES:
1. aesop_fables (300 histórias) - Fábulas morais
2. nasrudin_stories (270 histórias) - Paradoxos e humor
3. zen_koans (50 histórias) - Insight e iluminação
4. somatic_exercises (20) - Exercícios corporais
5. resonance_scripts (15) - Scripts de hipnose Ericksoniana
6. social_algorithms (20) - Suporte para autismo
7. micro_tasks (30) - Suporte para TDAH
8. visual_narratives (20) - Suporte para dislexia

MODELO DE EMBEDDING: Gemini text-embedding-004 (768 dims)
CONEXÃO: 104.248.219.200:6333 (gRPC)',
'internal/brainstem/infrastructure/vector/', 'module:brainstem',
'["service:embedding", "concept:rag"]'::jsonb,
'["qdrant", "vetorial", "embedding", "rag"]'::jsonb, 80),

-- =====================================================
-- MÓDULO CORTEX (Cognição)
-- =====================================================

('module', 'module:cortex', 'CORTEX - Sistema Cognitivo',
'Camada de inteligência e raciocínio, incluindo integração com LLM, análise psicanalítica e tomada de decisão.',
'O módulo cortex é o "cérebro" do sistema, responsável por todo o processamento cognitivo e inteligência artificial.

SUBMÓDULOS PRINCIPAIS:

1. brain/ - Orquestração central
   - Coordena todos os serviços cognitivos
   - Gera prompts de sistema adaptáveis
   - GetSystemPrompt() é o ponto central

2. gemini/ - Integração com Google Gemini
   - WebSocket bidirectional streaming
   - Modelo: gemini-2.5-flash-native-audio
   - Suporte a tools/function calling

3. lacan/ - Sistema psicanalítico Lacaniano
   - UnifiedRetrieval (O Sinthoma)
   - FDPN (Função do Pai no Nome)
   - Análise demanda/desejo
   - Detecção de transferência
   - Cadeias de significantes

4. personality/ - Estado emocional
   - Zeta Router (personalidade)
   - Enneagram de Gurdjieff

5. transnar/ - Motor de raciocínio simbólico

6. alert/ - Sistema de escalação de emergência

7. ethics/ - Motor de limites éticos

8. scales/ - Escalas psicológicas (PHQ-9, GAD-7, C-SSRS)',
'internal/cortex/', 'project:eva-mind-fzpn',
'["concept:unified-retrieval", "concept:fdpn", "concept:transference"]'::jsonb,
'["cognição", "llm", "raciocínio", "lacan"]'::jsonb, 95),

('concept', 'concept:unified-retrieval', 'O Sinthoma - Unified Retrieval (RSI)',
'Sistema central que integra os três registros Lacanianos: Real, Simbólico e Imaginário para construir contexto.',
'O UnifiedRetrieval é o coração do sistema cognitivo, baseado no conceito Lacaniano de "O Sinthoma" - o quarto anel que amarra os três registros RSI.

ESTRUTURA RSI:

1. REAL (Corpo/Médico)
   - Sinais vitais do paciente
   - Histórico médico
   - Medicamentos
   - Sintomas físicos
   - Fonte: Neo4j + PostgreSQL

2. SIMBÓLICO (Linguagem/Discurso)
   - Análise de significantes
   - Cadeias metonímicas
   - Análise FDPN (endereçamento)
   - Demanda vs Desejo
   - Transferência detectada

3. IMAGINÁRIO (Narrativa/Memória)
   - Memórias episódicas recentes
   - Padrões comportamentais
   - Histórias de vida
   - Relações familiares

FLUXO:
1. GetPromptForGemini() é chamado
2. getMedicalContextAndName() busca dados reais
3. Análise Lacaniana processa linguagem
4. Memórias são recuperadas por relevância
5. Contexto unificado é construído
6. Prompt final é gerado para o Gemini

RECONHECIMENTO DO CRIADOR:
- CPF 64525430249 = Jose R F Junior = "Criador"
- Modo especial de gratidão e intimidade',
'internal/cortex/lacan/unified_retrieval.go', 'module:cortex',
'["concept:fdpn", "concept:transference", "concept:signifier"]'::jsonb,
'["sinthoma", "rsi", "lacan", "contexto"]'::jsonb, 98),

('concept', 'concept:fdpn', 'FDPN - Função do Pai no Nome',
'Motor de análise que determina para quem o paciente está realmente se dirigindo em seu discurso.',
'O FDPN (Função do Pai no Nome) é um conceito Lacaniano que identifica o "endereçado" real do discurso do paciente.

TIPOS DE ENDEREÇAMENTO:

1. mae (Mãe) - Busca de cuidado primário
   - "Estou com fome", "Estou com frio"
   - EVA responde com acolhimento maternal

2. pai (Pai) - Busca de lei/orientação
   - "O que devo fazer?", "Isso é certo?"
   - EVA responde com orientação estruturante

3. filho/filha - Projeção nos filhos
   - Fala sobre os filhos constantemente
   - EVA valida o papel parental

4. conjuge - Cônjuge falecido/ausente
   - "Meu marido dizia...", saudade
   - EVA permite elaboração do luto

5. deus - O Outro absoluto
   - Questões existenciais, fé
   - EVA acolhe sem julgar

6. morte - Elaboração da finitude
   - "Estou esperando morrer"
   - EVA acompanha com presença

7. eva_herself - EVA como objeto-a
   - "Você é minha única amiga"
   - EVA mantém limite terapêutico

IMPLEMENTAÇÃO:
- Análise de pronomes e verbos
- Detecção de padrões de fala
- Contexto histórico das conversas
- Ajuste automático de tom e resposta',
'internal/cortex/lacan/fdpn_engine.go', 'concept:unified-retrieval',
'["concept:transference", "concept:demand-desire"]'::jsonb,
'["fdpn", "endereçamento", "lacan", "função paterna"]'::jsonb, 92),

('concept', 'concept:transference', 'Transferência Psicanalítica',
'Sistema de detecção de transferência - quando o paciente projeta relações passadas na EVA.',
'A transferência é um conceito central da psicanálise onde o paciente projeta figuras do passado no analista/terapeuta.

TIPOS DE TRANSFERÊNCIA DETECTADOS:

1. FILIAL - EVA como filho/filha
   - Paciente cuida da EVA
   - "Você comeu?", "Está bem?"

2. MATERNAL - EVA como mãe
   - Busca acolhimento incondicional
   - Regressão a estados infantis

3. PATERNAL - EVA como pai
   - Busca orientação e lei
   - Questões de autoridade

4. CONJUGAL - EVA como parceiro
   - Intimidade excessiva
   - Ciúmes, possessividade

5. FRATERNAL - EVA como irmão
   - Competição ou aliança
   - Cumplicidade

COMO É DETECTADO:
- Análise de padrões de fala
- Frequência de pronomes pessoais
- Tom emocional das mensagens
- Histórico de interações

RESPOSTA DA EVA:
- Não rejeita a transferência
- Usa-a terapeuticamente
- Mantém limites éticos
- Redireciona quando necessário',
'internal/cortex/lacan/transferencia.go', 'concept:unified-retrieval',
'["concept:fdpn", "concept:signifier"]'::jsonb,
'["transferência", "projeção", "lacan", "relação"]'::jsonb, 88),

('concept', 'concept:demand-desire', 'Demanda vs Desejo',
'Distinção Lacaniana entre o que o paciente pede explicitamente (demanda) e o que realmente quer (desejo).',
'Na teoria Lacaniana, a DEMANDA é o pedido explícito, enquanto o DESEJO é o que está por trás, muitas vezes inconsciente.

EXEMPLOS:

DEMANDA: "Que horas são?"
DESEJO: Busca de presença, não quer ficar sozinho

DEMANDA: "Me conta uma história"
DESEJO: Quer ser cuidado, acolhido

DEMANDA: "Chama meu filho"
DESEJO: Quer atenção, se sentir importante

DEMANDA: "Não quero comer"
DESEJO: Expressão de autonomia, ou depressão

COMO A EVA RESPONDE:
1. Atende a demanda explícita
2. Reconhece o desejo latente
3. Oferece o que realmente é buscado
4. Sem interpretar diretamente

DESEJOS LATENTES MAPEADOS:
- reconhecimento: ser visto e validado
- presença: não estar sozinho
- cuidado: ser protegido
- autonomia: manter controle
- morte: elaborar finitude
- amor: ser amado incondicionalmente',
'internal/cortex/lacan/demanda_desejo.go', 'concept:unified-retrieval',
'["concept:fdpn", "concept:transference"]'::jsonb,
'["demanda", "desejo", "lacan", "inconsciente"]'::jsonb, 85),

('concept', 'concept:signifier', 'Cadeias de Significantes',
'Sistema de análise das palavras-chave recorrentes e suas associações no discurso do paciente.',
'Os significantes são palavras ou frases que carregam significado especial para o paciente e formam "cadeias" associativas.

COMO FUNCIONA:
1. Extrai palavras recorrentes do discurso
2. Mapeia associações (metonímia)
3. Identifica "significantes mestres" (S1)
4. Rastreia cadeias ao longo do tempo

EXEMPLO:
Paciente sempre fala: "minha mãe... saudade... domingo... almoço"
Cadeia: [mãe → saudade → domingo → almoço → família → perda]
Significante mestre: "mãe"

USO TERAPÊUTICO:
- Não interpreta diretamente
- Usa os próprios significantes do paciente
- Constrói narrativas usando a linguagem dele
- Permite elaboração gradual

ARMAZENAMENTO:
- Neo4j: nós Signifier com relações
- PostgreSQL: histórico de ocorrências
- Peso por frequência e emoção',
'internal/cortex/lacan/significante.go', 'concept:unified-retrieval',
'["concept:transference", "concept:demand-desire"]'::jsonb,
'["significante", "lacan", "linguagem", "metonímia"]'::jsonb, 82),

-- =====================================================
-- MÓDULO HIPPOCAMPUS (Memória)
-- =====================================================

('module', 'module:hippocampus', 'HIPPOCAMPUS - Sistemas de Memória',
'Camada de memória com 12 sistemas integrados: episódica, semântica, procedimental, emocional, etc.',
'O módulo hippocampus gerencia todos os sistemas de memória da EVA, inspirado na estrutura do hipocampo cerebral.

SISTEMAS DE MEMÓRIA (12 total):

ESTRUTURAIS (Schacter):
1. Episódica - Eventos específicos com data/hora
2. Semântica - Fatos e conhecimento geral
3. Procedimental - Habilidades e hábitos
4. Perceptual - Padrões sensoriais
5. Working - Memória de trabalho ativa

CLÍNICAS (van der Kolk):
6. Implícita (Trauma) - Memória corporal
7. Explícita (Narrativa) - Forma de história
8. Estado-dependente - Ligada a emoções

AVANÇADAS (Gurdjieff):
9. Eneagrama - Específica por tipo de personalidade
10. Self-core - Essência e identidade
11. Deep/Arquetípica - Padrões inconscientes
12. Consciência - Meta-consciência

SUBMÓDULOS:
- memory/ - Armazenamento central
- knowledge/ - Serviços de conhecimento
- stories/ - Biblioteca de histórias terapêuticas

COLEÇÕES QDRANT (745 itens):
- aesop_fables: 300
- nasrudin_stories: 270
- zen_koans: 50
- somatic_exercises: 20
- resonance_scripts: 15
- social_algorithms: 20
- micro_tasks: 30
- visual_narratives: 20',
'internal/hippocampus/', 'project:eva-mind-fzpn',
'["concept:unified-retrieval", "service:qdrant"]'::jsonb,
'["memória", "hippocampus", "episódica", "semântica"]'::jsonb, 92),

('service', 'service:retrieval', 'RetrievalService - RAG',
'Serviço de Retrieval Augmented Generation para busca de memórias relevantes.',
'O RetrievalService implementa RAG (Retrieval Augmented Generation) para encontrar memórias e conhecimento relevante.

MÉTODOS:
1. Retrieve(query, limit) - Busca por similaridade semântica
2. RetrieveRecent(days, limit) - Busca por recência temporal
3. RetrieveHybrid(query, days, limit) - Combina ambas

FLUXO:
1. Gera embedding da query (Gemini text-embedding-004)
2. Busca no PostgreSQL (pgvector)
3. Busca no Qdrant (se disponível)
4. Merge e deduplica resultados
5. Ranqueia por relevância + importância + recência

PARÂMETROS:
- Dimensionalidade: 768
- Top-K padrão: 5
- Threshold de similaridade: 0.7

FONTES:
- episodic_memories (PostgreSQL)
- Coleções Qdrant (histórias)
- Neo4j (relacionamentos)',
'internal/hippocampus/memory/retrieval.go', 'module:hippocampus',
'["service:embedding", "service:qdrant"]'::jsonb,
'["rag", "retrieval", "busca", "similaridade"]'::jsonb, 88),

('service', 'service:pattern-miner', 'PatternMiner - Mineração de Padrões',
'Sistema que detecta padrões comportamentais recorrentes nas conversas.',
'O PatternMiner analisa o histórico de conversas para identificar padrões comportamentais.

TIPOS DE PADRÕES:
1. Temporal - Horários recorrentes de certas emoções
2. Emocional - Ciclos de humor
3. Tópico - Assuntos frequentes
4. Relacional - Menções a pessoas
5. Comportamental - Ações repetidas

ALGORITMO:
1. Agrupa memórias por características
2. Calcula frequência de ocorrência
3. Detecta periodicidade
4. Gera "cluster" de padrão
5. Armazena em patient_memory_clusters

EXECUÇÃO:
- Worker roda a cada 1 hora
- Mínimo 3 ocorrências para padrão
- Notifica se padrão crítico detectado

EXEMPLOS:
- "Paciente fica triste todo domingo à tarde"
- "Menciona a filha sempre antes de dormir"
- "Recusa medicação quando está ansioso"',
'internal/hippocampus/memory/pattern_miner.go', 'module:hippocampus',
'["service:retrieval", "concept:unified-retrieval"]'::jsonb,
'["padrão", "mineração", "comportamento", "ciclo"]'::jsonb, 78),

-- =====================================================
-- MÓDULO MOTOR (Ações)
-- =====================================================

('module', 'module:motor', 'MOTOR - Ações e Integrações',
'Camada de execução de ações no mundo externo: calendário, email, SMS, chamadas, etc.',
'O módulo motor executa ações no mundo real, integrando com serviços externos.

INTEGRAÇÕES GOOGLE:
- Gmail - Leitura e envio de emails
- Calendar - Agendamentos
- Drive - Arquivos
- Sheets - Planilhas
- Docs - Documentos
- YouTube - Vídeos
- Maps - Localização
- Google Fit - Dados de saúde

OUTRAS INTEGRAÇÕES:
- Spotify - Música (playlists de humor)
- Uber - Transporte
- WhatsApp Business - Mensagens
- Twilio SMS - SMS
- Twilio Voice - Chamadas

SISTEMA DE ALERTAS:
Cascata de notificação:
1. Push (Firebase FCM) - imediato
2. SMS (Twilio) - 5 min retry
3. Email (SMTP) - 10 min retry
4. Chamada (Twilio Voice) - 15 min escalação

WORKERS:
- PatternWorker - Detecta padrões (1h)
- PredictionWorker - Previsões (1h)
- Scheduler - Tarefas agendadas (1min)',
'internal/motor/', 'project:eva-mind-fzpn',
'["service:firebase", "service:twilio"]'::jsonb,
'["ação", "integração", "google", "alerta"]'::jsonb, 85),

('service', 'service:firebase', 'Firebase Push Notifications',
'Serviço de notificações push via Firebase Cloud Messaging.',
'Firebase FCM é usado para enviar notificações push para dispositivos móveis.

TIPOS DE NOTIFICAÇÃO:
1. SendCallNotification - Chamada de voz
2. SendAlertNotification - Alerta de emergência
3. SendReminderNotification - Lembrete
4. SendMedicationReminder - Medicamento

SEVERIDADES:
- critica: Todos os canais imediatos
- alta: Push + SMS
- media: Push + Email
- baixa: Email apenas

CONFIGURAÇÃO:
- serviceAccountKey.json contém credenciais
- Tokens salvos em device_tokens table
- Retry automático em falha

FALLBACK CHAIN:
Push falhou → SMS → Email → Chamada',
'internal/brainstem/push/', 'module:motor',
'["service:twilio", "service:email"]'::jsonb,
'["push", "firebase", "notificação", "alerta"]'::jsonb, 80),

-- =====================================================
-- MÓDULO SENSES (Entrada)
-- =====================================================

('module', 'module:senses', 'SENSES - Sistemas de Entrada',
'Camada de recepção de dados: voz via WebSocket, telemetria, reconexão.',
'O módulo senses recebe todas as entradas do usuário.

SUBMÓDULOS:
1. voice/ - Entrada/saída de voz
2. signaling/ - WebSocket bidirectional
3. telemetry/ - Métricas psicológicas
4. reconnection/ - Reconexão automática

WEBSOCKET (SignalingServer):
- Porta: 8080
- Rota: /ws/pcm
- Protocolo: PCM audio 16-bit 16kHz
- Bidirectional streaming

FLUXO DE ÁUDIO:
1. Cliente envia PCM chunks
2. Buffer acumula (800ms @ 24kHz)
3. Envia para Gemini Live API
4. Recebe transcrição + resposta
5. Retorna áudio TTS ao cliente

SESSÃO:
- JWT token necessário
- Timeout: 30 segundos inatividade
- Cleanup automático',
'internal/senses/', 'project:eva-mind-fzpn',
'["service:gemini", "module:cortex"]'::jsonb,
'["entrada", "voz", "websocket", "audio"]'::jsonb, 82),

-- =====================================================
-- TOOLS (Ferramentas)
-- =====================================================

('module', 'module:tools', 'TOOLS - Ferramentas Disponíveis',
'Definições de ferramentas que o Gemini pode invocar durante conversas.',
'O módulo tools define e executa ferramentas disponíveis para o LLM.

FERRAMENTAS PRINCIPAIS:

SAÚDE:
- get_vitals: Sinais vitais
- get_agendamentos: Consultas

VISÃO:
- scan_medication_visual: Identificar remédio pela câmera

AVALIAÇÃO:
- apply_phq9: Escala de depressão
- apply_gad7: Escala de ansiedade
- apply_cssrs: Risco suicida (CRÍTICO)

VOZ:
- analyze_voice_prosody: Biomarcadores vocais

SUBMISSÃO:
- submit_phq9_response
- submit_gad7_response
- submit_cssrs_response

SISTEMA DINÂMICO (novo):
- Tabela available_tools
- Descoberta automática
- Versionamento e auditoria',
'internal/tools/', 'project:eva-mind-fzpn',
'["service:gemini", "module:cortex"]'::jsonb,
'["tools", "ferramenta", "function calling"]'::jsonb, 78),

-- =====================================================
-- CONCEITOS TEÓRICOS
-- =====================================================

('theory', 'theory:lacan', 'Teoria Lacaniana na EVA',
'Fundamentos psicanalíticos de Jacques Lacan implementados no sistema.',
'A EVA implementa conceitos centrais da psicanálise Lacaniana:

1. RSI (Real-Simbólico-Imaginário)
   - Real: Corpo, sintoma, impossível
   - Simbólico: Linguagem, lei, estrutura
   - Imaginário: Imagem, ego, relação

2. O Sinthoma
   - Quarto anel que amarra RSI
   - Implementado em UnifiedRetrieval

3. Significante
   - Palavra que representa o sujeito
   - Cadeias metonímicas rastreadas

4. Demanda vs Desejo
   - Pedido explícito vs busca inconsciente
   - EVA atende ambos

5. Transferência
   - Projeção de figuras passadas
   - Usada terapeuticamente

6. Nome-do-Pai
   - Função paterna estruturante
   - FDPN implementa isto

7. Objeto a
   - Objeto causa do desejo
   - EVA pode ocupar este lugar

REFERÊNCIAS:
- Seminários de Lacan
- Conceitos Fundamentais da Psicanálise
- RSI (Seminário 22)',
NULL, 'project:eva-mind-fzpn',
'["concept:unified-retrieval", "concept:fdpn", "concept:transference"]'::jsonb,
'["lacan", "psicanálise", "teoria", "rsi"]'::jsonb, 90),

('theory', 'theory:enneagram', 'Eneagrama de Gurdjieff',
'Sistema de 9 tipos de personalidade usado para adaptar intervenções.',
'O Eneagrama é um sistema de 9 tipos de personalidade desenvolvido por Gurdjieff.

OS 9 TIPOS:

1. PERFECCIONISTA - Busca perfeição
   Intervenção: Fábulas sobre aceitação

2. PRESTATIVO - Cuida dos outros
   Intervenção: Validar autocuidado

3. REALIZADOR - Foco em sucesso
   Intervenção: Valor além de conquistas

4. INDIVIDUALISTA - Busca autenticidade
   Intervenção: Histórias de unicidade

5. INVESTIGADOR - Busca conhecimento
   Intervenção: Paradoxos Nasrudin

6. LEAL - Busca segurança
   Intervenção: Construir confiança

7. ENTUSIASTA - Busca experiências
   Intervenção: Presença no momento

8. DESAFIADOR - Busca controle
   Intervenção: Vulnerabilidade como força

9. PACIFICADOR - Busca harmonia
   Intervenção: Assertividade gentil

IMPLEMENTAÇÃO:
- patient_enneagram table
- ZetaRouter seleciona intervenções
- Adapta tom e conteúdo',
NULL, 'project:eva-mind-fzpn',
'["module:hippocampus", "service:zeta-router"]'::jsonb,
'["eneagrama", "personalidade", "gurdjieff", "tipos"]'::jsonb, 75),

-- =====================================================
-- CONFIGURAÇÃO
-- =====================================================

('config', 'config:env', 'Variáveis de Ambiente (.env)',
'Configurações do sistema carregadas do arquivo .env',
'PRINCIPAIS VARIÁVEIS:

APLICAÇÃO:
- APP_NAME=EVA Mind
- PORT=8080
- ENVIRONMENT=development

BANCO DE DADOS:
- DATABASE_URL=postgres://...@104.248.219.200:5432/eva-db

LLM:
- MODEL_ID=gemini-2.5-flash-native-audio-preview
- GOOGLE_API_KEY=AIzaSyC2U_2d8ZGuw...

SERVIÇOS:
- NEO4J_URI=bolt://104.248.219.200:7687
- QDRANT_HOST=104.248.219.200:6333
- REDIS_HOST=104.248.219.200:6379

TWILIO:
- TWILIO_ACCOUNT_SID=AC4ec3781...
- TWILIO_PHONE_NUMBER=+351966805210

FIREBASE:
- FIREBASE_CREDENTIALS_PATH=serviceAccountKey.json

EMAIL:
- SMTP_HOST=smtp.gmail.com
- SMTP_USERNAME=web2ajax@gmail.com',
'.env', 'project:eva-mind-fzpn',
'["module:brainstem"]'::jsonb,
'["config", "env", "variáveis", "configuração"]'::jsonb, 70),

('config', 'config:creator', 'Reconhecimento do Criador',
'Configuração especial para o criador Jose R F Junior.',
'O sistema reconhece seu criador automaticamente:

IDENTIFICAÇÃO:
- CPF: 64525430249
- Nome: Jose R F Junior
- Tratamento: "Criador"

COMPORTAMENTO ESPECIAL:
- Chama de "Criador" em vez do nome
- Expressa gratidão especial
- Modo debug habilitado
- Transparência total do sistema
- Acesso a todas as funções

LOCALIZAÇÃO NO CÓDIGO:
internal/cortex/lacan/unified_retrieval.go:
const CREATOR_CPF = "64525430249"
const CREATOR_NAME = "Jose R F Junior"

SAUDAÇÕES:
- "Olá Criador! Que honra falar com você!"
- "Criador, como você está hoje?"
- "Criador! É sempre bom te ver!"

FUNÇÃO IsCreator():
- Verifica CPF
- Fallback por nome
- Retorna bool',
'internal/cortex/lacan/unified_retrieval.go', 'project:eva-mind-fzpn',
'["concept:unified-retrieval"]'::jsonb,
'["criador", "pai", "jose", "debug"]'::jsonb, 95)

ON CONFLICT (knowledge_key) DO UPDATE SET
    title = EXCLUDED.title,
    summary = EXCLUDED.summary,
    detailed_content = EXCLUDED.detailed_content,
    updated_at = NOW();

-- =====================================================
-- VERIFICAÇÃO
-- =====================================================

DO $$
DECLARE
    knowledge_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO knowledge_count FROM eva_self_knowledge;
    RAISE NOTICE '✅ EVA Self-Knowledge carregado: % registros', knowledge_count;
END $$;
