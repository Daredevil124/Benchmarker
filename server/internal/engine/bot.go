package engine

import (
	"context"
	"iicpc-backend/internal/models"
	"net/http"
	"time"
)

func FireBot(ctx context.Context, botID int, targetURL string, results chan<- models.AttackResult) {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()
		resp, err := client.Get(targetURL)
		latency := time.Since(start).Milliseconds()

		var res models.AttackResult
		if err != nil {
			res = models.AttackResult{
				BotID: botID,
				Error: err.Error(),
			}
		} else {
			res = models.AttackResult{
				BotID:      botID,
				Latency:    latency,
				StatusCode: resp.StatusCode,
			}
			resp.Body.Close()
		}

		select {
		case <-ctx.Done():
			return
		case results <- res:
		}

		// Sleek delay between requests to keep the load high but stable
		time.Sleep(50 * time.Millisecond)
	}
}
