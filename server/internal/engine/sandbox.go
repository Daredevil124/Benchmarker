package engine

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func RunInSandbox(workspacePath string, filename string) error {
	fmt.Println("Intiating Docker Quarantine...")
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("Failed to get the absolute path!")
	}
	cmd := exec.Command("docker", "run",
		"--rm",
		"--memory=512m",
		"--cpus", "1.0",
		"-v", fmt.Sprintf("%s:/usr/src/app", absPath),
		"-w", "/usr/src/app",
		"node:alpine", "node", filename,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Sandbox Crashed: \n%s\n", string(output))
		return fmt.Errorf("Container Execution Failed")
	}
	fmt.Printf("Sandbox Execution Successful:\n%s\n", string(output))
	return nil
}
