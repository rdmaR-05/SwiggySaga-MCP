package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type SagaState struct {
	SagaID        string   `json:"saga_id"`
	Name          string   `json:"name"`
	Status        string   `json:"status"` // "started", "completed", "failed", "rolled_back"
	ExecutedSteps []string `json:"executed_steps"`
	UpdatedAt     int64    `json:"updated_at"`
}

type Store interface {
	SaveState(ctx context.Context, state SagaState) error
	LoadState(ctx context.Context, sagaID string) (*SagaState, error)
	GetStuckSagas(ctx context.Context, timeout time.Duration) ([]SagaState, error)
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (r *RedisStore) SaveState(ctx context.Context, state SagaState) error {
	state.UpdatedAt = time.Now().Unix()
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	
	// Keep saga state for 7 days
	err = r.client.Set(ctx, "saga_state:"+state.SagaID, data, 7*24*time.Hour).Err()
	if err != nil {
		return err
	}
	
	// Register active sagas in a ZSET for async recovery daemon sweeps
	if state.Status == "started" {
		r.client.ZAdd(ctx, "active_sagas", redis.Z{
			Score:  float64(state.UpdatedAt),
			Member: state.SagaID,
		})
	} else {
		// Prune terminal sagas from the active recovery tracker
		r.client.ZRem(ctx, "active_sagas", state.SagaID)
	}
	
	return nil
}

func (r *RedisStore) LoadState(ctx context.Context, sagaID string) (*SagaState, error) {
	data, err := r.client.Get(ctx, "saga_state:"+sagaID).Result()
	if err != nil {
		return nil, err
	}
	
	var state SagaState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *RedisStore) GetStuckSagas(ctx context.Context, timeout time.Duration) ([]SagaState, error) {
	threshold := float64(time.Now().Add(-timeout).Unix())
	
	// Get all saga IDs that were updated before the threshold
	sagaIDs, err := r.client.ZRangeByScore(ctx, "active_sagas", &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", threshold),
	}).Result()
	
	if err != nil && err != redis.Nil {
		return nil, err
	}
	
	var stuckSagas []SagaState
	for _, id := range sagaIDs {
		state, err := r.LoadState(ctx, id)
		if err == nil && state != nil {
			stuckSagas = append(stuckSagas, *state)
		}
	}
	return stuckSagas, nil
}

type NoOpStore struct{}

func (n *NoOpStore) SaveState(ctx context.Context, state SagaState) error { return nil }
func (n *NoOpStore) LoadState(ctx context.Context, sagaID string) (*SagaState, error) { return nil, nil }
func (n *NoOpStore) GetStuckSagas(ctx context.Context, timeout time.Duration) ([]SagaState, error) { return nil, nil }
