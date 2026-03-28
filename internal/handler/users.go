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

// UsersHandler handles POST /users requests.
type UsersHandler struct {
	users      *service.UserService
	sessions   session.Store
	ttlSeconds int
}

// NewUsersHandler creates a users registration handler.
func NewUsersHandler(users *service.UserService, sessions session.Store, ttlSeconds int) *UsersHandler {
	return &UsersHandler{
		users:      users,
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
