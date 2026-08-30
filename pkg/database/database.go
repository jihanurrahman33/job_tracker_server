package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DB wraps standard database/sql DB connection handle.
type DB struct {
	*sql.DB
}

// PoolConfig holds database connection pooling options.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Connect initializes a connection to the SQL database with custom pool config and ping verification.
func Connect(driverName, dataSourceName string, pool PoolConfig) (*DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Apply pooling settings
	if pool.MaxOpenConns > 0 {
		db.SetMaxOpenConns(pool.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(15)
	}

	if pool.MaxIdleConns > 0 {
		db.SetMaxIdleConns(pool.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}

	if pool.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(15 * time.Minute)
	}

	if pool.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
	} else {
		db.SetConnMaxIdleTime(5 * time.Minute)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db}, nil
}

// ConnectWithRetry attempts to connect to the database with retries and exponential backoff.
// This is especially beneficial for cloud hosted databases (like Render/Neon/Supabase) during wake-up.
func ConnectWithRetry(driverName, dataSourceName string, pool PoolConfig, attempts int, initialDelay time.Duration) (*DB, error) {
	var lastErr error
	delay := initialDelay

	for i := 1; i <= attempts; i++ {
		db, err := Connect(driverName, dataSourceName, pool)
		if err == nil {
			return db, nil
		}

		lastErr = err
		if i < attempts {
			time.Sleep(delay)
			delay *= 2
		}
	}

	return nil, fmt.Errorf("database connection failed after %d attempts: %w", attempts, lastErr)
}
