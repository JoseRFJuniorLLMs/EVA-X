-- =====================================================
-- EVA-Mind-FZPN: Escalation Logs
-- Sprint 8: Alert Escalation System
-- =====================================================

-- Tabela para registrar logs de escalonamento de alertas
CREATE TABLE IF NOT EXISTS escalation_logs (
    id SERIAL PRIMARY KEY,
    alert_id VARCHAR(100) NOT NULL UNIQUE,
    elder_name VARCHAR(255) NOT NULL,
    reason TEXT NOT NULL,
    priority VARCHAR(20) NOT NULL CHECK (priority IN ('critical', 'high', 'medium', 'low')),

    -- Acknowledgment
    acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_by VARCHAR(255),
    acknowledged_at TIMESTAMPTZ,

    -- Delivery
    final_channel VARCHAR(20) CHECK (final_channel IN ('push', 'sms', 'whatsapp', 'email', 'call')),
    attempts_count INTEGER DEFAULT 0,

    -- Timestamps
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Tabela para registrar cada tentativa de entrega
CREATE TABLE IF NOT EXISTS escalation_attempts (
    id SERIAL PRIMARY KEY,
    alert_id VARCHAR(100) NOT NULL REFERENCES escalation_logs(alert_id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL CHECK (channel IN ('push', 'sms', 'whatsapp', 'email', 'call')),
    success BOOLEAN NOT NULL,
    message_id VARCHAR(255),
    error_message TEXT,
    latency_ms INTEGER,
    attempted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Tabela para configuracao de contatos de emergencia por idoso
CREATE TABLE IF NOT EXISTS emergency_contacts (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    cuidador_id INTEGER REFERENCES cuidadores(id) ON DELETE CASCADE,

    -- Contact info (pode ser diferente do cuidador cadastrado)
    nome VARCHAR(255) NOT NULL,
    telefone VARCHAR(20),
    email VARCHAR(255),

    -- Preferences
    priority INTEGER DEFAULT 1, -- 1 = primary, 2 = secondary
    channels_enabled TEXT[] DEFAULT ARRAY['push', 'sms', 'email'], -- canais habilitados
    quiet_hours_start TIME, -- horario de nao perturbe (inicio)
    quiet_hours_end TIME,   -- horario de nao perturbe (fim)

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    last_contacted_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indices para performance
CREATE INDEX IF NOT EXISTS idx_escalation_logs_alert_id ON escalation_logs(alert_id);
CREATE INDEX IF NOT EXISTS idx_escalation_logs_priority ON escalation_logs(priority);
CREATE INDEX IF NOT EXISTS idx_escalation_logs_acknowledged ON escalation_logs(acknowledged);
CREATE INDEX IF NOT EXISTS idx_escalation_logs_started_at ON escalation_logs(started_at);

CREATE INDEX IF NOT EXISTS idx_escalation_attempts_alert_id ON escalation_attempts(alert_id);
CREATE INDEX IF NOT EXISTS idx_escalation_attempts_channel ON escalation_attempts(channel);

CREATE INDEX IF NOT EXISTS idx_emergency_contacts_idoso ON emergency_contacts(idoso_id);
CREATE INDEX IF NOT EXISTS idx_emergency_contacts_cuidador ON emergency_contacts(cuidador_id);
CREATE INDEX IF NOT EXISTS idx_emergency_contacts_priority ON emergency_contacts(priority);

-- View para metricas de escalonamento
CREATE OR REPLACE VIEW v_escalation_metrics AS
SELECT
    DATE_TRUNC('day', started_at) as day,
    priority,
    COUNT(*) as total_alerts,
    SUM(CASE WHEN acknowledged THEN 1 ELSE 0 END) as acknowledged_count,
    AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as avg_resolution_seconds,
    AVG(attempts_count) as avg_attempts,
    MODE() WITHIN GROUP (ORDER BY final_channel) as most_used_channel
FROM escalation_logs
WHERE started_at > NOW() - INTERVAL '30 days'
GROUP BY DATE_TRUNC('day', started_at), priority
ORDER BY day DESC, priority;

-- View para contatos de emergencia ativos
CREATE OR REPLACE VIEW v_emergency_contacts_active AS
SELECT
    ec.id,
    ec.idoso_id,
    i.nome as idoso_nome,
    ec.cuidador_id,
    ec.nome as contato_nome,
    ec.telefone,
    ec.email,
    ec.priority,
    ec.channels_enabled,
    ec.quiet_hours_start,
    ec.quiet_hours_end,
    ec.last_contacted_at,
    -- Check if currently in quiet hours
    CASE
        WHEN ec.quiet_hours_start IS NOT NULL AND ec.quiet_hours_end IS NOT NULL
        THEN CURRENT_TIME BETWEEN ec.quiet_hours_start AND ec.quiet_hours_end
        ELSE FALSE
    END as in_quiet_hours
FROM emergency_contacts ec
JOIN idosos i ON ec.idoso_id = i.id
WHERE ec.is_active = TRUE
ORDER BY ec.idoso_id, ec.priority;

-- Funcao para obter contatos de emergencia de um idoso
CREATE OR REPLACE FUNCTION get_emergency_contacts(p_idoso_id INTEGER)
RETURNS TABLE (
    id INTEGER,
    nome VARCHAR,
    telefone VARCHAR,
    email VARCHAR,
    priority INTEGER,
    channels_enabled TEXT[],
    in_quiet_hours BOOLEAN
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        ec.id::INTEGER,
        ec.nome,
        ec.telefone,
        ec.email,
        ec.priority,
        ec.channels_enabled,
        CASE
            WHEN ec.quiet_hours_start IS NOT NULL AND ec.quiet_hours_end IS NOT NULL
            THEN CURRENT_TIME BETWEEN ec.quiet_hours_start AND ec.quiet_hours_end
            ELSE FALSE
        END
    FROM emergency_contacts ec
    WHERE ec.idoso_id = p_idoso_id
      AND ec.is_active = TRUE
    ORDER BY ec.priority;
END;
$$ LANGUAGE plpgsql;

-- Trigger para atualizar updated_at
CREATE OR REPLACE FUNCTION update_emergency_contacts_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_emergency_contacts_updated
    BEFORE UPDATE ON emergency_contacts
    FOR EACH ROW
    EXECUTE FUNCTION update_emergency_contacts_timestamp();

-- Comentarios
COMMENT ON TABLE escalation_logs IS 'Logs de escalonamento de alertas de emergencia';
COMMENT ON TABLE escalation_attempts IS 'Tentativas individuais de entrega de alertas';
COMMENT ON TABLE emergency_contacts IS 'Contatos de emergencia configurados por idoso';

COMMENT ON COLUMN escalation_logs.priority IS 'Prioridade: critical (30s), high (2min), medium (5min), low (15min)';
COMMENT ON COLUMN escalation_logs.final_channel IS 'Canal que conseguiu entregar o alerta';
COMMENT ON COLUMN emergency_contacts.channels_enabled IS 'Canais habilitados: push, sms, whatsapp, email, call';
COMMENT ON COLUMN emergency_contacts.quiet_hours_start IS 'Horario de nao perturbe (inicio) - alertas criticos ignoram';
