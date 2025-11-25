package ws

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// TestConfig holds configuration for test environment
type TestConfig struct {
	MaxCandles    int
	RedisAddr     string
	DBConnString  string
	TestTimeout   time.Duration
}

// DefaultTestConfig returns default test configuration
func DefaultTestConfig() TestConfig {
	return TestConfig{
		MaxCandles:   500,
		RedisAddr:    "localhost:6379",
		DBConnString: "postgres://test:test@localhost:5432/test?sslmode=disable",
		TestTimeout:  5 * time.Second,
	}
}

// TestRedisClient creates a Redis client for testing
// Returns nil if Redis is not available (tests should handle gracefully)
func TestRedisClient(addr string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil
	}
	
	return client
}

// TestDB creates a database connection for testing
// Returns nil if database is not available (tests should handle gracefully)
func TestDB(connString string) *sqlx.DB {
	db, err := sqlx.Connect("postgres", connString)
	if err != nil {
		return nil
	}
	
	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil
	}
	
	return db
}

// CleanupTestRedis cleans up test data from Redis
func CleanupTestRedis(client *redis.Client, pattern string) error {
	if client == nil {
		return nil
	}
	
	ctx := context.Background()
	iter := client.Scan(ctx, 0, pattern, 0).Iterator()
	
	for iter.Next(ctx) {
		if err := client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	
	return iter.Err()
}

// CleanupTestDB cleans up test data from database
func CleanupTestDB(db *sqlx.DB, table string) error {
	if db == nil {
		return nil
	}
	
	_, err := db.Exec("DELETE FROM " + table + " WHERE symbol LIKE 'TEST%'")
	return err
}
