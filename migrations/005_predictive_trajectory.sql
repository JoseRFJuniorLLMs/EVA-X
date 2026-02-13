-- ============================================================================
-- SPRINT 3: PREDICTIVE LIFE TRAJECTORY ENGINE
-- ============================================================================
-- Descrição: Sistema de simulação prospectiva usando Bayesian Belief Networks
--            e Monte Carlo para prever trajetórias de saúde mental
-- Autor: EVA-Mind Development Team
-- Data: 2026-01-24
-- ============================================================================

-- ============================================================================
-- 1. TRAJECTORY SIMULATIONS
-- ============================================================================
-- Armazena resultados de simulações Monte Carlo de trajetórias de pacientes
CREATE TABLE IF NOT EXISTS trajectory_simulations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    simulation_date TIMESTAMP NOT NULL DEFAULT NOW(),
    days_ahead INTEGER NOT NULL CHECK (days_ahead > 0),
    n_simulations INTEGER NOT NULL DEFAULT 1000 CHECK (n_simulations >= 100),

    -- Probabilidades de desfechos (0-1)
    crisis_probability_7d DECIMAL(5,4) CHECK (crisis_probability_7d BETWEEN 0 AND 1),
    crisis_probability_30d DECIMAL(5,4) CHECK (crisis_probability_30d BETWEEN 0 AND 1),
    hospitalization_probability_30d DECIMAL(5,4) CHECK (hospitalization_probability_30d BETWEEN 0 AND 1),
    treatment_dropout_probability_90d DECIMAL(5,4) CHECK (treatment_dropout_probability_90d BETWEEN 0 AND 1),
    fall_risk_probability_7d DECIMAL(5,4) CHECK (fall_risk_probability_7d BETWEEN 0 AND 1),

    -- Trajetórias projetadas (valores médios ao final do período)
    projected_phq9_score DECIMAL(4,2),
    projected_medication_adherence DECIMAL(3,2),
    projected_sleep_hours DECIMAL(3,1),
    projected_social_isolation_days INTEGER,

    -- Fatores críticos identificados
    critical_factors TEXT[] DEFAULT ARRAY[]::TEXT[],

    -- Amostra de trajetórias individuais (primeiras 10 para visualização)
    sample_trajectories JSONB,
    -- Formato: [
    --   {"day": 0, "phq9": 14, "adherence": 0.65, "sleep": 4.2},
    --   {"day": 1, "phq9": 14.3, "adherence": 0.63, "sleep": 4.1},
    --   ...
    -- ]

    -- Estado inicial usado na simulação
    initial_state JSONB,

    -- Modelo e versão
    model_version VARCHAR(50) NOT NULL DEFAULT 'v1.0.0',
    bayesian_network_config JSONB,

    -- Metadados
    computation_time_ms INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Índices para performance
    CONSTRAINT trajectory_simulations_patient_date_unique UNIQUE(patient_id, simulation_date)
);

CREATE INDEX IF NOT EXISTS idx_trajectory_simulations_patient ON trajectory_simulations(patient_id, simulation_date DESC);
CREATE INDEX IF NOT EXISTS idx_trajectory_simulations_high_risk ON trajectory_simulations(patient_id)
    WHERE crisis_probability_30d > 0.3;

COMMENT ON TABLE trajectory_simulations IS 'Simulações Monte Carlo de trajetórias de saúde mental usando Bayesian Networks';
COMMENT ON COLUMN trajectory_simulations.n_simulations IS 'Número de simulações Monte Carlo executadas (default: 1000)';
COMMENT ON COLUMN trajectory_simulations.sample_trajectories IS 'Amostra de 10 trajetórias individuais para visualização';


-- ============================================================================
-- 2. INTERVENTION SCENARIOS
-- ============================================================================
-- Armazena simulações "what-if" com diferentes intervenções
CREATE TABLE IF NOT EXISTS intervention_scenarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    simulation_id UUID NOT NULL REFERENCES trajectory_simulations(id) ON DELETE CASCADE,
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Tipo de cenário
    scenario_type VARCHAR(50) NOT NULL, -- 'baseline', 'with_intervention'
    scenario_name VARCHAR(200) NOT NULL,
    scenario_description TEXT,

    -- Intervenções aplicadas no cenário
    interventions JSONB,
    -- Formato: [
    --   {"type": "medication_reminders", "frequency": "2x/day", "impact_adherence": +0.15},
    --   {"type": "psychiatric_consultation", "schedule": "weekly", "impact_phq9": -3},
    --   {"type": "sleep_hygiene_protocol", "impact_sleep": +2.5},
    --   {"type": "family_calls", "frequency": "2x/week", "impact_isolation": -3}
    -- ]

    -- Resultados do cenário
    crisis_probability_7d DECIMAL(5,4),
    crisis_probability_30d DECIMAL(5,4),
    hospitalization_probability_30d DECIMAL(5,4),

    -- Projeções ao final do período
    projected_phq9_score DECIMAL(4,2),
    projected_medication_adherence DECIMAL(3,2),
    projected_sleep_hours DECIMAL(3,1),

    -- Impacto (diferença vs baseline)
    risk_reduction_7d DECIMAL(5,4), -- Redução de risco vs baseline
    risk_reduction_30d DECIMAL(5,4),

    -- Efetividade estimada
    effectiveness_score DECIMAL(3,2) CHECK (effectiveness_score BETWEEN 0 AND 1),
    -- 0-1: quanto melhor que baseline

    -- Viabilidade
    estimated_cost_monthly DECIMAL(8,2), -- Custo estimado em R$
    feasibility VARCHAR(20) CHECK (feasibility IN ('high', 'medium', 'low')),
    required_resources TEXT[],

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_intervention_scenarios_simulation ON intervention_scenarios(simulation_id);
CREATE INDEX IF NOT EXISTS idx_intervention_scenarios_patient ON intervention_scenarios(patient_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_intervention_scenarios_effective ON intervention_scenarios(patient_id)
    WHERE effectiveness_score > 0.5;

COMMENT ON TABLE intervention_scenarios IS 'Cenários "what-if" comparando trajetórias com e sem intervenções';
COMMENT ON COLUMN intervention_scenarios.effectiveness_score IS 'Score 0-1 de efetividade vs baseline';


-- ============================================================================
-- 3. RECOMMENDED INTERVENTIONS
-- ============================================================================
-- Intervenções recomendadas baseadas nas simulações
CREATE TABLE IF NOT EXISTS recommended_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    simulation_id UUID NOT NULL REFERENCES trajectory_simulations(id) ON DELETE CASCADE,
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Recomendação
    intervention_type VARCHAR(100) NOT NULL,
    -- medication_adherence_boost, psychiatric_consultation, sleep_protocol,
    -- family_engagement, crisis_prevention_plan, therapy_intensification

    priority VARCHAR(20) NOT NULL CHECK (priority IN ('critical', 'high', 'medium', 'low')),
    urgency_timeframe VARCHAR(50), -- '24h', '3-5 days', '1 week'

    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    rationale TEXT NOT NULL, -- Por que essa intervenção é recomendada

    -- Impacto esperado
    expected_risk_reduction DECIMAL(5,4), -- Redução esperada na probabilidade de crise
    expected_phq9_improvement DECIMAL(4,2),
    confidence_level DECIMAL(3,2) CHECK (confidence_level BETWEEN 0 AND 1),

    -- Implementação
    action_steps TEXT[], -- Lista de passos concretos
    responsible_parties TEXT[], -- ['family', 'psychiatrist', 'caregiver']
    estimated_cost DECIMAL(8,2),

    -- Tracking
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'in_progress', 'completed', 'rejected')),
    implemented_at TIMESTAMP,
    implemented_by VARCHAR(100),

    -- Feedback
    actual_outcome_measured BOOLEAN DEFAULT FALSE,
    actual_risk_reduction DECIMAL(5,4), -- Medido após implementação
    effectiveness_rating INTEGER CHECK (effectiveness_rating BETWEEN 1 AND 5),

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recommended_interventions_simulation ON recommended_interventions(simulation_id);
CREATE INDEX IF NOT EXISTS idx_recommended_interventions_patient_status ON recommended_interventions(patient_id, status, priority);
CREATE INDEX IF NOT EXISTS idx_recommended_interventions_pending ON recommended_interventions(patient_id, created_at DESC)
    WHERE status = 'pending';

COMMENT ON TABLE recommended_interventions IS 'Intervenções recomendadas com base nas simulações de trajetória';
COMMENT ON COLUMN recommended_interventions.confidence_level IS 'Confiança na estimativa de impacto (baseado em dados históricos)';


-- ============================================================================
-- 4. TRAJECTORY ACCURACY LOG
-- ============================================================================
-- Rastreia acurácia das predições ao longo do tempo
CREATE TABLE IF NOT EXISTS trajectory_prediction_accuracy (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    simulation_id UUID NOT NULL REFERENCES trajectory_simulations(id) ON DELETE CASCADE,
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,

    -- Predição original
    predicted_crisis_7d BOOLEAN,
    predicted_crisis_30d BOOLEAN,
    prediction_date TIMESTAMP NOT NULL,

    -- Desfecho real
    actual_crisis_occurred BOOLEAN,
    crisis_occurred_at TIMESTAMP,
    days_until_crisis INTEGER,

    -- Métricas de acurácia
    prediction_correct BOOLEAN,
    false_positive BOOLEAN,
    false_negative BOOLEAN,

    -- Erro nas projeções
    phq9_prediction_error DECIMAL(5,2), -- |predicted - actual|
    adherence_prediction_error DECIMAL(4,3),
    sleep_prediction_error DECIMAL(3,1),

    -- Calibração
    calibration_score DECIMAL(3,2) CHECK (calibration_score BETWEEN 0 AND 1),
    -- Quão bem calibrado estava o modelo (probabilidades vs outcomes reais)

    evaluation_date TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Metadados
    model_version VARCHAR(50),
    notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_trajectory_accuracy_patient ON trajectory_prediction_accuracy(patient_id, evaluation_date DESC);
CREATE INDEX IF NOT EXISTS idx_trajectory_accuracy_model ON trajectory_prediction_accuracy(model_version, prediction_correct);
CREATE INDEX IF NOT EXISTS idx_trajectory_accuracy_false_negatives ON trajectory_prediction_accuracy(patient_id)
    WHERE false_negative = TRUE;

COMMENT ON TABLE trajectory_prediction_accuracy IS 'Rastreia acurácia das predições para melhorar modelo ao longo do tempo';
COMMENT ON COLUMN trajectory_prediction_accuracy.calibration_score IS 'Quão bem calibrado estava o modelo (0=péssimo, 1=perfeito)';


-- ============================================================================
-- 5. BAYESIAN NETWORK PARAMETERS
-- ============================================================================
-- Armazena parâmetros aprendidos da rede Bayesiana
CREATE TABLE IF NOT EXISTS bayesian_network_parameters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_version VARCHAR(50) NOT NULL,

    -- Nó da rede
    node_name VARCHAR(100) NOT NULL,
    node_type VARCHAR(50) CHECK (node_type IN ('observable', 'latent', 'outcome')),

    -- Nós pais (dependências causais)
    parent_nodes TEXT[],

    -- Tabela de probabilidade condicional (CPT)
    conditional_probability_table JSONB NOT NULL,
    -- Formato: {
    --   "conditions": [
    --     {"parent_values": {"medication_adherence": "low", "sleep": "poor"}, "prob_crisis": 0.65},
    --     {"parent_values": {"medication_adherence": "high", "sleep": "good"}, "prob_crisis": 0.12}
    --   ]
    -- }

    -- Estatísticas de aprendizado
    learned_from_n_patients INTEGER,
    confidence_interval JSONB, -- {"lower": X, "upper": Y}
    last_updated TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Validação
    cross_validation_score DECIMAL(3,2),
    auc_roc DECIMAL(4,3),

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT bayesian_params_unique UNIQUE(model_version, node_name)
);

CREATE INDEX IF NOT EXISTS idx_bayesian_params_model ON bayesian_network_parameters(model_version, node_type);

COMMENT ON TABLE bayesian_network_parameters IS 'Parâmetros aprendidos da rede Bayesiana (CPTs)';
COMMENT ON COLUMN bayesian_network_parameters.conditional_probability_table IS 'Tabela de probabilidade condicional (CPT) para inferência';


-- ============================================================================
-- 6. VIEWS ÚTEIS
-- ============================================================================

-- View: Últimas simulações por paciente
CREATE OR REPLACE VIEW v_latest_trajectory_simulations AS
SELECT DISTINCT ON (patient_id)
    ts.id,
    ts.patient_id,
    i.nome AS patient_name,
    ts.simulation_date,
    ts.days_ahead,
    ts.crisis_probability_7d,
    ts.crisis_probability_30d,
    ts.hospitalization_probability_30d,
    ts.critical_factors,
    ts.projected_phq9_score,
    ts.model_version,
    -- Classificação de risco
    CASE
        WHEN ts.crisis_probability_30d >= 0.6 THEN 'critical'
        WHEN ts.crisis_probability_30d >= 0.4 THEN 'high'
        WHEN ts.crisis_probability_30d >= 0.2 THEN 'moderate'
        ELSE 'low'
    END AS risk_level
FROM trajectory_simulations ts
JOIN idosos i ON ts.patient_id = i.id
ORDER BY ts.patient_id, ts.simulation_date DESC;

COMMENT ON VIEW v_latest_trajectory_simulations IS 'Última simulação de trajetória para cada paciente';


-- View: Pacientes de alto risco com intervenções pendentes
CREATE OR REPLACE VIEW v_high_risk_patients_pending_interventions AS
SELECT
    ts.patient_id,
    i.nome AS patient_name,
    ts.crisis_probability_30d,
    COUNT(ri.id) AS pending_interventions_count,
    ARRAY_AGG(ri.title ORDER BY ri.priority DESC) AS pending_intervention_titles,
    MAX(ri.created_at) AS last_recommendation_date
FROM trajectory_simulations ts
JOIN idosos i ON ts.patient_id = i.id
LEFT JOIN recommended_interventions ri ON ts.id = ri.simulation_id AND ri.status = 'pending'
WHERE ts.crisis_probability_30d > 0.3
  AND ts.simulation_date > NOW() - INTERVAL '7 days'
GROUP BY ts.patient_id, i.nome, ts.crisis_probability_30d
HAVING COUNT(ri.id) > 0
ORDER BY ts.crisis_probability_30d DESC;

COMMENT ON VIEW v_high_risk_patients_pending_interventions IS 'Pacientes de alto risco com intervenções recomendadas não implementadas';


-- View: Acurácia do modelo por versão
CREATE OR REPLACE VIEW v_model_accuracy_by_version AS
SELECT
    model_version,
    COUNT(*) AS total_predictions,
    SUM(CASE WHEN prediction_correct THEN 1 ELSE 0 END) AS correct_predictions,
    ROUND(
        SUM(CASE WHEN prediction_correct THEN 1 ELSE 0 END)::NUMERIC /
        NULLIF(COUNT(*), 0) * 100,
        2
    ) AS accuracy_percentage,
    SUM(CASE WHEN false_positive THEN 1 ELSE 0 END) AS false_positives,
    SUM(CASE WHEN false_negative THEN 1 ELSE 0 END) AS false_negatives,
    AVG(calibration_score) AS avg_calibration_score,
    MAX(evaluation_date) AS last_evaluation
FROM trajectory_prediction_accuracy
GROUP BY model_version
ORDER BY last_evaluation DESC;

COMMENT ON VIEW v_model_accuracy_by_version IS 'Métricas de acurácia do modelo de trajetória por versão';


-- ============================================================================
-- 7. TRIGGERS
-- ============================================================================

-- Trigger: Atualizar updated_at em recommended_interventions
CREATE OR REPLACE FUNCTION update_recommended_interventions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_recommended_interventions_updated_at ON recommended_interventions;
CREATE TRIGGER trigger_update_recommended_interventions_updated_at
    BEFORE UPDATE ON recommended_interventions
    FOR EACH ROW
    EXECUTE FUNCTION update_recommended_interventions_updated_at();


-- ============================================================================
-- 8. FUNCTION: Gerar relatório de trajetória para paciente
-- ============================================================================
CREATE OR REPLACE FUNCTION get_patient_trajectory_report(p_patient_id INTEGER)
RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    SELECT jsonb_build_object(
        'patient_id', ts.patient_id,
        'patient_name', i.nome,
        'latest_simulation', jsonb_build_object(
            'date', ts.simulation_date,
            'risk_7d', ts.crisis_probability_7d,
            'risk_30d', ts.crisis_probability_30d,
            'hospitalization_risk', ts.hospitalization_probability_30d,
            'critical_factors', ts.critical_factors,
            'projected_phq9', ts.projected_phq9_score
        ),
        'pending_interventions', (
            SELECT COALESCE(jsonb_agg(
                jsonb_build_object(
                    'title', ri.title,
                    'priority', ri.priority,
                    'expected_reduction', ri.expected_risk_reduction,
                    'timeframe', ri.urgency_timeframe
                ) ORDER BY ri.priority DESC
            ), '[]'::jsonb)
            FROM recommended_interventions ri
            WHERE ri.patient_id = p_patient_id
              AND ri.status = 'pending'
        ),
        'best_scenario', (
            SELECT jsonb_build_object(
                'name', scenario_name,
                'risk_30d', crisis_probability_30d,
                'risk_reduction', risk_reduction_30d
            )
            FROM intervention_scenarios isc
            WHERE isc.patient_id = p_patient_id
              AND isc.scenario_type = 'with_intervention'
            ORDER BY effectiveness_score DESC
            LIMIT 1
        )
    ) INTO result
    FROM trajectory_simulations ts
    JOIN idosos i ON ts.patient_id = i.id
    WHERE ts.patient_id = p_patient_id
    ORDER BY ts.simulation_date DESC
    LIMIT 1;

    RETURN result;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_patient_trajectory_report IS 'Gera relatório JSON completo de trajetória para um paciente';


-- ============================================================================
-- 9. DADOS DE EXEMPLO (COMENTADOS - DESCOMENTAR SE NECESSÁRIO)
-- ============================================================================
/*
-- Exemplo de simulação
INSERT INTO trajectory_simulations (
    patient_id, days_ahead, n_simulations,
    crisis_probability_7d, crisis_probability_30d,
    hospitalization_probability_30d,
    projected_phq9_score, projected_medication_adherence,
    critical_factors,
    initial_state
) VALUES (
    1, 30, 1000,
    0.15, 0.42, 0.08,
    19.0, 0.45,
    ARRAY['low_medication_adherence', 'poor_sleep', 'social_isolation'],
    '{"phq9": 14, "adherence": 0.65, "sleep": 4.2, "isolation_days": 5}'::jsonb
);
*/

-- ============================================================================
-- FIM DA MIGRATION
-- ============================================================================
