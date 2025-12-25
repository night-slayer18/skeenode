package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	HttpClient *http.Client
}

type PredictionRequest struct {
	JobID    string                 `json:"job_id"`
	Features map[string]interface{} `json:"features"`
}

type PredictionResponse struct {
	JobID              string  `json:"job_id"`
	FailureProbability float64 `json:"failure_probability"`
	Confidence         float64 `json:"confidence"`
	Decision           string  `json:"decision"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HttpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) PredictFailure(jobID string, features map[string]interface{}) (*PredictionResponse, error) {
	reqBody := PredictionRequest{
		JobID:    jobID,
		Features: features,
	}

	jsonValue, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.HttpClient.Post(fmt.Sprintf("%s/predict/failure", c.BaseURL), "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned status: %d", resp.StatusCode)
	}

	var prediction PredictionResponse
	if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
		return nil, err
	}

	return &prediction, nil
}
