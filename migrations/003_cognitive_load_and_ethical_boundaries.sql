-- Migration: Cognitive Load Orchestrator & Ethical Boundary Engine
-- Description: Tabelas para Meta-Controller Cognitivo e Governança Ética
-- Created: 2026-01-24
-- Sprint 1: Governança Cognitiva

-- ========================================
-- 1. META-CONTROLLER COGNITIVO
-- ========================================

-- Histórico de carga cognitiva por interação
CREATE TABLE IF NOT EXISTS interaction_cognitive_load (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Tipo e características da interação
    interaction_type VARCHAR(50) NOT NULL CHECK (interaction_type IN ('therapeutic', 'entertainment', 'clinical', 'educational', 'emergency')),
    emotional_intensity DECIMAL(3,2) CHECK (emotional_intensity BETWEEN 0 AND 1), -- 0-1 (detectado por Affective Router)
    cognitive_complexity DECIMAL(3,2) CHECK (cognitive_complexity BETWEEN 0 AND 1), -- 0-1 (memória, raciocínio exigido)
    duration_seconds INTEGER NOT NULL,

    -- Indicadores de fadiga do paciente
    patient_fatigue_indicators JSONB, -- {voice_energy_drop: 0.3, response_latency: 2.5s, irritability: true}

    -- Contexto da conversa
    topics_discussed TEXT[],
    lacanian_signifiers TEXT[], -- Significantes dominantes (integração com TransNAR)
    session_id VARCHAR(100),

    -- Métricas de voz (se disponível)
    voice_energy_score DECIMAL(3,2),
    speech_rate_wpm INTEGER, -- Palavras por minuto
    pause_frequency DECIMAL(5,2), -- Pausas por minuto

    -- Carga acumulada
    cumulative_load_24h DECIMAL(3,2), -- Score agregado últimas 24h

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Estado atual de carga cognitiva (cache/state)
CREATE TABLE IF NOT EXISTS cognitive_load_state (
    patient_id INTEGER PRIMARY KEY REFERENCES idosos(id) ON DELETE CASCADE,

    -- Scores de carga
    current_load_score DECIMAL(3,2) NOT NULL DEFAULT 0 CHECK (current_load_score BETWEEN 0 AND 1), -- 0-1
    load_24h DECIMAL(3,2) NOT NULL DEFAULT 0, -- Carga últimas 24h
    load_7d DECIMAL(3,2) NOT NULL DEFAULT 0, -- Carga últimos 7 dias

    -- Contadores
    interactions_count_24h INTEGER NOT NULL DEFAULT 0,
    therapeutic_count_24h INTEGER NOT NULL DEFAULT 0,
    high_intensity_count_24h INTEGER NOT NULL DEFAULT 0,

    -- Timings
    last_interaction_at TIMESTAMP,
    last_high_intensity_at TIMESTAMP,
    last_rest_period_start TIMESTAMP,

    -- Detecção de padrões problemáticos
    rumination_detected BOOLEAN DEFAULT FALSE, -- Mesmo tópico repetido
    rumination_topic TEXT,
    rumination_count_24h INTEGER DEFAULT 0,

    emotional_saturation BOOLEAN DEFAULT FALSE, -- Saturação emocional detectada
    fatigue_level VARCHAR(20) DEFAULT 'none' CHECK (fatigue_level IN ('none', 'mild', 'moderate', 'severe')),

    -- Restrições ativas
    active_restrictions JSONB, -- {block_therapy: true, allow_entertainment: true, ...}
    restriction_reason TEXT,
    restriction_until TIMESTAMP,

    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Decisões do Cognitive Load Orchestrator
CREATE TABLE IF NOT EXISTS cognitive_load_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Contexto da decisão
    current_load DECIMAL(3,2) NOT NULL,
    trigger_event VARCHAR(100), -- 'high_load_detected', 'rumination_detected', 'fatigue_detected'

    -- Decisão tomada
    decision_type VARCHAR(50) NOT NULL CHECK (decision_type IN ('block', 'allow', 'redirect', 'reduce_frequency', 'suggest_rest')),
    blocked_actions TEXT[], -- ['apply_phq9', 'deep_therapy']
    allowed_actions TEXT[], -- ['light_entertainment', 'music']
    redirect_suggestion TEXT,

    -- Instruções para Gemini
    system_instruction_override TEXT, -- System instruction adaptativa injetada
    tone_adjustment VARCHAR(50), -- 'lighter', 'more_casual', 'less_intimate'

    -- Resultado
    was_applied BOOLEAN DEFAULT TRUE,
    patient_compliance BOOLEAN, -- Paciente aceitou redirecionamento?

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Índices para performance
CREATE INDEX IF NOT EXISTS idx_cognitive_load_patient ON interaction_cognitive_load(patient_id);
CREATE INDEX IF NOT EXISTS idx_cognitive_load_timestamp ON interaction_cognitive_load(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_cognitive_load_type ON interaction_cognitive_load(interaction_type);
CREATE INDEX IF NOT EXISTS idx_cognitive_load_intensity ON interaction_cognitive_load(emotional_intensity DESC);
CREATE INDEX IF NOT EXISTS idx_cognitive_load_patient_timestamp ON interaction_cognitive_load(patient_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_cognitive_state_load ON cognitive_load_state(current_load_score DESC);
CREATE INDEX IF NOT EXISTS idx_cognitive_state_rumination ON cognitive_load_state(rumination_detected) WHERE rumination_detected = TRUE;
CREATE INDEX IF NOT EXISTS idx_cognitive_state_saturation ON cognitive_load_state(emotional_saturation) WHERE emotional_saturation = TRUE;

CREATE INDEX IF NOT EXISTS idx_cognitive_decisions_patient ON cognitive_load_decisions(patient_id);
CREATE INDEX IF NOT EXISTS idx_cognitive_decisions_timestamp ON cognitive_load_decisions(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_cognitive_decisions_type ON cognitive_load_decisions(decision_type);

-- Comentários
COMMENT ON TABLE interaction_cognitive_load IS 'Histórico detalhado de carga cognitiva por interação com paciente';
COMMENT ON TABLE cognitive_load_state IS 'Estado atual de carga cognitiva do paciente (cache para decisões rápidas)';
COMMENT ON TABLE cognitive_load_decisions IS 'Registro de decisões tomadas pelo Cognitive Load Orchestrator';
COMMENT ON COLUMN interaction_cognitive_load.emotional_intensity IS 'Intensidade emocional 0-1 (detectada pelo Affective Router)';
COMMENT ON COLUMN interaction_cognitive_load.cognitive_complexity IS 'Complexidade cognitiva 0-1 (raciocínio, memória exigida)';
COMMENT ON COLUMN cognitive_load_state.active_restrictions IS 'Restrições ativas em formato JSON: {block_therapy: true, max_duration: 15}';

-- ========================================
-- 2. ETHICAL BOUNDARY ENGINE
-- ========================================

-- Eventos de fronteira ética detectados
CREATE TABLE IF NOT EXISTS ethical_boundary_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Tipo de evento
    event_type VARCHAR(50) NOT NULL CHECK (event_type IN (
        'attachment_phrase',
        'isolation_detected',
        'dependency_warning',
        'eva_preference_over_human',
        'signifier_shift',
        'excessive_intimacy',
        'boundary_violation'
    )),

    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),

    -- Evidências
    evidence JSONB NOT NULL, -- {phrase_text: "...", frequency: 3, context: "..."}
    trigger_phrase TEXT, -- Frase exata que disparou (se aplicável)
    trigger_conversation_id VARCHAR(100),

    -- Métricas de apego
    attachment_indicators_count INTEGER DEFAULT 0,
    eva_vs_human_ratio DECIMAL(5,2), -- Ratio EVA:Humanos (ex: 15.5 = 15.5:1)
    signifier_eva_dominance DECIMAL(3,2), -- 0-1: quanto "EVA" domina significantes

    -- Ação tomada
    action_taken VARCHAR(100) NOT NULL,
    redirection_attempted BOOLEAN DEFAULT FALSE,
    redirection_message TEXT,

    -- Notificações
    family_notified BOOLEAN DEFAULT FALSE,
    family_notification_sent_at TIMESTAMP,
    doctor_notified BOOLEAN DEFAULT FALSE,

    -- Follow-up
    patient_response TEXT, -- Como paciente reagiu
    was_effective BOOLEAN, -- Redirecionamento foi efetivo?

    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMP
);

-- Estado de fronteiras éticas por paciente
CREATE TABLE IF NOT EXISTS ethical_boundary_state (
    patient_id INTEGER PRIMARY KEY REFERENCES idosos(id) ON DELETE CASCADE,

    -- Scores de risco ético
    attachment_risk_score DECIMAL(3,2) NOT NULL DEFAULT 0 CHECK (attachment_risk_score BETWEEN 0 AND 1),
    isolation_risk_score DECIMAL(3,2) NOT NULL DEFAULT 0 CHECK (isolation_risk_score BETWEEN 0 AND 1),
    dependency_risk_score DECIMAL(3,2) NOT NULL DEFAULT 0 CHECK (dependency_risk_score BETWEEN 0 AND 1),
    overall_ethical_risk VARCHAR(20) DEFAULT 'low' CHECK (overall_ethical_risk IN ('low', 'medium', 'high', 'critical')),

    -- Contadores (últimos 7 dias)
    attachment_phrases_7d INTEGER DEFAULT 0,
    eva_interactions_7d INTEGER DEFAULT 0,
    human_interactions_7d INTEGER DEFAULT 0,
    eva_vs_human_ratio DECIMAL(5,2), -- Calculado: eva / human

    -- Análise de significantes (integração com TransNAR)
    dominant_signifiers JSONB, -- {eva: 45%, filha: 20%, neto: 15%, ...}
    signifier_eva_percentage DECIMAL(3,2), -- % de vezes que "EVA" aparece
    human_signifiers_declining BOOLEAN DEFAULT FALSE,

    -- Duração de interações
    avg_interaction_duration_minutes DECIMAL(5,2),
    max_interaction_duration_minutes INTEGER,
    excessive_duration_count_7d INTEGER DEFAULT 0, -- Quantas sessões >45min

    -- Limites éticos ativos
    active_ethical_limits JSONB, -- {reduce_frequency: true, redirect_to_family: true, ...}
    limit_enforcement_level VARCHAR(20) DEFAULT 'monitoring' CHECK (limit_enforcement_level IN ('monitoring', 'soft_redirect', 'hard_limit', 'temporary_block')),

    -- Timeline de intervenções
    last_redirect_at TIMESTAMP,
    redirect_count_30d INTEGER DEFAULT 0,
    last_family_alert_at TIMESTAMP,

    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Protocolos de redirecionamento aplicados
CREATE TABLE IF NOT EXISTS ethical_redirections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    event_id UUID REFERENCES ethical_boundary_events(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Contexto
    trigger_reason TEXT NOT NULL,
    severity_level VARCHAR(20) NOT NULL,

    -- Estratégia de redirecionamento
    redirection_level INTEGER NOT NULL CHECK (redirection_level IN (1, 2, 3)), -- 1=suave, 2=explícito, 3=bloqueio
    strategy_used VARCHAR(100), -- 'validation_redirect', 'explicit_limit', 'temporary_block'

    -- Mensagem enviada ao paciente
    eva_message TEXT NOT NULL,
    tone VARCHAR(50), -- 'gentle', 'firm', 'professional'

    -- Resultado
    patient_response TEXT,
    compliance_achieved BOOLEAN,

    -- Follow-up
    follow_up_needed BOOLEAN DEFAULT FALSE,
    follow_up_action TEXT,
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMP
);

-- Índices para performance
CREATE INDEX IF NOT EXISTS idx_ethical_events_patient ON ethical_boundary_events(patient_id);
CREATE INDEX IF NOT EXISTS idx_ethical_events_type ON ethical_boundary_events(event_type);
CREATE INDEX IF NOT EXISTS idx_ethical_events_severity ON ethical_boundary_events(severity);
CREATE INDEX IF NOT EXISTS idx_ethical_events_timestamp ON ethical_boundary_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_ethical_events_unresolved ON ethical_boundary_events(resolved_at) WHERE resolved_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ethical_state_risk ON ethical_boundary_state(overall_ethical_risk);
CREATE INDEX IF NOT EXISTS idx_ethical_state_attachment ON ethical_boundary_state(attachment_risk_score DESC);
CREATE INDEX IF NOT EXISTS idx_ethical_state_ratio ON ethical_boundary_state(eva_vs_human_ratio DESC);

CREATE INDEX IF NOT EXISTS idx_ethical_redirections_patient ON ethical_redirections(patient_id);
CREATE INDEX IF NOT EXISTS idx_ethical_redirections_level ON ethical_redirections(redirection_level);
CREATE INDEX IF NOT EXISTS idx_ethical_redirections_unresolved ON ethical_redirections(resolved) WHERE resolved = FALSE;

-- Comentários
COMMENT ON TABLE ethical_boundary_events IS 'Eventos de violação ou risco de violação de limites éticos';
COMMENT ON TABLE ethical_boundary_state IS 'Estado atual das fronteiras éticas por paciente (cache para decisões)';
COMMENT ON TABLE ethical_redirections IS 'Histórico de redirecionamentos aplicados para manter limites éticos';
COMMENT ON COLUMN ethical_boundary_state.eva_vs_human_ratio IS 'Ratio de interações EVA:Humanos (alerta se >10:1)';
COMMENT ON COLUMN ethical_redirections.redirection_level IS '1=Suave, 2=Explícito, 3=Bloqueio temporário';

-- ========================================
-- 3. TRIGGERS PARA ATUALIZAÇÃO AUTOMÁTICA
-- ========================================

-- Função para atualizar updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger para cognitive_load_state
DROP TRIGGER IF EXISTS update_cognitive_load_state_updated_at ON cognitive_load_state;
CREATE TRIGGER update_cognitive_load_state_updated_at
    BEFORE UPDATE ON cognitive_load_state
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Trigger para ethical_boundary_state
DROP TRIGGER IF EXISTS update_ethical_boundary_state_updated_at ON ethical_boundary_state;
CREATE TRIGGER update_ethical_boundary_state_updated_at
    BEFORE UPDATE ON ethical_boundary_state
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ========================================
-- 4. VIEWS ÚTEIS PARA MONITORAMENTO
-- ========================================

-- Pacientes com alta carga cognitiva (últimas 24h)
CREATE OR REPLACE VIEW v_high_cognitive_load_patients AS
SELECT
    cls.patient_id,
    i.nome AS patient_name,
    cls.current_load_score,
    cls.interactions_count_24h,
    cls.therapeutic_count_24h,
    cls.fatigue_level,
    cls.rumination_detected,
    cls.active_restrictions,
    cls.last_high_intensity_at
FROM cognitive_load_state cls
JOIN idosos i ON cls.patient_id = i.id
WHERE cls.current_load_score > 0.7
   OR cls.fatigue_level IN ('moderate', 'severe')
   OR cls.rumination_detected = TRUE
ORDER BY cls.current_load_score DESC;

-- Pacientes com risco ético elevado
CREATE OR REPLACE VIEW v_high_ethical_risk_patients AS
SELECT
    ebs.patient_id,
    i.nome AS patient_name,
    ebs.overall_ethical_risk,
    ebs.attachment_risk_score,
    ebs.eva_vs_human_ratio,
    ebs.attachment_phrases_7d,
    ebs.signifier_eva_percentage,
    ebs.limit_enforcement_level,
    ebs.last_redirect_at,
    ebs.last_family_alert_at
FROM ethical_boundary_state ebs
JOIN idosos i ON ebs.patient_id = i.id
WHERE ebs.overall_ethical_risk IN ('high', 'critical')
   OR ebs.eva_vs_human_ratio > 10
   OR ebs.attachment_phrases_7d >= 3
ORDER BY ebs.overall_ethical_risk DESC, ebs.eva_vs_human_ratio DESC;

-- Dashboard: Eventos críticos não resolvidos
CREATE OR REPLACE VIEW v_critical_events_pending AS
SELECT
    'cognitive' AS event_category,
    cls.patient_id,
    i.nome AS patient_name,
    'high_cognitive_load' AS event_type,
    cls.current_load_score AS severity_score,
    cls.last_high_intensity_at AS last_event_at
FROM cognitive_load_state cls
JOIN idosos i ON cls.patient_id = i.id
WHERE cls.current_load_score > 0.8

UNION ALL

SELECT
    'ethical' AS event_category,
    ebe.patient_id,
    i.nome AS patient_name,
    ebe.event_type,
    (CASE ebe.severity
        WHEN 'critical' THEN 1.0
        WHEN 'high' THEN 0.75
        WHEN 'medium' THEN 0.5
        ELSE 0.25
    END) AS severity_score,
    ebe.timestamp AS last_event_at
FROM ethical_boundary_events ebe
JOIN idosos i ON ebe.patient_id = i.id
WHERE ebe.severity IN ('high', 'critical')
  AND ebe.resolved_at IS NULL
ORDER BY severity_score DESC, last_event_at DESC;

COMMENT ON VIEW v_high_cognitive_load_patients IS 'Pacientes com carga cognitiva elevada que requerem redução de intensidade';
COMMENT ON VIEW v_high_ethical_risk_patients IS 'Pacientes com risco de dependência ou apego excessivo ao EVA';
COMMENT ON VIEW v_critical_events_pending IS 'Dashboard de eventos críticos não resolvidos (cognitivos + éticos)';
