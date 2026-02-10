-- ============================================================================
-- SPRINT 7: INTEGRATION LAYER
-- ============================================================================
-- Descrição: APIs REST, Webhooks, HL7 FHIR, OAuth2, Rate Limiting
-- Autor: EVA-Mind Development Team
-- Data: 2026-01-24
-- ============================================================================

-- ============================================================================
-- 1. API CLIENTS (APLICAÇÕES EXTERNAS)
-- ============================================================================
-- Registra aplicações que podem acessar a API
CREATE TABLE IF NOT EXISTS api_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_name VARCHAR(200) NOT NULL,
    client_type VARCHAR(50) CHECK (client_type IN (
        'web_app',
        'mobile_app',
        'hospital_system',
        'research_platform',
        'ehr_system',
        'third_party'
    )),

    -- Credenciais OAuth2
    client_id VARCHAR(100) NOT NULL UNIQUE,
    client_secret_hash VARCHAR(256) NOT NULL, -- bcrypt hash

    -- Permissões
    scopes TEXT[] NOT NULL, -- ['read:patients', 'write:assessments', 'read:research']
    allowed_endpoints TEXT[], -- Lista de endpoints permitidos

    -- Rate limiting
    rate_limit_per_minute INTEGER DEFAULT 60,
    rate_limit_per_hour INTEGER DEFAULT 1000,
    rate_limit_per_day INTEGER DEFAULT 10000,

    -- IP whitelisting (opcional)
    allowed_ips INET[],

    -- Webhook callback
    webhook_url VARCHAR(500),
    webhook_secret VARCHAR(256), -- Para verificar assinatura
    webhook_events TEXT[], -- Eventos que devem ser notificados

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    is_approved BOOLEAN DEFAULT FALSE, -- Requer aprovação manual
    approved_by VARCHAR(200),
    approved_at TIMESTAMP,

    -- Metadados
    contact_email VARCHAR(200),
    contact_name VARCHAR(200),
    organization VARCHAR(200),
    description TEXT,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_clients_client_id ON api_clients(client_id);
CREATE INDEX IF NOT EXISTS idx_api_clients_active ON api_clients(is_active, is_approved) WHERE is_active = TRUE AND is_approved = TRUE;

COMMENT ON TABLE api_clients IS 'Aplicações externas autorizadas a usar a API';


-- ============================================================================
-- 2. API TOKENS (ACCESS TOKENS OAUTH2)
-- ============================================================================
-- Tokens de acesso temporários gerados via OAuth2
CREATE TABLE IF NOT EXISTS api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES api_clients(id) ON DELETE CASCADE,

    -- Token
    access_token VARCHAR(256) NOT NULL UNIQUE, -- JWT ou random string
    refresh_token VARCHAR(256) UNIQUE,
    token_type VARCHAR(50) DEFAULT 'Bearer',

    -- Escopo
    scopes TEXT[] NOT NULL,

    -- Expiração
    expires_at TIMESTAMP NOT NULL,
    refresh_expires_at TIMESTAMP,

    -- Associado a usuário/paciente (opcional)
    user_id INTEGER, -- Se token está em nome de um usuário específico
    patient_id INTEGER REFERENCES idosos(id) ON DELETE CASCADE,

    -- Revogação
    is_revoked BOOLEAN DEFAULT FALSE,
    revoked_at TIMESTAMP,
    revoked_reason TEXT,

    -- Uso
    last_used_at TIMESTAMP,
    usage_count INTEGER DEFAULT 0,

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_access_token ON api_tokens(access_token) WHERE is_revoked = FALSE;
CREATE INDEX IF NOT EXISTS idx_api_tokens_client ON api_tokens(client_id);
CREATE INDEX IF NOT EXISTS idx_api_tokens_expires ON api_tokens(expires_at) WHERE is_revoked = FALSE;

COMMENT ON TABLE api_tokens IS 'Tokens de acesso OAuth2 temporários';


-- ============================================================================
-- 3. API REQUEST LOGS (AUDITORIA DE CHAMADAS)
-- ============================================================================
-- Registra todas as chamadas à API para auditoria
CREATE TABLE IF NOT EXISTS api_request_logs (
    id BIGSERIAL PRIMARY KEY,
    request_id UUID DEFAULT gen_random_uuid() UNIQUE,

    -- Identificação
    client_id UUID REFERENCES api_clients(id) ON DELETE SET NULL,
    token_id UUID REFERENCES api_tokens(id) ON DELETE SET NULL,

    -- Request
    http_method VARCHAR(10) NOT NULL, -- GET, POST, PUT, DELETE
    endpoint VARCHAR(500) NOT NULL,
    query_params JSONB,
    request_body JSONB, -- Apenas se não contiver dados sensíveis

    -- Headers importantes
    user_agent TEXT,
    ip_address INET,

    -- Response
    http_status_code INTEGER,
    response_time_ms INTEGER, -- Tempo de resposta em ms
    response_size_bytes INTEGER,
    error_message TEXT,

    -- Rate limiting
    rate_limit_hit BOOLEAN DEFAULT FALSE,

    -- Contexto
    patient_id INTEGER REFERENCES idosos(id) ON DELETE SET NULL,
    resource_type VARCHAR(100), -- 'patient', 'assessment', 'research_study'
    resource_id VARCHAR(100),

    timestamp TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Partitioning por mês (opcional, para performance)
CREATE INDEX IF NOT EXISTS idx_api_logs_timestamp ON api_request_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_api_logs_client ON api_request_logs(client_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_api_logs_endpoint ON api_request_logs(endpoint, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_api_logs_status ON api_request_logs(http_status_code, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_api_logs_errors ON api_request_logs(timestamp DESC) WHERE http_status_code >= 400;

COMMENT ON TABLE api_request_logs IS 'Log de todas as requisições à API para auditoria';


-- ============================================================================
-- 4. WEBHOOKS (EVENTOS ASSÍNCRONOS)
-- ============================================================================
-- Fila de webhooks a serem enviados
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES api_clients(id) ON DELETE CASCADE,

    -- Evento
    event_type VARCHAR(100) NOT NULL, -- 'patient.created', 'assessment.completed', 'crisis.detected'
    event_data JSONB NOT NULL,

    -- Delivery
    webhook_url VARCHAR(500) NOT NULL,
    http_method VARCHAR(10) DEFAULT 'POST',

    -- Status
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN (
        'pending',
        'sent',
        'failed',
        'cancelled'
    )),
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,

    -- Resultado
    last_attempt_at TIMESTAMP,
    last_http_status INTEGER,
    last_error_message TEXT,
    delivered_at TIMESTAMP,

    -- Segurança
    signature VARCHAR(256), -- HMAC-SHA256 para verificação

    -- Retry
    next_retry_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhook_deliveries(status, next_retry_at);
CREATE INDEX IF NOT EXISTS idx_webhooks_client ON webhook_deliveries(client_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhooks_event_type ON webhook_deliveries(event_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhooks_pending ON webhook_deliveries(next_retry_at) WHERE status = 'pending';

COMMENT ON TABLE webhook_deliveries IS 'Fila de webhooks para notificação assíncrona de eventos';


-- ============================================================================
-- 5. RATE LIMIT TRACKING
-- ============================================================================
-- Rastreia uso de API por cliente para rate limiting
CREATE TABLE IF NOT EXISTS rate_limit_tracking (
    id BIGSERIAL PRIMARY KEY,
    client_id UUID NOT NULL REFERENCES api_clients(id) ON DELETE CASCADE,

    -- Janela de tempo
    window_type VARCHAR(20) NOT NULL CHECK (window_type IN ('minute', 'hour', 'day')),
    window_start TIMESTAMP NOT NULL,

    -- Contagem
    request_count INTEGER NOT NULL DEFAULT 1,

    -- Cache
    last_request_at TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(client_id, window_type, window_start)
);

CREATE INDEX IF NOT EXISTS idx_rate_limit_client_window ON rate_limit_tracking(client_id, window_type, window_start);

COMMENT ON TABLE rate_limit_tracking IS 'Tracking de requisições para rate limiting';


-- ============================================================================
-- 6. FHIR MAPPINGS (HL7 FHIR)
-- ============================================================================
-- Mapeamento de recursos EVA para recursos FHIR
CREATE TABLE IF NOT EXISTS fhir_resource_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Recurso EVA
    eva_resource_type VARCHAR(100) NOT NULL, -- 'patient', 'assessment', 'observation'
    eva_resource_id VARCHAR(100) NOT NULL,

    -- Recurso FHIR
    fhir_resource_type VARCHAR(100) NOT NULL, -- 'Patient', 'Observation', 'QuestionnaireResponse'
    fhir_resource_id VARCHAR(100) NOT NULL,
    fhir_version VARCHAR(20) DEFAULT 'R4', -- FHIR R4, R5

    -- Dados FHIR completos
    fhir_resource JSONB NOT NULL,

    -- Sincronização
    last_synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sync_status VARCHAR(50) DEFAULT 'synced' CHECK (sync_status IN (
        'synced',
        'pending',
        'failed',
        'conflict'
    )),

    -- Sistema externo
    external_system VARCHAR(200), -- 'Hospital XYZ EHR', 'Lab System'
    external_endpoint VARCHAR(500),

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(eva_resource_type, eva_resource_id, external_system)
);

CREATE INDEX IF NOT EXISTS idx_fhir_mappings_eva ON fhir_resource_mappings(eva_resource_type, eva_resource_id);
CREATE INDEX IF NOT EXISTS idx_fhir_mappings_fhir ON fhir_resource_mappings(fhir_resource_type, fhir_resource_id);
CREATE INDEX IF NOT EXISTS idx_fhir_mappings_sync ON fhir_resource_mappings(sync_status) WHERE sync_status != 'synced';

COMMENT ON TABLE fhir_resource_mappings IS 'Mapeamento entre recursos EVA e HL7 FHIR';


-- ============================================================================
-- 7. EXTERNAL SYSTEM CREDENTIALS
-- ============================================================================
-- Credenciais para conectar com sistemas externos (EHR, lab systems)
CREATE TABLE IF NOT EXISTS external_system_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    system_name VARCHAR(200) NOT NULL UNIQUE,
    system_type VARCHAR(100), -- 'ehr', 'lab', 'pharmacy', 'hospital_system'

    -- Endpoint
    base_url VARCHAR(500) NOT NULL,
    api_version VARCHAR(50),

    -- Autenticação
    auth_type VARCHAR(50) CHECK (auth_type IN (
        'basic',
        'bearer_token',
        'oauth2',
        'api_key',
        'mutual_tls',
        'saml'
    )),

    -- Credenciais (criptografadas)
    credentials_encrypted BYTEA NOT NULL, -- JSON criptografado

    -- FHIR específico
    fhir_version VARCHAR(20),
    supported_resources TEXT[], -- ['Patient', 'Observation', 'Condition']

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    last_connection_test TIMESTAMP,
    connection_status VARCHAR(50) DEFAULT 'untested',

    -- Metadados
    contact_person VARCHAR(200),
    contact_email VARCHAR(200),
    notes TEXT,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_external_systems_active ON external_system_credentials(is_active) WHERE is_active = TRUE;

COMMENT ON TABLE external_system_credentials IS 'Credenciais para sistemas externos (EHR, labs, hospitais)';


-- ============================================================================
-- 8. DATA EXPORT JOBS
-- ============================================================================
-- Jobs de exportação de dados (LGPD, pesquisa, backup)
CREATE TABLE IF NOT EXISTS data_export_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Solicitante
    requested_by VARCHAR(200) NOT NULL,
    patient_id INTEGER REFERENCES idosos(id) ON DELETE CASCADE, -- Se export de paciente específico

    -- Tipo de export
    export_type VARCHAR(50) CHECK (export_type IN (
        'lgpd_portability', -- Direito à portabilidade (LGPD)
        'research_dataset',
        'backup',
        'fhir_bundle',
        'csv_export',
        'clinical_summary'
    )),

    -- Configuração
    export_config JSONB NOT NULL,
    -- Exemplo: {
    --   "resources": ["patients", "assessments", "medications"],
    --   "date_range": {"start": "2024-01-01", "end": "2024-12-31"},
    --   "format": "json",
    --   "anonymize": true
    -- }

    -- Status
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN (
        'pending',
        'processing',
        'completed',
        'failed',
        'cancelled'
    )),

    -- Resultado
    file_path VARCHAR(500),
    file_size_bytes BIGINT,
    file_format VARCHAR(50), -- 'json', 'csv', 'xml', 'fhir'
    download_url VARCHAR(500),
    download_expires_at TIMESTAMP,

    -- Progresso
    progress_percentage INTEGER DEFAULT 0,
    records_processed INTEGER DEFAULT 0,
    total_records INTEGER,
    error_message TEXT,

    -- Timing
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_export_jobs_status ON data_export_jobs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_export_jobs_patient ON data_export_jobs(patient_id) WHERE patient_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_export_jobs_pending ON data_export_jobs(created_at) WHERE status = 'pending';

COMMENT ON TABLE data_export_jobs IS 'Jobs de exportação de dados (LGPD, pesquisa, FHIR)';


-- ============================================================================
-- TRIGGERS E FUNÇÕES
-- ============================================================================

-- Função para atualizar updated_at automaticamente
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_api_clients_updated_at
    BEFORE UPDATE ON api_clients
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_external_systems_updated_at
    BEFORE UPDATE ON external_system_credentials
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();


-- Função para incrementar usage_count de tokens
CREATE OR REPLACE FUNCTION increment_token_usage()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE api_tokens
    SET usage_count = usage_count + 1,
        last_used_at = NOW()
    WHERE id = NEW.token_id;

    UPDATE api_clients
    SET last_used_at = NOW()
    WHERE id = NEW.client_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_increment_token_usage
    AFTER INSERT ON api_request_logs
    FOR EACH ROW
    WHEN (NEW.token_id IS NOT NULL)
    EXECUTE FUNCTION increment_token_usage();


-- Função para limpar logs antigos (manter últimos 90 dias)
CREATE OR REPLACE FUNCTION cleanup_old_api_logs()
RETURNS void AS $$
BEGIN
    DELETE FROM api_request_logs
    WHERE timestamp < NOW() - INTERVAL '90 days';

    DELETE FROM rate_limit_tracking
    WHERE window_start < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

-- Job agendado (executar via cron ou pg_cron)
-- SELECT cron.schedule('cleanup-api-logs', '0 2 * * *', 'SELECT cleanup_old_api_logs()');


-- ============================================================================
-- VIEWS
-- ============================================================================

-- View: Estatísticas de uso da API por cliente
CREATE OR REPLACE VIEW v_api_usage_stats AS
SELECT
    ac.id AS client_id,
    ac.client_name,
    ac.client_type,
    ac.organization,

    -- Requests
    COUNT(arl.id) AS total_requests,
    COUNT(arl.id) FILTER (WHERE arl.http_status_code < 400) AS successful_requests,
    COUNT(arl.id) FILTER (WHERE arl.http_status_code >= 400) AS failed_requests,

    -- Performance
    AVG(arl.response_time_ms) AS avg_response_time_ms,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY arl.response_time_ms) AS p95_response_time_ms,

    -- Rate limiting
    COUNT(arl.id) FILTER (WHERE arl.rate_limit_hit = TRUE) AS rate_limit_hits,

    -- Timing
    MAX(arl.timestamp) AS last_request_at,
    MIN(arl.timestamp) AS first_request_at,

    -- Tokens ativos
    COUNT(DISTINCT at.id) FILTER (WHERE at.is_revoked = FALSE AND at.expires_at > NOW()) AS active_tokens

FROM api_clients ac
LEFT JOIN api_request_logs arl ON arl.client_id = ac.id
LEFT JOIN api_tokens at ON at.client_id = ac.id
WHERE ac.is_active = TRUE
GROUP BY ac.id, ac.client_name, ac.client_type, ac.organization;

COMMENT ON VIEW v_api_usage_stats IS 'Estatísticas de uso da API por cliente';


-- View: Webhooks pendentes de retry
CREATE OR REPLACE VIEW v_pending_webhooks AS
SELECT
    wd.id,
    wd.client_id,
    ac.client_name,
    wd.event_type,
    wd.webhook_url,
    wd.attempts,
    wd.max_attempts,
    wd.last_error_message,
    wd.next_retry_at,
    wd.created_at
FROM webhook_deliveries wd
JOIN api_clients ac ON ac.id = wd.client_id
WHERE wd.status = 'pending'
  AND wd.attempts < wd.max_attempts
  AND wd.next_retry_at <= NOW()
ORDER BY wd.created_at ASC;

COMMENT ON VIEW v_pending_webhooks IS 'Webhooks pendentes prontos para retry';


-- View: Recursos FHIR com problemas de sincronização
CREATE OR REPLACE VIEW v_fhir_sync_issues AS
SELECT
    frm.id,
    frm.eva_resource_type,
    frm.eva_resource_id,
    frm.fhir_resource_type,
    frm.external_system,
    frm.sync_status,
    frm.last_synced_at,
    EXTRACT(EPOCH FROM (NOW() - frm.last_synced_at)) / 3600 AS hours_since_sync
FROM fhir_resource_mappings frm
WHERE frm.sync_status IN ('pending', 'failed', 'conflict')
   OR frm.last_synced_at < NOW() - INTERVAL '24 hours'
ORDER BY frm.last_synced_at ASC;

COMMENT ON VIEW v_fhir_sync_issues IS 'Recursos FHIR com problemas de sincronização';


-- View: Endpoints mais usados
CREATE OR REPLACE VIEW v_top_api_endpoints AS
SELECT
    endpoint,
    COUNT(*) AS request_count,
    AVG(response_time_ms) AS avg_response_time,
    COUNT(*) FILTER (WHERE http_status_code >= 400) AS error_count,
    MAX(timestamp) AS last_used
FROM api_request_logs
WHERE timestamp > NOW() - INTERVAL '7 days'
GROUP BY endpoint
ORDER BY request_count DESC
LIMIT 50;

COMMENT ON VIEW v_top_api_endpoints IS 'Endpoints mais usados nos últimos 7 dias';


-- ============================================================================
-- ÍNDICES ADICIONAIS PARA PERFORMANCE
-- ============================================================================

-- Composite index para queries comuns de audit
CREATE INDEX IF NOT EXISTS idx_api_logs_client_status_time
    ON api_request_logs(client_id, http_status_code, timestamp DESC);

-- Index para busca de webhooks por evento
CREATE INDEX IF NOT EXISTS idx_webhooks_event_client
    ON webhook_deliveries(event_type, client_id, created_at DESC);

-- Index para exports em progresso
CREATE INDEX IF NOT EXISTS idx_export_jobs_processing
    ON data_export_jobs(status, started_at)
    WHERE status IN ('pending', 'processing');


-- ============================================================================
-- ✅ SPRINT 7: INTEGRATION LAYER - SCHEMA COMPLETO
-- ============================================================================

DO $$
BEGIN
    RAISE NOTICE '✅ Sprint 7 (Integration Layer) - Schema criado com sucesso';
    RAISE NOTICE '   Tabelas:';
    RAISE NOTICE '   - api_clients (aplicações autorizadas)';
    RAISE NOTICE '   - api_tokens (OAuth2 tokens)';
    RAISE NOTICE '   - api_request_logs (auditoria completa)';
    RAISE NOTICE '   - webhook_deliveries (eventos assíncronos)';
    RAISE NOTICE '   - rate_limit_tracking (proteção contra abuso)';
    RAISE NOTICE '   - fhir_resource_mappings (integração HL7 FHIR)';
    RAISE NOTICE '   - external_system_credentials (EHR, labs)';
    RAISE NOTICE '   - data_export_jobs (LGPD, pesquisa)';
    RAISE NOTICE '   ';
    RAISE NOTICE '   Views:';
    RAISE NOTICE '   - v_api_usage_stats';
    RAISE NOTICE '   - v_pending_webhooks';
    RAISE NOTICE '   - v_fhir_sync_issues';
    RAISE NOTICE '   - v_top_api_endpoints';
    RAISE NOTICE '   ';
    RAISE NOTICE '   Triggers:';
    RAISE NOTICE '   - Auto-update updated_at';
    RAISE NOTICE '   - Increment token usage counter';
END $$;
