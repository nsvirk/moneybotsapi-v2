// Package service contains the service layer for the Moneybots API
package service

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/nsvirk/moneybotsapi/pkg/utils/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var PostgresChannel = "CH:API:TICKER:DATA"
var RedisChannel = "CH:API:TICKER:DATA"

type PublishService struct {
	db          *gorm.DB
	redisClient *redis.Client
	pgConnStr   string
}

func NewPublishService(db *gorm.DB, redisClient *redis.Client, pgConnStr string) *PublishService {
	return &PublishService{db: db, redisClient: redisClient, pgConnStr: pgConnStr}
}

func (s *PublishService) PublishTicksToRedisChannel() {
	//  Create a logger
	ticksLogger, err := logger.New(s.db, "TICKS SERVICE")
	if err != nil {
		panic(err)
	}

	// Create a PostgreSQL listener
	listener := pq.NewListener(s.pgConnStr, 10*time.Second, time.Minute, nil)
	err = listener.Listen(PostgresChannel)
	if err != nil {
		ticksLogger.Error("Failed to create listener", map[string]interface{}{
			"Postgres Channel": PostgresChannel,
			"error":            err,
		})
		return
	}

	ticksLogger.Info("Starting to Publish", map[string]interface{}{
		"Postgres Channel": PostgresChannel,
		"Redis Channel":    RedisChannel,
	})

	ctx := context.Background()

	for {
		select {
		case n := <-listener.Notify:
			// Publish the notification to Redis
			err := s.redisClient.Publish(ctx, RedisChannel, n.Extra).Err()
			if err != nil {
				ticksLogger.Error("Failed to publish to Redis", map[string]interface{}{
					"Postgres Channel": PostgresChannel,
					"Redis Channel":    RedisChannel,
					"error":            err,
				})
			}
		case <-time.After(90 * time.Second):
			go func() {
				err := listener.Ping()
				if err != nil {
					ticksLogger.Error("Error pinging PostgreSQL", map[string]interface{}{
						"Postgres Channel": PostgresChannel,
						"Redis Channel":    RedisChannel,
						"error":            err,
					})
				}
			}()
		}
	}
}
