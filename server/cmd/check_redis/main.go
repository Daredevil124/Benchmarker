package main

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	fmt.Println("🔍 [DIAGNOSTIC] Connecting to local Redis...")
	_, err := client.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("❌ Failed to connect to Redis: %v\n", err)
		return
	}

	fmt.Println("✅ Connected successfully!")

	// 1. Query the contest start time
	startTime, err := client.Get(ctx, "contest:start_time").Result()
	if err == nil {
		fmt.Printf("⏱️ Contest Start Time: %s\n", startTime)
	} else {
		fmt.Println("⏱️ Contest Start Time key not found.")
	}

	// 2. Query total submissions
	totalSub, err := client.Get(ctx, "stats:total_submissions").Result()
	if err == nil {
		fmt.Printf("📥 Total submissions counter: %s\n", totalSub)
	}

	// 3. Query all keys
	keys, _ := client.Keys(ctx, "*").Result()
	fmt.Printf("🔑 All active keys in DB: %v\n", keys)

	// 4. Query the leaderboard standings
	standings, err := client.ZRevRangeWithScores(ctx, "leaderboard", 0, -1).Result()
	if err != nil {
		fmt.Printf("❌ Failed to fetch leaderboard: %v\n", err)
		return
	}

	fmt.Printf("\n🏆 Leaderboard Standings (%d members):\n", len(standings))
	for idx, entry := range standings {
		fmt.Printf("  Rank %d: Team='%s', Score=%.1f\n", idx+1, entry.Member, entry.Score)
	}
}
