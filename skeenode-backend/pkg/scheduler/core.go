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
		}
	}
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
