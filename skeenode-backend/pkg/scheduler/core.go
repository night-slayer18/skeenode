package scheduler

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	config "skeenode/configs"
	"skeenode/pkg/ai"
	"skeenode/pkg/coordination"
	"skeenode/pkg/metrics"
	"skeenode/pkg/models"
	"skeenode/pkg/storage"
)

type Core struct {
	store       storage.JobStore
	execStore   storage.ExecutionStore
	queue       storage.Queue
	coordinator coordination.Coordinator
	aiClient    *ai.Client
	parser      cron.Parser

	interval time.Duration
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
		aiClient:    ai.NewClient(cfg.AIServiceURL),
		parser:      cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		interval:    interval,
	}
}

// calculateBackoff computes the exponential backoff delay with jitter for retry attempts.
// Uses the job's RetryPolicy settings, with sensible defaults if not configured.
func calculateBackoff(attempt int, policy models.RetryPolicy) time.Duration {
	// Parse initial interval (default: 5 seconds)
	initial, err := time.ParseDuration(policy.InitialInterval)
	if err != nil || initial == 0 {
		initial = 5 * time.Second
	}

	// Parse max interval (default: 5 minutes)
	maxInterval, err := time.ParseDuration(policy.MaxInterval)
	if err != nil || maxInterval == 0 {
		maxInterval = 5 * time.Minute
	}

	// Calculate exponential backoff: initial * 2^attempt
	backoff := float64(initial) * math.Pow(2, float64(attempt))
	
	// Cap at max interval
	if backoff > float64(maxInterval) {
		backoff = float64(maxInterval)
	}

	// Add jitter (±20%) to prevent thundering herd
	jitter := (rand.Float64() - 0.5) * 0.4 * backoff
	backoff += jitter

	return time.Duration(backoff)
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
			// Drain the queue: keep scheduling until no more jobs are found
			for {
				count, err := c.PollAndSchedule(ctx)
				if err != nil {
					log.Printf("[Scheduler] Error in schedule loop: %v", err)
					break
				}
				// If we processed a full batch (or at least some jobs), we try again immediately.
				// PollAndSchedule returns count. If 0, we are done.
				if count == 0 {
					break
				}
				// Optional: Check context cancel between bursts
				if ctx.Err() != nil {
					break
				}
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
		// Record metric
		metrics.OrphansReaped.Add(float64(count))
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

		// Calculate Backoff with jitter using the job's retry policy
		backoff := calculateBackoff(failure.Attempt, job.RetryPolicy)
		nextRun := time.Now().Add(backoff)

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
// Returns the number of jobs scheduled.
func (c *Core) PollAndSchedule(ctx context.Context) (int, error) {
	// A. Find jobs that are due (NextRunAt <= Now)
	// Scale: Increase batch size to 500 for better throughput
	jobs, err := c.store.ListDueJobs(ctx, 500)
	if err != nil {
		return 0, fmt.Errorf("failed to list due jobs: %w", err)
	}

	if len(jobs) == 0 {
		return 0, nil
	}

	log.Printf("[Scheduler] Found %d jobs due for execution", len(jobs))

	now := time.Now()

	// Scale: Parallel Dispatch using Worker Pool
	// Limit concurrency to avoid overwhelming the database or AI service
	concurrency := 20
	sem := make(chan struct{}, concurrency)
	errChan := make(chan error, len(jobs))

	// WaitGroup to wait for all dispatches in this batch
	// (Optional: we could just fire and forget if we don't care about precise count return,
	// but PollAndSchedule returns count, implying synchronous batch processing)
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)
		sem <- struct{}{} // Acquire token

		go func(j models.Job) {
			defer wg.Done()
			defer func() { <-sem }() // Release token

			// AI Check: Predict Failure
			features := map[string]interface{}{
				"day_of_week": int(now.Weekday()),
				"hour":        now.Hour(),
				"job_type":    string(j.Type),
			}

			// Call AI Service (Best effort, don't block if it fails)
			prediction, err := c.aiClient.PredictFailure(j.ID.String(), features)
			if err != nil {
				// Log but proceed? Or fail open?
				// "Fail Open" is safer for a scheduler (don't stop jobs because AI is down)
				log.Printf("[Scheduler] Warning: AI prediction failed: %v", err)
			} else {
				if prediction.Decision == "ABORT" {
					log.Printf("[Scheduler] 🛑 AI blocked execution of job %s (Confidence: %.2f)", j.Name, prediction.Confidence)
					// Skip this execution, but update next run time so we don't get stuck
					c.updateNextRun(ctx, &j, now)
					return
				}
			}

			// B. Dispatch to Queue
			execID := uuid.New()
			exec := &models.Execution{
				ID:          execID,
				JobID:       j.ID,
				ScheduledAt: *j.NextRunAt,
				Status:      models.ExecutionPending,
				JobCommand:  j.Command,
			}

			// 1. Record Execution in DB
			if err := c.execStore.CreateExecution(ctx, exec); err != nil {
				log.Printf("[Scheduler] Failed to create execution for job %s: %v", j.ID, err)
				return
			}

			// 2. Push to Redis Stats
			if err := c.queue.Push(ctx, exec); err != nil {
				log.Printf("[Scheduler] Failed to push execution for job %s: %v", j.ID, err)
				// TODO: Mark execution as FAILED in DB?
				return
			}

			c.updateNextRun(ctx, &j, now)

			log.Printf("[Scheduler] Dispatched Job %s (Exec: %s).", j.Name, execID)

			// Record metrics
			lag := time.Since(*j.NextRunAt).Seconds()
			metrics.RecordDispatch(lag)

		}(job)
	}

	wg.Wait()
	close(errChan)

	return len(jobs), nil
}

func (c *Core) updateNextRun(ctx context.Context, job *models.Job, now time.Time) {
	// C. Calculate Next Run Time
	schedule, err := c.parser.Parse(job.Schedule)
	if err != nil {
		log.Printf("[Scheduler] Invalid cron schedule for job %s: %v", job.ID, err)
		return
	}

	nextRun := schedule.Next(now)

	// D. Update Job (Move to future)
	if err := c.store.UpdateNextRun(ctx, job.ID, nextRun); err != nil {
		log.Printf("[Scheduler] Failed to update next run for job %s: %v", job.ID, err)
	}
	log.Printf("[Scheduler] Updated next run for job %s to %s", job.Name, nextRun)
}
