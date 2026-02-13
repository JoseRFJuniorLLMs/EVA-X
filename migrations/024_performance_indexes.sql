-- ============================================================================
-- PERFORMANCE MIGRATION: Indices para otimizacao de queries
-- Issue: Full table scans em queries frequentes
-- Fix: Criar indices compostos para colunas mais usadas
-- NOTA: Usa verificacao de existencia para evitar erros em tabelas faltantes
-- ============================================================================

-- ============================================================================
-- FUNCAO HELPER: Criar indice apenas se tabela existe
-- ============================================================================
CREATE OR REPLACE FUNCTION create_index_if_table_exists(
    p_index_name TEXT,
    p_table_name TEXT,
    p_columns TEXT,
    p_where_clause TEXT DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = p_table_name) THEN
        IF p_where_clause IS NULL THEN
            EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (%s)',
                p_index_name, p_table_name, p_columns);
        ELSE
            EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (%s) WHERE %s',
                p_index_name, p_table_name, p_columns, p_where_clause);
        END IF;
        RAISE NOTICE 'Index % created on %', p_index_name, p_table_name;
    ELSE
        RAISE NOTICE 'Table % does not exist, skipping index %', p_table_name, p_index_name;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- TABELA: idosos
-- ============================================================================
SELECT create_index_if_table_exists('idx_idosos_cpf', 'idosos', 'cpf');
SELECT create_index_if_table_exists('idx_idosos_telefone', 'idosos', 'telefone');

-- ============================================================================
-- TABELA: memories
-- ============================================================================
SELECT create_index_if_table_exists('idx_memories_idoso_timestamp', 'memories', 'idoso_id, created_at DESC');
SELECT create_index_if_table_exists('idx_memories_type', 'memories', 'memory_type');

-- ============================================================================
-- TABELA: transcriptions
-- ============================================================================
SELECT create_index_if_table_exists('idx_transcriptions_idoso_timestamp', 'transcriptions', 'idoso_id, timestamp DESC');
SELECT create_index_if_table_exists('idx_transcriptions_session', 'transcriptions', 'session_id');

-- ============================================================================
-- TABELA: interaction_cognitive_load
-- ============================================================================
SELECT create_index_if_table_exists('idx_cognitive_load_patient_time', 'interaction_cognitive_load', 'patient_id, timestamp DESC');

-- ============================================================================
-- TABELA: ethical_boundary_events
-- ============================================================================
SELECT create_index_if_table_exists('idx_ethical_events_patient_severity', 'ethical_boundary_events', 'patient_id, severity, timestamp DESC');
SELECT create_index_if_table_exists('idx_ethical_high_severity', 'ethical_boundary_events', 'patient_id, timestamp DESC', 'severity IN (''high'', ''critical'')');

-- ============================================================================
-- TABELA: clinical_decision_explanations
-- ============================================================================
SELECT create_index_if_table_exists('idx_clinical_decisions_patient', 'clinical_decision_explanations', 'patient_id, created_at DESC');
SELECT create_index_if_table_exists('idx_clinical_decisions_type', 'clinical_decision_explanations', 'decision_type');

-- ============================================================================
-- TABELA: trajectory_simulations
-- ============================================================================
SELECT create_index_if_table_exists('idx_trajectory_patient_date', 'trajectory_simulations', 'patient_id, simulation_date DESC');

-- ============================================================================
-- TABELA: persona_sessions
-- ============================================================================
SELECT create_index_if_table_exists('idx_persona_sessions_patient', 'persona_sessions', 'patient_id, started_at DESC');

-- ============================================================================
-- TABELA: exit_protocols
-- ============================================================================
SELECT create_index_if_table_exists('idx_exit_protocols_patient', 'exit_protocols', 'patient_id, current_phase');

-- ============================================================================
-- TABELA: deep_memory_events
-- ============================================================================
SELECT create_index_if_table_exists('idx_deep_memory_patient_type', 'deep_memory_events', 'patient_id, event_type, detected_at DESC');

-- ============================================================================
-- TABELA: device_tokens
-- ============================================================================
SELECT create_index_if_table_exists('idx_device_tokens_idoso', 'device_tokens', 'idoso_id');

-- ============================================================================
-- TABELA: enneagram_results
-- ============================================================================
SELECT create_index_if_table_exists('idx_enneagram_idoso', 'enneagram_results', 'idoso_id');

-- ============================================================================
-- ANALYZE tabelas existentes
-- ============================================================================
DO $$
DECLARE
    tbl TEXT;
    tables TEXT[] := ARRAY['idosos', 'memories', 'transcriptions', 'interaction_cognitive_load',
                           'ethical_boundary_events', 'clinical_decision_explanations',
                           'trajectory_simulations', 'persona_sessions', 'device_tokens'];
BEGIN
    FOREACH tbl IN ARRAY tables LOOP
        IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = tbl) THEN
            EXECUTE format('ANALYZE %I', tbl);
            RAISE NOTICE 'Analyzed table %', tbl;
        END IF;
    END LOOP;
END $$;

-- ============================================================================
-- Limpar funcao helper (opcional)
-- ============================================================================
-- DROP FUNCTION IF EXISTS create_index_if_table_exists;

-- ============================================================================
-- FIM DA MIGRATION
-- ============================================================================
SELECT 'Performance indexes migration complete!' AS status;
