package services

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/nsvirk/moneybotsapi/shared/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var PostgresChannel = "CH:API:TICKER:DATA"
var RedisChannel = "CH:API:TICKER:DATA"

func PublishTicksToRedisChannel(db *gorm.DB, redisClient *redis.Client, pgConnStr string) {
	//  Create a logger
	ticksLogger, err := logger.New(db, ServicesLogsTableName)
	if err != nil {
		panic(err)
	}

	// Create a PostgreSQL listener
	listener := pq.NewListener(pgConnStr, 10*time.Second, time.Minute, nil)
	err = listener.Listen(PostgresChannel)
	if err != nil {
		ticksLogger.Error("TICKS SERVICE: Failed to create listener", map[string]interface{}{
			"Postgres Channel": PostgresChannel,
			"error":            err,
		})
		return
	}

	ticksLogger.Info("TICKS SERVICE: Starting to Publish", map[string]interface{}{
		"Postgres Channel": PostgresChannel,
		"Redis Channel":    RedisChannel,
	})

	ctx := context.Background()

	for {
		select {
		case n := <-listener.Notify:
			// Publish the notification to Redis
			err := redisClient.Publish(ctx, RedisChannel, n.Extra).Err()
			if err != nil {
				ticksLogger.Error("TICKS SERVICE: Failed to publish to Redis", map[string]interface{}{
					"Postgres Channel": PostgresChannel,
					"Redis Channel":    RedisChannel,
					"error":            err,
				})
			}
		case <-time.After(90 * time.Second):
			go func() {
				err := listener.Ping()
				if err != nil {
					ticksLogger.Error("TICKS SERVICE: Error pinging PostgreSQL", map[string]interface{}{
						"Postgres Channel": PostgresChannel,
						"Redis Channel":    RedisChannel,
						"error":            err,
					})
				}
			}()
		}
	}
}
