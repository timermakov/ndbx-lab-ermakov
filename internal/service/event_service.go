package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
	"github.com/timermakov/ndbx-lab-ermakov/internal/repository"
)

// EventService implements event business logic.
type EventService struct {
	events      repository.EventRepository
	users       repository.UserRepository
	reactions   repository.EventReactionRepository
	cache       repository.EventReactionCache
	reviews     repository.EventReviewRepository
	reviewCache repository.EventReviewCache
}

// NewEventService creates a new EventService.
func NewEventService(events repository.EventRepository, users repository.UserRepository) *EventService {
	return &EventService{
		events: events,
		users:  users,
	}
}

// SetReactionsStorage configures reactions persistence and cache dependencies.
func (s *EventService) SetReactionsStorage(
	reactions repository.EventReactionRepository,
	cache repository.EventReactionCache,
) {
	s.reactions = reactions
	s.cache = cache
}

// SetReviewsStorage configures reviews persistence and cache dependencies.
func (s *EventService) SetReviewsStorage(
	reviews repository.EventReviewRepository,
	cache repository.EventReviewCache,
) {
	s.reviews = reviews
	s.reviewCache = cache
}

// CreateEventInput is input data for event creation.
type CreateEventInput struct {
	Title       string
	Address     string
	Description string
	StartedAt   string
	FinishedAt  string
	CreatedBy   string
}

// EventsQuery holds GET /events query parameters.
type EventsQuery struct {
	ID        string
	Title     string
	Category  string
	PriceFrom string
	PriceTo   string
	City      string
	DateFrom  string
	DateTo    string
	User      string
	Limit     string
	Offset    string
}

// UpdateEventInput is input data for PATCH /events/{id}.
type UpdateEventInput struct {
	Category *string
	Price    *uint64
	City     *string
	HasCity  bool
}

// EventReviewsQuery holds GET /events/{id}/reviews query parameters.
type EventReviewsQuery struct {
	Limit  string
	Offset string
}

// UpdateEventReviewInput stores mutable review fields.
type UpdateEventReviewInput struct {
	Comment    *string
	HasComment bool
	Rating     *int
	HasRating  bool
}

// ValidateListQuery validates and converts list query parameters.
func (s *EventService) ValidateListQuery(query EventsQuery) (repository.EventFilter, string, error) {
	filter := repository.EventFilter{
		ID:            strings.TrimSpace(query.ID),
		Title:         strings.TrimSpace(query.Title),
		Category:      strings.TrimSpace(strings.ToLower(query.Category)),
		City:          strings.TrimSpace(query.City),
		CreatedByName: strings.TrimSpace(query.User),
	}

	if filter.ID != "" {
		if _, err := primitive.ObjectIDFromHex(filter.ID); err != nil {
			return repository.EventFilter{}, "id", ErrInvalidParameter
		}
	}

	if filter.Category != "" {
		if !isValidCategory(filter.Category) {
			return repository.EventFilter{}, "category", ErrInvalidParameter
		}
	}

	if strings.TrimSpace(query.PriceFrom) != "" {
		value, err := strconv.ParseUint(strings.TrimSpace(query.PriceFrom), 10, 64)
		if err != nil {
			return repository.EventFilter{}, "price_from", ErrInvalidParameter
		}
		filter.PriceFrom = &value
	}

	if strings.TrimSpace(query.PriceTo) != "" {
		value, err := strconv.ParseUint(strings.TrimSpace(query.PriceTo), 10, 64)
		if err != nil {
			return repository.EventFilter{}, "price_to", ErrInvalidParameter
		}
		filter.PriceTo = &value
	}

	if filter.PriceFrom != nil && filter.PriceTo != nil && *filter.PriceFrom > *filter.PriceTo {
		return repository.EventFilter{}, "price_to", ErrInvalidParameter
	}

	if strings.TrimSpace(query.DateFrom) != "" {
		day, err := parseYYYYMMDD(strings.TrimSpace(query.DateFrom))
		if err != nil {
			return repository.EventFilter{}, "date_from", ErrInvalidParameter
		}
		filter.StartedAtFrom = day.UTC().Format(time.RFC3339)
	}

	if strings.TrimSpace(query.DateTo) != "" {
		day, err := parseYYYYMMDD(strings.TrimSpace(query.DateTo))
		if err != nil {
			return repository.EventFilter{}, "date_to", ErrInvalidParameter
		}
		endOfDay := day.Add(24*time.Hour - time.Second)
		filter.StartedAtTo = endOfDay.UTC().Format(time.RFC3339)
	}

	if filter.StartedAtFrom != "" && filter.StartedAtTo != "" && filter.StartedAtFrom > filter.StartedAtTo {
		return repository.EventFilter{}, "date_to", ErrInvalidParameter
	}

	if strings.TrimSpace(query.Limit) != "" {
		limit, err := strconv.ParseUint(strings.TrimSpace(query.Limit), 10, 64)
		if err != nil {
			return repository.EventFilter{}, "limit", ErrInvalidParameter
		}
		filter.Limit = limit
	}

	if strings.TrimSpace(query.Offset) != "" {
		offset, err := strconv.ParseUint(strings.TrimSpace(query.Offset), 10, 64)
		if err != nil {
			return repository.EventFilter{}, "offset", ErrInvalidParameter
		}
		filter.Offset = offset
	}

	return filter, "", nil
}

// Create validates and stores a new event.
func (s *EventService) Create(ctx context.Context, input CreateEventInput, now time.Time) (model.Event, string, error) {
	if strings.TrimSpace(input.Title) == "" {
		return model.Event{}, "title", ErrInvalidField
	}
	if strings.TrimSpace(input.Address) == "" {
		return model.Event{}, "address", ErrInvalidField
	}
	if strings.TrimSpace(input.StartedAt) == "" {
		return model.Event{}, "started_at", ErrInvalidField
	}
	if strings.TrimSpace(input.FinishedAt) == "" {
		return model.Event{}, "finished_at", ErrInvalidField
	}

	startedAt, err := time.Parse(time.RFC3339, input.StartedAt)
	if err != nil {
		return model.Event{}, "started_at", ErrInvalidField
	}

	finishedAt, err := time.Parse(time.RFC3339, input.FinishedAt)
	if err != nil {
		return model.Event{}, "finished_at", ErrInvalidField
	}

	if finishedAt.Before(startedAt) {
		return model.Event{}, "finished_at", ErrInvalidField
	}

	event, err := s.events.Create(ctx, model.Event{
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		Location: model.EventLocation{
			Address: strings.TrimSpace(input.Address),
		},
		CreatedAt:  now.UTC().Format(time.RFC3339),
		CreatedBy:  input.CreatedBy,
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
	})
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return model.Event{}, "", ErrAlreadyExists
		}

		return model.Event{}, "", fmt.Errorf("create event: %w", err)
	}

	return event, "", nil
}

// List returns events by filter.
func (s *EventService) List(ctx context.Context, filter repository.EventFilter) ([]model.Event, error) {
	if filter.CreatedByName != "" {
		user, err := s.users.GetByUsername(ctx, filter.CreatedByName)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return []model.Event{}, nil
			}

			return nil, fmt.Errorf("get user by username: %w", err)
		}
		filter.CreatedBy = user.ID
	}

	events, err := s.events.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	return events, nil
}

// GetByID returns event by id.
func (s *EventService) GetByID(ctx context.Context, id string) (model.Event, error) {
	event, err := s.events.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Event{}, ErrNotFound
		}

		return model.Event{}, fmt.Errorf("get event by id: %w", err)
	}

	return event, nil
}

// PutReaction creates or updates user reaction for the event.
func (s *EventService) PutReaction(
	ctx context.Context,
	eventID string,
	userID string,
	value model.ReactionValue,
	now time.Time,
) error {
	if s.reactions == nil || s.cache == nil {
		return fmt.Errorf("reaction storage is not configured")
	}

	event, err := s.GetByID(ctx, eventID)
	if err != nil {
		return err
	}

	if err := s.reactions.Put(ctx, strings.TrimSpace(eventID), strings.TrimSpace(userID), value, now); err != nil {
		return fmt.Errorf("save reaction: %w", err)
	}
	if err := s.cache.DeleteByTitle(ctx, event.Title); err != nil {
		return fmt.Errorf("invalidate reactions cache: %w", err)
	}
	if _, err := s.BuildReactionsByTitle(ctx, []model.Event{event}); err != nil {
		return fmt.Errorf("refresh reactions cache: %w", err)
	}

	return nil
}

// BuildReactionsByTitle returns aggregated reactions for event titles from input list.
func (s *EventService) BuildReactionsByTitle(
	ctx context.Context,
	events []model.Event,
) (map[string]model.EventReactions, error) {
	if s.reactions == nil || s.cache == nil {
		return map[string]model.EventReactions{}, nil
	}

	targetTitles := make(map[string]struct{}, len(events))
	for _, event := range events {
		title := strings.TrimSpace(event.Title)
		if title == "" {
			continue
		}
		targetTitles[title] = struct{}{}
	}
	if len(targetTitles) == 0 {
		return map[string]model.EventReactions{}, nil
	}

	allEvents, err := s.events.List(ctx, repository.EventFilter{})
	if err != nil {
		return nil, fmt.Errorf("list events for reactions aggregation: %w", err)
	}

	eventIDsByTitle := make(map[string][]string, len(targetTitles))
	for _, event := range allEvents {
		title := strings.TrimSpace(event.Title)
		if _, ok := targetTitles[title]; !ok {
			continue
		}

		eventIDsByTitle[title] = append(eventIDsByTitle[title], event.ID)
	}

	reactionsByTitle := make(map[string]model.EventReactions, len(targetTitles))
	for title := range targetTitles {
		cachedReactions, found, cacheErr := s.cache.GetByTitle(ctx, title)
		if cacheErr != nil {
			return nil, fmt.Errorf("read reactions cache by title %q: %w", title, cacheErr)
		}
		if found {
			reactionsByTitle[title] = cachedReactions
			continue
		}

		eventIDs := eventIDsByTitle[title]
		if len(eventIDs) == 0 {
			reactionsByTitle[title] = model.EventReactions{}
			continue
		}

		reactionsByEventID, countErr := s.reactions.CountByEventIDs(ctx, eventIDs)
		if countErr != nil {
			return nil, fmt.Errorf("count reactions by event ids: %w", countErr)
		}

		aggregated := model.EventReactions{}
		for _, eventID := range eventIDs {
			eventReactions, ok := reactionsByEventID[eventID]
			if !ok {
				continue
			}

			aggregated.Likes += eventReactions.Likes
			aggregated.Dislikes += eventReactions.Dislikes
		}

		reactionsByTitle[title] = aggregated
		if aggregated.Likes == 0 && aggregated.Dislikes == 0 {
			continue
		}

		if setErr := s.cache.SetByTitle(ctx, title, aggregated); setErr != nil {
			return nil, fmt.Errorf("cache reactions by title %q: %w", title, setErr)
		}
	}

	return reactionsByTitle, nil
}

// ValidateReviewsListQuery validates and converts reviews list query parameters.
func (s *EventService) ValidateReviewsListQuery(query EventReviewsQuery) (uint64, uint64, string, error) {
	var limit uint64
	if strings.TrimSpace(query.Limit) != "" {
		parsedLimit, err := strconv.ParseUint(strings.TrimSpace(query.Limit), 10, 64)
		if err != nil {
			return 0, 0, "limit", ErrInvalidParameter
		}
		limit = parsedLimit
	}

	var offset uint64
	if strings.TrimSpace(query.Offset) != "" {
		parsedOffset, err := strconv.ParseUint(strings.TrimSpace(query.Offset), 10, 64)
		if err != nil {
			return 0, 0, "offset", ErrInvalidParameter
		}
		offset = parsedOffset
	}

	return limit, offset, "", nil
}

// CreateReview validates and stores a new event review.
func (s *EventService) CreateReview(
	ctx context.Context,
	eventID, userID, comment string,
	rating int,
	now time.Time,
) (model.EventReview, string, error) {
	if s.reviews == nil || s.reviewCache == nil {
		return model.EventReview{}, "", fmt.Errorf("review storage is not configured")
	}

	trimmedEventID := strings.TrimSpace(eventID)
	if trimmedEventID == "" {
		return model.EventReview{}, "event_id", ErrInvalidField
	}

	if strings.TrimSpace(userID) == "" {
		return model.EventReview{}, "user_id", ErrInvalidField
	}

	trimmedComment := strings.TrimSpace(comment)
	if trimmedComment == "" {
		return model.EventReview{}, "comment", ErrInvalidField
	}
	if len([]rune(trimmedComment)) > 300 {
		return model.EventReview{}, "comment", ErrInvalidField
	}
	if rating < 1 || rating > 5 {
		return model.EventReview{}, "rating", ErrInvalidField
	}

	event, err := s.GetByID(ctx, trimmedEventID)
	if err != nil {
		return model.EventReview{}, "", err
	}

	review, err := s.reviews.Create(ctx, trimmedEventID, strings.TrimSpace(userID), trimmedComment, rating, now)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return model.EventReview{}, "", ErrAlreadyExists
		}
		return model.EventReview{}, "", fmt.Errorf("create review: %w", err)
	}

	if err := s.reviewCache.DeleteByTitle(ctx, event.Title); err != nil {
		return model.EventReview{}, "", fmt.Errorf("invalidate reviews cache: %w", err)
	}
	if _, err := s.BuildReviewsByTitle(ctx, []model.Event{event}); err != nil {
		return model.EventReview{}, "", fmt.Errorf("refresh reviews cache: %w", err)
	}

	return review, "", nil
}

// ListReviews returns event reviews with pagination.
func (s *EventService) ListReviews(
	ctx context.Context,
	eventID string,
	limit, offset uint64,
) ([]model.EventReview, error) {
	if s.reviews == nil {
		return []model.EventReview{}, nil
	}

	trimmedEventID := strings.TrimSpace(eventID)
	if trimmedEventID == "" {
		return nil, ErrNotFound
	}

	reviews, err := s.reviews.ListByEventID(ctx, trimmedEventID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list event reviews: %w", err)
	}

	return reviews, nil
}

// UpdateReview updates user review for event.
func (s *EventService) UpdateReview(
	ctx context.Context,
	eventID, reviewID, userID string,
	input UpdateEventReviewInput,
	now time.Time,
) (string, error) {
	if s.reviews == nil || s.reviewCache == nil {
		return "", fmt.Errorf("review storage is not configured")
	}

	trimmedEventID := strings.TrimSpace(eventID)
	trimmedReviewID := strings.TrimSpace(reviewID)
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedEventID == "" || trimmedReviewID == "" || trimmedUserID == "" {
		return "", ErrNotFound
	}

	event, err := s.GetByID(ctx, trimmedEventID)
	if err != nil {
		return "", err
	}

	review, err := s.reviews.GetByEventIDAndUserID(ctx, trimmedEventID, trimmedUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get user review: %w", err)
	}
	if review.ID != trimmedReviewID {
		return "", ErrNotFound
	}

	if input.HasComment {
		if input.Comment == nil {
			return "comment", ErrInvalidField
		}
		trimmedComment := strings.TrimSpace(*input.Comment)
		if trimmedComment == "" || len([]rune(trimmedComment)) > 300 {
			return "comment", ErrInvalidField
		}
		review.Comment = trimmedComment
	}

	if input.HasRating {
		if input.Rating == nil {
			return "rating", ErrInvalidField
		}
		if *input.Rating < 1 || *input.Rating > 5 {
			return "rating", ErrInvalidField
		}
		review.Rating = *input.Rating
	}

	review.UpdatedAt = now.UTC().Format(time.RFC3339)
	if err := s.reviews.Update(ctx, review); err != nil {
		return "", fmt.Errorf("update review: %w", err)
	}

	if err := s.reviewCache.DeleteByTitle(ctx, event.Title); err != nil {
		return "", fmt.Errorf("invalidate reviews cache: %w", err)
	}
	if _, err := s.BuildReviewsByTitle(ctx, []model.Event{event}); err != nil {
		return "", fmt.Errorf("refresh reviews cache: %w", err)
	}

	return "", nil
}

// BuildReviewsByTitle returns aggregated review counters for event titles from input list.
func (s *EventService) BuildReviewsByTitle(
	ctx context.Context,
	events []model.Event,
) (map[string]model.EventReviewsSummary, error) {
	if s.reviews == nil || s.reviewCache == nil {
		return map[string]model.EventReviewsSummary{}, nil
	}

	targetTitles := make(map[string]struct{}, len(events))
	for _, event := range events {
		title := strings.TrimSpace(event.Title)
		if title == "" {
			continue
		}
		targetTitles[title] = struct{}{}
	}
	if len(targetTitles) == 0 {
		return map[string]model.EventReviewsSummary{}, nil
	}

	allEvents, err := s.events.List(ctx, repository.EventFilter{})
	if err != nil {
		return nil, fmt.Errorf("list events for reviews aggregation: %w", err)
	}

	eventIDsByTitle := make(map[string][]string, len(targetTitles))
	for _, event := range allEvents {
		title := strings.TrimSpace(event.Title)
		if _, ok := targetTitles[title]; !ok {
			continue
		}

		eventIDsByTitle[title] = append(eventIDsByTitle[title], event.ID)
	}

	reviewsByTitle := make(map[string]model.EventReviewsSummary, len(targetTitles))
	for title := range targetTitles {
		cachedReviews, found, cacheErr := s.reviewCache.GetByTitle(ctx, title)
		if cacheErr != nil {
			return nil, fmt.Errorf("read reviews cache by title %q: %w", title, cacheErr)
		}
		if found {
			reviewsByTitle[title] = cachedReviews
			continue
		}

		eventIDs := eventIDsByTitle[title]
		if len(eventIDs) == 0 {
			reviewsByTitle[title] = model.EventReviewsSummary{}
			continue
		}

		countersByEventID, countErr := s.reviews.CountByEventIDs(ctx, eventIDs)
		if countErr != nil {
			return nil, fmt.Errorf("count reviews by event ids: %w", countErr)
		}

		summary := model.EventReviewsSummary{}
		var totalRating uint64
		for _, eventID := range eventIDs {
			counters, ok := countersByEventID[eventID]
			if !ok {
				continue
			}
			summary.Count += counters.Count
			totalRating += counters.TotalRating
		}
		if summary.Count > 0 {
			summary.Rating = roundToOne(float64(totalRating) / float64(summary.Count))
		}

		reviewsByTitle[title] = summary
		if summary.Count == 0 {
			continue
		}

		if setErr := s.reviewCache.SetByTitle(ctx, title, summary); setErr != nil {
			return nil, fmt.Errorf("cache reviews by title %q: %w", title, setErr)
		}
	}

	return reviewsByTitle, nil
}

// UpdateByOrganizer updates event fields for event organizer.
func (s *EventService) UpdateByOrganizer(
	ctx context.Context,
	id, organizerID string,
	input UpdateEventInput,
) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "id", ErrInvalidField
	}
	if strings.TrimSpace(organizerID) == "" {
		return "user_id", ErrInvalidField
	}

	patch := repository.EventPatch{}
	if input.Category != nil {
		category := strings.TrimSpace(strings.ToLower(*input.Category))
		if !isValidCategory(category) {
			return "category", ErrInvalidField
		}
		patch.Category = &category
	}

	if input.Price != nil {
		patch.Price = input.Price
	}

	if input.HasCity {
		if input.City == nil {
			return "city", ErrInvalidField
		}

		city := strings.TrimSpace(*input.City)
		if city == "" {
			patch.RemoveCity = true
		} else {
			patch.City = &city
		}
	}

	updated, err := s.events.UpdateByIDAndOrganizer(ctx, id, organizerID, patch)
	if err != nil {
		return "", fmt.Errorf("update event by organizer: %w", err)
	}
	if !updated {
		return "", ErrNotFound
	}

	return "", nil
}

// parseYYYYMMDD parses a YYYYMMDD string to a time.Time.
// e.g. 20060102 -> 01/02 03:04:05PM 2006 -0700.
func parseYYYYMMDD(value string) (time.Time, error) {
	return time.Parse("20060102", value)
}

func roundToOne(value float64) float64 {
	return math.Round(value*10) / 10
}

func isValidCategory(value string) bool {
	switch value {
	case "meetup", "concert", "exhibition", "party", "other":
		return true
	default:
		return false
	}
}
