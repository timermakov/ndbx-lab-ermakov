package repository

import (
	"context"
	"crypto/md5" //nolint:gosec // md5 is required by lab contract for cache key format.
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

const (
	reactionCacheFieldLikes    = "likes"
	reactionCacheFieldDislikes = "dislikes"
)

// RedisEventReactionCache stores aggregated event reactions in Redis hash.
type RedisEventReactionCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisEventReactionCache creates reactions cache backed by Redis.
func NewRedisEventReactionCache(client *redis.Client, ttl time.Duration) *RedisEventReactionCache {
	return &RedisEventReactionCache{
		client: client,
		ttl:    ttl,
	}
}

// GetByTitle returns reactions by event title hash key.
func (c *RedisEventReactionCache) GetByTitle(ctx context.Context, title string) (model.EventReactions, bool, error) {
	reactions, err := c.client.HGetAll(ctx, c.keyByTitle(title)).Result()
	if err != nil {
		return model.EventReactions{}, false, fmt.Errorf("read reactions cache: %w", err)
	}
	if len(reactions) == 0 {
		return model.EventReactions{}, false, nil
	}

	likes, err := strconv.ParseUint(reactions[reactionCacheFieldLikes], 10, 64)
	if err != nil {
		return model.EventReactions{}, false, fmt.Errorf("parse likes from cache: %w", err)
	}
	dislikes, err := strconv.ParseUint(reactions[reactionCacheFieldDislikes], 10, 64)
	if err != nil {
		return model.EventReactions{}, false, fmt.Errorf("parse dislikes from cache: %w", err)
	}

	return model.EventReactions{
		Likes:    likes,
		Dislikes: dislikes,
	}, true, nil
}

// SetByTitle writes reactions into Redis hash and applies configured TTL.
func (c *RedisEventReactionCache) SetByTitle(ctx context.Context, title string, reactions model.EventReactions) error {
	key := c.keyByTitle(title)
	_, err := c.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key, reactionCacheFieldLikes, reactions.Likes)
		pipe.HSet(ctx, key, reactionCacheFieldDislikes, reactions.Dislikes)
		pipe.Expire(ctx, key, c.ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("write reactions cache: %w", err)
	}

	return nil
}

// DeleteByTitle drops cached reactions key for event title.
func (c *RedisEventReactionCache) DeleteByTitle(ctx context.Context, title string) error {
	if err := c.client.Del(ctx, c.keyByTitle(title)).Err(); err != nil {
		return fmt.Errorf("delete reactions cache: %w", err)
	}

	return nil
}

func (c *RedisEventReactionCache) keyByTitle(title string) string {
	return "event:" + md5Hex(title) + ":reactions"
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}
