-- =====================================================
-- MIGRATION 016: DYNAMIC TOOLS SYSTEM
-- =====================================================
-- Sistema de ferramentas dinâmicas para EVA
-- Permite:
--   1. Registro dinâmico de ferramentas no banco
--   2. Descoberta automática de capacidades
--   3. Versionamento e auditoria de ferramentas
--   4. Categorização e organização
-- =====================================================

-- =====================================================
-- 1. TABELA PRINCIPAL: available_tools
-- Armazena definições de ferramentas disponíveis
-- =====================================================

CREATE TABLE IF NOT EXISTS available_tools (
    id SERIAL PRIMARY KEY,

    -- Identificação
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(200),
    description TEXT NOT NULL,

    -- Categorização
    category VARCHAR(50) NOT NULL DEFAULT 'general',
    -- Categories: health, assessment, communication, calendar, vision, voice, system

    subcategory VARCHAR(50),
    tags JSONB DEFAULT '[]',

    -- Definição de parâmetros (formato Gemini Function Calling)
    parameters JSONB NOT NULL DEFAULT '{}',
    -- Estrutura: {
    --   "type": "OBJECT",
    --   "properties": { "param_name": { "type": "STRING", "description": "..." } },
    --   "required": ["param_name"]
    -- }

    -- Controle de acesso
    enabled BOOLEAN DEFAULT true,
    requires_permission BOOLEAN DEFAULT false,
    permission_level VARCHAR(50) DEFAULT 'user',
    -- Levels: user, caregiver, professional, admin, creator

    -- Metadata de execução
    handler_type VARCHAR(50) NOT NULL DEFAULT 'internal',
    -- Types: internal (Go code), webhook, mcp, plugin

    handler_config JSONB DEFAULT '{}',
    -- Para webhook: { "url": "...", "method": "POST", "headers": {} }
    -- Para MCP: { "server": "...", "tool_name": "..." }

    -- Limites e rate limiting
    rate_limit_per_minute INTEGER DEFAULT 60,
    rate_limit_per_hour INTEGER DEFAULT 500,
    timeout_seconds INTEGER DEFAULT 30,

    -- Contexto de uso
    usage_hint TEXT,
    -- Dica para o LLM sobre quando usar esta ferramenta

    example_prompts JSONB DEFAULT '[]',
    -- Exemplos de frases que devem acionar esta ferramenta

    -- Dependências
    depends_on JSONB DEFAULT '[]',
    -- Lista de outras ferramentas que devem estar disponíveis

    conflicts_with JSONB DEFAULT '[]',
    -- Ferramentas que não devem ser usadas junto

    -- Criticidade e prioridade
    priority INTEGER DEFAULT 50,
    -- 0-100, ferramentas críticas têm prioridade maior

    is_critical BOOLEAN DEFAULT false,
    -- Se true, requer confirmação especial ou logging

    -- Versionamento
    version VARCHAR(20) DEFAULT '1.0.0',
    deprecated BOOLEAN DEFAULT false,
    deprecated_message TEXT,
    replacement_tool VARCHAR(100),

    -- Auditoria
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(100) DEFAULT 'system',

    -- Estatísticas de uso
    total_invocations BIGINT DEFAULT 0,
    successful_invocations BIGINT DEFAULT 0,
    failed_invocations BIGINT DEFAULT 0,
    last_invoked_at TIMESTAMP,
    avg_execution_time_ms INTEGER DEFAULT 0
);

-- Índices para performance
CREATE INDEX IF NOT EXISTS idx_tools_category ON available_tools(category);
CREATE INDEX IF NOT EXISTS idx_tools_enabled ON available_tools(enabled);
CREATE INDEX IF NOT EXISTS idx_tools_name ON available_tools(name);
CREATE INDEX IF NOT EXISTS idx_tools_priority ON available_tools(priority DESC);
CREATE INDEX IF NOT EXISTS idx_tools_tags ON available_tools USING GIN(tags);

-- =====================================================
-- 2. TABELA DE HISTÓRICO: tool_invocation_log
-- Auditoria de todas as invocações de ferramentas
-- =====================================================

CREATE TABLE IF NOT EXISTS tool_invocation_log (
    id SERIAL PRIMARY KEY,

    -- Referências
    tool_id INTEGER REFERENCES available_tools(id),
    tool_name VARCHAR(100) NOT NULL,
    idoso_id INTEGER REFERENCES idosos(id),
    session_id VARCHAR(255),

    -- Contexto da invocação
    invoked_at TIMESTAMP DEFAULT NOW(),
    invoked_by VARCHAR(100), -- gemini, user, system, scheduled

    -- Parâmetros e resultado
    input_parameters JSONB,
    output_result JSONB,

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- Status: pending, running, success, failed, timeout, cancelled

    error_message TEXT,
    error_code VARCHAR(50),

    -- Performance
    execution_time_ms INTEGER,

    -- Contexto adicional
    trigger_phrase TEXT,
    -- Frase do usuário que acionou a ferramenta

    conversation_context JSONB,
    -- Últimas mensagens relevantes

    -- Metadados
    metadata JSONB DEFAULT '{}'
);

-- Índices para queries frequentes
CREATE INDEX IF NOT EXISTS idx_invocation_tool ON tool_invocation_log(tool_id);
CREATE INDEX IF NOT EXISTS idx_invocation_idoso ON tool_invocation_log(idoso_id);
CREATE INDEX IF NOT EXISTS idx_invocation_time ON tool_invocation_log(invoked_at DESC);
CREATE INDEX IF NOT EXISTS idx_invocation_status ON tool_invocation_log(status);

-- =====================================================
-- 3. TABELA DE PERMISSÕES: tool_permissions
-- Controle granular de acesso por usuário/grupo
-- =====================================================

CREATE TABLE IF NOT EXISTS tool_permissions (
    id SERIAL PRIMARY KEY,

    -- Quem
    entity_type VARCHAR(20) NOT NULL,
    -- Types: user, caregiver, professional, group, all

    entity_id INTEGER,
    -- ID do idoso, cuidador, ou profissional (NULL para 'all')

    -- Qual ferramenta
    tool_id INTEGER REFERENCES available_tools(id),
    tool_name VARCHAR(100),
    -- Se tool_id for NULL, aplica-se por nome ou categoria

    tool_category VARCHAR(50),
    -- Se preenchido, aplica-se a toda categoria

    -- Permissão
    permission VARCHAR(20) NOT NULL DEFAULT 'allow',
    -- Permissions: allow, deny, require_confirmation

    -- Limites específicos
    custom_rate_limit INTEGER,

    -- Período de validade
    valid_from TIMESTAMP DEFAULT NOW(),
    valid_until TIMESTAMP,

    -- Auditoria
    created_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(100),
    reason TEXT
);

CREATE INDEX IF NOT EXISTS idx_permission_entity ON tool_permissions(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_permission_tool ON tool_permissions(tool_id);

-- =====================================================
-- 4. TABELA DE CAPACIDADES: eva_capabilities
-- Metaconhecimento sobre o que EVA pode fazer
-- =====================================================

CREATE TABLE IF NOT EXISTS eva_capabilities (
    id SERIAL PRIMARY KEY,

    -- Identificação
    capability_name VARCHAR(100) NOT NULL UNIQUE,
    capability_type VARCHAR(50) NOT NULL,
    -- Types: tool, knowledge, skill, integration

    -- Descrição para o LLM
    description TEXT NOT NULL,
    short_description VARCHAR(500),

    -- Mapeamento para ferramentas
    related_tools JSONB DEFAULT '[]',
    -- Lista de tool_names que implementam esta capacidade

    -- Contexto de uso
    when_to_use TEXT,
    when_not_to_use TEXT,

    -- Exemplos
    example_queries JSONB DEFAULT '[]',

    -- Status
    enabled BOOLEAN DEFAULT true,

    -- Prioridade no prompt
    prompt_priority INTEGER DEFAULT 50,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- =====================================================
-- 5. FUNÇÃO: Atualizar timestamp de updated_at
-- =====================================================

CREATE OR REPLACE FUNCTION update_tools_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers para auto-update
DROP TRIGGER IF EXISTS trigger_tools_updated ON available_tools;
CREATE TRIGGER trigger_tools_updated
    BEFORE UPDATE ON available_tools
    FOR EACH ROW
    EXECUTE FUNCTION update_tools_timestamp();

DROP TRIGGER IF EXISTS trigger_capabilities_updated ON eva_capabilities;
CREATE TRIGGER trigger_capabilities_updated
    BEFORE UPDATE ON eva_capabilities
    FOR EACH ROW
    EXECUTE FUNCTION update_tools_timestamp();

-- =====================================================
-- 6. FUNÇÃO: Incrementar contadores de uso
-- =====================================================

CREATE OR REPLACE FUNCTION increment_tool_usage(
    p_tool_name VARCHAR(100),
    p_success BOOLEAN,
    p_execution_time_ms INTEGER
)
RETURNS VOID AS $$
BEGIN
    UPDATE available_tools
    SET
        total_invocations = total_invocations + 1,
        successful_invocations = CASE WHEN p_success THEN successful_invocations + 1 ELSE successful_invocations END,
        failed_invocations = CASE WHEN NOT p_success THEN failed_invocations + 1 ELSE failed_invocations END,
        last_invoked_at = NOW(),
        avg_execution_time_ms = (avg_execution_time_ms * total_invocations + p_execution_time_ms) / (total_invocations + 1)
    WHERE name = p_tool_name;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 7. VIEW: Ferramentas ativas com estatísticas
-- =====================================================

CREATE OR REPLACE VIEW v_active_tools AS
SELECT
    t.id,
    t.name,
    t.display_name,
    t.description,
    t.category,
    t.parameters,
    t.priority,
    t.is_critical,
    t.usage_hint,
    t.total_invocations,
    t.successful_invocations,
    CASE
        WHEN t.total_invocations > 0
        THEN ROUND(t.successful_invocations::numeric / t.total_invocations * 100, 2)
        ELSE 100
    END as success_rate,
    t.avg_execution_time_ms,
    t.last_invoked_at
FROM available_tools t
WHERE t.enabled = true
  AND t.deprecated = false
ORDER BY t.priority DESC, t.category, t.name;

-- =====================================================
-- 8. VIEW: Resumo de capacidades para prompt
-- =====================================================

CREATE OR REPLACE VIEW v_capability_summary AS
SELECT
    c.capability_name,
    c.capability_type,
    c.short_description,
    c.when_to_use,
    ARRAY_AGG(t.name) FILTER (WHERE t.enabled = true) as active_tools
FROM eva_capabilities c
LEFT JOIN LATERAL (
    SELECT name, enabled
    FROM available_tools
    WHERE name = ANY(
        SELECT jsonb_array_elements_text(c.related_tools)
    )
) t ON true
WHERE c.enabled = true
GROUP BY c.id
ORDER BY c.prompt_priority DESC;

-- =====================================================
-- COMENTÁRIOS DE DOCUMENTAÇÃO
-- =====================================================

COMMENT ON TABLE available_tools IS 'Registro dinâmico de todas as ferramentas disponíveis para EVA';
COMMENT ON TABLE tool_invocation_log IS 'Log de auditoria de todas as invocações de ferramentas';
COMMENT ON TABLE tool_permissions IS 'Permissões granulares de acesso às ferramentas';
COMMENT ON TABLE eva_capabilities IS 'Metaconhecimento sobre capacidades da EVA para prompts';

COMMENT ON COLUMN available_tools.handler_type IS 'Tipo de handler: internal (Go), webhook, mcp, plugin';
COMMENT ON COLUMN available_tools.parameters IS 'Schema JSON no formato Gemini Function Calling';
COMMENT ON COLUMN available_tools.is_critical IS 'Ferramentas críticas requerem logging especial (ex: C-SSRS)';
