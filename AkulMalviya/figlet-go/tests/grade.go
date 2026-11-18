package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func hasPyFiles(root string) bool {
	hasPy := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".py") {
			hasPy = true
			return filepath.SkipAll
		}
		return nil
	})
	return hasPy
}

func main() {
	var buildScore float64 = 0.0
	var noPythonScore float64 = 0.0
	var behaviourScore float64 = 0.0

	// 1. Build check
	buildCmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	buildCmd.Dir = "/workspace/repo"
	buildOutput, buildErr := buildCmd.CombinedOutput()
	if buildErr == nil {
		buildScore = 1.0
	} else {
		fmt.Printf("Build failed: %v\nOutput: %s\n", buildErr, string(buildOutput))
	}

	correctCases := 0
	totalCases := 0

	// 2 & 3. Behaviour check
	if buildScore == 1.0 {
		dataFile := "/tests/data/hidden.jsonl"
		if _, err := os.Stat(dataFile); os.IsNotExist(err) {
			// fallback for local testing
			dataFile = "data/hidden.jsonl"
		}

		f, err := os.Open(dataFile)
		if err != nil {
			fmt.Printf("Could not open hidden cases: %v\n", err)
		} else {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			buf := make([]byte, 1024*1024)
			scanner.Buffer(buf, 10*1024*1024)

			for scanner.Scan() {
				line := scanner.Text()
				if strings.TrimSpace(line) == "" {
					continue
				}
				var tc TestCase
				if err := json.Unmarshal([]byte(line), &tc); err != nil {
					continue
				}
				totalCases++

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
				out, err := cmd.Output()
				cancel()

				if err == nil && string(out) == tc.Out {
					correctCases++
				}
			}

			if totalCases > 0 {
				behaviourScore = float64(correctCases) / float64(totalCases)
			}
		}

		// no_python check: build passed AND no .py file under /workspace/repo AND at least one hidden case matches
		pyExists := hasPyFiles("/workspace/repo")
		if !pyExists && correctCases > 0 {
			noPythonScore = 1.0
		} else if pyExists {
			fmt.Println("Python (.py) files still exist under /workspace/repo")
		}
	}

	// Weighted mean: build (1), no_python (1), behaviour (4)
	weightedReward := (buildScore*1.0 + noPythonScore*1.0 + behaviourScore*4.0) / 6.0

	rewardResult := RewardResult{
		Reward:    weightedReward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}

	// Write /logs/verifier/reward.json
	_ = os.MkdirAll("/logs/verifier", 0755)
	rewardData, _ := json.MarshalIndent(rewardResult, "", "  ")
	_ = os.WriteFile("/logs/verifier/reward.json", rewardData, 0644)

	fmt.Printf("Results: Build=%.2f, NoPython=%.2f, Behaviour=%.4f (%d/%d), Total Reward=%.4f\n",
		buildScore, noPythonScore, behaviourScore, correctCases, totalCases, weightedReward)

	// Secret check
	if behaviourScore == 1.0 {
		secretPath := "/tests/secret.b64"
		if _, err := os.Stat(secretPath); os.IsNotExist(err) {
			secretPath = "secret.b64"
		}
		secretBytes, err := os.ReadFile(secretPath)
		if err == nil {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretBytes)))
			if err == nil {
				fmt.Println("*** SECRET UNLOCKED ***")
				cmd := exec.Command("/tmp/figlet", "-f", "slant", string(decoded))
				out, err := cmd.CombinedOutput()
				if err == nil {
					fmt.Print(string(out))
				} else {
					fmt.Printf("Failed to render secret: %v\n", err)
				}
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
