-- Migration: Add crisis protocol, session synthesis, and family tracking
-- Version: 038
-- Description: Tables for crisis events, session syntheses, and family changes

-- 1. Crisis Events Table
CREATE TABLE IF NOT EXISTS crisis_events (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    session_id BIGINT,
    
    -- Crisis details
    crisis_type VARCHAR(50) NOT NULL CHECK (crisis_type IN ('abuse', 'self_harm', 'neglect', 'violence', 'other')),
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('MODERATE', 'HIGH', 'CRITICAL')),
    trigger_statement TEXT NOT NULL,
    
    -- Response
    response_actions JSONB DEFAULT '{}',
    notifications_sent JSONB DEFAULT '{}',
    
    -- Acknowledgment
    acknowledged_by BIGINT,  -- psychologist_id
    acknowledged_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

CREATE INDEX idx_crisis_events_patient ON crisis_events(patient_id, created_at DESC);
CREATE INDEX idx_crisis_events_unacknowledged ON crisis_events(acknowledged_by) WHERE acknowledged_by IS NULL;
CREATE INDEX idx_crisis_events_severity ON crisis_events(severity) WHERE severity IN ('HIGH', 'CRITICAL');
CREATE INDEX idx_crisis_events_tenant ON crisis_events(tenant_id);

-- 2. Session Syntheses Table
CREATE TABLE IF NOT EXISTS session_syntheses (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    session_id BIGINT NOT NULL,
    
    -- Synthesis content
    main_themes JSONB DEFAULT '[]',
    alerts JSONB DEFAULT '[]',
    treatment_progress JSONB DEFAULT '[]',
    risk_summary JSONB DEFAULT '{}',
    suggestions TEXT[] DEFAULT '{}',
    
    generated_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE,
    FOREIGN KEY (session_id) REFERENCES conversations(id) ON DELETE CASCADE,
    
    UNIQUE(session_id)  -- One synthesis per session
);

CREATE INDEX idx_session_syntheses_patient ON session_syntheses(patient_id, generated_at DESC);
CREATE INDEX idx_session_syntheses_session ON session_syntheses(session_id);
CREATE INDEX idx_session_syntheses_tenant ON session_syntheses(tenant_id);

-- 3. Family Changes Table
CREATE TABLE IF NOT EXISTS family_changes (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    
    -- Change details
    change_type VARCHAR(50) NOT NULL CHECK (change_type IN ('NEW_PERSON', 'PERSON_LEFT', 'RELATIONSHIP_CHANGE')),
    person_neo4j_id VARCHAR(255),  -- Reference to Neo4j node
    person_name VARCHAR(255) NOT NULL,
    relationship VARCHAR(100),
    
    detected_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

CREATE INDEX idx_family_changes_patient ON family_changes(patient_id, detected_at DESC);
CREATE INDEX idx_family_changes_type ON family_changes(change_type);
CREATE INDEX idx_family_changes_tenant ON family_changes(tenant_id);

-- 4. Add locked field to conversations (for crisis protocol)
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS locked BOOLEAN DEFAULT FALSE;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP;

CREATE INDEX idx_conversations_locked ON conversations(locked) WHERE locked = TRUE;

-- 5. Enable RLS for new tables
ALTER TABLE crisis_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE session_syntheses ENABLE ROW LEVEL SECURITY;
ALTER TABLE family_changes ENABLE ROW LEVEL SECURITY;

-- 6. Create RLS policies
CREATE POLICY tenant_isolation_crisis_events ON crisis_events
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_session_syntheses ON session_syntheses
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_family_changes ON family_changes
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);
