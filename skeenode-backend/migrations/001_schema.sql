-- ============================================
-- Skeenode Database Schema
-- Migration: 001_schema.sql
-- 
-- Tables are also created via GORM AutoMigrate,
-- but this explicit SQL allows for:
-- - Version-controlled schema changes
-- - Database-level constraints
-- - Production migrations without Go runtime
-- ============================================

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- JOBS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS jobs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    schedule        VARCHAR(100) NOT NULL,  -- cron expression
    command         TEXT NOT NULL,
    type            VARCHAR(20) NOT NULL DEFAULT 'SHELL',  -- SHELL, DOCKER, HTTP, KUBERNETES
    owner_id        VARCHAR(255),
    
    -- Retry policy stored as JSONB
    retry_policy    JSONB DEFAULT '{"max_retries": 3, "backoff_strategy": "exponential", "initial_interval": "1s", "max_interval": "5m"}',
    
    -- Resource constraints
    constraints     JSONB DEFAULT '{}',
    
    -- Status tracking
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE, PAUSED, DELETED
    next_run_at     TIMESTAMP WITH TIME ZONE,
    last_run_at     TIMESTAMP WITH TIME ZONE,
    
    -- Timestamps
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT chk_job_type CHECK (type IN ('SHELL', 'DOCKER', 'HTTP', 'KUBERNETES')),
    CONSTRAINT chk_job_status CHECK (status IN ('ACTIVE', 'PAUSED', 'DELETED'))
);

-- ============================================
-- EXECUTIONS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS executions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    
    -- Timing
    scheduled_at    TIMESTAMP WITH TIME ZONE NOT NULL,
    started_at      TIMESTAMP WITH TIME ZONE,
    completed_at    TIMESTAMP WITH TIME ZONE,
    
    -- Status
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING, RUNNING, SUCCESS, FAILED, CANCELLED
    exit_code       INTEGER,
    
    -- Execution context
    node_id         VARCHAR(255),       -- Which executor ran this
    job_command     TEXT,               -- Snapshot of command at execution time
    log_path        TEXT,               -- Path to logs (S3 or local)
    
    -- Duration in milliseconds
    duration_ms     BIGINT,
    
    -- Timestamps
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT chk_execution_status CHECK (status IN ('PENDING', 'RUNNING', 'SUCCESS', 'FAILED', 'CANCELLED'))
);

-- ============================================
-- DEPENDENCIES TABLE (DAG Support)
-- ============================================
CREATE TABLE IF NOT EXISTS dependencies (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id              UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    depends_on_job_id   UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    
    -- Prevent self-references
    CONSTRAINT chk_no_self_dependency CHECK (job_id != depends_on_job_id),
    
    -- Unique dependency pair
    CONSTRAINT uq_job_dependency UNIQUE (job_id, depends_on_job_id)
);

-- ============================================
-- TRIGGER: Auto-update updated_at
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_executions_updated_at
    BEFORE UPDATE ON executions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- INITIAL DATA (Optional)
-- ============================================
-- Insert a sample job for testing
-- INSERT INTO jobs (name, schedule, command, type) 
-- VALUES ('Sample Health Check', '*/5 * * * *', 'echo "Health check OK"', 'SHELL');
