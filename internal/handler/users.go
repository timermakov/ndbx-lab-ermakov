package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/service"
	"github.com/timermakov/ndbx-lab-ermakov/internal/session"
)

// UsersHandler handles POST /users requests.
type UsersHandler struct {
	users      *service.UserService
	events     *service.EventService
	sessions   session.Store
	ttlSeconds int
}

// NewUsersHandler creates a users registration handler.
func NewUsersHandler(
	users *service.UserService,
	events *service.EventService,
	sessions session.Store,
	ttlSeconds int,
) *UsersHandler {
	return &UsersHandler{
		users:      users,
		events:     events,
		sessions:   sessions,
		ttlSeconds: ttlSeconds,
	}
}

// Register handles user registration.
func (h *UsersHandler) Register(w http.ResponseWriter, r *http.Request) {
	type registerRequest struct {
		FullName string `json:"full_name"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.touchExistingSessionIfPossible(w, r)
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "body" field`})
		return
	}

	user, invalidField, err := h.users.Register(r.Context(), service.RegisterInput{
		FullName: req.FullName,
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidField):
			h.touchExistingSessionIfPossible(w, r)
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidField + `" field`,
			})
		case errors.Is(err, service.ErrAlreadyExists):
			h.touchExistingSessionIfPossible(w, r)
			writeJSON(w, http.StatusConflict, errorResponse{Message: "user already exists"})
		default:
			log.Printf("users register: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	now := time.Now()
	sessionID, createErr := createSession(r.Context(), h.sessions, now)
	if createErr != nil {
		log.Printf("users register create session: %v", createErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, bindErr := h.sessions.BindUser(r.Context(), sessionID, user.ID, now); bindErr != nil {
		log.Printf("users register bind session user: %v", bindErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	w.WriteHeader(http.StatusCreated)
}

func (h *UsersHandler) touchExistingSessionIfPossible(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := getValidSessionCookie(r)
	if !ok {
		return
	}

	if _, touchErr := h.sessions.Touch(r.Context(), sessionID, time.Now()); touchErr != nil {
		return
	}

	setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
}

// List handles GET /users.
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	type usersResponse struct {
		Users []any `json:"users"`
		Count int   `json:"count"`
	}

	filter, invalidParameter, err := h.users.ValidateListQuery(service.UsersQuery{
		ID:     r.URL.Query().Get("id"),
		Name:   r.URL.Query().Get("name"),
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

		log.Printf("users list query: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	users, listErr := h.users.List(r.Context(), filter)
	if listErr != nil {
		log.Printf("users list: %v", listErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.touchExistingSessionIfPossible(w, r)

	responseUsers := make([]any, 0, len(users))
	for _, user := range users {
		responseUsers = append(responseUsers, map[string]any{
			"id":        user.ID,
			"full_name": user.FullName,
			"username":  user.Username,
		})
	}

	writeJSON(w, http.StatusOK, usersResponse{
		Users: responseUsers,
		Count: len(responseUsers),
	})
}

// GetByID handles GET /users/{id}.
func (h *UsersHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("id"))
	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			h.touchExistingSessionIfPossible(w, r)
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "Not found"})
			return
		}

		log.Printf("users get by id: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.touchExistingSessionIfPossible(w, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        user.ID,
		"full_name": user.FullName,
		"username":  user.Username,
	})
}

// ListEvents handles GET /users/{id}/events.
func (h *UsersHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	type eventsResponse struct {
		Events []any `json:"events"`
		Count  int   `json:"count"`
	}

	userID := strings.TrimSpace(r.PathValue("id"))
	if _, err := h.users.GetByID(r.Context(), userID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			h.touchExistingSessionIfPossible(w, r)
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "User not found"})
			return
		}

		log.Printf("users events get user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
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
		Limit:     r.URL.Query().Get("limit"),
		Offset:    r.URL.Query().Get("offset"),
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidParameter) {
			h.touchExistingSessionIfPossible(w, r)
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidParameter + `" field`,
			})
			return
		}

		log.Printf("users events list query: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	filter.CreatedBy = userID
	filter.CreatedByName = ""

	events, listErr := h.events.List(r.Context(), filter)
	if listErr != nil {
		log.Printf("users events list: %v", listErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.touchExistingSessionIfPossible(w, r)

	responseEvents := make([]any, 0, len(events))
	for _, event := range events {
		responseEvents = append(responseEvents, eventToResponse(event))
	}

	writeJSON(w, http.StatusOK, eventsResponse{
		Events: responseEvents,
		Count:  len(responseEvents),
	})
}
