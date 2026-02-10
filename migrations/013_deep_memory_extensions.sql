-- =====================================================
-- EVA-Mind-FZPN: EXTENSOES PROFUNDAS DE MEMORIA
-- Baseado em eva-memoria.md (Schacter, Van der Kolk, Casey)
--
-- PRINCIPIO: EVA NAO INTERPRETA. EVA TECE NARRATIVA OBJETIVA.
-- Conecta dados temporalmente sem adicionar opiniao.
-- =====================================================

-- =====================================================
-- PARTE 1: PERSISTENCIA TRAUMATICA (Schacter - 7o Pecado)
-- Memorias que o paciente tenta evitar mas voltam sempre
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_persistent_memories (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- O tema que persiste
    persistent_topic TEXT NOT NULL,

    -- Palavras-chave associadas
    keywords JSONB DEFAULT '[]',

    -- Evidencias de evitacao
    avoidance_attempts INTEGER DEFAULT 0,      -- Quantas vezes mudou de assunto
    avoidance_phrases JSONB DEFAULT '[]',      -- "nao quero falar", "deixa pra la"

    -- Evidencias de retorno
    return_count INTEGER DEFAULT 0,            -- Quantas vezes voltou ao tema
    avg_days_between_returns DECIMAL(5,2),     -- Intervalo medio entre retornos

    -- Indicadores de distress (dados objetivos)
    prosodic_distress_avg DECIMAL(3,2),        -- Score medio de distress vocal
    voice_tremor_percentage DECIMAL(3,2),      -- % de mencoes com tremor
    long_pause_percentage DECIMAL(3,2),        -- % de mencoes com pausas longas

    -- Sintomas fisicos relatados junto
    associated_physical_symptoms JSONB DEFAULT '[]',

    -- Padroes temporais
    typical_triggers JSONB DEFAULT '[]',       -- O que dispara o tema
    typical_time_periods JSONB DEFAULT '[]',   -- Horarios/datas que aparece mais
    anniversary_dates JSONB DEFAULT '[]',      -- Datas significativas

    -- Pessoas envolvidas
    involved_persons JSONB DEFAULT '[]',

    -- Classificacao (baseada em padroes, nao interpretacao)
    persistence_score DECIMAL(3,2) DEFAULT 0,  -- 0-1: quanto insiste em voltar
    avoidance_score DECIMAL(3,2) DEFAULT 0,    -- 0-1: quanto tenta evitar

    -- Timestamps
    first_detected TIMESTAMPTZ,
    last_occurrence TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, persistent_topic)
);

-- Ocorrencias individuais do tema persistente
CREATE TABLE IF NOT EXISTS persistent_memory_occurrences (
    id SERIAL PRIMARY KEY,
    persistent_memory_id INTEGER REFERENCES patient_persistent_memories(id) ON DELETE CASCADE,

    -- Referencia a memoria episodica
    memory_id INTEGER,

    -- Tipo de ocorrencia
    occurrence_type VARCHAR(20) CHECK (occurrence_type IN (
        'mention',           -- Mencionou diretamente
        'avoidance',         -- Tentou evitar/mudar assunto
        'return',            -- Voltou ao tema apos evitar
        'triggered',         -- Foi disparado por algo
        'spontaneous'        -- Surgiu espontaneamente
    )),

    -- Contexto
    trigger_context TEXT,    -- O que disparou (se identificavel)
    verbatim TEXT,           -- O que disse

    -- Dados prosodicos objetivos
    voice_tremor BOOLEAN DEFAULT FALSE,
    pause_duration_avg DECIMAL(5,2),
    pitch_variance DECIMAL(5,2),

    -- Timestamp
    occurred_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- PARTE 2: MEMORIA DE LUGAR PROFUNDA (Casey)
-- Lugares que ancoram a identidade
-- =====================================================

-- Expandir tabela existente (se existir)
DO $$
BEGIN
    -- Adicionar colunas se nao existirem
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'patient_world_places'
                   AND column_name = 'identity_anchor_score') THEN
        ALTER TABLE patient_world_places ADD COLUMN identity_anchor_score DECIMAL(3,2);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'patient_world_places'
                   AND column_name = 'sensory_memories') THEN
        ALTER TABLE patient_world_places ADD COLUMN sensory_memories JSONB DEFAULT '{}';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'patient_world_places'
                   AND column_name = 'identity_period') THEN
        ALTER TABLE patient_world_places ADD COLUMN identity_period VARCHAR(100);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'patient_world_places'
                   AND column_name = 'years_lived') THEN
        ALTER TABLE patient_world_places ADD COLUMN years_lived INTEGER;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'patient_world_places'
                   AND column_name = 'associated_persons') THEN
        ALTER TABLE patient_world_places ADD COLUMN associated_persons JSONB DEFAULT '[]';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'patient_world_places'
                   AND column_name = 'associated_emotions') THEN
        ALTER TABLE patient_world_places ADD COLUMN associated_emotions JSONB DEFAULT '[]';
    END IF;
END $$;

-- Transicoes de lugar que marcaram a vida
CREATE TABLE IF NOT EXISTS patient_place_transitions (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- De onde para onde
    from_place VARCHAR(200),
    to_place VARCHAR(200),

    -- Quando
    transition_year INTEGER,
    transition_age INTEGER,

    -- Motivo (como ele descreve)
    stated_reason TEXT,

    -- Impacto na identidade (palavras dele)
    described_impact TEXT,

    -- Dados objetivos
    emotional_valence DECIMAL(3,2),  -- -1 a 1
    mention_count INTEGER DEFAULT 0,

    -- Classificacao da transicao
    transition_type VARCHAR(50) CHECK (transition_type IN (
        'voluntary',         -- Escolha propria
        'forced',            -- Forcada (guerra, trabalho, etc)
        'family',            -- Por motivo familiar
        'health',            -- Por saude
        'loss'               -- Perda do lugar (venda, destruicao)
    )),

    -- Saudade/nostalgia
    nostalgia_score DECIMAL(3,2),    -- Baseado em frequencia + tom

    first_mentioned TIMESTAMPTZ,
    last_mentioned TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Memorias sensoriais de lugares
CREATE TABLE IF NOT EXISTS patient_place_sensory_memories (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    place_id INTEGER REFERENCES patient_world_places(id) ON DELETE CASCADE,

    -- O lugar (se nao tiver ID)
    place_name VARCHAR(200),

    -- Tipo de memoria sensorial
    sensory_type VARCHAR(20) CHECK (sensory_type IN (
        'smell',     -- Cheiro
        'sound',     -- Som
        'taste',     -- Gosto
        'touch',     -- Tato/textura
        'visual',    -- Visual especifico
        'temperature' -- Temperatura/clima
    )),

    -- Descricao (palavras dele)
    description TEXT NOT NULL,

    -- Associacoes
    associated_emotions JSONB DEFAULT '[]',
    associated_persons JSONB DEFAULT '[]',

    -- Frequencia
    mention_count INTEGER DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- PARTE 3: REMEMORACAO COMPARTILHADA (Casey - Commemoration)
-- Memorias que o paciente quer/queria compartilhar
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_shared_memories (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- A memoria (resumo objetivo)
    memory_summary TEXT NOT NULL,

    -- Palavras exatas quando mencionou
    verbatim_mentions JSONB DEFAULT '[]',

    -- Com quem quer compartilhar
    intended_audience JSONB DEFAULT '[]',      -- ["netos", "filhos", "bisnetos"]

    -- Ja compartilhou com quem
    shared_with JSONB DEFAULT '[]',

    -- Status
    sharing_status VARCHAR(20) CHECK (sharing_status IN (
        'wishes_to_share',    -- Quer compartilhar
        'partially_shared',   -- Compartilhou com alguns
        'fully_shared',       -- Ja contou para todos
        'unable_to_share',    -- Nao consegue (pessoa faleceu, etc)
        'private'             -- Decidiu manter privado
    )),

    -- Tipo de conteudo
    memory_type VARCHAR(50) CHECK (memory_type IN (
        'life_lesson',        -- Licao de vida
        'family_history',     -- Historia da familia
        'love_story',         -- Historia de amor
        'achievement',        -- Conquista
        'warning',            -- Aviso/alerta
        'tradition',          -- Tradicao familiar
        'recipe',             -- Receita/conhecimento pratico
        'other'
    )),

    -- Urgencia percebida (baseada em frequencia de mencao)
    urgency_score DECIMAL(3,2),

    -- Ritual associado (se houver)
    associated_ritual VARCHAR(100),  -- "natal", "aniversario", "dia dos pais"

    -- Estatisticas
    mention_count INTEGER DEFAULT 0,
    first_mentioned TIMESTAMPTZ,
    last_mentioned TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Mensagens de legado nao entregues (conexao com exit_protocol)
CREATE TABLE IF NOT EXISTS patient_undelivered_messages (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Destinatario
    recipient_name VARCHAR(200) NOT NULL,
    recipient_relationship VARCHAR(100),

    -- Conteudo (como ele expressou)
    message_essence TEXT NOT NULL,
    verbatim_expressions JSONB DEFAULT '[]',

    -- Status
    delivery_status VARCHAR(20) CHECK (delivery_status IN (
        'unspoken',           -- Nunca disse
        'attempted',          -- Tentou mas nao conseguiu
        'written',            -- Escreveu mas nao entregou
        'delivered',          -- Entregou
        'impossible'          -- Impossivel (pessoa faleceu)
    )),

    -- Bloqueio (se ele verbalizou)
    stated_blocker TEXT,

    -- Importancia
    importance_score DECIMAL(3,2),
    mention_count INTEGER DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- PARTE 4: CORPO COMO MEMORIA (Van der Kolk - Profundo)
-- Sintomas fisicos que sao memorias somaticas
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_body_memories (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Sintoma fisico relatado
    physical_symptom TEXT NOT NULL,

    -- Localizacao no corpo
    body_location VARCHAR(100),

    -- Descricoes que ele usa
    patient_descriptions JSONB DEFAULT '[]',  -- "aperto", "peso", "facada"

    -- Correlacoes detectadas (dados objetivos)
    correlated_topics JSONB DEFAULT '[]',
    correlated_persons JSONB DEFAULT '[]',
    correlated_places JSONB DEFAULT '[]',
    correlated_times JSONB DEFAULT '[]',      -- Horarios, datas

    -- Forca das correlacoes
    strongest_correlation_topic TEXT,
    strongest_correlation_strength DECIMAL(3,2),

    -- Estatisticas
    occurrence_count INTEGER DEFAULT 0,
    first_reported TIMESTAMPTZ,
    last_reported TIMESTAMPTZ,

    -- O paciente fez a conexao?
    patient_aware BOOLEAN DEFAULT FALSE,
    patient_verbalization TEXT,  -- Se ele mesmo disse "quando penso em X, sinto Y"
    awareness_date TIMESTAMPTZ,

    -- Medico descartou causa fisica?
    medical_cleared BOOLEAN,
    medical_notes TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(idoso_id, physical_symptom, body_location)
);

-- Ocorrencias individuais de sintomas
CREATE TABLE IF NOT EXISTS body_memory_occurrences (
    id SERIAL PRIMARY KEY,
    body_memory_id INTEGER REFERENCES patient_body_memories(id) ON DELETE CASCADE,
    idoso_id INTEGER NOT NULL,

    -- Referencia a memoria episodica
    memory_id INTEGER,

    -- O que ele disse
    verbatim TEXT,

    -- Contexto temporal
    occurred_at TIMESTAMPTZ NOT NULL,
    time_of_day VARCHAR(20),

    -- Contexto tematico (o que estava falando antes)
    preceding_topics JSONB DEFAULT '[]',
    preceding_persons JSONB DEFAULT '[]',

    -- Intensidade relatada (se mencionou)
    reported_intensity INTEGER CHECK (reported_intensity BETWEEN 1 AND 10),

    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- PARTE 5: RECONSTRUCAO NARRATIVA (Schacter #2)
-- Conexoes temporais entre dados para tecer narrativa
-- =====================================================

-- Conexoes narrativas detectadas
CREATE TABLE IF NOT EXISTS patient_narrative_threads (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Nome/titulo do fio narrativo
    thread_name VARCHAR(200) NOT NULL,

    -- Elementos conectados
    connected_elements JSONB NOT NULL,  -- {persons: [], topics: [], places: [], symptoms: []}

    -- Linha do tempo
    timeline JSONB DEFAULT '[]',  -- [{date, event, element_type, element_name}]

    -- Forca da conexao (baseada em co-ocorrencia)
    connection_strength DECIMAL(3,2),

    -- Baseado em quantas observacoes
    evidence_count INTEGER DEFAULT 0,

    -- Texto narrativo objetivo (sem interpretacao)
    narrative_summary TEXT,

    -- Tipo de conexao
    connection_type VARCHAR(50) CHECK (connection_type IN (
        'causal_sequence',     -- A levou a B levou a C
        'emotional_cluster',   -- Aparecem juntos emocionalmente
        'temporal_pattern',    -- Padrao no tempo
        'person_topic',        -- Pessoa sempre ligada a tema
        'place_identity',      -- Lugar ligado a quem ele e
        'body_mind'            -- Conexao corpo-mente
    )),

    -- Perguntas geradas
    generated_questions JSONB DEFAULT '[]',

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Marcos temporais significativos
CREATE TABLE IF NOT EXISTS patient_life_markers (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- O marco
    marker_description TEXT NOT NULL,
    marker_year INTEGER,
    marker_age INTEGER,

    -- Tipo
    marker_type VARCHAR(50) CHECK (marker_type IN (
        'birth',              -- Nascimento (dele ou de alguem)
        'death',              -- Morte de alguem
        'marriage',           -- Casamento
        'divorce',            -- Separacao
        'move',               -- Mudanca de lugar
        'career',             -- Marco profissional
        'health',             -- Evento de saude
        'loss',               -- Perda (nao morte)
        'achievement',        -- Conquista
        'trauma',             -- Evento traumatico
        'other'
    )),

    -- Impacto (como ele descreve)
    described_impact TEXT,
    emotional_valence DECIMAL(3,2),

    -- O que mudou depois (palavras dele)
    before_description TEXT,  -- "antes eu era..."
    after_description TEXT,   -- "depois eu..."

    -- Frequencia de mencao
    mention_count INTEGER DEFAULT 0,

    -- Pessoas envolvidas
    involved_persons JSONB DEFAULT '[]',

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- INDICES
-- =====================================================

CREATE INDEX IF NOT EXISTS idx_persistent_memories_idoso ON patient_persistent_memories(idoso_id);
CREATE INDEX IF NOT EXISTS idx_persistent_memories_score ON patient_persistent_memories(persistence_score DESC);
CREATE INDEX IF NOT EXISTS idx_persistent_occurrences_memory ON persistent_memory_occurrences(persistent_memory_id);

CREATE INDEX IF NOT EXISTS idx_place_transitions_idoso ON patient_place_transitions(idoso_id);
CREATE INDEX IF NOT EXISTS idx_place_sensory_idoso ON patient_place_sensory_memories(idoso_id);

CREATE INDEX IF NOT EXISTS idx_shared_memories_idoso ON patient_shared_memories(idoso_id);
CREATE INDEX IF NOT EXISTS idx_shared_memories_status ON patient_shared_memories(sharing_status);
CREATE INDEX IF NOT EXISTS idx_undelivered_messages_idoso ON patient_undelivered_messages(idoso_id);

CREATE INDEX IF NOT EXISTS idx_body_memories_idoso ON patient_body_memories(idoso_id);
CREATE INDEX IF NOT EXISTS idx_body_memories_symptom ON patient_body_memories(physical_symptom);
CREATE INDEX IF NOT EXISTS idx_body_occurrences_memory ON body_memory_occurrences(body_memory_id);

CREATE INDEX IF NOT EXISTS idx_narrative_threads_idoso ON patient_narrative_threads(idoso_id);
CREATE INDEX IF NOT EXISTS idx_life_markers_idoso ON patient_life_markers(idoso_id);
CREATE INDEX IF NOT EXISTS idx_life_markers_year ON patient_life_markers(marker_year);

-- =====================================================
-- VIEWS PARA RECONSTRUCAO NARRATIVA
-- =====================================================

-- View: Memorias persistentes com alto score de evitacao+retorno
CREATE OR REPLACE VIEW v_traumatic_persistence AS
SELECT
    pm.idoso_id,
    pm.persistent_topic,
    pm.avoidance_attempts,
    pm.return_count,
    pm.persistence_score,
    pm.avoidance_score,
    pm.voice_tremor_percentage,
    pm.involved_persons,
    pm.typical_triggers,
    CASE
        WHEN pm.persistence_score > 0.7 AND pm.avoidance_score > 0.5
        THEN 'high_persistence_high_avoidance'
        WHEN pm.persistence_score > 0.7
        THEN 'high_persistence'
        WHEN pm.avoidance_score > 0.5
        THEN 'high_avoidance'
        ELSE 'moderate'
    END as pattern_classification
FROM patient_persistent_memories pm
WHERE pm.persistence_score > 0.3 OR pm.avoidance_score > 0.3
ORDER BY (pm.persistence_score + pm.avoidance_score) DESC;

-- View: Lugares que ancoram identidade
CREATE OR REPLACE VIEW v_identity_anchor_places AS
SELECT
    wp.idoso_id,
    wp.place_name,
    wp.place_type,
    wp.identity_period,
    wp.years_lived,
    wp.emotional_valence,
    wp.mention_count,
    wp.identity_anchor_score,
    wp.sensory_memories,
    wp.associated_persons,
    (SELECT COUNT(*) FROM patient_place_sensory_memories psm
     WHERE psm.place_id = wp.id) as sensory_memory_count
FROM patient_world_places wp
WHERE wp.identity_anchor_score > 0.5 OR wp.mention_count > 10
ORDER BY wp.identity_anchor_score DESC NULLS LAST, wp.mention_count DESC;

-- View: Memorias que quer compartilhar (urgentes)
CREATE OR REPLACE VIEW v_urgent_shared_memories AS
SELECT
    sm.idoso_id,
    sm.memory_summary,
    sm.intended_audience,
    sm.sharing_status,
    sm.memory_type,
    sm.urgency_score,
    sm.mention_count,
    sm.first_mentioned,
    EXTRACT(DAY FROM NOW() - sm.last_mentioned) as days_since_last_mention
FROM patient_shared_memories sm
WHERE sm.sharing_status IN ('wishes_to_share', 'partially_shared', 'unable_to_share')
ORDER BY sm.urgency_score DESC NULLS LAST, sm.mention_count DESC;

-- View: Sintomas corporais com forte correlacao
CREATE OR REPLACE VIEW v_strong_body_correlations AS
SELECT
    bm.idoso_id,
    bm.physical_symptom,
    bm.body_location,
    bm.strongest_correlation_topic,
    bm.strongest_correlation_strength,
    bm.occurrence_count,
    bm.patient_aware,
    bm.correlated_persons,
    bm.patient_descriptions
FROM patient_body_memories bm
WHERE bm.strongest_correlation_strength > 0.5
ORDER BY bm.strongest_correlation_strength DESC;

-- =====================================================
-- FUNCOES PARA RECONSTRUCAO NARRATIVA
-- =====================================================

-- Funcao para gerar narrativa conectando dados
CREATE OR REPLACE FUNCTION generate_narrative_thread(
    p_idoso_id INTEGER,
    p_topic TEXT
) RETURNS TABLE (
    element_type TEXT,
    element_name TEXT,
    connection_strength DECIMAL,
    timeline_position INTEGER
) AS $$
BEGIN
    RETURN QUERY

    -- Buscar pessoas correlacionadas
    SELECT
        'person'::TEXT,
        jsonb_array_elements_text(correlated_persons),
        correlation_strength,
        1
    FROM patient_somatic_correlations
    WHERE idoso_id = p_idoso_id AND correlated_topic = p_topic

    UNION ALL

    -- Buscar lugares correlacionados
    SELECT
        'place'::TEXT,
        place_name,
        emotional_valence + 1,  -- Normalizar para 0-2
        2
    FROM patient_world_places
    WHERE idoso_id = p_idoso_id
    AND associated_emotions::text ILIKE '%' || p_topic || '%'

    UNION ALL

    -- Buscar sintomas corporais correlacionados
    SELECT
        'body_symptom'::TEXT,
        physical_symptom,
        strongest_correlation_strength,
        3
    FROM patient_body_memories
    WHERE idoso_id = p_idoso_id
    AND strongest_correlation_topic = p_topic

    ORDER BY connection_strength DESC NULLS LAST;
END;
$$ LANGUAGE plpgsql;

-- Funcao para calcular score de persistencia
CREATE OR REPLACE FUNCTION update_persistence_scores(p_idoso_id INTEGER)
RETURNS VOID AS $$
BEGIN
    UPDATE patient_persistent_memories
    SET
        persistence_score = LEAST(1.0,
            (return_count::DECIMAL / GREATEST(1, avoidance_attempts + return_count)) *
            (1 + voice_tremor_percentage)
        ),
        avoidance_score = LEAST(1.0,
            avoidance_attempts::DECIMAL / GREATEST(1, avoidance_attempts + return_count)
        ),
        updated_at = NOW()
    WHERE idoso_id = p_idoso_id;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- COMENTARIOS
-- =====================================================

COMMENT ON TABLE patient_persistent_memories IS 'Memorias traumaticas que persistem (7o pecado de Schacter)';
COMMENT ON TABLE patient_place_transitions IS 'Transicoes de lugar que marcaram a identidade (Casey)';
COMMENT ON TABLE patient_shared_memories IS 'Memorias que o paciente quer compartilhar - rememoracao (Casey)';
COMMENT ON TABLE patient_body_memories IS 'Sintomas fisicos como memorias somaticas (Van der Kolk)';
COMMENT ON TABLE patient_narrative_threads IS 'Conexoes narrativas entre elementos (Schacter #2)';
COMMENT ON TABLE patient_life_markers IS 'Marcos temporais significativos na vida do paciente';

COMMENT ON COLUMN patient_persistent_memories.persistence_score IS 'Score 0-1: quanto o tema insiste em voltar mesmo com evitacao';
COMMENT ON COLUMN patient_persistent_memories.avoidance_score IS 'Score 0-1: quanto o paciente tenta evitar o tema';
COMMENT ON COLUMN patient_body_memories.patient_aware IS 'Se o paciente verbalizou a conexao corpo-mente';
COMMENT ON COLUMN patient_shared_memories.urgency_score IS 'Score baseado em frequencia de mencao do desejo de compartilhar';

-- =====================================================
-- FIM DA MIGRATION
-- =====================================================
