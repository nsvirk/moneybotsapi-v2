// Package service contains the service layer for the Moneybots API
package service

import (
	"context"
	"time"

	"github.com/lib/pq"
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
}

func NewPublishService(db *gorm.DB, redisClient *redis.Client, pgConnStr string) *PublishService {

	return &PublishService{
		db:          db,
		redisClient: redisClient,
		pgConnStr:   pgConnStr,
	}
}

func (s *PublishService) PublishTicksToRedisChannel() {

	// Create a PostgreSQL listener
	listener := pq.NewListener(s.pgConnStr, 10*time.Second, time.Minute, nil)
	err := listener.Listen(PostgresChannel)
	if err != nil {
		return
	}

	ctx := context.Background()

	for {
		select {
		case n := <-listener.Notify:
			// Publish the notification to Redis
			err := s.redisClient.Publish(ctx, RedisChannel, n.Extra).Err()
			if err != nil {
				zaplogger.Error("Failed to publish to Redis", zaplogger.Fields{"error": err})
			}
		case <-time.After(90 * time.Second):
			go func() {
				err := listener.Ping()
				if err != nil {

					zaplogger.Error("Error pinging PostgreSQL", zaplogger.Fields{"error": err})
				}
			}()
		}
	}
}
