package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"iicpc-backend/internal/broker"
	"iicpc-backend/internal/engine"
	"iicpc-backend/internal/handlers"
	"iicpc-backend/internal/models"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

func main() {
	mode := flag.String("mode", "master", "Defines the node roles: 'master' or 'worker'")
	flag.Parse()
	broker.InitRedis()
	if *mode == "master" {
		startMasterNode()
	} else if *mode == "worker" {
		startWorkerNode()
	} else {
		log.Fatal("Invalide mode. Use -mode=master or -mode=worker")
	}
}
func startMasterNode() {
	go func() {
		pubsub := broker.Client.Subscribe(broker.Ctx, "telemetry_stream")
		defer pubsub.Close()
		for {
			msg, err := pubsub.ReceiveMessage(broker.Ctx)
			if err != nil {
				continue
			}
			var stats map[string]interface{}
			json.Unmarshal([]byte(msg.Payload), &stats)
			handlers.BroadcastTelemetry(stats)
		}
	}()
	http.HandleFunc("/api/health", handlers.HeartbeatHandler)
	http.HandleFunc("/api/attack", handlers.AttackHandler)
	http.HandleFunc("/api/upload", handlers.UploadHandler)
	http.HandleFunc("/api/stream", handlers.StreamHandler)
	fmt.Println("Master Node Online: API Gateway listening on Port 9000")
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		fmt.Println("Master Node crashed:", err)
	}
}
func startWorkerNode() {
	fmt.Println("Worker Node online!")
	pubsub := broker.Client.Subscribe(broker.Ctx, "attack_commands")
	defer pubsub.Close()
	for {
		msg, err := pubsub.ReceiveMessage(broker.Ctx)
		if err != nil {
			fmt.Printf("Worker subscription Error: %v", err)
			continue
		}
		fmt.Printf("[WORKER] Message Recieved! Processing Co-ordinates...\n")
		var command models.AttackCommand
		err = json.Unmarshal([]byte(msg.Payload), &command)
		if err != nil {
			fmt.Println("❌ Failed to parse master command payload:", err)
			continue
		}
		fmt.Printf("🔥 [WORKER] Unleashing Local Concurrency: Spawning %d Bots against target: %s\n", command.BotConcurrency, command.TargetEndpoint)
		resultsChan := make(chan models.AttackResult, command.BotConcurrency)
		var wg sync.WaitGroup
		for i := 1; i <= command.BotConcurrency; i++ {
			wg.Add(1)
			go engine.FireBot(i, command.TargetEndpoint, &wg, resultsChan)
		}
		go func() {
			wg.Wait()
			close(resultsChan)
		}()
		var totalSuccess, totalFailed int
		var latencies []int64
		processedCount := 0
		startTime := time.Now()
		for results := range resultsChan {
			processedCount++
			if results.Error != "" {
				totalFailed++
			} else {
				totalSuccess++
				latencies = append(latencies, results.Latency)
			}
			if processedCount%50 == 0 || processedCount == command.BotConcurrency {
				var p50, p90, p99 int64
				if len(latencies) > 0 {
					sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
					p50 = latencies[len(latencies)*50/100]
					p90 = latencies[len(latencies)*90/100]
					p99 = latencies[len(latencies)*99/100]
				}
				elapsedSeconds := time.Since(startTime).Seconds()
				tps := 0.0
				if elapsedSeconds > 0 {
					tps = float64(processedCount) / elapsedSeconds
				}
				stats := map[string]interface{}{
					"type":          "live_metrics",
					"progress":      (float64(processedCount) / float64(command.BotConcurrency)) * 100,
					"success":       totalSuccess,
					"failed":        totalFailed,
					"p50_latency":   p50,
					"p90_latency":   p90,
					"p99_latency":   p99,
					"tps":           tps,
					"current_count": processedCount,
					"total_target":  command.BotConcurrency,
				}
				payload, _ := json.Marshal(stats)
				broker.Client.Publish(broker.Ctx, "telemetry_stream", payload)

			}
		}
		fmt.Printf("[WORKER] Bombardment Complete. Final TPS: %.2f\n", float64(command.BotConcurrency)/time.Since(startTime).Seconds())
	}
}
