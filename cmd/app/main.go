// Package main is the entrypoint for the EventHub HTTP service.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/timermakov/ndbx-lab-ermakov/internal/handler"
)

func main() {
	host := env("APP_HOST")
	port := env("APP_PORT")
	addr := net.JoinHostPort(host, port)

	logger := log.New(os.Stderr, "eventhub: ", log.LstdFlags|log.Lshortfile)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		logger.Printf("server listening on %s", addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-listenErr:
		logger.Fatalf("listen error: %v", err)
	case <-quit:
		logger.Println("shutting down server...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("shutdown error: %v", err)
	}
	logger.Println("server stopped")
}

// env returns the value of the environment variable or empty string if not set.
func env(key string) string {
	return os.Getenv(key)
}
