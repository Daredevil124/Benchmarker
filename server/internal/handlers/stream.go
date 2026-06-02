package handlers

import (
	"fmt"
	"iicpc-backend/internal/broker"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
var clients = make(map[*websocket.Conn]bool)
var clientsMutex sync.Mutex

func StreamHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	
	clientsMutex.Lock()
	clients[ws] = true
	clientsMutex.Unlock()

	// INSTANT UPDATE ON CONNECT: Query Redis and immediately send current standings
	sendCurrentStandingsToClient(ws)

	// Keep connection alive and listen for exit
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			clientsMutex.Lock()
			delete(clients, ws)
			clientsMutex.Unlock()
			break
		}
	}
}

func BroadcastTelemetry(stats map[string]interface{}) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for client := range clients {
		err := client.WriteJSON(stats)
		if err != nil {
			client.Close()
			delete(clients, client)
		}
	}
}

// Helper to push the current Redis scoreboard directly to a single socket
func sendCurrentStandingsToClient(ws *websocket.Conn) {
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

	// Write directly to this connection
	err = ws.WriteJSON(stats)
	if err != nil {
		fmt.Printf("⚠️ Failed to write initial standings to new WS client: %v\n", err)
	}
}
