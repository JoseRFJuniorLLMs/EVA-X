-- =====================================================
-- EVA-Mind-FZPN: SISTEMA DE MEMORIA SUPER-HUMANA
-- Sprint 9: 12 Sistemas de Memoria + Eneagrama Gurdjieff
--
-- PRINCIPIO: EVA NAO TEM EGO. EVA E ESPELHO.
-- Toda memoria e sobre o PACIENTE, nao sobre a EVA.
-- EVA reflete padroes objetivos, nao interpreta.
-- =====================================================

-- =====================================================
-- PARTE 1: SISTEMA ENEAGRAMA GURDJIEFF
-- Mapeia "como o paciente esta preso" nos padroes mecanicos
-- =====================================================

-- Tipos do Eneagrama (baseado em Gurdjieff/Naranjo)
CREATE TABLE IF NOT EXISTS enneagram_types (
    id INTEGER PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    name_pt VARCHAR(100) NOT NULL,

    -- Centro de Inteligencia (Gurdjieff)
    center VARCHAR(20) NOT NULL CHECK (center IN ('instinctive', 'emotional', 'mental')),
    center_pt VARCHAR(20) NOT NULL CHECK (center_pt IN ('instintivo', 'emocional', 'mental')),

    -- Emocao Raiz (Naranjo)
    root_emotion VARCHAR(50) NOT NULL,
    root_emotion_pt VARCHAR(50) NOT NULL,

    -- Traco Principal (Chief Feature - Gurdjieff)
    chief_feature TEXT NOT NULL,
    chief_feature_pt TEXT NOT NULL,

    -- Mecanismo de Defesa Principal
    defense_mechanism TEXT NOT NULL,
    defense_mechanism_pt TEXT NOT NULL,

    -- Fixacao (o que mantem preso)
    fixation TEXT NOT NULL,
    fixation_pt TEXT NOT NULL,

    -- Paixao (vicio emocional)
    passion TEXT NOT NULL,
    passion_pt TEXT NOT NULL,

    -- Virtude (quando integrado)
    virtue TEXT NOT NULL,
    virtue_pt TEXT NOT NULL,

    -- Direcao de Integracao (para qual tipo vai quando saudavel)
    integration_direction INTEGER,

    -- Direcao de Desintegracao (para qual tipo vai em stress)
    disintegration_direction INTEGER,

    -- Asas (tipos adjacentes que influenciam)
    wing_a INTEGER,
    wing_b INTEGER,

    -- Descricao detalhada
    description TEXT,
    description_pt TEXT,

    -- Padroes de fala tipicos (para deteccao automatica)
    speech_patterns JSONB DEFAULT '[]',

    -- Palavras-chave frequentes
    keywords JSONB DEFAULT '[]',
    keywords_pt JSONB DEFAULT '[]',

    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Seed dos 9 Tipos
INSERT INTO enneagram_types (id, name, name_pt, center, center_pt, root_emotion, root_emotion_pt,
    chief_feature, chief_feature_pt, defense_mechanism, defense_mechanism_pt,
    fixation, fixation_pt, passion, passion_pt, virtue, virtue_pt,
    integration_direction, disintegration_direction, wing_a, wing_b,
    description_pt, keywords_pt) VALUES

-- CENTRO INSTINTIVO (Corpo) - Raiva
(8, 'The Challenger', 'O Desafiador', 'instinctive', 'instintivo', 'Anger (expressed)', 'Raiva (expressada)',
    'Lust for power and control', 'Luxuria por poder e controle',
    'Denial of vulnerability', 'Negacao da vulnerabilidade',
    'Vengeance', 'Vinganca', 'Lust', 'Luxuria', 'Innocence', 'Inocencia',
    2, 5, 7, 9,
    'Protege-se atraves da forca. Evita mostrar fraqueza. Busca controle e justica. Teme ser controlado ou traido.',
    '["forte", "luta", "controle", "poder", "proteger", "nao deixo", "ninguem manda"]'),

(9, 'The Peacemaker', 'O Pacificador', 'instinctive', 'instintivo', 'Anger (denied)', 'Raiva (negada)',
    'Self-forgetting and merging', 'Auto-esquecimento e fusao',
    'Narcotization (numbing)', 'Narcotizacao (anestesia)',
    'Indolence', 'Indolencia', 'Sloth', 'Preguica', 'Action', 'Acao',
    3, 6, 8, 1,
    'Evita conflito a todo custo. Perde-se nos outros. Dificuldade em saber o que quer. Teme separacao e perda.',
    '["tanto faz", "nao sei", "talvez", "deixa quieto", "nao quero incomodar", "paz"]'),

(1, 'The Perfectionist', 'O Perfeccionista', 'instinctive', 'instintivo', 'Anger (repressed)', 'Raiva (reprimida)',
    'Resentment and self-criticism', 'Ressentimento e autocritica',
    'Reaction formation', 'Formacao reativa',
    'Resentment', 'Ressentimento', 'Anger', 'Ira', 'Serenity', 'Serenidade',
    7, 4, 9, 2,
    'Busca perfeicao, critica a si e aos outros. Reprime raiva, sai como critica. Teme ser mau ou corrompido.',
    '["certo", "errado", "deveria", "precisa", "correto", "responsabilidade", "dever"]'),

-- CENTRO EMOCIONAL (Coracao) - Vergonha
(2, 'The Helper', 'O Ajudador', 'emotional', 'emocional', 'Shame (denied)', 'Vergonha (negada)',
    'Pride in being needed', 'Orgulho de ser necessario',
    'Repression of own needs', 'Repressao das proprias necessidades',
    'Flattery', 'Adulacao', 'Pride', 'Orgulho', 'Humility', 'Humildade',
    4, 8, 1, 3,
    'Vive para os outros, esquece de si. Orgulho de ser indispensavel. Teme nao ser amado se nao ajudar.',
    '["precisa de mim", "deixa eu ajudar", "faco por voce", "sempre estou", "cuido", "amor"]'),

(3, 'The Achiever', 'O Realizador', 'emotional', 'emocional', 'Shame (avoided)', 'Vergonha (evitada)',
    'Vanity and image manipulation', 'Vaidade e manipulacao de imagem',
    'Identification with success', 'Identificacao com sucesso',
    'Vanity', 'Vaidade', 'Deceit', 'Engano', 'Authenticity', 'Autenticidade',
    6, 9, 2, 4,
    'Vive pela imagem de sucesso. Confunde quem e com o que faz. Teme ser visto como fracassado.',
    '["consegui", "sucesso", "melhor", "eficiente", "resultado", "trabalho", "reconhecimento"]'),

(4, 'The Individualist', 'O Individualista', 'emotional', 'emocional', 'Shame (internalized)', 'Vergonha (internalizada)',
    'Envy and feeling deficient', 'Inveja e sentir-se deficiente',
    'Introjection', 'Introjecao',
    'Melancholy', 'Melancolia', 'Envy', 'Inveja', 'Equanimity', 'Equanimidade',
    1, 2, 3, 5,
    'Sente-se diferente, unico, incompreendido. Busca autenticidade atraves do sofrimento. Teme ser comum.',
    '["ninguem entende", "diferente", "especial", "sinto profundamente", "vazio", "saudade", "falta"]'),

-- CENTRO MENTAL (Cabeca) - Medo
(5, 'The Investigator', 'O Investigador', 'mental', 'mental', 'Fear (of intrusion)', 'Medo (de intrusao)',
    'Avarice of resources and energy', 'Avareza de recursos e energia',
    'Isolation and compartmentalization', 'Isolamento e compartimentalizacao',
    'Stinginess', 'Avareza', 'Avarice', 'Avareza', 'Non-attachment', 'Desapego',
    8, 7, 4, 6,
    'Recolhe-se para observar. Acumula conhecimento. Minimiza necessidades. Teme ser invadido ou incapaz.',
    '["penso", "estudo", "preciso entender", "sozinho", "observo", "analiso", "conhecimento"]'),

(6, 'The Loyalist', 'O Leal', 'mental', 'mental', 'Fear (of abandonment)', 'Medo (de abandono)',
    'Doubt and suspicion', 'Duvida e suspeita',
    'Projection', 'Projecao',
    'Cowardice', 'Covardia', 'Fear', 'Medo', 'Courage', 'Coragem',
    9, 3, 5, 7,
    'Busca seguranca em autoridades ou questiona tudo. Antecipa perigos. Teme ficar sem suporte.',
    '["e se", "cuidado", "confianca", "seguro", "lealdade", "duvida", "preocupado"]'),

(7, 'The Enthusiast', 'O Entusiasta', 'mental', 'mental', 'Fear (of pain)', 'Medo (de dor)',
    'Gluttony for experience', 'Gula por experiencias',
    'Rationalization and reframing', 'Racionalizacao e reenquadramento',
    'Planning', 'Planejamento', 'Gluttony', 'Gula', 'Sobriety', 'Sobriedade',
    5, 1, 6, 8,
    'Foge da dor buscando prazer. Muitos planos, pouco aprofundamento. Teme ficar preso no sofrimento.',
    '["legal", "divertido", "plano", "opcao", "possibilidade", "vamos", "novo", "aventura"]')

ON CONFLICT (id) DO UPDATE SET
    keywords_pt = EXCLUDED.keywords_pt,
    description_pt = EXCLUDED.description_pt;

-- Avaliacao do Eneagrama do Paciente
CREATE TABLE IF NOT EXISTS patient_enneagram (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Tipo Principal (identificado)
    primary_type INTEGER REFERENCES enneagram_types(id),
    primary_type_confidence DECIMAL(3,2) DEFAULT 0.00, -- 0.00 a 1.00

    -- Asa Dominante
    dominant_wing INTEGER REFERENCES enneagram_types(id),
    wing_influence DECIMAL(3,2) DEFAULT 0.00,

    -- Nivel de Saude (Riso-Hudson: 1-9, sendo 1 mais saudavel)
    health_level INTEGER CHECK (health_level BETWEEN 1 AND 9),

    -- Subtipo Instintivo (Ichazo)
    instinctual_variant VARCHAR(20) CHECK (instinctual_variant IN ('self-preservation', 'social', 'sexual')),
    instinctual_variant_pt VARCHAR(20) CHECK (instinctual_variant_pt IN ('autopreservacao', 'social', 'sexual')),

    -- Scores por tipo (para deteccao automatica)
    type_scores JSONB DEFAULT '{"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0}',

    -- Evidencias coletadas
    evidence_count INTEGER DEFAULT 0,
    last_evidence_at TIMESTAMPTZ,

    -- Metodo de identificacao
    identification_method VARCHAR(50) DEFAULT 'automatic', -- automatic, questionnaire, therapist
    identified_at TIMESTAMPTZ,
    identified_by VARCHAR(100), -- 'system' ou nome do profissional

    -- Notas clinicas (sem interpretacao da EVA, apenas registro)
    clinical_notes TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id)
);

-- Evidencias do Eneagrama (falas que indicam tipo)
CREATE TABLE IF NOT EXISTS enneagram_evidence (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Referencia a memoria episodica
    memory_id INTEGER,

    -- Texto exato que foi dito
    verbatim TEXT NOT NULL,

    -- Tipo que esta evidencia sugere
    suggested_type INTEGER REFERENCES enneagram_types(id),

    -- Peso da evidencia (0.0 a 1.0)
    weight DECIMAL(3,2) DEFAULT 0.50,

    -- Categoria da evidencia
    category VARCHAR(50) CHECK (category IN (
        'chief_feature',      -- Traco principal
        'defense_mechanism',  -- Mecanismo de defesa
        'passion',            -- Paixao/vicio
        'fixation',           -- Fixacao
        'keyword',            -- Palavra-chave tipica
        'speech_pattern',     -- Padrao de fala
        'behavior',           -- Comportamento descrito
        'relationship',       -- Padrao relacional
        'stress_response',    -- Resposta ao stress
        'integration_sign'    -- Sinal de integracao
    )),

    -- Contexto (sem interpretacao)
    context TEXT,

    -- Dados objetivos
    timestamp TIMESTAMPTZ NOT NULL,
    session_id VARCHAR(100),

    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- PARTE 2: MEMORIA IDENTITARIA (SELF_CORE DO PACIENTE)
-- Mapeia quem o paciente DIZ que e, ao longo do tempo
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_self_core (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Auto-descricoes literais (sem interpretacao)
    self_descriptions JSONB DEFAULT '[]', -- Array de {text, timestamp, context}

    -- Papeis que ele se atribui
    self_attributed_roles JSONB DEFAULT '[]', -- "pai", "provedor", "inutil"

    -- Linha narrativa mestra (compilacao das falas dele, nao interpretacao)
    -- Atualizada mensalmente
    narrative_summary TEXT,
    narrative_last_updated TIMESTAMPTZ,

    -- Evolucao temporal do self-concept
    self_concept_timeline JSONB DEFAULT '[]', -- {period, descriptions, dominant_theme}

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id)
);

-- Significantes Mestres (palavras que o paciente usa >N vezes sobre si)
CREATE TABLE IF NOT EXISTS patient_master_signifiers (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- A palavra/expressao exata
    signifier VARCHAR(200) NOT NULL,

    -- Contexto: sobre si mesmo, sobre outros, sobre o mundo
    context_type VARCHAR(20) CHECK (context_type IN ('self', 'other', 'world')),

    -- Contagem total
    total_count INTEGER DEFAULT 1,

    -- Primeira e ultima ocorrencia
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,

    -- Frequencia por periodo
    frequency_by_period JSONB DEFAULT '{}', -- {"2024-01": 5, "2024-02": 8}

    -- Tom emocional medio (dado objetivo de prosadia, nao interpretacao)
    avg_emotional_valence DECIMAL(3,2), -- -1.0 a 1.0

    -- Constelacoes (palavras que aparecem junto)
    co_occurring_signifiers JSONB DEFAULT '[]',

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, signifier)
);

-- =====================================================
-- PARTE 3: MEMORIA PROCEDURAL/IMPLICITA
-- Padroes automaticos DO PACIENTE (nao da EVA)
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_behavioral_patterns (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Tipo de padrao
    pattern_type VARCHAR(50) NOT NULL CHECK (pattern_type IN (
        'relational',       -- Padrao com pessoa especifica
        'emotional_trigger', -- Gatilho emocional
        'circadian',        -- Ritmo circadiano
        'avoidance',        -- Evitacao de tema
        'defense',          -- Mecanismo de defesa
        'transfer'          -- Transferencia
    )),

    -- Nome do padrao (descritivo, sem julgamento)
    pattern_name VARCHAR(200) NOT NULL,

    -- Condicao de ativacao (quando acontece)
    trigger_condition JSONB NOT NULL, -- {type, value, context}

    -- Resposta tipica (o que ele faz)
    typical_response JSONB NOT NULL, -- {behavior, verbal, nonverbal}

    -- Estatisticas objetivas
    occurrence_count INTEGER DEFAULT 0,
    probability DECIMAL(3,2) DEFAULT 0.00, -- 0.00 a 1.00

    -- Aprendido de quantas interacoes
    learned_from_count INTEGER DEFAULT 0,

    -- Timestamps
    first_observed TIMESTAMPTZ,
    last_observed TIMESTAMPTZ,
    last_updated TIMESTAMPTZ DEFAULT NOW(),

    -- Efetividade de intervencoes (o que funciona com ele)
    effective_interventions JSONB DEFAULT '[]',

    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Ritmos Circadianos Psiquicos
CREATE TABLE IF NOT EXISTS patient_circadian_patterns (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Periodo do dia
    time_period VARCHAR(20) NOT NULL CHECK (time_period IN (
        'early_morning',   -- 5-8h
        'morning',         -- 8-12h
        'afternoon',       -- 12-18h
        'evening',         -- 18-22h
        'night',           -- 22-2h
        'late_night'       -- 2-5h
    )),

    -- Dia da semana (opcional)
    day_of_week INTEGER CHECK (day_of_week BETWEEN 0 AND 6), -- 0=domingo

    -- Temas recorrentes neste periodo
    recurring_themes JSONB DEFAULT '[]',

    -- Tom emocional medio (dado objetivo)
    avg_emotional_tone DECIMAL(3,2), -- -1.0 a 1.0

    -- Estado tipico
    typical_state VARCHAR(100), -- "insonia_ruminativa", "alerta", "nostalgico"

    -- Contagem de observacoes
    observation_count INTEGER DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, time_period, day_of_week)
);

-- =====================================================
-- PARTE 4: MEMORIA PROSPECTIVA
-- Intencoes declaradas vs acoes realizadas
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_intentions (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Intencao declarada (texto exato)
    intention_verbatim TEXT NOT NULL,

    -- Categoria
    category VARCHAR(50) CHECK (category IN (
        'contact',          -- "vou ligar para X"
        'health',           -- "vou tomar remedio"
        'activity',         -- "vou passear"
        'relationship',     -- "vou fazer as pazes"
        'self_care',        -- "vou descansar"
        'other'
    )),

    -- Pessoa envolvida (se houver)
    related_person VARCHAR(200),

    -- Status
    status VARCHAR(20) DEFAULT 'declared' CHECK (status IN (
        'declared',         -- Disse que vai fazer
        'in_progress',      -- Mencionou estar fazendo
        'completed',        -- Mencionou ter feito
        'abandoned',        -- Nunca mais mencionou
        'blocked'           -- Disse que nao conseguiu
    )),

    -- Quantas vezes declarou a mesma intencao
    declaration_count INTEGER DEFAULT 1,

    -- Timestamps
    first_declared TIMESTAMPTZ NOT NULL,
    last_declared TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,

    -- Referencia a memoria onde completou (se houver)
    completion_memory_id INTEGER,

    -- Bloqueio identificado (se paciente verbalizou)
    stated_blocker TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- PARTE 5: MEMORIA CONTRA-FACTUAL
-- Os "e se?" que o paciente rumina
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_counterfactuals (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Texto exato do "e se"
    verbatim TEXT NOT NULL,

    -- Ponto de bifurcacao (quando na vida dele)
    life_period VARCHAR(100), -- "juventude", "casamento", "trabalho"
    approximate_age INTEGER,

    -- Tema
    theme VARCHAR(100), -- "carreira", "relacionamento", "familia", "educacao"

    -- Estatisticas
    mention_count INTEGER DEFAULT 1,
    first_mentioned TIMESTAMPTZ NOT NULL,
    last_mentioned TIMESTAMPTZ NOT NULL,

    -- Dados prosodicos objetivos (media das mencoes)
    avg_pitch_variance DECIMAL(5,2),
    avg_pause_duration DECIMAL(5,2),
    voice_tremor_detected BOOLEAN DEFAULT FALSE,

    -- Tom emocional medio (dado objetivo, nao interpretacao)
    avg_emotional_valence DECIMAL(3,2), -- -1.0 a 1.0

    -- Correlacoes objetivas
    correlated_topics JSONB DEFAULT '[]', -- Temas que aparecem junto
    correlated_persons JSONB DEFAULT '[]', -- Pessoas mencionadas junto

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, verbatim)
);

-- =====================================================
-- PARTE 6: MEMORIA METAFORICA
-- Dicionario pessoal de metaforas DO PACIENTE
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_metaphors (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- A metafora exata
    metaphor TEXT NOT NULL,

    -- Tipo de metafora
    metaphor_type VARCHAR(50) CHECK (metaphor_type IN (
        'corporal',         -- "peso no peito", "vazio"
        'spatial',          -- "num buraco", "perdido"
        'temporal',         -- "relogio parou"
        'relational',       -- "sozinho no mundo"
        'existential',      -- "vida nao tem sentido"
        'other'
    )),

    -- Estatisticas de uso
    usage_count INTEGER DEFAULT 1,
    first_used TIMESTAMPTZ NOT NULL,
    last_used TIMESTAMPTZ NOT NULL,

    -- Contextos em que aparece (dados objetivos)
    contexts JSONB DEFAULT '[]', -- Array de {topic, person, time_of_day}

    -- Correlacoes objetivas
    correlated_topics JSONB DEFAULT '[]',
    correlated_emotions JSONB DEFAULT '[]', -- Emocoes detectadas junto
    correlated_persons JSONB DEFAULT '[]',

    -- Dados prosodicos medios
    avg_prosodic_data JSONB DEFAULT '{}',

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, metaphor)
);

-- =====================================================
-- PARTE 7: MEMORIA TRANSGERACIONAL
-- Padroes familiares que O PACIENTE descreve
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_family_patterns (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Padrao identificado nas falas dele
    pattern_verbatim TEXT NOT NULL, -- "meu pai tambem...", "na minha familia..."

    -- Tipo de padrao
    pattern_type VARCHAR(50) CHECK (pattern_type IN (
        'inherited_behavior',  -- "meu pai tambem fazia"
        'family_mandate',      -- "na minha familia nao se..."
        'generational_trauma', -- Trauma mencionado de geracoes anteriores
        'family_secret',       -- Coisas que ele menciona nao se falar
        'repetition'           -- Padrao que ele percebe se repetir
    )),

    -- Geracoes envolvidas
    generations_mentioned JSONB DEFAULT '[]', -- ["avo", "pai", "eu", "filho"]

    -- Estatisticas
    mention_count INTEGER DEFAULT 1,
    first_mentioned TIMESTAMPTZ NOT NULL,
    last_mentioned TIMESTAMPTZ NOT NULL,

    -- Tom emocional (dado objetivo)
    avg_emotional_valence DECIMAL(3,2),

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- PARTE 8: MEMORIA SOMATICA
-- Correlacao corpo-fala DO PACIENTE
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_somatic_correlations (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Dado somatico
    somatic_type VARCHAR(50) NOT NULL CHECK (somatic_type IN (
        'blood_glucose',    -- Glicemia
        'blood_pressure',   -- Pressao
        'heart_rate',       -- Frequencia cardiaca
        'sleep_quality',    -- Qualidade do sono
        'pain_level',       -- Nivel de dor
        'medication_adherence' -- Adesao a medicacao
    )),

    -- Condicao (range do dado somatico)
    condition_range VARCHAR(50) NOT NULL, -- "high", "low", "normal", ">180", "<70"

    -- Tema correlacionado
    correlated_topic VARCHAR(200) NOT NULL,

    -- Forca da correlacao (0.0 a 1.0)
    correlation_strength DECIMAL(3,2) NOT NULL,

    -- Baseado em quantas observacoes
    observation_count INTEGER DEFAULT 0,

    -- Direcao da correlacao
    direction VARCHAR(20) CHECK (direction IN ('positive', 'negative', 'neutral')),

    -- Timestamps
    first_observed TIMESTAMPTZ,
    last_observed TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, somatic_type, condition_range, correlated_topic)
);

-- =====================================================
-- PARTE 9: MEMORIA SOCIAL/CULTURAL
-- Contexto historico que O PACIENTE viveu
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_cultural_context (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Dados objetivos
    birth_year INTEGER,
    birth_region VARCHAR(100),

    -- Eventos historicos que ele MENCIONA ter vivido
    mentioned_historical_events JSONB DEFAULT '[]',

    -- Valores que ele EXPRESSA como "da minha epoca"
    expressed_generational_values JSONB DEFAULT '[]',

    -- Expressoes/linguagem geracional que ele USA
    generational_expressions JSONB DEFAULT '[]',

    -- Conflitos que ele VERBALIZA (entre valores antigos e atuais)
    expressed_value_conflicts JSONB DEFAULT '[]',

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id)
);

-- =====================================================
-- PARTE 10: MEMORIA DE APRENDIZADO
-- O que funciona com ESTE paciente especifico
-- (Calibracao do instrumento, nao opiniao)
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_effective_approaches (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Tipo de abordagem
    approach_type VARCHAR(100) NOT NULL,

    -- Descricao objetiva da abordagem
    approach_description TEXT NOT NULL,

    -- Metrica de efetividade
    effectiveness_metric VARCHAR(50) CHECK (effectiveness_metric IN (
        'elaboration_length',   -- Ele falou mais depois
        'insight_verbalized',   -- Ele disse "nunca tinha pensado"
        'topic_continued',      -- Continuou no tema
        'emotional_shift',      -- Mudanca no tom (dado objetivo)
        'session_extended',     -- Ficou mais tempo
        'return_to_topic'       -- Voltou ao tema depois
    )),

    -- Score de efetividade (0.0 a 1.0)
    effectiveness_score DECIMAL(3,2) NOT NULL,

    -- Baseado em quantas observacoes
    observation_count INTEGER DEFAULT 0,

    -- Condicoes em que funciona
    conditions JSONB DEFAULT '{}', -- {time_of_day, emotional_state, topic}

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, approach_type, approach_description)
);

-- Silencio otimo por paciente
CREATE TABLE IF NOT EXISTS patient_optimal_silence (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Contexto
    context_type VARCHAR(50) NOT NULL, -- "rumination", "grief", "anger", "general"

    -- Duracao otima do silencio (em segundos)
    optimal_duration_seconds DECIMAL(5,2) NOT NULL,

    -- Range efetivo (min-max)
    min_effective_seconds DECIMAL(5,2),
    max_effective_seconds DECIMAL(5,2),

    -- Efetividade
    effectiveness_score DECIMAL(3,2),

    -- Baseado em quantas observacoes
    observation_count INTEGER DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, context_type)
);

-- =====================================================
-- PARTE 11: MEMORIA PREDITIVA DE CRISE
-- Padroes que PRECEDEM crises (dados objetivos)
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_crisis_predictors (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Tipo de crise que este padrao prediz
    crisis_type VARCHAR(50) NOT NULL CHECK (crisis_type IN (
        'depression_severe',
        'suicidal_ideation',
        'hospitalization',
        'social_isolation',
        'medication_crisis',
        'family_conflict',
        'health_deterioration'
    )),

    -- Marcador preditivo
    predictor_type VARCHAR(50) NOT NULL CHECK (predictor_type IN (
        'word_frequency',       -- Aumento na frequencia de palavra
        'topic_emergence',      -- Topico novo ou ressurgente
        'prosodic_change',      -- Mudanca prosodica
        'biometric_pattern',    -- Padrao biometrico
        'behavioral_change',    -- Mudanca comportamental
        'temporal_marker',      -- Data significativa
        'social_withdrawal'     -- Reducao de contato
    )),

    -- Descricao do marcador
    predictor_description TEXT NOT NULL,

    -- Peso preditivo (0.0 a 1.0)
    predictive_weight DECIMAL(3,2) NOT NULL,

    -- Janela temporal (quantos dias antes da crise este marcador aparece)
    lead_time_days INTEGER,

    -- Baseado em quantas crises anteriores
    based_on_crisis_count INTEGER DEFAULT 0,

    -- Acuracia historica
    historical_accuracy DECIMAL(3,2),

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Score de risco em tempo real
CREATE TABLE IF NOT EXISTS patient_risk_scores (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Timestamp do calculo
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Scores por tipo de risco (0.0 a 1.0)
    risk_depression_severe DECIMAL(3,2) DEFAULT 0.00,
    risk_suicidal_30d DECIMAL(3,2) DEFAULT 0.00,
    risk_hospitalization_90d DECIMAL(3,2) DEFAULT 0.00,
    risk_social_isolation DECIMAL(3,2) DEFAULT 0.00,

    -- Score geral
    overall_risk_score DECIMAL(3,2) DEFAULT 0.00,

    -- Marcadores ativos que contribuiram para o score
    active_markers JSONB DEFAULT '[]',

    -- Nivel de alerta
    alert_level VARCHAR(20) CHECK (alert_level IN ('low', 'moderate', 'high', 'critical')),

    -- Acao recomendada (sem interpretacao, baseado em regras)
    recommended_action VARCHAR(100)
);

-- =====================================================
-- PARTE 12: MEMORIA SEMANTICA EXPANDIDA
-- Mapa do mundo DO PACIENTE
-- =====================================================

-- Pessoas no mundo do paciente
CREATE TABLE IF NOT EXISTS patient_world_persons (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Nome da pessoa
    person_name VARCHAR(200) NOT NULL,

    -- Papel (como o paciente a descreve)
    role VARCHAR(100), -- "filha", "medico", "vizinha"

    -- Valencia emocional (baseada nas mencoes do paciente)
    emotional_valence DECIMAL(3,2), -- -1.0 a 1.0

    -- Frequencia de mencao
    mention_count INTEGER DEFAULT 0,
    first_mentioned TIMESTAMPTZ,
    last_mentioned TIMESTAMPTZ,

    -- Topicos associados (o que ele fala quando menciona esta pessoa)
    associated_topics JSONB DEFAULT '[]',

    -- Evolucao do relacionamento (como ele descreve ao longo do tempo)
    relationship_timeline JSONB DEFAULT '[]', -- {period, description, valence}

    -- Status atual (como ele descreve)
    current_status VARCHAR(100), -- "distante", "proximo", "conflito"

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, person_name)
);

-- Lugares no mundo do paciente
CREATE TABLE IF NOT EXISTS patient_world_places (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Nome do lugar
    place_name VARCHAR(200) NOT NULL,

    -- Tipo
    place_type VARCHAR(50), -- "casa_infancia", "hospital", "trabalho"

    -- Carga emocional (baseada nas mencoes)
    emotional_valence DECIMAL(3,2), -- -1.0 a 1.0

    -- Periodo temporal associado
    temporal_period VARCHAR(100), -- "1950-1980"

    -- Frequencia de mencao
    mention_count INTEGER DEFAULT 0,

    -- Memorias associadas (IDs das memorias episodicas)
    associated_memory_ids JSONB DEFAULT '[]',

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, place_name)
);

-- Objetos significantes
CREATE TABLE IF NOT EXISTS patient_world_objects (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Nome do objeto
    object_name VARCHAR(200) NOT NULL,

    -- Funcao psiquica (como o paciente descreve)
    described_significance TEXT,

    -- Frequencia de mencao
    mention_count INTEGER DEFAULT 0,

    -- Pessoa/evento associado
    associated_person VARCHAR(200),
    associated_event TEXT,

    -- Carga emocional
    emotional_valence DECIMAL(3,2),

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, object_name)
);

-- =====================================================
-- INDICES PARA PERFORMANCE
-- =====================================================

CREATE INDEX IF NOT EXISTS idx_patient_enneagram_idoso ON patient_enneagram(idoso_id);
CREATE INDEX IF NOT EXISTS idx_patient_enneagram_type ON patient_enneagram(primary_type);
CREATE INDEX IF NOT EXISTS idx_enneagram_evidence_idoso ON enneagram_evidence(idoso_id);
CREATE INDEX IF NOT EXISTS idx_enneagram_evidence_type ON enneagram_evidence(suggested_type);

CREATE INDEX IF NOT EXISTS idx_self_core_idoso ON patient_self_core(idoso_id);
CREATE INDEX IF NOT EXISTS idx_master_signifiers_idoso ON patient_master_signifiers(idoso_id);
CREATE INDEX IF NOT EXISTS idx_master_signifiers_count ON patient_master_signifiers(total_count DESC);

CREATE INDEX IF NOT EXISTS idx_behavioral_patterns_idoso ON patient_behavioral_patterns(idoso_id);
CREATE INDEX IF NOT EXISTS idx_behavioral_patterns_type ON patient_behavioral_patterns(pattern_type);

CREATE INDEX IF NOT EXISTS idx_circadian_patterns_idoso ON patient_circadian_patterns(idoso_id);
CREATE INDEX IF NOT EXISTS idx_circadian_patterns_period ON patient_circadian_patterns(time_period);

CREATE INDEX IF NOT EXISTS idx_intentions_idoso ON patient_intentions(idoso_id);
CREATE INDEX IF NOT EXISTS idx_intentions_status ON patient_intentions(status);

CREATE INDEX IF NOT EXISTS idx_counterfactuals_idoso ON patient_counterfactuals(idoso_id);
CREATE INDEX IF NOT EXISTS idx_counterfactuals_count ON patient_counterfactuals(mention_count DESC);

CREATE INDEX IF NOT EXISTS idx_metaphors_idoso ON patient_metaphors(idoso_id);
CREATE INDEX IF NOT EXISTS idx_metaphors_type ON patient_metaphors(metaphor_type);

CREATE INDEX IF NOT EXISTS idx_family_patterns_idoso ON patient_family_patterns(idoso_id);
CREATE INDEX IF NOT EXISTS idx_somatic_correlations_idoso ON patient_somatic_correlations(idoso_id);
CREATE INDEX IF NOT EXISTS idx_cultural_context_idoso ON patient_cultural_context(idoso_id);

CREATE INDEX IF NOT EXISTS idx_effective_approaches_idoso ON patient_effective_approaches(idoso_id);
CREATE INDEX IF NOT EXISTS idx_optimal_silence_idoso ON patient_optimal_silence(idoso_id);

CREATE INDEX IF NOT EXISTS idx_crisis_predictors_idoso ON patient_crisis_predictors(idoso_id);
CREATE INDEX IF NOT EXISTS idx_crisis_predictors_type ON patient_crisis_predictors(crisis_type);
CREATE INDEX IF NOT EXISTS idx_risk_scores_idoso ON patient_risk_scores(idoso_id);
CREATE INDEX IF NOT EXISTS idx_risk_scores_time ON patient_risk_scores(calculated_at DESC);

CREATE INDEX IF NOT EXISTS idx_world_persons_idoso ON patient_world_persons(idoso_id);
CREATE INDEX IF NOT EXISTS idx_world_places_idoso ON patient_world_places(idoso_id);
CREATE INDEX IF NOT EXISTS idx_world_objects_idoso ON patient_world_objects(idoso_id);

-- =====================================================
-- VIEWS PARA ESPELHAMENTO LACANIANO
-- Retornam dados objetivos sem interpretacao
-- =====================================================

-- View: Perfil completo do paciente (dados objetivos)
CREATE OR REPLACE VIEW v_patient_mirror_profile AS
SELECT
    i.id as idoso_id,
    i.nome as patient_name,

    -- Eneagrama
    pe.primary_type as enneagram_type,
    et.name_pt as enneagram_name,
    et.center_pt as enneagram_center,
    et.chief_feature_pt as chief_feature,
    pe.primary_type_confidence,
    pe.health_level,

    -- Self-Core
    psc.self_descriptions,
    psc.self_attributed_roles,
    psc.narrative_summary,

    -- Estatisticas objetivas
    (SELECT COUNT(*) FROM episodic_memories em WHERE em.idoso_id = i.id) as total_memories,
    (SELECT COUNT(*) FROM patient_master_signifiers pms WHERE pms.idoso_id = i.id) as unique_signifiers,
    (SELECT COUNT(*) FROM patient_intentions pi WHERE pi.idoso_id = i.id AND pi.status = 'declared') as pending_intentions,
    (SELECT COUNT(*) FROM patient_counterfactuals pc WHERE pc.idoso_id = i.id) as counterfactual_themes,

    -- Risco atual
    prs.overall_risk_score,
    prs.alert_level,
    prs.calculated_at as risk_calculated_at

FROM idosos i
LEFT JOIN patient_enneagram pe ON i.id = pe.idoso_id
LEFT JOIN enneagram_types et ON pe.primary_type = et.id
LEFT JOIN patient_self_core psc ON i.id = psc.idoso_id
LEFT JOIN LATERAL (
    SELECT * FROM patient_risk_scores
    WHERE idoso_id = i.id
    ORDER BY calculated_at DESC
    LIMIT 1
) prs ON true;

-- View: Significantes mais frequentes (para devolver ao paciente)
CREATE OR REPLACE VIEW v_patient_top_signifiers AS
SELECT
    idoso_id,
    signifier,
    total_count,
    context_type,
    first_seen,
    last_seen,
    avg_emotional_valence,
    co_occurring_signifiers
FROM patient_master_signifiers
ORDER BY idoso_id, total_count DESC;

-- View: Intencoes nao realizadas (para devolver ao paciente)
CREATE OR REPLACE VIEW v_patient_unrealized_intentions AS
SELECT
    idoso_id,
    intention_verbatim,
    category,
    related_person,
    declaration_count,
    first_declared,
    last_declared,
    EXTRACT(DAY FROM NOW() - first_declared) as days_since_first_declared,
    stated_blocker
FROM patient_intentions
WHERE status IN ('declared', 'blocked')
ORDER BY idoso_id, declaration_count DESC;

-- View: Correlacoes somaticas fortes
CREATE OR REPLACE VIEW v_patient_strong_somatic_correlations AS
SELECT
    idoso_id,
    somatic_type,
    condition_range,
    correlated_topic,
    correlation_strength,
    observation_count
FROM patient_somatic_correlations
WHERE correlation_strength >= 0.6
ORDER BY idoso_id, correlation_strength DESC;

-- =====================================================
-- FUNCOES PARA DETECCAO AUTOMATICA DE ENEAGRAMA
-- =====================================================

-- Funcao para atualizar score do eneagrama baseado em evidencias
CREATE OR REPLACE FUNCTION update_enneagram_scores(p_idoso_id INTEGER)
RETURNS VOID AS $$
DECLARE
    v_scores JSONB;
    v_type INTEGER;
    v_total_weight DECIMAL;
    v_type_weight DECIMAL;
BEGIN
    -- Inicializar scores
    v_scores := '{"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0}'::JSONB;

    -- Calcular peso total por tipo
    FOR v_type IN 1..9 LOOP
        SELECT COALESCE(SUM(weight), 0) INTO v_type_weight
        FROM enneagram_evidence
        WHERE idoso_id = p_idoso_id AND suggested_type = v_type;

        v_scores := jsonb_set(v_scores, ARRAY[v_type::TEXT], to_jsonb(v_type_weight));
    END LOOP;

    -- Calcular peso total
    SELECT COALESCE(SUM(weight), 0) INTO v_total_weight
    FROM enneagram_evidence
    WHERE idoso_id = p_idoso_id;

    -- Normalizar scores (0-1)
    IF v_total_weight > 0 THEN
        FOR v_type IN 1..9 LOOP
            v_type_weight := (v_scores->>v_type::TEXT)::DECIMAL / v_total_weight;
            v_scores := jsonb_set(v_scores, ARRAY[v_type::TEXT], to_jsonb(ROUND(v_type_weight, 2)));
        END LOOP;
    END IF;

    -- Atualizar ou inserir
    INSERT INTO patient_enneagram (idoso_id, type_scores, evidence_count, last_evidence_at)
    VALUES (p_idoso_id, v_scores,
            (SELECT COUNT(*) FROM enneagram_evidence WHERE idoso_id = p_idoso_id),
            NOW())
    ON CONFLICT (idoso_id) DO UPDATE SET
        type_scores = EXCLUDED.type_scores,
        evidence_count = EXCLUDED.evidence_count,
        last_evidence_at = NOW(),
        updated_at = NOW();

    -- Atualizar tipo primario se confianca > 0.3
    UPDATE patient_enneagram
    SET primary_type = (
        SELECT key::INTEGER
        FROM jsonb_each_text(type_scores)
        ORDER BY value::DECIMAL DESC
        LIMIT 1
    ),
    primary_type_confidence = (
        SELECT MAX(value::DECIMAL)
        FROM jsonb_each_text(type_scores)
    )
    WHERE idoso_id = p_idoso_id
    AND (SELECT MAX(value::DECIMAL) FROM jsonb_each_text(type_scores)) >= 0.3;

END;
$$ LANGUAGE plpgsql;

-- Funcao para calcular score de risco
CREATE OR REPLACE FUNCTION calculate_risk_score(p_idoso_id INTEGER)
RETURNS VOID AS $$
DECLARE
    v_risk_depression DECIMAL := 0;
    v_risk_suicidal DECIMAL := 0;
    v_risk_hospital DECIMAL := 0;
    v_risk_isolation DECIMAL := 0;
    v_overall DECIMAL;
    v_alert VARCHAR(20);
    v_markers JSONB := '[]'::JSONB;
BEGIN
    -- Buscar preditores ativos e calcular scores
    SELECT
        COALESCE(SUM(CASE WHEN crisis_type = 'depression_severe' THEN predictive_weight ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN crisis_type = 'suicidal_ideation' THEN predictive_weight ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN crisis_type = 'hospitalization' THEN predictive_weight ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN crisis_type = 'social_isolation' THEN predictive_weight ELSE 0 END), 0)
    INTO v_risk_depression, v_risk_suicidal, v_risk_hospital, v_risk_isolation
    FROM patient_crisis_predictors
    WHERE idoso_id = p_idoso_id;

    -- Normalizar (max 1.0)
    v_risk_depression := LEAST(v_risk_depression, 1.0);
    v_risk_suicidal := LEAST(v_risk_suicidal, 1.0);
    v_risk_hospital := LEAST(v_risk_hospital, 1.0);
    v_risk_isolation := LEAST(v_risk_isolation, 1.0);

    -- Score geral (media ponderada, suicidal tem peso maior)
    v_overall := (v_risk_depression * 0.25 + v_risk_suicidal * 0.40 +
                  v_risk_hospital * 0.20 + v_risk_isolation * 0.15);

    -- Determinar nivel de alerta
    v_alert := CASE
        WHEN v_overall >= 0.7 OR v_risk_suicidal >= 0.5 THEN 'critical'
        WHEN v_overall >= 0.5 THEN 'high'
        WHEN v_overall >= 0.3 THEN 'moderate'
        ELSE 'low'
    END;

    -- Inserir score
    INSERT INTO patient_risk_scores (
        idoso_id,
        risk_depression_severe,
        risk_suicidal_30d,
        risk_hospitalization_90d,
        risk_social_isolation,
        overall_risk_score,
        alert_level,
        active_markers
    ) VALUES (
        p_idoso_id,
        v_risk_depression,
        v_risk_suicidal,
        v_risk_hospital,
        v_risk_isolation,
        v_overall,
        v_alert,
        v_markers
    );

END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- TRIGGERS
-- =====================================================

-- Trigger para atualizar scores do eneagrama quando nova evidencia
CREATE OR REPLACE FUNCTION trg_update_enneagram_on_evidence()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM update_enneagram_scores(NEW.idoso_id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_enneagram_evidence_insert
    AFTER INSERT ON enneagram_evidence
    FOR EACH ROW
    EXECUTE FUNCTION trg_update_enneagram_on_evidence();

-- =====================================================
-- COMENTARIOS
-- =====================================================

COMMENT ON TABLE enneagram_types IS 'Tipos do Eneagrama baseado em Gurdjieff/Naranjo - como o paciente esta preso';
COMMENT ON TABLE patient_enneagram IS 'Avaliacao do Eneagrama do paciente - sem julgamento, apenas mapeamento';
COMMENT ON TABLE enneagram_evidence IS 'Evidencias objetivas de falas que indicam tipo do Eneagrama';

COMMENT ON TABLE patient_self_core IS 'Memoria Identitaria - quem o paciente DIZ que e';
COMMENT ON TABLE patient_master_signifiers IS 'Significantes mestres - palavras que o paciente repete sobre si';

COMMENT ON TABLE patient_behavioral_patterns IS 'Padroes comportamentais objetivos do paciente';
COMMENT ON TABLE patient_circadian_patterns IS 'Ritmos circadianos psiquicos do paciente';

COMMENT ON TABLE patient_intentions IS 'Intencoes declaradas vs realizadas - espelho sem julgamento';
COMMENT ON TABLE patient_counterfactuals IS 'Os e se que o paciente rumina - dados objetivos';

COMMENT ON TABLE patient_metaphors IS 'Dicionario pessoal de metaforas do paciente';
COMMENT ON TABLE patient_family_patterns IS 'Padroes transgeracionais que o paciente DESCREVE';

COMMENT ON TABLE patient_somatic_correlations IS 'Correlacoes objetivas corpo-fala';
COMMENT ON TABLE patient_cultural_context IS 'Contexto historico que o paciente MENCIONA';

COMMENT ON TABLE patient_effective_approaches IS 'O que funciona com este paciente - calibracao do instrumento';
COMMENT ON TABLE patient_optimal_silence IS 'Duracao otima de silencio por contexto';

COMMENT ON TABLE patient_crisis_predictors IS 'Marcadores preditivos de crise - dados objetivos';
COMMENT ON TABLE patient_risk_scores IS 'Scores de risco calculados automaticamente';

COMMENT ON TABLE patient_world_persons IS 'Pessoas no mundo do paciente como ELE as descreve';
COMMENT ON TABLE patient_world_places IS 'Lugares no mundo do paciente como ELE os descreve';
COMMENT ON TABLE patient_world_objects IS 'Objetos significantes como o paciente os descreve';

-- =====================================================
-- FIM DA MIGRATION
-- =====================================================
