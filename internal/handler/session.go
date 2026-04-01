package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/session"
)

const sessionCookieName = "X-Session-Id"

// NewSessionHandler returns an HTTP handler for managing anonymous sessions.
//
//	@Summary		Manage anonymous session
//	@Description	Creates a new anonymous session or refreshes an existing one.
//	@Tags			session
//	@Accept			json
//	@Produce		json
//	@Success		200	"Session refreshed"
//	@Success		201	"Session created"
//	@Failure		500	"Internal server error"
//	@Router			/session [post]
func NewSessionHandler(store session.Store, ttlSeconds int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Length", "0")

		ctx := r.Context()
		now := time.Now()

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !session.ValidateID(cookie.Value) {
			// No valid cookie: create new session.
			if err := createNewSession(ctx, w, store, now, ttlSeconds); err != nil {
				log.Printf("session: create error: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			return
		}

		id := cookie.Value

		existing, ok, err := store.Get(ctx, id)
		if err != nil {
			log.Printf("session: get error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !ok {
			// Session does not exist (expired) – create a new one.
			if err := createNewSession(ctx, w, store, now, ttlSeconds); err != nil {
				log.Printf("session: create after missing error: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			return
		}

		_ = existing

		// Session exists – refresh TTL.
		if _, err := store.Touch(ctx, id, now); err != nil {
			log.Printf("session: touch error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		setSessionCookie(w, id, ttlSeconds)
		w.WriteHeader(http.StatusOK)
	}
}

func createNewSession(ctx context.Context, w http.ResponseWriter, store session.Store, now time.Time, ttlSeconds int) error {
	for i := 0; i < 3; i++ {
		id, err := session.GenerateID()
		if err != nil {
			return err
		}

		if _, err := store.Create(ctx, id, now); err != nil {
			log.Printf("session: create attempt %d failed: %v", i+1, err)
			continue
		}

		setSessionCookie(w, id, ttlSeconds)
		return nil
	}

	return fmt.Errorf("could not create session after retries")
}

func setSessionCookie(w http.ResponseWriter, id string, ttlSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   ttlSeconds,
	})
}
