package handlers

import (
	"encoding/json"
	"fmt"
	"iicpc-backend/internal/broker"
	"iicpc-backend/internal/models"
	"net/http"
)

func HeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "online",
		"message": "Enterprise Backend is alive and ready",
	})
}
func AttackHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	var command models.AttackCommand
	if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
		http.Error(w, "Invalid Json format", http.StatusBadRequest)
		return
	}
	payload, err := json.Marshal(command)
	if err != nil {
		http.Error(w, "Failed to serialize the command", http.StatusInternalServerError)
		return
	}
	fmt.Printf("[MASTER] Broadcasting attack command via Redis channel 'attack_commands'...\n")
	err = broker.Client.Publish(broker.Ctx, "attack_commands", payload).Err()
	if err != nil {
		http.Error(w, "Message Broker Failure", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Attack distributed to worker clusters successfully",
	})
}
