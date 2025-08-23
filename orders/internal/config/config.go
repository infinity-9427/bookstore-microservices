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

	// Feature flags
	IdempotencyEnabled bool // default: false

	// L1 cache for Books
	BooksCacheTTL        time.Duration // default: 5s
	BooksCacheMaxEntries int           // default: 10000

	// Background DB pool (for cleanup jobs)
	BackgroundDatabaseURL string // default: DatabaseURL
	BackgroundMaxConns    int    // default: 2
}

var (
	cfg  *Config
	once sync.Once
)

func Load() (*Config, error) {
	var err error
	once.Do(func() { cfg, err = load() })
	return cfg, err
}

func load() (*Config, error) {
	c := &Config{
		DBTimeout:            3 * time.Second,
		HTTPTimeout:          3 * time.Second,
		CircuitThreshold:     5,
		IdempotencyEnabled:   false, // Default: disabled for backward compatibility
		BooksCacheTTL:        5 * time.Second,
		BooksCacheMaxEntries: 10000,
		BackgroundMaxConns:   2,
	}

	if v := os.Getenv("DATABASE_URL"); v != "" {
		c.DatabaseURL = v
	} else {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if v := os.Getenv("BOOKS_SERVICE_URL"); v != "" {
		c.BooksServiceURL = v
	} else {
		return nil, fmt.Errorf("BOOKS_SERVICE_URL is required")
	}

	if v := os.Getenv("PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Port = n
		}
	}
	if v := os.Getenv("DB_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.DBTimeout = d
		}
	}
	if v := os.Getenv("HTTP_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.HTTPTimeout = d
		}
	}
	if v := os.Getenv("CIRCUIT_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.CircuitThreshold = n
		}
	}

	if v := os.Getenv("ORDERS_ENABLE_IDEMPOTENCY"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.IdempotencyEnabled = b
		}
	}

	if v := os.Getenv("BOOKS_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.BooksCacheTTL = d
		}
	}
	if v := os.Getenv("BOOKS_CACHE_MAX"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.BooksCacheMaxEntries = n
		}
	}

	if v := os.Getenv("BACKGROUND_DATABASE_URL"); v != "" {
		c.BackgroundDatabaseURL = v
	} else {
		c.BackgroundDatabaseURL = c.DatabaseURL
	}
	if v := os.Getenv("BACKGROUND_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.BackgroundMaxConns = n
		}
	}

	return c, nil
}
