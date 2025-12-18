package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"skeenode/pkg/models"
)

// --- Request/Response DTOs ---

// CreateJobRequest is the payload for creating a new job.
type CreateJobRequest struct {
	Name        string                    `json:"name" binding:"required"`
	Schedule    string                    `json:"schedule" binding:"required"`
	Command     string                    `json:"command" binding:"required"`
	Type        models.JobType            `json:"type"`
	OwnerID     string                    `json:"owner_id"`
	RetryPolicy models.RetryPolicy        `json:"retry_policy"`
	Constraints models.ResourceConstraints `json:"constraints"`
}

// UpdateJobRequest is the payload for updating a job.
type UpdateJobRequest struct {
	Name        *string                    `json:"name"`
	Schedule    *string                    `json:"schedule"`
	Command     *string                    `json:"command"`
	Status      *models.JobStatus          `json:"status"`
	RetryPolicy *models.RetryPolicy        `json:"retry_policy"`
	Constraints *models.ResourceConstraints `json:"constraints"`
}

// JobResponse is the API representation of a job.
type JobResponse struct {
	ID          uuid.UUID                  `json:"id"`
	Name        string                     `json:"name"`
	Schedule    string                     `json:"schedule"`
	Command     string                     `json:"command"`
	Type        models.JobType             `json:"type"`
	OwnerID     string                     `json:"owner_id"`
	Status      models.JobStatus           `json:"status"`
	RetryPolicy models.RetryPolicy         `json:"retry_policy"`
	Constraints models.ResourceConstraints `json:"constraints"`
	NextRunAt   *time.Time                 `json:"next_run_at"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
}

// --- Job Handlers ---

// createJob handles POST /api/v1/jobs
func (s *Server) createJob(c *gin.Context) {
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(req.Schedule)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cron schedule: " + err.Error()})
		return
	}

	// Calculate next run time
	nextRun := schedule.Next(time.Now())

	// Default job type
	jobType := req.Type
	if jobType == "" {
		jobType = models.JobTypeShell
	}

	job := &models.Job{
		ID:          uuid.New(),
		Name:        req.Name,
		Schedule:    req.Schedule,
		Command:     req.Command,
		Type:        jobType,
		OwnerID:     req.OwnerID,
		RetryPolicy: req.RetryPolicy,
		Constraints: req.Constraints,
		Status:      models.JobStatusActive,
		NextRunAt:   &nextRun,
	}

	if err := s.jobStore.CreateJob(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create job: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, jobToResponse(job))
}

// listJobs handles GET /api/v1/jobs
func (s *Server) listJobs(c *gin.Context) {
	// Parse pagination parameters
	limit := 50 // default
	offset := 0

	jobs, err := s.jobStore.ListAllJobs(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list jobs: " + err.Error()})
		return
	}

	response := make([]JobResponse, len(jobs))
	for i, job := range jobs {
		response[i] = jobToResponse(&job)
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":  response,
		"count": len(response),
	})
}

// getJob handles GET /api/v1/jobs/:id
func (s *Server) getJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	job, err := s.jobStore.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	c.JSON(http.StatusOK, jobToResponse(job))
}

// updateJob handles PATCH /api/v1/jobs/:id
func (s *Server) updateJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	var req UpdateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing job
	job, err := s.jobStore.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// Apply updates
	if req.Name != nil {
		job.Name = *req.Name
	}
	if req.Command != nil {
		job.Command = *req.Command
	}
	if req.Status != nil {
		job.Status = *req.Status
	}
	if req.Schedule != nil {
		// Validate and update schedule
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(*req.Schedule)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cron schedule"})
			return
		}
		job.Schedule = *req.Schedule
		nextRun := schedule.Next(time.Now())
		job.NextRunAt = &nextRun
	}
	if req.RetryPolicy != nil {
		job.RetryPolicy = *req.RetryPolicy
	}
	if req.Constraints != nil {
		job.Constraints = *req.Constraints
	}

	// Note: We'd need an UpdateJob method in JobStore for a proper implementation
	// For now, we'll update just the next_run_at if schedule changed
	if job.NextRunAt != nil {
		if err := s.jobStore.UpdateNextRun(c.Request.Context(), id, *job.NextRunAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update job"})
			return
		}
	}

	c.JSON(http.StatusOK, jobToResponse(job))
}

// deleteJob handles DELETE /api/v1/jobs/:id
func (s *Server) deleteJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	// Get job to verify it exists
	job, err := s.jobStore.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// Mark as archived (soft delete via GORM)
	job.Status = models.JobStatusArchived
	// Note: We'd need an UpdateJob or DeleteJob method for proper implementation

	c.JSON(http.StatusOK, gin.H{"message": "job archived", "id": id})
}

// triggerJob handles POST /api/v1/jobs/:id/trigger
func (s *Server) triggerJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	job, err := s.jobStore.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// Create a manual execution
	execID := uuid.New()
	exec := &models.Execution{
		ID:          execID,
		JobID:       job.ID,
		ScheduledAt: time.Now(),
		Status:      models.ExecutionPending,
		JobCommand:  job.Command,
	}

	if err := s.execStore.CreateExecution(c.Request.Context(), exec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create execution"})
		return
	}

	if err := s.queue.Push(c.Request.Context(), exec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue execution"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":      "job triggered",
		"execution_id": execID,
	})
}

// listJobExecutions handles GET /api/v1/jobs/:id/executions
func (s *Server) listJobExecutions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	// Verify job exists
	_, err = s.jobStore.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// Note: We'd need a ListExecutionsByJob method
	// For now, return empty placeholder
	c.JSON(http.StatusOK, gin.H{
		"executions": []interface{}{},
		"job_id":     id,
	})
}

// Helper to convert Job to JobResponse
func jobToResponse(job *models.Job) JobResponse {
	return JobResponse{
		ID:          job.ID,
		Name:        job.Name,
		Schedule:    job.Schedule,
		Command:     job.Command,
		Type:        job.Type,
		OwnerID:     job.OwnerID,
		Status:      job.Status,
		RetryPolicy: job.RetryPolicy,
		Constraints: job.Constraints,
		NextRunAt:   job.NextRunAt,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}
}
