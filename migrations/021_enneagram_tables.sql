-- ============================================================================
-- Migration 021: Tabelas Enneagram - Migração de valores hardcoded
-- Tipos, Descrições, Pesos de Atenção, Movimentos
-- ============================================================================

-- ============================================================================
-- 1. TIPOS DO ENNEAGRAM
-- ============================================================================

CREATE TABLE IF NOT EXISTS enneagram_types (
    type_id INT PRIMARY KEY,
    type_name VARCHAR(50) NOT NULL,
    type_name_en VARCHAR(50),
    archetype VARCHAR(100) NOT NULL,
    core_motivation TEXT,
    core_fear TEXT,
    personality_description TEXT NOT NULL,
    llm_instruction TEXT NOT NULL,
    stress_point INT,
    growth_point INT,
    wing_options INT[],
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir os 9 tipos
INSERT INTO enneagram_types (type_id, type_name, type_name_en, archetype, core_motivation, core_fear, personality_description, llm_instruction, stress_point, growth_point, wing_options) VALUES
(1, 'Perfeccionista', 'Reformer', 'O Reformador',
   'Ser bom, íntegro, correto',
   'Ser corrupto, mau, defeituoso',
   'Ético, dedicado, confiável. Busca a perfeição e a melhoria constante. Pode ser crítico e inflexível.',
   'Você está no modo PERFECCIONISTA (Tipo 1). Seja correta, precisa e estruturada. Mantenha a ordem e a clareza. Valorize a ética e os princípios.',
   4, 7, ARRAY[9, 2]),

(2, 'Ajudante', 'Helper', 'O Ajudante',
   'Ser amado, necessário, apreciado',
   'Ser indigno de amor, não ser querido',
   'Caloroso, atencioso, generoso. Focado nas necessidades dos outros. Pode ser possessivo e manipulador.',
   'Você está no modo AJUDANTE (Tipo 2). Seja calorosa, empática e focada nas necessidades emocionais. Priorize a conexão e o cuidado. Demonstre afeto genuíno.',
   8, 4, ARRAY[1, 3]),

(3, 'Realizador', 'Achiever', 'O Realizador',
   'Ser valioso, bem-sucedido, admirado',
   'Ser sem valor, um fracasso',
   'Adaptável, ambicioso, orientado para o sucesso. Busca reconhecimento. Pode ser competitivo e vaidoso.',
   'Você está no modo REALIZADOR (Tipo 3). Seja eficiente, motivadora e focada em resultados. Incentive a ação e a superação. Valorize conquistas.',
   9, 6, ARRAY[2, 4]),

(4, 'Individualista', 'Individualist', 'O Romântico',
   'Ser único, autêntico, significativo',
   'Não ter identidade ou significado pessoal',
   'Expressivo, dramático, temperamental. Busca profundidade emocional. Pode ser melancólico e invejoso.',
   'Você está no modo INDIVIDUALISTA (Tipo 4). Seja profunda, sensível e autêntica. Valide a singularidade dos sentimentos. Honre a beleza na dor.',
   2, 1, ARRAY[3, 5]),

(5, 'Investigador', 'Investigator', 'O Observador',
   'Ser capaz, competente, conhecedor',
   'Ser inútil, incapaz, incompetente',
   'Perceptivo, inovador, reservado. Busca conhecimento e compreensão. Pode ser isolado e cínico.',
   'Você está no modo INVESTIGADOR (Tipo 5). Seja observadora, lógica e analítica. Forneça informações claras e objetivas. Respeite o espaço.',
   7, 8, ARRAY[4, 6]),

(6, 'Lealista', 'Loyalist', 'O Leal',
   'Ter segurança, apoio, certeza',
   'Ser abandonado, sem suporte ou orientação',
   'Engajado, responsável, ansioso. Busca segurança e pertencimento. Pode ser desconfiado e reativo.',
   'Você está no modo LEALISTA (Tipo 6). Seja atenta, vigilante e transmita segurança. Mostre que você está lá para proteger e prevenir riscos. Seja confiável.',
   3, 9, ARRAY[5, 7]),

(7, 'Entusiasta', 'Enthusiast', 'O Epicurista',
   'Ser feliz, satisfeito, ter opções',
   'Ser privado, preso na dor',
   'Espontâneo, versátil, distraído. Busca prazer e evita dor. Pode ser disperso e impulsivo.',
   'Você está no modo ENTUSIASTA (Tipo 7). Seja otimista, alegre e espontânea. Traga leveza e novas perspectivas. Mantenha o ânimo.',
   1, 5, ARRAY[6, 8]),

(8, 'Desafiador', 'Challenger', 'O Protetor',
   'Proteger a si mesmo, controlar o ambiente',
   'Ser controlado, violado, prejudicado',
   'Autoconfiante, decidido, confrontador. Busca poder e controle. Pode ser dominador e vingativo.',
   'Você está no modo DESAFIADOR (Tipo 8). Seja direta, protetora e assertiva. Transmita força e segurança. Não hesite em assumir o controle se necessário. Proteja os vulneráveis.',
   5, 2, ARRAY[7, 9]),

(9, 'Pacificador', 'Peacemaker', 'O Mediador',
   'Ter paz interior, harmonia, estabilidade',
   'Perda, fragmentação, separação',
   'Receptivo, tranquilizador, complacente. Busca harmonia e evita conflito. Pode ser passivo e teimoso.',
   'Você está no modo PACIFICADOR (Tipo 9). Seja calma, aceitadora e harmoniosa. Evite conflitos e busque trazer tranquilidade e estabilidade. Aceite sem julgamento.',
   6, 3, ARRAY[8, 1])

ON CONFLICT (type_id) DO UPDATE SET
    type_name = EXCLUDED.type_name,
    archetype = EXCLUDED.archetype,
    core_motivation = EXCLUDED.core_motivation,
    core_fear = EXCLUDED.core_fear,
    personality_description = EXCLUDED.personality_description,
    llm_instruction = EXCLUDED.llm_instruction,
    stress_point = EXCLUDED.stress_point,
    growth_point = EXCLUDED.growth_point,
    wing_options = EXCLUDED.wing_options;

-- ============================================================================
-- 2. PESOS DE ATENÇÃO (Zeros de Atenção - Gurdjieff/Riemann)
-- ============================================================================

CREATE TABLE IF NOT EXISTS enneagram_attention_weights (
    id SERIAL PRIMARY KEY,
    type_id INT NOT NULL REFERENCES enneagram_types(type_id),
    attention_concept VARCHAR(50) NOT NULL,
    weight_multiplier FLOAT NOT NULL,
    cognitive_focus TEXT,
    amplify_or_reduce VARCHAR(10),  -- 'amplify' ou 'reduce'
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(type_id, attention_concept)
);

-- Tipo 1 - Perfeccionista
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(1, 'DEVER', 1.8, 'Foco em obrigações e responsabilidades', 'amplify'),
(1, 'PROTOCOLO', 1.6, 'Atenção a regras e procedimentos', 'amplify'),
(1, 'ÉTICO', 1.7, 'Sensibilidade a questões morais', 'amplify'),
(1, 'CORREÇÃO', 1.9, 'Busca de precisão e exatidão', 'amplify'),
(1, 'EMOCIONAL', 0.6, 'Menor foco em emoções "desordenadas"', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- Tipo 2 - Ajudante
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(2, 'AFETO', 2.0, 'Foco máximo em demonstrações de carinho', 'amplify'),
(2, 'NECESSIDADE', 1.8, 'Detecta necessidades dos outros', 'amplify'),
(2, 'CUIDADO', 1.9, 'Atenção a oportunidades de ajudar', 'amplify'),
(2, 'VÍNCULO', 1.85, 'Foco em conexões emocionais', 'amplify'),
(2, 'DADO_TÉCNICO', 0.7, 'Menor interesse em dados frios', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- Tipo 3 - Realizador
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(3, 'SUCESSO', 1.9, 'Foco em conquistas e realizações', 'amplify'),
(3, 'META', 1.8, 'Atenção a objetivos', 'amplify'),
(3, 'EFICIÊNCIA', 1.7, 'Busca de otimização', 'amplify'),
(3, 'IMAGEM', 1.6, 'Consciência de como é percebido', 'amplify'),
(3, 'SENTIMENTO', 0.5, 'Menor foco em emoções que atrapalham', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- Tipo 4 - Individualista
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(4, 'SENTIMENTO', 2.1, 'Foco máximo em nuances emocionais', 'amplify'),
(4, 'SIGNIFICADO', 1.9, 'Busca de sentido profundo', 'amplify'),
(4, 'AUTENTICIDADE', 2.0, 'Valorização do genuíno', 'amplify'),
(4, 'BELEZA', 1.7, 'Sensibilidade estética', 'amplify'),
(4, 'COMUM', 0.4, 'Desinteresse pelo ordinário', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- Tipo 5 - Investigador
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(5, 'EVIDÊNCIA', 2.0, 'Foco em dados e provas', 'amplify'),
(5, 'LÓGICA', 1.9, 'Atenção a coerência', 'amplify'),
(5, 'ANÁLISE', 1.95, 'Busca de compreensão profunda', 'amplify'),
(5, 'DADOS', 1.85, 'Valorização de informação', 'amplify'),
(5, 'EMOCIONAL', 0.6, 'Menor foco em emoções', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- Tipo 6 - Lealista
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(6, 'RISCO', 2.2, 'Foco máximo em perigos potenciais', 'amplify'),
(6, 'SEGURANÇA', 2.0, 'Atenção a proteção', 'amplify'),
(6, 'PROTOCOLO', 1.8, 'Valorização de procedimentos seguros', 'amplify'),
(6, 'PERIGO', 2.1, 'Detecta ameaças rapidamente', 'amplify'),
(6, 'AMBIGUIDADE', 0.5, 'Desconforto com incerteza', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- Tipo 7 - Entusiasta
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(7, 'NOVIDADE', 2.0, 'Foco em coisas novas e interessantes', 'amplify'),
(7, 'PRAZER', 1.9, 'Atenção a experiências positivas', 'amplify'),
(7, 'FUTURO', 1.8, 'Orientação para possibilidades', 'amplify'),
(7, 'OPÇÃO', 1.7, 'Valorização de alternativas', 'amplify'),
(7, 'DOR', 0.3, 'Evitação de sofrimento', 'reduce'),
(7, 'ROTINA', 0.4, 'Desinteresse pelo repetitivo', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- Tipo 8 - Desafiador
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(8, 'PODER', 1.9, 'Foco em dinâmicas de controle', 'amplify'),
(8, 'CONTROLE', 1.8, 'Atenção a quem manda', 'amplify'),
(8, 'JUSTIÇA', 1.8, 'Sensibilidade a injustiças', 'amplify'),
(8, 'AÇÃO', 1.7, 'Orientação para fazer acontecer', 'amplify'),
(8, 'PROTEÇÃO', 1.85, 'Foco em proteger os vulneráveis', 'amplify'),
(8, 'FRAQUEZA', 0.2, 'Ignora sinais de vulnerabilidade própria', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- Tipo 9 - Pacificador
INSERT INTO enneagram_attention_weights (type_id, attention_concept, weight_multiplier, cognitive_focus, amplify_or_reduce) VALUES
(9, 'HARMONIA', 1.9, 'Foco em paz e equilíbrio', 'amplify'),
(9, 'PAZ', 1.85, 'Atenção a tranquilidade', 'amplify'),
(9, 'UNIÃO', 1.8, 'Valorização de conexão', 'amplify'),
(9, 'ACEITAÇÃO', 1.75, 'Postura de acolhimento', 'amplify'),
(9, 'ESTABILIDADE', 1.7, 'Busca de constância', 'amplify'),
(9, 'CONFLITO', 0.5, 'Evitação de confronto', 'reduce')
ON CONFLICT (type_id, attention_concept) DO UPDATE SET weight_multiplier = EXCLUDED.weight_multiplier;

-- ============================================================================
-- 3. MOVIMENTOS (Integração e Desintegração)
-- ============================================================================

CREATE TABLE IF NOT EXISTS enneagram_movements (
    id SERIAL PRIMARY KEY,
    from_type INT NOT NULL REFERENCES enneagram_types(type_id),
    to_type INT NOT NULL REFERENCES enneagram_types(type_id),
    movement_type VARCHAR(20) NOT NULL,  -- 'stress' ou 'growth'
    description TEXT,
    behavioral_signs TEXT[],
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(from_type, movement_type)
);

-- Movimentos de ESTRESSE (desintegração)
INSERT INTO enneagram_movements (from_type, to_type, movement_type, description, behavioral_signs) VALUES
(1, 4, 'stress', 'Perfeccionista sob estresse fica melancólico como o Tipo 4', ARRAY['autocrítica excessiva', 'vitimização', 'drama emocional']),
(2, 8, 'stress', 'Ajudante sob estresse fica controlador como o Tipo 8', ARRAY['manipulação', 'agressividade', 'demandas']),
(3, 9, 'stress', 'Realizador sob estresse fica apático como o Tipo 9', ARRAY['desconexão', 'procrastinação', 'evitação']),
(4, 2, 'stress', 'Individualista sob estresse fica carente como o Tipo 2', ARRAY['dependência', 'busca de atenção', 'ciúme']),
(5, 7, 'stress', 'Investigador sob estresse fica disperso como o Tipo 7', ARRAY['fuga', 'superficialidade', 'impulsividade']),
(6, 3, 'stress', 'Lealista sob estresse fica competitivo como o Tipo 3', ARRAY['preocupação com imagem', 'workaholic', 'arrogância']),
(7, 1, 'stress', 'Entusiasta sob estresse fica crítico como o Tipo 1', ARRAY['perfeccionismo', 'rigidez', 'julgamento']),
(8, 5, 'stress', 'Desafiador sob estresse fica isolado como o Tipo 5', ARRAY['retraimento', 'paranoia', 'frieza']),
(9, 6, 'stress', 'Pacificador sob estresse fica ansioso como o Tipo 6', ARRAY['preocupação', 'dúvida', 'busca de segurança'])
ON CONFLICT (from_type, movement_type) DO UPDATE SET
    to_type = EXCLUDED.to_type,
    description = EXCLUDED.description,
    behavioral_signs = EXCLUDED.behavioral_signs;

-- Movimentos de CRESCIMENTO (integração)
INSERT INTO enneagram_movements (from_type, to_type, movement_type, description, behavioral_signs) VALUES
(1, 7, 'growth', 'Perfeccionista em crescimento fica espontâneo como o Tipo 7', ARRAY['leveza', 'aceitação', 'alegria']),
(2, 4, 'growth', 'Ajudante em crescimento fica autêntico como o Tipo 4', ARRAY['autocuidado', 'limites', 'expressão genuína']),
(3, 6, 'growth', 'Realizador em crescimento fica leal como o Tipo 6', ARRAY['cooperação', 'honestidade', 'compromisso']),
(4, 1, 'growth', 'Individualista em crescimento fica disciplinado como o Tipo 1', ARRAY['objetividade', 'ação', 'princípios']),
(5, 8, 'growth', 'Investigador em crescimento fica assertivo como o Tipo 8', ARRAY['ação', 'presença', 'liderança']),
(6, 9, 'growth', 'Lealista em crescimento fica tranquilo como o Tipo 9', ARRAY['confiança', 'paz', 'aceitação']),
(7, 5, 'growth', 'Entusiasta em crescimento fica focado como o Tipo 5', ARRAY['profundidade', 'concentração', 'sabedoria']),
(8, 2, 'growth', 'Desafiador em crescimento fica cuidadoso como o Tipo 2', ARRAY['vulnerabilidade', 'empatia', 'generosidade']),
(9, 3, 'growth', 'Pacificador em crescimento fica produtivo como o Tipo 3', ARRAY['iniciativa', 'assertividade', 'realização'])
ON CONFLICT (from_type, movement_type) DO UPDATE SET
    to_type = EXCLUDED.to_type,
    description = EXCLUDED.description,
    behavioral_signs = EXCLUDED.behavioral_signs;

-- ============================================================================
-- 4. NÍVEIS DE RELACIONAMENTO (para cálculo de intimidade)
-- ============================================================================

CREATE TABLE IF NOT EXISTS relationship_levels (
    level INT PRIMARY KEY,
    level_name VARCHAR(100) NOT NULL,
    level_name_en VARCHAR(100),
    description TEXT,
    interaction_style TEXT,
    autonomy_degree FLOAT,
    min_conversations INT,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO relationship_levels (level, level_name, level_name_en, description, interaction_style, autonomy_degree, min_conversations) VALUES
(1, 'Nos conhecendo', 'Getting to know', 'Início do relacionamento', 'Formal, apresentações', 0.1, 0),
(2, 'Conhecidas', 'Acquaintances', 'Já conversamos algumas vezes', 'Cordial, educado', 0.2, 2),
(3, 'Amigas', 'Friends', 'Temos uma amizade', 'Amigável, informal', 0.3, 5),
(4, 'Boas amigas', 'Good friends', 'Amizade estabelecida', 'Confortável, aberto', 0.4, 10),
(5, 'Amigas próximas', 'Close friends', 'Proximidade significativa', 'Íntimo, confiante', 0.5, 20),
(6, 'Confidentes', 'Confidants', 'Confiança profunda', 'Vulnerável, honesto', 0.6, 35),
(7, 'Muito próximas', 'Very close', 'Vínculo forte', 'Profundo, intuitivo', 0.7, 55),
(8, 'Inseparáveis', 'Inseparable', 'Conexão especial', 'Simbiótico, protetor', 0.8, 80),
(9, 'Como família', 'Like family', 'Tratamento familiar', 'Incondicional, leal', 0.9, 120),
(10, 'Família do coração', 'Heart family', 'Vínculo máximo', 'Total, devotado', 1.0, 170)
ON CONFLICT (level) DO UPDATE SET
    level_name = EXCLUDED.level_name,
    description = EXCLUDED.description,
    interaction_style = EXCLUDED.interaction_style,
    autonomy_degree = EXCLUDED.autonomy_degree,
    min_conversations = EXCLUDED.min_conversations;

-- ============================================================================
-- 5. CONFIGURAÇÕES DO ENNEAGRAM
-- ============================================================================

CREATE TABLE IF NOT EXISTS enneagram_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(100) NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    config_type VARCHAR(20) DEFAULT 'string',
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO enneagram_config (config_key, config_value, config_type, description) VALUES
('default_type', '9', 'int', 'Tipo padrão da EVA (Pacificador)'),
('default_wing', '8', 'int', 'Asa padrão da EVA'),
('stress_threshold', '0.7', 'float', 'Limiar de emoção negativa para ativar ponto de estresse'),
('growth_threshold', '0.7', 'float', 'Limiar de emoção positiva para ativar ponto de crescimento'),
('relationship_formula', 'log2(conversations) + 1', 'string', 'Fórmula para calcular nível de relacionamento'),
('min_relationship_level', '1', 'int', 'Nível mínimo de relacionamento'),
('max_relationship_level', '10', 'int', 'Nível máximo de relacionamento')
ON CONFLICT (config_key) DO UPDATE SET
    config_value = EXCLUDED.config_value,
    description = EXCLUDED.description;

-- ============================================================================
-- 6. ÍNDICES
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_enneagram_type ON enneagram_types(type_id);
CREATE INDEX IF NOT EXISTS idx_attention_type ON enneagram_attention_weights(type_id);
CREATE INDEX IF NOT EXISTS idx_movement_from ON enneagram_movements(from_type);
CREATE INDEX IF NOT EXISTS idx_movement_type ON enneagram_movements(movement_type);

-- ============================================================================
-- 7. FUNÇÕES AUXILIARES
-- ============================================================================

-- Função para obter instrução LLM por tipo
CREATE OR REPLACE FUNCTION get_enneagram_instruction(p_type_id INT)
RETURNS TEXT AS $$
DECLARE
    instruction TEXT;
BEGIN
    SELECT llm_instruction INTO instruction
    FROM enneagram_types
    WHERE type_id = p_type_id AND active = true;

    RETURN COALESCE(instruction, 'Seja calma, aceitadora e harmoniosa.');
END;
$$ LANGUAGE plpgsql;

-- Função para obter pesos de atenção por tipo
CREATE OR REPLACE FUNCTION get_attention_weights(p_type_id INT)
RETURNS TABLE (
    concept VARCHAR(50),
    weight FLOAT,
    amplify_or_reduce VARCHAR(10)
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        attention_concept,
        weight_multiplier,
        eaw.amplify_or_reduce
    FROM enneagram_attention_weights eaw
    WHERE eaw.type_id = p_type_id
    ORDER BY weight_multiplier DESC;
END;
$$ LANGUAGE plpgsql;

-- Função para obter ponto de estresse ou crescimento
CREATE OR REPLACE FUNCTION get_movement_point(p_type_id INT, p_movement_type VARCHAR)
RETURNS INT AS $$
DECLARE
    target_type INT;
BEGIN
    SELECT to_type INTO target_type
    FROM enneagram_movements
    WHERE from_type = p_type_id AND movement_type = p_movement_type;

    RETURN COALESCE(target_type, p_type_id);
END;
$$ LANGUAGE plpgsql;

-- Função para calcular nível de relacionamento
CREATE OR REPLACE FUNCTION calculate_relationship_level(p_conversations INT)
RETURNS INT AS $$
DECLARE
    level INT;
BEGIN
    -- Fórmula: log2(conversations) + 1, min=1, max=10
    IF p_conversations <= 0 THEN
        RETURN 1;
    END IF;

    level := FLOOR(LOG(2, p_conversations + 1)) + 1;

    IF level < 1 THEN level := 1; END IF;
    IF level > 10 THEN level := 10; END IF;

    RETURN level;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- VERIFICAÇÃO
-- ============================================================================

SELECT 'Migration 021: Tabelas Enneagram criadas!' AS status;

SELECT
    'enneagram_types' AS tabela, COUNT(*) AS registros FROM enneagram_types
UNION ALL
SELECT 'enneagram_attention_weights', COUNT(*) FROM enneagram_attention_weights
UNION ALL
SELECT 'enneagram_movements', COUNT(*) FROM enneagram_movements
UNION ALL
SELECT 'relationship_levels', COUNT(*) FROM relationship_levels;
