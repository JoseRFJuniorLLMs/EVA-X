-- Script para verificar se todas as tabelas foram criadas corretamente

-- Verificar tabelas existentes
SELECT
    table_name,
    CASE
        WHEN table_name IN ('medication_visual_logs', 'medication_identifications',
                           'clinical_assessments', 'clinical_assessment_responses',
                           'voice_prosody_analyses', 'voice_prosody_features')
        THEN '✅ Existe'
        ELSE '❌ Não encontrada'
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

-- Contar total
SELECT COUNT(*) as total_tabelas_criadas
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'medication_visual_logs',
    'medication_identifications',
    'clinical_assessments',
    'clinical_assessment_responses',
    'voice_prosody_analyses',
    'voice_prosody_features'
  );

-- Verificar índices criados
SELECT
    schemaname,
    tablename,
    indexname
FROM pg_indexes
WHERE tablename IN (
    'medication_visual_logs',
    'medication_identifications',
    'clinical_assessments',
    'clinical_assessment_responses',
    'voice_prosody_analyses',
    'voice_prosody_features'
)
ORDER BY tablename, indexname;

-- Verificar views criadas
SELECT
    table_name as view_name
FROM information_schema.views
WHERE table_schema = 'public'
  AND table_name IN (
    'v_latest_clinical_assessments',
    'v_critical_risk_patients',
    'v_medication_scan_summary'
  )
ORDER BY table_name;
