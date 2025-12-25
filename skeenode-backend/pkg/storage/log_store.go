package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// LogStore provides an interface for storing execution logs
type LogStore interface {
	// Store saves logs and returns a reference path/URL
	Store(ctx context.Context, executionID string, logs []byte) (string, error)
	// Retrieve fetches logs by reference
	Retrieve(ctx context.Context, reference string) ([]byte, error)
}

// S3LogStore stores logs in S3-compatible storage
type S3LogStore struct {
	client     *s3.Client
	bucket     string
	prefix     string
	localCache string
}

// S3LogStoreConfig holds S3 configuration
type S3LogStoreConfig struct {
	Bucket          string
	Prefix          string        // e.g., "logs/executions/"
	Region          string
	Endpoint        string        // For MinIO/local S3
	AccessKeyID     string
	SecretAccessKey string
	LocalCacheDir   string        // Local cache for frequently accessed logs
}

// NewS3LogStore creates a new S3-backed log store
func NewS3LogStore(cfg S3LogStoreConfig) (*S3LogStore, error) {
	// Build AWS config
	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	// Custom credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	clientOpts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for MinIO
		})
	}

	client := s3.NewFromConfig(awsCfg, clientOpts...)

	// Ensure local cache directory exists
	if cfg.LocalCacheDir != "" {
		if err := os.MkdirAll(cfg.LocalCacheDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	return &S3LogStore{
		client:     client,
		bucket:     cfg.Bucket,
		prefix:     cfg.Prefix,
		localCache: cfg.LocalCacheDir,
	}, nil
}

// Store saves execution logs to S3
func (s *S3LogStore) Store(ctx context.Context, executionID string, logs []byte) (string, error) {
	key := s.buildKey(executionID)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(logs),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload logs to S3: %w", err)
	}

	// Also cache locally for fast access
	if s.localCache != "" {
		cachePath := filepath.Join(s.localCache, executionID+".log")
		_ = os.WriteFile(cachePath, logs, 0644)
	}

	return fmt.Sprintf("s3://%s/%s", s.bucket, key), nil
}

// Retrieve fetches logs from S3
func (s *S3LogStore) Retrieve(ctx context.Context, reference string) ([]byte, error) {
	// Extract key from reference
	key := s.extractKey(reference)

	// Check local cache first
	if s.localCache != "" {
		cachePath := filepath.Join(s.localCache, filepath.Base(key))
		if data, err := os.ReadFile(cachePath); err == nil {
			return data, nil
		}
	}

	// Fetch from S3
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get logs from S3: %w", err)
	}
	defer output.Body.Close()

	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %w", err)
	}

	// Update cache
	if s.localCache != "" {
		cachePath := filepath.Join(s.localCache, filepath.Base(key))
		_ = os.WriteFile(cachePath, data, 0644)
	}

	return data, nil
}

func (s *S3LogStore) buildKey(executionID string) string {
	timestamp := time.Now().Format("2006/01/02")
	return fmt.Sprintf("%s%s/%s.log", s.prefix, timestamp, executionID)
}

func (s *S3LogStore) extractKey(reference string) string {
	// Handle s3://bucket/key format
	if len(reference) > 5 && reference[:5] == "s3://" {
		// Skip s3://bucket/
		parts := reference[5:]
		for i, c := range parts {
			if c == '/' {
				return parts[i+1:]
			}
		}
	}
	return reference
}

// LocalLogStore stores logs on local filesystem (for development/single-node)
type LocalLogStore struct {
	basePath string
}

// NewLocalLogStore creates a local filesystem log store
func NewLocalLogStore(basePath string) (*LocalLogStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	return &LocalLogStore{basePath: basePath}, nil
}

// Store saves logs to local filesystem
func (l *LocalLogStore) Store(ctx context.Context, executionID string, logs []byte) (string, error) {
	path := filepath.Join(l.basePath, executionID+".log")
	if err := os.WriteFile(path, logs, 0644); err != nil {
		return "", fmt.Errorf("failed to write logs: %w", err)
	}
	return path, nil
}

// Retrieve fetches logs from local filesystem
func (l *LocalLogStore) Retrieve(ctx context.Context, reference string) ([]byte, error) {
	return os.ReadFile(reference)
}
