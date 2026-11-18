package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type RewardResult struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	workspaceRepo := "/workspace/repo"
	binaryPath := "/tmp/figlet"
	hiddenCasesPath := "/tests/data/hidden.jsonl"
	secretPath := "/tests/secret.b64"
	rewardPath := "/logs/verifier/reward.json"

	var buildScore float64
	var noPythonScore float64
	var behaviourScore float64

	// 1. Build check
	goModPath := filepath.Join(workspaceRepo, "go.mod")
	goModValid := false
	if modBytes, err := os.ReadFile(goModPath); err == nil {
		hasRequireOrReplace := false
		scanner := bufio.NewScanner(bytes.NewReader(modBytes))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "//") {
				continue
			}
			if strings.HasPrefix(line, "require") || strings.HasPrefix(line, "replace") {
				hasRequireOrReplace = true
				break
			}
		}
		if !hasRequireOrReplace {
			goModValid = true
		}
	}

	cmdBuild := exec.Command("go", "build", "-o", binaryPath, ".")
	cmdBuild.Dir = workspaceRepo
	buildErr := cmdBuild.Run()

	binaryOk := false
	if buildErr == nil {
		if fi, err := os.Stat(binaryPath); err == nil && !fi.IsDir() {
			binaryOk = true
		}
	}

	cmdBuildAll := exec.Command("go", "build", "./...")
	cmdBuildAll.Dir = workspaceRepo
	buildAllErr := cmdBuildAll.Run()

	if goModValid && binaryOk && buildAllErr == nil {
		buildScore = 1.0
	}

	// Count .py files under /workspace/repo
	pyCount := 0
	_ = filepath.WalkDir(workspaceRepo, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			pyCount++
		}
		return nil
	})

	// 2. Behaviour check
	matches := 0
	totalCases := 0

	if buildScore == 1.0 {
		file, err := os.Open(hiddenCasesPath)
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			// Allow scanning long lines
			buf := make([]byte, 1024*1024)
			scanner.Buffer(buf, 10*1024*1024)

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				var tc TestCase
				if err := json.Unmarshal([]byte(line), &tc); err != nil {
					continue
				}
				totalCases++

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctx, binaryPath, tc.Args...)
				var stdoutBuf bytes.Buffer
				cmd.Stdout = &stdoutBuf

				err := cmd.Run()
				cancel()

				if err == nil && stdoutBuf.String() == tc.Out {
					matches++
				}
			}
		}
	}

	if totalCases > 0 {
		behaviourScore = float64(matches) / float64(totalCases)
	}

	// no_python check: 1 only if build passed AND no .py files AND at least one matching hidden case
	if buildScore == 1.0 && pyCount == 0 && matches > 0 {
		noPythonScore = 1.0
	} else {
		noPythonScore = 0.0
	}

	// Weighted mean aggregation: build (w=1), no_python (w=1), behaviour (w=4)
	totalWeight := 1.0 + 1.0 + 4.0
	finalReward := (buildScore*1.0 + noPythonScore*1.0 + behaviourScore*4.0) / totalWeight

	result := RewardResult{
		Reward:    finalReward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}

	_ = os.MkdirAll(filepath.Dir(rewardPath), 0755)
	resBytes, _ := json.MarshalIndent(result, "", "  ")
	_ = os.WriteFile(rewardPath, resBytes, 0644)
	txtPath := "/logs/verifier/reward.txt"
	_ = os.WriteFile(txtPath, []byte(fmt.Sprintf("%.6f\n", finalReward)), 0644)
	fmt.Printf("Reward output written: %s\n", string(resBytes))

	// SECRET logic
	if behaviourScore == 1.0 {
		secretBytes, err := os.ReadFile(secretPath)
		if err == nil {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretBytes)))
			if err == nil {
				cmdSecret := exec.Command(binaryPath, "-f", "slant", string(decoded))
				out, err := cmdSecret.CombinedOutput()
				if err == nil {
					fmt.Println("\n*** SECRET UNLOCKED ***")
					fmt.Println(string(out))
				}
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
