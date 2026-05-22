package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

const (
	cassandraReviewTableName = "event_reviews"
)

// CassandraEventReviewRepository stores event reviews in Cassandra.
type CassandraEventReviewRepository struct {
	session *gocql.Session
}

// NewCassandraEventReviewRepository creates a review repository backed by Cassandra.
func NewCassandraEventReviewRepository(session *gocql.Session) *CassandraEventReviewRepository {
	return &CassandraEventReviewRepository{
		session: session,
	}
}

// Create stores a new event review with one-review-per-user-per-event contract.
func (r *CassandraEventReviewRepository) Create(
	ctx context.Context,
	eventID, userID, comment string,
	rating int,
	now time.Time,
) (model.EventReview, error) {
	reviewUUID := gocql.TimeUUID()
	query := fmt.Sprintf(
		`INSERT INTO %s (event_id, created_by, id, rating, comment, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`,
		cassandraReviewTableName,
	)
	applied, err := r.session.Query(
		query,
		eventID,
		userID,
		reviewUUID,
		int8(rating),
		comment,
		now.UTC(),
		now.UTC(),
	).WithContext(ctx).ScanCAS()
	if err != nil {
		return model.EventReview{}, fmt.Errorf("insert review: %w", err)
	}
	if !applied {
		return model.EventReview{}, ErrAlreadyExists
	}

	return model.EventReview{
		ID:        reviewUUID.String(),
		EventID:   eventID,
		Comment:   comment,
		CreatedAt: now.UTC().Format(time.RFC3339),
		CreatedBy: userID,
		Rating:    rating,
		UpdatedAt: now.UTC().Format(time.RFC3339),
	}, nil
}

// GetByEventIDAndUserID returns review by event/user pair.
func (r *CassandraEventReviewRepository) GetByEventIDAndUserID(
	ctx context.Context,
	eventID, userID string,
) (model.EventReview, error) {
	query := fmt.Sprintf(
		`SELECT id, event_id, comment, created_at, created_by, rating, updated_at FROM %s WHERE event_id = ? AND created_by = ? LIMIT 1`,
		cassandraReviewTableName,
	)
	var reviewID gocql.UUID
	var reviewEventID string
	var comment string
	var createdAt time.Time
	var createdBy string
	var rating int8
	var updatedAt time.Time

	if err := r.session.Query(query, eventID, userID).WithContext(ctx).Scan(
		&reviewID,
		&reviewEventID,
		&comment,
		&createdAt,
		&createdBy,
		&rating,
		&updatedAt,
	); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return model.EventReview{}, ErrNotFound
		}
		return model.EventReview{}, fmt.Errorf("get review by event and user: %w", err)
	}

	return model.EventReview{
		ID:        reviewID.String(),
		EventID:   reviewEventID,
		Comment:   comment,
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
		CreatedBy: createdBy,
		Rating:    int(rating),
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// ListByEventID returns event reviews with app-side offset handling.
func (r *CassandraEventReviewRepository) ListByEventID(
	ctx context.Context,
	eventID string,
	limit, offset uint64,
) ([]model.EventReview, error) {
	query := fmt.Sprintf(
		`SELECT id, event_id, comment, created_at, created_by, rating, updated_at FROM %s WHERE event_id = ?`,
		cassandraReviewTableName,
	)
	iter := r.session.Query(query, eventID).WithContext(ctx).Iter()

	var (
		reviewID    gocql.UUID
		reviewEvent string
		comment     string
		createdAt   time.Time
		createdBy   string
		rating      int8
		updatedAt   time.Time
	)

	reviews := make([]model.EventReview, 0)
	var index uint64
	for iter.Scan(&reviewID, &reviewEvent, &comment, &createdAt, &createdBy, &rating, &updatedAt) {
		if index < offset {
			index++
			continue
		}
		if limit > 0 && uint64(len(reviews)) >= limit {
			break
		}
		reviews = append(reviews, model.EventReview{
			ID:        reviewID.String(),
			EventID:   reviewEvent,
			Comment:   comment,
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
			CreatedBy: createdBy,
			Rating:    int(rating),
			UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		})
		index++
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list reviews by event_id %q: %w", eventID, err)
	}

	return reviews, nil
}

// Update updates review mutable fields by event/user key.
func (r *CassandraEventReviewRepository) Update(ctx context.Context, review model.EventReview) error {
	updatedAt, err := parseRFC3339UTC(review.UpdatedAt)
	if err != nil {
		return fmt.Errorf("parse review updated_at: %w", err)
	}

	query := fmt.Sprintf(
		`UPDATE %s SET rating = ?, comment = ?, updated_at = ? WHERE event_id = ? AND created_by = ?`,
		cassandraReviewTableName,
	)
	if err := r.session.Query(
		query,
		int8(review.Rating),
		review.Comment,
		updatedAt,
		review.EventID,
		review.CreatedBy,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("update review: %w", err)
	}

	return nil
}

// CountByEventIDs returns per-event count and non-rounded rating totals.
func (r *CassandraEventReviewRepository) CountByEventIDs(
	ctx context.Context,
	eventIDs []string,
) (map[string]model.EventReviewsCounters, error) {
	countersByEventID := make(map[string]model.EventReviewsCounters, len(eventIDs))
	for _, eventID := range eventIDs {
		trimmedEventID := strings.TrimSpace(eventID)
		if trimmedEventID == "" {
			continue
		}

		query := fmt.Sprintf(`SELECT rating FROM %s WHERE event_id = ?`, cassandraReviewTableName)
		iter := r.session.Query(query, trimmedEventID).WithContext(ctx).Iter()

		var rating int8
		counters := model.EventReviewsCounters{}
		for iter.Scan(&rating) {
			counters.Count++
			counters.TotalRating += uint64(rating)
		}
		if err := iter.Close(); err != nil {
			return nil, fmt.Errorf("iterate reviews by event_id %q: %w", trimmedEventID, err)
		}

		countersByEventID[trimmedEventID] = counters
	}

	return countersByEventID, nil
}

func parseRFC3339UTC(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}

	return parsed.UTC(), nil
}
