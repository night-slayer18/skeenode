-- Skeenode Database Optimization Indexes
-- Run these on your PostgreSQL database for improved query performance

-- ============================================
-- JOBS TABLE INDEXES
-- ============================================

-- Index for scheduler polling: find active jobs ready to run
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_jobs_status_next_run 
ON jobs(status, next_run_at) 
WHERE status = 'ACTIVE';

-- Index for listing jobs by owner (multi-tenancy)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_jobs_owner_id 
ON jobs(owner_id);

-- Index for job type filtering
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_jobs_type 
ON jobs(type);

-- ============================================
-- EXECUTIONS TABLE INDEXES
-- ============================================

-- Index for execution status queries with completion time
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_status_completed 
ON executions(status, completed_at DESC);

-- Partial index for running executions (executor heartbeat checks)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_running 
ON executions(node_id, started_at) 
WHERE status = 'RUNNING';

-- Partial index for pending executions (queue management)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_pending 
ON executions(scheduled_at) 
WHERE status = 'PENDING';

-- Index for job execution history
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_job_id_scheduled 
ON executions(job_id, scheduled_at DESC);

-- Index for orphan detection (executions stuck on dead nodes)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_orphan_check 
ON executions(node_id, updated_at) 
WHERE status = 'RUNNING';

-- ============================================
-- DEPENDENCIES TABLE INDEXES
-- ============================================

-- Index for dependency resolution (find dependencies for a job)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dependencies_job_id 
ON dependencies(job_id);

-- Index for reverse lookup (find jobs depending on a specific job)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dependencies_depends_on 
ON dependencies(depends_on_job_id);

-- ============================================
-- COMPOSITE INDEXES FOR COMMON QUERIES
-- ============================================

-- Dashboard query: recent executions for a job
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_job_recent 
ON executions(job_id, created_at DESC) 
INCLUDE (status, exit_code, duration_ms);

-- Metrics query: execution success rate by job type
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_metrics 
ON executions(status, created_at) 
INCLUDE (job_id);

-- ============================================
-- STATISTICS
-- ============================================

-- Update statistics for query planner
ANALYZE jobs;
ANALYZE executions;
ANALYZE dependencies;

-- ============================================
-- OPTIONAL: PARTITIONING FOR LARGE TABLES
-- ============================================

-- If you have millions of executions, consider partitioning by month:
-- This is a template - uncomment and modify dates as needed

-- CREATE TABLE executions_partitioned (
--     LIKE executions INCLUDING ALL
-- ) PARTITION BY RANGE (created_at);
-- 
-- CREATE TABLE executions_y2024m12 PARTITION OF executions_partitioned
--     FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');
-- 
-- CREATE TABLE executions_y2025m01 PARTITION OF executions_partitioned
--     FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
