package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
	"github.com/timermakov/ndbx-lab-ermakov/internal/repository"
)

// EventService implements event business logic.
type EventService struct {
	events repository.EventRepository
}

// NewEventService creates a new EventService.
func NewEventService(events repository.EventRepository) *EventService {
	return &EventService{events: events}
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
	Title  string
	Limit  string
	Offset string
}

// ValidateListQuery validates and converts list query parameters.
func (s *EventService) ValidateListQuery(query EventsQuery) (repository.EventFilter, string, error) {
	filter := repository.EventFilter{
		Title: strings.TrimSpace(query.Title),
	}

	if strings.TrimSpace(query.Limit) != "" {
		limit, err := strconv.ParseUint(query.Limit, 10, 64)
		if err != nil {
			return repository.EventFilter{}, "limit", ErrInvalidParameter
		}
		filter.Limit = limit
	}

	if strings.TrimSpace(query.Offset) != "" {
		offset, err := strconv.ParseUint(query.Offset, 10, 64)
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
	events, err := s.events.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	return events, nil
}
