package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"skeenode/pkg/models"

	"github.com/redis/go-redis/v9"
)

const (
	StreamKeyPending = "jobs:queue:pending"
)

type RedisQueue struct {
	client *redis.Client
}

// RedisQueueConfig holds Redis connection configuration
type RedisQueueConfig struct {
	Addr         string
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
}

// DefaultRedisQueueConfig returns enterprise-grade defaults
func DefaultRedisQueueConfig(addr string) RedisQueueConfig {
	return RedisQueueConfig{
		Addr:         addr,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	}
}

// NewRedisQueue initializes a new Redis client with default config.
func NewRedisQueue(addr string) (*RedisQueue, error) {
	return NewRedisQueueWithConfig(DefaultRedisQueueConfig(addr))
}

// NewRedisQueueWithConfig initializes a new Redis client with custom config.
func NewRedisQueueWithConfig(cfg RedisQueueConfig) (*RedisQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
	})

	// Ping to verify connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
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

// EnsureGroup creates the consumer group if it doesn't exist.
func (r *RedisQueue) EnsureGroup(ctx context.Context, group string) error {
	err := r.client.XGroupCreateMkStream(ctx, StreamKeyPending, group, "$").Err()
	if err != nil {
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return nil
		}
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	return nil
}

// Pop retrieves a job from the queue for a specific consumer group.
func (r *RedisQueue) Pop(ctx context.Context, group string, consumer string) (string, *models.Execution, error) {
	// Block for 2 seconds waiting for new messages
	streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{StreamKeyPending, ">"},
		Count:    1,
		Block:    2 * time.Second,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return "", nil, nil // Timeout, no jobs
		}
		return "", nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return "", nil, nil
	}

	msg := streams[0].Messages[0]
	msgID := msg.ID
	
	payloadStr, ok := msg.Values["payload"].(string)
	if !ok {
		return msgID, nil, fmt.Errorf("invalid payload format")
	}

	var exec models.Execution
	if err := json.Unmarshal([]byte(payloadStr), &exec); err != nil {
		return msgID, nil, fmt.Errorf("failed to unmarshal execution: %w", err)
	}

	return msgID, &exec, nil
}

// Ack acknowledges a job execution as processed.
func (r *RedisQueue) Ack(ctx context.Context, group string, msgID string) error {
	return r.client.XAck(ctx, StreamKeyPending, group, msgID).Err()
}
