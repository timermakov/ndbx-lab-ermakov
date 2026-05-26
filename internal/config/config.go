// Package config provides environment-backed application configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config contains all application runtime settings from environment.
type Config struct {
	AppHost              string
	AppPort              string
	AppUserSessionTTL    int
	AppLikeTTL           int
	AppEventReviewsTTL   int
	AppRecommendationsTTL int
	RedisHost            string
	RedisPort            string
	RedisPassword        string
	RedisDB              int
	MongoDatabase        string
	MongoUser            string
	MongoPassword        string
	MongoHost            string
	MongoPort            string
	CassandraHosts       string
	CassandraPort        string
	CassandraUsername    string
	CassandraPassword    string
	CassandraKeyspace    string
	CassandraConsistency string
	Neo4jURL             string
	Neo4jUser            string
	Neo4jPassword        string
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	appTTL, err := requiredInt("APP_USER_SESSION_TTL")
	if err != nil {
		return Config{}, err
	}
	appLikeTTL, err := requiredInt("APP_LIKE_TTL")
	if err != nil {
		return Config{}, err
	}
	appEventReviewsTTL, err := requiredInt("APP_EVENT_REVIEWS_TTL")
	if err != nil {
		return Config{}, err
	}
	appRecommendationsTTL, err := requiredInt("APP_RECOMMENDATIONS_TTL")
	if err != nil {
		return Config{}, err
	}

	redisDB, err := requiredInt("REDIS_DB")
	if err != nil {
		return Config{}, err
	}

	appHost, err := requiredString("APP_HOST")
	if err != nil {
		return Config{}, err
	}

	appPort, err := requiredString("APP_PORT")
	if err != nil {
		return Config{}, err
	}

	redisHost, err := requiredString("REDIS_HOST")
	if err != nil {
		return Config{}, err
	}

	redisPort, err := requiredString("REDIS_PORT")
	if err != nil {
		return Config{}, err
	}

	mongoDatabase, err := requiredString("MONGODB_DATABASE")
	if err != nil {
		return Config{}, err
	}

	mongoUser, err := requiredString("MONGODB_USER")
	if err != nil {
		return Config{}, err
	}

	mongoPassword, err := requiredString("MONGODB_PASSWORD")
	if err != nil {
		return Config{}, err
	}

	mongoHost, err := requiredString("MONGODB_HOST")
	if err != nil {
		return Config{}, err
	}

	mongoPort, err := requiredString("MONGODB_PORT")
	if err != nil {
		return Config{}, err
	}
	cassandraHosts, err := requiredString("CASSANDRA_HOSTS")
	if err != nil {
		return Config{}, err
	}
	cassandraPort, err := requiredString("CASSANDRA_PORT")
	if err != nil {
		return Config{}, err
	}
	cassandraKeyspace, err := requiredString("CASSANDRA_KEYSPACE")
	if err != nil {
		return Config{}, err
	}
	cassandraConsistency, err := requiredString("CASSANDRA_CONSISTENCY")
	if err != nil {
		return Config{}, err
	}
	neo4jURL, err := requiredString("NEO4J_URL")
	if err != nil {
		return Config{}, err
	}
	neo4jUser, err := requiredString("NEO4J_USER")
	if err != nil {
		return Config{}, err
	}
	neo4jPassword, err := requiredString("NEO4J_PASSWORD")
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppHost:              appHost,
		AppPort:              appPort,
		AppUserSessionTTL:    appTTL,
		AppLikeTTL:           appLikeTTL,
		AppEventReviewsTTL:   appEventReviewsTTL,
		AppRecommendationsTTL: appRecommendationsTTL,
		RedisHost:            redisHost,
		RedisPort:            redisPort,
		RedisPassword:        optionalString("REDIS_PASSWORD"),
		RedisDB:              redisDB,
		MongoDatabase:        mongoDatabase,
		MongoUser:            mongoUser,
		MongoPassword:        mongoPassword,
		MongoHost:            mongoHost,
		MongoPort:            mongoPort,
		CassandraHosts:       cassandraHosts,
		CassandraPort:        cassandraPort,
		CassandraUsername:    optionalString("CASSANDRA_USERNAME"),
		CassandraPassword:    optionalString("CASSANDRA_PASSWORD"),
		CassandraKeyspace:    cassandraKeyspace,
		CassandraConsistency: cassandraConsistency,
		Neo4jURL:             neo4jURL,
		Neo4jUser:            neo4jUser,
		Neo4jPassword:        neo4jPassword,
	}, nil
}

func requiredString(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is not set", key)
	}

	return value, nil
}

func optionalString(key string) string {
	return os.Getenv(key)
}

func requiredInt(key string) (int, error) {
	value, err := requiredString(key)
	if err != nil {
		return 0, err
	}

	parsed, parseErr := strconv.Atoi(value)
	if parseErr != nil {
		return 0, fmt.Errorf("parse %s: %w", key, parseErr)
	}

	return parsed, nil
}
