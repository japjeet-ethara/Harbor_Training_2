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

type RewardOutput struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	var buildScore float64 = 0.0
	var noPythonScore float64 = 0.0
	var behaviourScore float64 = 0.0

	binaryPath := "/tmp/figlet"
	repoDir := "/workspace/repo"

	// 1. Build step (weight 1)
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = repoDir
	var buildOut bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut
	if err := buildCmd.Run(); err == nil {
		buildScore = 1.0
		fmt.Println("Build succeeded")
	} else {
		fmt.Printf("Build failed: %v\n%s\n", err, buildOut.String())
	}

	// Check for any .py file under repoDir
	hasPyFiles := false
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			hasPyFiles = true
		}
		return nil
	})

	// 2 & 3. Behaviour (weight 4) on /tests/data/hidden.jsonl
	matchedCases := 0
	totalCases := 0
	hiddenDataPath := "/tests/data/hidden.jsonl"

	file, err := os.Open(hiddenDataPath)
	if err != nil {
		fmt.Printf("Failed to open %s: %v\n", hiddenDataPath, err)
	} else {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		// Allocate larger buffer in case of large lines
		buf := make([]byte, 1024*1024)
		scanner.Buffer(buf, 10*1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var tc TestCase
			if err := json.Unmarshal(line, &tc); err != nil {
				continue
			}
			totalCases++

			if buildScore == 1.0 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctx, binaryPath, tc.Args...)
				var stdout bytes.Buffer
				cmd.Stdout = &stdout
				if err := cmd.Run(); err == nil {
					if stdout.String() == tc.Out {
						matchedCases++
					}
				}
				cancel()
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Scanner error reading %s: %v\n", hiddenDataPath, err)
		}
	}

	if totalCases > 0 {
		behaviourScore = float64(matchedCases) / float64(totalCases)
	}

	// no_python (weight 1): 1 only if build passed AND no .py exists under /workspace/repo AND at least one hidden case matches
	if buildScore == 1.0 && !hasPyFiles && matchedCases >= 1 {
		noPythonScore = 1.0
	}

	// Weighted mean: (build*1 + no_python*1 + behaviour*4) / 6.0
	weightedReward := (buildScore*1.0 + noPythonScore*1.0 + behaviourScore*4.0) / 6.0

	rewardData := RewardOutput{
		Reward:    weightedReward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}

	logDir := "/logs/verifier"
	_ = os.MkdirAll(logDir, 0755)
	rewardFilePath := filepath.Join(logDir, "reward.json")
	if data, err := json.MarshalIndent(rewardData, "", "  "); err == nil {
		_ = os.WriteFile(rewardFilePath, data, 0644)
	}

	fmt.Printf("Results: Build=%.2f NoPython=%.2f Behaviour=%.4f TotalReward=%.4f (%d/%d matched)\n",
		buildScore, noPythonScore, behaviourScore, weightedReward, matchedCases, totalCases)

	// Secret unlock logic
	if behaviourScore == 1.0 {
		secretFile := "/tests/secret.b64"
		secB64, err := os.ReadFile(secretFile)
		if err != nil {
			fmt.Printf("Failed to read secret file: %v\n", err)
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secB64)))
		if err != nil {
			fmt.Printf("Failed to decode secret: %v\n", err)
			return
		}
		fmt.Println("*** SECRET UNLOCKED ***")
		slantCmd := exec.Command(binaryPath, "-f", "slant", string(decoded))
		slantCmd.Stdout = os.Stdout
		slantCmd.Stderr = os.Stderr
		_ = slantCmd.Run()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

