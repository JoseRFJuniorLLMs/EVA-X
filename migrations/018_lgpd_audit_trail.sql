-- =====================================================
-- LGPD Audit Trail System
-- Lei Geral de Proteção de Dados (Brazil)
-- =====================================================

-- Main audit log table
CREATE TABLE IF NOT EXISTS lgpd_audit_log (
    id VARCHAR(64) PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Event classification
    event_type VARCHAR(50) NOT NULL,
    data_category VARCHAR(30) NOT NULL,
    legal_basis VARCHAR(50) NOT NULL,

    -- Actor (who performed the action)
    actor_id VARCHAR(100) NOT NULL,
    actor_type VARCHAR(30) NOT NULL, -- user, system, caregiver, admin
    actor_ip INET,

    -- Data subject (whose data was accessed)
    subject_id BIGINT NOT NULL REFERENCES idosos(id),
    subject_cpf VARCHAR(64), -- Hashed for privacy

    -- Event details
    resource VARCHAR(100) NOT NULL, -- Table/resource affected
    action VARCHAR(50) NOT NULL,
    description TEXT,
    fields_accessed JSONB DEFAULT '[]',

    -- Additional metadata
    metadata JSONB DEFAULT '{}',

    -- Retention policy
    retention_days INTEGER NOT NULL DEFAULT 365,
    expires_at TIMESTAMPTZ,

    -- Result
    success BOOLEAN NOT NULL DEFAULT true,
    error_message TEXT,

    -- Indexes for common queries
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_lgpd_audit_subject ON lgpd_audit_log(subject_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_lgpd_audit_event_type ON lgpd_audit_log(event_type, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_lgpd_audit_actor ON lgpd_audit_log(actor_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_lgpd_audit_timestamp ON lgpd_audit_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_lgpd_audit_legal_basis ON lgpd_audit_log(legal_basis);
CREATE INDEX IF NOT EXISTS idx_lgpd_audit_expires ON lgpd_audit_log(expires_at) WHERE expires_at IS NOT NULL;

-- Set expiration date trigger
CREATE OR REPLACE FUNCTION set_audit_expiration()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.expires_at IS NULL THEN
        NEW.expires_at := NEW.timestamp + (NEW.retention_days || ' days')::INTERVAL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_set_audit_expiration ON lgpd_audit_log;
CREATE TRIGGER trigger_set_audit_expiration
    BEFORE INSERT ON lgpd_audit_log
    FOR EACH ROW
    EXECUTE FUNCTION set_audit_expiration();

-- =====================================================
-- Consent Management
-- =====================================================

CREATE TABLE IF NOT EXISTS lgpd_consents (
    id SERIAL PRIMARY KEY,
    subject_id BIGINT NOT NULL REFERENCES idosos(id),

    -- Consent details
    consent_type VARCHAR(50) NOT NULL, -- general, health_data, research, marketing
    purpose TEXT NOT NULL,
    granted BOOLEAN NOT NULL DEFAULT true,

    -- Consent metadata
    version INTEGER NOT NULL DEFAULT 1,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,

    -- Evidence
    evidence_type VARCHAR(30), -- verbal, written, digital
    evidence_reference TEXT, -- Link to evidence storage

    -- Legal basis for processing
    legal_basis VARCHAR(50) NOT NULL DEFAULT 'CONSENT',

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(subject_id, consent_type)
);

CREATE INDEX IF NOT EXISTS idx_lgpd_consents_subject ON lgpd_consents(subject_id);
CREATE INDEX IF NOT EXISTS idx_lgpd_consents_type ON lgpd_consents(consent_type);
CREATE INDEX IF NOT EXISTS idx_lgpd_consents_active ON lgpd_consents(subject_id) WHERE granted = true AND revoked_at IS NULL;

-- =====================================================
-- Data Subject Requests (Art. 18 LGPD)
-- =====================================================

CREATE TABLE IF NOT EXISTS lgpd_requests (
    id SERIAL PRIMARY KEY,
    subject_id BIGINT NOT NULL REFERENCES idosos(id),

    -- Request type (Art. 18 rights)
    request_type VARCHAR(50) NOT NULL, -- access, rectification, deletion, portability, restriction, objection

    -- Request details
    description TEXT,
    scope JSONB DEFAULT '{}', -- Which data categories

    -- Processing
    status VARCHAR(30) NOT NULL DEFAULT 'pending', -- pending, in_progress, completed, denied
    priority VARCHAR(20) NOT NULL DEFAULT 'normal', -- normal, urgent

    -- Deadlines (LGPD requires response within 15 days)
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deadline_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '15 days'),
    completed_at TIMESTAMPTZ,

    -- Response
    response_type VARCHAR(30), -- fulfilled, partially_fulfilled, denied
    response_details TEXT,
    denial_reason TEXT,

    -- Handler
    handled_by VARCHAR(100),
    handler_notes TEXT,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lgpd_requests_subject ON lgpd_requests(subject_id);
CREATE INDEX IF NOT EXISTS idx_lgpd_requests_status ON lgpd_requests(status, deadline_at);
CREATE INDEX IF NOT EXISTS idx_lgpd_requests_pending ON lgpd_requests(deadline_at) WHERE status = 'pending';

-- =====================================================
-- Data Processing Records (Art. 37 LGPD)
-- =====================================================

CREATE TABLE IF NOT EXISTS lgpd_processing_records (
    id SERIAL PRIMARY KEY,

    -- Processing activity identification
    activity_name VARCHAR(100) NOT NULL,
    activity_description TEXT,

    -- Data categories processed
    data_categories JSONB NOT NULL DEFAULT '[]',
    sensitive_data BOOLEAN NOT NULL DEFAULT false,

    -- Legal basis
    legal_basis VARCHAR(50) NOT NULL,
    legal_basis_justification TEXT,

    -- Purpose
    purpose TEXT NOT NULL,

    -- Data subjects
    subject_categories JSONB DEFAULT '[]', -- idosos, cuidadores, etc
    estimated_subjects INTEGER,

    -- Data recipients
    recipients JSONB DEFAULT '[]', -- Internal departments, third parties
    international_transfer BOOLEAN NOT NULL DEFAULT false,
    transfer_countries JSONB DEFAULT '[]',

    -- Retention
    retention_period VARCHAR(50),
    retention_criteria TEXT,

    -- Security measures
    security_measures JSONB DEFAULT '[]',

    -- Data Protection Impact Assessment
    dpia_required BOOLEAN NOT NULL DEFAULT false,
    dpia_reference VARCHAR(100),

    -- Status
    active BOOLEAN NOT NULL DEFAULT true,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_lgpd_processing_active ON lgpd_processing_records(active);
CREATE INDEX IF NOT EXISTS idx_lgpd_processing_sensitive ON lgpd_processing_records(sensitive_data) WHERE sensitive_data = true;

-- =====================================================
-- Data Deletion Requests (Right to Erasure)
-- =====================================================

CREATE TABLE IF NOT EXISTS lgpd_deletion_requests (
    id SERIAL PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES lgpd_requests(id),
    subject_id BIGINT NOT NULL REFERENCES idosos(id),

    -- Scope
    full_deletion BOOLEAN NOT NULL DEFAULT false,
    tables_to_delete JSONB DEFAULT '[]',

    -- Execution
    status VARCHAR(30) NOT NULL DEFAULT 'pending', -- pending, approved, executed, denied
    executed_at TIMESTAMPTZ,

    -- What was deleted
    deletion_log JSONB DEFAULT '{}',
    records_deleted INTEGER DEFAULT 0,

    -- Exceptions (data that must be retained)
    retained_data JSONB DEFAULT '{}',
    retention_reason TEXT,

    -- Verification
    verified_by VARCHAR(100),
    verified_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =====================================================
-- Data Export/Portability (Art. 18, V LGPD)
-- =====================================================

CREATE TABLE IF NOT EXISTS lgpd_data_exports (
    id SERIAL PRIMARY KEY,
    request_id INTEGER REFERENCES lgpd_requests(id),
    subject_id BIGINT NOT NULL REFERENCES idosos(id),

    -- Export details
    format VARCHAR(20) NOT NULL DEFAULT 'json', -- json, csv, xml
    scope JSONB DEFAULT '[]', -- Which tables/data

    -- Execution
    status VARCHAR(30) NOT NULL DEFAULT 'pending', -- pending, generating, completed, failed
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Output
    file_path VARCHAR(500),
    file_size_bytes BIGINT,
    checksum VARCHAR(64),

    -- Access
    download_token VARCHAR(100) UNIQUE,
    download_expires_at TIMESTAMPTZ,
    downloaded_at TIMESTAMPTZ,
    download_count INTEGER DEFAULT 0,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lgpd_exports_subject ON lgpd_data_exports(subject_id);
CREATE INDEX IF NOT EXISTS idx_lgpd_exports_token ON lgpd_data_exports(download_token) WHERE download_token IS NOT NULL;

-- =====================================================
-- Initial Processing Records (Art. 37)
-- =====================================================

INSERT INTO lgpd_processing_records (
    activity_name,
    activity_description,
    data_categories,
    sensitive_data,
    legal_basis,
    legal_basis_justification,
    purpose,
    subject_categories,
    retention_period,
    security_measures
) VALUES
(
    'Conversational Support',
    'AI-powered conversational support for elderly patients',
    '["personal", "conversation", "behavioral"]',
    false,
    'CONSENT',
    'Explicit consent obtained during registration',
    'Provide emotional and practical support to elderly patients',
    '["idosos"]',
    '2 years',
    '["encryption_at_rest", "encryption_in_transit", "access_control", "audit_logging"]'
),
(
    'Clinical Assessment',
    'PHQ-9, GAD-7, and C-SSRS clinical mental health assessments',
    '["clinical", "sensitive"]',
    true,
    'HEALTH_PROTECTION',
    'Art. 7, VIII - Protection of life and physical safety of the data subject',
    'Early detection of mental health risks including suicide risk',
    '["idosos"]',
    '5 years',
    '["encryption_at_rest", "encryption_in_transit", "access_control", "audit_logging", "data_minimization", "pseudonymization"]'
),
(
    'Emergency Alerts',
    'Emergency notification system to caregivers and family members',
    '["personal", "contact", "sensitive"]',
    true,
    'HEALTH_PROTECTION',
    'Art. 7, VIII - Protection of life; Art. 11, II, f - Health protection in emergencies',
    'Ensure timely response to health emergencies',
    '["idosos", "cuidadores"]',
    '3 years',
    '["encryption_at_rest", "encryption_in_transit", "access_control", "audit_logging"]'
),
(
    'Memory System',
    'Long-term memory storage for personalized care',
    '["conversation", "behavioral", "personal"]',
    false,
    'CONSENT',
    'Explicit consent for memory storage to improve care quality',
    'Maintain context and personalization across conversations',
    '["idosos"]',
    '2 years',
    '["encryption_at_rest", "encryption_in_transit", "access_control", "audit_logging", "vector_anonymization"]'
)
ON CONFLICT DO NOTHING;

-- =====================================================
-- Cleanup function for expired audit logs
-- =====================================================

CREATE OR REPLACE FUNCTION cleanup_expired_audit_logs()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM lgpd_audit_log
    WHERE expires_at < NOW();

    GET DIAGNOSTICS deleted_count = ROW_COUNT;

    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Schedule daily cleanup (requires pg_cron extension)
-- SELECT cron.schedule('0 3 * * *', 'SELECT cleanup_expired_audit_logs()');

COMMENT ON TABLE lgpd_audit_log IS 'LGPD compliance audit trail for all data operations';
COMMENT ON TABLE lgpd_consents IS 'Consent records as required by Art. 8 LGPD';
COMMENT ON TABLE lgpd_requests IS 'Data subject requests as per Art. 18 LGPD';
COMMENT ON TABLE lgpd_processing_records IS 'Processing activity records as per Art. 37 LGPD';
