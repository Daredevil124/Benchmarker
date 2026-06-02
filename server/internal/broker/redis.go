package broker

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var Ctx = context.Background()
var Client *redis.Client

func InitRedis() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	Client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	var err error
	maxRetries := 15
	for i := 1; i <= maxRetries; i++ {
		_, err = Client.Ping(Ctx).Result()
		if err == nil {
			fmt.Printf("Connected to Redis Message Broker after %d attempt(s)\n", i)
			return
		}
		fmt.Printf("Redis not ready (attempt %d/%d): %v. Retrying in 2 seconds...\n", i, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	panic(fmt.Sprintf("Cannot connect to Redis Broker after %d attempts: %v", maxRetries, err))
}
