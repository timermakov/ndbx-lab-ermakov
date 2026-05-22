package repository

import (
	"context"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

// EventReactionRepository stores event reactions in persistent storage.
type EventReactionRepository interface {
	// Put updates or creates reaction for event/user pair.
	Put(ctx context.Context, eventID, userID string, value model.ReactionValue, now time.Time) error
	// CountByEventIDs returns per-event counters by physical event identifier.
	CountByEventIDs(ctx context.Context, eventIDs []string) (map[string]model.EventReactions, error)
}

// EventReactionCache stores aggregated reaction counters for logical events.
type EventReactionCache interface {
	// GetByTitle returns cached counters by event title.
	GetByTitle(ctx context.Context, title string) (model.EventReactions, bool, error)
	// SetByTitle caches counters for event title.
	SetByTitle(ctx context.Context, title string, reactions model.EventReactions) error
	// DeleteByTitle removes cache entry for event title.
	DeleteByTitle(ctx context.Context, title string) error
}
