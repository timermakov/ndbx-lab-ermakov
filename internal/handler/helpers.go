package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/session"
)

type errorResponse struct {
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func setSessionCookieWithMaxAge(w http.ResponseWriter, id string, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   maxAgeSeconds,
	})
}

func expireSessionCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   0,
	})
}

func createSession(ctx context.Context, store session.Store, now time.Time) (string, error) {
	for i := 0; i < 3; i++ {
		id, err := session.GenerateID()
		if err != nil {
			return "", fmt.Errorf("generate session id: %w", err)
		}

		if _, createErr := store.Create(ctx, id, now); createErr == nil {
			return id, nil
		}
	}

	return "", fmt.Errorf("could not create session after retries")
}

func getValidSessionCookie(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || !session.ValidateID(cookie.Value) {
		return "", false
	}

	return cookie.Value, true
}
