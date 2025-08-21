package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	DatabaseURL      string
	BooksServiceURL  string
	Port             int
	DBTimeout        time.Duration
	HTTPTimeout      time.Duration
	CircuitThreshold int
}

var (
	config *Config
	once   sync.Once
)

func Load() (*Config, error) {
	var err error
	once.Do(func() {
		config, err = load()
	})
	return config, err
}

func load() (*Config, error) {
	cfg := &Config{
		DBTimeout:        3 * time.Second,
		HTTPTimeout:      3 * time.Second,
		CircuitThreshold: 5,
	}

	var err error

	// Required environment variables
	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	cfg.BooksServiceURL = os.Getenv("BOOKS_SERVICE_URL")
	if cfg.BooksServiceURL == "" {
		return nil, fmt.Errorf("BOOKS_SERVICE_URL environment variable is required")
	}

	// Optional environment variables with defaults
	portStr := os.Getenv("PORT")
	if portStr == "" {
		cfg.Port = 8082
	} else {
		cfg.Port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT value: %v", err)
		}
	}

	return cfg, nil
}