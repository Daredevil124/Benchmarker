package engine

import (
	"iicpc-backend/internal/models"
	"net/http"
	"sync"
	"time"
)

func FireBot(botID int, targetURL string, wg *sync.WaitGroup, results chan<- models.AttackResult) {
	defer wg.Done()
	start := time.Now()
	resp, err := http.Get(targetURL)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		results <- models.AttackResult{
			BotID: botID,
			Error: err.Error(),
		}
		return
	}
	defer resp.Body.Close()
	results <- models.AttackResult{
		BotID:      botID,
		Latency:    latency,
		StatusCode: resp.StatusCode,
	}
}
