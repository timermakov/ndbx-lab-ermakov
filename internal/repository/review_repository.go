package repository

import (
	"context"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

// EventReviewRepository stores event reviews in persistent storage.
type EventReviewRepository interface {
	// Create stores a new event review.
	Create(ctx context.Context, eventID, userID, comment string, rating int, now time.Time) (model.EventReview, error)
	// GetByEventIDAndUserID returns one user review for an event.
	GetByEventIDAndUserID(ctx context.Context, eventID, userID string) (model.EventReview, error)
	// ListByEventID returns event reviews using application-side pagination.
	ListByEventID(ctx context.Context, eventID string, limit, offset uint64) ([]model.EventReview, error)
	// Update updates mutable review fields.
	Update(ctx context.Context, review model.EventReview) error
	// CountByEventIDs returns per-event non-rounded counters.
	CountByEventIDs(ctx context.Context, eventIDs []string) (map[string]model.EventReviewsCounters, error)
}

// EventReviewCache stores aggregated review counters for logical events.
type EventReviewCache interface {
	// GetByTitle returns cached review counters by event title.
	GetByTitle(ctx context.Context, title string) (model.EventReviewsSummary, bool, error)
	// SetByTitle caches review counters by event title.
	SetByTitle(ctx context.Context, title string, reviews model.EventReviewsSummary) error
	// DeleteByTitle removes cached counters by event title.
	DeleteByTitle(ctx context.Context, title string) error
}
