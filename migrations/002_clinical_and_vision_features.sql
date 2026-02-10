-- Migration: Create clinical assessment and medication vision tables
-- Description: Tabelas para escalas clínicas (PHQ-9, GAD-7, C-SSRS), análise de voz e identificação visual de medicamentos
-- Created: 2026-01-24

-- ========================================
-- 1. MEDICATION VISUAL IDENTIFICATION
-- ========================================

-- Logs de tentativas de scan visual de medicamentos
CREATE TABLE IF NOT EXISTS medication_visual_logs (
    id SERIAL PRIMARY KEY,
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    session_id VARCHAR(100) NOT NULL UNIQUE,
    scan_status VARCHAR(50) NOT NULL CHECK (scan_status IN ('success', 'not_found', 'error', 'cancelled')),
    confidence_score DECIMAL(5,2), -- 0.00 a 100.00
    gemini_model_used VARCHAR(100) DEFAULT 'gemini-2.0-flash-exp',
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Resultados das identificações de medicamentos
CREATE TABLE IF NOT EXISTS medication_identifications (
    id SERIAL PRIMARY KEY,
    visual_log_id INTEGER NOT NULL REFERENCES medication_visual_logs(id) ON DELETE CASCADE,
    medication_name VARCHAR(255) NOT NULL,
    dosage VARCHAR(100),
    pharmaceutical_form VARCHAR(50), -- comprimido, cápsula, xarope, etc
    pill_color VARCHAR(100),
    manufacturer VARCHAR(255),
    confidence DECIMAL(5,2) NOT NULL, -- 0.00 a 100.00
    matched_medication_id INTEGER REFERENCES medicamentos(id), -- Se encontrou no banco
    safety_status VARCHAR(50) CHECK (safety_status IN ('safe', 'warning', 'dangerous', 'unknown')),
    safety_warnings JSONB, -- Array de alertas: ["Já tomou há 2h", "Overdose risk"]
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Índices para medication_visual_logs
CREATE INDEX IF NOT EXISTS idx_medication_visual_logs_patient ON medication_visual_logs(patient_id);
CREATE INDEX IF NOT EXISTS idx_medication_visual_logs_session ON medication_visual_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_medication_visual_logs_status ON medication_visual_logs(scan_status);
CREATE INDEX IF NOT EXISTS idx_medication_visual_logs_created ON medication_visual_logs(created_at DESC);

-- Índices para medication_identifications
CREATE INDEX IF NOT EXISTS idx_medication_identifications_log ON medication_identifications(visual_log_id);
CREATE INDEX IF NOT EXISTS idx_medication_identifications_matched ON medication_identifications(matched_medication_id);
CREATE INDEX IF NOT EXISTS idx_medication_identifications_safety ON medication_identifications(safety_status);
CREATE INDEX IF NOT EXISTS idx_medication_identifications_confidence ON medication_identifications(confidence DESC);

-- Comentários
COMMENT ON TABLE medication_visual_logs IS 'Logs de todas as tentativas de scan visual de medicamentos via Gemini Vision';
COMMENT ON TABLE medication_identifications IS 'Resultados detalhados das identificações de medicamentos';
COMMENT ON COLUMN medication_identifications.confidence IS 'Nível de confiança da identificação (0-100%)';
COMMENT ON COLUMN medication_identifications.safety_warnings IS 'Alertas de segurança em formato JSON';

-- ========================================
-- 2. CLINICAL ASSESSMENTS (PHQ-9, GAD-7, C-SSRS)
-- ========================================

-- Sessões de avaliação clínica
CREATE TABLE IF NOT EXISTS clinical_assessments (
    id SERIAL PRIMARY KEY,
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    assessment_type VARCHAR(50) NOT NULL CHECK (assessment_type IN ('PHQ-9', 'GAD-7', 'C-SSRS', 'MMSE', 'MoCA')),
    session_id VARCHAR(100) NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed', 'cancelled', 'timeout')),
    total_score INTEGER,
    severity_level VARCHAR(50), -- minimal, mild, moderate, moderately_severe, severe, critical
    trigger_phrase TEXT, -- Para C-SSRS: frase que disparou a avaliação
    priority VARCHAR(20) DEFAULT 'normal' CHECK (priority IN ('normal', 'high', 'CRITICAL')),
    clinical_interpretation TEXT, -- Interpretação automatizada do resultado
    alert_sent BOOLEAN DEFAULT false, -- Se alertou família/médico
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Respostas individuais de cada pergunta
CREATE TABLE IF NOT EXISTS clinical_assessment_responses (
    id SERIAL PRIMARY KEY,
    assessment_id INTEGER NOT NULL REFERENCES clinical_assessments(id) ON DELETE CASCADE,
    question_number INTEGER NOT NULL, -- 1 a 9 (PHQ-9), 1 a 7 (GAD-7), 1 a 6 (C-SSRS)
    question_text TEXT NOT NULL,
    response_value INTEGER, -- 0, 1, 2, 3 (escalas likert) ou 0/1 (C-SSRS sim/não)
    response_text TEXT, -- Resposta textual do paciente
    responded_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_question_per_assessment UNIQUE(assessment_id, question_number)
);

-- Índices para clinical_assessments
CREATE INDEX IF NOT EXISTS idx_clinical_assessments_patient ON clinical_assessments(patient_id);
CREATE INDEX IF NOT EXISTS idx_clinical_assessments_type ON clinical_assessments(assessment_type);
CREATE INDEX IF NOT EXISTS idx_clinical_assessments_status ON clinical_assessments(status);
CREATE INDEX IF NOT EXISTS idx_clinical_assessments_severity ON clinical_assessments(severity_level);
CREATE INDEX IF NOT EXISTS idx_clinical_assessments_priority ON clinical_assessments(priority);
CREATE INDEX IF NOT EXISTS idx_clinical_assessments_created ON clinical_assessments(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_clinical_assessments_completed ON clinical_assessments(completed_at DESC) WHERE completed_at IS NOT NULL;

-- Índices para clinical_assessment_responses
CREATE INDEX IF NOT EXISTS idx_clinical_responses_assessment ON clinical_assessment_responses(assessment_id);
CREATE INDEX IF NOT EXISTS idx_clinical_responses_question ON clinical_assessment_responses(question_number);

-- Comentários
COMMENT ON TABLE clinical_assessments IS 'Sessões de avaliação clínica (depressão, ansiedade, risco suicida)';
COMMENT ON TABLE clinical_assessment_responses IS 'Respostas individuais de cada pergunta das escalas clínicas';
COMMENT ON COLUMN clinical_assessments.trigger_phrase IS 'Frase do paciente que disparou avaliação (crucial para C-SSRS)';
COMMENT ON COLUMN clinical_assessments.priority IS 'CRITICAL = risco suicida detectado, requer ação imediata';

-- ========================================
-- 3. VOICE PROSODY ANALYSIS
-- ========================================

-- Análises de biomarcadores vocais
CREATE TABLE IF NOT EXISTS voice_prosody_analyses (
    id SERIAL PRIMARY KEY,
    patient_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    session_id VARCHAR(100) NOT NULL UNIQUE,
    analysis_type VARCHAR(50) NOT NULL CHECK (analysis_type IN ('depression', 'anxiety', 'parkinson', 'hydration', 'full')),
    audio_duration_seconds INTEGER NOT NULL,
    transcript TEXT, -- Transcrição do áudio
    gemini_model_used VARCHAR(100) DEFAULT 'gemini-2.5-flash-exp',

    -- Resultados da análise
    depression_risk_score DECIMAL(5,2), -- 0.00 a 100.00
    anxiety_risk_score DECIMAL(5,2),
    parkinson_risk_score DECIMAL(5,2),
    hydration_score DECIMAL(5,2),

    overall_assessment TEXT, -- Avaliação geral do Gemini
    clinical_flags JSONB, -- ["monotone_speech", "slow_articulation", "tremor_detected"]
    alert_sent BOOLEAN DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Features acústicas extraídas da voz
CREATE TABLE IF NOT EXISTS voice_prosody_features (
    id SERIAL PRIMARY KEY,
    analysis_id INTEGER NOT NULL REFERENCES voice_prosody_analyses(id) ON DELETE CASCADE,

    -- Features de pitch (tom/altura)
    pitch_mean DECIMAL(10,4), -- Hz
    pitch_std DECIMAL(10,4),
    pitch_min DECIMAL(10,4),
    pitch_max DECIMAL(10,4),

    -- Features de qualidade vocal
    jitter DECIMAL(10,6), -- Variação de período (%)
    shimmer DECIMAL(10,6), -- Variação de amplitude (%)
    hnr DECIMAL(10,4), -- Harmonic-to-Noise Ratio (dB)

    -- Features temporais
    speech_rate DECIMAL(10,4), -- Palavras por minuto
    pause_duration_mean DECIMAL(10,4), -- Duração média de pausas (s)
    pause_frequency DECIMAL(10,4), -- Pausas por minuto

    -- Indicadores específicos
    monotonicity_score DECIMAL(5,2), -- 0-100 (depressão)
    tremor_indicator DECIMAL(5,2), -- 0-100 (Parkinson)
    breathlessness_score DECIMAL(5,2), -- 0-100 (desidratação)

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Índices para voice_prosody_analyses
CREATE INDEX IF NOT EXISTS idx_voice_prosody_patient ON voice_prosody_analyses(patient_id);
CREATE INDEX IF NOT EXISTS idx_voice_prosody_type ON voice_prosody_analyses(analysis_type);
CREATE INDEX IF NOT EXISTS idx_voice_prosody_session ON voice_prosody_analyses(session_id);
CREATE INDEX IF NOT EXISTS idx_voice_prosody_created ON voice_prosody_analyses(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_voice_prosody_depression ON voice_prosody_analyses(depression_risk_score DESC) WHERE depression_risk_score IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_voice_prosody_anxiety ON voice_prosody_analyses(anxiety_risk_score DESC) WHERE anxiety_risk_score IS NOT NULL;

-- Índices para voice_prosody_features
CREATE INDEX IF NOT EXISTS idx_voice_features_analysis ON voice_prosody_features(analysis_id);
CREATE INDEX IF NOT EXISTS idx_voice_features_pitch_mean ON voice_prosody_features(pitch_mean);
CREATE INDEX IF NOT EXISTS idx_voice_features_jitter ON voice_prosody_features(jitter);
CREATE INDEX IF NOT EXISTS idx_voice_features_hnr ON voice_prosody_features(hnr);

-- Comentários
COMMENT ON TABLE voice_prosody_analyses IS 'Análises de biomarcadores vocais para detecção de condições mentais e físicas';
COMMENT ON TABLE voice_prosody_features IS 'Features acústicas detalhadas extraídas da voz (pitch, jitter, shimmer, HNR)';
COMMENT ON COLUMN voice_prosody_features.jitter IS 'Variação de período vocal (instabilidade) - indicador de Parkinson';
COMMENT ON COLUMN voice_prosody_features.shimmer IS 'Variação de amplitude vocal - indicador de qualidade vocal';
COMMENT ON COLUMN voice_prosody_features.hnr IS 'Razão harmônico-ruído - indicador de qualidade vocal geral';

-- ========================================
-- 4. TRIGGERS PARA UPDATED_AT
-- ========================================

-- Função genérica para atualizar updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger para clinical_assessments
DROP TRIGGER IF EXISTS update_clinical_assessments_updated_at ON clinical_assessments;
CREATE TRIGGER update_clinical_assessments_updated_at
    BEFORE UPDATE ON clinical_assessments
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ========================================
-- 5. VIEWS ÚTEIS
-- ========================================

-- View: Últimas avaliações clínicas por paciente
CREATE OR REPLACE VIEW v_latest_clinical_assessments AS
SELECT DISTINCT ON (patient_id, assessment_type)
    id,
    patient_id,
    assessment_type,
    total_score,
    severity_level,
    status,
    created_at,
    completed_at
FROM clinical_assessments
WHERE status = 'completed'
ORDER BY patient_id, assessment_type, completed_at DESC NULLS LAST;

-- View: Pacientes com risco crítico (últimas 24h)
CREATE OR REPLACE VIEW v_critical_risk_patients AS
SELECT
    ca.patient_id,
    i.nome AS patient_name,
    ca.assessment_type,
    ca.severity_level,
    ca.trigger_phrase,
    ca.created_at,
    ca.alert_sent
FROM clinical_assessments ca
JOIN idosos i ON ca.patient_id = i.id
WHERE ca.priority = 'CRITICAL'
  AND ca.created_at >= NOW() - INTERVAL '24 hours'
ORDER BY ca.created_at DESC;

-- View: Resumo de identificações de medicamentos (últimos 7 dias)
CREATE OR REPLACE VIEW v_medication_scan_summary AS
SELECT
    mvl.patient_id,
    COUNT(*) AS total_scans,
    COUNT(*) FILTER (WHERE mvl.scan_status = 'success') AS successful_scans,
    COUNT(*) FILTER (WHERE mvl.scan_status = 'not_found') AS not_found,
    AVG(mvl.confidence_score) FILTER (WHERE mvl.scan_status = 'success') AS avg_confidence,
    MAX(mvl.created_at) AS last_scan_at
FROM medication_visual_logs mvl
WHERE mvl.created_at >= NOW() - INTERVAL '7 days'
GROUP BY mvl.patient_id;

COMMENT ON VIEW v_latest_clinical_assessments IS 'Últimas avaliações clínicas completadas por paciente e tipo';
COMMENT ON VIEW v_critical_risk_patients IS 'Pacientes com risco crítico detectado nas últimas 24 horas';
COMMENT ON VIEW v_medication_scan_summary IS 'Resumo de scans de medicamentos dos últimos 7 dias por paciente';
