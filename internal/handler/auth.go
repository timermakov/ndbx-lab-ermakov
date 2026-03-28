package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/service"
	"github.com/timermakov/ndbx-lab-ermakov/internal/session"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	users      *service.UserService
	sessions   session.Store
	ttlSeconds int
}

// NewAuthHandler creates authentication handlers.
func NewAuthHandler(users *service.UserService, sessions session.Store, ttlSeconds int) *AuthHandler {
	return &AuthHandler{
		users:      users,
		sessions:   sessions,
		ttlSeconds: ttlSeconds,
	}
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	type loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: `invalid "body" field`})
		return
	}

	user, invalidField, err := h.users.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidField):
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Message: `invalid "` + invalidField + `" field`,
			})
		case errors.Is(err, service.ErrInvalidCredentials):
			sessionID, sessionErr := h.sessionIDForPost(r.Context(), r)
			if sessionErr != nil {
				log.Printf("auth login session create/touch: %v", sessionErr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
			writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "invalid credentials"})
		default:
			log.Printf("auth login: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	now := time.Now()
	sessionID, sessionErr := h.sessionIDForPost(r.Context(), r)
	if sessionErr != nil {
		log.Printf("auth login session create/touch: %v", sessionErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, bindErr := h.sessions.BindUser(r.Context(), sessionID, user.ID, now); bindErr != nil {
		log.Printf("auth login bind user: %v", bindErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	setSessionCookieWithMaxAge(w, sessionID, h.ttlSeconds)
	w.WriteHeader(http.StatusNoContent)
}

// Logout handles POST /auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := getValidSessionCookie(r)
	if ok {
		if err := h.sessions.Delete(r.Context(), sessionID); err != nil {
			log.Printf("auth logout delete session: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	expireID := sessionID
	if expireID == "" {
		expireID = "deleted"
	}

	expireSessionCookie(w, expireID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) sessionIDForPost(ctx context.Context, r *http.Request) (string, error) {
	now := time.Now()
	if sessionID, ok := getValidSessionCookie(r); ok {
		if _, err := h.sessions.Touch(ctx, sessionID, now); err == nil {
			return sessionID, nil
		}
	}

	return createSession(ctx, h.sessions, now)
}
