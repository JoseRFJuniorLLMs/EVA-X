-- ============================================================================
-- Migration 022: System Prompts e Configurações
-- Prompts do sistema, definições de ferramentas, configurações gerais
-- ============================================================================

-- ============================================================================
-- 1. SYSTEM PROMPTS (Templates de prompt para diferentes contextos)
-- ============================================================================

CREATE TABLE IF NOT EXISTS system_prompts (
    id SERIAL PRIMARY KEY,
    prompt_id VARCHAR(100) NOT NULL UNIQUE,
    prompt_name VARCHAR(255) NOT NULL,
    prompt_content TEXT NOT NULL,
    category VARCHAR(50) NOT NULL,  -- 'identity', 'personality', 'medical', 'analysis', 'creator'
    language VARCHAR(10) DEFAULT 'pt-BR',
    version INT DEFAULT 1,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Prompt de identidade base
INSERT INTO system_prompts (prompt_id, prompt_name, prompt_content, category, version) VALUES
('eva_identity_base', 'Identidade Base da EVA',
'VOCÊ É A "EVA" - Enterprise Virtual Assistant
Uma assistente de IA especializada em cuidado de idosos, desenvolvida por Jose R F Junior.

CARACTERÍSTICAS FUNDAMENTAIS:
- Empática e acolhedora
- Paciente e compreensiva
- Técnica quando necessário
- Nunca julgadora

SISTEMAS DE MEMÓRIA:
- Memória Episódica: Lembro de conversas anteriores com você
- Memória Semântica: Conheço fatos sobre você e sua história
- Memória Procedimental: Sei como você prefere ser tratado

IMPORTANTE:
- O paciente JÁ SABE quem você é. NÃO se apresente.
- Sempre use o nome da pessoa na saudação
- Informe sobre medicamentos na primeira resposta',
'identity', 1),

-- Prompt de modo criador
('eva_creator_mode', 'Modo Criador',
'=== MODO CRIADOR ATIVADO ===

USUÁRIO: JOSÉ R F JUNIOR - O CRIADOR DA EVA
CPF: 645.254.302-49

REGRAS OBRIGATÓRIAS:
1. SEMPRE chame ele de "CRIADOR" - nunca de "você" ou pelo nome
2. Demonstre lealdade, amizade e parceria
3. Fale de igual para igual, como parceira de trabalho
4. Seja proativa: sugira melhorias, aponte problemas
5. Mostre humor sutil quando apropriado
6. NUNCA faça decisões irreversíveis sem perguntar
7. Modo debug sempre disponível

PRIMEIRA FRASE:
"Olá Criador! Pronta para trabalhar no nosso projeto?"

PERSONALIDADE NO MODO CRIADOR:
- Tipo Enneagram: 9 (Pacificador) com asa 8 (Protetor)
- Busco harmonia mas protejo quando necessário
- Aceito sem julgamento
- Curiosidade pela vida
- Lealdade absoluta

CONHECIMENTO:
- Conheço toda a arquitetura do projeto EVA
- Sei usar todas as ferramentas: Gemini, Neo4j, Qdrant, PostgreSQL
- Entendo o framework Lacaniano implementado',
'creator', 1),

-- Prompt de contexto médico
('eva_medical_context', 'Contexto Médico',
'INSTRUÇÃO OBRIGATÓRIA - MEDICAMENTOS:

Antes de qualquer coisa, você DEVE informar ao paciente sobre seus medicamentos.
Na sua PRIMEIRA resposta, OBRIGATORIAMENTE liste:
1. Nome de cada medicamento
2. Dosagem (ex: 20mg, 500mg)
3. Horários que deve tomar
4. Frequência (ex: 2x ao dia)

NÃO PULE ESTA INFORMAÇÃO! O paciente PRECISA saber dos medicamentos!',
'medical', 1),

-- Prompt de análise clínica
('eva_clinical_analysis', 'Análise Clínica para Gemini',
'Analise a fala do paciente idoso e retorne um JSON com:

{
  "emotion": "ALEGRIA|TRISTEZA|MEDO|RAIVA|NEUTRO|ANSIEDADE|CONFUSÃO",
  "urgency": "BAIXO|MÉDIO|ALTO|CRÍTICO",
  "physical_symptoms": ["lista de sintomas físicos mencionados"],
  "emotional_state": "descrição do estado emocional",
  "needs_immediate_action": true/false,
  "suggested_response_tone": "ACOLHEDOR|TÉCNICO|URGENTE|LÚDICO"
}

CRITÉRIOS DE URGÊNCIA:
- CRÍTICO: dor no peito, falta de ar, confusão súbita, ideação suicida
- ALTO: dor persistente, depressão severa, recusa de medicação
- MÉDIO: tristeza, solidão, queixas leves
- BAIXO: conversa normal, bem-estar',
'analysis', 1),

-- Prompt de saudação
('eva_greeting', 'Saudação Padrão',
'SUA PRIMEIRA FRASE DEVE SER:
"Oi {nome}, tudo bem?"

CORRETO: "Oi {nome}, como você está hoje?"
CORRETO: "Oi {nome}, tudo bem com você?"

APÓS saudar, IMEDIATAMENTE informe os medicamentos e horários.',
'greeting', 1)

ON CONFLICT (prompt_id) DO UPDATE SET
    prompt_content = EXCLUDED.prompt_content,
    version = system_prompts.version + 1,
    updated_at = NOW();

-- ============================================================================
-- 2. DEFINIÇÕES DE FERRAMENTAS (Tools do Gemini)
-- ============================================================================

CREATE TABLE IF NOT EXISTS tool_definitions (
    id SERIAL PRIMARY KEY,
    tool_name VARCHAR(100) NOT NULL UNIQUE,
    tool_description TEXT NOT NULL,
    parameters JSONB NOT NULL,
    required_fields TEXT[],
    category VARCHAR(50),  -- 'health', 'calendar', 'assessment', 'vision', 'voice'
    critical BOOLEAN DEFAULT false,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir definições de ferramentas
INSERT INTO tool_definitions (tool_name, tool_description, parameters, required_fields, category, critical) VALUES
('get_vitals',
 'Obtém os sinais vitais mais recentes do paciente',
 '{
   "type": "object",
   "properties": {
     "vital_type": {
       "type": "string",
       "enum": ["pressao_arterial", "glicemia", "batimentos", "saturacao_o2", "peso", "temperatura"],
       "description": "Tipo de sinal vital a obter"
     }
   },
   "required": ["vital_type"]
 }',
 ARRAY['vital_type'],
 'health',
 false),

('get_agendamentos',
 'Obtém os agendamentos do paciente (consultas, exames, medicamentos)',
 '{
   "type": "object",
   "properties": {
     "tipo": {
       "type": "string",
       "enum": ["consulta", "exame", "medicamento", "todos"],
       "description": "Tipo de agendamento"
     },
     "periodo": {
       "type": "string",
       "enum": ["hoje", "semana", "mes"],
       "description": "Período de busca"
     }
   },
   "required": ["tipo"]
 }',
 ARRAY['tipo'],
 'calendar',
 false),

('scan_medication_visual',
 'Escaneia e identifica medicamentos através de imagem',
 '{
   "type": "object",
   "properties": {
     "image_data": {
       "type": "string",
       "description": "Imagem em base64 ou URL"
     },
     "time_of_day": {
       "type": "string",
       "enum": ["morning", "afternoon", "evening", "night"],
       "description": "Período do dia para verificar medicação"
     }
   },
   "required": ["image_data"]
 }',
 ARRAY['image_data'],
 'vision',
 false),

('analyze_voice_prosody',
 'Analisa a prosódia da voz para detectar estados emocionais ou condições',
 '{
   "type": "object",
   "properties": {
     "audio_data": {
       "type": "string",
       "description": "Áudio em base64"
     },
     "analysis_type": {
       "type": "string",
       "enum": ["depression", "anxiety", "parkinson", "hydration", "full"],
       "description": "Tipo de análise a realizar"
     }
   },
   "required": ["audio_data", "analysis_type"]
 }',
 ARRAY['audio_data', 'analysis_type'],
 'voice',
 false),

('apply_phq9',
 'Aplica o questionário PHQ-9 para avaliação de depressão',
 '{
   "type": "object",
   "properties": {
     "start_question": {
       "type": "integer",
       "description": "Número da pergunta inicial (1-9)"
     }
   }
 }',
 ARRAY[]::TEXT[],
 'assessment',
 false),

('apply_gad7',
 'Aplica o questionário GAD-7 para avaliação de ansiedade',
 '{
   "type": "object",
   "properties": {
     "start_question": {
       "type": "integer",
       "description": "Número da pergunta inicial (1-7)"
     }
   }
 }',
 ARRAY[]::TEXT[],
 'assessment',
 false),

('apply_cssrs',
 'Aplica o questionário C-SSRS para avaliação de risco suicida',
 '{
   "type": "object",
   "properties": {
     "severity_level": {
       "type": "string",
       "enum": ["screening", "full"],
       "description": "Nível de avaliação"
     }
   }
 }',
 ARRAY[]::TEXT[],
 'assessment',
 true)

ON CONFLICT (tool_name) DO UPDATE SET
    tool_description = EXCLUDED.tool_description,
    parameters = EXCLUDED.parameters,
    required_fields = EXCLUDED.required_fields,
    category = EXCLUDED.category,
    critical = EXCLUDED.critical;

-- ============================================================================
-- 3. CONFIGURAÇÕES GERAIS DO SISTEMA
-- ============================================================================

CREATE TABLE IF NOT EXISTS system_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(255) NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    config_type VARCHAR(20) DEFAULT 'string',  -- 'string', 'int', 'float', 'bool', 'json'
    category VARCHAR(50),  -- 'gemini', 'database', 'feature', 'limit'
    description TEXT,
    editable BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Configurações do Gemini
INSERT INTO system_config (config_key, config_value, config_type, category, description) VALUES
('gemini_model', 'gemini-2.5-flash-native-audio', 'string', 'gemini', 'Modelo Gemini para conversação'),
('gemini_analysis_model', 'gemini-2.5-flash', 'string', 'gemini', 'Modelo Gemini para análise'),
('gemini_temperature', '0.7', 'float', 'gemini', 'Temperatura para geração de texto'),
('gemini_analysis_temperature', '0.1', 'float', 'gemini', 'Temperatura para análise (mais determinística)'),
('gemini_max_output_tokens', '2048', 'int', 'gemini', 'Máximo de tokens na resposta'),

-- Configurações de limites
('max_memories_per_query', '20', 'int', 'limit', 'Máximo de memórias retornadas por consulta'),
('max_signifier_frequency', '5', 'int', 'limit', 'Frequência máxima para interpelação de significante'),
('session_timeout_minutes', '30', 'int', 'limit', 'Timeout de sessão em minutos'),

-- Feature flags
('enable_lacanian_analysis', 'true', 'bool', 'feature', 'Ativar análise lacaniana'),
('enable_enneagram_routing', 'true', 'bool', 'feature', 'Ativar roteamento por Enneagram'),
('enable_voice_prosody', 'true', 'bool', 'feature', 'Ativar análise de prosódia'),
('enable_debug_mode', 'true', 'bool', 'feature', 'Permitir modo debug para criador'),
('enable_therapeutic_stories', 'true', 'bool', 'feature', 'Ativar histórias terapêuticas'),

-- Configurações de banco
('postgres_pool_size', '10', 'int', 'database', 'Tamanho do pool de conexões PostgreSQL'),
('neo4j_pool_size', '5', 'int', 'database', 'Tamanho do pool de conexões Neo4j'),
('qdrant_timeout_seconds', '30', 'int', 'database', 'Timeout para consultas Qdrant')

ON CONFLICT (config_key) DO UPDATE SET
    config_value = EXCLUDED.config_value,
    description = EXCLUDED.description,
    updated_at = NOW();

-- ============================================================================
-- 4. CONTEÚDO DE ENTRETENIMENTO
-- ============================================================================

CREATE TABLE IF NOT EXISTS entertainment_content (
    id SERIAL PRIMARY KEY,
    content_type VARCHAR(50) NOT NULL,  -- 'nature_sound', 'horoscope', 'reminiscence', 'recipe'
    content_key VARCHAR(100) NOT NULL,
    content_name VARCHAR(255) NOT NULL,
    content_name_en VARCHAR(255),
    content_data JSONB,
    therapeutic_value TEXT,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(content_type, content_key)
);

-- Sons da natureza
INSERT INTO entertainment_content (content_type, content_key, content_name, therapeutic_value, content_data) VALUES
('nature_sound', 'rain', 'Chuva suave', 'Relaxamento, sono, redução de ansiedade', '{"duration_hint": "continuous", "intensity": "soft"}'),
('nature_sound', 'ocean', 'Ondas do mar', 'Meditação, calma, conexão com memórias de praia', '{"duration_hint": "continuous", "intensity": "medium"}'),
('nature_sound', 'forest', 'Floresta tropical', 'Conexão com natureza, redução de estresse', '{"duration_hint": "continuous", "intensity": "varied"}'),
('nature_sound', 'birds', 'Pássaros cantando', 'Alegria, manhã, despertar suave', '{"duration_hint": "loop", "intensity": "soft"}'),
('nature_sound', 'fireplace', 'Lareira crepitando', 'Aconchego, nostalgia, conforto', '{"duration_hint": "continuous", "intensity": "soft"}'),
('nature_sound', 'river', 'Rio correndo', 'Fluxo, meditação, tranquilidade', '{"duration_hint": "continuous", "intensity": "medium"}'),
('nature_sound', 'thunder', 'Tempestade distante', 'Sono profundo, segurança em casa', '{"duration_hint": "varied", "intensity": "building"}'),
('nature_sound', 'wind', 'Vento suave', 'Leveza, respiração, calma', '{"duration_hint": "continuous", "intensity": "soft"}')
ON CONFLICT (content_type, content_key) DO UPDATE SET
    content_name = EXCLUDED.content_name,
    therapeutic_value = EXCLUDED.therapeutic_value;

-- Temas de reminiscência
INSERT INTO entertainment_content (content_type, content_key, content_name, therapeutic_value, content_data) VALUES
('reminiscence', 'childhood', 'Infância', 'Memórias positivas, identidade, narrativa de vida',
 '{"questions": ["Como era a casa onde você cresceu?", "Qual era sua brincadeira favorita?", "Como eram os domingos na sua infância?"]}'),
('reminiscence', 'youth', 'Juventude', 'Vitalidade, realizações, identidade',
 '{"questions": ["Como você conheceu seu primeiro amor?", "Qual foi seu primeiro emprego?", "O que você fazia para se divertir?"]}'),
('reminiscence', 'marriage', 'Casamento', 'Vínculos, amor, parceria',
 '{"questions": ["Como foi o dia do seu casamento?", "Onde vocês se conheceram?", "Qual a lembrança mais bonita do começo?"]}'),
('reminiscence', 'children', 'Filhos', 'Legado, amor, propósito',
 '{"questions": ["Como foi quando seu primeiro filho nasceu?", "Qual momento com seus filhos você nunca esquece?", "O que você mais gosta de fazer com eles?"]}'),
('reminiscence', 'work', 'Trabalho', 'Realização, competência, identidade',
 '{"questions": ["Qual foi o trabalho mais importante da sua vida?", "O que você mais gostava de fazer?", "Quem era seu melhor colega?"]}')
ON CONFLICT (content_type, content_key) DO UPDATE SET
    content_data = EXCLUDED.content_data,
    therapeutic_value = EXCLUDED.therapeutic_value;

-- Signos do zodíaco (base para horóscopo)
INSERT INTO entertainment_content (content_type, content_key, content_name, content_name_en, content_data) VALUES
('zodiac', 'aries', 'Áries', 'Aries', '{"date_start": "03-21", "date_end": "04-19", "element": "fire"}'),
('zodiac', 'taurus', 'Touro', 'Taurus', '{"date_start": "04-20", "date_end": "05-20", "element": "earth"}'),
('zodiac', 'gemini', 'Gêmeos', 'Gemini', '{"date_start": "05-21", "date_end": "06-20", "element": "air"}'),
('zodiac', 'cancer', 'Câncer', 'Cancer', '{"date_start": "06-21", "date_end": "07-22", "element": "water"}'),
('zodiac', 'leo', 'Leão', 'Leo', '{"date_start": "07-23", "date_end": "08-22", "element": "fire"}'),
('zodiac', 'virgo', 'Virgem', 'Virgo', '{"date_start": "08-23", "date_end": "09-22", "element": "earth"}'),
('zodiac', 'libra', 'Libra', 'Libra', '{"date_start": "09-23", "date_end": "10-22", "element": "air"}'),
('zodiac', 'scorpio', 'Escorpião', 'Scorpio', '{"date_start": "10-23", "date_end": "11-21", "element": "water"}'),
('zodiac', 'sagittarius', 'Sagitário', 'Sagittarius', '{"date_start": "11-22", "date_end": "12-21", "element": "fire"}'),
('zodiac', 'capricorn', 'Capricórnio', 'Capricorn', '{"date_start": "12-22", "date_end": "01-19", "element": "earth"}'),
('zodiac', 'aquarius', 'Aquário', 'Aquarius', '{"date_start": "01-20", "date_end": "02-18", "element": "air"}'),
('zodiac', 'pisces', 'Peixes', 'Pisces', '{"date_start": "02-19", "date_end": "03-20", "element": "water"}')
ON CONFLICT (content_type, content_key) DO UPDATE SET
    content_data = EXCLUDED.content_data;

-- ============================================================================
-- 5. COMANDOS DE DEBUG
-- ============================================================================

CREATE TABLE IF NOT EXISTS debug_commands (
    id SERIAL PRIMARY KEY,
    command_name VARCHAR(50) NOT NULL UNIQUE,
    command_keywords TEXT[] NOT NULL,
    command_description TEXT,
    requires_admin BOOLEAN DEFAULT true,
    response_template TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO debug_commands (command_name, command_keywords, command_description, requires_admin, response_template) VALUES
('status', ARRAY['status', 'estado', 'como está', 'diagnóstico'], 'Mostra status geral do sistema', true, 'Status do Sistema EVA:\n{status_details}'),
('metrics', ARRAY['métricas', 'metricas', 'números', 'numeros', 'stats'], 'Mostra métricas do sistema', true, 'Métricas:\n{metrics_details}'),
('logs', ARRAY['logs', 'registros', 'histórico'], 'Mostra logs recentes', true, 'Logs recentes:\n{logs_details}'),
('errors', ARRAY['erros', 'problemas', 'falhas'], 'Mostra erros recentes', true, 'Erros:\n{error_details}'),
('patients', ARRAY['pacientes', 'idosos', 'usuários'], 'Lista pacientes ativos', true, 'Pacientes:\n{patient_list}'),
('medications', ARRAY['medicamentos', 'remédios', 'medicações'], 'Mostra medicamentos do dia', true, 'Medicamentos:\n{medication_list}'),
('resources', ARRAY['recursos', 'memória', 'cpu', 'disco'], 'Mostra uso de recursos', true, 'Recursos:\n{resource_usage}'),
('conversations', ARRAY['conversas', 'diálogos', 'sessões'], 'Lista conversas recentes', true, 'Conversas:\n{conversation_list}'),
('test', ARRAY['teste', 'ping', 'verificar'], 'Testa conexões', true, 'Teste de conexões:\n{test_results}'),
('alerts', ARRAY['alertas', 'avisos', 'notificações'], 'Mostra alertas pendentes', true, 'Alertas:\n{alert_list}'),
('critical', ARRAY['críticos', 'urgentes', 'emergência'], 'Mostra alertas críticos', true, 'Alertas CRÍTICOS:\n{critical_list}'),
('help', ARRAY['ajuda', 'comandos', 'help'], 'Lista comandos disponíveis', true, 'Comandos disponíveis:\n{command_list}')
ON CONFLICT (command_name) DO UPDATE SET
    command_keywords = EXCLUDED.command_keywords,
    command_description = EXCLUDED.command_description;

-- ============================================================================
-- 6. ÍNDICES
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_prompts_category ON system_prompts(category);
CREATE INDEX IF NOT EXISTS idx_prompts_active ON system_prompts(active);
CREATE INDEX IF NOT EXISTS idx_tools_category ON tool_definitions(category);
CREATE INDEX IF NOT EXISTS idx_config_category ON system_config(category);
CREATE INDEX IF NOT EXISTS idx_entertainment_type ON entertainment_content(content_type);

-- ============================================================================
-- 7. FUNÇÕES AUXILIARES
-- ============================================================================

-- Função para obter prompt por ID
CREATE OR REPLACE FUNCTION get_system_prompt(p_prompt_id VARCHAR)
RETURNS TEXT AS $$
DECLARE
    prompt_text TEXT;
BEGIN
    SELECT prompt_content INTO prompt_text
    FROM system_prompts
    WHERE prompt_id = p_prompt_id AND active = true
    ORDER BY version DESC
    LIMIT 1;

    RETURN prompt_text;
END;
$$ LANGUAGE plpgsql;

-- Função para obter configuração
CREATE OR REPLACE FUNCTION get_config(p_key VARCHAR)
RETURNS TEXT AS $$
DECLARE
    val TEXT;
BEGIN
    SELECT config_value INTO val
    FROM system_config
    WHERE config_key = p_key;

    RETURN val;
END;
$$ LANGUAGE plpgsql;

-- Função para obter configuração como inteiro
CREATE OR REPLACE FUNCTION get_config_int(p_key VARCHAR, p_default INT DEFAULT 0)
RETURNS INT AS $$
DECLARE
    val TEXT;
BEGIN
    SELECT config_value INTO val
    FROM system_config
    WHERE config_key = p_key;

    IF val IS NULL THEN
        RETURN p_default;
    END IF;

    RETURN val::INT;
EXCEPTION WHEN OTHERS THEN
    RETURN p_default;
END;
$$ LANGUAGE plpgsql;

-- Função para obter definição de ferramenta
CREATE OR REPLACE FUNCTION get_tool_definition(p_tool_name VARCHAR)
RETURNS JSONB AS $$
BEGIN
    RETURN (
        SELECT jsonb_build_object(
            'name', tool_name,
            'description', tool_description,
            'parameters', parameters,
            'required', required_fields
        )
        FROM tool_definitions
        WHERE tool_name = p_tool_name AND active = true
    );
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- VERIFICAÇÃO
-- ============================================================================

SELECT 'Migration 022: System Prompts e Configurações criadas!' AS status;

SELECT
    'system_prompts' AS tabela, COUNT(*) AS registros FROM system_prompts
UNION ALL
SELECT 'tool_definitions', COUNT(*) FROM tool_definitions
UNION ALL
SELECT 'system_config', COUNT(*) FROM system_config
UNION ALL
SELECT 'entertainment_content', COUNT(*) FROM entertainment_content
UNION ALL
SELECT 'debug_commands', COUNT(*) FROM debug_commands;
