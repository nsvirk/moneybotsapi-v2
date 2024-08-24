package database

import (
	"context"
	"time"

	"github.com/nsvirk/moneybotsapi/config"
	"github.com/nsvirk/moneybotsapi/utils/applogger"
	"github.com/redis/go-redis/v9"
)

func ConnectRedis(cfg *config.Config) (*redis.Client, error) {

	applogger.Info(config.SingleLine)
	applogger.Info("Connecting to Redis")

	// Setup Redis
	redisOpts, err := redis.ParseURL(cfg.RedisUrl)
	if err != nil {
		// log.Fatalf("Failed to parse Redis URL: %v", err)
		return nil, err
	}
	redisClient := redis.NewClient(redisOpts)

	// Check Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		// log.Fatalf("Failed to connect to Redis: %v", err)
		return nil, err
	}

	applogger.Info("  * connected")

	return redisClient, nil

}
