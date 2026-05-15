package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

const (
	cassandraReactionTableName = "event_reactions"
)

// CassandraEventReactionRepository stores event reactions in Cassandra.
type CassandraEventReactionRepository struct {
	session *gocql.Session
}

// NewCassandraEventReactionRepository creates a reaction repository backed by Cassandra.
func NewCassandraEventReactionRepository(session *gocql.Session) *CassandraEventReactionRepository {
	return &CassandraEventReactionRepository{
		session: session,
	}
}

// Put updates or creates user's reaction for a physical event occurrence.
func (r *CassandraEventReactionRepository) Put(
	ctx context.Context,
	eventID, userID string,
	value model.ReactionValue,
	now time.Time,
) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (event_id, created_by, like_value, created_at) VALUES (?, ?, ?, ?)`,
		cassandraReactionTableName,
	)
	if err := r.session.Query(query, eventID, userID, int8(value), now.UTC()).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("insert reaction: %w", err)
	}

	return nil
}

// CountByEventIDs returns likes/dislikes counters per physical event id.
func (r *CassandraEventReactionRepository) CountByEventIDs(
	ctx context.Context,
	eventIDs []string,
) (map[string]model.EventReactions, error) {
	reactionsByEventID := make(map[string]model.EventReactions, len(eventIDs))
	for _, eventID := range eventIDs {
		trimmedEventID := strings.TrimSpace(eventID)
		if trimmedEventID == "" {
			continue
		}

		query := fmt.Sprintf("SELECT like_value FROM %s WHERE event_id = ?", cassandraReactionTableName)
		iter := r.session.Query(query, trimmedEventID).WithContext(ctx).Iter()

		var likeValue int8
		reactions := model.EventReactions{}
		for iter.Scan(&likeValue) {
			switch model.ReactionValue(likeValue) {
			case model.ReactionLike:
				reactions.Likes++
			case model.ReactionDislike:
				reactions.Dislikes++
			}
		}
		if err := iter.Close(); err != nil {
			return nil, fmt.Errorf("iterate reactions by event_id %q: %w", trimmedEventID, err)
		}

		reactionsByEventID[trimmedEventID] = reactions
	}

	return reactionsByEventID, nil
}
