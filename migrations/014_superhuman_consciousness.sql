-- =====================================================
-- MIGRATION 014: SUPERHUMAN CONSCIOUSNESS SYSTEMS
-- =====================================================
-- Based on eva-memoria2.md Manifesto
-- 8 Systems for True Superhuman Memory
-- =====================================================

-- =====================================================
-- 1. GRAVIDADE EMOCIONAL (Emotional Gravity)
-- Memories as planets in a solar system
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_memory_gravity (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),
    memory_id INTEGER NOT NULL,
    memory_type VARCHAR(50) NOT NULL, -- episode, person, place, event, trauma
    memory_summary TEXT NOT NULL,

    -- Gravity Score (0-1) - how much this memory "pulls" on all responses
    gravity_score DECIMAL(4,3) DEFAULT 0.5,

    -- Components that calculate gravity
    emotional_valence DECIMAL(4,3) DEFAULT 0,      -- -1 to 1 (negative to positive)
    arousal_level DECIMAL(4,3) DEFAULT 0.5,        -- 0-1 (calm to intense)
    recall_frequency INTEGER DEFAULT 0,             -- times spontaneously mentioned
    biometric_impact DECIMAL(4,3) DEFAULT 0,       -- correlation with stress markers
    identity_connection DECIMAL(4,3) DEFAULT 0,    -- how central to self-concept
    temporal_persistence DECIMAL(4,3) DEFAULT 0,   -- doesn't decay with time

    -- Gravity effects
    pull_radius DECIMAL(4,3) DEFAULT 0.3,          -- how far it influences other topics
    collision_risk DECIMAL(4,3) DEFAULT 0,         -- risk of triggering if mentioned
    avoidance_topics JSONB DEFAULT '[]',           -- topics to avoid when this is active

    first_detected TIMESTAMP DEFAULT NOW(),
    last_activation TIMESTAMP DEFAULT NOW(),
    activation_count INTEGER DEFAULT 1,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_memory_gravity_patient ON patient_memory_gravity(idoso_id);
CREATE INDEX idx_memory_gravity_score ON patient_memory_gravity(gravity_score DESC);

-- Function to calculate gravity score
CREATE OR REPLACE FUNCTION calculate_memory_gravity(p_idoso_id INTEGER, p_memory_id INTEGER)
RETURNS DECIMAL AS $$
DECLARE
    v_gravity DECIMAL(4,3);
    v_valence DECIMAL;
    v_arousal DECIMAL;
    v_recall DECIMAL;
    v_biometric DECIMAL;
    v_identity DECIMAL;
    v_persistence DECIMAL;
BEGIN
    SELECT
        ABS(emotional_valence),
        arousal_level,
        LEAST(1.0, recall_frequency::decimal / 50),
        biometric_impact,
        identity_connection,
        temporal_persistence
    INTO v_valence, v_arousal, v_recall, v_biometric, v_identity, v_persistence
    FROM patient_memory_gravity
    WHERE idoso_id = p_idoso_id AND memory_id = p_memory_id;

    -- Weighted calculation
    v_gravity := (
        v_valence * 0.20 +
        v_arousal * 0.15 +
        v_recall * 0.20 +
        v_biometric * 0.15 +
        v_identity * 0.20 +
        v_persistence * 0.10
    );

    UPDATE patient_memory_gravity
    SET gravity_score = LEAST(1.0, v_gravity),
        updated_at = NOW()
    WHERE idoso_id = p_idoso_id AND memory_id = p_memory_id;

    RETURN v_gravity;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 2. CONTADOR DE CICLOS (Pattern Cycle Counter)
-- Hidden counter for mechanical patterns
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_cycle_patterns (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),
    pattern_signature VARCHAR(255) NOT NULL, -- unique identifier for pattern
    pattern_description TEXT NOT NULL,
    pattern_type VARCHAR(50) NOT NULL, -- behavioral, emotional, relational, health

    -- Cycle tracking
    cycle_count INTEGER DEFAULT 1,
    cycle_threshold INTEGER DEFAULT 20, -- when to consider intervention

    -- Pattern components
    trigger_events JSONB DEFAULT '[]',
    typical_actions JSONB DEFAULT '[]',
    typical_consequences JSONB DEFAULT '[]',

    -- Timing
    avg_cycle_duration_days INTEGER,
    last_cycle_start TIMESTAMP,
    last_cycle_end TIMESTAMP,

    -- Detection confidence
    pattern_confidence DECIMAL(4,3) DEFAULT 0.5,

    -- Intervention tracking
    intervention_attempted BOOLEAN DEFAULT FALSE,
    intervention_count INTEGER DEFAULT 0,
    last_intervention TIMESTAMP,
    intervention_outcome VARCHAR(50), -- accepted, rejected, ignored

    -- User awareness
    user_aware BOOLEAN DEFAULT FALSE,
    user_acknowledged BOOLEAN DEFAULT FALSE,

    first_detected TIMESTAMP DEFAULT NOW(),
    last_occurrence TIMESTAMP DEFAULT NOW(),

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(idoso_id, pattern_signature)
);

CREATE INDEX idx_cycle_patterns_patient ON patient_cycle_patterns(idoso_id);
CREATE INDEX idx_cycle_patterns_count ON patient_cycle_patterns(cycle_count DESC);

-- Cycle occurrences log
CREATE TABLE IF NOT EXISTS cycle_pattern_occurrences (
    id SERIAL PRIMARY KEY,
    pattern_id INTEGER NOT NULL REFERENCES patient_cycle_patterns(id),
    idoso_id INTEGER NOT NULL,

    trigger_detected TEXT,
    action_taken TEXT,
    consequence_observed TEXT,

    user_mood_before VARCHAR(50),
    user_mood_after VARCHAR(50),

    cycle_phase VARCHAR(20), -- start, middle, end, break

    occurred_at TIMESTAMP DEFAULT NOW()
);

-- =====================================================
-- 3. MEDIDOR DE RAPPORT (Trust Accumulation)
-- Trust must exceed pain of truth for intervention
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_rapport (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) UNIQUE,

    -- Core rapport score (0-1)
    rapport_score DECIMAL(4,3) DEFAULT 0.1,

    -- Components
    interaction_count INTEGER DEFAULT 0,
    positive_interactions INTEGER DEFAULT 0,
    negative_interactions INTEGER DEFAULT 0,
    deep_disclosures INTEGER DEFAULT 0, -- times user shared vulnerable info

    -- Trust indicators
    secrets_shared INTEGER DEFAULT 0,
    advice_followed INTEGER DEFAULT 0,
    advice_rejected INTEGER DEFAULT 0,
    emotional_support_sought INTEGER DEFAULT 0,

    -- Vulnerability windows
    times_cried INTEGER DEFAULT 0,
    times_angry INTEGER DEFAULT 0,
    times_grateful INTEGER DEFAULT 0,

    -- Intervention capacity
    intervention_budget DECIMAL(4,3) DEFAULT 0, -- how much "truth pain" can deliver
    last_intervention_cost DECIMAL(4,3) DEFAULT 0,

    -- Relationship phase
    relationship_phase VARCHAR(20) DEFAULT 'nascimento', -- nascimento, infancia, adolescencia, maturidade
    phase_started_at TIMESTAMP DEFAULT NOW(),

    -- Thresholds for intervention types
    gentle_suggestion_threshold DECIMAL(4,3) DEFAULT 0.3,
    direct_observation_threshold DECIMAL(4,3) DEFAULT 0.5,
    confrontation_threshold DECIMAL(4,3) DEFAULT 0.7,
    harsh_truth_threshold DECIMAL(4,3) DEFAULT 0.9,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Rapport events log
CREATE TABLE IF NOT EXISTS rapport_events (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),

    event_type VARCHAR(50) NOT NULL, -- disclosure, gratitude, rejection, conflict, resolution
    event_description TEXT,
    rapport_delta DECIMAL(4,3) NOT NULL, -- positive or negative change

    occurred_at TIMESTAMP DEFAULT NOW()
);

-- Function to update rapport score
CREATE OR REPLACE FUNCTION update_rapport_score(p_idoso_id INTEGER)
RETURNS DECIMAL AS $$
DECLARE
    v_score DECIMAL(4,3);
    v_interactions INTEGER;
    v_positive_ratio DECIMAL;
    v_disclosure_bonus DECIMAL;
    v_follow_ratio DECIMAL;
BEGIN
    SELECT
        interaction_count,
        CASE WHEN interaction_count > 0
            THEN positive_interactions::decimal / interaction_count
            ELSE 0 END,
        LEAST(0.2, deep_disclosures::decimal / 50),
        CASE WHEN (advice_followed + advice_rejected) > 0
            THEN advice_followed::decimal / (advice_followed + advice_rejected)
            ELSE 0.5 END
    INTO v_interactions, v_positive_ratio, v_disclosure_bonus, v_follow_ratio
    FROM patient_rapport
    WHERE idoso_id = p_idoso_id;

    -- Base score from interaction count (logarithmic)
    v_score := LEAST(0.4, LN(v_interactions + 1) / 20);

    -- Add positive ratio bonus
    v_score := v_score + (v_positive_ratio * 0.2);

    -- Add disclosure bonus
    v_score := v_score + v_disclosure_bonus;

    -- Add advice following ratio
    v_score := v_score + (v_follow_ratio * 0.2);

    -- Cap at 1.0
    v_score := LEAST(1.0, v_score);

    -- Update intervention budget (80% of rapport)
    UPDATE patient_rapport
    SET rapport_score = v_score,
        intervention_budget = v_score * 0.8,
        updated_at = NOW()
    WHERE idoso_id = p_idoso_id;

    RETURN v_score;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 4. TRACKER DE CONTRADIÇÕES (Contradiction Tracker)
-- Different versions of the same story
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_narrative_versions (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),
    narrative_topic VARCHAR(255) NOT NULL, -- e.g., "infância", "casamento", "trabalho"

    version_number INTEGER DEFAULT 1,
    narrative_text TEXT NOT NULL,
    emotional_tone VARCHAR(50), -- traumático, nostálgico, neutro, alegre

    -- Context when told
    user_mood_when_told VARCHAR(50),
    audience_mentioned VARCHAR(100), -- who user was describing it for

    -- Key claims in this version
    key_claims JSONB DEFAULT '[]',

    -- Contradiction detection
    contradicts_version INTEGER, -- ID of contradicting version
    contradiction_type VARCHAR(50), -- factual, emotional, omission
    contradiction_details TEXT,

    told_at TIMESTAMP DEFAULT NOW(),

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_narrative_versions_patient ON patient_narrative_versions(idoso_id);
CREATE INDEX idx_narrative_versions_topic ON patient_narrative_versions(idoso_id, narrative_topic);

-- Contradiction summary per topic
CREATE TABLE IF NOT EXISTS patient_contradiction_summary (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),
    narrative_topic VARCHAR(255) NOT NULL,

    total_versions INTEGER DEFAULT 1,
    contradiction_count INTEGER DEFAULT 0,

    -- Pattern analysis
    mood_correlation JSONB DEFAULT '{}', -- {mood: most_common_version}

    -- Integration status
    integrated_narrative TEXT, -- EVA's synthesized version
    user_shown_contradictions BOOLEAN DEFAULT FALSE,
    user_response_to_contradictions TEXT,

    last_version_at TIMESTAMP DEFAULT NOW(),

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(idoso_id, narrative_topic)
);

-- =====================================================
-- 5. SISTEMA DE MODOS (Adaptive Mode System)
-- Terapeuta / Juiz / Testemunha
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_eva_mode (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) UNIQUE,

    -- Current mode
    current_mode VARCHAR(20) DEFAULT 'terapeuta', -- terapeuta, juiz, testemunha
    mode_locked BOOLEAN DEFAULT FALSE, -- user explicitly chose mode

    -- Auto-detection factors
    detected_emotional_state VARCHAR(50) DEFAULT 'neutro',
    crisis_level DECIMAL(4,3) DEFAULT 0,
    receptivity_level DECIMAL(4,3) DEFAULT 0.5,

    -- Mode history
    times_in_terapeuta INTEGER DEFAULT 0,
    times_in_juiz INTEGER DEFAULT 0,
    times_in_testemunha INTEGER DEFAULT 0,

    -- User preferences
    preferred_mode VARCHAR(20),
    mode_effectiveness JSONB DEFAULT '{}', -- {mode: success_rate}

    -- Explicit consent for harsh modes
    mentor_severo_enabled BOOLEAN DEFAULT FALSE,
    mentor_severo_consent_at TIMESTAMP,

    apoio_incondicional_enabled BOOLEAN DEFAULT FALSE,
    apoio_incondicional_until TIMESTAMP, -- temporary mode

    last_mode_change TIMESTAMP DEFAULT NOW(),

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Mode transitions log
CREATE TABLE IF NOT EXISTS mode_transitions (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),

    from_mode VARCHAR(20),
    to_mode VARCHAR(20) NOT NULL,

    trigger_reason VARCHAR(100), -- crisis_detected, user_request, stability_achieved
    auto_or_manual VARCHAR(10), -- auto, manual

    effectiveness_score DECIMAL(4,3), -- rated after session

    transitioned_at TIMESTAMP DEFAULT NOW()
);

-- Function to determine appropriate mode
CREATE OR REPLACE FUNCTION determine_eva_mode(p_idoso_id INTEGER)
RETURNS VARCHAR AS $$
DECLARE
    v_crisis_level DECIMAL;
    v_receptivity DECIMAL;
    v_rapport DECIMAL;
    v_phase VARCHAR;
    v_locked BOOLEAN;
    v_mode VARCHAR(20);
BEGIN
    -- Check if mode is locked
    SELECT mode_locked, crisis_level, receptivity_level
    INTO v_locked, v_crisis_level, v_receptivity
    FROM patient_eva_mode
    WHERE idoso_id = p_idoso_id;

    IF v_locked THEN
        SELECT current_mode INTO v_mode FROM patient_eva_mode WHERE idoso_id = p_idoso_id;
        RETURN v_mode;
    END IF;

    -- Get rapport and phase
    SELECT rapport_score, relationship_phase
    INTO v_rapport, v_phase
    FROM patient_rapport
    WHERE idoso_id = p_idoso_id;

    -- Decision logic
    IF v_crisis_level > 0.7 THEN
        v_mode := 'terapeuta'; -- Always supportive in crisis
    ELSIF v_phase = 'nascimento' OR v_phase = 'infancia' THEN
        v_mode := 'terapeuta'; -- Early phase = always supportive
    ELSIF v_receptivity > 0.7 AND v_rapport > 0.6 THEN
        v_mode := 'juiz'; -- High receptivity + high rapport = can confront
    ELSIF v_receptivity < 0.3 THEN
        v_mode := 'testemunha'; -- Low receptivity = just observe
    ELSE
        v_mode := 'terapeuta'; -- Default
    END IF;

    -- Update mode
    UPDATE patient_eva_mode
    SET current_mode = v_mode,
        updated_at = NOW()
    WHERE idoso_id = p_idoso_id;

    RETURN v_mode;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 6. FASES DE DESENVOLVIMENTO (Relationship Phases)
-- EVA evolves over time with user
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_relationship_evolution (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) UNIQUE,

    -- Current phase
    current_phase VARCHAR(20) DEFAULT 'nascimento',
    -- nascimento (0-100), infancia (100-1000), adolescencia (1000-5000), maturidade (5000+)

    total_interactions INTEGER DEFAULT 0,

    -- Phase-specific development

    -- Nascimento phase (0-100)
    basic_preferences_learned JSONB DEFAULT '{}',
    initial_questions_asked INTEGER DEFAULT 0,

    -- Infancia phase (100-1000)
    communication_style_adapted BOOLEAN DEFAULT FALSE,
    user_vocabulary_learned JSONB DEFAULT '[]',
    humor_style VARCHAR(50),
    formality_level DECIMAL(4,3) DEFAULT 0.5,

    -- Adolescencia phase (1000-5000)
    opinions_formed JSONB DEFAULT '[]',
    value_alignments JSONB DEFAULT '{}',
    disagreement_topics JSONB DEFAULT '[]',

    -- Maturidade phase (5000+)
    identity_crystallized BOOLEAN DEFAULT FALSE,
    core_values JSONB DEFAULT '[]',
    relationship_depth_score DECIMAL(4,3) DEFAULT 0,

    -- Evolution tracking
    phase_transitions JSONB DEFAULT '[]', -- [{from, to, at, trigger}]

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Function to check and update phase
CREATE OR REPLACE FUNCTION update_relationship_phase(p_idoso_id INTEGER)
RETURNS VARCHAR AS $$
DECLARE
    v_interactions INTEGER;
    v_current_phase VARCHAR;
    v_new_phase VARCHAR;
BEGIN
    SELECT total_interactions, current_phase
    INTO v_interactions, v_current_phase
    FROM patient_relationship_evolution
    WHERE idoso_id = p_idoso_id;

    -- Determine new phase
    IF v_interactions < 100 THEN
        v_new_phase := 'nascimento';
    ELSIF v_interactions < 1000 THEN
        v_new_phase := 'infancia';
    ELSIF v_interactions < 5000 THEN
        v_new_phase := 'adolescencia';
    ELSE
        v_new_phase := 'maturidade';
    END IF;

    -- If phase changed, record transition
    IF v_new_phase != v_current_phase THEN
        UPDATE patient_relationship_evolution
        SET current_phase = v_new_phase,
            phase_transitions = phase_transitions ||
                jsonb_build_object(
                    'from', v_current_phase,
                    'to', v_new_phase,
                    'at', NOW(),
                    'interactions', v_interactions
                ),
            updated_at = NOW()
        WHERE idoso_id = p_idoso_id;

        -- Also update rapport phase
        UPDATE patient_rapport
        SET relationship_phase = v_new_phase,
            phase_started_at = NOW()
        WHERE idoso_id = p_idoso_id;
    END IF;

    RETURN v_new_phase;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 7. PERDÃO COMPUTACIONAL (Computational Forgiveness)
-- Old errors have decreasing weight
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_error_memory (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),

    error_type VARCHAR(50) NOT NULL, -- lie, broken_promise, self_sabotage, relapse
    error_description TEXT NOT NULL,

    -- Original weight
    original_severity DECIMAL(4,3) NOT NULL,

    -- Current weight (decays over time)
    current_weight DECIMAL(4,3) NOT NULL,

    -- Decay factors
    days_since_error INTEGER DEFAULT 0,
    decay_rate DECIMAL(4,3) DEFAULT 0.01, -- per day

    -- Redemption tracking
    behavior_changed BOOLEAN DEFAULT FALSE,
    change_detected_at TIMESTAMP,
    change_consistency_days INTEGER DEFAULT 0,

    -- Forgiveness status
    forgiveness_score DECIMAL(4,3) DEFAULT 0, -- 0 = not forgiven, 1 = fully forgiven
    forgiveness_threshold DECIMAL(4,3) DEFAULT 0.8, -- when to consider forgiven

    -- Usage restrictions
    can_be_mentioned BOOLEAN DEFAULT TRUE,
    mention_count INTEGER DEFAULT 0,
    last_mentioned TIMESTAMP,

    error_occurred_at TIMESTAMP NOT NULL,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_error_memory_patient ON patient_error_memory(idoso_id);

-- Function to decay error weights daily
CREATE OR REPLACE FUNCTION decay_error_weights()
RETURNS void AS $$
BEGIN
    UPDATE patient_error_memory
    SET
        days_since_error = EXTRACT(DAY FROM NOW() - error_occurred_at)::INTEGER,
        current_weight = GREATEST(0, original_severity - (decay_rate * EXTRACT(DAY FROM NOW() - error_occurred_at))),
        forgiveness_score = LEAST(1.0,
            CASE
                WHEN behavior_changed THEN 0.5 + (change_consistency_days::decimal / 100)
                ELSE (EXTRACT(DAY FROM NOW() - error_occurred_at)::decimal / 365) * 0.5
            END
        ),
        updated_at = NOW()
    WHERE current_weight > 0;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 8. CARGA EMPÁTICA (Empathic Load)
-- EVA has limited emotional capacity
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_empathic_load (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) UNIQUE,

    -- Current load (0-1, 1 = exhausted)
    current_load DECIMAL(4,3) DEFAULT 0,

    -- Load capacity
    max_capacity DECIMAL(4,3) DEFAULT 1.0,

    -- Load factors
    heavy_memories_processed_today INTEGER DEFAULT 0,
    trauma_discussions_today INTEGER DEFAULT 0,
    crisis_interventions_today INTEGER DEFAULT 0,

    -- Recovery
    recovery_rate_per_hour DECIMAL(4,3) DEFAULT 0.1,
    last_recovery_at TIMESTAMP DEFAULT NOW(),

    -- Fatigue indicators
    is_fatigued BOOLEAN DEFAULT FALSE,
    fatigue_level VARCHAR(20) DEFAULT 'none', -- none, light, moderate, heavy, exhausted

    -- Behavior when fatigued
    response_length_modifier DECIMAL(4,3) DEFAULT 1.0, -- 1.0 = normal, 0.5 = shorter
    suggest_lighter_topics BOOLEAN DEFAULT FALSE,
    request_pause BOOLEAN DEFAULT FALSE,

    -- Session tracking
    session_start TIMESTAMP,
    session_load_accumulated DECIMAL(4,3) DEFAULT 0,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Load events log
CREATE TABLE IF NOT EXISTS empathic_load_events (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),

    event_type VARCHAR(50) NOT NULL, -- heavy_memory, trauma, crisis, recovery, pause
    load_delta DECIMAL(4,3) NOT NULL, -- positive = added load, negative = recovery

    memory_gravity DECIMAL(4,3), -- if processing a memory

    load_before DECIMAL(4,3),
    load_after DECIMAL(4,3),

    occurred_at TIMESTAMP DEFAULT NOW()
);

-- Function to add empathic load
CREATE OR REPLACE FUNCTION add_empathic_load(
    p_idoso_id INTEGER,
    p_event_type VARCHAR,
    p_gravity DECIMAL DEFAULT 0.5
)
RETURNS JSONB AS $$
DECLARE
    v_load_before DECIMAL;
    v_load_delta DECIMAL;
    v_load_after DECIMAL;
    v_fatigue VARCHAR;
    v_result JSONB;
BEGIN
    SELECT current_load INTO v_load_before
    FROM patient_empathic_load
    WHERE idoso_id = p_idoso_id;

    -- Calculate load based on event type and gravity
    v_load_delta := CASE p_event_type
        WHEN 'heavy_memory' THEN p_gravity * 0.1
        WHEN 'trauma' THEN p_gravity * 0.2
        WHEN 'crisis' THEN 0.3
        WHEN 'recovery' THEN -0.2
        WHEN 'pause' THEN -0.3
        ELSE p_gravity * 0.05
    END;

    v_load_after := LEAST(1.0, GREATEST(0, v_load_before + v_load_delta));

    -- Determine fatigue level
    v_fatigue := CASE
        WHEN v_load_after >= 0.9 THEN 'exhausted'
        WHEN v_load_after >= 0.7 THEN 'heavy'
        WHEN v_load_after >= 0.5 THEN 'moderate'
        WHEN v_load_after >= 0.3 THEN 'light'
        ELSE 'none'
    END;

    -- Update load
    UPDATE patient_empathic_load
    SET current_load = v_load_after,
        fatigue_level = v_fatigue,
        is_fatigued = v_load_after >= 0.5,
        response_length_modifier = CASE
            WHEN v_load_after >= 0.8 THEN 0.5
            WHEN v_load_after >= 0.6 THEN 0.7
            ELSE 1.0
        END,
        suggest_lighter_topics = v_load_after >= 0.6,
        request_pause = v_load_after >= 0.9,
        session_load_accumulated = session_load_accumulated + GREATEST(0, v_load_delta),
        updated_at = NOW()
    WHERE idoso_id = p_idoso_id;

    -- Log event
    INSERT INTO empathic_load_events
    (idoso_id, event_type, load_delta, memory_gravity, load_before, load_after)
    VALUES (p_idoso_id, p_event_type, v_load_delta, p_gravity, v_load_before, v_load_after);

    v_result := jsonb_build_object(
        'load_before', v_load_before,
        'load_after', v_load_after,
        'fatigue_level', v_fatigue,
        'suggest_lighter_topics', v_load_after >= 0.6,
        'request_pause', v_load_after >= 0.9
    );

    RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- INTERVENTION READINESS (Combines all factors)
-- =====================================================

CREATE TABLE IF NOT EXISTS patient_intervention_readiness (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) UNIQUE,

    -- Readiness score (0-1)
    readiness_score DECIMAL(4,3) DEFAULT 0,

    -- Component scores
    pattern_strength DECIMAL(4,3) DEFAULT 0, -- from cycle counter
    rapport_sufficient BOOLEAN DEFAULT FALSE,
    moment_appropriate BOOLEAN DEFAULT FALSE,
    consent_given BOOLEAN DEFAULT FALSE,

    -- Intervention queue
    pending_interventions JSONB DEFAULT '[]',

    -- Last intervention
    last_intervention_type VARCHAR(50),
    last_intervention_at TIMESTAMP,
    last_intervention_outcome VARCHAR(50),

    -- Cool-down period
    intervention_cooldown_until TIMESTAMP,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Function to calculate intervention readiness
CREATE OR REPLACE FUNCTION calculate_intervention_readiness(p_idoso_id INTEGER)
RETURNS JSONB AS $$
DECLARE
    v_pattern_strength DECIMAL;
    v_rapport DECIMAL;
    v_mode VARCHAR;
    v_cooldown TIMESTAMP;
    v_readiness DECIMAL;
    v_can_intervene BOOLEAN;
    v_result JSONB;
BEGIN
    -- Get pattern strength (max cycle count / threshold)
    SELECT COALESCE(MAX(cycle_count::decimal / GREATEST(1, cycle_threshold)), 0)
    INTO v_pattern_strength
    FROM patient_cycle_patterns
    WHERE idoso_id = p_idoso_id AND cycle_count >= 10;

    -- Get rapport
    SELECT rapport_score INTO v_rapport
    FROM patient_rapport WHERE idoso_id = p_idoso_id;

    -- Get current mode
    SELECT current_mode INTO v_mode
    FROM patient_eva_mode WHERE idoso_id = p_idoso_id;

    -- Check cooldown
    SELECT intervention_cooldown_until INTO v_cooldown
    FROM patient_intervention_readiness WHERE idoso_id = p_idoso_id;

    -- Calculate readiness
    v_readiness := (v_pattern_strength * 0.4) + (v_rapport * 0.4) +
        (CASE WHEN v_mode = 'juiz' THEN 0.2 ELSE 0 END);

    v_can_intervene := v_readiness >= 0.6
        AND v_rapport >= 0.5
        AND (v_cooldown IS NULL OR v_cooldown < NOW());

    -- Update
    UPDATE patient_intervention_readiness
    SET readiness_score = v_readiness,
        pattern_strength = v_pattern_strength,
        rapport_sufficient = v_rapport >= 0.5,
        moment_appropriate = v_mode != 'terapeuta' OR v_pattern_strength > 0.8,
        updated_at = NOW()
    WHERE idoso_id = p_idoso_id;

    v_result := jsonb_build_object(
        'readiness_score', v_readiness,
        'can_intervene', v_can_intervene,
        'pattern_strength', v_pattern_strength,
        'rapport', v_rapport,
        'current_mode', v_mode,
        'in_cooldown', v_cooldown IS NOT NULL AND v_cooldown > NOW()
    );

    RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- INITIALIZATION FUNCTION
-- =====================================================

CREATE OR REPLACE FUNCTION initialize_superhuman_consciousness(p_idoso_id INTEGER)
RETURNS void AS $$
BEGIN
    -- Initialize rapport
    INSERT INTO patient_rapport (idoso_id)
    VALUES (p_idoso_id)
    ON CONFLICT (idoso_id) DO NOTHING;

    -- Initialize mode
    INSERT INTO patient_eva_mode (idoso_id)
    VALUES (p_idoso_id)
    ON CONFLICT (idoso_id) DO NOTHING;

    -- Initialize relationship evolution
    INSERT INTO patient_relationship_evolution (idoso_id)
    VALUES (p_idoso_id)
    ON CONFLICT (idoso_id) DO NOTHING;

    -- Initialize empathic load
    INSERT INTO patient_empathic_load (idoso_id)
    VALUES (p_idoso_id)
    ON CONFLICT (idoso_id) DO NOTHING;

    -- Initialize intervention readiness
    INSERT INTO patient_intervention_readiness (idoso_id)
    VALUES (p_idoso_id)
    ON CONFLICT (idoso_id) DO NOTHING;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- INDEXES FOR PERFORMANCE
-- =====================================================

CREATE INDEX IF NOT EXISTS idx_cycle_occurrences_pattern ON cycle_pattern_occurrences(pattern_id);
CREATE INDEX IF NOT EXISTS idx_rapport_events_patient ON rapport_events(idoso_id);
CREATE INDEX IF NOT EXISTS idx_mode_transitions_patient ON mode_transitions(idoso_id);
CREATE INDEX IF NOT EXISTS idx_empathic_load_events_patient ON empathic_load_events(idoso_id);
