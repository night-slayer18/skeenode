package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"skeenode/pkg/models"
)

// --- Execution Handlers ---

// getExecution handles GET /api/v1/executions/:id
func (s *Server) getExecution(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid execution ID"})
		return
	}

	// Note: We'd need a GetExecution method in ExecutionStore
	// For now, return a placeholder response
	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"status": "API method requires GetExecution implementation",
	})
}

// cancelExecution handles POST /api/v1/executions/:id/cancel
func (s *Server) cancelExecution(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid execution ID"})
		return
	}

	// Note: Cancellation would require:
	// 1. Find the executor running this job
	// 2. Send a cancel signal via Redis pub/sub or direct RPC
	// 3. Update execution status to CANCELLED
	
	// For now, just update the status
	if err := s.execStore.UpdateResult(c.Request.Context(), id, models.ExecutionCancelled, -1, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel execution"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "execution cancelled",
		"id":      id,
	})
}

// --- Cluster Handlers ---

// listNodes handles GET /api/v1/cluster/nodes
func (s *Server) listNodes(c *gin.Context) {
	nodes, err := s.coordinator.GetActiveNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get nodes: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"count": len(nodes),
	})
}

// getLeader handles GET /api/v1/cluster/leader
func (s *Server) getLeader(c *gin.Context) {
	// Note: We'd need to store the election instance or query etcd directly
	// For now, return a placeholder
	c.JSON(http.StatusOK, gin.H{
		"leader": "scheduler-leader",
		"note":   "Full implementation requires election instance access",
	})
}
