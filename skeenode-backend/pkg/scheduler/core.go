package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	config "skeenode/configs"
	"skeenode/pkg/coordination"
	"skeenode/pkg/models"
	"skeenode/pkg/storage"
)

type Core struct {
	store       storage.JobStore
	execStore   storage.ExecutionStore
	queue       storage.Queue
	coordinator coordination.Coordinator
	parser      cron.Parser
	
	interval    time.Duration
}

func NewCore(cfg *config.Config, store storage.JobStore, execStore storage.ExecutionStore, queue storage.Queue, coord coordination.Coordinator) *Core {
	interval, _ := time.ParseDuration(cfg.SchedulerInterval)
	if interval == 0 {
		interval = 10 * time.Second
	}

	return &Core{
		store:       store,
		execStore:   execStore,
		queue:       queue,
		coordinator: coord,
		parser:      cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		interval:    interval,
	}
}

// Run starts the main scheduler loop.
// It blocks until the context is cancelled.
func (c *Core) Run(ctx context.Context, election coordination.Election) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	
	reconcileTicker := time.NewTicker(30 * time.Second)
	defer reconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[Scheduler] Shutting down...")
			return
		case <-ticker.C:
			// 1. Verify we are still leader
			leader, err := election.Leader(ctx)
			if err != nil {
				log.Printf("[Scheduler] Error checking leadership: %v", err)
				continue
			}
			// Note: strict check should match our ID, simplifying for Day 4 MVP
			_ = leader 

			// 2. Poll and Scheduling Logic
			if err := c.PollAndSchedule(ctx); err != nil {
				log.Printf("[Scheduler] Error in schedule loop: %v", err)
			}

		case <-reconcileTicker.C:
			// 3. Reconcile Loop (The Reaper)
			// Runs every 30 seconds
			// 1. Verify leadership (Reaper must be leader too)
			leader, err := election.Leader(ctx)
			if err != nil {
				continue
			}
			_ = leader 

			if err := c.Reconcile(ctx); err != nil {
				log.Printf("[Scheduler] Error in reconcile loop: %v", err)
			}
		}
	}
}

// Reconcile checks for dead nodes and orphans.
func (c *Core) Reconcile(ctx context.Context) error {
	// A. Get Active Nodes
	nodes, err := c.coordinator.GetActiveNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active nodes: %w", err)
	}
	
	// Fast path: if nodes exist, good. If 0 nodes, everything is an orphan.
	
	// B. Mark Orphans as FAILED
	count, err := c.execStore.MarkOrphansAsFailed(ctx, nodes)
	if err != nil {
		return fmt.Errorf("failed to reap orphans: %w", err)
	}
	
	if count > 0 {
		log.Printf("[Scheduler] 💀 Reaped %d orphaned executions from dead nodes", count)
	}

	// C. Smart Retries
	if err := c.RetryFailures(ctx); err != nil {
		log.Printf("[Scheduler] Error retrying failures: %v", err)
	}

	return nil
}

// RetryFailures finds recently failed jobs and reschedules them if policy allows.
func (c *Core) RetryFailures(ctx context.Context) error {
	// 1. Find recent failures (last 2 minutes)
	// We look back slightly longer than interval to ensure we don't miss any
	since := time.Now().Add(-2 * time.Minute)
	failures, err := c.execStore.ListRecentFailures(ctx, since, 20)
	if err != nil {
		return err
	}

	for _, failure := range failures {
		// Optimization: Check if already retried?
		// For MVP, we don't have a "HasRetried" link. 
		// We rely on "Pending" execution creation. If a pending execution exists for this job, we skip.
		// BUT, simpler logic: Only retry if "failed" and "not retried yet".
		// We can't easily check "not retried yet" without querying.
		// PRO FIX: We need idempotency.
		// For now, we assume if we process it soon enough, we are good.
		// Real impl would have "RetriedExecutionID" on the failed record.
		
		job, err := c.store.GetJob(ctx, failure.JobID)
		if err != nil {
			log.Printf("[Scheduler] Failed to get job for retry check: %v", err)
			continue
		}

		if failure.Attempt >= job.RetryPolicy.MaxRetries {
			continue // Exhausted retries
		}

		// Calculate Backoff (Exponential: 2^attempt * initial)
		// MVP: Fixed 10s backoff
		nextRun := time.Now().Add(10 * time.Second)

		// Create Retry Execution
		retryID := uuid.New()
		retryExec := &models.Execution{
			ID:          retryID,
			JobID:       job.ID,
			// Important: increment attempt
			Attempt:     failure.Attempt + 1,
			ScheduledAt: nextRun,
			Status:      models.ExecutionPending,
			JobCommand:  job.Command,
		}

		// Persist & Push
		// Note: This creates a race where we might retry same failure twice if loop runs fast.
		// Ideally we mark 'failure' as 'retried'.
		if err := c.execStore.CreateExecution(ctx, retryExec); err != nil {
			log.Printf("[Scheduler] Failed to schedule retry: %v", err)
			continue
		}
		
		if err := c.queue.Push(ctx, retryExec); err != nil {
			log.Printf("[Scheduler] Failed to push retry: %v", err)
		}
		
		log.Printf("[Scheduler] 🔄 Scheduled Retry %d/%d for Job %s (Exec: %s)", 
			retryExec.Attempt, job.RetryPolicy.MaxRetries, job.Name, retryID)
	}
	return nil
}

// PollAndSchedule fetches due jobs and dispatches them.
func (c *Core) PollAndSchedule(ctx context.Context) error {
	// A. Find jobs that are due (NextRunAt <= Now)
	jobs, err := c.store.ListDueJobs(ctx, 50) // Batch size 50
	if err != nil {
		return fmt.Errorf("failed to list due jobs: %w", err)
	}

	if len(jobs) == 0 {
		return nil
	}

	log.Printf("[Scheduler] Found %d jobs due for execution", len(jobs))

	now := time.Now()

	for _, job := range jobs {
		// B. Dispatch to Queue
		execID := uuid.New()
		exec := &models.Execution{
			ID:          execID,
			JobID:       job.ID,
			ScheduledAt: *job.NextRunAt,
			Status:      models.ExecutionPending,
			JobCommand:  job.Command,
		}

		// Transactional safety would be ideal here (Outbox pattern), 
		// but for Week 1 MVP we do: DB Write -> Redis Push
		
		// 1. Record Execution in DB
		if err := c.execStore.CreateExecution(ctx, exec); err != nil {
			log.Printf("[Scheduler] Failed to create execution for job %s: %v", job.ID, err)
			continue
		}

		// 2. Push to Redis Stats
		if err := c.queue.Push(ctx, exec); err != nil {
			log.Printf("[Scheduler] Failed to push execution for job %s: %v", job.ID, err)
			// TODO: Mark execution as FAILED in DB?
			continue
		}

		// C. Calculate Next Run Time
		schedule, err := c.parser.Parse(job.Schedule)
		if err != nil {
			log.Printf("[Scheduler] Invalid cron schedule for job %s: %v", job.ID, err)
			continue
		}
		
		nextRun := schedule.Next(now)
		
		// D. Update Job (Move to future)
		if err := c.store.UpdateNextRun(ctx, job.ID, nextRun); err != nil {
			log.Printf("[Scheduler] Failed to update next run for job %s: %v", job.ID, err)
		}
		
		log.Printf("[Scheduler] Dispatched Job %s (Exec: %s). Next run: %s", job.Name, execID, nextRun)
	}

	return nil
}
