package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort            string
	DBHost                string
	DBPort                string
	DBUser                string
	DBPassword            string
	DBName                string
	DBSSLMode             string
	ClassifierURL         string
	ClassifierThreshold   float64
	ClassifierTimeout     time.Duration
	VoteQueueCapacity     int
	ClassifierWorkers     int
	DBFlushInterval       time.Duration
	WSPingInterval        time.Duration
	WSPongTimeout         time.Duration
	CORSAllowedOrigins    []string
	LogLevel              string
	AdminKey              string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		ServerPort:         getEnv("SERVER_PORT", "8585"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "topicvoting"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		ClassifierURL:      getEnv("CLASSIFIER_URL", "http://localhost:4747"),
		CORSAllowedOrigins: parseOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5442")),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		AdminKey:           getEnv("ADMIN_KEY", ""),
	}

	var err error

	cfg.ClassifierThreshold, err = getEnvFloat("CLASSIFIER_THRESHOLD", 0.5)
	if err != nil {
		return nil, fmt.Errorf("CLASSIFIER_THRESHOLD: %w", err)
	}

	classifierTimeoutSec, err := getEnvInt("CLASSIFIER_TIMEOUT_S", 2)
	if err != nil {
		return nil, fmt.Errorf("CLASSIFIER_TIMEOUT_S: %w", err)
	}
	cfg.ClassifierTimeout = time.Duration(classifierTimeoutSec) * time.Second

	cfg.VoteQueueCapacity, err = getEnvInt("VOTE_QUEUE_CAPACITY", 1000)
	if err != nil {
		return nil, fmt.Errorf("VOTE_QUEUE_CAPACITY: %w", err)
	}

	cfg.ClassifierWorkers, err = getEnvInt("CLASSIFIER_WORKERS", 4)
	if err != nil {
		return nil, fmt.Errorf("CLASSIFIER_WORKERS: %w", err)
	}

	flushMs, err := getEnvInt("DB_FLUSH_INTERVAL_MS", 500)
	if err != nil {
		return nil, fmt.Errorf("DB_FLUSH_INTERVAL_MS: %w", err)
	}
	cfg.DBFlushInterval = time.Duration(flushMs) * time.Millisecond

	pingSec, err := getEnvInt("WS_PING_INTERVAL_S", 30)
	if err != nil {
		return nil, fmt.Errorf("WS_PING_INTERVAL_S: %w", err)
	}
	cfg.WSPingInterval = time.Duration(pingSec) * time.Second

	pongSec, err := getEnvInt("WS_PONG_TIMEOUT_S", 10)
	if err != nil {
		return nil, fmt.Errorf("WS_PONG_TIMEOUT_S: %w", err)
	}
	cfg.WSPongTimeout = time.Duration(pongSec) * time.Second

	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", v, err)
	}
	return n, nil
}

func getEnvFloat(key string, fallback float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float %q: %w", v, err)
	}
	return f, nil
}

func parseOrigins(s string) []string {
	origins := strings.Split(s, ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}
	return origins
}