-- SPDX-License-Identifier: MIT

-- UptimeFoundry Schema Migration 002: Discovery Runs Table

CREATE TABLE IF NOT EXISTS discovery_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    adapter_type VARCHAR(64) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    resources_discovered INT NOT NULL DEFAULT 0,
    relationships_discovered INT NOT NULL DEFAULT 0,
    errors JSONB DEFAULT '[]'::jsonb,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_discovery_runs_env_time ON discovery_runs(environment_id, started_at DESC);
