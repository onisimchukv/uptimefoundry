-- UptimeFoundry Schema Migration 001: Initial Relational, Property Graph, and Vector Schema

-- 1. Enable Core and Vector Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "ltree";
CREATE EXTENSION IF NOT EXISTS "vector";

-- 2. Enable Apache AGE (Property Graph) if available
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_available_extensions WHERE name = 'age'
    ) THEN
        CREATE EXTENSION IF NOT EXISTS "age";
        PERFORM create_graph('opsfoundry_graph');
    ELSE
        RAISE NOTICE 'Apache AGE extension not found in system libraries; falling back to relational adjacency graph';
    END IF;
EXCEPTION
    WHEN duplicate_table THEN
        NULL; -- graph already created
    WHEN OTHERS THEN
        RAISE NOTICE 'AGE initialization notice: %', SQLERRM;
END $$;

-- 3. Environments Table
CREATE TABLE IF NOT EXISTS environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL UNIQUE,
    description TEXT,
    environment_type VARCHAR(64) NOT NULL, -- 'production', 'staging', 'datacenter', 'hybrid'
    configuration JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Normalized Resources Table
CREATE TABLE IF NOT EXISTS resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    kind VARCHAR(64) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    external_id TEXT NOT NULL,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255),
    labels JSONB DEFAULT '{}'::jsonb,
    attributes JSONB DEFAULT '{}'::jsonb,
    status VARCHAR(64) NOT NULL DEFAULT 'unknown',
    source VARCHAR(64) NOT NULL,
    hierarchy_path ltree,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    stale_since TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_resource_identity UNIQUE (environment_id, kind, provider, external_id)
);

CREATE INDEX IF NOT EXISTS idx_resources_env_kind ON resources(environment_id, kind);
CREATE INDEX IF NOT EXISTS idx_resources_hierarchy ON resources USING GIST (hierarchy_path);
CREATE INDEX IF NOT EXISTS idx_resources_labels ON resources USING GIN (labels);

-- 5. Relationships Table (Relational adjacency + temporal tracking)
CREATE TABLE IF NOT EXISTS relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    source_resource_id UUID NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    target_resource_id UUID NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    relationship_type VARCHAR(64) NOT NULL,
    confidence FLOAT NOT NULL DEFAULT 1.0,
    evidence_source VARCHAR(64) NOT NULL, -- 'k8s_api', 'ebpf_socket', 'static_config'
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_to TIMESTAMPTZ,
    weight FLOAT DEFAULT 1.0, -- e.g. RTT in ms or bytes/sec
    attributes JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_relationships_active ON relationships (source_resource_id, target_resource_id)
WHERE valid_to IS NULL;

-- 6. Incidents Table
CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    severity VARCHAR(32) NOT NULL, -- 'critical', 'high', 'medium', 'low'
    status VARCHAR(32) NOT NULL DEFAULT 'open', -- 'open', 'investigating', 'mitigated', 'resolved', 'closed'
    source VARCHAR(64) NOT NULL,
    primary_resource_id UUID REFERENCES resources(id),
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    attributes JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_incidents_env_status ON incidents(environment_id, status);

-- 7. Investigations Table
CREATE TABLE IF NOT EXISTS investigations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL, -- 'running', 'completed', 'failed'
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    hypotheses JSONB DEFAULT '[]'::jsonb,
    evidence_bundle JSONB DEFAULT '{}'::jsonb,
    summary TEXT,
    confidence FLOAT,
    model_metadata JSONB DEFAULT '{}'::jsonb
);

-- 8. Remediation Proposals Table
CREATE TABLE IF NOT EXISTS remediations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    investigation_id UUID NOT NULL REFERENCES investigations(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    action_type VARCHAR(64) NOT NULL,
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'pending_approval',
    proposed_by VARCHAR(128) NOT NULL,
    approved_by VARCHAR(128),
    approved_at TIMESTAMPTZ,
    executed_at TIMESTAMPTZ,
    execution_result JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 9. Audit Events Table
CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor VARCHAR(128) NOT NULL,
    action VARCHAR(128) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128) NOT NULL,
    outcome VARCHAR(32) NOT NULL, -- 'success', 'failure', 'denied'
    details JSONB DEFAULT '{}'::jsonb,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp DESC);

-- 10. Operational Knowledge Documents Table (pgvector)
CREATE TABLE IF NOT EXISTS knowledge_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type VARCHAR(32) NOT NULL, -- 'runbook', 'postmortem', 'alert_doc', 'gitops_manifest'
    title VARCHAR(255) NOT NULL,
    path_or_url TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    embedding vector(768),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- HNSW Vector Index for Cosine Similarity Search
CREATE INDEX IF NOT EXISTS idx_knowledge_embeddings ON knowledge_documents 
USING hnsw (embedding vector_cosine_ops) 
WITH (m = 16, ef_construction = 64);

-- Full-Text Search Index for Hybrid Retrieval
CREATE INDEX IF NOT EXISTS idx_knowledge_fts ON knowledge_documents 
USING GIN (to_tsvector('english', title || ' ' || content));
