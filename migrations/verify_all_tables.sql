-- ============================================================================
-- SCRIPT DE VERIFICAÇÃO COMPLETA - TODAS AS MIGRATIONS
-- ============================================================================

\echo '================================================================================'
\echo 'VERIFICANDO TABELAS DO EVA-MIND-FZPN'
\echo '================================================================================'
\echo ''

-- ============================================================================
-- SPRINT 2: Clinical and Vision Features (002)
-- ============================================================================
\echo '--- SPRINT 2: Clinical and Vision Features ---'
SELECT
    table_name,
    CASE
        WHEN table_name IN (
            'medication_visual_logs',
            'medication_identifications',
            'clinical_assessments',
            'clinical_assessment_responses',
            'voice_prosody_analyses',
            'voice_prosody_features'
        ) THEN '✅'
        ELSE '❌'
    END as status
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'medication_visual_logs',
    'medication_identifications',
    'clinical_assessments',
    'clinical_assessment_responses',
    'voice_prosody_analyses',
    'voice_prosody_features'
  )
ORDER BY table_name;

\echo ''

-- ============================================================================
-- SPRINT 1: Cognitive Load & Ethical Boundaries (003)
-- ============================================================================
\echo '--- SPRINT 1: Cognitive Load & Ethical Boundaries ---'
SELECT
    table_name,
    CASE
        WHEN table_name IN (
            'interaction_cognitive_load',
            'cognitive_load_state',
            'cognitive_load_decisions',
            'ethical_boundary_events',
            'ethical_boundary_state',
            'ethical_redirections'
        ) THEN '✅'
        ELSE '❌'
    END as status
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'interaction_cognitive_load',
    'cognitive_load_state',
    'cognitive_load_decisions',
    'ethical_boundary_events',
    'ethical_boundary_state',
    'ethical_redirections'
  )
ORDER BY table_name;

\echo ''

-- ============================================================================
-- SPRINT 2: Clinical Decision Explainer (004)
-- ============================================================================
\echo '--- SPRINT 2: Clinical Decision Explainer ---'
SELECT
    table_name,
    CASE
        WHEN table_name IN (
            'clinical_decision_explanations',
            'decision_factors',
            'prediction_accuracy_log'
        ) THEN '✅'
        ELSE '❌'
    END as status
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'clinical_decision_explanations',
    'decision_factors',
    'prediction_accuracy_log'
  )
ORDER BY table_name;

\echo ''

-- ============================================================================
-- SPRINT 3: Predictive Life Trajectory (005)
-- ============================================================================
\echo '--- SPRINT 3: Predictive Life Trajectory ---'
SELECT
    table_name,
    CASE
        WHEN table_name IN (
            'trajectory_simulations',
            'intervention_scenarios',
            'recommended_interventions',
            'trajectory_prediction_accuracy',
            'bayesian_network_parameters'
        ) THEN '✅'
        ELSE '❌'
    END as status
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'trajectory_simulations',
    'intervention_scenarios',
    'recommended_interventions',
    'trajectory_prediction_accuracy',
    'bayesian_network_parameters'
  )
ORDER BY table_name;

\echo ''
\echo '================================================================================'
\echo 'RESUMO GERAL'
\echo '================================================================================'

-- Contar tabelas por sprint
SELECT
    'SPRINT 2 (Clinical/Vision)' as sprint,
    COUNT(*) as tabelas_criadas,
    6 as esperado
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'medication_visual_logs',
    'medication_identifications',
    'clinical_assessments',
    'clinical_assessment_responses',
    'voice_prosody_analyses',
    'voice_prosody_features'
  )

UNION ALL

SELECT
    'SPRINT 1 (Cognitive/Ethics)' as sprint,
    COUNT(*) as tabelas_criadas,
    6 as esperado
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'interaction_cognitive_load',
    'cognitive_load_state',
    'cognitive_load_decisions',
    'ethical_boundary_events',
    'ethical_boundary_state',
    'ethical_redirections'
  )

UNION ALL

SELECT
    'SPRINT 2 (Explainer)' as sprint,
    COUNT(*) as tabelas_criadas,
    3 as esperado
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'clinical_decision_explanations',
    'decision_factors',
    'prediction_accuracy_log'
  )

UNION ALL

SELECT
    'SPRINT 3 (Trajectory)' as sprint,
    COUNT(*) as tabelas_criadas,
    5 as esperado
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'trajectory_simulations',
    'intervention_scenarios',
    'recommended_interventions',
    'trajectory_prediction_accuracy',
    'bayesian_network_parameters'
  );

\echo ''
\echo '================================================================================'
\echo 'VIEWS CRIADAS'
\echo '================================================================================'

SELECT
    table_name as view_name,
    '✅' as status
FROM information_schema.views
WHERE table_schema = 'public'
  AND table_name LIKE 'v_%'
ORDER BY table_name;

\echo ''
\echo '================================================================================'
\echo 'FIM DA VERIFICAÇÃO'
\echo '================================================================================'
