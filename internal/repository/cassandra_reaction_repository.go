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
	systemSession *gocql.Session
	session       *gocql.Session
	keyspace      string
}

// NewCassandraEventReactionRepository creates a reaction repository backed by Cassandra.
func NewCassandraEventReactionRepository(
	systemSession *gocql.Session,
	keyspaceSession *gocql.Session,
	keyspace string,
) *CassandraEventReactionRepository {
	return &CassandraEventReactionRepository{
		systemSession: systemSession,
		session:       keyspaceSession,
		keyspace:      keyspace,
	}
}

// EnsureSchema creates keyspace, table, and supporting indexes.
func (r *CassandraEventReactionRepository) EnsureSchema(ctx context.Context) error {
	createKeyspaceQuery := fmt.Sprintf(
		`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`,
		r.keyspace,
	)
	if err := r.systemSession.Query(createKeyspaceQuery).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("create keyspace: %w", err)
	}

	createTableQuery := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (
			event_id text,
			created_by text,
			like_value tinyint,
			created_at timestamp,
			PRIMARY KEY ((event_id), created_by)
		)`,
		cassandraReactionTableName,
	)
	if err := r.session.Query(createTableQuery).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	createLikeValueIndexQuery := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s_like_value_idx ON %s (like_value)",
		cassandraReactionTableName,
		cassandraReactionTableName,
	)
	if err := r.session.Query(createLikeValueIndexQuery).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("create like value index: %w", err)
	}

	return nil
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
