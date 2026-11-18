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

	// 1. Build check (weight 1)
	buildCmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	buildCmd.Dir = "/workspace/repo"
	var buildOutput bytes.Buffer
	buildCmd.Stdout = &buildOutput
	buildCmd.Stderr = &buildOutput
	if err := buildCmd.Run(); err == nil {
		buildScore = 1.0
		fmt.Println("[verifier] Build succeeded.")
	} else {
		fmt.Printf("[verifier] Build failed: %v\nOutput: %s\n", err, buildOutput.String())
	}

	// Read hidden test cases
	var testCases []TestCase
	hiddenPath := "/tests/data/hidden.jsonl"
	if f, err := os.Open(hiddenPath); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var tc TestCase
			if err := json.Unmarshal([]byte(line), &tc); err != nil {
				fmt.Printf("[verifier] Invalid case at %s:%d: %v\n", hiddenPath, lineNumber, err)
				continue
			}
			testCases = append(testCases, tc)
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("[verifier] Failed to read %s: %v\n", hiddenPath, err)
		}
		if err := f.Close(); err != nil {
			fmt.Printf("[verifier] Failed to close %s: %v\n", hiddenPath, err)
		}
	} else {
		fmt.Printf("[verifier] Warning: failed to open %s: %v\n", hiddenPath, err)
	}

	totalCases := len(testCases)
	matchedCases := 0

	// 3. Behaviour check (weight 4)
	if buildScore == 1.0 && totalCases > 0 {
		for i, tc := range testCases {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
			var stdoutBuf bytes.Buffer
			cmd.Stdout = &stdoutBuf
			err := cmd.Run()
			cancel()

			if err == nil && stdoutBuf.String() == tc.Out {
				matchedCases++
			} else if i < 3 && err != nil {
				// Log first few errors for debugging
				fmt.Printf("[verifier] Case %d error: %v (args: %v)\n", i, err, tc.Args)
			}
		}
		behaviourScore = float64(matchedCases) / float64(totalCases)
		fmt.Printf("[verifier] Behaviour score: %d/%d (%.4f)\n", matchedCases, totalCases, behaviourScore)
	} else {
		fmt.Println("[verifier] Behaviour skipped due to build failure or missing cases.")
	}

	// 2. no_python check (weight 1)
	hasPythonFiles := false
	if buildScore == 1.0 {
		walkErr := filepath.WalkDir("/workspace/repo", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
				hasPythonFiles = true
				return filepath.SkipAll
			}
			return nil
		})
		if walkErr != nil {
			fmt.Printf("[verifier] Failed to inspect /workspace/repo for Python files: %v\n", walkErr)
		}

		if walkErr == nil && !hasPythonFiles && matchedCases >= 1 {
			noPythonScore = 1.0
			fmt.Println("[verifier] No python files detected and at least one hidden case matched.")
		} else {
			fmt.Printf("[verifier] no_python failed: hasPythonFiles=%v, matchedCases=%d\n", hasPythonFiles, matchedCases)
		}
	}

	// 4. Weighted reward calculation
	// Total weight = 1 (build) + 1 (no_python) + 4 (behaviour) = 6
	reward := (buildScore*1.0 + noPythonScore*1.0 + behaviourScore*4.0) / 6.0

	// Write reward.json
	if err := os.MkdirAll("/logs/verifier", 0755); err != nil {
		fmt.Printf("[verifier] Failed to create reward directory: %v\n", err)
	}
	rewardData := RewardOutput{
		Reward:    reward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}
	rewardBytes, _ := json.MarshalIndent(rewardData, "", "  ")
	if err := os.WriteFile("/logs/verifier/reward.json", rewardBytes, 0644); err != nil {
		fmt.Printf("[verifier] Failed to write reward.json: %v\n", err)
	}
	fmt.Printf("[verifier] Result: reward=%.4f (build=%.1f, no_python=%.1f, behaviour=%.4f)\n",
		reward, buildScore, noPythonScore, behaviourScore)

	// 5. SECRET handling
	if behaviourScore == 1.0 {
		secretBytes, err := os.ReadFile("/tests/secret.b64")
		if err == nil {
			trimmedSecret := strings.TrimSpace(string(secretBytes))
			decodedSecret, decErr := base64.StdEncoding.DecodeString(trimmedSecret)
			if decErr == nil {
				slantCmd := exec.Command("/tmp/figlet", "-f", "slant", string(decodedSecret))
				out, runErr := slantCmd.CombinedOutput()
				if runErr == nil {
					fmt.Println("*** SECRET UNLOCKED ***")
					fmt.Print(string(out))
				} else {
					fmt.Printf("[verifier] Failed to render secret: %v\n", runErr)
				}
			} else {
				fmt.Printf("[verifier] Failed to decode secret: %v\n", decErr)
			}
		} else {
			fmt.Printf("[verifier] Failed to read secret file: %v\n", err)
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
