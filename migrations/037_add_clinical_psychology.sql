-- Migration: Add clinical psychology features
-- Version: 037
-- Description: Tables for pediatric risk detection, clinical notes, person tracking, and silence alerts

-- 1. Risk Detections Table
CREATE TABLE IF NOT EXISTS risk_detections (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    session_id BIGINT,
    
    -- Statement analysis
    statement TEXT NOT NULL,
    risk_level VARCHAR(20) NOT NULL CHECK (risk_level IN ('NONE', 'LOW', 'MODERATE', 'HIGH', 'CRITICAL')),
    risk_score FLOAT NOT NULL CHECK (risk_score >= 0 AND risk_score <= 1),
    
    -- Detection details
    detected_metaphors JSONB DEFAULT '[]',
    contextual_factors JSONB DEFAULT '[]',
    age INT NOT NULL,
    recommended_action TEXT,
    
    created_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

CREATE INDEX idx_risk_detections_patient ON risk_detections(patient_id, created_at DESC);
CREATE INDEX idx_risk_detections_level ON risk_detections(risk_level) WHERE risk_level IN ('HIGH', 'CRITICAL');
CREATE INDEX idx_risk_detections_tenant ON risk_detections(tenant_id);

-- 2. Clinical Notes Table
CREATE TABLE IF NOT EXISTS clinical_notes (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    session_id BIGINT,
    
    -- Note content
    raw_statement TEXT NOT NULL,
    possible_meanings JSONB DEFAULT '[]',
    related_memories JSONB DEFAULT '[]',
    
    -- Analysis
    sentiment_delta FLOAT DEFAULT 0,
    alert_level INT DEFAULT 0 CHECK (alert_level >= 0 AND alert_level <= 3),
    clinical_themes JSONB DEFAULT '[]',
    recommended_focus TEXT,
    
    created_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

CREATE INDEX idx_clinical_notes_patient ON clinical_notes(patient_id, session_id);
CREATE INDEX idx_clinical_notes_alert ON clinical_notes(alert_level DESC) WHERE alert_level >= 2;
CREATE INDEX idx_clinical_notes_tenant ON clinical_notes(tenant_id);

-- 3. Person Mentions Table (for tracking people in child's life)
CREATE TABLE IF NOT EXISTS person_mentions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    session_id BIGINT,
    
    -- Person details
    person_neo4j_id VARCHAR(255),  -- Reference to Neo4j node
    name_used VARCHAR(255) NOT NULL,
    relationship VARCHAR(100),
    
    -- Sentiment
    sentiment FLOAT,  -- -1 to 1
    
    mentioned_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

CREATE INDEX idx_person_mentions_patient ON person_mentions(patient_id, mentioned_at DESC);
CREATE INDEX idx_person_mentions_neo4j ON person_mentions(person_neo4j_id);
CREATE INDEX idx_person_mentions_tenant ON person_mentions(tenant_id);

-- 4. Silence Alerts Table (for detecting topic disappearance)
CREATE TABLE IF NOT EXISTS silence_alerts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    
    -- Topic tracking
    topic VARCHAR(255) NOT NULL,
    expected_frequency FLOAT NOT NULL,  -- mentions per session
    actual_frequency FLOAT NOT NULL,
    sessions_silent INT DEFAULT 0,
    
    -- Alert details
    alert_level VARCHAR(20) DEFAULT 'LOW',
    first_detected TIMESTAMP DEFAULT NOW(),
    last_checked TIMESTAMP DEFAULT NOW(),
    resolved BOOLEAN DEFAULT FALSE,
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

CREATE INDEX idx_silence_alerts_patient ON silence_alerts(patient_id, resolved);
CREATE INDEX idx_silence_alerts_unresolved ON silence_alerts(resolved) WHERE NOT resolved;
CREATE INDEX idx_silence_alerts_tenant ON silence_alerts(tenant_id);

-- 5. Treatment Goals Table
CREATE TABLE IF NOT EXISTS treatment_goals (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    
    -- Goal details
    description TEXT NOT NULL,
    target_sessions INT DEFAULT 4,
    sessions_completed INT DEFAULT 0,
    
    -- Tracking
    related_themes JSONB DEFAULT '[]',
    progress_metrics JSONB DEFAULT '{}',  -- {"choro": [5,3,1,0]}
    progress_notes TEXT[],
    
    -- Status
    status VARCHAR(20) DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'COMPLETED', 'PAUSED', 'CANCELLED')),
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP,
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

CREATE INDEX idx_treatment_goals_patient ON treatment_goals(patient_id, status);
CREATE INDEX idx_treatment_goals_active ON treatment_goals(status) WHERE status = 'ACTIVE';
CREATE INDEX idx_treatment_goals_tenant ON treatment_goals(tenant_id);

-- 6. Development Timeline Table (milestones and regressions)
CREATE TABLE IF NOT EXISTS development_timeline (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    
    -- Event details
    event_type VARCHAR(50) NOT NULL CHECK (event_type IN ('MILESTONE', 'REGRESSION', 'CHANGE')),
    description TEXT NOT NULL,
    age_at_event INT,
    
    -- Context
    session_id BIGINT,
    related_theme VARCHAR(100),
    significance VARCHAR(20) DEFAULT 'MEDIUM' CHECK (significance IN ('LOW', 'MEDIUM', 'HIGH')),
    
    detected_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

CREATE INDEX idx_development_timeline_patient ON development_timeline(patient_id, detected_at DESC);
CREATE INDEX idx_development_timeline_type ON development_timeline(event_type);
CREATE INDEX idx_development_timeline_tenant ON development_timeline(tenant_id);

-- 7. Enable RLS for new tables
ALTER TABLE risk_detections ENABLE ROW LEVEL SECURITY;
ALTER TABLE clinical_notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE person_mentions ENABLE ROW LEVEL SECURITY;
ALTER TABLE silence_alerts ENABLE ROW LEVEL SECURITY;
ALTER TABLE treatment_goals ENABLE ROW LEVEL SECURITY;
ALTER TABLE development_timeline ENABLE ROW LEVEL SECURITY;

-- 8. Create RLS policies
CREATE POLICY tenant_isolation_risk_detections ON risk_detections
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_clinical_notes ON clinical_notes
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_person_mentions ON person_mentions
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_silence_alerts ON silence_alerts
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_treatment_goals ON treatment_goals
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_development_timeline ON development_timeline
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

-- 9. Add session tracking to conversations if not exists
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS session_number INT DEFAULT 1;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS psychologist_notes TEXT;
