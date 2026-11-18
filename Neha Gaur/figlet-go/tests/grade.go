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

func findRepoDir() string {
	if _, err := os.Stat("/workspace/repo"); err == nil {
		return "/workspace/repo"
	}
	// Fallback for local testing
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "..", "solution")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "/workspace/repo"
}

func findFigletBin() string {
	if _, err := os.Stat("/tmp/figlet"); err == nil {
		return "/tmp/figlet"
	}
	if filepath.Separator == '\\' {
		candidate := filepath.Join(os.TempDir(), "figlet.exe")
		return candidate
	}
	return "/tmp/figlet"
}

func findHiddenDataPath() string {
	candidates := []string{
		"/tests/data/hidden.jsonl",
		"data/hidden.jsonl",
		"tests/data/hidden.jsonl",
		"../tests/data/hidden.jsonl",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "/tests/data/hidden.jsonl"
}

func findSecretPath() string {
	candidates := []string{
		"/tests/secret.b64",
		"secret.b64",
		"tests/secret.b64",
		"../tests/secret.b64",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "/tests/secret.b64"
}

func checkNoPython(repoDir string) bool {
	hasPy := false
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			hasPy = true
			return filepath.SkipAll
		}
		return nil
	})
	return !hasPy
}

func main() {
	repoDir := findRepoDir()
	figletBin := findFigletBin()
	hiddenPath := findHiddenDataPath()
	secretPath := findSecretPath()

	fmt.Printf("Starting SOP Grader...\n")
	fmt.Printf("Repo Directory: %s\n", repoDir)
	fmt.Printf("Figlet Binary: %s\n", figletBin)
	fmt.Printf("Hidden Cases:  %s\n", hiddenPath)

	// 1. Build component, weight 1: run `go build -o /tmp/figlet .` inside /workspace/repo
	var buildScore float64 = 0.0
	buildCmd := exec.Command("go", "build", "-o", figletBin, ".")
	buildCmd.Dir = repoDir
	buildOutput, buildErr := buildCmd.CombinedOutput()
	if buildErr == nil {
		if _, statErr := os.Stat(figletBin); statErr == nil {
			buildScore = 1.0
			fmt.Printf("[BUILD] SUCCESS (score: 1.0)\n")
		} else {
			fmt.Printf("[BUILD] FAILED: Binary not found after build: %v\n", statErr)
		}
	} else {
		fmt.Printf("[BUILD] FAILED: %v\nOutput: %s\n", buildErr, string(buildOutput))
	}

	// 2. Behaviour component, weight 4: test every case in /tests/data/hidden.jsonl
	// with a 5-second timeout per case, comparing stdout byte-for-byte.
	var behaviourScore float64 = 0.0
	totalCases := 0
	passedCases := 0

	dataFile, err := os.Open(hiddenPath)
	if err != nil {
		fmt.Printf("[BEHAVIOUR] Error opening hidden cases file %s: %v\n", hiddenPath, err)
	} else {
		defer dataFile.Close()
		scanner := bufio.NewScanner(dataFile)
		// Increase buffer size for large Figlet outputs
		buf := make([]byte, 1024*1024)
		scanner.Buffer(buf, 10*1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var tc TestCase
			if err := json.Unmarshal(line, &tc); err != nil {
				fmt.Printf("[BEHAVIOUR] JSON parse error: %v\n", err)
				continue
			}
			totalCases++

			if buildScore == 1.0 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctx, figletBin, tc.Args...)
				// Run outside repo directory (e.g. in /tmp)
				cmd.Dir = os.TempDir()

				var stdoutBuf bytes.Buffer
				cmd.Stdout = &stdoutBuf

				runErr := cmd.Run()
				cancel()

				if runErr == nil {
					if stdoutBuf.String() == tc.Out {
						passedCases++
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("[BEHAVIOUR] Error reading dataset: %v\n", err)
		}
	}

	if totalCases > 0 {
		behaviourScore = float64(passedCases) / float64(totalCases)
	}
	fmt.Printf("[BEHAVIOUR] Passed %d / %d cases (score: %.4f)\n", passedCases, totalCases, behaviourScore)

	// 3. no_python component, weight 1: award 1 only if the build succeeds,
	// no .py files exist under /workspace/repo, and at least one hidden case matches.
	var noPythonScore float64 = 0.0
	if buildScore == 1.0 && passedCases >= 1 && checkNoPython(repoDir) {
		noPythonScore = 1.0
		fmt.Printf("[NO_PYTHON] SUCCESS (score: 1.0)\n")
	} else {
		fmt.Printf("[NO_PYTHON] FAILED (score: 0.0) [build=%.1f, passed=%d, no_py=%v]\n",
			buildScore, passedCases, checkNoPython(repoDir))
	}

	// 4. Calculate weighted mean reward: weights: build=1, no_python=1, behaviour=4 (total=6)
	totalWeight := 1.0 + 1.0 + 4.0
	weightedReward := (1.0*buildScore + 1.0*noPythonScore + 4.0*behaviourScore) / totalWeight
	fmt.Printf("[SUMMARY] Final Weighted Reward: %.6f\n", weightedReward)

	// 5. Write /logs/verifier/reward.json
	result := RewardResult{
		Reward:    weightedReward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}

	resultBytes, _ := json.MarshalIndent(result, "", "  ")

	// Ensure target directory exists
	rewardPaths := []string{
		"/logs/verifier/reward.json",
	}
	if logsDir := filepath.Dir(rewardPaths[0]); logsDir != "" {
		_ = os.MkdirAll(logsDir, 0755)
	}

	for _, p := range rewardPaths {
		if err := os.WriteFile(p, resultBytes, 0644); err != nil {
			fmt.Printf("Warning: Could not write %s: %v\n", p, err)
		} else {
			fmt.Printf("Wrote %s\n", p)
		}
	}

	// Also write reward.txt for backward compatibility
	txtContent := []byte(fmt.Sprintf("%f\n", weightedReward))
	_ = os.WriteFile("/logs/verifier/reward.txt", txtContent, 0644)

	// 6. Unlock and print the base64-decoded secret using `/tmp/figlet -f slant`
	// only when behaviour equals exactly 1.0. Otherwise print "No secret for you. Yet."
	if behaviourScore == 1.0 {
		secretData, err := os.ReadFile(secretPath)
		if err != nil {
			fmt.Printf("Error reading secret file %s: %v\n", secretPath, err)
		} else {
			cleanB64 := strings.TrimSpace(string(secretData))
			decodedSecret, err := base64.StdEncoding.DecodeString(cleanB64)
			if err != nil {
				fmt.Printf("Error decoding base64 secret: %v\n", err)
			} else {
				secretStr := string(decodedSecret)
				slantCmd := exec.Command(figletBin, "-f", "slant", secretStr)
				slantCmd.Dir = os.TempDir()
				slantOutput, slantErr := slantCmd.CombinedOutput()
				if slantErr != nil {
					fmt.Printf("Error rendering secret with figlet: %v\n", slantErr)
				} else {
					fmt.Printf("\n--- SECRET UNLOCKED ---\n%s\n", string(slantOutput))
				}
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
