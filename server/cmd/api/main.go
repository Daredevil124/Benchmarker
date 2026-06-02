package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"iicpc-backend/internal/broker"
	"iicpc-backend/internal/engine"
	"iicpc-backend/internal/handlers"
	"iicpc-backend/internal/models"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	botStatsMu sync.Mutex
	botStats   = make(map[string]map[string]interface{})
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

			if workerID, ok := stats["worker_id"].(string); ok && stats["type"] == "live_metrics" {
				botStatsMu.Lock()
				botStats[workerID] = stats

				var totalSuccess, totalFailed, totalCount float64
				var totalTps, totalProgress, totalP50, totalP90, totalP99 float64
				numWorkers := float64(len(botStats))

				for _, s := range botStats {
					if v, ok := s["success"].(float64); ok { totalSuccess += v }
					if v, ok := s["failed"].(float64); ok { totalFailed += v }
					if v, ok := s["tps"].(float64); ok { totalTps += v }
					if v, ok := s["current_count"].(float64); ok { totalCount += v }
					if v, ok := s["progress"].(float64); ok { totalProgress += v }
					if v, ok := s["p50_latency"].(float64); ok { totalP50 += v }
					if v, ok := s["p90_latency"].(float64); ok { totalP90 += v }
					if v, ok := s["p99_latency"].(float64); ok { totalP99 += v }
				}

				aggregated := map[string]interface{}{
					"type":          "live_metrics",
					"progress":      totalProgress / numWorkers,
					"success":       totalSuccess,
					"failed":        totalFailed,
					"p50_latency":   totalP50 / numWorkers,
					"p90_latency":   totalP90 / numWorkers,
					"p99_latency":   totalP99 / numWorkers,
					"tps":           totalTps,
					"current_count": totalCount,
					"total_target":  totalCount,
				}
				botStatsMu.Unlock()
				handlers.BroadcastTelemetry(aggregated)
			} else {
				if stats["type"] == "live_metrics_clear" {
					botStatsMu.Lock()
					botStats = make(map[string]map[string]interface{})
					botStatsMu.Unlock()
				}
				handlers.BroadcastTelemetry(stats)
			}
		}
	}()

	go func() {
		pubsub := broker.Client.Subscribe(broker.Ctx, "grading_results")
		defer pubsub.Close()
		for {
			msg, err := pubsub.ReceiveMessage(broker.Ctx)
			if err != nil {
				continue
			}
			var result models.GradingResult
			if err := json.Unmarshal([]byte(msg.Payload), &result); err != nil {
				continue
			}
			
			handlers.PendingGradingMu.Lock()
			if ch, ok := handlers.PendingGrading[result.SubmissionID]; ok {
				ch <- result
			}
			handlers.PendingGradingMu.Unlock()
		}
	}()
	http.HandleFunc("/api/health", handlers.HeartbeatHandler)
	http.HandleFunc("/api/attack", handlers.AttackHandler)
	http.HandleFunc("/api/upload", handlers.UploadHandler)
	http.HandleFunc("/api/questions", handlers.QuestionsHandler)
	http.HandleFunc("/api/stream", handlers.StreamHandler)
	http.HandleFunc("/api/reset", handlers.ResetHandler)
	http.HandleFunc("/api/start", handlers.StartContestHandler)
	http.HandleFunc("/api/contest/status", handlers.ContestStatusHandler)
	fmt.Println("Master Node Online: API Gateway listening on Port 9000")
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		fmt.Println("Master Node crashed:", err)
	}
}
var workerCancel context.CancelFunc
var workerMu sync.Mutex

func startWorkerNode() {
	fmt.Println("Worker Node online! Building/Verifying Sandbox Image...")
	
	// Build the monolithic sandbox image
	buildCmd := exec.Command("docker", "build", "-t", "benchmarker-sandbox", "-f", "Dockerfile.sandbox", ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("❌ Failed to build benchmarker-sandbox image: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Sandbox Image Ready!")

	pubsub := broker.Client.Subscribe(broker.Ctx, "attack_commands")
	defer pubsub.Close()
	
	// Add grading queue worker subscription using BLPop
	go func() {
		semaphore := make(chan struct{}, 10) // Limit to 10 concurrent Docker sandboxes
		for {
			res, err := broker.Client.BLPop(broker.Ctx, 0, "grading_queue").Result()
			if err != nil || len(res) < 2 {
				continue
			}

			payload := res[1]
			var task models.GradingTask
			if err := json.Unmarshal([]byte(payload), &task); err != nil {
				continue
			}

			semaphore <- struct{}{}
			go func(t models.GradingTask) {
				defer func() { <-semaphore }()

				// Reconstruct workspace
				workspacePath := filepath.Join(".", "temp_sandboxes", fmt.Sprintf("worker_%s_%d", t.SubmissionID, time.Now().UnixNano()))
				os.MkdirAll(workspacePath, os.ModePerm)
				filePath := filepath.Join(workspacePath, t.FileName)
				os.WriteFile(filePath, t.FileContent, 0644)

				res := engine.GradeSubmission(workspacePath, t.FileName, t.QuestionID)
				
				// Clean up worker workspace
				handlers.SafeCleanupDir(workspacePath)

				gradingRes := models.GradingResult{
					SubmissionID: t.SubmissionID,
					Status:       res.Status,
					LatencyMs:    res.LatencyMs,
					Output:       res.Output,
					ErrorDetail:  res.ErrorDetail,
				}
				resBytes, _ := json.Marshal(gradingRes)
				broker.Client.Publish(broker.Ctx, "grading_results", string(resBytes))
			}(task)
		}
	}()
	for {
		msg, err := pubsub.ReceiveMessage(broker.Ctx)
		if err != nil {
			fmt.Printf("Worker subscription Error: %v\n", err)
			continue
		}
		fmt.Printf("[WORKER] Message Recieved! Processing Co-ordinates...\n")
		var command models.AttackCommand
		err = json.Unmarshal([]byte(msg.Payload), &command)
		if err != nil {
			fmt.Println("❌ Failed to parse master command payload:", err)
			continue
		}

		workerMu.Lock()
		if workerCancel != nil {
			workerCancel()
			workerCancel = nil
		}

		if command.TestDurationSeconds <= 0 {
			workerMu.Unlock()
			fmt.Println("🛑 [WORKER] Received cancel command. Stopping bots.")
			continue
		}

		duration := time.Duration(command.TestDurationSeconds) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		workerCancel = cancel
		workerMu.Unlock()

		go executeAttack(ctx, command, duration)
	}
}

func executeAttack(ctx context.Context, command models.AttackCommand, duration time.Duration) {
	workerID := fmt.Sprintf("worker_%d", time.Now().UnixNano())
	fmt.Printf("🔥 [%s] Unleashing Local Concurrency: Spawning %d Bots against target: %s for %d seconds\n", workerID, command.BotConcurrency, command.TargetEndpoint, command.TestDurationSeconds)

		resultsChan := make(chan models.AttackResult, command.BotConcurrency*15)

		var wg sync.WaitGroup
		// Spawn bots
		for i := 1; i <= command.BotConcurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				engine.FireBot(ctx, id, command.TargetEndpoint, resultsChan)
			}(i)
		}

		var (
			statsMu      sync.Mutex
			totalSuccess int
			totalFailed  int
			latencies    []int64
		)

		// Goroutine to drain resultsChan
		go func() {
			for res := range resultsChan {
				statsMu.Lock()
				if res.Error != "" {
					totalFailed++
				} else {
					totalSuccess++
					latencies = append(latencies, res.Latency)
				}
				statsMu.Unlock()
			}
		}()

		startTime := time.Now()
		ticker := time.NewTicker(200 * time.Millisecond)

		tickerLoop:
		for {
			select {
			case <-ctx.Done():
				break tickerLoop
			case <-ticker.C:
				elapsed := time.Since(startTime)
				progress := (elapsed.Seconds() / duration.Seconds()) * 100
				if progress > 99.0 {
					progress = 99.0
				}

				statsMu.Lock()
				successVal := totalSuccess
				failedVal := totalFailed
				latsCopy := make([]int64, len(latencies))
				copy(latsCopy, latencies)
				statsMu.Unlock()

				var p50, p90, p99 int64
				if len(latsCopy) > 0 {
					sort.Slice(latsCopy, func(i, j int) bool { return latsCopy[i] < latsCopy[j] })
					p50 = latsCopy[len(latsCopy)*50/100]
					p90 = latsCopy[len(latsCopy)*90/100]
					p99 = latsCopy[len(latsCopy)*99/100]
				}

				totalProcessed := successVal + failedVal
				tps := 0.0
				if elapsed.Seconds() > 0 {
					tps = float64(totalProcessed) / elapsed.Seconds()
				}

				stats := map[string]interface{}{
					"type":          "live_metrics",
					"worker_id":     workerID,
					"progress":      progress,
					"success":       successVal,
					"failed":        failedVal,
					"p50_latency":   p50,
					"p90_latency":   p90,
					"p99_latency":   p99,
					"tps":           tps,
					"current_count": totalProcessed,
					"total_target":  totalProcessed,
				}
				payload, _ := json.Marshal(stats)
				broker.Client.Publish(broker.Ctx, "telemetry_stream", payload)
			}
		}

		ticker.Stop()

		// Wait for all bots to complete their loop before closing resultsChan
		wg.Wait()
		close(resultsChan)

		// Final publish to signal 100% progress
		statsMu.Lock()
		successVal := totalSuccess
		failedVal := totalFailed
		latsCopy := make([]int64, len(latencies))
		copy(latsCopy, latencies)
		statsMu.Unlock()

		var p50, p90, p99 int64
		if len(latsCopy) > 0 {
			sort.Slice(latsCopy, func(i, j int) bool { return latsCopy[i] < latsCopy[j] })
			p50 = latsCopy[len(latsCopy)*50/100]
			p90 = latsCopy[len(latsCopy)*90/100]
			p99 = latsCopy[len(latsCopy)*99/100]
		}

		totalProcessed := successVal + failedVal
		tps := 0.0
		elapsedSeconds := time.Since(startTime).Seconds()
		if elapsedSeconds > 0 {
			tps = float64(totalProcessed) / elapsedSeconds
		}

		stats := map[string]interface{}{
			"type":          "live_metrics",
			"worker_id":     workerID,
			"progress":      100.0,
			"success":       successVal,
			"failed":        failedVal,
			"p50_latency":   p50,
			"p90_latency":   p90,
			"p99_latency":   p99,
			"tps":           tps,
			"current_count": totalProcessed,
			"total_target":  totalProcessed,
		}
		payload, _ := json.Marshal(stats)
		broker.Client.Publish(broker.Ctx, "telemetry_stream", payload)
		fmt.Printf("[%s] Bombardment Complete. Final TPS: %.2f\n", workerID, tps)
}
