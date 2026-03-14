package session

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Session represents a user session stored in Redis.
type Session struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store defines an interface for managing sessions.
type Store interface {
	// Create creates a new session with the given id.
	Create(ctx context.Context, id string, now time.Time) (Session, error)
	// Get returns a session by id.
	Get(ctx context.Context, id string) (Session, bool, error)
	// Touch updates the session updated_at field and refreshes TTL.
	Touch(ctx context.Context, id string, now time.Time) (Session, error)
}

// RedisStore is a Redis-backed implementation of Store.
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore creates a new RedisStore with the given client and TTL.
func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	return &RedisStore{
		client: client,
		ttl:    ttl,
	}
}

func (s *RedisStore) key(id string) string {
	return "sid:" + id
}

// Create creates a new session in Redis.
func (s *RedisStore) Create(ctx context.Context, id string, now time.Time) (Session, error) {
	createdAt := now.UTC().Format(time.RFC3339)
	updatedAt := createdAt

	key := s.key(id)

	// Use a transaction to ensure hash creation and TTL setting are applied together.
	_, err := s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key, "created_at", createdAt)
		pipe.HSet(ctx, key, "updated_at", updatedAt)
		pipe.Expire(ctx, key, s.ttl)
		return nil
	})
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}

	session := Session{
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}

	return session, nil
}

// Get returns the session for the given id.
func (s *RedisStore) Get(ctx context.Context, id string) (Session, bool, error) {
	key := s.key(id)

	data, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return Session{}, false, fmt.Errorf("get session: %w", err)
	}

	if len(data) == 0 {
		return Session{}, false, nil
	}

	createdAt, err := time.Parse(time.RFC3339, data["created_at"])
	if err != nil {
		return Session{}, false, fmt.Errorf("parse created_at: %w", err)
	}

	updatedAt, err := time.Parse(time.RFC3339, data["updated_at"])
	if err != nil {
		return Session{}, false, fmt.Errorf("parse updated_at: %w", err)
	}

	return Session{
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, true, nil
}

// Touch updates the session updated_at field and refreshes TTL.
func (s *RedisStore) Touch(ctx context.Context, id string, now time.Time) (Session, error) {
	key := s.key(id)
	updatedAt := now.UTC().Format(time.RFC3339)

	_, err := s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key, "updated_at", updatedAt)
		pipe.Expire(ctx, key, s.ttl)
		return nil
	})
	if err != nil {
		return Session{}, fmt.Errorf("touch session: %w", err)
	}

	session, ok, err := s.Get(ctx, id)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, fmt.Errorf("session not found after touch")
	}

	return session, nil
}
