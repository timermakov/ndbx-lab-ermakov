package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
	"github.com/timermakov/ndbx-lab-ermakov/internal/repository"
)

// EventService implements event business logic.
type EventService struct {
	events    repository.EventRepository
	users     repository.UserRepository
	reactions repository.EventReactionRepository
	cache     repository.EventReactionCache
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

func isValidCategory(value string) bool {
	switch value {
	case "meetup", "concert", "exhibition", "party", "other":
		return true
	default:
		return false
	}
}
