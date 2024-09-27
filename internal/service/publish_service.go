// Package service contains the service layer for the Moneybots API
package service

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/nsvirk/moneybotsapi/pkg/utils/logger"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var PostgresChannel = "CH:API:TICKER:DATA"
var RedisChannel = "CH:API:TICKER:DATA"

type PublishService struct {
	db          *gorm.DB
	redisClient *redis.Client
	pgConnStr   string
	logger      *logger.Logger
}

func NewPublishService(db *gorm.DB, redisClient *redis.Client, pgConnStr string) *PublishService {
	logger, err := logger.New(db, "PUBLISH SERVICE")
	if err != nil {
		zaplogger.Error("failed to create publish logger", zaplogger.Fields{"error": err})
	}
	return &PublishService{
		db:          db,
		redisClient: redisClient,
		pgConnStr:   pgConnStr,
		logger:      logger,
	}
}

func (s *PublishService) PublishTicksToRedisChannel() {

	// Create a PostgreSQL listener
	listener := pq.NewListener(s.pgConnStr, 10*time.Second, time.Minute, nil)
	err := listener.Listen(PostgresChannel)
	if err != nil {
		s.logger.Error("Failed to create listener", map[string]interface{}{
			"Postgres Channel": PostgresChannel,
			"error":            err,
		})
		return
	}

	s.logger.Info("Starting to Publish", map[string]interface{}{
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
				s.logger.Error("Failed to publish to Redis", map[string]interface{}{
					"Postgres Channel": PostgresChannel,
					"Redis Channel":    RedisChannel,
					"error":            err,
				})
			}
		case <-time.After(90 * time.Second):
			go func() {
				err := listener.Ping()
				if err != nil {
					s.logger.Error("Error pinging PostgreSQL", map[string]interface{}{
						"Postgres Channel": PostgresChannel,
						"Redis Channel":    RedisChannel,
						"error":            err,
					})
				}
			}()
		}
	}
}
