-- ============================================================================
-- SPRINT 6: EXIT PROTOCOL & QUALITY OF LIFE MONITORING
-- ============================================================================
-- Descrição: Sistema de despedida digna, qualidade de vida e cuidados paliativos
-- Autor: EVA-Mind Development Team
-- Data: 2026-01-24
-- ============================================================================

-- ============================================================================
-- 1. LAST WISHES (TESTAMENTO VITAL DIGITAL)
-- ============================================================================
-- Armazena desejos do paciente para fim de vida
CREATE TABLE IF NOT EXISTS last_wishes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Decisões médicas
    resuscitation_preference VARCHAR(50) CHECK (resuscitation_preference IN (
        'full_code',           -- Ressuscitação completa
        'dnr',                 -- Do Not Resuscitate
        'dni',                 -- Do Not Intubate
        'comfort_care_only'    -- Apenas cuidados de conforto
    )),
    mechanical_ventilation BOOLEAN,
    artificial_nutrition BOOLEAN,
    artificial_hydration BOOLEAN,
    dialysis BOOLEAN,
    hospitalization_preference VARCHAR(50) CHECK (hospitalization_preference IN (
        'hospital_if_needed',
        'home_care_preferred',
        'hospice_preferred',
        'no_hospitalization'
    )),

    -- Preferências de local
    preferred_death_location VARCHAR(100), -- 'home', 'hospital', 'hospice', 'family_home'

    -- Conforto e dor
    pain_management_preference VARCHAR(50) CHECK (pain_management_preference IN (
        'aggressive_pain_control',
        'balanced_approach',
        'minimal_medication',
        'natural_only'
    )),
    sedation_acceptable BOOLEAN,

    -- Espiritual e emocional
    religious_preferences TEXT,
    spiritual_practices TEXT[],
    want_spiritual_support BOOLEAN DEFAULT FALSE,
    preferred_clergy VARCHAR(200),

    -- Presença e despedida
    who_wants_present TEXT[], -- Lista de pessoas
    farewell_ceremony_preferences TEXT,
    music_preferences TEXT,
    readings_preferences TEXT,

    -- Órgãos e corpo
    organ_donation_preference VARCHAR(50) CHECK (organ_donation_preference IN (
        'donate_all',
        'donate_specific',
        'no_donation',
        'undecided'
    )),
    specific_organs_donate TEXT[],
    body_donation_science BOOLEAN,
    autopsy_preference VARCHAR(50) CHECK (autopsy_preference IN (
        'yes_if_helpful',
        'only_if_required',
        'prefer_not',
        'absolutely_not'
    )),

    -- Funeral e memorial
    funeral_preferences TEXT,
    burial_cremation VARCHAR(50) CHECK (burial_cremation IN ('burial', 'cremation', 'natural_burial', 'undecided')),
    memorial_service_preferences TEXT,

    -- Declarações gerais
    personal_statement TEXT, -- "Como quero ser lembrado", "O que é importante para mim"
    specific_fears TEXT,
    specific_hopes TEXT,

    -- Metadados
    completed BOOLEAN DEFAULT FALSE,
    completion_percentage INTEGER DEFAULT 0,
    last_reviewed_at TIMESTAMP,
    witnessed_by VARCHAR(200), -- Profissional que testemunhou
    legally_binding BOOLEAN DEFAULT FALSE,
    legal_document_path VARCHAR(500),

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_last_wishes_patient ON last_wishes(patient_id);
CREATE INDEX IF NOT EXISTS idx_last_wishes_completed ON last_wishes(completed);

COMMENT ON TABLE last_wishes IS 'Testamento vital digital - desejos do paciente para fim de vida';


-- ============================================================================
-- 2. QUALITY OF LIFE ASSESSMENTS (WHOQOL-BREF)
-- ============================================================================
-- Rastreamento longitudinal de qualidade de vida
CREATE TABLE IF NOT EXISTS quality_of_life_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    assessment_date TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Domínio Físico (WHOQOL-BREF)
    physical_pain INTEGER CHECK (physical_pain BETWEEN 1 AND 5), -- 1=nada, 5=extremamente
    energy_fatigue INTEGER CHECK (energy_fatigue BETWEEN 1 AND 5),
    sleep_quality INTEGER CHECK (sleep_quality BETWEEN 1 AND 5),
    mobility INTEGER CHECK (mobility BETWEEN 1 AND 5),
    daily_activities INTEGER CHECK (daily_activities BETWEEN 1 AND 5),
    medication_dependence INTEGER CHECK (medication_dependence BETWEEN 1 AND 5),
    work_capacity INTEGER CHECK (work_capacity BETWEEN 1 AND 5),

    -- Domínio Psicológico
    positive_feelings INTEGER CHECK (positive_feelings BETWEEN 1 AND 5),
    thinking_concentration INTEGER CHECK (thinking_concentration BETWEEN 1 AND 5),
    self_esteem INTEGER CHECK (self_esteem BETWEEN 1 AND 5),
    body_image INTEGER CHECK (body_image BETWEEN 1 AND 5),
    negative_feelings INTEGER CHECK (negative_feelings BETWEEN 1 AND 5),
    meaning_in_life INTEGER CHECK (meaning_in_life BETWEEN 1 AND 5),

    -- Domínio Social
    personal_relationships INTEGER CHECK (personal_relationships BETWEEN 1 AND 5),
    social_support INTEGER CHECK (social_support BETWEEN 1 AND 5),
    sexual_activity INTEGER CHECK (sexual_activity BETWEEN 1 AND 5),

    -- Domínio Ambiental
    physical_safety INTEGER CHECK (physical_safety BETWEEN 1 AND 5),
    home_environment INTEGER CHECK (home_environment BETWEEN 1 AND 5),
    financial_resources INTEGER CHECK (financial_resources BETWEEN 1 AND 5),
    healthcare_access INTEGER CHECK (healthcare_access BETWEEN 1 AND 5),
    information_access INTEGER CHECK (information_access BETWEEN 1 AND 5),
    leisure_opportunities INTEGER CHECK (leisure_opportunities BETWEEN 1 AND 5),
    environment_quality INTEGER CHECK (environment_quality BETWEEN 1 AND 5),
    transportation INTEGER CHECK (transportation BETWEEN 1 AND 5),

    -- Scores calculados
    physical_domain_score DECIMAL(5,2),
    psychological_domain_score DECIMAL(5,2),
    social_domain_score DECIMAL(5,2),
    environmental_domain_score DECIMAL(5,2),
    overall_qol_score DECIMAL(5,2),

    -- Questões gerais
    overall_quality_of_life INTEGER CHECK (overall_quality_of_life BETWEEN 1 AND 5),
    overall_health_satisfaction INTEGER CHECK (overall_health_satisfaction BETWEEN 1 AND 5),

    -- Contexto
    administered_by VARCHAR(100),
    assessment_method VARCHAR(50) CHECK (assessment_method IN ('self_report', 'interview', 'proxy', 'eva_assisted')),
    notes TEXT,

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_qol_patient_date ON quality_of_life_assessments(patient_id, assessment_date DESC);

COMMENT ON TABLE quality_of_life_assessments IS 'Avaliações de qualidade de vida (WHOQOL-BREF)';


-- ============================================================================
-- 3. PAIN & SYMPTOM MONITORING
-- ============================================================================
-- Monitoramento diário de dor e sintomas
CREATE TABLE IF NOT EXISTS pain_symptom_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    log_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Dor
    pain_present BOOLEAN NOT NULL,
    pain_intensity INTEGER CHECK (pain_intensity BETWEEN 0 AND 10), -- 0=sem dor, 10=pior dor imaginável
    pain_location TEXT[],
    pain_quality TEXT[], -- 'burning', 'stabbing', 'aching', 'throbbing', 'shooting'
    pain_interference INTEGER CHECK (pain_interference BETWEEN 0 AND 10), -- Quanto interfere nas atividades

    -- Sintomas físicos
    nausea_vomiting INTEGER CHECK (nausea_vomiting BETWEEN 0 AND 10),
    shortness_of_breath INTEGER CHECK (shortness_of_breath BETWEEN 0 AND 10),
    constipation INTEGER CHECK (constipation BETWEEN 0 AND 10),
    fatigue INTEGER CHECK (fatigue BETWEEN 0 AND 10),
    drowsiness INTEGER CHECK (drowsiness BETWEEN 0 AND 10),
    lack_of_appetite INTEGER CHECK (lack_of_appetite BETWEEN 0 AND 10),

    -- Sintomas psicológicos
    anxiety_level INTEGER CHECK (anxiety_level BETWEEN 0 AND 10),
    depression_level INTEGER CHECK (depression_level BETWEEN 0 AND 10),
    confusion INTEGER CHECK (confusion BETWEEN 0 AND 10),

    -- Bem-estar geral
    overall_wellbeing INTEGER CHECK (overall_wellbeing BETWEEN 0 AND 10),

    -- Intervenções
    medications_taken TEXT[],
    non_pharmacological_interventions TEXT[], -- 'massage', 'music', 'breathing', 'meditation'
    intervention_effectiveness INTEGER CHECK (intervention_effectiveness BETWEEN 0 AND 10),

    -- Contexto
    reported_by VARCHAR(50) CHECK (reported_by IN ('patient', 'family', 'caregiver', 'eva', 'nurse')),
    notes TEXT,

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pain_logs_patient_time ON pain_symptom_logs(patient_id, log_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_pain_logs_high_pain ON pain_symptom_logs(patient_id) WHERE pain_intensity >= 7;

COMMENT ON TABLE pain_symptom_logs IS 'Registro diário de dor e sintomas para cuidados paliativos';


-- ============================================================================
-- 4. LEGACY MESSAGES (MENSAGENS DE LEGADO)
-- ============================================================================
-- Mensagens gravadas pelo paciente para deixar para entes queridos
CREATE TABLE IF NOT EXISTS legacy_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Destinatário
    recipient_name VARCHAR(200) NOT NULL,
    recipient_relationship VARCHAR(100), -- 'daughter', 'son', 'spouse', 'grandchild', 'friend'

    -- Conteúdo
    message_type VARCHAR(50) CHECK (message_type IN (
        'text',
        'audio',
        'video',
        'letter',
        'combined'
    )),
    text_content TEXT,
    audio_file_path VARCHAR(500),
    video_file_path VARCHAR(500),

    -- Trigger de entrega
    delivery_trigger VARCHAR(50) CHECK (delivery_trigger IN (
        'after_death',
        'specific_date',
        'milestone',  -- e.g., wedding, graduation
        'when_ready', -- Quando destinatário estiver pronto
        'immediately'
    )),
    delivery_date TIMESTAMP,
    milestone_description TEXT,

    -- Status
    is_complete BOOLEAN DEFAULT FALSE,
    has_been_delivered BOOLEAN DEFAULT FALSE,
    delivered_at TIMESTAMP,

    -- Emoção e contexto
    emotional_tone VARCHAR(100), -- 'loving', 'grateful', 'apologetic', 'hopeful', 'instructional'
    topics TEXT[], -- 'advice', 'memories', 'gratitude', 'apology', 'wishes', 'instructions'

    -- Privacidade
    encryption_required BOOLEAN DEFAULT FALSE,
    requires_passcode BOOLEAN DEFAULT FALSE,
    authorized_viewers TEXT[], -- Quem pode ver além do destinatário principal

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_legacy_messages_patient ON legacy_messages(patient_id);
CREATE INDEX IF NOT EXISTS idx_legacy_messages_pending_delivery ON legacy_messages(has_been_delivered) WHERE has_been_delivered = FALSE;

COMMENT ON TABLE legacy_messages IS 'Mensagens de legado deixadas pelo paciente para entes queridos';


-- ============================================================================
-- 5. FAREWELL PREPARATION (PREPARAÇÃO PARA DESPEDIDA)
-- ============================================================================
-- Rastreia o progresso da preparação emocional e prática para o fim de vida
CREATE TABLE IF NOT EXISTS farewell_preparation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Preparação prática
    legal_affairs_complete BOOLEAN DEFAULT FALSE, -- Testamento, procurações
    financial_affairs_complete BOOLEAN DEFAULT FALSE,
    funeral_arrangements_complete BOOLEAN DEFAULT FALSE,
    digital_legacy_complete BOOLEAN DEFAULT FALSE, -- Senhas, redes sociais

    -- Preparação relacional
    reconciliations_needed TEXT[], -- Lista de pessoas com quem quer se reconciliar
    reconciliations_completed TEXT[],
    goodbyes_needed TEXT[], -- Lista de pessoas com quem quer se despedir
    goodbyes_completed TEXT[],

    -- Preparação emocional
    five_stages_grief_position VARCHAR(50) CHECK (five_stages_grief_position IN (
        'denial',
        'anger',
        'bargaining',
        'depression',
        'acceptance',
        'fluctuating'
    )),
    emotional_readiness INTEGER CHECK (emotional_readiness BETWEEN 0 AND 10),
    fears_addressed TEXT[],
    unresolved_fears TEXT[],

    -- Preparação espiritual
    spiritual_readiness INTEGER CHECK (spiritual_readiness BETWEEN 0 AND 10),
    existential_questions_addressed TEXT[],
    meaning_found BOOLEAN,
    peace_with_life BOOLEAN,
    peace_with_death BOOLEAN,

    -- Bucket list / últimas experiências
    bucket_list_items TEXT[],
    bucket_list_completed TEXT[],
    last_wishes_list TEXT[],
    last_wishes_fulfilled TEXT[],

    -- Suporte
    professional_support_received TEXT[], -- 'chaplain', 'therapist', 'social_worker'
    family_support_level INTEGER CHECK (family_support_level BETWEEN 0 AND 10),

    -- Progresso geral
    overall_preparation_score INTEGER CHECK (overall_preparation_score BETWEEN 0 AND 100),

    -- Conversas importantes
    had_conversation_with_doctor BOOLEAN DEFAULT FALSE,
    had_conversation_with_family BOOLEAN DEFAULT FALSE,
    had_conversation_with_spiritual_advisor BOOLEAN DEFAULT FALSE,

    last_updated TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_farewell_prep_patient ON farewell_preparation(patient_id);

COMMENT ON TABLE farewell_preparation IS 'Rastreamento da preparação emocional, prática e espiritual para despedida';


-- ============================================================================
-- 6. COMFORT CARE PLANS (PLANOS DE CUIDADO CONFORTÁVEL)
-- ============================================================================
-- Planos de ação específicos para diferentes sintomas/situações
CREATE TABLE IF NOT EXISTS comfort_care_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Para que situação este plano se aplica
    trigger_symptom VARCHAR(100) NOT NULL, -- 'severe_pain', 'dyspnea', 'anxiety', 'agitation', 'nausea'
    trigger_threshold INTEGER, -- e.g., pain >= 7/10

    -- Intervenções ordenadas por prioridade
    interventions JSONB NOT NULL,
    -- Exemplo: [
    --   {"order": 1, "type": "pharmacological", "action": "Morphine 5mg sublingual", "repeat_after_minutes": 30},
    --   {"order": 2, "type": "positioning", "action": "Elevate head of bed 45 degrees"},
    --   {"order": 3, "type": "comfort", "action": "Cool compress, dim lights, soft music"}
    -- ]

    -- Contatos de emergência para escalação
    escalation_contacts JSONB,
    -- Exemplo: [
    --   {"role": "primary_nurse", "name": "Maria", "phone": "555-1234"},
    --   {"role": "physician", "name": "Dr. Santos", "phone": "555-5678"}
    -- ]

    -- Quando ativar
    auto_activate BOOLEAN DEFAULT FALSE,
    requires_caregiver_action BOOLEAN DEFAULT TRUE,
    eva_can_suggest BOOLEAN DEFAULT TRUE,

    -- Eficácia
    times_used INTEGER DEFAULT 0,
    average_effectiveness DECIMAL(3,2), -- 0-10
    last_used TIMESTAMP,

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    reviewed_by VARCHAR(200), -- Médico/enfermeiro que revisou
    last_reviewed TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_comfort_plans_patient ON comfort_care_plans(patient_id);
CREATE INDEX IF NOT EXISTS idx_comfort_plans_active ON comfort_care_plans(patient_id, is_active) WHERE is_active = TRUE;

COMMENT ON TABLE comfort_care_plans IS 'Planos de ação para manejo de sintomas em cuidados paliativos';


-- ============================================================================
-- 7. SPIRITUAL CARE SESSIONS (SESSÕES DE CUIDADO ESPIRITUAL)
-- ============================================================================
-- Rastreia conversas espirituais e existenciais
CREATE TABLE IF NOT EXISTS spiritual_care_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    session_date TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Quem conduziu
    conducted_by VARCHAR(50) CHECK (conducted_by IN ('eva', 'chaplain', 'clergy', 'spiritual_advisor', 'family', 'therapist')),
    conductor_name VARCHAR(200),

    -- Temas abordados
    topics_discussed TEXT[], -- 'meaning_of_life', 'afterlife', 'forgiveness', 'regrets', 'gratitude', 'fear_of_death'

    -- Questões existenciais
    existential_questions TEXT[],
    insights_gained TEXT,

    -- Práticas espirituais
    practices_performed TEXT[], -- 'prayer', 'meditation', 'scripture_reading', 'ritual', 'confession'

    -- Estado emocional
    pre_session_peace_level INTEGER CHECK (pre_session_peace_level BETWEEN 0 AND 10),
    post_session_peace_level INTEGER CHECK (post_session_peace_level BETWEEN 0 AND 10),

    -- Próximos passos
    spiritual_needs_identified TEXT[],
    follow_up_needed BOOLEAN DEFAULT FALSE,
    follow_up_plans TEXT,

    session_notes TEXT,
    duration_minutes INTEGER,

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_spiritual_sessions_patient ON spiritual_care_sessions(patient_id, session_date DESC);

COMMENT ON TABLE spiritual_care_sessions IS 'Sessões de cuidado espiritual e conversas existenciais';


-- ============================================================================
-- TRIGGERS E FUNÇÕES AUXILIARES
-- ============================================================================

-- Função para calcular scores do WHOQOL-BREF
CREATE OR REPLACE FUNCTION calculate_whoqol_scores()
RETURNS TRIGGER AS $$
BEGIN
    -- Domínio Físico (7 questões)
    NEW.physical_domain_score := (
        NEW.physical_pain + NEW.energy_fatigue + NEW.sleep_quality +
        NEW.mobility + NEW.daily_activities + NEW.medication_dependence + NEW.work_capacity
    ) / 7.0 * 20; -- Normalizar para 0-100

    -- Domínio Psicológico (6 questões)
    NEW.psychological_domain_score := (
        NEW.positive_feelings + NEW.thinking_concentration + NEW.self_esteem +
        NEW.body_image + (6 - NEW.negative_feelings) + NEW.meaning_in_life
    ) / 6.0 * 20;

    -- Domínio Social (3 questões)
    NEW.social_domain_score := (
        NEW.personal_relationships + NEW.social_support + NEW.sexual_activity
    ) / 3.0 * 20;

    -- Domínio Ambiental (8 questões)
    NEW.environmental_domain_score := (
        NEW.physical_safety + NEW.home_environment + NEW.financial_resources +
        NEW.healthcare_access + NEW.information_access + NEW.leisure_opportunities +
        NEW.environment_quality + NEW.transportation
    ) / 8.0 * 20;

    -- Score geral (média dos 4 domínios)
    NEW.overall_qol_score := (
        NEW.physical_domain_score + NEW.psychological_domain_score +
        NEW.social_domain_score + NEW.environmental_domain_score
    ) / 4.0;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_calculate_whoqol_scores
    BEFORE INSERT OR UPDATE ON quality_of_life_assessments
    FOR EACH ROW
    EXECUTE FUNCTION calculate_whoqol_scores();


-- Função para atualizar completion percentage de Last Wishes
CREATE OR REPLACE FUNCTION update_last_wishes_completion()
RETURNS TRIGGER AS $$
DECLARE
    total_fields INTEGER := 20; -- Número de campos importantes
    completed_fields INTEGER := 0;
BEGIN
    -- Contar campos preenchidos
    IF NEW.resuscitation_preference IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF NEW.mechanical_ventilation IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF NEW.hospitalization_preference IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF NEW.preferred_death_location IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF NEW.pain_management_preference IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF NEW.religious_preferences IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF NEW.organ_donation_preference IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF NEW.burial_cremation IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF NEW.personal_statement IS NOT NULL THEN completed_fields := completed_fields + 1; END IF;
    IF array_length(NEW.who_wants_present, 1) > 0 THEN completed_fields := completed_fields + 1; END IF;

    -- Adicionar mais verificações conforme necessário

    NEW.completion_percentage := (completed_fields * 100) / total_fields;
    NEW.completed := (NEW.completion_percentage >= 80);
    NEW.updated_at := NOW();

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_last_wishes_completion
    BEFORE INSERT OR UPDATE ON last_wishes
    FOR EACH ROW
    EXECUTE FUNCTION update_last_wishes_completion();


-- ============================================================================
-- VIEWS
-- ============================================================================

-- View: Pacientes em cuidados paliativos com resumo de qualidade de vida
CREATE OR REPLACE VIEW v_palliative_care_summary AS
SELECT
    i.id AS patient_id,
    i.nome,
    i.data_nascimento,
    EXTRACT(YEAR FROM AGE(i.data_nascimento)) AS age,

    -- Last Wishes
    lw.completion_percentage AS last_wishes_completion,
    lw.resuscitation_preference,
    lw.preferred_death_location,

    -- Latest QoL
    latest_qol.overall_qol_score,
    latest_qol.physical_domain_score,
    latest_qol.psychological_domain_score,
    latest_qol.assessment_date AS last_qol_assessment,

    -- Recent pain levels
    recent_pain.avg_pain_intensity AS avg_pain_7days,
    recent_pain.max_pain_intensity AS max_pain_7days,
    recent_pain.pain_logs_count,

    -- Farewell preparation
    fp.overall_preparation_score,
    fp.emotional_readiness,
    fp.spiritual_readiness,

    -- Legacy messages
    (SELECT COUNT(*) FROM legacy_messages WHERE patient_id = i.id AND is_complete = TRUE) AS legacy_messages_completed,
    (SELECT COUNT(*) FROM legacy_messages WHERE patient_id = i.id AND has_been_delivered = FALSE) AS legacy_messages_pending

FROM idosos i
LEFT JOIN last_wishes lw ON lw.patient_id = i.id
LEFT JOIN LATERAL (
    SELECT * FROM quality_of_life_assessments
    WHERE patient_id = i.id
    ORDER BY assessment_date DESC
    LIMIT 1
) latest_qol ON TRUE
LEFT JOIN LATERAL (
    SELECT
        AVG(pain_intensity) AS avg_pain_intensity,
        MAX(pain_intensity) AS max_pain_intensity,
        COUNT(*) AS pain_logs_count
    FROM pain_symptom_logs
    WHERE patient_id = i.id
      AND log_timestamp > NOW() - INTERVAL '7 days'
      AND pain_present = TRUE
) recent_pain ON TRUE
LEFT JOIN farewell_preparation fp ON fp.patient_id = i.id
WHERE EXISTS (
    SELECT 1 FROM last_wishes WHERE patient_id = i.id
) OR EXISTS (
    SELECT 1 FROM farewell_preparation WHERE patient_id = i.id
);

COMMENT ON VIEW v_palliative_care_summary IS 'Resumo de pacientes em cuidados paliativos com métricas de qualidade de vida';


-- View: Alertas de dor não controlada
CREATE OR REPLACE VIEW v_uncontrolled_pain_alerts AS
SELECT
    i.id AS patient_id,
    i.nome AS patient_name,
    psl.pain_intensity,
    psl.pain_location,
    psl.pain_quality,
    psl.log_timestamp,
    psl.intervention_effectiveness,
    EXTRACT(EPOCH FROM (NOW() - psl.log_timestamp)) / 3600 AS hours_since_report,
    lw.pain_management_preference
FROM pain_symptom_logs psl
JOIN idosos i ON i.id = psl.patient_id
LEFT JOIN last_wishes lw ON lw.patient_id = psl.patient_id
WHERE psl.pain_intensity >= 7
  AND psl.log_timestamp > NOW() - INTERVAL '24 hours'
  AND (psl.intervention_effectiveness IS NULL OR psl.intervention_effectiveness < 5)
ORDER BY psl.pain_intensity DESC, psl.log_timestamp DESC;

COMMENT ON VIEW v_uncontrolled_pain_alerts IS 'Alertas de dor severa não controlada nas últimas 24h';


-- View: Progresso de preparação para despedida
CREATE OR REPLACE VIEW v_farewell_readiness AS
SELECT
    i.id AS patient_id,
    i.nome,
    fp.overall_preparation_score,
    fp.emotional_readiness,
    fp.spiritual_readiness,
    fp.five_stages_grief_position,

    -- Contagens
    array_length(fp.goodbyes_needed, 1) AS goodbyes_needed_count,
    array_length(fp.goodbyes_completed, 1) AS goodbyes_completed_count,
    array_length(fp.bucket_list_items, 1) AS bucket_list_total,
    array_length(fp.bucket_list_completed, 1) AS bucket_list_completed,

    -- Progresso percentual
    CASE
        WHEN array_length(fp.goodbyes_needed, 1) > 0 THEN
            (array_length(fp.goodbyes_completed, 1)::DECIMAL / array_length(fp.goodbyes_needed, 1) * 100)
        ELSE 100
    END AS goodbyes_progress_pct,

    CASE
        WHEN array_length(fp.bucket_list_items, 1) > 0 THEN
            (array_length(fp.bucket_list_completed, 1)::DECIMAL / array_length(fp.bucket_list_items, 1) * 100)
        ELSE NULL
    END AS bucket_list_progress_pct,

    -- Flags
    fp.legal_affairs_complete,
    fp.financial_affairs_complete,
    fp.funeral_arrangements_complete,
    fp.peace_with_life,
    fp.peace_with_death,

    fp.last_updated
FROM farewell_preparation fp
JOIN idosos i ON i.id = fp.patient_id;

COMMENT ON VIEW v_farewell_readiness IS 'Progresso de preparação emocional e prática para despedida';


-- ============================================================================
-- ÍNDICES ADICIONAIS PARA PERFORMANCE
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_qol_overall_score ON quality_of_life_assessments(patient_id, overall_qol_score);
CREATE INDEX IF NOT EXISTS idx_pain_logs_severe ON pain_symptom_logs(patient_id, pain_intensity) WHERE pain_intensity >= 7;
CREATE INDEX IF NOT EXISTS idx_legacy_delivery_date ON legacy_messages(delivery_date) WHERE has_been_delivered = FALSE;

-- ============================================================================
-- ✅ SPRINT 6: EXIT PROTOCOL - SCHEMA COMPLETO
-- ============================================================================

DO $$
BEGIN
    RAISE NOTICE '✅ Sprint 6 (Exit Protocol) - Schema criado com sucesso';
    RAISE NOTICE '   Tabelas:';
    RAISE NOTICE '   - last_wishes (testamento vital)';
    RAISE NOTICE '   - quality_of_life_assessments (WHOQOL-BREF)';
    RAISE NOTICE '   - pain_symptom_logs (monitoramento de dor)';
    RAISE NOTICE '   - legacy_messages (mensagens de legado)';
    RAISE NOTICE '   - farewell_preparation (preparação para despedida)';
    RAISE NOTICE '   - comfort_care_plans (planos de cuidado)';
    RAISE NOTICE '   - spiritual_care_sessions (cuidado espiritual)';
    RAISE NOTICE '   ';
    RAISE NOTICE '   Views:';
    RAISE NOTICE '   - v_palliative_care_summary';
    RAISE NOTICE '   - v_uncontrolled_pain_alerts';
    RAISE NOTICE '   - v_farewell_readiness';
END $$;
