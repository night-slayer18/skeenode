package executor

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/mem"

	config "skeenode/configs"
	"skeenode/pkg/coordination"
	"skeenode/pkg/executor/runner"
	"skeenode/pkg/metrics"
	"skeenode/pkg/models"
	"skeenode/pkg/storage"
)

type Executor struct {
	ID        string
	Hostname  string
	
	// Resources
	TotalCPU  int
	TotalMem  uint64 // In MB
	
	coordinator coordination.Coordinator
	queue       storage.Queue
	execStore   storage.ExecutionStore
	interval    time.Duration
}

func NewExecutor(cfg *config.Config, coord coordination.Coordinator, queue storage.Queue, execStore storage.ExecutionStore) *Executor {
	hostname, _ := os.Hostname()
	id := fmt.Sprintf("%s-%s", hostname, uuid.New().String()[:8])

	return &Executor{
		ID:          id,
		Hostname:    hostname,
		TotalCPU:    runtime.NumCPU(),
		TotalMem:    detectTotalMemory(),
		coordinator: coord,
		queue:       queue,
		execStore:   execStore,
		interval:    5 * time.Second,
	}
}

func detectTotalMemory() uint64 {
	v, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("[Executor] Warning: Failed to detect memory: %v. Defaulting to 1GB.", err)
		return 1024
	}
	// Return in MB
	return v.Total / 1024 / 1024
}

// Start begins the executor's heartbeat and work loops.
func (e *Executor) Start(ctx context.Context) {
	log.Printf("[Executor %s] Starting up using %d CPUs...", e.ID, e.TotalCPU)

	// Ensure Consumer Group exists
	if err := e.queue.EnsureGroup(ctx, "skeenode-executors"); err != nil {
		log.Printf("[Executor] Warning: Failed to ensure consumer group: %v", err)
	}
	
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	// Start separate goroutine for heartbeat
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := e.RegisterHeartbeat(ctx); err != nil {
					log.Printf("[Executor] Heartbeat failed: %v", err)
				}
			}
		}
	}()

	// Main Work Loop
	log.Printf("[Executor] Waiting for jobs... (Concurrency: %d)", e.TotalCPU)

	// Worker Pool Semaphore
	sem := make(chan struct{}, e.TotalCPU)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Acquire token
			sem <- struct{}{}
			go func() {
				defer func() { <-sem }() // Release token
				e.consumeOne(ctx)
			}()
		}
	}
}

func (e *Executor) consumeOne(ctx context.Context) {
	// Pop job (blocks up to 2s)
	msgID, exec, err := e.queue.Pop(ctx, "skeenode-executors", e.ID)
	if err != nil {
		// Only log if it's not a timeout (redis nil)
		// Assuming Pop returns specific error or nil on timeout
		log.Printf("[Executor] Error popping job: %v", err)
		time.Sleep(1 * time.Second) // Backoff
		return
	}
	
	if exec == nil {
		// No job, sleep briefly to avoid spin loop if queue is empty but semaphore is full
		// Actually if exec is nil, we should return immediately so the worker is freed
		// But wait, if we return immediately, the loop will spin acquiring/releasing tokens.
		// So we should sleep a bit if queue is empty.
		time.Sleep(1 * time.Second)
		return
	}

	metrics.ExecutorJobsRunning.Inc()
	defer metrics.ExecutorJobsRunning.Dec()

	log.Printf("[Executor] 🚀 Received Job %s (Exec: %s) Cmd: %s", exec.JobID, exec.ID, exec.JobCommand)

	// 1. Report RUNNING state
	if err := e.execStore.UpdateRunState(ctx, exec.ID, e.ID, time.Now()); err != nil {
		log.Printf("[Executor] Failed to report run state: %v", err)
		// We continue anyway, but ideally we might retry or fail
	}

	// Execute Job
	r := runner.NewShellRunner()
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	result := r.Run(runCtx, "sh", []string{"-c", exec.JobCommand})

	log.Printf("[Executor] ✅ Finished Job %s. ExitCode: %d, Duration: %s", exec.JobID, result.ExitCode, result.Duration)
	if result.ExitCode != 0 {
		log.Printf("[Executor] ⚠️ Job Failed. Stderr: %s", result.Stderr)
	}

	// 2. Report Result
	status := models.ExecutionSuccess
	if result.ExitCode != 0 {
		status = models.ExecutionFailed
	}
	
	// Record metrics
	metrics.RecordExecution("", string(models.JobTypeShell), string(status), result.Duration.Seconds())
	
	// Logs Handling - writes to local path (use storage.S3LogStore for S3 in production)
	logPath := fmt.Sprintf("/tmp/skeenode-%s.log", exec.ID)
	// Combine logs
	fullLog := fmt.Sprintf("STDOUT:\n%s\nSTDERR:\n%s", result.Stdout, result.Stderr)
	_ = os.WriteFile(logPath, []byte(fullLog), 0644)

	if err := e.execStore.UpdateResult(ctx, exec.ID, status, result.ExitCode, logPath); err != nil {
		log.Printf("[Executor] Failed to report result: %v", err)
	}

	// Ack
	if err := e.queue.Ack(ctx, "skeenode-executors", msgID); err != nil {
		log.Printf("[Executor] Failed to ack job: %v", err)
	}
}

// RegisterHeartbeat updates the node status in Etcd/Redis.
func (e *Executor) RegisterHeartbeat(ctx context.Context) error {
	// TTL of 10 seconds, heartbeat every 5s (safe margin)
	err := e.coordinator.RegisterNode(ctx, e.ID, 10)
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}
	log.Printf("[Executor] Heartbeat sent (ID: %s)", e.ID)
	metrics.HeartbeatsSent.Inc()
	return nil
}
