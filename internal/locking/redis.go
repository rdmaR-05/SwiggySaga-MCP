package locking

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrLockFailed = errors.New("failed to acquire lock, resource is busy")

// releaseLockScript atomically deletes a lock only if the stored token matches,
// preventing a pod from releasing a lock it no longer owns after TTL expiry.
var releaseLockScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
`)

// Locker is the interface for distributed locking.
type Locker interface {
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
}

// RedisLocker implements Locker with fenced tokens to prevent cross-pod lock theft.
type RedisLocker struct {
	client *redis.Client
	mu     sync.Mutex
	tokens map[string]string // lockKey → owner token
}

func NewRedisLocker(client *redis.Client) *RedisLocker {
	return &RedisLocker{
		client: client,
		tokens: make(map[string]string),
	}
}

func (r *RedisLocker) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("saga_lock:%s", key)
	token := uuid.New().String()

	acquired, err := r.client.SetNX(ctx, lockKey, token, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis setnx: %w", err)
	}
	if acquired {
		r.mu.Lock()
		r.tokens[lockKey] = token
		r.mu.Unlock()
	}
	return acquired, nil
}

func (r *RedisLocker) ReleaseLock(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("saga_lock:%s", key)

	r.mu.Lock()
	token, ok := r.tokens[lockKey]
	if ok {
		delete(r.tokens, lockKey)
	}
	r.mu.Unlock()

	if !ok {
		return nil // never acquired on this instance
	}

	if err := releaseLockScript.Run(ctx, r.client, []string{lockKey}, token).Err(); err != nil && err != redis.Nil {
		return fmt.Errorf("redis release lock: %w", err)
	}
	return nil
}

// NoOpLocker is a no-op fallback for local dev without Redis.
type NoOpLocker struct{}

func (n *NoOpLocker) AcquireLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}
func (n *NoOpLocker) ReleaseLock(_ context.Context, _ string) error { return nil }
