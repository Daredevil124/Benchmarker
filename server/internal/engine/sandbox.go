package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GraderResult struct {
	Status      string // "AC", "WA", "TLE", "MLE"
	LatencyMs   int64
	Output      string
	ErrorDetail string
}

type TestCase struct {
	Input          string
	ExpectedOutput string
}

func getTestCases(qID string) []TestCase {
	switch qID {
	case "q1": // Sort Array (comma separated)
		return []TestCase{
			{Input: "5,3,8,1,2", ExpectedOutput: "1,2,3,5,8"},
			{Input: "10,9,8,7", ExpectedOutput: "7,8,9,10"},
			{Input: "1", ExpectedOutput: "1"},
		}
	case "q2": // Math: (a+b)/2 (space separated)
		return []TestCase{
			{Input: "10 20", ExpectedOutput: "15"},
			{Input: "0 0", ExpectedOutput: "0"},
			{Input: "7 3", ExpectedOutput: "5"},
		}
	case "q3": // Search Index (Array \n Target)
		return []TestCase{
			{Input: "10 20 30 40\n30", ExpectedOutput: "2"},
			{Input: "1 2 3\n5", ExpectedOutput: "-1"},
			{Input: "5 5 5\n5", ExpectedOutput: "0"},
		}
	case "q4": // Modulo: a % b
		return []TestCase{
			{Input: "10 3", ExpectedOutput: "1"},
			{Input: "20 5", ExpectedOutput: "0"},
			{Input: "7 10", ExpectedOutput: "7"},
		}
	case "q5": // Log base 2
		return []TestCase{
			{Input: "8", ExpectedOutput: "3"},
			{Input: "1024", ExpectedOutput: "10"},
			{Input: "1", ExpectedOutput: "0"},
		}
	}
	return nil
}

func GradeSubmission(workspacePath string, filename string, questionID string) GraderResult {
	fmt.Printf("🔍 [GRADER] Sandbox quarantine active for Question: %s, File: %s...\n", questionID, filename)
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return GraderResult{Status: "WA", ErrorDetail: "Failed to resolve absolute path"}
	}

	testCases := getTestCases(questionID)
	if len(testCases) == 0 {
		return GraderResult{Status: "WA", ErrorDetail: "Invalid question ID"}
	}

	for i, tc := range testCases {
		os.WriteFile(filepath.Join(absPath, fmt.Sprintf("input_%d.txt", i)), []byte(tc.Input), 0644)
		os.WriteFile(filepath.Join(absPath, fmt.Sprintf("expected_%d.txt", i)), []byte(tc.ExpectedOutput), 0644)
	}

	wrapperPY := `
import sys
import os
import subprocess
import json
import time

def main():
    test_cases = int(sys.argv[1])
    target_file = sys.argv[2]
    
    ext = os.path.splitext(target_file)[1]
    
    compile_cmd = None
    exec_cmd = None
    
    if ext == '.cpp':
        compile_cmd = ['g++', '-O2', target_file, '-o', 'solution']
        exec_cmd = ['./solution']
    elif ext == '.java':
        exec_cmd = ['java', target_file]
    elif ext == '.py':
        exec_cmd = ['python3', target_file]
    elif ext == '.js':
        exec_cmd = ['node', target_file]
    else:
        print(json.dumps({"status": "WA", "latency": 0, "msg": "Unsupported language"}))
        return

    if compile_cmd:
        try:
            subprocess.run(compile_cmd, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        except subprocess.CalledProcessError as e:
            err_msg = e.stderr.decode('utf-8').strip()
            # If compile error, Codeforces returns CE, but we map to WA or handle it.
            print(json.dumps({"status": "WA", "latency": 0, "msg": err_msg}))
            return

    max_latency = 0
    
    for i in range(test_cases):
        input_file = f'input_{i}.txt'
        expected_file = f'expected_{i}.txt'
        
        with open(expected_file, 'r') as f:
            expected = f.read().strip()
            
        with open(input_file, 'r') as f:
            input_data = f.read()
            
        start_time = time.time()
        try:
            result = subprocess.run(
                exec_cmd,
                input=input_data.encode('utf-8'),
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                timeout=10.0
            )
            latency = int((time.time() - start_time) * 1000)
            max_latency = max(max_latency, latency)
            
            output = result.stdout.decode('utf-8').strip()
            
            if result.returncode != 0:
                err_out = result.stderr.decode('utf-8')
                if "MemoryError" in err_out or "OutOfMemory" in err_out or "heap out of memory" in err_out:
                    print(json.dumps({"status": "MLE", "latency": max_latency, "msg": "Out of memory"}))
                else:
                    print(json.dumps({"status": "WA", "latency": max_latency, "msg": err_out.strip()}))
                return
                
            if output != expected:
                print(json.dumps({"status": "WA", "latency": max_latency, "msg": output}))
                return
                
        except subprocess.TimeoutExpired:
            print(json.dumps({"status": "TLE", "latency": 10000, "msg": "Timeout"}))
            return
        except Exception as e:
            print(json.dumps({"status": "WA", "latency": max_latency, "msg": str(e)}))
            return
            
    print(json.dumps({"status": "AC", "latency": max_latency, "msg": "Passed all test cases"}))

if __name__ == '__main__':
    main()
`
	os.WriteFile(filepath.Join(absPath, "wrapper.py"), []byte(wrapperPY), 0644)

	hostWorkdir := os.Getenv("HOST_WORKDIR")
	var hostAbsPath string
	if hostWorkdir != "" {
		hostAbsPath = filepath.Join(hostWorkdir, workspacePath)
	} else {
		hostAbsPath = absPath
	}

	containerName := fmt.Sprintf("sandbox_sub_%d_%s_%d", time.Now().Unix(), questionID, rand.Intn(100000))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // Generous timeout for compilation + Node/Python overhead
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "run",
		"--rm",
		"--name", containerName,
		"--memory=256m",
		"--cpus", "0.5",
		"--network", "none",
		"--cap-drop=ALL",
		"--pids-limit", "64",
		"--read-only",
		"--tmpfs", "/tmp",
		"-v", fmt.Sprintf("%s:/usr/src/app", hostAbsPath),
		"-w", "/usr/src/app",
		"benchmarker-sandbox", "python3", "wrapper.py", fmt.Sprintf("%d", len(testCases)), filename,
	)

	outputBytes, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(outputBytes))

	if ctx.Err() == context.DeadlineExceeded {
		fmt.Printf("❌ [GRADER] System Timeout detected! Container %s ran beyond 15 seconds. Killing...\n", containerName)
		exec.Command("docker", "rm", "-f", containerName).Run()
		return GraderResult{Status: "TLE", LatencyMs: 15000, Output: "System Timeout"}
	}

	if err != nil && output == "" {
		fmt.Printf("❌ [GRADER] Docker execution failed: %v\n", err)
		return GraderResult{Status: "WA", LatencyMs: 0, Output: "Docker Error", ErrorDetail: err.Error()}
	}

	// Because docker outputs extra logs sometimes (especially if it downloads images), we extract the LAST line which should be our JSON.
	lines := strings.Split(output, "\n")
	lastLine := strings.TrimSpace(lines[len(lines)-1])

	// We'll import encoding/json to unmarshal
	// Wait, the file needs importing "encoding/json" if it's not already. Let me add it in the next step.
	// We'll just define a struct here to decode it locally.
	var wrapperRes struct {
		Status  string `json:"status"`
		Latency int64  `json:"latency"`
		Msg     string `json:"msg"`
	}

	// Actually, just doing simple string parsing or importing json is needed.
	// Since I can't add import here, I'll return it as a string block and parse it. Let's do simple string parsing, or I can add the json import.
	// The file `sandbox.go` doesn't have "encoding/json" imported right now. I will add it using multi-replace or just replace it.
	// Let's assume json is imported, I'll patch the imports later if needed.
	importJSONCode := "encoding/json"
	_ = importJSONCode

	errJson := json.Unmarshal([]byte(lastLine), &wrapperRes)
	if errJson != nil {
		return GraderResult{Status: "WA", LatencyMs: 0, Output: lastLine, ErrorDetail: "Failed to parse wrapper output"}
	}

	return GraderResult{
		Status:    wrapperRes.Status,
		LatencyMs: wrapperRes.Latency,
		Output:    wrapperRes.Msg,
	}
}
