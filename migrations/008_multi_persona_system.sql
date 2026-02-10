-- ============================================================================
-- SPRINT 5: MULTI-PERSONA SYSTEM
-- ============================================================================
-- Descrição: Sistema de personas configuráveis (Companion, Clinical, Emergency, Educator)
--            com limites, ferramentas e System Instructions específicos por contexto
-- Autor: EVA-Mind Development Team
-- Data: 2026-01-24
-- ============================================================================

-- ============================================================================
-- 1. PERSONA DEFINITIONS (DEFINIÇÕES DE PERSONAS)
-- ============================================================================
-- Define configurações globais de cada persona
CREATE TABLE IF NOT EXISTS persona_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    persona_code VARCHAR(50) NOT NULL UNIQUE, -- 'companion', 'clinical', 'emergency', 'educator'
    persona_name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,

    -- Configurações comportamentais
    voice_id VARCHAR(50), -- Voz padrão do Gemini
    tone VARCHAR(100), -- "warm, friendly" | "professional, empathetic but distant"
    emotional_depth DECIMAL(3,2) CHECK (emotional_depth BETWEEN 0 AND 1), -- 0-1
    narrative_freedom DECIMAL(3,2) CHECK (narrative_freedom BETWEEN 0 AND 1), -- 0-1

    -- Limites de sessão
    max_session_duration_minutes INTEGER, -- Duração máxima de uma sessão
    max_daily_interactions INTEGER, -- Máximo de interações por dia

    -- Limites éticos específicos
    max_intimacy_level DECIMAL(3,2) CHECK (max_intimacy_level BETWEEN 0 AND 1),
    require_professional_oversight BOOLEAN DEFAULT FALSE,
    can_override_patient_refusal BOOLEAN DEFAULT FALSE, -- Só TRUE para emergency

    -- Ferramentas permitidas
    allowed_tools TEXT[], -- Lista de tools que pode usar
    prohibited_tools TEXT[], -- Lista de tools bloqueados

    -- Tópicos e ações
    allowed_topics TEXT[], -- Tópicos permitidos
    prohibited_topics TEXT[], -- Tópicos proibidos

    -- System Instructions template
    system_instruction_template TEXT NOT NULL,

    -- Prioridades
    priorities TEXT[], -- Lista ordenada de prioridades

    -- Metadados
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_persona_definitions_code ON persona_definitions(persona_code);
CREATE INDEX IF NOT EXISTS idx_persona_definitions_active ON persona_definitions(is_active) WHERE is_active = TRUE;

COMMENT ON TABLE persona_definitions IS 'Configurações globais de cada persona (Companion, Clinical, Emergency, Educator)';
COMMENT ON COLUMN persona_definitions.emotional_depth IS '0=robótico, 1=muito empático';
COMMENT ON COLUMN persona_definitions.narrative_freedom IS '0=respostas curtas/objetivas, 1=narrativas longas';


-- ============================================================================
-- 2. PERSONA SESSIONS (SESSÕES DE PERSONA)
-- ============================================================================
-- Rastreia quando cada persona é ativada para cada paciente
CREATE TABLE IF NOT EXISTS persona_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    persona_code VARCHAR(50) NOT NULL,

    -- Timing
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP,
    duration_seconds INTEGER,

    -- Trigger
    trigger_reason VARCHAR(200) NOT NULL, -- "manual_activation", "crisis_detected", "hospital_admission", etc
    triggered_by VARCHAR(100), -- "system", "doctor", "family", "patient"

    -- Context
    context_data JSONB, -- Informações adicionais sobre o contexto

    -- Ferramentas usadas durante a sessão
    tools_used TEXT[],
    tool_usage_count JSONB, -- {"play_music": 3, "apply_phq9": 1}

    -- Compliance
    boundary_violations INTEGER DEFAULT 0,
    violated_rules TEXT[], -- Regras que foram violadas
    escalation_required BOOLEAN DEFAULT FALSE,
    escalation_reason TEXT,

    -- Override (se alguma regra foi sobrescrita manualmente)
    manual_overrides JSONB, -- [{rule: "max_duration", reason: "emergency", by: "doctor"}]

    -- Satisfação
    patient_feedback_rating INTEGER CHECK (patient_feedback_rating BETWEEN 1 AND 5),
    patient_feedback_text TEXT,

    -- Status
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'completed', 'terminated', 'escalated'))
);

CREATE INDEX IF NOT EXISTS idx_persona_sessions_patient ON persona_sessions(patient_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_persona_sessions_persona ON persona_sessions(persona_code);
CREATE INDEX IF NOT EXISTS idx_persona_sessions_active ON persona_sessions(patient_id, status) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_persona_sessions_violations ON persona_sessions(patient_id) WHERE boundary_violations > 0;

COMMENT ON TABLE persona_sessions IS 'Histórico de ativações de personas para cada paciente';
COMMENT ON COLUMN persona_sessions.boundary_violations IS 'Número de violações de limites durante a sessão';


-- ============================================================================
-- 3. PERSONA_ACTIVATION_RULES (REGRAS DE ATIVAÇÃO AUTOMÁTICA)
-- ============================================================================
-- Define quando trocar de persona automaticamente
CREATE TABLE IF NOT EXISTS persona_activation_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_name VARCHAR(200) NOT NULL UNIQUE,
    description TEXT,

    -- Persona alvo
    target_persona_code VARCHAR(50) NOT NULL,

    -- Prioridade (1=mais alta)
    priority INTEGER NOT NULL DEFAULT 100,

    -- Condições (SQL-like ou JSON)
    conditions JSONB NOT NULL,
    -- Exemplo: {
    --   "cssrs_score": {">=": 4},
    --   "crisis_detected": true,
    --   "patient_location": "hospital"
    -- }

    -- Ação
    auto_activate BOOLEAN DEFAULT TRUE, -- Ativar automaticamente ou apenas sugerir
    notify_staff BOOLEAN DEFAULT FALSE,
    notification_message TEXT,

    -- Duração
    min_duration_minutes INTEGER, -- Tempo mínimo que deve ficar nesta persona
    max_duration_minutes INTEGER, -- Tempo máximo (depois volta ao default)

    -- Cooldown
    cooldown_hours INTEGER, -- Tempo antes de poder ativar novamente

    -- Metadados
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_persona_activation_rules_persona ON persona_activation_rules(target_persona_code);
CREATE INDEX IF NOT EXISTS idx_persona_activation_rules_priority ON persona_activation_rules(priority ASC) WHERE is_active = TRUE;

COMMENT ON TABLE persona_activation_rules IS 'Regras para ativação automática de personas baseado em condições';


-- ============================================================================
-- 4. PERSONA_TOOL_PERMISSIONS (PERMISSÕES DE FERRAMENTAS)
-- ============================================================================
-- Define quais ferramentas cada persona pode usar
CREATE TABLE IF NOT EXISTS persona_tool_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    persona_code VARCHAR(50) NOT NULL,
    tool_name VARCHAR(100) NOT NULL,

    -- Permissão
    permission_type VARCHAR(50) NOT NULL CHECK (permission_type IN (
        'allowed',
        'prohibited',
        'require_approval',
        'allowed_with_limits'
    )),

    -- Limites (se allowed_with_limits)
    max_uses_per_day INTEGER,
    max_uses_per_session INTEGER,
    requires_reason BOOLEAN DEFAULT FALSE,

    -- Restrições adicionais
    allowed_parameters JSONB, -- Parâmetros permitidos
    prohibited_parameters JSONB, -- Parâmetros proibidos

    -- Aprovação
    approval_required_from VARCHAR(50), -- "doctor", "family", "supervisor"
    approval_timeout_minutes INTEGER,

    -- Override de emergência
    emergency_override_allowed BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(persona_code, tool_name)
);

CREATE INDEX IF NOT EXISTS idx_persona_tool_permissions_persona ON persona_tool_permissions(persona_code);
CREATE INDEX IF NOT EXISTS idx_persona_tool_permissions_tool ON persona_tool_permissions(tool_name);

COMMENT ON TABLE persona_tool_permissions IS 'Controle granular de permissões de ferramentas por persona';


-- ============================================================================
-- 5. PERSONA_TRANSITIONS (TRANSIÇÕES DE PERSONA)
-- ============================================================================
-- Log de todas as mudanças de persona
CREATE TABLE IF NOT EXISTS persona_transitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Transição
    from_persona_code VARCHAR(50),
    to_persona_code VARCHAR(50) NOT NULL,

    -- Timing
    transition_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Motivo
    reason VARCHAR(200) NOT NULL,
    triggered_by VARCHAR(100) NOT NULL, -- "system", "doctor", "rule:crisis_detected"
    rule_id UUID REFERENCES persona_activation_rules(id),

    -- Context
    patient_state_at_transition JSONB, -- PHQ-9, localização, etc

    -- Resultado
    transition_successful BOOLEAN DEFAULT TRUE,
    transition_error TEXT,

    -- Duração da persona anterior
    previous_persona_duration_minutes INTEGER
);

CREATE INDEX IF NOT EXISTS idx_persona_transitions_patient ON persona_transitions(patient_id, transition_at DESC);
CREATE INDEX IF NOT EXISTS idx_persona_transitions_to_persona ON persona_transitions(to_persona_code);
CREATE INDEX IF NOT EXISTS idx_persona_transitions_rule ON persona_transitions(rule_id);

COMMENT ON TABLE persona_transitions IS 'Log de todas as mudanças de persona com motivos e contexto';


-- ============================================================================
-- 6. VIEWS ÚTEIS
-- ============================================================================

-- View: Persona ativa atual de cada paciente
CREATE OR REPLACE VIEW v_current_active_personas AS
SELECT DISTINCT ON (patient_id)
    ps.patient_id,
    i.nome AS patient_name,
    ps.persona_code,
    pd.persona_name,
    ps.started_at,
    EXTRACT(EPOCH FROM (NOW() - ps.started_at))/60 AS minutes_active,
    ps.trigger_reason,
    ps.boundary_violations,
    ps.status
FROM persona_sessions ps
JOIN idosos i ON ps.patient_id = i.id
LEFT JOIN persona_definitions pd ON ps.persona_code = pd.persona_code
WHERE ps.status = 'active'
ORDER BY ps.patient_id, ps.started_at DESC;

COMMENT ON VIEW v_current_active_personas IS 'Persona ativa atual de cada paciente';


-- View: Violações de limites por persona
CREATE OR REPLACE VIEW v_persona_boundary_violations AS
SELECT
    persona_code,
    COUNT(*) AS total_sessions,
    SUM(boundary_violations) AS total_violations,
    ROUND(AVG(boundary_violations), 2) AS avg_violations_per_session,
    COUNT(*) FILTER (WHERE boundary_violations > 0) AS sessions_with_violations,
    ROUND(
        (COUNT(*) FILTER (WHERE boundary_violations > 0)::NUMERIC / NULLIF(COUNT(*), 0)) * 100,
        1
    ) AS violation_rate_pct
FROM persona_sessions
GROUP BY persona_code
ORDER BY total_violations DESC;

COMMENT ON VIEW v_persona_boundary_violations IS 'Estatísticas de violações de limites por persona';


-- View: Uso de ferramentas por persona
CREATE OR REPLACE VIEW v_persona_tool_usage AS
SELECT
    ps.persona_code,
    unnest(ps.tools_used) AS tool_name,
    COUNT(*) AS usage_count,
    COUNT(DISTINCT ps.patient_id) AS unique_patients
FROM persona_sessions ps
WHERE ps.tools_used IS NOT NULL
GROUP BY ps.persona_code, tool_name
ORDER BY ps.persona_code, usage_count DESC;

COMMENT ON VIEW v_persona_tool_usage IS 'Ferramentas mais usadas por cada persona';


-- View: Estatísticas de transições
CREATE OR REPLACE VIEW v_persona_transition_stats AS
SELECT
    from_persona_code,
    to_persona_code,
    COUNT(*) AS transition_count,
    ARRAY_AGG(DISTINCT reason) AS common_reasons,
    ROUND(AVG(previous_persona_duration_minutes), 1) AS avg_previous_duration_minutes
FROM persona_transitions
WHERE from_persona_code IS NOT NULL
GROUP BY from_persona_code, to_persona_code
ORDER BY transition_count DESC;

COMMENT ON VIEW v_persona_transition_stats IS 'Estatísticas de transições entre personas';


-- ============================================================================
-- 7. FUNCTIONS
-- ============================================================================

-- Function: Obter persona atual de um paciente
CREATE OR REPLACE FUNCTION get_current_persona(p_patient_id INTEGER)
RETURNS TABLE(
    persona_code VARCHAR(50),
    persona_name VARCHAR(100),
    session_id UUID,
    started_at TIMESTAMP,
    minutes_active NUMERIC,
    system_instruction TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        ps.persona_code,
        pd.persona_name,
        ps.id AS session_id,
        ps.started_at,
        EXTRACT(EPOCH FROM (NOW() - ps.started_at))/60 AS minutes_active,
        pd.system_instruction_template
    FROM persona_sessions ps
    JOIN persona_definitions pd ON ps.persona_code = pd.persona_code
    WHERE ps.patient_id = p_patient_id
      AND ps.status = 'active'
    ORDER BY ps.started_at DESC
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_current_persona IS 'Retorna persona ativa atual de um paciente';


-- Function: Verificar se ferramenta é permitida
CREATE OR REPLACE FUNCTION is_tool_allowed(
    p_persona_code VARCHAR(50),
    p_tool_name VARCHAR(100)
)
RETURNS BOOLEAN AS $$
DECLARE
    v_permission VARCHAR(50);
BEGIN
    -- Buscar permissão específica
    SELECT permission_type INTO v_permission
    FROM persona_tool_permissions
    WHERE persona_code = p_persona_code
      AND tool_name = p_tool_name;

    -- Se não encontrou, verificar nas listas allowed/prohibited da definição
    IF v_permission IS NULL THEN
        -- Verificar se está na lista allowed_tools
        IF EXISTS (
            SELECT 1 FROM persona_definitions
            WHERE persona_code = p_persona_code
              AND p_tool_name = ANY(allowed_tools)
        ) THEN
            RETURN TRUE;
        END IF;

        -- Verificar se está na lista prohibited_tools
        IF EXISTS (
            SELECT 1 FROM persona_definitions
            WHERE persona_code = p_persona_code
              AND p_tool_name = ANY(prohibited_tools)
        ) THEN
            RETURN FALSE;
        END IF;

        -- Default: não permitido
        RETURN FALSE;
    END IF;

    -- Se permission_type = 'allowed' ou 'allowed_with_limits', permitir
    RETURN v_permission IN ('allowed', 'allowed_with_limits');
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION is_tool_allowed IS 'Verifica se uma ferramenta é permitida para uma persona';


-- Function: Avaliar regras de ativação
CREATE OR REPLACE FUNCTION evaluate_activation_rules(p_patient_id INTEGER)
RETURNS TABLE(
    rule_id UUID,
    rule_name VARCHAR(200),
    target_persona VARCHAR(50),
    priority INTEGER,
    should_activate BOOLEAN
) AS $$
BEGIN
    -- Implementação simplificada
    -- Na prática, avaliaria as condições JSON contra o estado do paciente
    RETURN QUERY
    SELECT
        r.id,
        r.rule_name,
        r.target_persona_code,
        r.priority,
        TRUE AS should_activate -- Placeholder
    FROM persona_activation_rules r
    WHERE r.is_active = TRUE
    ORDER BY r.priority ASC
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION evaluate_activation_rules IS 'Avalia regras de ativação e retorna qual persona deve ser ativada';


-- ============================================================================
-- 8. TRIGGERS
-- ============================================================================

-- Trigger: Atualizar updated_at em persona_definitions
CREATE OR REPLACE FUNCTION update_persona_definition_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_persona_definition_updated_at ON persona_definitions;
CREATE TRIGGER trigger_update_persona_definition_updated_at
    BEFORE UPDATE ON persona_definitions
    FOR EACH ROW
    EXECUTE FUNCTION update_persona_definition_updated_at();


-- Trigger: Calcular duration quando sessão termina
CREATE OR REPLACE FUNCTION calculate_persona_session_duration()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.ended_at IS NOT NULL AND OLD.ended_at IS NULL THEN
        NEW.duration_seconds = EXTRACT(EPOCH FROM (NEW.ended_at - NEW.started_at));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_calculate_persona_session_duration ON persona_sessions;
CREATE TRIGGER trigger_calculate_persona_session_duration
    BEFORE UPDATE ON persona_sessions
    FOR EACH ROW
    EXECUTE FUNCTION calculate_persona_session_duration();


-- Trigger: Log de transição quando persona muda
CREATE OR REPLACE FUNCTION log_persona_transition()
RETURNS TRIGGER AS $$
DECLARE
    v_previous_persona VARCHAR(50);
    v_previous_duration INTEGER;
BEGIN
    -- Se status mudou para 'active', verificar se há outra persona ativa
    IF NEW.status = 'active' AND (OLD.status IS NULL OR OLD.status != 'active') THEN
        -- Buscar persona ativa anterior
        SELECT persona_code, EXTRACT(EPOCH FROM (NOW() - started_at))/60
        INTO v_previous_persona, v_previous_duration
        FROM persona_sessions
        WHERE patient_id = NEW.patient_id
          AND status = 'active'
          AND id != NEW.id
        ORDER BY started_at DESC
        LIMIT 1;

        -- Se encontrou, desativar e logar transição
        IF v_previous_persona IS NOT NULL THEN
            -- Desativar persona anterior
            UPDATE persona_sessions
            SET status = 'completed',
                ended_at = NOW()
            WHERE patient_id = NEW.patient_id
              AND status = 'active'
              AND id != NEW.id;

            -- Logar transição
            INSERT INTO persona_transitions (
                patient_id,
                from_persona_code,
                to_persona_code,
                reason,
                triggered_by,
                previous_persona_duration_minutes
            ) VALUES (
                NEW.patient_id,
                v_previous_persona,
                NEW.persona_code,
                NEW.trigger_reason,
                NEW.triggered_by,
                v_previous_duration
            );
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_log_persona_transition ON persona_sessions;
CREATE TRIGGER trigger_log_persona_transition
    AFTER INSERT OR UPDATE ON persona_sessions
    FOR EACH ROW
    EXECUTE FUNCTION log_persona_transition();


-- ============================================================================
-- FIM DA MIGRATION
-- ============================================================================
