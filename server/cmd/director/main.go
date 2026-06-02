package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func main() {
	targetURL := flag.String("target", "http://localhost:9000/api/upload", "API URL to submit file")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	
	// Folder containing the 25 physical files we wrote earlier
	dir := "./dummy_submissions"
	
	teams := []string{
		"ApexQuant", "HFT_Speedrunners", "AlphaArbitrage", "DeltaMarketMaker",
		"BetaBook", "OmegaTrader", "SigmaExchange", "GammaLiquidity",
		"ZetaExecution", "ThetaSpread",
	}
	
	var wg sync.WaitGroup
	startTime := time.Now()
	fmt.Printf("🎬 STARTING CONTEST: 10 teams simulating concurrent coding submissions...\n\n")
	
	for _, teamName := range teams {
		wg.Add(1)
		go func(team string) {
			defer wg.Done()
			for qNum := 1; qNum <= 5; qNum++ {
				qId := fmt.Sprintf("q%d", qNum)
				resolved := false
				attempts := 0
				
				for !resolved && attempts < 4 {
					attempts++
					
					// Thinking/coding delay between 5 to 15 seconds
					codingTime := time.Duration(rand.Intn(10)+5) * time.Second
					time.Sleep(codingTime)
					
					behaviours := []string{"fast_ac", "slow_ac", "wa", "tle", "mle"}
					chosenBehaviour := behaviours[rand.Intn(len(behaviours))]
					
					fileName := fmt.Sprintf("%s_%s.js", qId, chosenBehaviour)
					filePath := filepath.Join(dir, fileName)
					
					elapsedSeconds := int(time.Since(startTime).Seconds())
					fmt.Printf("🚀 [Team %s] Submitting %s (Attempt %d) at T+%ds using '%s' behavior...\n",
						team, qId, attempts, elapsedSeconds, chosenBehaviour)
					
					err := uploadSubmission(*targetURL, team, qId, filePath)
					if err != nil {
						fmt.Printf("❌ [Team %s] Submission failed: %v\n", team, err)
						continue
					}
					
					if chosenBehaviour == "fast_ac" || chosenBehaviour == "slow_ac" {
						resolved = true
						fmt.Printf("🏆 [Team %s] Solved %s successfully!\n", team, qId)
					} else {
						fmt.Printf("⚠️ [Team %s] Penalty on %s. Retrying...\n", team, qId)
					}
				}
			}
			fmt.Printf("🏁 [Team %s] Finished the contest!\n", team)
		}(teamName)
	}
	
	wg.Wait()
	fmt.Println("\n🏁 CONTEST OVER! All dummy submissions have completed.")
}

func uploadSubmission(url string, team string, question string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("code_file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}
	
	err = writer.WriteField("team", team)
	if err != nil {
		return err
	}
	
	err = writer.WriteField("question", question)
	if err != nil {
		return err
	}
	
	err = writer.Close()
	if err != nil {
		return err
	}
	
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s, body: %s", resp.Status, string(respBody))
	}
	
	return nil
}
