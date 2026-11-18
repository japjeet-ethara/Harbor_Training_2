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

type RewardReport struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	var buildScore float64 = 0.0
	var noPythonScore float64 = 0.0
	var behaviourScore float64 = 0.0

	// 1. Build check (weight 1)
	// Runs `go build -o /tmp/figlet .` in /workspace/repo
	buildCmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	buildCmd.Dir = "/workspace/repo"
	var buildOutput bytes.Buffer
	buildCmd.Stdout = &buildOutput
	buildCmd.Stderr = &buildOutput

	if err := buildCmd.Run(); err == nil {
		buildScore = 1.0
		fmt.Println("[grader] Build successful: /tmp/figlet created.")
	} else {
		fmt.Printf("[grader] Build failed: %v\nOutput: %s\n", err, buildOutput.String())
	}

	// 2 & 3. If build passed, evaluate behaviour on hidden.jsonl
	totalCases := 0
	matchedCases := 0

	if buildScore == 1.0 {
		dataFile := "/tests/data/hidden.jsonl"
		file, err := os.Open(dataFile)
		if err != nil {
			fmt.Printf("[grader] Error opening %s: %v\n", dataFile, err)
		} else {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			// Increase buffer capacity for large JSON lines
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

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
				var stdoutBuf bytes.Buffer
				cmd.Stdout = &stdoutBuf

				if err := cmd.Run(); err == nil {
					if stdoutBuf.String() == tc.Out {
						matchedCases++
					}
				}
				cancel()
			}
		}

		if totalCases > 0 {
			behaviourScore = float64(matchedCases) / float64(totalCases)
		}
		fmt.Printf("[grader] Behaviour: %d / %d matched (%.4f)\n", matchedCases, totalCases, behaviourScore)

		// Check for .py files in /workspace/repo
		hasPython := false
		err = filepath.WalkDir("/workspace/repo", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
				hasPython = true
				return filepath.SkipAll
			}
			return nil
		})

		// no_python (weight 1):
		// 1 only if build passed AND no .py file exists under /workspace/repo AND at least one hidden case matches
		if !hasPython && matchedCases >= 1 {
			noPythonScore = 1.0
			fmt.Println("[grader] no_python check passed (no .py files and >= 1 match).")
		} else {
			fmt.Printf("[grader] no_python check failed: hasPython=%v, matchedCases=%d\n", hasPython, matchedCases)
		}
	} else {
		fmt.Println("[grader] Skipping behaviour and no_python checks due to build failure.")
	}

	// Calculate weighted reward (build: 1, no_python: 1, behaviour: 4, total: 6)
	weightedReward := (1.0*buildScore + 1.0*noPythonScore + 4.0*behaviourScore) / 6.0

	report := RewardReport{
		Reward:    weightedReward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}

	// Write /logs/verifier/reward.json
	_ = os.MkdirAll("/logs/verifier", 0755)
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile("/logs/verifier/reward.json", reportJSON, 0644); err != nil {
		fmt.Printf("[grader] Failed to write reward.json: %v\n", err)
	}

	fmt.Println("--- GRADER REPORT ---")
	fmt.Printf("Build:     %.4f\n", buildScore)
	fmt.Printf("No Python: %.4f\n", noPythonScore)
	fmt.Printf("Behaviour: %.4f\n", behaviourScore)
	fmt.Printf("Reward:    %.4f\n", weightedReward)
	fmt.Println("---------------------")

	// 5. SECRET
	if behaviourScore == 1.0 {
		secretRaw, err := os.ReadFile("/tests/secret.b64")
		if err == nil {
			secretText, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretRaw)))
			if err == nil {
				secretCmd := exec.Command("/tmp/figlet", "-f", "slant", string(secretText))
				out, err := secretCmd.CombinedOutput()
				if err == nil {
					fmt.Println("*** SECRET UNLOCKED ***")
					fmt.Print(string(out))
				}
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
