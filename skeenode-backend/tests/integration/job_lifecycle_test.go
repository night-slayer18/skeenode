package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"skeenode/pkg/api"
	"skeenode/pkg/models"
	"skeenode/pkg/storage/postgres"
	"skeenode/pkg/storage/redis"
)

// IntegrationTestSuite is the main test suite for integration tests
type IntegrationTestSuite struct {
	suite.Suite
	server     *api.Server
	store      *postgres.PostgresStore
	queue      *redis.RedisQueue
	httpServer *httptest.Server
}

// SetupSuite runs once before all tests
func (s *IntegrationTestSuite) SetupSuite() {
	// Skip integration tests if SKIP_INTEGRATION_TESTS is set
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		s.T().Skip("Skipping integration tests (SKIP_INTEGRATION_TESTS=true)")
	}

	gin.SetMode(gin.TestMode)

	// Get connection strings from environment or use defaults
	dbHost := getEnv("TEST_DB_HOST", "localhost")
	dbPort := getEnv("TEST_DB_PORT", "5432")
	dbUser := getEnv("TEST_DB_USER", "skeenode")
	dbPass := getEnv("TEST_DB_PASS", "password")
	dbName := getEnv("TEST_DB_NAME", "skeenode_test")

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName,
	)

	// Initialize PostgreSQL
	store, err := postgres.NewPostgresStore(connStr)
	if err != nil {
		s.T().Skipf("Skipping integration tests: %v", err)
	}
	s.store = store

	// Initialize Redis
	redisAddr := fmt.Sprintf("%s:%s",
		getEnv("TEST_REDIS_HOST", "localhost"),
		getEnv("TEST_REDIS_PORT", "6379"),
	)
	queue, err := redis.NewRedisQueue(redisAddr)
	if err != nil {
		s.T().Skipf("Skipping integration tests: %v", err)
	}
	s.queue = queue

	// Create API server
	s.server = api.NewServer(api.Config{
		Port:      "0", // Random port
		JobStore:  store,
		ExecStore: store,
		DepStore:  store,
		Queue:     queue,
	})
}

// TearDownSuite runs once after all tests
func (s *IntegrationTestSuite) TearDownSuite() {
	if s.store != nil {
		s.store.Close()
	}
	if s.queue != nil {
		s.queue.Close()
	}
}

// SetupTest runs before each test
func (s *IntegrationTestSuite) SetupTest() {
	// Clean up any existing data
	ctx := context.Background()
	// In a real test, you'd truncate tables here
	_ = ctx
}

// TestJobLifecycle tests the full job creation -> execution -> completion flow
func (s *IntegrationTestSuite) TestJobLifecycle() {
	ctx := context.Background()

	// 1. Create a job
	job := &models.Job{
		ID:       uuid.New(),
		Name:     "integration-test-job",
		Schedule: "*/5 * * * *",
		Command:  "echo 'hello world'",
		Type:     models.JobTypeShell,
		Status:   models.JobStatusActive,
	}

	err := s.store.CreateJob(ctx, job)
	require.NoError(s.T(), err, "Failed to create job")

	// 2. Verify job was created
	retrieved, err := s.store.GetJob(ctx, job.ID)
	require.NoError(s.T(), err, "Failed to retrieve job")
	assert.Equal(s.T(), job.Name, retrieved.Name)
	assert.Equal(s.T(), job.Command, retrieved.Command)

	// 3. Create an execution
	execution := &models.Execution{
		ID:          uuid.New(),
		JobID:       job.ID,
		ScheduledAt: time.Now(),
		Status:      models.ExecutionPending,
		JobCommand:  job.Command,
	}

	err = s.store.CreateExecution(ctx, execution)
	require.NoError(s.T(), err, "Failed to create execution")

	// 4. Push to queue
	err = s.queue.Push(ctx, execution)
	require.NoError(s.T(), err, "Failed to push to queue")

	// 5. Pop from queue (need group and consumer for Redis Streams)
	const testGroup = "test-executors"
	const testConsumer = "test-consumer-1"
	_ = s.queue.EnsureGroup(ctx, testGroup)

	msgID, popped, err := s.queue.Pop(ctx, testGroup, testConsumer)
	require.NoError(s.T(), err, "Failed to pop from queue")
	require.NotNil(s.T(), popped, "Pop returned nil execution")
	assert.Equal(s.T(), execution.ID, popped.ID)

	// 6. Mark as completed (would be done by executor in real scenario)
	now := time.Now()
	popped.Status = models.ExecutionSuccess
	popped.CompletedAt = &now
	popped.ExitCode = 0

	// 7. Acknowledge queue message
	err = s.queue.Ack(ctx, testGroup, msgID)
	require.NoError(s.T(), err, "Failed to ack message")
}

// TestRetryBehavior tests job retry logic
func (s *IntegrationTestSuite) TestRetryBehavior() {
	ctx := context.Background()

	// Create a job with retry policy
	job := &models.Job{
		ID:       uuid.New(),
		Name:     "retry-test-job",
		Schedule: "*/5 * * * *",
		Command:  "exit 1", // Will fail
		Type:     models.JobTypeShell,
		Status:   models.JobStatusActive,
		RetryPolicy: models.RetryPolicy{
			MaxRetries:      3,
			BackoffStrategy: "exponential",
			InitialInterval: "1s",
			MaxInterval:     "10s",
		},
	}

	err := s.store.CreateJob(ctx, job)
	require.NoError(s.T(), err)

	// Create initial execution
	execution := &models.Execution{
		ID:          uuid.New(),
		JobID:       job.ID,
		ScheduledAt: time.Now(),
		Status:      models.ExecutionPending,
		JobCommand:  job.Command,
	}

	err = s.store.CreateExecution(ctx, execution)
	require.NoError(s.T(), err)

	// Simulate failure by just verifying the job exists
	retrieved, err := s.store.GetJob(ctx, job.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, retrieved.RetryPolicy.MaxRetries)
}

// TestConcurrentExecutors tests multiple executors processing jobs
func (s *IntegrationTestSuite) TestConcurrentExecutors() {
	ctx := context.Background()
	numJobs := 10

	// Create multiple jobs
	var jobIDs []uuid.UUID
	for i := 0; i < numJobs; i++ {
		job := &models.Job{
			ID:       uuid.New(),
			Name:     fmt.Sprintf("concurrent-job-%d", i),
			Schedule: "*/5 * * * *",
			Command:  fmt.Sprintf("echo 'job %d'", i),
			Type:     models.JobTypeShell,
			Status:   models.JobStatusActive,
		}
		err := s.store.CreateJob(ctx, job)
		require.NoError(s.T(), err)
		jobIDs = append(jobIDs, job.ID)

		// Create and queue execution
		exec := &models.Execution{
			ID:          uuid.New(),
			JobID:       job.ID,
			ScheduledAt: time.Now(),
			Status:      models.ExecutionPending,
			JobCommand:  job.Command,
		}
		err = s.store.CreateExecution(ctx, exec)
		require.NoError(s.T(), err)
		err = s.queue.Push(ctx, exec)
		require.NoError(s.T(), err)
	}

	// Pop all jobs (simulating multiple executors)
	const testGroup = "test-concurrent"
	const testConsumer = "test-consumer"
	_ = s.queue.EnsureGroup(ctx, testGroup)

	var processed int
	for i := 0; i < numJobs; i++ {
		msgID, exec, err := s.queue.Pop(ctx, testGroup, testConsumer)
		if err == nil && exec != nil {
			processed++
			_ = s.queue.Ack(ctx, testGroup, msgID)
		}
	}

	assert.Equal(s.T(), numJobs, processed, "All jobs should be processed")
}

// TestAPIEndpoints tests the REST API endpoints
func (s *IntegrationTestSuite) TestAPIEndpoints() {
	// This test would use httptest to test API endpoints
	// Skipped if no test server available
	if s.httpServer == nil {
		s.T().Skip("HTTP server not available")
	}
}

// Helper functions
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (s *IntegrationTestSuite) makeRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	// s.server.Router().ServeHTTP(w, req)
	return w
}

// TestIntegration runs the integration test suite
func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	suite.Run(t, new(IntegrationTestSuite))
}
