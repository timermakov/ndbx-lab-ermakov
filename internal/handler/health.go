// Package handler provides HTTP request handlers for the EventHub service.
package handler

import (
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/timermakov/ndbx-lab-ermakov/internal/session"
)

// Health handles GET /health requests.
//
//	@Summary		Service healthcheck
//	@Description	Returns service availability status
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	map[string]string	"{"status":"ok"}"
//	@Router			/health [get]
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Echo existing session cookie without touching Redis TTL.
	if cookie, err := r.Cookie(sessionCookieName); err == nil && session.ValidateID(cookie.Value) {
		if ttl, err := strconv.Atoi(os.Getenv("APP_USER_SESSION_TTL")); err == nil && ttl > 0 {
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    cookie.Value,
				Path:     "/",
				HttpOnly: true,
				MaxAge:   ttl,
			})
		}
	}

	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, `{"status":"ok"}`); err != nil {
		log.Printf("health: write error: %v", err)
	}
}
