package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

const recommendationCacheFieldEvents = "events"

// RedisRecommendationCache stores user recommendations in Redis hash.
type RedisRecommendationCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisRecommendationCache creates recommendations cache backed by Redis.
func NewRedisRecommendationCache(client *redis.Client, ttl time.Duration) *RedisRecommendationCache {
	return &RedisRecommendationCache{
		client: client,
		ttl:    ttl,
	}
}

// GetByUserID returns cached recommendations by user id.
func (c *RedisRecommendationCache) GetByUserID(
	ctx context.Context,
	userID string,
) ([]model.Event, bool, error) {
	payload, err := c.client.HGet(ctx, c.keyByUserID(userID), recommendationCacheFieldEvents).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read recommendations cache: %w", err)
	}

	var events []model.Event
	if err := json.Unmarshal([]byte(payload), &events); err != nil {
		return nil, false, fmt.Errorf("decode recommendations cache: %w", err)
	}

	return events, true, nil
}

// SetByUserID writes recommendations to Redis hash and applies configured TTL.
func (c *RedisRecommendationCache) SetByUserID(
	ctx context.Context,
	userID string,
	events []model.Event,
) error {
	eventsJSON, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("encode recommendations cache: %w", err)
	}

	key := c.keyByUserID(userID)
	_, err = c.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key, recommendationCacheFieldEvents, string(eventsJSON))
		pipe.Expire(ctx, key, c.ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("write recommendations cache: %w", err)
	}

	return nil
}

func (c *RedisRecommendationCache) keyByUserID(userID string) string {
	return "user:" + userID + ":recomms"
}
