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
	"strings"
	"syscall"
	"time"

	"github.com/gocql/gocql"
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

	cassandraSession, err := createCassandraSession(cfg, cfg.CassandraKeyspace)
	if err != nil {
		logger.Fatalf("cassandra keyspace session failed: %v", err)
	}
	defer cassandraSession.Close()

	reactionRepo := repository.NewCassandraEventReactionRepository(cassandraSession)

	mongoDB := mongoClient.Database(cfg.MongoDatabase)
	userRepo := repository.NewMongoUserRepository(mongoDB)
	eventRepo := repository.NewMongoEventRepository(mongoDB)
	reactionCache := repository.NewRedisEventReactionCache(redisClient, time.Duration(cfg.AppLikeTTL)*time.Second)

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
	eventService := service.NewEventService(eventRepo, userRepo)
	eventService.SetReactionsStorage(reactionRepo, reactionCache)

	usersHandler := handler.NewUsersHandler(userService, eventService, store, cfg.AppUserSessionTTL)
	authHandler := handler.NewAuthHandler(userService, store, cfg.AppUserSessionTTL)
	eventsHandler := handler.NewEventsHandler(eventService, store, cfg.AppUserSessionTTL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /session", handler.NewSessionHandler(store, cfg.AppUserSessionTTL))
	mux.HandleFunc("POST /users", usersHandler.Register)
	mux.HandleFunc("GET /users", usersHandler.List)
	mux.HandleFunc("GET /users/{id}", usersHandler.GetByID)
	mux.HandleFunc("GET /users/{id}/events", usersHandler.ListEvents)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("POST /events", eventsHandler.Create)
	mux.HandleFunc("GET /events", eventsHandler.List)
	mux.HandleFunc("GET /events/{id}", eventsHandler.GetByID)
	mux.HandleFunc("POST /events/{id}/like", eventsHandler.Like)
	mux.HandleFunc("POST /events/{id}/dislike", eventsHandler.Dislike)
	mux.HandleFunc("PATCH /events/{id}", eventsHandler.Patch)
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ErrorLog:     logger,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
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
		"mongodb://%s:%s@%s:%s/%s",
		cfg.MongoUser,
		cfg.MongoPassword,
		cfg.MongoHost,
		cfg.MongoPort,
		cfg.MongoDatabase,
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

func createCassandraSession(cfg config.Config, keyspace string) (*gocql.Session, error) {
	hosts := splitAndTrim(cfg.CassandraHosts)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("cassandra hosts are empty")
	}

	port, err := strconv.Atoi(cfg.CassandraPort)
	if err != nil {
		return nil, fmt.Errorf("parse cassandra port: %w", err)
	}

	consistency, err := parseCassandraConsistency(cfg.CassandraConsistency)
	if err != nil {
		return nil, err
	}

	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Consistency = consistency
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Timeout = 10 * time.Second
	cluster.Keyspace = keyspace
	if cfg.CassandraUsername != "" || cfg.CassandraPassword != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: cfg.CassandraUsername,
			Password: cfg.CassandraPassword,
		}
	}

	var lastErr error
	for i := 0; i < 30; i++ {
		session, createErr := cluster.CreateSession()
		if createErr == nil {
			return session, nil
		}
		lastErr = createErr
		time.Sleep(time.Second)
	}

	return nil, fmt.Errorf("create cassandra session: %w", lastErr)
}

func parseCassandraConsistency(value string) (gocql.Consistency, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ANY":
		return gocql.Any, nil
	case "ONE":
		return gocql.One, nil
	case "TWO":
		return gocql.Two, nil
	case "THREE":
		return gocql.Three, nil
	case "QUORUM":
		return gocql.Quorum, nil
	case "ALL":
		return gocql.All, nil
	case "LOCAL_QUORUM":
		return gocql.LocalQuorum, nil
	case "EACH_QUORUM":
		return gocql.EachQuorum, nil
	case "LOCAL_ONE":
		return gocql.LocalOne, nil
	default:
		return gocql.Any, fmt.Errorf("unsupported cassandra consistency: %s", value)
	}
}

func splitAndTrim(values string) []string {
	parts := strings.Split(values, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}

	return result
}
