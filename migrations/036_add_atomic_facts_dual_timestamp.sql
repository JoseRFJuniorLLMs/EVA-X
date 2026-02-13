-- Migration: Add atomic facts and dual timestamp support
-- Version: 036
-- Description: Implements atomic fact extraction and dual timestamp tracking

-- 1. Create atomic_facts table
CREATE TABLE IF NOT EXISTS atomic_facts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    patient_id BIGINT NOT NULL,
    
    -- Fact content
    content TEXT NOT NULL,
    confidence FLOAT NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    source VARCHAR(50) NOT NULL CHECK (source IN ('user_stated', 'inferred', 'observed', 'revised')),
    revisable BOOLEAN DEFAULT TRUE,
    
    -- Versioning
    version INT DEFAULT 1,
    previous_version_id BIGINT REFERENCES atomic_facts(id),
    
    -- Dual timestamp
    event_time TIMESTAMP NOT NULL,      -- When the fact occurred
    ingestion_time TIMESTAMP NOT NULL,  -- When it was recorded
    
    -- Metadata
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE
);

-- 2. Add indexes
CREATE INDEX idx_atomic_facts_patient ON atomic_facts(patient_id, event_time DESC);
CREATE INDEX idx_atomic_facts_tenant ON atomic_facts(tenant_id, patient_id);
CREATE INDEX idx_atomic_facts_source ON atomic_facts(source);
CREATE INDEX idx_atomic_facts_version ON atomic_facts(previous_version_id) WHERE previous_version_id IS NOT NULL;

-- 3. Add dual timestamp to existing tables
ALTER TABLE memories ADD COLUMN IF NOT EXISTS event_time TIMESTAMP;
ALTER TABLE memories ADD COLUMN IF NOT EXISTS ingestion_time TIMESTAMP DEFAULT NOW();

-- Backfill event_time from created_at for existing records
UPDATE memories SET event_time = created_at WHERE event_time IS NULL;
UPDATE memories SET ingestion_time = created_at WHERE ingestion_time IS NULL;

-- Make event_time NOT NULL after backfill
ALTER TABLE memories ALTER COLUMN event_time SET NOT NULL;

-- 4. Add importance_score to memories
ALTER TABLE memories ADD COLUMN IF NOT EXISTS importance_score FLOAT DEFAULT 0.5;
ALTER TABLE memories ADD COLUMN IF NOT EXISTS importance_updated_at TIMESTAMP;

CREATE INDEX idx_memories_importance ON memories(importance_score DESC);

-- 5. Create memory_access_log for tracking access patterns
CREATE TABLE IF NOT EXISTS memory_access_log (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    memory_id BIGINT NOT NULL,
    accessed_at TIMESTAMP DEFAULT NOW(),
    access_type VARCHAR(50) DEFAULT 'retrieval',
    
    FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
);

CREATE INDEX idx_memory_access_log_memory ON memory_access_log(memory_id, accessed_at DESC);
CREATE INDEX idx_memory_access_log_tenant ON memory_access_log(tenant_id);

-- 6. Create memory_connections for synaptic connections
CREATE TABLE IF NOT EXISTS memory_connections (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    source_memory_id BIGINT NOT NULL,
    target_memory_id BIGINT NOT NULL,
    
    -- Connection strength
    weight FLOAT DEFAULT 1.0,
    activation_count INT DEFAULT 0,
    
    -- Aging for pruning
    age INT DEFAULT 0,
    last_activation TIMESTAMP DEFAULT NOW(),
    
    created_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (source_memory_id) REFERENCES memories(id) ON DELETE CASCADE,
    FOREIGN KEY (target_memory_id) REFERENCES memories(id) ON DELETE CASCADE,
    
    UNIQUE(source_memory_id, target_memory_id)
);

CREATE INDEX idx_memory_connections_source ON memory_connections(source_memory_id);
CREATE INDEX idx_memory_connections_target ON memory_connections(target_memory_id);
CREATE INDEX idx_memory_connections_weak ON memory_connections(age DESC, activation_count ASC);

-- 7. Create fact_contradictions table
CREATE TABLE IF NOT EXISTS fact_contradictions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    fact1_id BIGINT NOT NULL,
    fact2_id BIGINT NOT NULL,
    confidence FLOAT NOT NULL,
    detected_at TIMESTAMP DEFAULT NOW(),
    resolved BOOLEAN DEFAULT FALSE,
    resolution_notes TEXT,
    
    FOREIGN KEY (fact1_id) REFERENCES atomic_facts(id) ON DELETE CASCADE,
    FOREIGN KEY (fact2_id) REFERENCES atomic_facts(id) ON DELETE CASCADE
);

CREATE INDEX idx_fact_contradictions_unresolved ON fact_contradictions(resolved) WHERE NOT resolved;

-- 8. Enable RLS for new tables
ALTER TABLE atomic_facts ENABLE ROW LEVEL SECURITY;
ALTER TABLE memory_access_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE memory_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE fact_contradictions ENABLE ROW LEVEL SECURITY;

-- 9. Create RLS policies
CREATE POLICY tenant_isolation_atomic_facts ON atomic_facts
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_memory_access ON memory_access_log
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_memory_connections ON memory_connections
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_fact_contradictions ON fact_contradictions
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);
