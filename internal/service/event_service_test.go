package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
	"github.com/timermakov/ndbx-lab-ermakov/internal/repository"
	"github.com/timermakov/ndbx-lab-ermakov/internal/service"
)

type eventRepoStub struct {
	createFn            func(ctx context.Context, event model.Event) (model.Event, error)
	getByIDFn           func(ctx context.Context, id string) (model.Event, error)
	updateByOrganizerFn func(ctx context.Context, id, organizerID string, patch repository.EventPatch) (bool, error)
	listFn              func(ctx context.Context, filter repository.EventFilter) ([]model.Event, error)
}

func (s eventRepoStub) EnsureIndexes(context.Context) error {
	return nil
}

func (s eventRepoStub) Create(ctx context.Context, event model.Event) (model.Event, error) {
	if s.createFn != nil {
		return s.createFn(ctx, event)
	}
	return event, nil
}

func (s eventRepoStub) GetByID(ctx context.Context, id string) (model.Event, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return model.Event{}, repository.ErrNotFound
}

func (s eventRepoStub) UpdateByIDAndOrganizer(
	ctx context.Context,
	id, organizerID string,
	patch repository.EventPatch,
) (bool, error) {
	if s.updateByOrganizerFn != nil {
		return s.updateByOrganizerFn(ctx, id, organizerID, patch)
	}
	return false, nil
}

func (s eventRepoStub) List(ctx context.Context, filter repository.EventFilter) ([]model.Event, error) {
	if s.listFn != nil {
		return s.listFn(ctx, filter)
	}
	return []model.Event{}, nil
}

type eventUserRepoStub struct {
	getByUsernameFn func(ctx context.Context, username string) (model.User, error)
}

func (s eventUserRepoStub) EnsureIndexes(context.Context) error {
	return nil
}

func (s eventUserRepoStub) Create(context.Context, model.User) (model.User, error) {
	return model.User{}, nil
}

func (s eventUserRepoStub) GetByID(context.Context, string) (model.User, error) {
	return model.User{}, repository.ErrNotFound
}

func (s eventUserRepoStub) GetByUsername(ctx context.Context, username string) (model.User, error) {
	if s.getByUsernameFn != nil {
		return s.getByUsernameFn(ctx, username)
	}
	return model.User{}, repository.ErrNotFound
}

func (s eventUserRepoStub) List(context.Context, repository.UserFilter) ([]model.User, error) {
	return []model.User{}, nil
}

func TestEventServiceValidateListQueryInvalidLimit(t *testing.T) {
	t.Parallel()

	svc := service.NewEventService(eventRepoStub{}, eventUserRepoStub{})
	_, parameter, err := svc.ValidateListQuery(service.EventsQuery{
		Limit: "bad",
	})

	if !errors.Is(err, service.ErrInvalidParameter) {
		t.Fatalf("expected ErrInvalidParameter, got %v", err)
	}
	if parameter != "limit" {
		t.Fatalf("expected limit parameter, got %q", parameter)
	}
}

func TestEventServiceCreateInvalidDateOrder(t *testing.T) {
	t.Parallel()

	svc := service.NewEventService(eventRepoStub{}, eventUserRepoStub{})
	_, field, err := svc.Create(context.Background(), service.CreateEventInput{
		Title:      "party",
		Address:    "home",
		StartedAt:  "2026-01-10T10:00:00+03:00",
		FinishedAt: "2026-01-10T09:00:00+03:00",
		CreatedBy:  "user-1",
	}, time.Now())

	if !errors.Is(err, service.ErrInvalidField) {
		t.Fatalf("expected ErrInvalidField, got %v", err)
	}
	if field != "finished_at" {
		t.Fatalf("expected finished_at field, got %q", field)
	}
}

func TestEventServiceCreateConflict(t *testing.T) {
	t.Parallel()

	svc := service.NewEventService(eventRepoStub{
		createFn: func(_ context.Context, _ model.Event) (model.Event, error) {
			return model.Event{}, repository.ErrAlreadyExists
		},
	}, eventUserRepoStub{})

	_, _, err := svc.Create(context.Background(), service.CreateEventInput{
		Title:      "party",
		Address:    "home",
		StartedAt:  "2026-01-10T10:00:00+03:00",
		FinishedAt: "2026-01-10T11:00:00+03:00",
		CreatedBy:  "user-1",
	}, time.Now())

	if !errors.Is(err, service.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}
