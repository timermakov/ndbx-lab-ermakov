package repository

import (
	"context"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

// RecommendationGraphRepository stores recommendation graph edges and retrieves candidates.
type RecommendationGraphRepository interface {
	UpsertUser(ctx context.Context, userID string) error
	UpsertEvent(ctx context.Context, eventID, title, startedAt string) error
	UpsertLike(ctx context.Context, userID, eventID string) error
	ListRecommendedEventIDs(ctx context.Context, userID string) ([]string, error)
}

// RecommendationCache stores user recommendations in cache.
type RecommendationCache interface {
	GetByUserID(ctx context.Context, userID string) ([]model.Event, bool, error)
	SetByUserID(ctx context.Context, userID string, events []model.Event) error
}
