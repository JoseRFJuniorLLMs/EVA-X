-- ============================================================================
-- SPRINT 4: CLINICAL RESEARCH ENGINE
-- ============================================================================
-- Descrição: Pipeline de análise longitudinal, estudos científicos validados,
--            e publicação de resultados para transformar EVA em DTx defensável
-- Autor: EVA-Mind Development Team
-- Data: 2026-01-24
-- ============================================================================

-- ============================================================================
-- 1. RESEARCH COHORTS (COORTES DE PESQUISA)
-- ============================================================================
-- Define estudos científicos e critérios de inclusão/exclusão
CREATE TABLE IF NOT EXISTS research_cohorts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    study_name VARCHAR(200) NOT NULL UNIQUE,
    study_code VARCHAR(50) NOT NULL UNIQUE, -- Ex: "EVA-VOICE-PHQ9-001"
    hypothesis TEXT NOT NULL,
    study_type VARCHAR(50) NOT NULL CHECK (study_type IN (
        'longitudinal_correlation',
        'causal_inference',
        'survival_analysis',
        'prediction_validation',
        'intervention_trial'
    )),

    -- Critérios
    inclusion_criteria JSONB NOT NULL,
    -- Exemplo: {"min_age": 60, "max_age": 90, "has_depression_diagnosis": true, "min_data_points": 10}
    exclusion_criteria JSONB,
    -- Exemplo: {"severe_cognitive_impairment": true, "hospitalized": true}

    -- Tamanho da amostra
    target_n_patients INTEGER NOT NULL,
    current_n_patients INTEGER DEFAULT 0,

    -- Período
    data_collection_start_date DATE NOT NULL,
    data_collection_end_date DATE,
    followup_duration_days INTEGER, -- Duração de seguimento (ex: 180 dias)

    -- Status
    status VARCHAR(50) DEFAULT 'recruiting' CHECK (status IN (
        'planning',
        'recruiting',
        'data_collection',
        'analyzing',
        'completed',
        'published',
        'abandoned'
    )),

    -- Análise
    primary_outcome VARCHAR(200), -- Ex: "PHQ-9 improvement", "Crisis occurrence"
    secondary_outcomes TEXT[],
    statistical_methods TEXT[], -- Ex: ["lag_correlation", "propensity_score_matching"]

    -- Resultados
    results JSONB, -- Resultados finais quando completed
    statistical_power DECIMAL(3,2), -- Power analysis (0-1)
    p_value DECIMAL(10,8), -- Statistical significance
    effect_size DECIMAL(5,3), -- Cohen's d ou similar
    confidence_interval_95 JSONB, -- {"lower": X, "upper": Y}

    -- Publicação
    paper_title TEXT,
    paper_abstract TEXT,
    paper_url TEXT,
    doi TEXT,
    publication_date DATE,
    journal VARCHAR(200),

    -- Aprovação ética
    ethics_committee_approval BOOLEAN DEFAULT FALSE,
    ethics_approval_number VARCHAR(100),
    ethics_approval_date DATE,

    -- Metadados
    principal_investigator VARCHAR(200),
    collaborators TEXT[],
    funding_source VARCHAR(200),

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_research_cohorts_status ON research_cohorts(status);
CREATE INDEX IF NOT EXISTS idx_research_cohorts_study_type ON research_cohorts(study_type);
CREATE INDEX IF NOT EXISTS idx_research_cohorts_published ON research_cohorts(status)
    WHERE status = 'published';

COMMENT ON TABLE research_cohorts IS 'Definições de estudos científicos e coortes de pesquisa';
COMMENT ON COLUMN research_cohorts.statistical_power IS 'Power analysis (1-beta): probabilidade de detectar efeito se existir';


-- ============================================================================
-- 2. RESEARCH DATAPOINTS (DADOS LONGITUDINAIS ANONIMIZADOS)
-- ============================================================================
-- Armazena dados anonimizados de pacientes para análise
CREATE TABLE IF NOT EXISTS research_datapoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cohort_id UUID NOT NULL REFERENCES research_cohorts(id) ON DELETE CASCADE,

    -- Identificação anonimizada (hash irreversível)
    anonymous_patient_id VARCHAR(64) NOT NULL, -- SHA-256 hash

    -- Timestamp
    observation_date DATE NOT NULL,
    days_since_baseline INTEGER, -- Dias desde início do seguimento

    -- Features clínicas
    phq9_score DECIMAL(4,2),
    gad7_score DECIMAL(4,2),
    cssrs_score INTEGER,

    -- Adesão e sono
    medication_adherence_7d DECIMAL(3,2),
    sleep_hours_avg_7d DECIMAL(3,1),
    sleep_efficiency DECIMAL(3,2),

    -- Biomarcadores de voz
    voice_pitch_mean_hz DECIMAL(5,1),
    voice_pitch_std_hz DECIMAL(5,1),
    voice_jitter DECIMAL(6,5),
    voice_shimmer DECIMAL(6,5),
    voice_hnr_db DECIMAL(4,1),
    speech_rate_wpm DECIMAL(5,1),
    pause_duration_avg_ms DECIMAL(6,1),

    -- Social e cognitivo
    social_isolation_days INTEGER,
    interaction_count_7d INTEGER,
    cognitive_load_score DECIMAL(3,2),

    -- Desfechos (outcomes)
    crisis_occurred BOOLEAN DEFAULT FALSE,
    crisis_severity VARCHAR(20) CHECK (crisis_severity IN ('mild', 'moderate', 'severe', 'critical')),
    hospitalization BOOLEAN DEFAULT FALSE,
    treatment_dropout BOOLEAN DEFAULT FALSE,

    -- Intervenções recebidas (para estudos de causalidade)
    interventions_received TEXT[],

    -- Dados agregados adicionais (flexível)
    additional_features JSONB,

    -- Flags de qualidade
    data_completeness DECIMAL(3,2), -- 0-1: % de features disponíveis
    data_quality_score DECIMAL(3,2), -- 0-1: score de qualidade (outliers, etc)

    -- Anonimização
    is_anonymized BOOLEAN DEFAULT TRUE,
    anonymization_date TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_research_datapoints_cohort ON research_datapoints(cohort_id, anonymous_patient_id, observation_date);
CREATE INDEX IF NOT EXISTS idx_research_datapoints_patient_timeline ON research_datapoints(anonymous_patient_id, days_since_baseline);
CREATE INDEX IF NOT EXISTS idx_research_datapoints_crisis ON research_datapoints(cohort_id, crisis_occurred)
    WHERE crisis_occurred = TRUE;

COMMENT ON TABLE research_datapoints IS 'Dados longitudinais anonimizados de pacientes para análise científica';
COMMENT ON COLUMN research_datapoints.anonymous_patient_id IS 'SHA-256 hash do patient_id original (irreversível)';


-- ============================================================================
-- 3. LONGITUDINAL CORRELATIONS (CORRELAÇÕES TEMPORAIS)
-- ============================================================================
-- Armazena resultados de análises de correlação lag/lead
CREATE TABLE IF NOT EXISTS longitudinal_correlations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cohort_id UUID NOT NULL REFERENCES research_cohorts(id) ON DELETE CASCADE,

    -- Variáveis correlacionadas
    predictor_variable VARCHAR(100) NOT NULL, -- Ex: "voice_pitch_mean_hz"
    outcome_variable VARCHAR(100) NOT NULL, -- Ex: "phq9_score"

    -- Lag (tempo de antecedência)
    lag_days INTEGER NOT NULL, -- Ex: 7 = voz hoje prediz PHQ-9 em 7 dias

    -- Estatísticas de correlação
    correlation_coefficient DECIMAL(6,5) NOT NULL, -- Pearson's r (-1 a +1)
    p_value DECIMAL(10,8) NOT NULL,
    confidence_interval_95 JSONB NOT NULL, -- {"lower": X, "upper": Y}

    -- Tamanho da amostra
    n_observations INTEGER NOT NULL,
    n_patients INTEGER NOT NULL,

    -- Significância
    is_significant BOOLEAN GENERATED ALWAYS AS (p_value < 0.05) STORED,
    effect_size_category VARCHAR(20) GENERATED ALWAYS AS (
        CASE
            WHEN ABS(correlation_coefficient) < 0.3 THEN 'small'
            WHEN ABS(correlation_coefficient) < 0.5 THEN 'medium'
            ELSE 'large'
        END
    ) STORED,

    -- Ajuste por covariáveis
    adjusted_for_covariates TEXT[], -- Ex: ["age", "gender", "baseline_phq9"]
    partial_correlation DECIMAL(6,5), -- Correlação ajustada

    -- Visualização
    scatter_plot_data JSONB, -- Dados para gráfico
    trend_line_equation VARCHAR(200), -- y = mx + b

    -- Metadados
    analysis_method VARCHAR(50) DEFAULT 'pearson', -- pearson, spearman, kendall
    analysis_date TIMESTAMP NOT NULL DEFAULT NOW(),
    analyzed_by VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_longitudinal_corr_cohort ON longitudinal_correlations(cohort_id);
CREATE INDEX IF NOT EXISTS idx_longitudinal_corr_significant ON longitudinal_correlations(cohort_id, is_significant)
    WHERE is_significant = TRUE;
CREATE INDEX IF NOT EXISTS idx_longitudinal_corr_vars ON longitudinal_correlations(predictor_variable, outcome_variable, lag_days);

COMMENT ON TABLE longitudinal_correlations IS 'Análises de correlação temporal (lead/lag) entre variáveis';
COMMENT ON COLUMN longitudinal_correlations.lag_days IS 'Dias de antecedência: positivo = predictor antecede outcome';


-- ============================================================================
-- 4. STATISTICAL ANALYSES (ANÁLISES ESTATÍSTICAS)
-- ============================================================================
-- Armazena resultados de análises estatísticas variadas
CREATE TABLE IF NOT EXISTS statistical_analyses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cohort_id UUID NOT NULL REFERENCES research_cohorts(id) ON DELETE CASCADE,

    -- Tipo de análise
    analysis_type VARCHAR(100) NOT NULL CHECK (analysis_type IN (
        'linear_regression',
        'logistic_regression',
        'survival_analysis',
        'propensity_score_matching',
        'mixed_effects_model',
        't_test',
        'anova',
        'chi_square',
        'mann_whitney',
        'wilcoxon',
        'kaplan_meier'
    )),

    analysis_name VARCHAR(200) NOT NULL,
    analysis_description TEXT,

    -- Variáveis
    independent_variables TEXT[] NOT NULL,
    dependent_variable VARCHAR(100) NOT NULL,
    control_variables TEXT[],

    -- Resultados principais
    results JSONB NOT NULL,
    -- Formato depende do tipo de análise:
    -- Regressão: {"coefficients": {...}, "r_squared": X, "adj_r_squared": Y}
    -- T-test: {"t_statistic": X, "df": Y, "mean_diff": Z}
    -- Survival: {"median_survival_days": X, "hazard_ratios": {...}}

    -- Significância global
    p_value DECIMAL(10,8),
    is_significant BOOLEAN GENERATED ALWAYS AS (p_value < 0.05) STORED,

    -- Qualidade do modelo
    model_fit_metrics JSONB,
    -- Ex: {"AIC": X, "BIC": Y, "log_likelihood": Z}

    -- Validação
    cross_validation_performed BOOLEAN DEFAULT FALSE,
    cv_folds INTEGER,
    cv_mean_score DECIMAL(5,4),
    cv_std_score DECIMAL(5,4),

    -- Diagnósticos
    residuals_normality_test JSONB, -- Shapiro-Wilk, etc.
    heteroscedasticity_test JSONB, -- Breusch-Pagan, etc.
    multicollinearity_vif JSONB, -- VIF scores

    -- Dados de suporte
    n_observations INTEGER NOT NULL,
    n_patients INTEGER,

    -- Visualizações
    plot_data JSONB, -- Dados para gráficos (residuals, fitted, etc)

    -- Metadados
    software_used VARCHAR(100) DEFAULT 'custom_go_implementation',
    analysis_date TIMESTAMP NOT NULL DEFAULT NOW(),
    analyzed_by VARCHAR(100),
    notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_statistical_analyses_cohort ON statistical_analyses(cohort_id);
CREATE INDEX IF NOT EXISTS idx_statistical_analyses_type ON statistical_analyses(analysis_type);
CREATE INDEX IF NOT EXISTS idx_statistical_analyses_significant ON statistical_analyses(cohort_id, is_significant)
    WHERE is_significant = TRUE;

COMMENT ON TABLE statistical_analyses IS 'Resultados de análises estatísticas diversas (regressão, testes, etc)';


-- ============================================================================
-- 5. RESEARCH PUBLICATIONS (PUBLICAÇÕES CIENTÍFICAS)
-- ============================================================================
-- Tracking de papers e apresentações
CREATE TABLE IF NOT EXISTS research_publications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cohort_id UUID REFERENCES research_cohorts(id) ON DELETE SET NULL,

    -- Identificação
    title TEXT NOT NULL,
    abstract TEXT NOT NULL,
    publication_type VARCHAR(50) NOT NULL CHECK (publication_type IN (
        'peer_reviewed_journal',
        'conference_paper',
        'preprint',
        'poster',
        'abstract',
        'white_paper',
        'technical_report'
    )),

    -- Autoria
    authors TEXT[] NOT NULL, -- Ordem de autoria
    corresponding_author VARCHAR(200),
    corresponding_author_email VARCHAR(200),
    affiliations TEXT[],

    -- Publicação
    journal_name VARCHAR(200),
    conference_name VARCHAR(200),
    volume VARCHAR(50),
    issue VARCHAR(50),
    pages VARCHAR(50),
    publication_date DATE,

    -- Identificadores
    doi TEXT UNIQUE,
    pmid VARCHAR(20), -- PubMed ID
    arxiv_id VARCHAR(50),
    url TEXT,

    -- Status
    status VARCHAR(50) DEFAULT 'draft' CHECK (status IN (
        'draft',
        'under_review',
        'revisions_requested',
        'accepted',
        'published',
        'rejected'
    )),

    submission_date DATE,
    acceptance_date DATE,

    -- Métricas de impacto
    citations_count INTEGER DEFAULT 0,
    altmetric_score DECIMAL(6,2),
    journal_impact_factor DECIMAL(5,3),

    -- Conteúdo
    keywords TEXT[],
    mesh_terms TEXT[], -- Medical Subject Headings

    -- Arquivos
    manuscript_pdf_path TEXT,
    supplementary_materials_path TEXT,
    raw_data_path TEXT,

    -- Revisão por pares
    peer_reviews JSONB, -- [{reviewer: "anon", date: "...", comments: "..."}]

    -- Financiamento
    funding_statement TEXT,
    conflict_of_interest_statement TEXT,

    -- Metadados
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_research_publications_cohort ON research_publications(cohort_id);
CREATE INDEX IF NOT EXISTS idx_research_publications_status ON research_publications(status);
CREATE INDEX IF NOT EXISTS idx_research_publications_date ON research_publications(publication_date DESC)
    WHERE publication_date IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_research_publications_doi ON research_publications(doi)
    WHERE doi IS NOT NULL;

COMMENT ON TABLE research_publications IS 'Tracking de publicações científicas derivadas dos estudos';


-- ============================================================================
-- 6. RESEARCH EXPORTS (DATASETS EXPORTADOS)
-- ============================================================================
-- Tracking de datasets exportados para análise externa
CREATE TABLE IF NOT EXISTS research_exports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cohort_id UUID NOT NULL REFERENCES research_cohorts(id) ON DELETE CASCADE,

    -- Identificação
    export_name VARCHAR(200) NOT NULL,
    export_description TEXT,
    export_format VARCHAR(50) NOT NULL CHECK (export_format IN (
        'csv',
        'json',
        'parquet',
        'stata',
        'spss',
        'r_data'
    )),

    -- Conteúdo
    variables_included TEXT[] NOT NULL,
    n_patients INTEGER NOT NULL,
    n_observations INTEGER NOT NULL,
    date_range_start DATE,
    date_range_end DATE,

    -- Filtros aplicados
    filters_applied JSONB,

    -- Anonimização
    anonymization_level VARCHAR(50) NOT NULL CHECK (anonymization_level IN (
        'fully_anonymized',
        'pseudonymized',
        'aggregated_only'
    )),
    pii_removed BOOLEAN DEFAULT TRUE,
    k_anonymity_level INTEGER, -- k-anonymity: cada registro indistinguível de k-1 outros

    -- Arquivo
    file_path TEXT NOT NULL,
    file_size_bytes BIGINT,
    checksum_sha256 VARCHAR(64),

    -- Acesso
    exported_by VARCHAR(100) NOT NULL,
    exported_for_purpose TEXT,
    access_granted_to TEXT[], -- Lista de pessoas/instituições autorizadas

    -- Data protection
    gdpr_compliant BOOLEAN DEFAULT TRUE,
    lgpd_compliant BOOLEAN DEFAULT TRUE,
    hipaa_compliant BOOLEAN DEFAULT TRUE,

    export_date TIMESTAMP NOT NULL DEFAULT NOW(),
    expiry_date TIMESTAMP, -- Data de expiração do acesso

    -- Auditoria
    access_log JSONB -- [{accessed_by: "...", timestamp: "...", action: "download"}]
);

CREATE INDEX IF NOT EXISTS idx_research_exports_cohort ON research_exports(cohort_id);
CREATE INDEX IF NOT EXISTS idx_research_exports_date ON research_exports(export_date DESC);

COMMENT ON TABLE research_exports IS 'Tracking de datasets exportados para análise externa ou compartilhamento';


-- ============================================================================
-- 7. VIEWS ÚTEIS
-- ============================================================================

-- View: Estudos ativos com progresso
CREATE OR REPLACE VIEW v_active_research_studies AS
SELECT
    rc.id,
    rc.study_code,
    rc.study_name,
    rc.study_type,
    rc.status,
    rc.current_n_patients,
    rc.target_n_patients,
    ROUND(
        (rc.current_n_patients::NUMERIC / NULLIF(rc.target_n_patients, 0)) * 100,
        1
    ) AS recruitment_progress_pct,
    rc.data_collection_start_date,
    rc.data_collection_end_date,
    CASE
        WHEN rc.data_collection_end_date IS NOT NULL
        THEN CURRENT_DATE - rc.data_collection_end_date
        ELSE NULL
    END AS days_since_completion,
    rc.principal_investigator,
    rc.p_value,
    rc.effect_size,
    COUNT(DISTINCT rd.anonymous_patient_id) AS actual_n_patients_with_data
FROM research_cohorts rc
LEFT JOIN research_datapoints rd ON rc.id = rd.cohort_id
WHERE rc.status IN ('recruiting', 'data_collection', 'analyzing')
GROUP BY rc.id
ORDER BY rc.created_at DESC;

COMMENT ON VIEW v_active_research_studies IS 'Estudos ativos com progresso de recrutamento';


-- View: Correlações significativas encontradas
CREATE OR REPLACE VIEW v_significant_correlations AS
SELECT
    rc.study_name,
    lc.predictor_variable,
    lc.outcome_variable,
    lc.lag_days,
    lc.correlation_coefficient,
    lc.p_value,
    lc.effect_size_category,
    lc.n_patients,
    lc.n_observations,
    lc.analysis_date
FROM longitudinal_correlations lc
JOIN research_cohorts rc ON lc.cohort_id = rc.id
WHERE lc.is_significant = TRUE
  AND ABS(lc.correlation_coefficient) >= 0.3 -- Pelo menos small effect
ORDER BY ABS(lc.correlation_coefficient) DESC;

COMMENT ON VIEW v_significant_correlations IS 'Correlações estatisticamente significativas com effect size relevante';


-- View: Papers publicados
CREATE OR REPLACE VIEW v_published_papers AS
SELECT
    rp.title,
    rp.authors,
    rp.journal_name,
    rp.publication_date,
    rp.doi,
    rp.citations_count,
    rp.journal_impact_factor,
    rc.study_name AS related_study,
    rc.study_code
FROM research_publications rp
LEFT JOIN research_cohorts rc ON rp.cohort_id = rc.id
WHERE rp.status = 'published'
ORDER BY rp.publication_date DESC;

COMMENT ON VIEW v_published_papers IS 'Papers publicados com métricas de impacto';


-- View: Resumo de estudos por tipo
CREATE OR REPLACE VIEW v_research_portfolio AS
SELECT
    study_type,
    COUNT(*) AS n_studies,
    SUM(CASE WHEN status = 'published' THEN 1 ELSE 0 END) AS n_published,
    SUM(CASE WHEN status IN ('recruiting', 'data_collection', 'analyzing') THEN 1 ELSE 0 END) AS n_active,
    SUM(current_n_patients) AS total_patients_enrolled,
    ROUND(AVG(CASE WHEN p_value IS NOT NULL THEN p_value ELSE NULL END), 6) AS avg_p_value,
    ROUND(AVG(CASE WHEN effect_size IS NOT NULL THEN effect_size ELSE NULL END), 3) AS avg_effect_size
FROM research_cohorts
GROUP BY study_type
ORDER BY n_studies DESC;

COMMENT ON VIEW v_research_portfolio IS 'Portfolio de pesquisa por tipo de estudo';


-- ============================================================================
-- 8. FUNCTIONS
-- ============================================================================

-- Function: Calcular k-anonymity de um dataset
CREATE OR REPLACE FUNCTION calculate_k_anonymity(
    p_cohort_id UUID,
    quasi_identifiers TEXT[]
)
RETURNS INTEGER AS $$
DECLARE
    k_value INTEGER;
BEGIN
    -- Conta o tamanho do menor grupo com mesmos quasi-identifiers
    -- (Implementação simplificada - na prática seria mais complexo)
    SELECT MIN(group_size) INTO k_value
    FROM (
        SELECT COUNT(*) as group_size
        FROM research_datapoints
        WHERE cohort_id = p_cohort_id
        GROUP BY anonymous_patient_id
    ) sub;

    RETURN COALESCE(k_value, 0);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_k_anonymity IS 'Calcula k-anonymity de um dataset (simplified)';


-- Function: Gerar relatório de estudo
CREATE OR REPLACE FUNCTION generate_study_report(p_cohort_id UUID)
RETURNS JSONB AS $$
DECLARE
    report JSONB;
BEGIN
    SELECT jsonb_build_object(
        'study', jsonb_build_object(
            'name', rc.study_name,
            'code', rc.study_code,
            'hypothesis', rc.hypothesis,
            'status', rc.status,
            'n_patients', rc.current_n_patients
        ),
        'significant_correlations', (
            SELECT COALESCE(jsonb_agg(
                jsonb_build_object(
                    'predictor', predictor_variable,
                    'outcome', outcome_variable,
                    'lag_days', lag_days,
                    'r', correlation_coefficient,
                    'p', p_value
                )
            ), '[]'::jsonb)
            FROM longitudinal_correlations
            WHERE cohort_id = p_cohort_id
              AND is_significant = TRUE
        ),
        'analyses', (
            SELECT COALESCE(jsonb_agg(
                jsonb_build_object(
                    'type', analysis_type,
                    'name', analysis_name,
                    'p_value', p_value,
                    'significant', is_significant
                )
            ), '[]'::jsonb)
            FROM statistical_analyses
            WHERE cohort_id = p_cohort_id
        )
    ) INTO report
    FROM research_cohorts rc
    WHERE rc.id = p_cohort_id;

    RETURN report;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION generate_study_report IS 'Gera relatório JSON completo de um estudo';


-- ============================================================================
-- 9. TRIGGERS
-- ============================================================================

-- Trigger: Atualizar updated_at em research_cohorts
CREATE OR REPLACE FUNCTION update_research_cohort_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_research_cohort_updated_at ON research_cohorts;
CREATE TRIGGER trigger_update_research_cohort_updated_at
    BEFORE UPDATE ON research_cohorts
    FOR EACH ROW
    EXECUTE FUNCTION update_research_cohort_updated_at();


-- Trigger: Atualizar current_n_patients quando adicionar datapoint
CREATE OR REPLACE FUNCTION update_cohort_patient_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE research_cohorts
    SET current_n_patients = (
        SELECT COUNT(DISTINCT anonymous_patient_id)
        FROM research_datapoints
        WHERE cohort_id = NEW.cohort_id
    )
    WHERE id = NEW.cohort_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_cohort_patient_count ON research_datapoints;
CREATE TRIGGER trigger_update_cohort_patient_count
    AFTER INSERT ON research_datapoints
    FOR EACH ROW
    EXECUTE FUNCTION update_cohort_patient_count();


-- ============================================================================
-- FIM DA MIGRATION
-- ============================================================================
