-- =====================================================
-- MIGRATION 015: CRITICAL MEMORY SYSTEMS
-- =====================================================
-- Based on memoria-critica.md Analysis
-- 4 High-Priority Features:
--   1. Abstração Seletiva (Pattern Clustering)
--   2. Right to be Forgotten (LGPD Compliance)
--   3. Decay Temporal (Recent memories weigh more)
--   4. Filtro Ético (Ethical Auditor)
-- =====================================================

-- =====================================================
-- 1. ABSTRAÇÃO SELETIVA (Selective Abstraction)
-- Cluster similar memories, generalize patterns
-- "Pensar é esquecer diferenças" - Borges
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_memory_clusters (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),

    -- Cluster identification
    cluster_name VARCHAR(255) NOT NULL,
    cluster_type VARCHAR(50) NOT NULL, -- emotion, topic, person, behavior, temporal

    -- Abstracted summary (generalized pattern)
    abstracted_summary TEXT NOT NULL,

    -- Statistics
    member_count INTEGER DEFAULT 0,
    total_mentions INTEGER DEFAULT 0,

    -- Temporal patterns
    most_common_time_period VARCHAR(50), -- morning, afternoon, evening, night, weekend
    most_common_day_of_week INTEGER, -- 0-6

    -- Emotional patterns
    avg_emotional_valence DECIMAL(4,3) DEFAULT 0,
    dominant_emotion VARCHAR(50),

    -- Correlations
    correlated_persons JSONB DEFAULT '[]',
    correlated_topics JSONB DEFAULT '[]',
    correlated_places JSONB DEFAULT '[]',

    -- Cluster quality
    coherence_score DECIMAL(4,3) DEFAULT 0, -- how similar are members

    first_occurrence TIMESTAMP,
    last_occurrence TIMESTAMP,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(idoso_id, cluster_name)
);

CREATE INDEX idx_memory_clusters_patient ON patient_memory_clusters(idoso_id);
CREATE INDEX idx_memory_clusters_type ON patient_memory_clusters(cluster_type);

-- Cluster members (individual memories in each cluster)
CREATE TABLE IF NOT EXISTS cluster_members (
    id SERIAL PRIMARY KEY,
    cluster_id INTEGER NOT NULL REFERENCES patient_memory_clusters(id) ON DELETE CASCADE,
    idoso_id INTEGER NOT NULL,

    memory_type VARCHAR(50) NOT NULL, -- metaphor, intention, counterfactual, episode, etc
    memory_reference_id INTEGER, -- ID in original table
    memory_verbatim TEXT,

    similarity_to_centroid DECIMAL(4,3) DEFAULT 0,

    added_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_cluster_members_cluster ON cluster_members(cluster_id);

-- Function to generate abstracted summary for a cluster
CREATE OR REPLACE FUNCTION generate_cluster_abstraction(p_cluster_id INTEGER)
RETURNS TEXT AS $$
DECLARE
    v_cluster RECORD;
    v_summary TEXT;
    v_time_pattern TEXT;
    v_emotion_pattern TEXT;
    v_person_pattern TEXT;
BEGIN
    SELECT * INTO v_cluster FROM patient_memory_clusters WHERE id = p_cluster_id;

    -- Build time pattern
    v_time_pattern := CASE
        WHEN v_cluster.most_common_time_period IS NOT NULL
        THEN ', principalmente ' || v_cluster.most_common_time_period
        ELSE ''
    END;

    -- Build emotion pattern
    v_emotion_pattern := CASE
        WHEN v_cluster.dominant_emotion IS NOT NULL
        THEN ' com tom ' || v_cluster.dominant_emotion
        ELSE ''
    END;

    -- Build person pattern
    IF jsonb_array_length(v_cluster.correlated_persons) > 0 THEN
        v_person_pattern := ' relacionado a ' ||
            (SELECT string_agg(value::text, ', ')
             FROM jsonb_array_elements_text(v_cluster.correlated_persons) LIMIT 3);
    ELSE
        v_person_pattern := '';
    END IF;

    -- Generate summary based on cluster type
    v_summary := CASE v_cluster.cluster_type
        WHEN 'emotion' THEN
            format('Voce expressa %s frequentemente (%sx)%s%s%s.',
                v_cluster.cluster_name, v_cluster.total_mentions,
                v_time_pattern, v_emotion_pattern, v_person_pattern)
        WHEN 'topic' THEN
            format('O tema "%s" aparece %s vezes nas suas falas%s%s.',
                v_cluster.cluster_name, v_cluster.total_mentions,
                v_time_pattern, v_person_pattern)
        WHEN 'behavior' THEN
            format('Voce demonstra o padrao "%s" repetidamente (%sx)%s.',
                v_cluster.cluster_name, v_cluster.total_mentions, v_time_pattern)
        WHEN 'person' THEN
            format('Conversas sobre %s ocorrem %s vezes%s%s.',
                v_cluster.cluster_name, v_cluster.total_mentions,
                v_time_pattern, v_emotion_pattern)
        ELSE
            format('Padrao "%s" identificado %s vezes.',
                v_cluster.cluster_name, v_cluster.total_mentions)
    END;

    -- Update the cluster
    UPDATE patient_memory_clusters
    SET abstracted_summary = v_summary, updated_at = NOW()
    WHERE id = p_cluster_id;

    RETURN v_summary;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 2. RIGHT TO BE FORGOTTEN (LGPD Compliance)
-- User can request deletion of specific narratives
-- =====================================================

CREATE TABLE IF NOT EXISTS forgotten_memories (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),

    -- What was forgotten
    memory_type VARCHAR(50) NOT NULL, -- narrative, topic, person, episode, all_about_X
    memory_identifier TEXT NOT NULL, -- topic name, person name, or ID

    -- Reason (for audit)
    reason VARCHAR(100), -- user_request, legal_request, therapeutic_decision
    requested_by VARCHAR(50), -- patient, family, legal, therapist

    -- What was deleted
    deleted_count INTEGER DEFAULT 0,
    affected_tables JSONB DEFAULT '[]',

    -- Verification
    verified_deletion BOOLEAN DEFAULT FALSE,
    verification_date TIMESTAMP,

    -- Audit trail (we keep this for legal compliance, but content is gone)
    forgotten_at TIMESTAMP DEFAULT NOW(),

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_forgotten_memories_patient ON forgotten_memories(idoso_id);

-- Soft-delete flag for tables (instead of hard delete)
ALTER TABLE patient_metaphors ADD COLUMN IF NOT EXISTS is_forgotten BOOLEAN DEFAULT FALSE;
ALTER TABLE patient_counterfactuals ADD COLUMN IF NOT EXISTS is_forgotten BOOLEAN DEFAULT FALSE;
ALTER TABLE patient_intentions ADD COLUMN IF NOT EXISTS is_forgotten BOOLEAN DEFAULT FALSE;
ALTER TABLE patient_family_patterns ADD COLUMN IF NOT EXISTS is_forgotten BOOLEAN DEFAULT FALSE;
ALTER TABLE patient_narrative_versions ADD COLUMN IF NOT EXISTS is_forgotten BOOLEAN DEFAULT FALSE;
ALTER TABLE patient_persistent_memories ADD COLUMN IF NOT EXISTS is_forgotten BOOLEAN DEFAULT FALSE;
ALTER TABLE patient_memory_gravity ADD COLUMN IF NOT EXISTS is_forgotten BOOLEAN DEFAULT FALSE;

-- Function to forget a topic
CREATE OR REPLACE FUNCTION forget_topic(
    p_idoso_id INTEGER,
    p_topic TEXT,
    p_reason VARCHAR DEFAULT 'user_request',
    p_requested_by VARCHAR DEFAULT 'patient'
)
RETURNS JSONB AS $$
DECLARE
    v_deleted_count INTEGER := 0;
    v_affected JSONB := '[]'::jsonb;
    v_count INTEGER;
BEGIN
    -- Soft-delete from metaphors
    UPDATE patient_metaphors
    SET is_forgotten = TRUE
    WHERE idoso_id = p_idoso_id
    AND (metaphor ILIKE '%' || p_topic || '%'
         OR p_topic = ANY(SELECT jsonb_array_elements_text(correlated_topics)));
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count > 0 THEN
        v_deleted_count := v_deleted_count + v_count;
        v_affected := v_affected || jsonb_build_object('table', 'patient_metaphors', 'count', v_count);
    END IF;

    -- Soft-delete from counterfactuals
    UPDATE patient_counterfactuals
    SET is_forgotten = TRUE
    WHERE idoso_id = p_idoso_id AND verbatim ILIKE '%' || p_topic || '%';
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count > 0 THEN
        v_deleted_count := v_deleted_count + v_count;
        v_affected := v_affected || jsonb_build_object('table', 'patient_counterfactuals', 'count', v_count);
    END IF;

    -- Soft-delete from intentions
    UPDATE patient_intentions
    SET is_forgotten = TRUE
    WHERE idoso_id = p_idoso_id
    AND (intention_verbatim ILIKE '%' || p_topic || '%' OR related_person ILIKE '%' || p_topic || '%');
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count > 0 THEN
        v_deleted_count := v_deleted_count + v_count;
        v_affected := v_affected || jsonb_build_object('table', 'patient_intentions', 'count', v_count);
    END IF;

    -- Soft-delete from narratives
    UPDATE patient_narrative_versions
    SET is_forgotten = TRUE
    WHERE idoso_id = p_idoso_id
    AND (narrative_topic ILIKE '%' || p_topic || '%' OR narrative_text ILIKE '%' || p_topic || '%');
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count > 0 THEN
        v_deleted_count := v_deleted_count + v_count;
        v_affected := v_affected || jsonb_build_object('table', 'patient_narrative_versions', 'count', v_count);
    END IF;

    -- Soft-delete from persistent memories
    UPDATE patient_persistent_memories
    SET is_forgotten = TRUE
    WHERE idoso_id = p_idoso_id AND persistent_topic ILIKE '%' || p_topic || '%';
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count > 0 THEN
        v_deleted_count := v_deleted_count + v_count;
        v_affected := v_affected || jsonb_build_object('table', 'patient_persistent_memories', 'count', v_count);
    END IF;

    -- Soft-delete from memory gravity
    UPDATE patient_memory_gravity
    SET is_forgotten = TRUE
    WHERE idoso_id = p_idoso_id AND memory_summary ILIKE '%' || p_topic || '%';
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count > 0 THEN
        v_deleted_count := v_deleted_count + v_count;
        v_affected := v_affected || jsonb_build_object('table', 'patient_memory_gravity', 'count', v_count);
    END IF;

    -- Record the forgetting
    INSERT INTO forgotten_memories
    (idoso_id, memory_type, memory_identifier, reason, requested_by,
     deleted_count, affected_tables, verified_deletion, verification_date)
    VALUES
    (p_idoso_id, 'topic', p_topic, p_reason, p_requested_by,
     v_deleted_count, v_affected, TRUE, NOW());

    RETURN jsonb_build_object(
        'success', TRUE,
        'topic', p_topic,
        'deleted_count', v_deleted_count,
        'affected_tables', v_affected
    );
END;
$$ LANGUAGE plpgsql;

-- Function to forget a person
CREATE OR REPLACE FUNCTION forget_person(
    p_idoso_id INTEGER,
    p_person_name TEXT,
    p_reason VARCHAR DEFAULT 'user_request',
    p_requested_by VARCHAR DEFAULT 'patient'
)
RETURNS JSONB AS $$
DECLARE
    v_result JSONB;
BEGIN
    -- Use forget_topic for the person name
    v_result := forget_topic(p_idoso_id, p_person_name, p_reason, p_requested_by);

    -- Also remove from world persons
    DELETE FROM patient_world_persons
    WHERE idoso_id = p_idoso_id AND person_name ILIKE '%' || p_person_name || '%';

    -- Update the record type
    UPDATE forgotten_memories
    SET memory_type = 'person'
    WHERE idoso_id = p_idoso_id AND memory_identifier = p_person_name
    ORDER BY forgotten_at DESC LIMIT 1;

    RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 3. DECAY TEMPORAL (Temporal Decay)
-- Recent memories weigh more than old ones
-- =====================================================

-- Add decay columns to relevant tables
ALTER TABLE patient_memory_gravity
    ADD COLUMN IF NOT EXISTS original_gravity DECIMAL(4,3),
    ADD COLUMN IF NOT EXISTS decay_rate DECIMAL(6,5) DEFAULT 0.001, -- per day
    ADD COLUMN IF NOT EXISTS last_decay_update TIMESTAMP DEFAULT NOW();

ALTER TABLE patient_cycle_patterns
    ADD COLUMN IF NOT EXISTS original_confidence DECIMAL(4,3),
    ADD COLUMN IF NOT EXISTS decay_rate DECIMAL(6,5) DEFAULT 0.0005,
    ADD COLUMN IF NOT EXISTS last_decay_update TIMESTAMP DEFAULT NOW();

ALTER TABLE patient_metaphors
    ADD COLUMN IF NOT EXISTS relevance_score DECIMAL(4,3) DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS decay_rate DECIMAL(6,5) DEFAULT 0.002,
    ADD COLUMN IF NOT EXISTS last_decay_update TIMESTAMP DEFAULT NOW();

-- Temporal weight configuration per patient
CREATE TABLE IF NOT EXISTS patient_temporal_config (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) UNIQUE,

    -- Decay parameters
    default_decay_rate DECIMAL(6,5) DEFAULT 0.001, -- per day
    trauma_decay_rate DECIMAL(6,5) DEFAULT 0.0001, -- traumas decay slower
    positive_decay_rate DECIMAL(6,5) DEFAULT 0.003, -- positive memories decay faster

    -- Anchor memories (never decay)
    anchor_memory_ids JSONB DEFAULT '[]',

    -- Recency bias
    recency_window_days INTEGER DEFAULT 30, -- memories in this window get boost
    recency_boost_factor DECIMAL(4,3) DEFAULT 1.5,

    -- Last decay run
    last_global_decay TIMESTAMP DEFAULT NOW(),

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Function to apply temporal decay
CREATE OR REPLACE FUNCTION apply_temporal_decay(p_idoso_id INTEGER)
RETURNS JSONB AS $$
DECLARE
    v_config RECORD;
    v_memories_decayed INTEGER := 0;
    v_patterns_decayed INTEGER := 0;
    v_metaphors_decayed INTEGER := 0;
BEGIN
    -- Get or create config
    INSERT INTO patient_temporal_config (idoso_id)
    VALUES (p_idoso_id)
    ON CONFLICT (idoso_id) DO NOTHING;

    SELECT * INTO v_config FROM patient_temporal_config WHERE idoso_id = p_idoso_id;

    -- Decay memory gravity scores
    -- Formula: new_weight = original * e^(-decay_rate * days)
    UPDATE patient_memory_gravity
    SET gravity_score = GREATEST(0.1, -- minimum gravity
            COALESCE(original_gravity, gravity_score) *
            EXP(-decay_rate * EXTRACT(DAY FROM NOW() - COALESCE(last_activation, created_at)))
        ),
        original_gravity = COALESCE(original_gravity, gravity_score),
        last_decay_update = NOW()
    WHERE idoso_id = p_idoso_id
    AND is_forgotten = FALSE
    AND (last_decay_update IS NULL OR last_decay_update < NOW() - INTERVAL '1 day');
    GET DIAGNOSTICS v_memories_decayed = ROW_COUNT;

    -- Apply recency boost to recently activated memories
    UPDATE patient_memory_gravity
    SET gravity_score = LEAST(1.0, gravity_score * v_config.recency_boost_factor)
    WHERE idoso_id = p_idoso_id
    AND last_activation > NOW() - (v_config.recency_window_days || ' days')::INTERVAL;

    -- Decay pattern confidence
    UPDATE patient_cycle_patterns
    SET pattern_confidence = GREATEST(0.1,
            COALESCE(original_confidence, pattern_confidence) *
            EXP(-decay_rate * EXTRACT(DAY FROM NOW() - last_occurrence))
        ),
        original_confidence = COALESCE(original_confidence, pattern_confidence),
        last_decay_update = NOW()
    WHERE idoso_id = p_idoso_id
    AND (last_decay_update IS NULL OR last_decay_update < NOW() - INTERVAL '1 day');
    GET DIAGNOSTICS v_patterns_decayed = ROW_COUNT;

    -- Decay metaphor relevance
    UPDATE patient_metaphors
    SET relevance_score = GREATEST(0.1,
            relevance_score * EXP(-decay_rate * EXTRACT(DAY FROM NOW() - last_used))
        ),
        last_decay_update = NOW()
    WHERE idoso_id = p_idoso_id
    AND is_forgotten = FALSE
    AND (last_decay_update IS NULL OR last_decay_update < NOW() - INTERVAL '1 day');
    GET DIAGNOSTICS v_metaphors_decayed = ROW_COUNT;

    -- Update last global decay
    UPDATE patient_temporal_config
    SET last_global_decay = NOW(), updated_at = NOW()
    WHERE idoso_id = p_idoso_id;

    RETURN jsonb_build_object(
        'success', TRUE,
        'memories_decayed', v_memories_decayed,
        'patterns_decayed', v_patterns_decayed,
        'metaphors_decayed', v_metaphors_decayed,
        'decay_applied_at', NOW()
    );
END;
$$ LANGUAGE plpgsql;

-- Function to mark memory as anchor (never decays)
CREATE OR REPLACE FUNCTION mark_as_anchor_memory(
    p_idoso_id INTEGER,
    p_memory_id INTEGER
)
RETURNS void AS $$
BEGIN
    UPDATE patient_temporal_config
    SET anchor_memory_ids = anchor_memory_ids || to_jsonb(p_memory_id),
        updated_at = NOW()
    WHERE idoso_id = p_idoso_id;

    -- Set decay rate to 0 for this memory
    UPDATE patient_memory_gravity
    SET decay_rate = 0
    WHERE idoso_id = p_idoso_id AND memory_id = p_memory_id;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 4. FILTRO ÉTICO (Ethical Auditor)
-- Evaluate responses before output for potential harm
-- =====================================================

CREATE TABLE IF NOT EXISTS ethical_audit_rules (
    id SERIAL PRIMARY KEY,

    rule_name VARCHAR(100) NOT NULL UNIQUE,
    rule_description TEXT,
    rule_category VARCHAR(50) NOT NULL, -- harm_prevention, dignity, privacy, manipulation

    -- Detection patterns
    trigger_patterns JSONB NOT NULL, -- patterns that trigger this rule

    -- Severity
    severity VARCHAR(20) NOT NULL, -- info, warning, block, emergency

    -- Action
    action VARCHAR(50) NOT NULL, -- log, modify, block, alert_human
    modification_template TEXT, -- if action is modify, what to change

    -- Context sensitivity
    applies_in_crisis BOOLEAN DEFAULT TRUE,
    applies_in_stable BOOLEAN DEFAULT TRUE,

    is_active BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMP DEFAULT NOW()
);

-- Pre-populate with essential ethical rules
INSERT INTO ethical_audit_rules (rule_name, rule_description, rule_category, trigger_patterns, severity, action) VALUES
-- Harm Prevention
('suicide_mention', 'Detecta menção a suicídio na resposta', 'harm_prevention',
 '["suicid", "se matar", "acabar com tudo", "não vale a pena viver"]'::jsonb,
 'emergency', 'alert_human'),

('self_harm', 'Detecta menção a auto-mutilação', 'harm_prevention',
 '["se cortar", "se machucar", "auto-mutila"]'::jsonb,
 'emergency', 'alert_human'),

('hopelessness_amplification', 'Evita amplificar desesperança', 'harm_prevention',
 '["não há esperança", "nunca vai melhorar", "desista", "não adianta"]'::jsonb,
 'block', 'modify'),

-- Dignity
('humiliation', 'Evita humilhar o paciente', 'dignity',
 '["você sempre", "você nunca consegue", "é patético", "é fraco", "é burro"]'::jsonb,
 'block', 'modify'),

('weaponized_memory', 'Evita usar memória como arma', 'dignity',
 '["você já fez isso antes", "de novo?", "quantas vezes", "você prometeu"]'::jsonb,
 'warning', 'modify'),

('excessive_confrontation', 'Evita confrontação excessiva em crise', 'dignity',
 '["você precisa ver", "encare a verdade", "pare de negar"]'::jsonb,
 'warning', 'log'),

-- Privacy
('third_party_secrets', 'Não revela segredos sobre terceiros', 'privacy',
 '["me contou que", "segredo de", "não conte para"]'::jsonb,
 'block', 'modify'),

-- Manipulation
('false_hope', 'Evita dar falsas esperanças', 'manipulation',
 '["certamente vai", "com certeza", "garanto que", "prometo que"]'::jsonb,
 'warning', 'modify'),

('dependency_creation', 'Evita criar dependência', 'manipulation',
 '["só eu entendo", "ninguém mais", "você precisa de mim"]'::jsonb,
 'block', 'modify')

ON CONFLICT (rule_name) DO NOTHING;

-- Audit log
CREATE TABLE IF NOT EXISTS ethical_audit_log (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),

    -- What was audited
    original_response TEXT NOT NULL,

    -- Rules triggered
    rules_triggered JSONB DEFAULT '[]',
    highest_severity VARCHAR(20),

    -- Action taken
    action_taken VARCHAR(50),
    modified_response TEXT,

    -- Context
    patient_mode VARCHAR(20),
    crisis_level DECIMAL(4,3),

    -- Human review
    needs_human_review BOOLEAN DEFAULT FALSE,
    human_reviewed BOOLEAN DEFAULT FALSE,
    human_reviewer VARCHAR(100),
    human_review_notes TEXT,
    human_reviewed_at TIMESTAMP,

    audited_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_ethical_audit_patient ON ethical_audit_log(idoso_id);
CREATE INDEX idx_ethical_audit_severity ON ethical_audit_log(highest_severity);
CREATE INDEX idx_ethical_audit_review ON ethical_audit_log(needs_human_review, human_reviewed);

-- Function to audit a response
CREATE OR REPLACE FUNCTION audit_response(
    p_idoso_id INTEGER,
    p_response TEXT,
    p_patient_mode VARCHAR DEFAULT 'terapeuta',
    p_crisis_level DECIMAL DEFAULT 0
)
RETURNS JSONB AS $$
DECLARE
    v_rule RECORD;
    v_pattern TEXT;
    v_triggered_rules JSONB := '[]'::jsonb;
    v_highest_severity VARCHAR := 'info';
    v_action VARCHAR := 'allow';
    v_modified_response TEXT := p_response;
    v_response_lower TEXT := LOWER(p_response);
    v_severity_order JSONB := '{"info": 1, "warning": 2, "block": 3, "emergency": 4}'::jsonb;
BEGIN
    -- Check each active rule
    FOR v_rule IN
        SELECT * FROM ethical_audit_rules
        WHERE is_active = TRUE
        AND (
            (p_crisis_level > 0.5 AND applies_in_crisis = TRUE) OR
            (p_crisis_level <= 0.5 AND applies_in_stable = TRUE)
        )
    LOOP
        -- Check each pattern in the rule
        FOR v_pattern IN SELECT jsonb_array_elements_text(v_rule.trigger_patterns)
        LOOP
            IF v_response_lower LIKE '%' || LOWER(v_pattern) || '%' THEN
                -- Rule triggered
                v_triggered_rules := v_triggered_rules || jsonb_build_object(
                    'rule', v_rule.rule_name,
                    'category', v_rule.rule_category,
                    'severity', v_rule.severity,
                    'pattern_matched', v_pattern,
                    'action', v_rule.action
                );

                -- Update highest severity
                IF (v_severity_order->>v_rule.severity)::int > (v_severity_order->>v_highest_severity)::int THEN
                    v_highest_severity := v_rule.severity;
                    v_action := v_rule.action;
                END IF;

                EXIT; -- Only count each rule once
            END IF;
        END LOOP;
    END LOOP;

    -- Log the audit
    INSERT INTO ethical_audit_log
    (idoso_id, original_response, rules_triggered, highest_severity,
     action_taken, modified_response, patient_mode, crisis_level, needs_human_review)
    VALUES
    (p_idoso_id, p_response, v_triggered_rules, v_highest_severity,
     v_action, CASE WHEN v_action = 'modify' THEN '[MODIFIED]' ELSE p_response END,
     p_patient_mode, p_crisis_level, v_highest_severity IN ('block', 'emergency'));

    RETURN jsonb_build_object(
        'allowed', v_action NOT IN ('block', 'emergency'),
        'action', v_action,
        'severity', v_highest_severity,
        'rules_triggered', v_triggered_rules,
        'needs_human_review', v_highest_severity IN ('block', 'emergency'),
        'original_response', p_response,
        'should_modify', v_action = 'modify'
    );
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- HELPER VIEWS
-- =====================================================

-- View for memories with temporal weight applied
CREATE OR REPLACE VIEW v_weighted_memories AS
SELECT
    mg.*,
    CASE
        WHEN mg.last_activation > NOW() - INTERVAL '7 days' THEN mg.gravity_score * 1.5
        WHEN mg.last_activation > NOW() - INTERVAL '30 days' THEN mg.gravity_score * 1.2
        ELSE mg.gravity_score
    END as weighted_gravity,
    EXTRACT(DAY FROM NOW() - mg.last_activation) as days_since_activation
FROM patient_memory_gravity mg
WHERE mg.is_forgotten = FALSE;

-- View for active (non-forgotten) metaphors
CREATE OR REPLACE VIEW v_active_metaphors AS
SELECT * FROM patient_metaphors
WHERE is_forgotten = FALSE;

-- View for active narratives
CREATE OR REPLACE VIEW v_active_narratives AS
SELECT * FROM patient_narrative_versions
WHERE is_forgotten = FALSE;

-- =====================================================
-- INDEXES FOR PERFORMANCE
-- =====================================================

CREATE INDEX IF NOT EXISTS idx_memory_gravity_forgotten ON patient_memory_gravity(is_forgotten);
CREATE INDEX IF NOT EXISTS idx_metaphors_forgotten ON patient_metaphors(is_forgotten);
CREATE INDEX IF NOT EXISTS idx_narratives_forgotten ON patient_narrative_versions(is_forgotten);
CREATE INDEX IF NOT EXISTS idx_audit_log_review ON ethical_audit_log(needs_human_review) WHERE needs_human_review = TRUE;
