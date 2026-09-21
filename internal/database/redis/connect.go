package redis

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var (
	RedisClient *redis.Client
	REDIS_URL   = os.Getenv("REDIS_URL")
)

func ConnectRedis() (*redis.Client, error) {
	redisHost := REDIS_URL

	if redisHost == "" {
		log.Fatal().Msg("REDIS_URL environment variable must be set")
	}

	opt, err := redis.ParseURL(redisHost)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse Redis URL configuration")
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
		log.Fatal().Err(err).Msg("error connecting to Redis")
		os.Exit(1)
	}

	log.Info().Msg("connected to Redis successfully")
	return RedisClient, nil
}
