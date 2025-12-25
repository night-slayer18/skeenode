package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Simple E2E test script to verify the system works
func main() {
	baseURL := "http://localhost:8080"
	fmt.Println("Starting E2E Test...")

	// 1. Create a Job
	jobID := uuid.New().String()
	jobName := fmt.Sprintf("test-job-%s", jobID[:8])

	job := map[string]interface{}{
		"name":     jobName,
		"command":  "echo 'Hello World'",
		"schedule": "* * * * *", // Every minute
		"type":     "SHELL",
		"owner_id": "tester",
	}

	jsonValue, _ := json.Marshal(job)
	resp, err := http.Post(baseURL+"/jobs", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Printf("❌ Failed to create job: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Failed to create job, status: %d\n", resp.StatusCode)
		return
	}
	fmt.Println("✅ Job created successfully.")

	// 2. Wait for execution (Scheduler needs to pick it up)
	// Since schedule is every minute, we might wait up to 60s.
	// To speed up, we might trigger manually if API supported it, but let's just check if job exists.

	// 3. List Jobs
	resp, err = http.Get(baseURL + "/jobs")
	if err != nil {
		fmt.Printf("❌ Failed to list jobs: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Failed to list jobs, status: %d\n", resp.StatusCode)
		return
	}
	fmt.Println("✅ Listed jobs successfully.")

	// 4. Verify AI Service (Health check)
	aiResp, err := http.Get("http://localhost:8000/health")
	if err != nil {
		fmt.Printf("❌ Failed to reach AI service: %v\n", err)
	} else {
		defer aiResp.Body.Close()
		if aiResp.StatusCode == http.StatusOK {
			fmt.Println("✅ AI Service is healthy.")
		} else {
			fmt.Printf("❌ AI Service returned status: %d\n", aiResp.StatusCode)
		}
	}

	fmt.Println("E2E Test Completed (Basic connectivity verification).")
}
