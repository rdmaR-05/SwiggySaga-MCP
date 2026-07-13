package main

import (
	"context"
	"log/slog"
	"time"

	"swiggy-saga-mcp/internal/saga"

	"github.com/redis/go-redis/v9"
)

// StartRecoveryDaemon scans for sagas stuck in "started" state (e.g. after a pod crash)
// and marks them rolled_back. Leader-elected via Redis SETNX so only one pod runs per cycle.
func StartRecoveryDaemon(ctx context.Context, store saga.Store, redisClient *redis.Client) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	
	slog.Info("Saga Recovery Daemon started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("Saga Recovery Daemon shutting down")
			return
		case <-ticker.C:
			if redisClient != nil {
				// leader election: skip if another pod holds the lock
				acquired, err := redisClient.SetNX(ctx, "recovery_leader_lock", "locked", 30*time.Second).Result()
				if err != nil || !acquired {
					slog.Debug("Another pod is leading recovery, skipping cycle")
					continue
				}
			}

			stuckSagas, err := store.GetStuckSagas(ctx, 5*time.Minute)
			if err != nil {
				slog.Error("Failed to fetch stuck sagas", "error", err)
				continue
			}

			for _, s := range stuckSagas {
				slog.Warn("Found stuck saga, initiating manual rollback", "saga_id", s.SagaID, "saga_name", s.Name)
				// TODO: reconstruct workflow steps from s.Name + s.ExecutedSteps and invoke Compensate.
				s.Status = "rolled_back"
				store.SaveState(ctx, s)
				slog.Info("Stuck saga marked rolled_back", "saga_id", s.SagaID)
			}
		}
	}
}
