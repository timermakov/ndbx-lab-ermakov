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
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/timermakov/ndbx-lab-ermakov/internal/config"
	"github.com/timermakov/ndbx-lab-ermakov/internal/handler"
	"github.com/timermakov/ndbx-lab-ermakov/internal/repository"
	"github.com/timermakov/ndbx-lab-ermakov/internal/service"
	"github.com/timermakov/ndbx-lab-ermakov/internal/session"

	_ "github.com/timermakov/ndbx-lab-ermakov/docs"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func main() {
	logger := log.New(os.Stderr, "eventhub: ", log.LstdFlags|log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}
	addr := net.JoinHostPort(cfg.AppHost, cfg.AppPort)
	redisAddr := net.JoinHostPort(cfg.RedisHost, cfg.RedisPort)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			logger.Printf("redis close failed: %v", closeErr)
		}
	}()

	if err := waitForRedis(redisClient, 30, time.Second); err != nil {
		logger.Fatalf("redis ping failed: %v", err)
	}

	mongoClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI(cfg)))
	if err != nil {
		logger.Fatalf("mongo connect failed: %v", err)
	}
	defer func() {
		disconnectCtx, cancelDisconnect := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelDisconnect()
		if disconnectErr := mongoClient.Disconnect(disconnectCtx); disconnectErr != nil {
			logger.Printf("mongo disconnect failed: %v", disconnectErr)
		}
	}()

	if err := waitForMongo(mongoClient, 30, time.Second); err != nil {
		logger.Fatalf("mongo ping failed: %v", err)
	}

	mongoDB := mongoClient.Database(cfg.MongoDatabase)
	userRepo := repository.NewMongoUserRepository(mongoDB)
	eventRepo := repository.NewMongoEventRepository(mongoDB)

	indexCtx, indexCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer indexCancel()
	if err := userRepo.EnsureIndexes(indexCtx); err != nil {
		logger.Fatalf("ensure user indexes failed: %v", err)
	}
	if err := eventRepo.EnsureIndexes(indexCtx); err != nil {
		logger.Fatalf("ensure event indexes failed: %v", err)
	}

	store := session.NewRedisStore(redisClient, time.Duration(cfg.AppUserSessionTTL)*time.Second)
	userService := service.NewUserService(userRepo)
	eventService := service.NewEventService(eventRepo)

	usersHandler := handler.NewUsersHandler(userService, store, cfg.AppUserSessionTTL)
	authHandler := handler.NewAuthHandler(userService, store, cfg.AppUserSessionTTL)
	eventsHandler := handler.NewEventsHandler(eventService, store, cfg.AppUserSessionTTL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /session", handler.NewSessionHandler(store, cfg.AppUserSessionTTL))
	mux.HandleFunc("POST /users", usersHandler.Register)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("POST /events", eventsHandler.Create)
	mux.HandleFunc("GET /events", eventsHandler.List)
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

func mongoURI(cfg config.Config) string {
	return fmt.Sprintf(
		"mongodb://%s:%s@%s:%s/?authSource=admin",
		cfg.MongoUser,
		cfg.MongoPassword,
		cfg.MongoHost,
		cfg.MongoPort,
	)
}

func waitForRedis(client *redis.Client, attempts int, delay time.Duration) error {
	var lastErr error

	for i := 0; i < attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		lastErr = client.Ping(ctx).Err()
		cancel()
		if lastErr == nil {
			return nil
		}

		time.Sleep(delay)
	}

	return fmt.Errorf("redis not ready after %d attempts: %w", attempts, lastErr)
}

func waitForMongo(client *mongo.Client, attempts int, delay time.Duration) error {
	var lastErr error

	for i := 0; i < attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		lastErr = client.Ping(ctx, nil)
		cancel()
		if lastErr == nil {
			return nil
		}

		time.Sleep(delay)
	}

	return fmt.Errorf("mongo not ready after %d attempts: %w", attempts, lastErr)
}
