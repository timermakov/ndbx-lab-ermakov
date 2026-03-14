// Package main is the entrypoint for the EventHub HTTP service.
//
//	@title		    EventHub API
//	@version	    1.0
//	@description	Backend service for the EventHub events platform.
//
//	@BasePath	    /
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/timermakov/ndbx-lab-ermakov/internal/handler"
	"github.com/timermakov/ndbx-lab-ermakov/internal/session"

	_ "github.com/timermakov/ndbx-lab-ermakov/docs"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func main() {
	logger := log.New(os.Stderr, "eventhub: ", log.LstdFlags|log.Lshortfile)

	host := env("APP_HOST")
	port := env("APP_PORT")
	if host == "" || port == "" {
		logger.Fatal("APP_HOST and APP_PORT must be set")
	}
	addr := net.JoinHostPort(host, port)

	sessionTTLSeconds, err := intFromEnv("APP_USER_SESSION_TTL")
	if err != nil {
		logger.Fatalf("invalid APP_USER_SESSION_TTL: %v", err)
	}
	redisDB, err := intFromEnv("REDIS_DB")
	if err != nil {
		logger.Fatalf("invalid REDIS_DB: %v", err)
	}

	redisAddr := net.JoinHostPort(env("REDIS_HOST"), env("REDIS_PORT"))
	if redisAddr == "" {
		logger.Fatal("REDIS_HOST and REDIS_PORT must be set")
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: env("REDIS_PASSWORD"),
		DB:       redisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatalf("redis ping failed: %v", err)
	}

	store := session.NewRedisStore(redisClient, time.Duration(sessionTTLSeconds)*time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /session", handler.NewSessionHandler(store, sessionTTLSeconds))
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatalf("shutdown error: %v", err)
	}
	logger.Println("server stopped")
}

// env returns the value of the environment variable or empty string if not set.
func env(key string) string {
	return os.Getenv(key)
}

// intFromEnv parses the environment variable value as an integer.
func intFromEnv(key string) (int, error) {
	value := env(key)
	if value == "" {
		return 0, fmt.Errorf("environment variable %s is not set", key)
	}

	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return v, nil
}
