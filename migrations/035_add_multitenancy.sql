-- Migration: Add tenant_id to all tables for multi-tenancy
-- Version: 035
-- Description: Implements row-level multi-tenancy isolation

-- 0. Create base tables if they don't exist (defensive)
CREATE TABLE IF NOT EXISTS patients (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    date_of_birth VARCHAR(50),
    gender VARCHAR(50),
    email VARCHAR(255),
    phone VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS conversations (
    id BIGSERIAL PRIMARY KEY,
    patient_id BIGINT REFERENCES patients(id),
    started_at TIMESTAMP DEFAULT NOW(),
    ended_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT REFERENCES conversations(id),
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS memories (
    id BIGSERIAL PRIMARY KEY,
    patient_id BIGINT REFERENCES patients(id),
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 1. Add tenant_id column to all existing tables
ALTER TABLE patients ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE memories ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE cognitive_load_state ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE ethical_boundary_state ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE ethical_boundary_events ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE interaction_loads ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE cognitive_decisions ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE redirection_protocols ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE family_notifications ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';

-- 2. Create composite indexes for performance
CREATE INDEX IF NOT EXISTS idx_patients_tenant_id ON patients(tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_conversations_tenant_id ON conversations(tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_messages_tenant_id ON messages(tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_memories_tenant_id ON memories(tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_cognitive_load_tenant_id ON cognitive_load_state(tenant_id, patient_id);
CREATE INDEX IF NOT EXISTS idx_ethical_boundary_tenant_id ON ethical_boundary_state(tenant_id, patient_id);

-- 3. Enable Row-Level Security (RLS)
ALTER TABLE patients ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE memories ENABLE ROW LEVEL SECURITY;
ALTER TABLE cognitive_load_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE ethical_boundary_state ENABLE ROW LEVEL SECURITY;

-- 4. Create RLS policies
-- Policy: Users can only see their tenant's data
CREATE POLICY tenant_isolation_patients ON patients
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_conversations ON conversations
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_messages ON messages
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_memories ON memories
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_cognitive ON cognitive_load_state
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

CREATE POLICY tenant_isolation_ethical ON ethical_boundary_state
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::VARCHAR);

-- 5. Create helper function to set tenant context
CREATE OR REPLACE FUNCTION set_tenant_context(p_tenant_id VARCHAR)
RETURNS VOID AS $$
BEGIN
    PERFORM set_config('app.current_tenant', p_tenant_id, false);
END;
$$ LANGUAGE plpgsql;

-- 6. Create tenants table
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    is_active BOOLEAN DEFAULT TRUE,
    max_patients INT DEFAULT 1000,
    max_storage_gb INT DEFAULT 100,
    
    -- Metadata
    contact_email VARCHAR(255),
    contact_phone VARCHAR(50),
    billing_plan VARCHAR(50) DEFAULT 'free',
    
    CONSTRAINT unique_tenant_name UNIQUE(name)
);

-- 7. Insert default tenant
INSERT INTO tenants (id, name, billing_plan)
VALUES ('default', 'Default Tenant', 'enterprise')
ON CONFLICT (id) DO NOTHING;

-- 8. Create audit log for tenant access
CREATE TABLE IF NOT EXISTS tenant_access_log (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(255),
    action VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50),
    resource_id BIGINT,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_tenant_access_log_tenant ON tenant_access_log(tenant_id, created_at DESC);
CREATE INDEX idx_tenant_access_log_resource ON tenant_access_log(resource_type, resource_id);
