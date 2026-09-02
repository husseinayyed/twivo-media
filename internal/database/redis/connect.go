package redis

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
)

func ConnectRedis() (redisClient *redis.Client, err error) {
	redisHost := os.Getenv("REDIS_URL")

	if redisHost == "" {
		fmt.Println("REDIS_URL environment variable must be set")
		os.Exit(1)
	}

	RedisClient = redis.NewClient(&redis.Options{
    Addr:            redisHost,        // Redis server address
    PoolSize:        20,               // Maximum open connections
    MinIdleConns:    5,                // Minimum idle connections to maintain
    MaxIdleConns:    10,               // Maximum idle connections to keep
    ConnMaxIdleTime: 5 * time.Minute,  // Close idle connections after 5 minutes
})

	ctx := context.Background()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		fmt.Println("Error connecting to Redis:", err)
		os.Exit(1)
	}

	fmt.Println("Connected to Redis successfully")
	return RedisClient, nil
}