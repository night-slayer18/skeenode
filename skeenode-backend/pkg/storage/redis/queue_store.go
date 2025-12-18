package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"skeenode/pkg/models"

	"github.com/redis/go-redis/v9"
)

const (
	StreamKeyPending = "jobs:queue:pending"
)

type RedisQueue struct {
	client *redis.Client
}

// NewRedisQueue initializes a new Redis client.
func NewRedisQueue(addr string) (*RedisQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Ping to verify connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisQueue{client: client}, nil
}

func (r *RedisQueue) Close() error {
	return r.client.Close()
}

// Push adds an execution payload to the pending stream.
func (r *RedisQueue) Push(ctx context.Context, exec *models.Execution) error {
	// Serialize payload
	payload, err := json.Marshal(exec)
	if err != nil {
		return fmt.Errorf("failed to marshal execution: %w", err)
	}

	// XADD jobs:queue:pending * payload {json}
	err = r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamKeyPending,
		Values: map[string]interface{}{
			"payload": payload,
			"job_id":  exec.JobID.String(),
			"exec_id": exec.ID.String(),
		},
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to push to queue: %w", err)
	}
	return nil
}

// TODO: Implement Pop/ConsumerGroup logic for Executor
