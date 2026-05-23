package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
	"github.com/timermakov/ndbx-lab-ermakov/internal/service"
	"github.com/timermakov/ndbx-lab-ermakov/internal/session"
)

// EventsHandler handles /events endpoints.
type EventsHandler struct {
	events     *service.EventService
	sessions   session.Store
	ttlSeconds int
}

// NewEventsHandler creates an events handler.
func NewEventsHandler(events *service.EventService, sessions session.Store, ttlSeconds int) *EventsHandler {
	return &EventsHandler{
		events:     events,
		sessions:   sessions,
		ttlSeconds: ttlSeconds,
	}
}

// Create handles POST /events.
func (h *EventsHandler) Create(w http.ResponseWriter, r *http.Request) {
	type createEventRequest struct {
		Title       string `json:"title"`
		Address     string `json:"address"`
		StartedAt   string `json:"started_at"`
		FinishedAt  string `json:"finished_at"`
		Description string `json:"description"`
	}
	type createEventResponse struct {
		ID string `json:"id"`
	}

	sessionID, sessionData, ok, err := h.requireSession(r)
	if err != nil {
		log.Printf("events create session check: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !ok || sessionData.UserID == "" {
		h.touchExistingSessionIfPossible(w, r)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req createEventRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "body" field`})
		return
	}

	event, invalidField, createErr := h.events.Create(r.Context(), service.CreateEventInput{
		Title:       req.Title,
		Address:     req.Address,
		Description: req.Description,
		StartedAt:   req.StartedAt,
		FinishedAt:  req.FinishedAt,
		CreatedBy:   sessionData.UserID,
	}, time.Now())
	if createErr != nil {
		switch {
		case errors.Is(createErr, service.ErrInvalidField):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidField + `" field`,
			})
		case errors.Is(createErr, service.ErrAlreadyExists):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusConflict, errorResponse{Message: "event already exists"})
		default:
			log.Printf("events create: %v", createErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if _, touchErr := h.sessions.Touch(r.Context(), sessionID, time.Now()); touchErr == nil {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	}

	writeJSON(w, http.StatusCreated, createEventResponse{ID: event.ID})
}

// List handles GET /events.
func (h *EventsHandler) List(w http.ResponseWriter, r *http.Request) {
	type eventsResponse struct {
		Events []any `json:"events"`
		Count  int   `json:"count"`
	}

	filter, invalidParameter, err := h.events.ValidateListQuery(service.EventsQuery{
		ID:        r.URL.Query().Get("id"),
		Title:     r.URL.Query().Get("title"),
		Category:  r.URL.Query().Get("category"),
		PriceFrom: r.URL.Query().Get("price_from"),
		PriceTo:   r.URL.Query().Get("price_to"),
		City:      r.URL.Query().Get("city"),
		DateFrom:  r.URL.Query().Get("date_from"),
		DateTo:    r.URL.Query().Get("date_to"),
		User:      r.URL.Query().Get("user"),
		Limit:     r.URL.Query().Get("limit"),
		Offset:    r.URL.Query().Get("offset"),
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidParameter) {
			if sessionID, _, ok, _ := h.requireSession(r); ok {
				setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			}
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidParameter + `" field`,
			})
			return
		}
		log.Printf("events list query: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	events, listErr := h.events.List(r.Context(), filter)
	if listErr != nil {
		log.Printf("events list: %v", listErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if sessionID, _, ok, _ := h.requireSession(r); ok {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	}

	includeReactions := hasIncludeReactions(r)
	includeReviews := hasIncludeReviews(r)
	reactionsByTitle := map[string]model.EventReactions{}
	if includeReactions {
		reactions, reactionsErr := h.events.BuildReactionsByTitle(r.Context(), events)
		if reactionsErr != nil {
			log.Printf("events list reactions: %v", reactionsErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		reactionsByTitle = reactions
	}
	reviewsByTitle := map[string]model.EventReviewsSummary{}
	if includeReviews {
		reviews, reviewsErr := h.events.BuildReviewsByTitle(r.Context(), events)
		if reviewsErr != nil {
			log.Printf("events list reviews: %v", reviewsErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		reviewsByTitle = reviews
	}

	responseEvents := make([]any, 0, len(events))
	for _, event := range events {
		var reactions *model.EventReactions
		if includeReactions {
			value := reactionsByTitle[event.Title]
			reactions = &value
		}
		var reviews *model.EventReviewsSummary
		if includeReviews {
			value := reviewsByTitle[event.Title]
			reviews = &value
		}

		responseEvents = append(responseEvents, eventToResponse(event, reactions, reviews))
	}

	writeJSON(w, http.StatusOK, eventsResponse{
		Events: responseEvents,
		Count:  len(responseEvents),
	})
}

// GetByID handles GET /events/{id}.
func (h *EventsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	eventID := strings.TrimSpace(r.PathValue("id"))
	event, err := h.events.GetByID(r.Context(), eventID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			if sessionID, _, ok, _ := h.requireSession(r); ok {
				setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			}
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "Not found"})
			return
		}

		log.Printf("events get by id: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if sessionID, _, ok, _ := h.requireSession(r); ok {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	}

	var reactions *model.EventReactions
	var reviews *model.EventReviewsSummary
	if hasIncludeReactions(r) {
		reactionsByTitle, reactionsErr := h.events.BuildReactionsByTitle(r.Context(), []model.Event{event})
		if reactionsErr != nil {
			log.Printf("events get by id reactions: %v", reactionsErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		value := reactionsByTitle[event.Title]
		reactions = &value
	}
	if hasIncludeReviews(r) {
		reviewsByTitle, reviewsErr := h.events.BuildReviewsByTitle(r.Context(), []model.Event{event})
		if reviewsErr != nil {
			log.Printf("events get by id reviews: %v", reviewsErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		value := reviewsByTitle[event.Title]
		reviews = &value
	}

	writeJSON(w, http.StatusOK, eventToResponse(event, reactions, reviews))
}

// Like handles POST /events/{id}/like.
func (h *EventsHandler) Like(w http.ResponseWriter, r *http.Request) {
	sessionID, sessionData, ok, err := h.requireSession(r)
	if err != nil {
		log.Printf("events like session check: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok || sessionData.UserID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	eventID := strings.TrimSpace(r.PathValue("id"))
	if likeErr := h.events.PutReaction(r.Context(), eventID, sessionData.UserID, model.ReactionLike, time.Now()); likeErr != nil {
		if errors.Is(likeErr, service.ErrNotFound) {
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "Event not found"})
			return
		}

		log.Printf("events like: %v", likeErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, touchErr := h.sessions.Touch(r.Context(), sessionID, time.Now()); touchErr != nil {
		log.Printf("events like touch session: %v", touchErr)
	}
	setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	w.WriteHeader(http.StatusNoContent)
}

// Dislike handles POST /events/{id}/dislike.
func (h *EventsHandler) Dislike(w http.ResponseWriter, r *http.Request) {
	sessionID, sessionData, ok, err := h.requireSession(r)
	if err != nil {
		log.Printf("events dislike session check: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok || sessionData.UserID == "" {
		expireSessionCookie(w, "")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	eventID := strings.TrimSpace(r.PathValue("id"))
	if dislikeErr := h.events.PutReaction(r.Context(), eventID, sessionData.UserID, model.ReactionDislike, time.Now()); dislikeErr != nil {
		if errors.Is(dislikeErr, service.ErrNotFound) {
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "Event not found"})
			return
		}

		log.Printf("events dislike: %v", dislikeErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, touchErr := h.sessions.Touch(r.Context(), sessionID, time.Now()); touchErr != nil {
		log.Printf("events dislike touch session: %v", touchErr)
	}
	setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	w.WriteHeader(http.StatusNoContent)
}

// CreateReview handles POST /events/{id}/reviews.
func (h *EventsHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	type createReviewRequest struct {
		Comment *string `json:"comment"`
		Rating  *int    `json:"rating"`
	}
	type createReviewResponse struct {
		ID string `json:"id"`
	}

	sessionID, sessionData, ok, err := h.requireSession(r)
	if err != nil {
		log.Printf("events create review session check: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok || sessionData.UserID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req createReviewRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "body" field`})
		return
	}
	if req.Comment == nil {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "comment" field`})
		return
	}
	if req.Rating == nil {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "rating" field`})
		return
	}

	review, invalidField, reviewErr := h.events.CreateReview(
		r.Context(),
		strings.TrimSpace(r.PathValue("id")),
		sessionData.UserID,
		*req.Comment,
		*req.Rating,
		time.Now(),
	)
	if reviewErr != nil {
		switch {
		case errors.Is(reviewErr, service.ErrInvalidField):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidField + `" field`,
			})
		case errors.Is(reviewErr, service.ErrAlreadyExists):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusConflict, errorResponse{Message: "Already exists"})
		case errors.Is(reviewErr, service.ErrNotFound):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "Event not found"})
		default:
			log.Printf("events create review: %v", reviewErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if _, touchErr := h.sessions.Touch(r.Context(), sessionID, time.Now()); touchErr != nil {
		log.Printf("events create review touch session: %v", touchErr)
	}
	setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	writeJSON(w, http.StatusCreated, createReviewResponse{ID: review.ID})
}

// ListReviews handles GET /events/{id}/reviews.
func (h *EventsHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	type listReviewsResponse struct {
		Reviews []model.EventReview `json:"reviews"`
		Count   int                 `json:"count"`
	}

	limit, offset, invalidParameter, err := h.events.ValidateReviewsListQuery(service.EventReviewsQuery{
		Limit:  r.URL.Query().Get("limit"),
		Offset: r.URL.Query().Get("offset"),
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidParameter) {
			h.touchExistingSessionIfPossible(w, r)
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidParameter + `" field`,
			})
			return
		}
		log.Printf("events list reviews query: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	reviews, listErr := h.events.ListReviews(r.Context(), strings.TrimSpace(r.PathValue("id")), limit, offset)
	if listErr != nil {
		log.Printf("events list reviews: %v", listErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.touchExistingSessionIfPossible(w, r)
	writeJSON(w, http.StatusOK, listReviewsResponse{
		Reviews: reviews,
		Count:   len(reviews),
	})
}

// PatchReview handles PATCH /events/{id}/reviews/{review_id}.
func (h *EventsHandler) PatchReview(w http.ResponseWriter, r *http.Request) {
	sessionID, sessionData, ok, err := h.requireSession(r)
	if err != nil {
		log.Printf("events patch review session check: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok || sessionData.UserID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	input, invalidField, decodeErr := decodeUpdateReviewRequest(r)
	if decodeErr != nil {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
		if invalidField == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "body" field`})
			return
		}
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "` + invalidField + `" field`})
		return
	}

	invalidField, updateErr := h.events.UpdateReview(
		r.Context(),
		strings.TrimSpace(r.PathValue("id")),
		strings.TrimSpace(r.PathValue("review_id")),
		sessionData.UserID,
		input,
		time.Now(),
	)
	if updateErr != nil {
		switch {
		case errors.Is(updateErr, service.ErrInvalidField):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidField + `" field`,
			})
		case errors.Is(updateErr, service.ErrNotFound):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "Event not found"})
		default:
			log.Printf("events patch review: %v", updateErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if _, touchErr := h.sessions.Touch(r.Context(), sessionID, time.Now()); touchErr != nil {
		log.Printf("events patch review touch session: %v", touchErr)
	}
	setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	w.WriteHeader(http.StatusNoContent)
}

// Patch handles PATCH /events/{id}.
func (h *EventsHandler) Patch(w http.ResponseWriter, r *http.Request) {
	sessionID, sessionData, ok, err := h.requireSession(r)
	if err != nil {
		log.Printf("events patch session check: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !ok || sessionData.UserID == "" {
		h.touchExistingSessionIfPossible(w, r)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	input, invalidField, decodeErr := decodeUpdateEventRequest(r)
	if decodeErr != nil {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
		if invalidField == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "body" field`})
			return
		}

		writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "` + invalidField + `" field`})
		return
	}

	eventID := strings.TrimSpace(r.PathValue("id"))
	invalidField, updateErr := h.events.UpdateByOrganizer(r.Context(), eventID, sessionData.UserID, input)
	if updateErr != nil {
		switch {
		case errors.Is(updateErr, service.ErrInvalidField):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidField + `" field`,
			})
		case errors.Is(updateErr, service.ErrNotFound):
			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusNotFound, errorResponse{
				Message: "Not found. Be sure that event exists and you are the organizer",
			})
		default:
			log.Printf("events patch: %v", updateErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if _, touchErr := h.sessions.Touch(r.Context(), sessionID, time.Now()); touchErr == nil {
		setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EventsHandler) requireSession(r *http.Request) (string, session.Session, bool, error) {
	sessionID, ok := getValidSessionCookie(r)
	if !ok {
		return "", session.Session{}, false, nil
	}

	sessionData, found, err := h.sessions.Get(r.Context(), sessionID)
	if err != nil {
		return "", session.Session{}, false, err
	}
	if !found {
		return "", session.Session{}, false, nil
	}

	return sessionID, sessionData, true, nil
}

func (h *EventsHandler) touchExistingSessionIfPossible(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := getValidSessionCookie(r)
	if !ok {
		return
	}

	if _, touchErr := h.sessions.Touch(r.Context(), sessionID, time.Now()); touchErr != nil {
		return
	}

	setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
}

func decodeUpdateEventRequest(r *http.Request) (service.UpdateEventInput, string, error) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return service.UpdateEventInput{}, "", err
	}

	input := service.UpdateEventInput{}

	if rawCategory, ok := body["category"]; ok {
		var category string
		if err := json.Unmarshal(rawCategory, &category); err != nil {
			return service.UpdateEventInput{}, "category", err
		}
		input.Category = &category
	}

	if rawPrice, ok := body["price"]; ok {
		var price uint64
		if err := json.Unmarshal(rawPrice, &price); err != nil {
			return service.UpdateEventInput{}, "price", err
		}
		input.Price = &price
	}

	if rawCity, ok := body["city"]; ok {
		input.HasCity = true
		var city string
		if err := json.Unmarshal(rawCity, &city); err != nil {
			return service.UpdateEventInput{}, "city", err
		}
		input.City = &city
	}

	return input, "", nil
}

func decodeUpdateReviewRequest(r *http.Request) (service.UpdateEventReviewInput, string, error) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return service.UpdateEventReviewInput{}, "", err
	}

	input := service.UpdateEventReviewInput{}

	if rawComment, ok := body["comment"]; ok {
		input.HasComment = true
		var comment string
		if err := json.Unmarshal(rawComment, &comment); err != nil {
			return service.UpdateEventReviewInput{}, "comment", err
		}
		input.Comment = &comment
	}

	if rawRating, ok := body["rating"]; ok {
		input.HasRating = true
		var rating int
		if err := json.Unmarshal(rawRating, &rating); err != nil {
			return service.UpdateEventReviewInput{}, "rating", err
		}
		input.Rating = &rating
	}

	return input, "", nil
}

func eventToResponse(
	event model.Event,
	reactions *model.EventReactions,
	reviews *model.EventReviewsSummary,
) map[string]any {
	location := map[string]any{
		"address": event.Location.Address,
	}
	if event.Location.City != "" {
		location["city"] = event.Location.City
	}

	response := map[string]any{
		"id":          event.ID,
		"title":       event.Title,
		"category":    event.Category,
		"price":       event.Price,
		"description": event.Description,
		"location":    location,
		"created_at":  event.CreatedAt,
		"created_by":  event.CreatedBy,
		"started_at":  event.StartedAt,
		"finished_at": event.FinishedAt,
	}
	if reactions != nil {
		response["reactions"] = map[string]uint64{
			"likes":    reactions.Likes,
			"dislikes": reactions.Dislikes,
		}
	}
	if reviews != nil {
		response["reviews"] = map[string]any{
			"count":  reviews.Count,
			"rating": reviews.Rating,
		}
	}

	return response
}
