-- Migration para auditoria de mudanças de diretrizes pelo arquiteto

CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    idoso_id BIGINT REFERENCES idosos(id),
    action_type VARCHAR(50) NOT NULL,
    action_data JSONB,
    performed_by VARCHAR(14), -- CPF do arquiteto
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_audit_log_idoso ON audit_log(idoso_id);
CREATE INDEX idx_audit_log_type ON audit_log(action_type);
CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);

COMMENT ON TABLE audit_log IS 'Registro de auditoria de mudanças críticas no sistema';
COMMENT ON COLUMN audit_log.action_type IS 'Tipo de ação: DIRECTIVE_CHANGE, DATA_EXPORT, PERSONAL_INFO_UPDATE, etc';
COMMENT ON COLUMN audit_log.performed_by IS 'CPF do usuário que executou a ação';

-- Tabela para armazenar informações pessoais extraídas
CREATE TABLE IF NOT EXISTS personal_info (
    id BIGSERIAL PRIMARY KEY,
    idoso_id BIGINT REFERENCES idosos(id) NOT NULL,
    info_type VARCHAR(50) NOT NULL, -- 'preference', 'family', 'hobby', 'medical', etc
    info_key VARCHAR(100) NOT NULL,  -- 'favorite_food', 'daughter_name', etc
    info_value TEXT NOT NULL,
    confidence FLOAT DEFAULT 0.8,    -- Confiança na extração (0-1)
    source VARCHAR(50),               -- 'conversation', 'manual', 'import'
    extracted_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(idoso_id, info_type, info_key)
);

CREATE INDEX idx_personal_info_idoso ON personal_info(idoso_id);
CREATE INDEX idx_personal_info_type ON personal_info(info_type);
CREATE INDEX idx_personal_info_updated ON personal_info(updated_at DESC);

COMMENT ON TABLE personal_info IS 'Informações pessoais extraídas de conversas (gostos, família, hobbies, etc)';
COMMENT ON COLUMN personal_info.info_type IS 'Categoria: preference, family, hobby, medical, location, etc';
COMMENT ON COLUMN personal_info.info_key IS 'Chave específica: favorite_food, daughter_name, hobby_gardening, etc';
COMMENT ON COLUMN personal_info.confidence IS 'Confiança na extração (0-1), usado para resolver conflitos';
