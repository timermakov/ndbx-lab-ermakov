package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

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
		Title:  r.URL.Query().Get("title"),
		Limit:  r.URL.Query().Get("limit"),
		Offset: r.URL.Query().Get("offset"),
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidParameter) {
			if sessionID, _, ok, _ := h.requireSession(r); ok {
				setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			}
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidParameter + `" parameter`,
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

	responseEvents := make([]any, 0, len(events))
	for _, event := range events {
		responseEvents = append(responseEvents, map[string]any{
			"id":          event.ID,
			"title":       event.Title,
			"description": event.Description,
			"location": map[string]any{
				"address": event.Location.Address,
			},
			"created_at":  event.CreatedAt,
			"created_by":  event.CreatedBy,
			"started_at":  event.StartedAt,
			"finished_at": event.FinishedAt,
		})
	}

	writeJSON(w, http.StatusOK, eventsResponse{
		Events: responseEvents,
		Count:  len(responseEvents),
	})
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
