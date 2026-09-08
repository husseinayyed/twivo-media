package redis

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
	REDIS_URL = os.Getenv("REDIS_URL")
)

func ConnectRedis() (*redis.Client, error) {
	redisHost := REDIS_URL

	if redisHost == "" {
		log.Fatalln("REDIS_URL environment variable must be set")
	}

	opt, err := redis.ParseURL(redisHost)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL configuration: %v\n", err)
	}

	opt.PoolSize = 20
	opt.MinIdleConns = 5
	opt.MaxIdleConns = 10
	opt.ConnMaxIdleTime = 5 * time.Minute

	// 3. 🌟 Pass the complete, modified configuration to the initialization driver
	RedisClient = redis.NewClient(opt)

	// 4. Use a dedicated short-lived context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		fmt.Println("Error connecting to Redis:", err)
		os.Exit(1)
	}

	fmt.Println("🚀 Connected to Redis successfully!")
	return RedisClient, nil
}
