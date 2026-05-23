package repository

import (
	"context"
	"crypto/md5" //nolint:gosec // md5 is required by lab contract for cache key format.
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

const (
	reviewCacheFieldCount  = "count"
	reviewCacheFieldRating = "rating"
)

// RedisEventReviewCache stores aggregated event reviews in Redis hash.
type RedisEventReviewCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisEventReviewCache creates reviews cache backed by Redis.
func NewRedisEventReviewCache(client *redis.Client, ttl time.Duration) *RedisEventReviewCache {
	return &RedisEventReviewCache{
		client: client,
		ttl:    ttl,
	}
}

// GetByTitle returns aggregated reviews by event title hash key.
func (c *RedisEventReviewCache) GetByTitle(ctx context.Context, title string) (model.EventReviewsSummary, bool, error) {
	values, err := c.client.HGetAll(ctx, c.keyByTitle(title)).Result()
	if err != nil {
		return model.EventReviewsSummary{}, false, fmt.Errorf("read reviews cache: %w", err)
	}
	if len(values) == 0 {
		return model.EventReviewsSummary{}, false, nil
	}

	count, err := strconv.ParseUint(values[reviewCacheFieldCount], 10, 64)
	if err != nil {
		return model.EventReviewsSummary{}, false, fmt.Errorf("parse reviews count from cache: %w", err)
	}
	rating, err := strconv.ParseFloat(values[reviewCacheFieldRating], 64)
	if err != nil {
		return model.EventReviewsSummary{}, false, fmt.Errorf("parse reviews rating from cache: %w", err)
	}

	return model.EventReviewsSummary{
		Count:  count,
		Rating: rating,
	}, true, nil
}

// SetByTitle writes reviews into Redis hash and applies configured TTL.
func (c *RedisEventReviewCache) SetByTitle(ctx context.Context, title string, reviews model.EventReviewsSummary) error {
	key := c.keyByTitle(title)
	_, err := c.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key, reviewCacheFieldCount, reviews.Count)
		pipe.HSet(ctx, key, reviewCacheFieldRating, reviews.Rating)
		pipe.Expire(ctx, key, c.ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("write reviews cache: %w", err)
	}

	return nil
}

// DeleteByTitle drops cached reviews key for event title.
func (c *RedisEventReviewCache) DeleteByTitle(ctx context.Context, title string) error {
	if err := c.client.Del(ctx, c.keyByTitle(title)).Err(); err != nil {
		return fmt.Errorf("delete reviews cache: %w", err)
	}

	return nil
}

func (c *RedisEventReviewCache) keyByTitle(title string) string {
	return "event:" + md5HexReview(title) + ":reviews"
}

func md5HexReview(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
