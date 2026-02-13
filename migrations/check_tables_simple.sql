-- Lista TODAS as tabelas do schema public (não views)
SELECT
    table_name,
    CASE
        WHEN table_name LIKE 'trajectory%' THEN '🔮 SPRINT 3'
        WHEN table_name LIKE 'intervention%' THEN '🔮 SPRINT 3'
        WHEN table_name LIKE 'recommended%' THEN '🔮 SPRINT 3'
        WHEN table_name LIKE 'bayesian%' THEN '🔮 SPRINT 3'
        WHEN table_name LIKE 'clinical_decision%' THEN '📊 SPRINT 2'
        WHEN table_name LIKE 'decision_factors%' THEN '📊 SPRINT 2'
        WHEN table_name LIKE 'prediction_accuracy%' THEN '📊 SPRINT 2'
        WHEN table_name LIKE 'cognitive%' THEN '🧠 SPRINT 1'
        WHEN table_name LIKE 'ethical%' THEN '🧠 SPRINT 1'
        WHEN table_name LIKE 'interaction_cognitive%' THEN '🧠 SPRINT 1'
        WHEN table_name LIKE 'medication_visual%' THEN '💊 Features'
        WHEN table_name LIKE 'medication_identif%' THEN '💊 Features'
        WHEN table_name LIKE 'clinical_assessment%' THEN '📋 Scales'
        WHEN table_name LIKE 'voice_prosody%' THEN '🎙️ Voice'
        ELSE '📁 Other'
    END as categoria
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_type = 'BASE TABLE'
ORDER BY categoria, table_name;
