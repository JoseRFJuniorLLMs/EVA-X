-- ============================================================================
-- Migration 018: Adicionar campo estilo_conversa na tabela idosos
-- Permite configurar o comportamento da EVA por usuário
-- ============================================================================

-- 1. Adicionar coluna estilo_conversa
ALTER TABLE idosos
ADD COLUMN IF NOT EXISTS estilo_conversa VARCHAR(20) DEFAULT 'hibrido';

-- 2. Adicionar coluna persona_preferida
ALTER TABLE idosos
ADD COLUMN IF NOT EXISTS persona_preferida VARCHAR(20) DEFAULT 'companion';

-- 3. Adicionar coluna profundidade_emocional (0.0 a 1.0)
ALTER TABLE idosos
ADD COLUMN IF NOT EXISTS profundidade_emocional DECIMAL(3,2) DEFAULT 0.70;

-- 4. Criar tipo ENUM para estilo_conversa (opcional, para validação)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'estilo_conversa_type') THEN
        CREATE TYPE estilo_conversa_type AS ENUM ('exploratorio', 'diretivo', 'hibrido');
    END IF;
END $$;

-- 5. Criar tipo ENUM para persona (opcional, para validação)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'persona_type') THEN
        CREATE TYPE persona_type AS ENUM ('companion', 'clinical', 'educator', 'emergency');
    END IF;
END $$;

-- 6. Adicionar comentários nas colunas
COMMENT ON COLUMN idosos.estilo_conversa IS 'Estilo de conversa da EVA: exploratorio (faz perguntas), diretivo (dá respostas), hibrido (adapta ao contexto)';
COMMENT ON COLUMN idosos.persona_preferida IS 'Persona preferida da EVA: companion (amigável), clinical (profissional), educator (pedagógica)';
COMMENT ON COLUMN idosos.profundidade_emocional IS 'Nível de profundidade emocional nas respostas (0.0 = técnica, 1.0 = muito emocional)';

-- ============================================================================
-- 7. Atualizar usuários existentes com valores padrão baseados no perfil
-- ============================================================================

-- Idosos com nível cognitivo baixo: mais diretivo, menos perguntas
UPDATE idosos
SET estilo_conversa = 'diretivo',
    profundidade_emocional = 0.80
WHERE nivel_cognitivo IN ('baixo', 'muito_baixo', 'comprometido')
  AND estilo_conversa = 'hibrido';

-- Idosos com limitações auditivas: mais direto
UPDATE idosos
SET estilo_conversa = 'diretivo'
WHERE limitacoes_auditivas = true
  AND estilo_conversa = 'hibrido';

-- ============================================================================
-- 8. Configuração especial para o Criador (Jose R F Junior)
-- ============================================================================

UPDATE idosos
SET estilo_conversa = 'hibrido',
    persona_preferida = 'companion',
    profundidade_emocional = 0.60
WHERE cpf = '64525430249';

-- ============================================================================
-- 9. Criar índice para consultas rápidas
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_idosos_estilo_conversa ON idosos(estilo_conversa);
CREATE INDEX IF NOT EXISTS idx_idosos_persona_preferida ON idosos(persona_preferida);

-- ============================================================================
-- 10. Criar tabela de configurações de estilo (para referência)
-- ============================================================================

CREATE TABLE IF NOT EXISTS estilos_conversa_config (
    id SERIAL PRIMARY KEY,
    estilo VARCHAR(20) UNIQUE NOT NULL,
    descricao TEXT NOT NULL,
    peso_exploratorio DECIMAL(3,2) NOT NULL, -- 0.0 a 1.0
    peso_diretivo DECIMAL(3,2) NOT NULL,     -- 0.0 a 1.0
    exemplos_comportamento JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir configurações padrão
INSERT INTO estilos_conversa_config (estilo, descricao, peso_exploratorio, peso_diretivo, exemplos_comportamento)
VALUES
(
    'exploratorio',
    'EVA faz muitas perguntas reflexivas, devolve a fala, incentiva elaboração. Estilo psicanalítico.',
    0.80,
    0.20,
    '{
        "saudacao": "Olá! Como você está se sentindo hoje?",
        "resposta_triste": "Você disse que está triste... O que te faz sentir assim?",
        "resposta_dor": "Onde exatamente você sente essa dor? Quando começou?",
        "resposta_medicamento": "Você tomou o remédio? Como se sentiu depois?"
    }'::jsonb
),
(
    'diretivo',
    'EVA dá respostas diretas, instruções claras, menos perguntas. Estilo assistente.',
    0.20,
    0.80,
    '{
        "saudacao": "Olá! Estou aqui para ajudar.",
        "resposta_triste": "Sinto muito que você esteja triste. Vou colocar uma música calma para você.",
        "resposta_dor": "Você deve tomar o analgésico agora. Vou avisar seu cuidador.",
        "resposta_medicamento": "Está na hora do seu remédio. Tome com um copo de água."
    }'::jsonb
),
(
    'hibrido',
    'EVA adapta o estilo conforme o contexto. Perguntas quando apropriado, direto quando necessário.',
    0.50,
    0.50,
    '{
        "saudacao": "Olá! Tudo bem com você?",
        "resposta_triste": "Percebo que você está triste. Quer conversar sobre isso ou prefere uma distração?",
        "resposta_dor": "Você está com dor? Me conta mais. Se for forte, vou avisar seu cuidador.",
        "resposta_medicamento": "Hora do remédio! Tomou? Como está se sentindo hoje?"
    }'::jsonb
)
ON CONFLICT (estilo) DO UPDATE SET
    descricao = EXCLUDED.descricao,
    peso_exploratorio = EXCLUDED.peso_exploratorio,
    peso_diretivo = EXCLUDED.peso_diretivo,
    exemplos_comportamento = EXCLUDED.exemplos_comportamento;

-- ============================================================================
-- 11. View para facilitar consultas
-- ============================================================================

CREATE OR REPLACE VIEW v_idosos_config_completa AS
SELECT
    i.id,
    i.nome,
    i.cpf,
    i.nivel_cognitivo,
    i.estilo_conversa,
    i.persona_preferida,
    i.profundidade_emocional,
    i.tom_voz,
    e.descricao AS estilo_descricao,
    e.peso_exploratorio,
    e.peso_diretivo,
    e.exemplos_comportamento
FROM idosos i
LEFT JOIN estilos_conversa_config e ON i.estilo_conversa = e.estilo
WHERE i.ativo = true;

-- ============================================================================
-- 12. Função para obter configuração de estilo de um idoso
-- ============================================================================

CREATE OR REPLACE FUNCTION get_estilo_conversa(p_idoso_id BIGINT)
RETURNS TABLE (
    estilo VARCHAR(20),
    peso_exploratorio DECIMAL(3,2),
    peso_diretivo DECIMAL(3,2),
    profundidade_emocional DECIMAL(3,2),
    persona VARCHAR(20),
    exemplos JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        i.estilo_conversa,
        COALESCE(e.peso_exploratorio, 0.50),
        COALESCE(e.peso_diretivo, 0.50),
        i.profundidade_emocional,
        i.persona_preferida,
        e.exemplos_comportamento
    FROM idosos i
    LEFT JOIN estilos_conversa_config e ON i.estilo_conversa = e.estilo
    WHERE i.id = p_idoso_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- RESUMO DAS ALTERAÇÕES
-- ============================================================================
--
-- Novas colunas em 'idosos':
--   - estilo_conversa: 'exploratorio', 'diretivo', 'hibrido'
--   - persona_preferida: 'companion', 'clinical', 'educator', 'emergency'
--   - profundidade_emocional: 0.0 a 1.0
--
-- Nova tabela:
--   - estilos_conversa_config: configurações detalhadas de cada estilo
--
-- Nova view:
--   - v_idosos_config_completa: visão completa com joins
--
-- Nova função:
--   - get_estilo_conversa(idoso_id): retorna config de estilo
--
-- ============================================================================

-- Verificar resultado
SELECT
    id,
    nome,
    estilo_conversa,
    persona_preferida,
    profundidade_emocional
FROM idosos
LIMIT 10;
