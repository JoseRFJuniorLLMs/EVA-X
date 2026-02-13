-- ============================================================================
-- Migration 026: Legacy Mode (Digital Immortality)
-- Pos-morte flag, heir consent, personality snapshots, read-only API
-- ============================================================================

-- 1. Adicionar flags de legacy mode na tabela de idosos
ALTER TABLE idosos ADD COLUMN IF NOT EXISTS legacy_mode BOOLEAN DEFAULT FALSE;
ALTER TABLE idosos ADD COLUMN IF NOT EXISTS pos_morte BOOLEAN DEFAULT FALSE;
ALTER TABLE idosos ADD COLUMN IF NOT EXISTS pos_morte_activated_at TIMESTAMP;
ALTER TABLE idosos ADD COLUMN IF NOT EXISTS pos_morte_activated_by VARCHAR(50); -- CPF do herdeiro que ativou

-- 2. Tabela de herdeiros com consent granular
CREATE TABLE IF NOT EXISTS legacy_heirs (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),
    heir_name VARCHAR(255) NOT NULL,
    heir_cpf VARCHAR(14) NOT NULL,
    heir_email VARCHAR(255),
    heir_phone VARCHAR(20),
    relationship VARCHAR(50) NOT NULL, -- 'filho', 'filha', 'neto', 'conjuge', 'outro'

    -- Consent granular por tipo de acesso
    can_read_memories BOOLEAN DEFAULT FALSE,
    can_read_emotions BOOLEAN DEFAULT FALSE,
    can_read_signifiers BOOLEAN DEFAULT FALSE,
    can_read_personality BOOLEAN DEFAULT FALSE,
    can_read_clinical BOOLEAN DEFAULT FALSE,
    can_activate_pos_morte BOOLEAN DEFAULT FALSE,
    can_export_snapshot BOOLEAN DEFAULT FALSE,

    -- Metadados
    consent_given_at TIMESTAMP,
    consent_given_method VARCHAR(50), -- 'verbal_recorded', 'written', 'digital_signed'
    consent_revoked_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(idoso_id, heir_cpf)
);

-- 3. Tabela de personality snapshots (exportacao JSON)
CREATE TABLE IF NOT EXISTS personality_snapshots (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),
    snapshot_version INTEGER DEFAULT 1,

    -- Enneagram data
    enneagram_type INTEGER,
    enneagram_wing INTEGER,
    enneagram_integration_direction INTEGER,
    enneagram_disintegration_direction INTEGER,

    -- Significantes (top-K palavras emocionais)
    top_signifiers JSONB, -- [{"word": "solidao", "frequency": 42, "charge": 0.9}]

    -- Top-K memorias mais importantes
    top_memories JSONB, -- [{"content": "...", "emotion": "saudade", "importance": 0.95}]

    -- Perfil emocional
    emotional_profile JSONB, -- {"dominant": "melancolia", "secondary": "esperanca", "triggers": [...]}

    -- Lacan state
    lacan_state JSONB, -- {"transferencia_patterns": [...], "desire_patterns": [...], "addressee": "..."}

    -- Relacoes significativas (grafo reduzido)
    significant_relations JSONB, -- [{"person": "Maria", "relation": "filha", "mentions": 150}]

    -- Full snapshot blob (para restauracao completa)
    full_snapshot JSONB,

    -- Metadados
    created_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(50), -- CPF de quem solicitou
    snapshot_size_bytes INTEGER,
    is_latest BOOLEAN DEFAULT TRUE
);

-- 4. Tabela de audit trail para acessos pos-morte
CREATE TABLE IF NOT EXISTS legacy_access_log (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id),
    heir_id INTEGER REFERENCES legacy_heirs(id),
    heir_cpf VARCHAR(14) NOT NULL,
    action_type VARCHAR(50) NOT NULL, -- 'read_memories', 'read_personality', 'export_snapshot', 'activate_pos_morte'
    action_detail TEXT,
    ip_address VARCHAR(45),
    user_agent TEXT,
    accessed_at TIMESTAMP DEFAULT NOW()
);

-- 5. Indexes
CREATE INDEX IF NOT EXISTS idx_legacy_heirs_idoso ON legacy_heirs(idoso_id);
CREATE INDEX IF NOT EXISTS idx_legacy_heirs_cpf ON legacy_heirs(heir_cpf);
CREATE INDEX IF NOT EXISTS idx_personality_snapshots_idoso ON personality_snapshots(idoso_id);
CREATE INDEX IF NOT EXISTS idx_personality_snapshots_latest ON personality_snapshots(idoso_id, is_latest) WHERE is_latest = TRUE;
CREATE INDEX IF NOT EXISTS idx_legacy_access_log_idoso ON legacy_access_log(idoso_id);
CREATE INDEX IF NOT EXISTS idx_legacy_access_log_heir ON legacy_access_log(heir_cpf);

-- 6. View: status de legacy de cada paciente
CREATE OR REPLACE VIEW v_legacy_status AS
SELECT
    i.id AS idoso_id,
    i.nome AS nome,
    i.legacy_mode,
    i.pos_morte,
    i.pos_morte_activated_at,
    COUNT(lh.id) AS total_heirs,
    COUNT(CASE WHEN lh.can_activate_pos_morte THEN 1 END) AS heirs_with_activation_rights,
    (SELECT COUNT(*) FROM personality_snapshots ps WHERE ps.idoso_id = i.id AND ps.is_latest = TRUE) AS has_snapshot,
    (SELECT MAX(ps.created_at) FROM personality_snapshots ps WHERE ps.idoso_id = i.id) AS last_snapshot_date
FROM idosos i
LEFT JOIN legacy_heirs lh ON i.id = lh.idoso_id AND lh.is_active = TRUE
WHERE i.legacy_mode = TRUE
GROUP BY i.id, i.nome, i.legacy_mode, i.pos_morte, i.pos_morte_activated_at;

-- 7. Function: ativar pos-morte (verificacao de permissao)
CREATE OR REPLACE FUNCTION activate_pos_morte(p_idoso_id INTEGER, p_heir_cpf VARCHAR)
RETURNS BOOLEAN AS $$
DECLARE
    v_can_activate BOOLEAN;
    v_heir_id INTEGER;
BEGIN
    -- Verificar se herdeiro tem permissao
    SELECT id, can_activate_pos_morte INTO v_heir_id, v_can_activate
    FROM legacy_heirs
    WHERE idoso_id = p_idoso_id AND heir_cpf = p_heir_cpf AND is_active = TRUE;

    IF v_can_activate IS NULL OR v_can_activate = FALSE THEN
        RETURN FALSE;
    END IF;

    -- Ativar pos-morte
    UPDATE idosos SET
        pos_morte = TRUE,
        pos_morte_activated_at = NOW(),
        pos_morte_activated_by = p_heir_cpf
    WHERE id = p_idoso_id AND legacy_mode = TRUE;

    -- Log
    INSERT INTO legacy_access_log (idoso_id, heir_id, heir_cpf, action_type, action_detail)
    VALUES (p_idoso_id, v_heir_id, p_heir_cpf, 'activate_pos_morte', 'Post-mortem mode activated');

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
