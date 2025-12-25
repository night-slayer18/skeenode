package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	apiKeyPrefix    = "apikey:"
	apiKeySecretLen = 32
)

// APIKeyStore stores and validates API keys
type APIKeyStore interface {
	ValidateKey(ctx context.Context, key string) (*APIKeyInfo, error)
	CreateKey(ctx context.Context, info APIKeyInfo) (string, error)
	RevokeKey(ctx context.Context, keyID string) error
	ListKeys(ctx context.Context, ownerID string) ([]APIKeyInfo, error)
}

// APIKeyInfo contains metadata about an API key
type APIKeyInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	KeyHash   string   `json:"key_hash"` // SHA-256 hash of the key
	OwnerID   string   `json:"owner_id"`
	Role      Role     `json:"role"`
	OrgID     string   `json:"org_id,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	CreatedAt int64    `json:"created_at"`
	ExpiresAt int64    `json:"expires_at,omitempty"` // 0 = never expires
	LastUsed  int64    `json:"last_used,omitempty"`
}

// RedisAPIKeyStore is a production-ready Redis-backed API key store
type RedisAPIKeyStore struct {
	client *redis.Client
	ttl    time.Duration // TTL for key lookups cache
}

// NewRedisAPIKeyStore creates a new Redis-backed API key store
func NewRedisAPIKeyStore(client *redis.Client) *RedisAPIKeyStore {
	return &RedisAPIKeyStore{
		client: client,
		ttl:    24 * time.Hour, // Keys cached for 24 hours
	}
}

// ValidateKey checks if an API key is valid and returns its info
func (s *RedisAPIKeyStore) ValidateKey(ctx context.Context, key string) (*APIKeyInfo, error) {
	// Hash the provided key
	keyHash := hashKey(key)

	// Look up by hash
	data, err := s.client.Get(ctx, apiKeyPrefix+keyHash).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to lookup key: %w", err)
	}

	var info APIKeyInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key info: %w", err)
	}

	// Check expiration
	if info.ExpiresAt > 0 && info.ExpiresAt < time.Now().Unix() {
		return nil, ErrExpiredToken
	}

	// Update last used timestamp asynchronously
	go func() {
		info.LastUsed = time.Now().Unix()
		if data, err := json.Marshal(info); err == nil {
			_ = s.client.Set(context.Background(), apiKeyPrefix+keyHash, data, s.ttl)
		}
	}()

	return &info, nil
}

// CreateKey stores a new API key and returns the plaintext key (only shown once)
func (s *RedisAPIKeyStore) CreateKey(ctx context.Context, info APIKeyInfo) (string, error) {
	// Generate cryptographically secure random key
	secret := make([]byte, apiKeySecretLen)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}

	// Format: sk_<hex-encoded-secret>
	plainKey := "sk_" + hex.EncodeToString(secret)

	// Store hash, never the plaintext
	info.KeyHash = hashKey(plainKey)
	info.CreatedAt = time.Now().Unix()

	// Generate ID if not provided
	if info.ID == "" {
		idBytes := make([]byte, 8)
		_, _ = rand.Read(idBytes)
		info.ID = "key_" + hex.EncodeToString(idBytes)
	}

	data, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("failed to marshal key info: %w", err)
	}

	// Store by hash for lookups
	if err := s.client.Set(ctx, apiKeyPrefix+info.KeyHash, data, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("failed to store key: %w", err)
	}

	// Also store ID -> hash mapping for revocation
	if err := s.client.Set(ctx, apiKeyPrefix+"id:"+info.ID, info.KeyHash, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("failed to store key mapping: %w", err)
	}

	// Store in owner's key set for listing
	if err := s.client.SAdd(ctx, apiKeyPrefix+"owner:"+info.OwnerID, info.ID).Err(); err != nil {
		return "", fmt.Errorf("failed to add to owner set: %w", err)
	}

	return plainKey, nil
}

// RevokeKey removes an API key
func (s *RedisAPIKeyStore) RevokeKey(ctx context.Context, keyID string) error {
	// Get hash from ID
	keyHash, err := s.client.Get(ctx, apiKeyPrefix+"id:"+keyID).Result()
	if err != nil {
		if err == redis.Nil {
			return ErrInvalidToken
		}
		return fmt.Errorf("failed to lookup key: %w", err)
	}

	// Get key info to find owner
	data, err := s.client.Get(ctx, apiKeyPrefix+keyHash).Bytes()
	if err != nil {
		return fmt.Errorf("failed to get key info: %w", err)
	}

	var info APIKeyInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("failed to unmarshal key info: %w", err)
	}

	// Delete all related keys
	pipe := s.client.Pipeline()
	pipe.Del(ctx, apiKeyPrefix+keyHash)
	pipe.Del(ctx, apiKeyPrefix+"id:"+keyID)
	pipe.SRem(ctx, apiKeyPrefix+"owner:"+info.OwnerID, keyID)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	return nil
}

// ListKeys returns all keys for an owner (without exposing the actual keys)
func (s *RedisAPIKeyStore) ListKeys(ctx context.Context, ownerID string) ([]APIKeyInfo, error) {
	// Get all key IDs for owner
	keyIDs, err := s.client.SMembers(ctx, apiKeyPrefix+"owner:"+ownerID).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	var keys []APIKeyInfo
	for _, keyID := range keyIDs {
		keyHash, err := s.client.Get(ctx, apiKeyPrefix+"id:"+keyID).Result()
		if err != nil {
			continue // Key may have been deleted
		}

		data, err := s.client.Get(ctx, apiKeyPrefix+keyHash).Bytes()
		if err != nil {
			continue
		}

		var info APIKeyInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		// Don't expose the hash in listings
		info.KeyHash = ""
		keys = append(keys, info)
	}

	return keys, nil
}

// hashKey creates a SHA-256 hash of an API key
func hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
