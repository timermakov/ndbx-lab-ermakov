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

type eventReactionRepoStub struct {
	putFn             func(ctx context.Context, eventID, userID string, value model.ReactionValue, now time.Time) error
	countByEventIDsFn func(ctx context.Context, eventIDs []string) (map[string]model.EventReactions, error)
}

func (s eventReactionRepoStub) Put(
	ctx context.Context,
	eventID, userID string,
	value model.ReactionValue,
	now time.Time,
) error {
	if s.putFn != nil {
		return s.putFn(ctx, eventID, userID, value, now)
	}

	return nil
}

func (s eventReactionRepoStub) CountByEventIDs(
	ctx context.Context,
	eventIDs []string,
) (map[string]model.EventReactions, error) {
	if s.countByEventIDsFn != nil {
		return s.countByEventIDsFn(ctx, eventIDs)
	}

	return map[string]model.EventReactions{}, nil
}

type eventReactionCacheStub struct {
	getByTitleFn    func(ctx context.Context, title string) (model.EventReactions, bool, error)
	setByTitleFn    func(ctx context.Context, title string, reactions model.EventReactions) error
	deleteByTitleFn func(ctx context.Context, title string) error
}

func (s eventReactionCacheStub) GetByTitle(ctx context.Context, title string) (model.EventReactions, bool, error) {
	if s.getByTitleFn != nil {
		return s.getByTitleFn(ctx, title)
	}

	return model.EventReactions{}, false, nil
}

func (s eventReactionCacheStub) SetByTitle(ctx context.Context, title string, reactions model.EventReactions) error {
	if s.setByTitleFn != nil {
		return s.setByTitleFn(ctx, title, reactions)
	}

	return nil
}

type eventReviewRepoStub struct {
	createFn                func(ctx context.Context, eventID, userID, comment string, rating int, now time.Time) (model.EventReview, error)
	getByEventIDAndUserIDFn func(ctx context.Context, eventID, userID string) (model.EventReview, error)
	listByEventIDFn         func(ctx context.Context, eventID string, limit, offset uint64) ([]model.EventReview, error)
	updateFn                func(ctx context.Context, review model.EventReview) error
	countByEventIDsFn       func(ctx context.Context, eventIDs []string) (map[string]model.EventReviewsCounters, error)
}

func (s eventReviewRepoStub) Create(
	ctx context.Context,
	eventID, userID, comment string,
	rating int,
	now time.Time,
) (model.EventReview, error) {
	if s.createFn != nil {
		return s.createFn(ctx, eventID, userID, comment, rating, now)
	}

	return model.EventReview{}, nil
}

func (s eventReviewRepoStub) GetByEventIDAndUserID(ctx context.Context, eventID, userID string) (model.EventReview, error) {
	if s.getByEventIDAndUserIDFn != nil {
		return s.getByEventIDAndUserIDFn(ctx, eventID, userID)
	}

	return model.EventReview{}, repository.ErrNotFound
}

func (s eventReviewRepoStub) ListByEventID(
	ctx context.Context,
	eventID string,
	limit, offset uint64,
) ([]model.EventReview, error) {
	if s.listByEventIDFn != nil {
		return s.listByEventIDFn(ctx, eventID, limit, offset)
	}

	return []model.EventReview{}, nil
}

func (s eventReviewRepoStub) Update(ctx context.Context, review model.EventReview) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, review)
	}

	return nil
}

func (s eventReviewRepoStub) CountByEventIDs(
	ctx context.Context,
	eventIDs []string,
) (map[string]model.EventReviewsCounters, error) {
	if s.countByEventIDsFn != nil {
		return s.countByEventIDsFn(ctx, eventIDs)
	}

	return map[string]model.EventReviewsCounters{}, nil
}

type eventReviewCacheStub struct {
	getByTitleFn    func(ctx context.Context, title string) (model.EventReviewsSummary, bool, error)
	setByTitleFn    func(ctx context.Context, title string, reviews model.EventReviewsSummary) error
	deleteByTitleFn func(ctx context.Context, title string) error
}

type recommendationGraphStub struct {
	upsertUserFn            func(ctx context.Context, userID string) error
	upsertEventFn           func(ctx context.Context, eventID, title, startedAt string) error
	upsertLikeFn            func(ctx context.Context, userID, eventID string) error
	listRecommendedEventIDs func(ctx context.Context, userID string) ([]string, error)
}

func (s recommendationGraphStub) UpsertUser(ctx context.Context, userID string) error {
	if s.upsertUserFn != nil {
		return s.upsertUserFn(ctx, userID)
	}

	return nil
}

func (s recommendationGraphStub) UpsertEvent(ctx context.Context, eventID, title, startedAt string) error {
	if s.upsertEventFn != nil {
		return s.upsertEventFn(ctx, eventID, title, startedAt)
	}

	return nil
}

func (s recommendationGraphStub) UpsertLike(ctx context.Context, userID, eventID string) error {
	if s.upsertLikeFn != nil {
		return s.upsertLikeFn(ctx, userID, eventID)
	}

	return nil
}

func (s recommendationGraphStub) ListRecommendedEventIDs(ctx context.Context, userID string) ([]string, error) {
	if s.listRecommendedEventIDs != nil {
		return s.listRecommendedEventIDs(ctx, userID)
	}

	return []string{}, nil
}

type recommendationCacheStub struct {
	getByUserIDFn func(ctx context.Context, userID string) ([]model.Event, bool, error)
	setByUserIDFn func(ctx context.Context, userID string, events []model.Event) error
}

func (s recommendationCacheStub) GetByUserID(ctx context.Context, userID string) ([]model.Event, bool, error) {
	if s.getByUserIDFn != nil {
		return s.getByUserIDFn(ctx, userID)
	}

	return nil, false, nil
}

func (s recommendationCacheStub) SetByUserID(ctx context.Context, userID string, events []model.Event) error {
	if s.setByUserIDFn != nil {
		return s.setByUserIDFn(ctx, userID, events)
	}

	return nil
}

func (s eventReviewCacheStub) GetByTitle(ctx context.Context, title string) (model.EventReviewsSummary, bool, error) {
	if s.getByTitleFn != nil {
		return s.getByTitleFn(ctx, title)
	}

	return model.EventReviewsSummary{}, false, nil
}

func (s eventReviewCacheStub) SetByTitle(ctx context.Context, title string, reviews model.EventReviewsSummary) error {
	if s.setByTitleFn != nil {
		return s.setByTitleFn(ctx, title, reviews)
	}

	return nil
}

func (s eventReviewCacheStub) DeleteByTitle(ctx context.Context, title string) error {
	if s.deleteByTitleFn != nil {
		return s.deleteByTitleFn(ctx, title)
	}

	return nil
}

func (s eventReactionCacheStub) DeleteByTitle(ctx context.Context, title string) error {
	if s.deleteByTitleFn != nil {
		return s.deleteByTitleFn(ctx, title)
	}

	return nil
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

func TestEventServicePutReactionNotFound(t *testing.T) {
	t.Parallel()

	svc := service.NewEventService(eventRepoStub{
		getByIDFn: func(context.Context, string) (model.Event, error) {
			return model.Event{}, repository.ErrNotFound
		},
	}, eventUserRepoStub{})
	svc.SetReactionsStorage(eventReactionRepoStub{}, eventReactionCacheStub{})

	err := svc.PutReaction(context.Background(), "event-1", "user-1", model.ReactionLike, time.Now())
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEventServicePutReactionUpdatesRepositoryAndCache(t *testing.T) {
	t.Parallel()

	var putCalled bool
	var deleteCalled bool

	svc := service.NewEventService(eventRepoStub{
		getByIDFn: func(context.Context, string) (model.Event, error) {
			return model.Event{ID: "event-1", Title: "The Event"}, nil
		},
	}, eventUserRepoStub{})
	svc.SetReactionsStorage(
		eventReactionRepoStub{
			putFn: func(_ context.Context, eventID, userID string, value model.ReactionValue, _ time.Time) error {
				putCalled = true
				if eventID != "event-1" || userID != "user-1" {
					t.Fatalf("unexpected reaction put args: %q %q", eventID, userID)
				}
				if value != model.ReactionLike {
					t.Fatalf("unexpected reaction value: %d", value)
				}

				return nil
			},
		},
		eventReactionCacheStub{
			deleteByTitleFn: func(_ context.Context, title string) error {
				deleteCalled = true
				if title != "The Event" {
					t.Fatalf("unexpected cache title %q", title)
				}

				return nil
			},
		},
	)

	if err := svc.PutReaction(context.Background(), "event-1", "user-1", model.ReactionLike, time.Now()); err != nil {
		t.Fatalf("put reaction failed: %v", err)
	}
	if !putCalled {
		t.Fatalf("expected reactions repository put call")
	}
	if !deleteCalled {
		t.Fatalf("expected reactions cache delete call")
	}
}

func TestEventServiceBuildReactionsByTitle(t *testing.T) {
	t.Parallel()

	svc := service.NewEventService(eventRepoStub{
		listFn: func(_ context.Context, _ repository.EventFilter) ([]model.Event, error) {
			return []model.Event{
				{ID: "event-1", Title: "The Event"},
				{ID: "event-2", Title: "The Event"},
				{ID: "event-3", Title: "Other Event"},
			}, nil
		},
	}, eventUserRepoStub{})

	svc.SetReactionsStorage(
		eventReactionRepoStub{
			countByEventIDsFn: func(_ context.Context, eventIDs []string) (map[string]model.EventReactions, error) {
				if len(eventIDs) != 2 {
					t.Fatalf("unexpected event ids count: %d", len(eventIDs))
				}

				return map[string]model.EventReactions{
					"event-1": {Likes: 2, Dislikes: 1},
					"event-2": {Likes: 1, Dislikes: 0},
				}, nil
			},
		},
		eventReactionCacheStub{
			getByTitleFn: func(_ context.Context, title string) (model.EventReactions, bool, error) {
				if title != "The Event" {
					t.Fatalf("unexpected cache lookup title: %q", title)
				}

				return model.EventReactions{}, false, nil
			},
			setByTitleFn: func(_ context.Context, title string, reactions model.EventReactions) error {
				if title != "The Event" {
					t.Fatalf("unexpected cache set title: %q", title)
				}
				if reactions.Likes != 3 || reactions.Dislikes != 1 {
					t.Fatalf("unexpected cached reactions: %+v", reactions)
				}

				return nil
			},
		},
	)

	reactionsByTitle, err := svc.BuildReactionsByTitle(context.Background(), []model.Event{{ID: "event-1", Title: "The Event"}})
	if err != nil {
		t.Fatalf("build reactions failed: %v", err)
	}

	reactions := reactionsByTitle["The Event"]
	if reactions.Likes != 3 || reactions.Dislikes != 1 {
		t.Fatalf("unexpected reactions result: %+v", reactions)
	}
}

func TestEventServiceCreateReviewConflict(t *testing.T) {
	t.Parallel()

	svc := service.NewEventService(eventRepoStub{
		getByIDFn: func(context.Context, string) (model.Event, error) {
			return model.Event{ID: "event-1", Title: "The Event"}, nil
		},
	}, eventUserRepoStub{})
	svc.SetReviewsStorage(
		eventReviewRepoStub{
			createFn: func(context.Context, string, string, string, int, time.Time) (model.EventReview, error) {
				return model.EventReview{}, repository.ErrAlreadyExists
			},
		},
		eventReviewCacheStub{},
	)

	_, _, err := svc.CreateReview(context.Background(), "event-1", "user-1", "good", 5, time.Now())
	if !errors.Is(err, service.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestEventServiceBuildReviewsByTitle(t *testing.T) {
	t.Parallel()

	svc := service.NewEventService(eventRepoStub{
		listFn: func(_ context.Context, _ repository.EventFilter) ([]model.Event, error) {
			return []model.Event{
				{ID: "event-1", Title: "The Event"},
				{ID: "event-2", Title: "The Event"},
				{ID: "event-3", Title: "Other Event"},
			}, nil
		},
	}, eventUserRepoStub{})

	svc.SetReviewsStorage(
		eventReviewRepoStub{
			countByEventIDsFn: func(_ context.Context, eventIDs []string) (map[string]model.EventReviewsCounters, error) {
				if len(eventIDs) != 2 {
					t.Fatalf("unexpected event ids count: %d", len(eventIDs))
				}

				return map[string]model.EventReviewsCounters{
					"event-1": {Count: 2, TotalRating: 7},
					"event-2": {Count: 1, TotalRating: 2},
				}, nil
			},
		},
		eventReviewCacheStub{
			getByTitleFn: func(_ context.Context, title string) (model.EventReviewsSummary, bool, error) {
				if title != "The Event" {
					t.Fatalf("unexpected cache lookup title: %q", title)
				}
				return model.EventReviewsSummary{}, false, nil
			},
			setByTitleFn: func(_ context.Context, title string, reviews model.EventReviewsSummary) error {
				if title != "The Event" {
					t.Fatalf("unexpected cache set title: %q", title)
				}
				if reviews.Count != 3 || reviews.Rating != 3 {
					t.Fatalf("unexpected cached reviews: %+v", reviews)
				}
				return nil
			},
		},
	)

	reviewsByTitle, err := svc.BuildReviewsByTitle(context.Background(), []model.Event{{ID: "event-1", Title: "The Event"}})
	if err != nil {
		t.Fatalf("build reviews failed: %v", err)
	}

	reviews := reviewsByTitle["The Event"]
	if reviews.Count != 3 || reviews.Rating != 3 {
		t.Fatalf("unexpected reviews result: %+v", reviews)
	}
}

func TestEventServicePutReactionLikeUpdatesRecommendationGraph(t *testing.T) {
	t.Parallel()

	var upsertEventCalled bool
	var upsertLikeCalled bool

	svc := service.NewEventService(eventRepoStub{
		getByIDFn: func(context.Context, string) (model.Event, error) {
			return model.Event{
				ID:        "event-1",
				Title:     "The Event",
				StartedAt: "2026-03-24T10:00:00Z",
			}, nil
		},
	}, eventUserRepoStub{})
	svc.SetReactionsStorage(eventReactionRepoStub{}, eventReactionCacheStub{})
	svc.SetRecommendationsStorage(
		recommendationGraphStub{
			upsertEventFn: func(_ context.Context, eventID, title, startedAt string) error {
				upsertEventCalled = true
				if eventID != "event-1" || title != "The Event" || startedAt != "2026-03-24T10:00:00Z" {
					t.Fatalf("unexpected upsert event args: %q %q %q", eventID, title, startedAt)
				}

				return nil
			},
			upsertLikeFn: func(_ context.Context, userID, eventID string) error {
				upsertLikeCalled = true
				if userID != "user-1" || eventID != "event-1" {
					t.Fatalf("unexpected upsert like args: %q %q", userID, eventID)
				}

				return nil
			},
		},
		recommendationCacheStub{},
	)

	if err := svc.PutReaction(context.Background(), "event-1", "user-1", model.ReactionLike, time.Now()); err != nil {
		t.Fatalf("put reaction failed: %v", err)
	}
	if !upsertEventCalled {
		t.Fatalf("expected recommendation upsert event call")
	}
	if !upsertLikeCalled {
		t.Fatalf("expected recommendation upsert like call")
	}
}

func TestEventServiceListRecommendationsCacheMiss(t *testing.T) {
	t.Parallel()

	var cacheSetCalled bool
	svc := service.NewEventService(eventRepoStub{
		getByIDFn: func(_ context.Context, id string) (model.Event, error) {
			switch id {
			case "event-2":
				return model.Event{ID: "event-2", Title: "Title 2"}, nil
			case "event-4":
				return model.Event{ID: "event-4", Title: "Title 4"}, nil
			default:
				return model.Event{}, repository.ErrNotFound
			}
		},
	}, eventUserRepoStub{})
	svc.SetRecommendationsStorage(
		recommendationGraphStub{
			listRecommendedEventIDs: func(_ context.Context, userID string) ([]string, error) {
				if userID != "user-1" {
					t.Fatalf("unexpected user id %q", userID)
				}

				return []string{"event-2", "event-4"}, nil
			},
		},
		recommendationCacheStub{
			getByUserIDFn: func(_ context.Context, userID string) ([]model.Event, bool, error) {
				if userID != "user-1" {
					t.Fatalf("unexpected cache user id %q", userID)
				}

				return nil, false, nil
			},
			setByUserIDFn: func(_ context.Context, userID string, events []model.Event) error {
				cacheSetCalled = true
				if userID != "user-1" {
					t.Fatalf("unexpected cache set user id %q", userID)
				}
				if len(events) != 2 {
					t.Fatalf("unexpected recommendations count %d", len(events))
				}

				return nil
			},
		},
	)

	events, err := svc.ListRecommendations(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("list recommendations failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("unexpected recommendations result count %d", len(events))
	}
	if !cacheSetCalled {
		t.Fatalf("expected cache set call")
	}
}

func TestEventServiceUpdateReviewByOwner(t *testing.T) {
	t.Parallel()

	var updated model.EventReview
	svc := service.NewEventService(eventRepoStub{
		getByIDFn: func(context.Context, string) (model.Event, error) {
			return model.Event{ID: "event-1", Title: "The Event"}, nil
		},
	}, eventUserRepoStub{})
	svc.SetReviewsStorage(
		eventReviewRepoStub{
			getByEventIDAndUserIDFn: func(context.Context, string, string) (model.EventReview, error) {
				return model.EventReview{
					ID:        "review-1",
					EventID:   "event-1",
					Comment:   "old",
					CreatedBy: "user-1",
					Rating:    2,
					UpdatedAt: "2026-01-01T00:00:00Z",
				}, nil
			},
			updateFn: func(_ context.Context, review model.EventReview) error {
				updated = review
				return nil
			},
		},
		eventReviewCacheStub{},
	)

	comment := "new"
	rating := 5
	_, err := svc.UpdateReview(
		context.Background(),
		"event-1",
		"review-1",
		"user-1",
		service.UpdateEventReviewInput{
			Comment:    &comment,
			HasComment: true,
			Rating:     &rating,
			HasRating:  true,
		},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("update review failed: %v", err)
	}
	if updated.Comment != "new" || updated.Rating != 5 {
		t.Fatalf("unexpected updated review: %+v", updated)
	}
}
