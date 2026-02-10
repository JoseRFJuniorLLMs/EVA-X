-- =====================================================
-- MIGRATION 017: EVA SELF-KNOWLEDGE SYSTEM
-- =====================================================
-- Sistema de autoconhecimento da EVA sobre seu próprio código
-- Permite que EVA "saiba" como ela funciona internamente
-- Vinculado ao criador: Jose R F Junior (CPF: 64525430249)
-- =====================================================

-- =====================================================
-- 1. TABELA: eva_self_knowledge
-- Conhecimento estruturado sobre o projeto
-- =====================================================

CREATE TABLE IF NOT EXISTS eva_self_knowledge (
    id SERIAL PRIMARY KEY,

    -- Classificação
    knowledge_type VARCHAR(50) NOT NULL,
    -- Types: architecture, module, service, concept, integration, theory, process, config

    knowledge_key VARCHAR(200) NOT NULL UNIQUE,
    -- Ex: "module:cortex:lacan", "concept:transference", "service:gemini"

    title VARCHAR(300) NOT NULL,

    -- Conteúdo
    summary TEXT NOT NULL,
    -- Resumo curto (1-3 frases)

    detailed_content TEXT NOT NULL,
    -- Explicação completa

    code_location TEXT,
    -- Caminho no código: "internal/cortex/lacan/unified_retrieval.go"

    -- Relacionamentos (para consultas rápidas)
    parent_key VARCHAR(200),
    -- Chave do pai na hierarquia

    related_keys JSONB DEFAULT '[]',
    -- Chaves relacionadas

    tags JSONB DEFAULT '[]',

    -- Metadados
    importance INTEGER DEFAULT 50,
    -- 0-100, conceitos mais importantes têm valor maior

    created_by VARCHAR(100) DEFAULT 'system',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Índices
CREATE INDEX IF NOT EXISTS idx_knowledge_type ON eva_self_knowledge(knowledge_type);
CREATE INDEX IF NOT EXISTS idx_knowledge_key ON eva_self_knowledge(knowledge_key);
CREATE INDEX IF NOT EXISTS idx_knowledge_parent ON eva_self_knowledge(parent_key);
CREATE INDEX IF NOT EXISTS idx_knowledge_tags ON eva_self_knowledge USING GIN(tags);

-- Trigger para updated_at
CREATE OR REPLACE FUNCTION update_knowledge_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_knowledge_updated ON eva_self_knowledge;
CREATE TRIGGER trigger_knowledge_updated
    BEFORE UPDATE ON eva_self_knowledge
    FOR EACH ROW
    EXECUTE FUNCTION update_knowledge_timestamp();

-- =====================================================
-- 2. TABELA: creator_knowledge_access
-- Registro de conhecimento especial do criador
-- =====================================================

CREATE TABLE IF NOT EXISTS creator_knowledge_access (
    id SERIAL PRIMARY KEY,
    creator_cpf VARCHAR(20) NOT NULL DEFAULT '64525430249',
    creator_name VARCHAR(100) NOT NULL DEFAULT 'Jose R F Junior',

    -- Acesso especial
    has_full_debug BOOLEAN DEFAULT true,
    has_architecture_access BOOLEAN DEFAULT true,
    has_code_access BOOLEAN DEFAULT true,

    -- Última interação sobre arquitetura
    last_architecture_query TIMESTAMP,
    total_architecture_queries INTEGER DEFAULT 0,

    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir registro do criador
INSERT INTO creator_knowledge_access (creator_cpf, creator_name)
VALUES ('64525430249', 'Jose R F Junior')
ON CONFLICT DO NOTHING;

-- =====================================================
-- 3. VIEW: Conhecimento organizado por módulo
-- =====================================================

CREATE OR REPLACE VIEW v_knowledge_by_module AS
SELECT
    knowledge_type,
    knowledge_key,
    title,
    summary,
    code_location,
    importance
FROM eva_self_knowledge
WHERE knowledge_type IN ('module', 'service', 'concept')
ORDER BY importance DESC, knowledge_type, title;

-- =====================================================
-- 4. FUNÇÃO: Buscar conhecimento relacionado
-- =====================================================

CREATE OR REPLACE FUNCTION get_related_knowledge(p_key VARCHAR(200))
RETURNS TABLE (
    key VARCHAR(200),
    title VARCHAR(300),
    summary TEXT,
    relation_type VARCHAR(50)
) AS $$
BEGIN
    RETURN QUERY
    -- Filhos diretos
    SELECT
        k.knowledge_key,
        k.title,
        k.summary,
        'child'::VARCHAR(50) as relation_type
    FROM eva_self_knowledge k
    WHERE k.parent_key = p_key

    UNION ALL

    -- Pai
    SELECT
        k.knowledge_key,
        k.title,
        k.summary,
        'parent'::VARCHAR(50)
    FROM eva_self_knowledge k
    WHERE k.knowledge_key = (
        SELECT parent_key FROM eva_self_knowledge WHERE knowledge_key = p_key
    )

    UNION ALL

    -- Relacionados
    SELECT
        k.knowledge_key,
        k.title,
        k.summary,
        'related'::VARCHAR(50)
    FROM eva_self_knowledge k
    WHERE k.knowledge_key IN (
        SELECT jsonb_array_elements_text(related_keys)
        FROM eva_self_knowledge
        WHERE knowledge_key = p_key
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON TABLE eva_self_knowledge IS 'Conhecimento da EVA sobre seu próprio código e arquitetura';
COMMENT ON TABLE creator_knowledge_access IS 'Acesso especial do criador ao conhecimento interno';
