-- Migration: Clinical Decision Explainer (CDE)
-- Description: Tabelas para explicabilidade de decisões clínicas usando SHAP
-- Created: 2026-01-24
-- Sprint 2: Explicabilidade

-- ========================================
-- 1. CLINICAL DECISION EXPLANATIONS
-- ========================================

-- Explicações detalhadas de decisões clínicas
CREATE TABLE IF NOT EXISTS clinical_decision_explanations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Tipo de decisão
    decision_type VARCHAR(50) NOT NULL CHECK (decision_type IN (
        'crisis_prediction',
        'depression_alert',
        'anxiety_alert',
        'medication_alert',
        'suicide_risk',
        'hospitalization_risk',
        'fall_risk'
    )),

    -- Predição
    prediction_score DECIMAL(5,4) NOT NULL CHECK (prediction_score BETWEEN 0 AND 1), -- 0-1 (probabilidade)
    prediction_timeframe VARCHAR(50), -- '24-48h', '7-14 days', etc
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),

    -- SHAP values (feature importance)
    feature_contributions JSONB NOT NULL, -- {medication_adherence: 0.35, voice: 0.28, sleep: 0.18, ...}

    -- Dados das features usadas na predição
    features_snapshot JSONB NOT NULL, -- {medication_adherence: 0.42, phq9_score: 18, sleep_hours: 4.2, ...}

    -- Explicação gerada
    explanation_text TEXT NOT NULL, -- Explicação em linguagem natural
    explanation_structured JSONB NOT NULL, -- Explicação estruturada (primary_factors, secondary_factors)

    -- Recomendações
    recommendations JSONB NOT NULL, -- [{urgency: 'high', action: 'Contato telefônico 24h', ...}]

    -- Evidências de suporte
    supporting_evidence JSONB, -- {audio_samples: [...], conversation_excerpts: [...], graphs: {...}}

    -- PDFs e relatórios
    explanation_pdf_url TEXT,
    report_generated_at TIMESTAMP,

    -- Auditoria
    model_version VARCHAR(50) NOT NULL DEFAULT 'v1.0.0',
    confidence_interval JSONB, -- {lower: 0.65, upper: 0.79}

    -- Feedback médico
    doctor_reviewed BOOLEAN DEFAULT FALSE,
    doctor_feedback TEXT,
    doctor_agreed BOOLEAN,
    reviewed_at TIMESTAMP,
    reviewed_by INTEGER, -- ID do médico/profissional

    -- Outcome real (para validação do modelo)
    actual_outcome BOOLEAN, -- O que foi predito realmente aconteceu?
    outcome_notes TEXT,
    outcome_recorded_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Fatores individuais que contribuíram para decisão
CREATE TABLE IF NOT EXISTS decision_factors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    explanation_id UUID NOT NULL REFERENCES clinical_decision_explanations(id) ON DELETE CASCADE,

    -- Factor info
    factor_name VARCHAR(100) NOT NULL, -- 'medication_adherence', 'voice_biomarkers', 'sleep_quality', etc
    factor_category VARCHAR(50) NOT NULL, -- 'primary', 'secondary', 'tertiary'

    -- Contribuição SHAP
    shap_value DECIMAL(5,4) NOT NULL, -- -1 to +1 (contribuição para predição)
    contribution_percentage DECIMAL(5,2) NOT NULL, -- 0-100%

    -- Valores das features
    current_value DECIMAL(10,4),
    baseline_value DECIMAL(10,4), -- Valor normal/esperado
    change_from_baseline DECIMAL(10,4), -- Delta

    -- Status
    status VARCHAR(20) NOT NULL CHECK (status IN ('normal', 'warning', 'concerning', 'critical')),

    -- Detalhes
    details JSONB, -- Informações específicas do fator
    human_readable_explanation TEXT, -- Explicação em português

    -- Evidências
    evidence_references TEXT[], -- Links para áudio, conversas, gráficos, etc

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Histórico de acurácia das predições (para melhorar modelo)
CREATE TABLE IF NOT EXISTS prediction_accuracy_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    explanation_id UUID NOT NULL REFERENCES clinical_decision_explanations(id) ON DELETE CASCADE,

    -- Predição vs realidade
    predicted_outcome BOOLEAN NOT NULL,
    predicted_probability DECIMAL(5,4) NOT NULL,
    actual_outcome BOOLEAN NOT NULL,
    was_correct BOOLEAN GENERATED ALWAYS AS (predicted_outcome = actual_outcome) STORED,

    -- Timing
    predicted_timeframe VARCHAR(50),
    actual_timeframe VARCHAR(50), -- Quando realmente aconteceu

    -- Métricas de erro
    prediction_error DECIMAL(5,4), -- |predicted - actual|
    brier_score DECIMAL(5,4), -- (predicted_prob - actual)^2

    -- Contexto
    interventions_applied JSONB, -- Intervenções que foram feitas após a predição
    external_factors JSONB, -- Fatores externos que influenciaram (hospitalização, mudança médico, etc)

    notes TEXT,
    logged_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Índices
CREATE INDEX IF NOT EXISTS idx_clinical_explanations_patient ON clinical_decision_explanations(patient_id);
CREATE INDEX IF NOT EXISTS idx_clinical_explanations_type ON clinical_decision_explanations(decision_type);
CREATE INDEX IF NOT EXISTS idx_clinical_explanations_severity ON clinical_decision_explanations(severity);
CREATE INDEX IF NOT EXISTS idx_clinical_explanations_score ON clinical_decision_explanations(prediction_score DESC);
CREATE INDEX IF NOT EXISTS idx_clinical_explanations_created ON clinical_decision_explanations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_clinical_explanations_reviewed ON clinical_decision_explanations(doctor_reviewed);

CREATE INDEX IF NOT EXISTS idx_decision_factors_explanation ON decision_factors(explanation_id);
CREATE INDEX IF NOT EXISTS idx_decision_factors_name ON decision_factors(factor_name);
CREATE INDEX IF NOT EXISTS idx_decision_factors_category ON decision_factors(factor_category);
CREATE INDEX IF NOT EXISTS idx_decision_factors_contribution ON decision_factors(contribution_percentage DESC);

CREATE INDEX IF NOT EXISTS idx_prediction_accuracy_explanation ON prediction_accuracy_log(explanation_id);
CREATE INDEX IF NOT EXISTS idx_prediction_accuracy_correct ON prediction_accuracy_log(was_correct);

-- Comentários
COMMENT ON TABLE clinical_decision_explanations IS 'Explicações detalhadas de decisões clínicas usando SHAP (Explainable AI)';
COMMENT ON TABLE decision_factors IS 'Fatores individuais que contribuíram para cada decisão clínica';
COMMENT ON TABLE prediction_accuracy_log IS 'Log de acurácia para melhorar modelo ao longo do tempo';

COMMENT ON COLUMN clinical_decision_explanations.feature_contributions IS 'SHAP values: contribuição de cada feature para a predição';
COMMENT ON COLUMN clinical_decision_explanations.features_snapshot IS 'Snapshot dos valores das features no momento da predição';
COMMENT ON COLUMN decision_factors.shap_value IS 'Valor SHAP (-1 a +1): contribuição do fator para a predição';

-- ========================================
-- 2. TRIGGERS
-- ========================================

-- Trigger para atualizar updated_at
DROP TRIGGER IF EXISTS update_clinical_explanations_updated_at ON clinical_decision_explanations;
CREATE TRIGGER update_clinical_explanations_updated_at
    BEFORE UPDATE ON clinical_decision_explanations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ========================================
-- 3. VIEWS PARA MÉDICOS
-- ========================================

-- View: Predições recentes de alto risco
CREATE OR REPLACE VIEW v_high_risk_predictions AS
SELECT
    cde.id,
    cde.patient_id,
    i.nome AS patient_name,
    cde.decision_type,
    cde.prediction_score,
    cde.prediction_timeframe,
    cde.severity,
    cde.explanation_text,
    cde.doctor_reviewed,
    cde.created_at,
    -- Top 3 contributing factors
    (
        SELECT json_agg(json_build_object(
            'factor', df.factor_name,
            'contribution', df.contribution_percentage,
            'status', df.status
        ) ORDER BY df.contribution_percentage DESC)
        FROM decision_factors df
        WHERE df.explanation_id = cde.id
        LIMIT 3
    ) AS top_factors
FROM clinical_decision_explanations cde
JOIN idosos i ON cde.patient_id = i.id
WHERE cde.severity IN ('high', 'critical')
  AND cde.created_at > NOW() - INTERVAL '7 days'
  AND cde.doctor_reviewed = FALSE
ORDER BY cde.severity DESC, cde.prediction_score DESC, cde.created_at DESC;

-- View: Acurácia do modelo por tipo de decisão
CREATE OR REPLACE VIEW v_model_accuracy_by_type AS
SELECT
    cde.decision_type,
    COUNT(*) AS total_predictions,
    COUNT(*) FILTER (WHERE pal.was_correct = TRUE) AS correct_predictions,
    ROUND(
        (COUNT(*) FILTER (WHERE pal.was_correct = TRUE)::DECIMAL / COUNT(*)) * 100,
        2
    ) AS accuracy_percentage,
    AVG(pal.brier_score) AS avg_brier_score,
    AVG(pal.prediction_error) AS avg_prediction_error
FROM clinical_decision_explanations cde
JOIN prediction_accuracy_log pal ON cde.id = pal.explanation_id
WHERE cde.created_at > NOW() - INTERVAL '90 days'
GROUP BY cde.decision_type
ORDER BY accuracy_percentage DESC;

-- View: Alertas pendentes de revisão médica
CREATE OR REPLACE VIEW v_pending_doctor_review AS
SELECT
    cde.id AS explanation_id,
    cde.patient_id,
    i.nome AS patient_name,
    i.telefone AS patient_phone,
    cde.decision_type,
    cde.severity,
    cde.prediction_score,
    cde.prediction_timeframe,
    cde.explanation_text,
    cde.recommendations,
    DATE_PART('hour', NOW() - cde.created_at) AS hours_since_alert,
    CASE
        WHEN cde.severity = 'critical' AND DATE_PART('hour', NOW() - cde.created_at) > 2 THEN TRUE
        WHEN cde.severity = 'high' AND DATE_PART('hour', NOW() - cde.created_at) > 12 THEN TRUE
        ELSE FALSE
    END AS is_overdue
FROM clinical_decision_explanations cde
JOIN idosos i ON cde.patient_id = i.id
WHERE cde.doctor_reviewed = FALSE
  AND cde.severity IN ('high', 'critical')
ORDER BY
    CASE cde.severity
        WHEN 'critical' THEN 1
        WHEN 'high' THEN 2
        ELSE 3
    END,
    cde.created_at ASC;

COMMENT ON VIEW v_high_risk_predictions IS 'Predições de alto risco dos últimos 7 dias não revisadas';
COMMENT ON VIEW v_model_accuracy_by_type IS 'Acurácia do modelo de predição por tipo de decisão (últimos 90 dias)';
COMMENT ON VIEW v_pending_doctor_review IS 'Alertas pendentes de revisão médica com indicador de atraso';
