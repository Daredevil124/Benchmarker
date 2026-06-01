package handlers

import (
	"net/http"
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
