package handlers

import (
	"encoding/json"
	"fmt"
	"iicpc-backend/internal/engine"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Method", "POST OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	r.ParseMultipartForm(10 << 20)
	file, handler, err := r.FormFile("code_file")
	if err != nil {
		http.Error(w, "failed to Retrieve the file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	submissionID := fmt.Sprintf("sub_%d", time.Now().Unix())
	workspacePath := filepath.Join(".", "temp_sandboxes", submissionID)
	if err := os.Mkdir(workspacePath, os.ModePerm); err != nil {
		http.Error(w, "Failed to create workspace", http.StatusInternalServerError)
		return
	}
	destinationPath := filepath.Join(workspacePath, handler.Filename)
	destinationFile, err := os.Create(destinationPath)
	defer destinationFile.Close()
	if _, err := io.Copy(destinationFile, file); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}
	go func() {
		err := engine.RunInSandbox(workspacePath, handler.Filename)
		if err != nil {
			fmt.Println("Sandbox failed:", err)
		}
		os.RemoveAll(workspacePath)
		fmt.Println("Workspace wiped from disk")
	}()
	fmt.Printf("📥 NEW SUBMISSION RECEIVED: %s -> %s\n", handler.Filename, workspacePath)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":        "success",
		"submission_id": submissionID,
		"message":       "Code successfully quarantined in workspace.",
	})
}
