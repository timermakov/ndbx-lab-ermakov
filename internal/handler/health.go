// Package handler provides HTTP request handlers for the EventHub service.
package handler

import (
	"io"
	"log"
	"net/http"
)

// Health handles GET /health requests.
//
//	@Summary		Service healthcheck
//	@Description	Returns service availability status
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	map[string]string	"{"status":"ok"}"
//	@Router			/health [get]
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, `{"status":"ok"}`); err != nil {
		log.Printf("health: write error: %v", err)
	}
}
