package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"skeenode/pkg/coordination"
	"skeenode/pkg/storage"
)

// Server encapsulates the HTTP API server and its dependencies.
type Server struct {
	router      *gin.Engine
	httpServer  *http.Server
	
	jobStore    storage.JobStore
	execStore   storage.ExecutionStore
	depStore    storage.DependencyStore
	queue       storage.Queue
	coordinator coordination.Coordinator
}

// Config holds API server configuration.
type Config struct {
	Port        string
	JobStore    storage.JobStore
	ExecStore   storage.ExecutionStore
	DepStore    storage.DependencyStore
	Queue       storage.Queue
	Coordinator coordination.Coordinator
}

// NewServer creates a new API server with all dependencies.
func NewServer(cfg Config) *Server {
	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)
	
	router := gin.New()
	
	// Middleware
	router.Use(gin.Recovery())
	router.Use(requestLogger())
	
	s := &Server{
		router:      router,
		jobStore:    cfg.JobStore,
		execStore:   cfg.ExecStore,
		depStore:    cfg.DepStore,
		queue:       cfg.Queue,
		coordinator: cfg.Coordinator,
	}
	
	// Register routes
	s.registerRoutes()
	
	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	return s
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	log.Printf("[API] Starting server on %s", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("[API] Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// registerRoutes sets up all API endpoints.
func (s *Server) registerRoutes() {
	// Health check
	s.router.GET("/health", s.healthCheck)
	
	// Prometheus metrics endpoint
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Jobs
		jobs := v1.Group("/jobs")
		{
			jobs.POST("", s.createJob)
			jobs.GET("", s.listJobs)
			jobs.GET("/:id", s.getJob)
			jobs.PATCH("/:id", s.updateJob)
			jobs.DELETE("/:id", s.deleteJob)
			jobs.POST("/:id/trigger", s.triggerJob)
			jobs.GET("/:id/executions", s.listJobExecutions)
		}
		
		// Executions
		executions := v1.Group("/executions")
		{
			executions.GET("/:id", s.getExecution)
			executions.POST("/:id/cancel", s.cancelExecution)
		}
		
		// Cluster
		cluster := v1.Group("/cluster")
		{
			cluster.GET("/nodes", s.listNodes)
			cluster.GET("/leader", s.getLeader)
		}
	}
}

// requestLogger is a middleware that logs HTTP requests.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		
		c.Next()
		
		latency := time.Since(start)
		status := c.Writer.Status()
		
		log.Printf("[API] %s %s %d %v", c.Request.Method, path, status, latency)
	}
}

// healthCheck returns server health status.
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	})
}
