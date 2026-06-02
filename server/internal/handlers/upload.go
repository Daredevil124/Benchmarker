package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"iicpc-backend/internal/broker"
	"iicpc-backend/internal/models"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	contestCancelFunc context.CancelFunc
	contestEndTime    time.Time
	contestMu         sync.Mutex
	contestIsActive   bool

	PendingGrading   = make(map[string]chan models.GradingResult)
	PendingGradingMu sync.Mutex
)

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("code_file")
	if err != nil {
		http.Error(w, "failed to Retrieve the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	teamName := r.FormValue("team")
	questionID := r.FormValue("question")

	if teamName == "" {
		teamName = "Anonymous"
	}
	if questionID == "" {
		questionID = "q1"
	}

	isAlreadySolved, _ := broker.Client.HExists(broker.Ctx, fmt.Sprintf("scores:%s", teamName), questionID).Result()
	if isAlreadySolved {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ignored",
			"message": "Question already accepted for this team. Submission ignored.",
		})
		return
	}

	submissionID := fmt.Sprintf("sub_%d", time.Now().UnixNano())
	workspacePath := filepath.Join(".", "temp_sandboxes", submissionID)
	if err := os.MkdirAll(workspacePath, os.ModePerm); err != nil {
		http.Error(w, "Failed to create workspace", http.StatusInternalServerError)
		return
	}

	destinationPath := filepath.Join(workspacePath, handler.Filename)
	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}
	defer destinationFile.Close()

	if _, err := io.Copy(destinationFile, file); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	// Dynamic Contest Start Time initialization
	contestStartStr, err := broker.Client.Get(broker.Ctx, "contest:start_time").Result()
	var startTimeUnix int64
	if err != nil {
		startTimeUnix = time.Now().Unix()
		broker.Client.Set(broker.Ctx, "contest:start_time", strconv.FormatInt(startTimeUnix, 10), 0)
		fmt.Printf("🏁 [CONTEST INITIALIZED] Contest Start Time set to %d!\n", startTimeUnix)
	} else {
		startTimeUnix, _ = strconv.ParseInt(contestStartStr, 10, 64)
	}

	broker.Client.Incr(broker.Ctx, "stats:total_submissions")
	broker.Client.Incr(broker.Ctx, fmt.Sprintf("stats:total_attempts:%s", teamName))

	submitTime := time.Now().UnixMilli()
	subID := fmt.Sprintf("SUB_%d_%d", submitTime, rand.Intn(1000))

	BroadcastTelemetry(map[string]interface{}{
		"type": "submission_queued",
		"data": map[string]interface{}{
			"id":          subID,
			"team":        teamName,
			"question":    questionID,
			"status":      "Running",
			"submit_time": submitTime,
		},
	})
	
	// Add them to leaderboard with 0 score if this is their first submission
	updateTeamTotalScore(teamName)

	go func() {
		resChan := make(chan models.GradingResult)
		PendingGradingMu.Lock()
		PendingGrading[subID] = resChan
		PendingGradingMu.Unlock()

		fileContent, _ := os.ReadFile(filepath.Join(workspacePath, handler.Filename))
		task := models.GradingTask{
			SubmissionID: subID,
			Team:         teamName,
			QuestionID:   questionID,
			FileName:     handler.Filename,
			FileContent:  fileContent,
		}
		taskBytes, _ := json.Marshal(task)
		broker.Client.LPush(broker.Ctx, "grading_queue", string(taskBytes))

		res := <-resChan

		PendingGradingMu.Lock()
		delete(PendingGrading, subID)
		PendingGradingMu.Unlock()

		verdictTime := time.Now().UnixMilli()
		SafeCleanupDir(workspacePath)

		BroadcastTelemetry(map[string]interface{}{
			"type": "submission_graded",
			"data": map[string]interface{}{
				"id":           subID,
				"status":       "Finished",
				"verdict":      res.Status,
				"verdict_time": verdictTime,
				"latency":      res.LatencyMs,
			},
		})

		// SCORING ENGINE LOGIC
		attemptsKey := fmt.Sprintf("attempts:%s:%s", teamName, questionID)
		isSolved, _ := broker.Client.HExists(broker.Ctx, fmt.Sprintf("scores:%s", teamName), questionID).Result()

		if !isSolved {
			if res.Status != "AC" {
				broker.Client.Incr(broker.Ctx, attemptsKey)
			} else {
				broker.Client.Incr(broker.Ctx, fmt.Sprintf("stats:total_acs:%s", teamName))
				elapsedSeconds := time.Now().Unix() - startTimeUnix
				if elapsedSeconds < 0 {
					elapsedSeconds = 0
				}
				
				timeDecay := elapsedSeconds * 2
				attemptsStr, _ := broker.Client.Get(broker.Ctx, attemptsKey).Result()
				attempts, _ := strconv.Atoi(attemptsStr)
				incorrectPenalty := attempts * 50

				score := 500 - timeDecay - int64(incorrectPenalty)
				if score < 200 {
					score = 200
				}

				broker.Client.HSet(broker.Ctx, fmt.Sprintf("scores:%s", teamName), questionID, strconv.FormatInt(score, 10))
				updateTeamTotalScore(teamName)
			}
		}
	}()

	fmt.Printf("📥 NEW SUBMISSION RECEIVED: Team: %s, Question: %s, File: %s\n", teamName, questionID, handler.Filename)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":        "success",
		"submission_id": submissionID,
		"message":       "Code successfully quarantined and sent to grading engine.",
	})
}

func updateTeamTotalScore(teamName string) {
	scoresMap, err := broker.Client.HGetAll(broker.Ctx, fmt.Sprintf("scores:%s", teamName)).Result()
	if err != nil {
		return
	}

	var totalScore int64 = 0
	for _, valStr := range scoresMap {
		val, _ := strconv.ParseInt(valStr, 10, 64)
		totalScore += val
	}

	broker.Client.ZAdd(broker.Ctx, "leaderboard", redis.Z{
		Score:  float64(totalScore),
		Member: teamName,
	})

	PublishLeaderboardUpdate()
}

func PublishLeaderboardUpdate() {
	standings, err := broker.Client.ZRevRangeWithScores(broker.Ctx, "leaderboard", 0, -1).Result()
	if err != nil {
		return
	}

	type LeaderboardEntry struct {
		Team       string `json:"team"`
		Score      int    `json:"score"`
		Attempts   int    `json:"attempts"`
		Accepted   int    `json:"accepted"`
		Efficiency string `json:"efficiency"`
	}

	var leaderboardList []LeaderboardEntry = make([]LeaderboardEntry, 0)
	for _, entry := range standings {
		teamName := entry.Member.(string)
		
		attemptsStr, _ := broker.Client.Get(broker.Ctx, fmt.Sprintf("stats:total_attempts:%s", teamName)).Result()
		attempts, _ := strconv.Atoi(attemptsStr)
		
		acsStr, _ := broker.Client.Get(broker.Ctx, fmt.Sprintf("stats:total_acs:%s", teamName)).Result()
		acs, _ := strconv.Atoi(acsStr)
		
		efficiency := "0%"
		if attempts > 0 {
			eff := float64(acs) / float64(attempts) * 100
			efficiency = fmt.Sprintf("%.1f%%", eff)
		}

		leaderboardList = append(leaderboardList, LeaderboardEntry{
			Team:       teamName,
			Score:      int(entry.Score),
			Attempts:   attempts,
			Accepted:   acs,
			Efficiency: efficiency,
		})
	}

	stats := map[string]interface{}{
		"type": "leaderboard",
		"data": leaderboardList,
	}

	BroadcastTelemetry(stats)
}

func cancelBotAttacks() {
	attackCmd := map[string]interface{}{
		"submission_id":         "cancel",
		"target_endpoint":       "",
		"bot_concurrency":       0,
		"test_duration_seconds": -1,
	}
	payload, _ := json.Marshal(attackCmd)
	broker.Client.Publish(broker.Ctx, "attack_commands", payload)
}

func ResetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	contestMu.Lock()
	if contestCancelFunc != nil {
		contestCancelFunc()
		contestCancelFunc = nil
		cancelBotAttacks()
	}
	contestIsActive = false
	contestMu.Unlock()

	broker.Client.FlushAll(broker.Ctx)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Redis Database flushed and contest reset!",
	})
	
	BroadcastTelemetry(map[string]interface{}{
		"type": "leaderboard",
		"data": []interface{}{},
	})
	BroadcastTelemetry(map[string]interface{}{
		"type": "live_feed_log",
		"log":  "⚠️ **CONTEST RESET** by Administrator! All scores cleared.",
	})
}

func ContestStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	contestMu.Lock()
	active := contestIsActive
	var timeRemaining int64 = 0
	if active {
		timeRemaining = int64(time.Until(contestEndTime).Seconds())
		if timeRemaining < 0 {
			timeRemaining = 0
			contestIsActive = false
			active = false
		}
	}
	contestMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active":                 active,
		"time_remaining_seconds": timeRemaining,
	})
}

func StartContestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	targetEndpoint := r.FormValue("target")
	if targetEndpoint == "" {
		targetEndpoint = "https://jsonplaceholder.typicode.com/posts/1"
	}

	contestMu.Lock()
	if contestCancelFunc != nil {
		contestCancelFunc() // cancel any running contest uploader thread
	}
	
	contestCtx, cancelContest := context.WithTimeout(context.Background(), 5*time.Minute)
	contestCancelFunc = cancelContest
	contestEndTime = time.Now().Add(5 * time.Minute)
	contestIsActive = true
	contestMu.Unlock()

	// Trigger staggered team simulation in background
	go func() {
		defer func() {
			contestMu.Lock()
			contestIsActive = false
			contestMu.Unlock()
			cancelContest()
			cancelBotAttacks()
		}()

		fmt.Println("🎬 [CONTEST START] Initiating 10 competing teams under a 5-minute timeout ceiling...")
		teams := []string{
			"ApexQuant", "HFT_Speedrunners", "AlphaArbitrage", "DeltaMarketMaker",
			"BetaBook", "OmegaTrader", "SigmaExchange", "GammaLiquidity",
			"ZetaExecution", "ThetaSpread",
		}
		
		dir := "./cmd/director/dummy_submissions"
		startTime := time.Now()
		startTimeUnix := startTime.Unix()
		
		// Flush Redis clean
		broker.Client.FlushAll(broker.Ctx)
		broker.Client.Set(broker.Ctx, "contest:start_time", strconv.FormatInt(startTimeUnix, 10), 0)

		BroadcastTelemetry(map[string]interface{}{
			"type": "leaderboard",
			"data": []interface{}{},
		})
		BroadcastTelemetry(map[string]interface{}{
			"type": "live_feed_log",
			"log":  "🏁 **CONTEST STARTED!** 5-minute limit is active.",
		})

		var wg sync.WaitGroup

		// Contest-wide 5-minute bot noise fleet running in parallel with submissions
		go func() {
			attackCmd := map[string]interface{}{
				"submission_id":         fmt.Sprintf("contest_noise_%d", time.Now().Unix()),
				"target_endpoint":       targetEndpoint,
				"bot_concurrency":       50,
				"test_duration_seconds": 300, // 5 minutes
			}
			payload, _ := json.Marshal(attackCmd)
				broker.Client.Publish(broker.Ctx, "attack_commands", payload)
		}()

		for _, teamName := range teams {
			wg.Add(1)
			go func(team string) {
				defer wg.Done()
				unsolved := []string{"q1", "q2", "q3", "q4", "q5"}
				attemptsMap := make(map[string]int)

				for len(unsolved) > 0 {
					// Exit immediately if contest timed out
					select {
					case <-contestCtx.Done():
						return
					default:
					}

					// Randomly pick an unsolved question
					idx := rand.Intn(len(unsolved))
					qId := unsolved[idx]

					// CP SANITY CHECK: Check if already solved
					isAlreadySolved, _ := broker.Client.HExists(broker.Ctx, fmt.Sprintf("scores:%s", team), qId).Result()
					if isAlreadySolved {
						unsolved = append(unsolved[:idx], unsolved[idx+1:]...)
						continue
					}

					attemptsMap[qId]++
					currentAttempt := attemptsMap[qId]
					
					broker.Client.Incr(broker.Ctx, fmt.Sprintf("stats:total_attempts:%s", team))
					
					codingTime := time.Duration(rand.Intn(17)+8) * time.Second
					
					// Sleek interruptible delay: Terminate immediately if context cancels during sleep
					timer := time.NewTimer(codingTime)
					select {
					case <-contestCtx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
					
					fileName := fmt.Sprintf("%s_%s.js", qId, chosenBehaviour(qId))
					filePath := filepath.Join(dir, fileName)

					workspacePath := filepath.Join(".", "temp_sandboxes", fmt.Sprintf("%s_%s_%d", team, qId, time.Now().UnixNano()))
					os.MkdirAll(workspacePath, os.ModePerm)
					
					srcFile, err := os.Open(filePath)
					if err == nil {
						destFile, err := os.Create(filepath.Join(workspacePath, fileName))
						if err == nil {
							io.Copy(destFile, srcFile)
							destFile.Close()
						}
						srcFile.Close()
					}

					submitTime := time.Now().UnixMilli()
					subID := fmt.Sprintf("SUB_%d_%d", submitTime, rand.Intn(1000))

					BroadcastTelemetry(map[string]interface{}{
						"type": "submission_queued",
						"data": map[string]interface{}{
							"id":          subID,
							"team":        team,
							"question":    qId,
							"status":      "Running",
							"submit_time": submitTime,
						},
					})

					resChan := make(chan models.GradingResult)
					PendingGradingMu.Lock()
					PendingGrading[subID] = resChan
					PendingGradingMu.Unlock()

					fileContent, _ := os.ReadFile(filepath.Join(workspacePath, fileName))
					task := models.GradingTask{
						SubmissionID: subID,
						Team:         team,
						QuestionID:   qId,
						FileName:     fileName,
						FileContent:  fileContent,
					}
					taskBytes, _ := json.Marshal(task)
					broker.Client.LPush(broker.Ctx, "grading_queue", string(taskBytes))

					res := <-resChan

					PendingGradingMu.Lock()
					delete(PendingGrading, subID)
					PendingGradingMu.Unlock()
					verdictTime := time.Now().UnixMilli()
					SafeCleanupDir(workspacePath)

					BroadcastTelemetry(map[string]interface{}{
						"type": "submission_graded",
						"data": map[string]interface{}{
							"id":           subID,
							"status":       "Finished",
							"verdict":      res.Status,
							"verdict_time": verdictTime,
							"latency":      res.LatencyMs,
						},
					})

					attemptsKey := fmt.Sprintf("attempts:%s:%s", team, qId)

					if res.Status != "AC" {
						broker.Client.Incr(broker.Ctx, attemptsKey)
					} else {
						broker.Client.Incr(broker.Ctx, fmt.Sprintf("stats:total_acs:%s", team))
						curElapsed := time.Now().Unix() - startTimeUnix
						timeDecay := curElapsed * 2
						
						prevAttempts := currentAttempt - 1
						incorrectPenalty := int64(prevAttempts * 50)
						score := 500 - timeDecay - incorrectPenalty
						if score < 200 {
							score = 200
						}

						broker.Client.HSet(broker.Ctx, fmt.Sprintf("scores:%s", team), qId, strconv.FormatInt(score, 10))
						
						BroadcastTelemetry(map[string]interface{}{
							"type": "live_feed_log",
							"log":  fmt.Sprintf("🏆 Team **%s** got **ACCEPTED** on %s! Score: **%d** (Attempts: %d, Decay: -%d)", team, qId, score, prevAttempts, timeDecay),
						})
						
						updateTeamTotalScore(team)
						unsolved = append(unsolved[:idx], unsolved[idx+1:]...) // Remove from pool since AC
					}
				}
			}(teamName)
		}

		// 2. Controlled Concurrency Barrier: Wait for finish OR timeout
		doneChan := make(chan struct{})
		go func() {
			wg.Wait()
			close(doneChan)
		}()

		select {
		case <-doneChan:
			fmt.Println("🏁 [CONTEST COMPLETE] All teams finished naturally.")
			BroadcastTelemetry(map[string]interface{}{
				"type": "live_feed_log",
				"log":  "🏁 **CONTEST COMPLETED!** All teams have finished.",
			})
		case <-contestCtx.Done():
			fmt.Println("⏱️ [CONTEST TIMEOUT] 5 minutes elapsed! Force-cancelling remaining uploaders...")
			BroadcastTelemetry(map[string]interface{}{
				"type": "live_feed_log",
				"log":  "⏱️ **CONTEST TIMED OUT!** 5-minute limit reached. Force-terminating remaining teams.",
			})
		}

		time.Sleep(5 * time.Second)
		BroadcastTelemetry(map[string]interface{}{
			"type": "live_feed_log_clear",
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                 "success",
		"message":                "Contest simulation started!",
		"time_remaining_seconds": 300,
	})
}

func QuestionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	questions := map[string]interface{}{
		"q1": map[string]interface{}{
			"title": "Sort an Array",
			"description": "Given a comma-separated list of integers, return them sorted in ascending order.",
			"sample_input": "5,3,8,1,2",
			"sample_output": "1,2,3,5,8",
		},
		"q2": map[string]interface{}{
			"title": "Average of Two Numbers",
			"description": "Given two space-separated integers, return their average.",
			"sample_input": "10 20",
			"sample_output": "15",
		},
		"q3": map[string]interface{}{
			"title": "Search Index",
			"description": "Given a space-separated array of integers on the first line, and a target integer on the second line, return the 0-based index of the target in the array. Return -1 if not found.",
			"sample_input": "10 20 30 40\n30",
			"sample_output": "2",
		},
		"q4": map[string]interface{}{
			"title": "Modulo Operation",
			"description": "Given two space-separated integers 'a' and 'b', return the result of 'a % b'.",
			"sample_input": "10 3",
			"sample_output": "1",
		},
		"q5": map[string]interface{}{
			"title": "Log Base 2",
			"description": "Given an integer, return its logarithm base 2.",
			"sample_input": "8",
			"sample_output": "3",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(questions)
}

// Stagger helper to dynamically sample a script behavior
func chosenBehaviour(qID string) string {
	behaviours := []string{"fast_ac", "slow_ac", "wa", "tle", "mle"}
	return behaviours[rand.Intn(len(behaviours))]
}

func SafeCleanupDir(path string) {
	go func() {
		for i := 0; i < 15; i++ {
			time.Sleep(1 * time.Second)
			err := os.RemoveAll(path)
			if err == nil {
				fmt.Printf("🧹 [CLEANER] Successfully cleaned quarantined folder: %s\n", path)
				return
			}
			fmt.Printf("⏳ [CLEANER] Folder %s locked by Docker daemon volume unmount lag. Retrying... (Attempt %d/15)\n", path, i+1)
		}
	}()
}
