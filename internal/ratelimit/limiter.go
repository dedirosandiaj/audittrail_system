package ratelimit

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter applies a sliding-window rate limit keyed by (application, bucket).
type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

// Allow returns true if the given (appID, bucket) is below the per-minute limit.
func (l *Limiter) Allow(ctx context.Context, appID, bucket string, limit int) (bool, int, error) {
	if l.rdb == nil || limit <= 0 {
		return true, limit, nil
	}
	window := time.Minute
	now := time.Now().UnixMilli()
	minScore := now - window.Milliseconds()
	key := "rl:" + appID + ":" + bucket

	pipe := l.rdb.TxPipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(minScore, 10))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	countCmd := pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, window*2)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}
	count := int(countCmd.Val())
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return count <= limit, remaining, nil
}
