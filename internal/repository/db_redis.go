// Package repository contains the repository layer for the Moneybots API
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/nsvirk/moneybotsapi/internal/config"
	"github.com/redis/go-redis/v9"
)

func ConnectRedis(cfg *config.Config) (*redis.Client, error) {
	// Setup Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
	})
	// Check Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		// log.Fatalf("Failed to connect to Redis: %v", err)
		return nil, err
	}
	return redisClient, nil
}
