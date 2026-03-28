package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config contains all application runtime settings from environment.
type Config struct {
	AppHost           string
	AppPort           string
	AppUserSessionTTL int
	RedisHost         string
	RedisPort         string
	RedisPassword     string
	RedisDB           int
	MongoDatabase     string
	MongoUser         string
	MongoPassword     string
	MongoHost         string
	MongoPort         string
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	appTTL, err := requiredInt("APP_USER_SESSION_TTL")
	if err != nil {
		return Config{}, err
	}

	redisDB, err := requiredInt("REDIS_DB")
	if err != nil {
		return Config{}, err
	}

	// Keep compatibility with lab spec typo.
	mongoDatabase := firstNonEmpty(
		os.Getenv("MONGODB_DATABSE"),
		os.Getenv("MONGODB_DATABASE"),
	)
	if mongoDatabase == "" {
		return Config{}, fmt.Errorf("environment variable MONGODB_DATABSE is not set")
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

	return Config{
		AppHost:           appHost,
		AppPort:           appPort,
		AppUserSessionTTL: appTTL,
		RedisHost:         redisHost,
		RedisPort:         redisPort,
		RedisPassword:     os.Getenv("REDIS_PASSWORD"),
		RedisDB:           redisDB,
		MongoDatabase:     mongoDatabase,
		MongoUser:         mongoUser,
		MongoPassword:     mongoPassword,
		MongoHost:         mongoHost,
		MongoPort:         mongoPort,
	}, nil
}

func requiredString(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is not set", key)
	}

	return value, nil
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
